package grok

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
	"testing"
)

const rawMemberID = "project/1234abcd/owned"

func TestProspectiveRawMembershipPreservesEveryForeignByte(t *testing.T) {
	for _, array := range []string{"[]", "[ ]", "['foreign']", "['foreign',]", "[ 'foreign' ]"} {
		t.Run(array, func(t *testing.T) {
			original := []byte("# PRIVATE foreign comment\r\n[plugins]\r\nenabled = " + array + " # exact tail\r\ndisabled = ['other']\r\n[models]\r\nvalue = 'PRIVATE'\r\n")
			saved := bytes.Clone(original)
			result, err := prepareProjectEnableBytes(original, rawMemberID, "owned")
			if err != nil {
				t.Fatal(err)
			}
			// Removing the sole inserted range must recover all original bytes.
			token := []byte("\"" + rawMemberID + "\"")
			start := bytes.Index(result, token)
			if start < 0 {
				t.Fatal("native ID absent")
			}
			position := bytes.Index(original, []byte("] # exact tail"))
			inserted := len(result) - len(original)
			if position < 0 || inserted <= 0 || !bytes.Equal(result[:position], original[:position]) || !bytes.Equal(result[position+inserted:], original[position:]) {
				t.Fatal("insertion changed an original byte")
			}
			restored := append(bytes.Clone(result[:position]), result[position+inserted:]...)
			if !bytes.Equal(restored, original) || !bytes.Equal(original, saved) {
				t.Fatal("foreign bytes or predecessor changed")
			}
			if _, err := prepareProjectEnableBytes(result, rawMemberID, "owned"); err == nil {
				t.Fatal("already enabled must not mutate")
			}
		})
	}
}

func TestProspectiveRawMembershipRefusesUnsupportedOrDisabledPredecessor(t *testing.T) {
	for _, source := range []string{
		"plugins={enabled=[],disabled=[]}\n",
		"plugins.enabled=[]\nplugins.disabled=[]\n",
		"[plugins]\n'enabled'=[]\ndisabled=[]\n",
		"[plugins]\nenabled=[\n'foreign',\n]\ndisabled=[]\n",
		"[plugins]\nenabled=[1]\ndisabled=[]\n",
		"[plugins]\nenabled=[]\ndisabled=['owned']\n",
		"[plugins]\nenabled=[]\ndisabled=['" + rawMemberID + "']\n",
		"[plugins]\nenabled=['owned']\ndisabled=[]\n",
		"[plugins]\nenabled=['alias','ALIAS']\ndisabled=[]\n",
		"[plugins]\nenabled=[]\nEnabled=[]\ndisabled=[]\n",
		"[plugins]\nenabled=[]\ndisabled=[]\n[Plugins]\nvalue=1\n",
		"[plugins]\nenabled=[]\nenabled=[]\ndisabled=[]\n",
	} {
		t.Run(source, func(t *testing.T) {
			original := []byte(source)
			saved := bytes.Clone(original)
			if result, err := prepareProjectEnableBytes(original, rawMemberID, "owned"); err == nil || result != nil {
				t.Fatal("unsupported predecessor admitted")
			}
			if !bytes.Equal(original, saved) {
				t.Fatal("predecessor mutated on refusal")
			}
		})
	}
}

func TestProspectiveRawMembershipBoundsBeforeParsingAndRefusesUnsafeIDs(t *testing.T) {
	original := []byte("[plugins]\nenabled=[]\ndisabled=[]\n")
	for _, id := range []string{"", "line\nvalue", "quote\"value", "slash\\value", "é"} {
		if result, err := prepareProjectEnableBytes(original, id, "owned"); err == nil || result != nil {
			t.Fatal("unsafe ID admitted")
		}
	}
	tooLarge := bytes.Repeat([]byte("!"), grokMembershipLimit+1)
	if _, err := prepareProjectEnableBytes(tooLarge, rawMemberID, "owned"); err == nil {
		t.Fatal("oversize input reached parser admission")
	}
	// The input fits, but the postimage would exceed the independent ceiling.
	prefix := bytes.Repeat([]byte("#"), grokMembershipLimit-len(original)-1)
	atLimit := append(append(prefix, '\n'), original...)
	if len(atLimit) != grokMembershipLimit {
		t.Fatal("bound fixture size")
	}
	if _, err := prepareProjectEnableBytes(atLimit, rawMemberID, "owned"); err == nil {
		t.Fatal("oversize postimage admitted")
	}
}

func TestProspectiveMembershipAdmitsActualDefaultAndMissingArrays(t *testing.T) {
	actual := "[marketplace]\ndefault_skills_installs_purged = true\n"
	if len(actual) != 52 || fmt.Sprintf("%x", sha256.Sum256([]byte(actual))) != "fe7b6bc7eb3f5321649cb10fd4ce35096cee471f9c8d04fd34958e848039ff92" {
		t.Fatal("actual original default fixture differs")
	}
	for _, source := range []string{actual, "", " \n", "# retained without final newline", "[plugins]\n", "[plugins]\ndisabled=['foreign']\n", "[plugins]\nenabled=[]\n", "[plugins]\npaths=['retained']\n[other]\nvalue='private'\n", "[plugins]\r\ndisabled=[]\r\n[other]\r\nvalue=true\r\n"} {
		t.Run(source, func(t *testing.T) {
			original := []byte(source)
			before := bytes.Clone(original)
			result, err := prepareProjectEnableBytes(original, "project/1234abcd/alternate", "alternate")
			if err != nil {
				t.Fatal(err)
			}
			// The one inserted contiguous range must be removable without changing any original byte.
			position := 0
			for position < len(original) && original[position] == result[position] {
				position++
			}
			added := len(result) - len(original)
			if added <= 0 || !bytes.Equal(append(bytes.Clone(result[:position]), result[position+added:]...), original) || !bytes.Equal(original, before) {
				t.Fatal("original bytes changed")
			}
			parsed, lists, err := membershipDocument(result)
			if err != nil || !slices.Equal(lists["enabled"], []string{"project/1234abcd/alternate"}) {
				t.Fatal(parsed, lists, err)
			}
			if !strings.Contains(source, "disabled") {
				if _, present := parsed["plugins"].(map[string]any)["disabled"]; present {
					t.Fatal("absent disabled array was synthesized")
				}
			}
		})
	}
}
func TestProspectiveMissingEnabledPreservesDisablementAndRejectsAmbiguity(t *testing.T) {
	for _, source := range []string{
		"[plugins]\ndisabled=['alternate']\n",
		"[plugins]\ndisabled=['PROJECT/1234abcd/alternate']\n",
		"[plugins]\ndisabled=1\n",
		"[plugins]\nenabled=true\n",
		"plugins={disabled=[]}\n",
		"plugins.disabled=[]\n",
		"[plugins.child]\nvalue=true\n",
		"[Plugins]\ndisabled=[]\n",
		"[[plugins]]\ndisabled=[]\n",
	} {
		original := []byte(source)
		saved := bytes.Clone(original)
		if result, err := prepareProjectEnableBytes(original, "project/1234abcd/alternate", "alternate"); err == nil || result != nil {
			t.Fatal("unsupported original admitted", source)
		}
		if !bytes.Equal(original, saved) {
			t.Fatal("refusal mutated input")
		}
	}
}
