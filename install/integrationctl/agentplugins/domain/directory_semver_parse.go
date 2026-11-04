package domain

import (
	"errors"
	"strconv"
	"strings"
)

// Exact semantic-version parsing is separated from Directory release selection.
type directorySemverVersion struct {
	core [3]uint64
	pre  []string
}
func parseDirectorySemver(value string) (directorySemverVersion, error) {
	var v directorySemverVersion
	value = strings.SplitN(strings.TrimSpace(value), "+", 2)[0]
	parts := strings.SplitN(value, "-", 2)
	core := strings.Split(parts[0], ".")
	if len(core) != 3 {
		return v, errors.New("not semver")
	}
	for i, s := range core {
		n, e := strconv.ParseUint(s, 10, 64)
		if e != nil || (len(s) > 1 && s[0] == '0') {
			return v, errors.New("not semver")
		}
		v.core[i] = n
	}
	if len(parts) == 2 {
		if parts[1] == "" {
			return v, errors.New("not semver")
		}
		v.pre = strings.Split(parts[1], ".")
	}
	return v, nil
}
