package pi

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
)

// Owned Pi trust content digests. piTrustHookSource implements the identical
// framing in JavaScript; tests execute both on the same bytes.
//
// Tree: depth-first over lstat entries, children sorted by UTF-8 bytes, each
// record "d\0<rel>\0" or "f\0<rel>\0<sha256 hex of bytes>\0" with "/"
// separators. Only real directories and single-link regular files qualify;
// links, special files and oversized trees are refused. File modes are not
// part of the digest: Pi does not depend on them for these resources.
//
// Value: a parsed JSON value framed as n, t, f, d<IEEE-754 big-endian hex>,
// s<UTF-8 length>:<bytes>, a<count>:<items> and o<count>:<key><value> with keys
// sorted by UTF-8 bytes, so formatting and key order never matter.
const (
	piTrustTreeEntryLimit = 16384
	piTrustTreeByteLimit  = 64 << 20
)

type piTrustBudget struct{ entries, bytes int64 }

func piTrustTreeDigest(root string) (string, error) {
	if piTrustKind(root) != "directory" {
		return "", fmt.Errorf("owned Pi resource %s is not a real directory", filepath.Base(root))
	}
	sum := sha256.New()
	if err := piTrustWalk(root, "", sum, &piTrustBudget{}); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(sum.Sum(nil)), nil
}

func piTrustWalk(directory, relative string, sum hash.Hash, budget *piTrustBudget) error {
	entries, err := os.ReadDir(directory) // sorted by name bytes
	if err != nil {
		return err
	}
	for _, entry := range entries {
		budget.entries++
		if budget.entries > piTrustTreeEntryLimit {
			return fmt.Errorf("owned Pi resource exceeds %d entries", piTrustTreeEntryLimit)
		}
		path, child := filepath.Join(directory, entry.Name()), entry.Name()
		if relative != "" {
			child = relative + "/" + child
		}
		switch piTrustKind(path) {
		case "directory":
			_, _ = sum.Write([]byte("d\x00" + child + "\x00"))
			if err := piTrustWalk(path, child, sum, budget); err != nil {
				return err
			}
		case "file":
			digest, err := piTrustFileDigest(path, budget)
			if err != nil {
				return err
			}
			_, _ = sum.Write([]byte("f\x00" + child + "\x00" + digest + "\x00"))
		default:
			return fmt.Errorf("owned Pi resource holds a link or special file at %s", child)
		}
	}
	return nil
}

func piTrustFileDigest(path string, budget *piTrustBudget) (string, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	single, err := piTrustSingleLink(file, info)
	if err != nil || !info.Mode().IsRegular() || !os.SameFile(before, info) || !single {
		return "", fmt.Errorf("owned Pi file %s is not a single-link regular file", filepath.Base(path))
	}
	if budget.bytes += info.Size(); budget.bytes > piTrustTreeByteLimit {
		return "", fmt.Errorf("owned Pi resource exceeds %d bytes", piTrustTreeByteLimit)
	}
	sum := sha256.New()
	if read, err := io.Copy(sum, io.LimitReader(file, info.Size()+1)); err != nil || read != info.Size() {
		return "", fmt.Errorf("owned Pi file %s changed while hashing", filepath.Base(path))
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func piTrustValueDigest(value any) (string, error) {
	var framed bytes.Buffer
	if err := piTrustFrame(&framed, value); err != nil {
		return "", err
	}
	sum := sha256.Sum256(framed.Bytes())
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func piTrustFrame(out *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case nil:
		out.WriteByte('n')
	case bool:
		out.WriteString(map[bool]string{true: "t", false: "f"}[typed])
	case float64:
		var bits [8]byte
		binary.BigEndian.PutUint64(bits[:], math.Float64bits(typed))
		out.WriteString("d" + hex.EncodeToString(bits[:]))
	case string:
		out.WriteString("s" + strconv.Itoa(len(typed)) + ":" + typed)
	case []any:
		out.WriteString("a" + strconv.Itoa(len(typed)) + ":")
		for _, item := range typed {
			if err := piTrustFrame(out, item); err != nil {
				return err
			}
		}
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		out.WriteString("o" + strconv.Itoa(len(keys)) + ":")
		for _, key := range keys {
			if err := piTrustFrame(out, key); err != nil {
				return err
			}
			if err := piTrustFrame(out, typed[key]); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported JSON value %T", value)
	}
	return nil
}

// piTrustMCPServers parses .pi/mcp.json as Pi does, as generic JSON values.
func piTrustMCPServers(path string) (map[string]any, error) {
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if !piTrustDecodes(body, &document) || document == nil {
		return nil, fmt.Errorf(".pi/mcp.json is not a JSON object")
	}
	servers, ok := document["mcpServers"].(map[string]any)
	if _, present := document["mcpServers"]; present && !ok {
		return nil, fmt.Errorf(".pi/mcp.json mcpServers is not an object")
	}
	return servers, nil
}
