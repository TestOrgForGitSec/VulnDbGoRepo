// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package report

import (
	"regexp"
	"sort"

	"golang.org/x/vulndb/internal/proxy"
	"golang.org/x/vulndb/internal/version"
)

var commitHashRegex = regexp.MustCompile(`^[a-f0-9]+$`)

func (r *Report) Fix() {
	for _, ref := range r.References {
		ref.URL = fixURL(ref.URL)
	}
	for _, m := range r.Modules {
		m.fixVersions()
	}
}

// fixVersions replaces each version with its canonical form (if possible),
// sorts version ranges, and collects version ranges into a compact form.
func (m *Module) fixVersions() {
	fixVersion := func(v string) string {
		if v == "" {
			return ""
		}
		if commitHashRegex.MatchString(v) {
			if c, err := proxy.CanonicalModuleVersion(m.Module, v); err == nil {
				v = c
			}
		}
		v = version.TrimPrefix(v)
		if version.IsValid(v) {
			v = version.Canonical(v)
		}
		return v
	}

	for i, vr := range m.Versions {
		m.Versions[i].Introduced = fixVersion(vr.Introduced)
		m.Versions[i].Fixed = fixVersion(vr.Fixed)
	}
	m.VulnerableAt = fixVersion(m.VulnerableAt)

	sort.SliceStable(m.Versions, func(i, j int) bool {
		intro, fixed := m.Versions[i].Introduced, m.Versions[i].Fixed
		intro2, fixed2 := m.Versions[j].Introduced, m.Versions[j].Fixed
		switch {
		case intro != "" && intro2 != "":
			return version.Before(intro, intro2)
		case intro != "" && fixed2 != "":
			return version.Before(intro, fixed2)
		case fixed != "" && intro2 != "":
			return version.Before(fixed, intro2)
		case fixed != "" && fixed2 != "":
			return version.Before(fixed, fixed2)
		default:
			return false
		}
	})

	// Collect together version ranges that don't need to be separate,
	// e.g:
	// [ {Introduced: 1.1.0}, {Fixed: 1.2.0} ] becomes
	// [ {Introduced: 1.1.0, Fixed: 1.2.0} ].
	for i := 0; i < len(m.Versions); i++ {
		if i != 0 {
			current, prev := m.Versions[i], m.Versions[i-1]
			if (prev.Introduced != "" && prev.Fixed == "") &&
				(current.Introduced == "" && current.Fixed != "") {
				m.Versions[i-1].Fixed = current.Fixed
				m.Versions = append(m.Versions[:i], m.Versions[i+1:]...)
				i--
			}
		}
	}
}

var urlReplacements = []struct {
	re   *regexp.Regexp
	repl string
}{{
	regexp.MustCompile(`golang.org`),
	`go.dev`,
}, {
	regexp.MustCompile(`https?://groups.google.com/forum/\#\![^/]*/([^/]+)/([^/]+)/(.*)`),

	`https://groups.google.com/g/$1/c/$2/m/$3`,
}, {
	regexp.MustCompile(`.*github.com/golang/go/issues`),
	`https://go.dev/issue`,
}, {
	regexp.MustCompile(`.*github.com/golang/go/commit`),
	`https://go.googlesource.com/+`,
},
}

func fixURL(u string) string {
	for _, repl := range urlReplacements {
		u = repl.re.ReplaceAllString(u, repl.repl)
	}
	return u
}
