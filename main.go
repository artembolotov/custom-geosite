package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	domainTypePlain  = 0
	domainTypeRegex  = 1
	domainTypeDomain = 2
	domainTypeFull   = 3
)

func main() {
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
			continue
		}

		var domains [][]byte
		for _, e := range entries {
			domains = append(domains, encodeDomain(e.typ, e.value))
		}

		geoSites = append(geoSites, encodeGeoSite(tag, domains))
	}

	out := encodeGeoSiteList(geoSites)
	if err := os.WriteFile("custom.dat", out, 0644); err != nil {
		os.Exit(1)
	}
}

type domainEntry struct {
	typ   int
	value string
}

func readDomains(path string) ([]domainEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []domainEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		e, err := parseLine(line)
		if err != nil {
			return nil, err
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
			return domainEntry{typ: pf.t, value: val}, nil
		}
	}
	return domainEntry{typ: domainTypeDomain, value: line}, nil
}

func encodeDomain(typ int, value string) []byte {
	var b []byte
	if typ != 0 {
		b = appendVarintField(b, 1, uint64(typ))
	}
	b = appendStringField(b, 2, value)
	return b
}

func encodeGeoSite(tag string, domains [][]byte) []byte {
	var b []byte
	b = appendStringField(b, 1, tag)
	for _, d := range domains {
		b = appendBytesField(b, 2, d)
	}
	return b
}

func encodeGeoSiteList(geoSites [][]byte) []byte {
	var b []byte
	for _, gs := range geoSites {
		b = appendBytesField(b, 1, gs)
	}
	return b
}

func appendVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func appendVarintField(b []byte, fieldNum int, v uint64) []byte {
	b = appendVarint(b, uint64(fieldNum<<3|0))
	b = appendVarint(b, v)
	return b
}

func appendBytesField(b []byte, fieldNum int, data []byte) []byte {
	b = appendVarint(b, uint64(fieldNum<<3|2))
	b = appendVarint(b, uint64(len(data)))
	b = append(b, data...)
	return b
}

func appendStringField(b []byte, fieldNum int, s string) []byte {
	return appendBytesField(b, fieldNum, []byte(s))
}