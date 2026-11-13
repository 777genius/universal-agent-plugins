//go:build linux

package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// This is a bounded refusal gate for one fixed subtree, not a mount-table API.
// Unknown escape/propagation forms are refused before any mount authority exists.
const publisherMountInfoLimit = 1 << 20

type publisherMountRow struct {
	id, parent       uint64
	root, target, fs string
	options          []string
}

func publisherMountRows(body []byte) ([]publisherMountRow, error) {
	if len(body) == 0 || len(body) > publisherMountInfoLimit || !utf8.Valid(body) || body[len(body)-1] != '\n' || bytes.IndexByte(body, 0) >= 0 {
		return nil, fmt.Errorf("publisher mount table bound/encoding refused")
	}
	rows := []publisherMountRow{}
	ids := map[uint64]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(string(body), "\n"), "\n") {
		if len(line) > 8192 || len(rows) >= 4096 {
			return nil, fmt.Errorf("publisher mount table row bound refused")
		}
		row, err := publisherMountRowFromLine(line)
		if err != nil || ids[row.id] {
			return nil, fmt.Errorf("publisher mount row/duplicate refused")
		}
		ids[row.id] = true
		rows = append(rows, row)
	}
	return rows, nil
}

func publisherMountPath(raw string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			out.WriteByte(raw[i])
			continue
		}
		if i+3 >= len(raw) {
			return "", fmt.Errorf("mount path escape refused")
		}
		switch raw[i+1 : i+4] {
		case "040":
			out.WriteByte(' ')
		case "011":
			out.WriteByte('\t')
		case "012":
			out.WriteByte('\n')
		case "134":
			out.WriteByte('\\')
		default:
			return "", fmt.Errorf("mount path escape refused")
		}
		i += 3
	}
	path := out.String()
	if !strings.HasPrefix(path, "/") {
		return "", fmt.Errorf("absolute mount path required")
	}
	return path, nil
}

func publisherOriginalTopology(body []byte, shareMount uint64) ([]publisherMountRow, error) {
	rows, err := publisherMountRows(body)
	if err != nil {
		return nil, err
	}
	found := false
	for _, row := range rows {
		if strings.HasPrefix(row.target, "/usr/share/") {
			return nil, fmt.Errorf("preexisting publisher subtree mount refused")
		}
		if row.id == shareMount && (row.target == "/" || row.target == "/usr" || row.target == "/usr/share") {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("held publisher parent mount missing")
	}
	return rows, nil
}

func publisherMountRowFromLine(line string) (publisherMountRow, error) {
	fields := strings.Fields(line)
	if len(fields) < 10 {
		return publisherMountRow{}, fmt.Errorf("publisher mount table shape refused")
	}
	split := 6
	for split < len(fields) && fields[split] != "-" {
		split++
	}
	if split+4 != len(fields) {
		return publisherMountRow{}, fmt.Errorf("publisher mount table separator refused")
	}
	id, err := strconv.ParseUint(fields[0], 10, 64)
	parent, pe := strconv.ParseUint(fields[1], 10, 64)
	if err != nil || pe != nil || id == 0 || parent == 0 {
		return publisherMountRow{}, fmt.Errorf("publisher mount table identity refused")
	}
	root, re := publisherMountPath(fields[3])
	target, te := publisherMountPath(fields[4])
	if re != nil || te != nil {
		return publisherMountRow{}, fmt.Errorf("publisher mount path refused")
	}
	// No propagation relationship is allowed anywhere in the private view.
	if split != 6 {
		return publisherMountRow{}, fmt.Errorf("publisher mount propagation/optional field refused")
	}
	return publisherMountRow{id, parent, root, target, fields[split+1], strings.Split(fields[5], ",")}, nil
}
