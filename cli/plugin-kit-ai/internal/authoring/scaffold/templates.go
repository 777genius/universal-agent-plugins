package scaffold

import (
	"bytes"
	_ "embed"
	"fmt"
	"regexp"
	"strings"
)

// Generated npm fixtures supplied with this task, not handwritten source.
// Only the two root lock names and package.json name are substituted. All
// resolution, integrity, version and formatting bytes are preserved.
//
//go:embed templates/package.json
var nodePackage []byte

//go:embed templates/package-lock.json
var nodeLock []byte

func nodePackages(name string) ([]byte, []byte, error) {
	old := []byte(`"name":"agent-plugin-template"`)
	lockOld := []byte(`"name": "agent-plugin-template"`)
	if bytes.Count(nodePackage, old) != 1 || bytes.Count(nodeLock, lockOld) != 2 {
		return nil, nil, fmt.Errorf("embedded Node fixture root names changed")
	}
	return bytes.Replace(nodePackage, old, []byte(`"name":"`+name+`"`), 1), bytes.Replace(nodeLock, lockOld, []byte(`"name": "`+name+`"`), 2), nil
}

var yearPattern = regexp.MustCompile(`^[0-9]{4}$`)

func licenseText(o Options) (string, error) {
	if o.License == "" {
		if o.CopyrightHolder != "" || o.CopyrightYear != "" {
			return "", fmt.Errorf("copyright values require an explicit license")
		}
		return "", nil
	}
	if o.License != "MIT" && o.License != "ISC" {
		return "", fmt.Errorf("unsupported license %q; supported explicit choices: MIT, ISC", o.License)
	}
	if !textValue(o.CopyrightHolder, 256) || strings.ContainsAny(o.CopyrightHolder, "\n\t") || !yearPattern.MatchString(o.CopyrightYear) {
		return "", fmt.Errorf("license requires explicit single-line copyright holder and four-digit year")
	}
	copyright := "Copyright (c) " + o.CopyrightYear + " " + o.CopyrightHolder + "\n\n"
	if o.License == "MIT" {
		return "MIT License\n\n" + copyright + `Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`, nil
	}
	return "ISC License\n\n" + copyright + `Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY
AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR
OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
PERFORMANCE OF THIS SOFTWARE.
`, nil
}
