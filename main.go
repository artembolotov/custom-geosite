package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Domain type enum values — must match v2ray-core app/router/config.proto
const (
	domainTypePlain  = 0
	domainTypeRegex  = 1
	domainTypeDomain = 2
	domainTypeFull   = 3
)

func main() {
	// each filename in lists/ becomes a tag in the resulting .dat
	tags := []string{"direct", "proxy"}
	var geoSites [][]byte

	for _, tag := range tags {
		path := filepath.Join("lists", tag+".txt")
		entries, err := readDomains(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", path, err)
			os.Exit(1)
		}
		if len(entries) == 0 {
			fmt.Fprintf(os.Stderr, "warning: %s is empty, skipping tag\n", tag)
			continue
		}

		var domains [][]byte
		for _, e := range entries {
			domains = append(domains, encodeDomain(e.typ, e.value))
		}

		geoSites = append(geoSites, encodeGeoSite(tag, domains))
		fmt.Printf("[%s] %d domains\n", tag, len(entries))
	}

	if len(geoSites) == 0 {
		fmt.Fprintln(os.Stderr, "no domains to write")
		os.Exit(1)
	}

	out := encodeGeoSiteList(geoSites)
	if err := os.WriteFile("custom.dat", out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing custom.dat: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("done: custom.dat (%d bytes)\n", len(out))
}

type domainEntry struct {
	typ   int
	value string
}

// readDomains reads a text file, one domain per line.
// Supported prefixes: full:, domain:, regex:
// No prefix is treated as domain: match.
// Lines starting with # are comments; empty lines are skipped.
func readDomains(path string) ([]domainEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []domainEntry
	scanner := bufio.NewScanner(f)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		e, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

func parseLine(line string) (domainEntry, error) {
	prefixes := []struct {
		p string
		t int
	}{
		{"full:", domainTypeFull},
		{"domain:", domainTypeDomain},
		{"regex:", domainTypeRegex},
	}
	for _, pf := range prefixes {
		if strings.HasPrefix(line, pf.p) {
			val := strings.TrimPrefix(line, pf.p)
			if val == "" {
				return domainEntry{}, fmt.Errorf("empty value after %s", pf.p)
			}
			return domainEntry{typ: pf.t, value: val}, nil
		}
	}
	// no prefix → domain match (host + all subdomains)
	return domainEntry{typ: domainTypeDomain, value: line}, nil
}

// ---------------------------------------------------------------------------
// Protobuf encoding — hand-rolled, zero external dependencies.
// Schema: v2ray-core app/router/config.proto (GeoSiteList / GeoSite / Domain)
// ---------------------------------------------------------------------------

// encodeDomain serializes a Domain message.
//
//	field 1: type  (enum → varint)
//	field 2: value (string)
func encodeDomain(typ int, value string) []byte {
	var b []byte
	if typ != 0 { // proto3: zero value is omitted from the wire
		b = appendVarintField(b, 1, uint64(typ))
	}
	b = appendStringField(b, 2, value)
	return b
}

// encodeGeoSite serializes a GeoSite message.
//
//	field 1: domain (repeated embedded message)
//	field 2: tag    (string)
func encodeGeoSite(tag string, domains [][]byte) []byte {
	var b []byte
	for _, d := range domains {
		b = appendBytesField(b, 1, d)
	}
	b = appendStringField(b, 2, tag)
	return b
}

// encodeGeoSiteList serializes a GeoSiteList message.
//
//	field 1: geosite (repeated embedded message)
func encodeGeoSiteList(geoSites [][]byte) []byte {
	var b []byte
	for _, gs := range geoSites {
		b = appendBytesField(b, 1, gs)
	}
	return b
}

// --- low-level protobuf wire helpers ---

func appendVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

// appendVarintField writes a varint field (wire type 0).
func appendVarintField(b []byte, fieldNum int, v uint64) []byte {
	b = appendVarint(b, uint64(fieldNum<<3|0))
	b = appendVarint(b, v)
	return b
}

// appendBytesField writes a length-delimited field (wire type 2).
func appendBytesField(b []byte, fieldNum int, data []byte) []byte {
	b = appendVarint(b, uint64(fieldNum<<3|2))
	b = appendVarint(b, uint64(len(data)))
	b = append(b, data...)
	return b
}

// appendStringField writes a string as a length-delimited field.
func appendStringField(b []byte, fieldNum int, s string) []byte {
	return appendBytesField(b, fieldNum, []byte(s))
}
