package main

import "strings"

func comparePublicationIssue(a, b publicationIssue) int {
	if cmp := strings.Compare(a.Code, b.Code); cmp != 0 {
		return cmp
	}
	if cmp := strings.Compare(a.Target, b.Target); cmp != 0 {
		return cmp
	}
	if cmp := strings.Compare(a.ChannelFamily, b.ChannelFamily); cmp != 0 {
		return cmp
	}
	return strings.Compare(a.Path, b.Path)
}
