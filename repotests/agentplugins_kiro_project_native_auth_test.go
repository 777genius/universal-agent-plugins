//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type kiroAttemptAuth struct {
	key   string
	forms [][]byte
}

func newKiroAttemptAuth(key string) (kiroAttemptAuth, error) {
	auth := kiroAttemptAuth{key: key}
	if key == "" {
		return auth, nil
	}
	if len(key) < 8 || len(key) > 4096 {
		return kiroAttemptAuth{}, fmt.Errorf("Kiro CI key refused by private capture bounds")
	}
	for _, char := range key {
		if char < 33 || char > 126 {
			return kiroAttemptAuth{}, fmt.Errorf("Kiro CI key refused by private capture bounds")
		}
	}
	quoted := []byte(strconv.Quote(key))
	auth.forms = [][]byte{[]byte(key), quoted[1 : len(quoted)-1]}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		auth.forms = append(auth.forms, []byte(encoding.EncodeToString([]byte(key))))
	}
	for _, form := range auth.forms {
		if len(form) > 4096 {
			return kiroAttemptAuth{}, fmt.Errorf("Kiro CI key refused by private capture bounds")
		}
	}
	return auth, nil
}

func (a kiroAttemptAuth) commandEnv(client, label string, original []string) []string {
	result := append([]string{}, original...)
	if client == "kiro" && label == "native-attempt" && a.key != "" {
		result = append(result, "KIRO_API_KEY="+a.key)
	}
	return result
}

func (a kiroAttemptAuth) redact(body []byte) []byte {
	for _, form := range a.forms {
		body = bytes.ReplaceAll(body, form, []byte("[redacted]"))
	}
	return body
}

func TestKiroDedicatedKeyIsOptionalBoundedAndOnlyEntersNativeAttempt(t *testing.T) {
	for _, value := range []string{"short", "private key", "private\nkey", "private\x00key", "private\x7fkey", "private\u00e9key", strings.Repeat("a", 4096)} {
		if _, err := newKiroAttemptAuth(value); err == nil || strings.Contains(err.Error(), value) {
			t.Fatal("invalid private key admitted or disclosed")
		}
	}
	for _, key := range []string{"", "ksk_inert_private_key_12345"} {
		auth, err := newKiroAttemptAuth(key)
		if err != nil {
			t.Fatal(err)
		}
		original := []string{"HOME=/original", "PATH=/usr/bin:/bin"}
		for _, client := range []string{"kiro", "qoder", "grok", "windsurf"} {
			for _, label := range []string{"git-a", "version", "add-a", "native-attempt"} {
				expected := append([]string{}, original...)
				if key != "" && client == "kiro" && label == "native-attempt" {
					expected = append(expected, "KIRO_API_KEY="+key)
				}
				if !reflect.DeepEqual(auth.commandEnv(client, label, original), expected) {
					t.Fatal("private key crossed selected native command boundary")
				}
			}
		}
		if !reflect.DeepEqual(original, []string{"HOME=/original", "PATH=/usr/bin:/bin"}) {
			t.Fatal("caller environment changed")
		}
	}
}

func TestKiroDedicatedKeyRedactsBeforeEvidenceKeep(t *testing.T) {
	auth, err := newKiroAttemptAuth(`ksk_inert_private_"key_12345`)
	if err != nil {
		t.Fatal(err)
	}
	for index, form := range auth.forms {
		f := &projectNativeCLIAttempt{files: map[string][]byte{}, kiroAuth: auth}
		body := append(append([]byte("before "), form...), []byte(" after")...)
		original := bytes.Clone(body)
		f.keep(t, "native-attempt.stdout.log", body)
		if !bytes.Equal(f.files["native-attempt.stdout.log"], []byte("before [redacted] after")) || !bytes.Equal(body, original) {
			t.Fatal("private form retained or raw caller buffer changed", index)
		}
	}
}
