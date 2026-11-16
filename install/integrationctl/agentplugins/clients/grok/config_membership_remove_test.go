package grok

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

func TestMembershipRemovalUsesCurrentFullIDAndPreservesForeignAdditions(t *testing.T) {
	for _, list := range []string{
		"[\"" + rawMemberID + "\"]",
		"[ \"" + rawMemberID + "\", ]",
		"['foreign-before', \"" + rawMemberID + "\"]",
		"[\"" + rawMemberID + "\", 'foreign-after']",
		"['foreign-before', '" + rawMemberID + "', 'foreign-after']",
		"['foreign-before', '" + rawMemberID + "', ]",
	} {
		t.Run(list, func(t *testing.T) {
			original := []byte("[models]\nvalue='PRIVATE_TOKEN'\n[plugins]\ndisabled=['foreign-disabled']\nenabled=" + list + " # retained\n[after]\nvalue=1\n")
			result, err := prepareProjectRemoveBytes(original, rawMemberID)
			if err != nil || !result.Present {
				t.Fatal(result, err)
			}
			_, lists, err := membershipDocument(result.Body)
			if err != nil || membershipContains(lists["enabled"], rawMemberID) || !slices.Equal(lists["disabled"], []string{"foreign-disabled"}) {
				t.Fatal(lists, err)
			}
			for _, foreign := range []string{"foreign-before", "foreign-after"} {
				if strings.Contains(list, foreign) && !membershipContains(lists["enabled"], foreign) {
					t.Fatal("foreign addition was removed")
				}
			}
			restored := append(bytes.Clone(result.Body[:result.Offset]), []byte(result.Removed)...)
			restored = append(restored, result.Body[result.Offset:]...)
			if !bytes.Equal(restored, original) || strings.Contains(result.Removed, "PRIVATE_TOKEN") || strings.Contains(result.Removed, "foreign") {
				t.Fatal("inverse span differs or includes foreign values")
			}
		})
	}
}

func TestMembershipRemovalDoesNotConsumeBareNameOrDisabledPreference(t *testing.T) {
	for _, original := range []string{
		"[plugins]\nenabled=['owned']\ndisabled=['" + rawMemberID + "']\n",
		"[plugins]\ndisabled=['" + rawMemberID + "']\n",
		"[models]\nvalue='retained'\n",
	} {
		result, err := prepareProjectRemoveBytes([]byte(original), rawMemberID)
		if err != nil || result.Present || string(result.Body) != original {
			t.Fatal("non-owned membership changed", result, err)
		}
	}
	for _, original := range []string{
		"[plugins]\nenabled=['PROJECT/1234abcd/owned']\n",
		"[plugins]\nenabled=['" + rawMemberID + "','" + rawMemberID + "']\n",
		"[plugins]\nenabled=[\n'" + rawMemberID + "',\n]\n",
	} {
		if _, err := prepareProjectRemoveBytes([]byte(original), rawMemberID); err == nil {
			t.Fatal("ambiguous or unqualified current array admitted")
		}
	}
}
