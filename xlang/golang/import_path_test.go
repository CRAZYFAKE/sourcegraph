// Copyright 2014 The Go Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd.

package golang

import (
	"io/ioutil"
	"net/http"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestResolveStaticImportPath(t *testing.T) {
	tests := []struct {
		importPath string
		dir        *directory
	}{
		{"fmt", &directory{
			importPath:  "fmt",
			projectRoot: "",
			cloneURL:    "https://github.com/golang/go",
			repoPrefix:  "src",
			vcs:         "git",
			rev:         runtime.Version(),
		}},
	}

	for _, tt := range tests {
		dir, err := resolveStaticImportPath(tt.importPath)
		if err != nil {
			t.Errorf("%s: %s", tt.importPath, err)
			continue
		}
		if !reflect.DeepEqual(dir, tt.dir) {
			t.Errorf("resolveStaticImportPath(%q) =\n     %+v,\nwant %+v", tt.importPath, dir, tt.dir)
		}
	}
}

type testTransport map[string]string

func (t testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	statusCode := http.StatusOK
	req.URL.RawQuery = ""
	body, ok := t[req.URL.String()]
	if !ok {
		statusCode = http.StatusNotFound
	}
	resp := &http.Response{
		StatusCode: statusCode,
		Body:       ioutil.NopCloser(strings.NewReader(body)),
	}
	return resp, nil
}

var githubPattern = regexp.MustCompile(`^github\.com/(?P<owner>[a-z0-9A-Z_.\-]+)/(?P<repo>[a-z0-9A-Z_.\-]+)(?P<dir>/[a-z0-9A-Z_.\-/]*)?$`)

func TestResolveDynamicImportPath(t *testing.T) {
	tests := []struct {
		importPath string
		dir        *directory
	}{
		{"alice.org/pkg", &directory{
			importPath:   "alice.org/pkg",
			projectRoot:  "alice.org/pkg",
			cloneURL:     "https://github.com/alice/pkg",
			resolvedPath: "github.com/alice/pkg",
			vcs:          "git",
		}},
		{"alice.org/pkg/sub", &directory{
			importPath:   "alice.org/pkg/sub",
			projectRoot:  "alice.org/pkg",
			cloneURL:     "https://github.com/alice/pkg",
			resolvedPath: "github.com/alice/pkg/sub",
			vcs:          "git",
		}},
		{"alice.org/pkg/http", &directory{
			importPath:   "alice.org/pkg/http",
			projectRoot:  "alice.org/pkg",
			cloneURL:     "https://github.com/alice/pkg",
			resolvedPath: "github.com/alice/pkg/http",
			vcs:          "git",
		}},
		{"alice.org/pkg/ignore", &directory{
			importPath:   "alice.org/pkg/ignore",
			projectRoot:  "alice.org/pkg",
			cloneURL:     "https://github.com/alice/pkg",
			resolvedPath: "github.com/alice/pkg/ignore",
			vcs:          "git",
		}},
		{"alice.org/pkg/mismatch", nil},
		{"alice.org/pkg/multiple", nil},
		{"alice.org/pkg/notfound", nil},

		{"bob.com/pkg", &directory{
			importPath:   "bob.com/pkg",
			projectRoot:  "bob.com/pkg",
			cloneURL:     "https://vcs.net/bob/pkg.git",
			resolvedPath: "vcs.net/bob/pkg.git",
			vcs:          "git",
		}},
		{"bob.com/pkg/sub", &directory{
			importPath:   "bob.com/pkg/sub",
			projectRoot:  "bob.com/pkg",
			cloneURL:     "https://vcs.net/bob/pkg.git",
			resolvedPath: "vcs.net/bob/pkg.git/sub",
			vcs:          "git",
		}},
	}

	pages := map[string]string{
		// Package at root of a GitHub repo.
		"https://alice.org/pkg": `<head> <meta name="go-import" content="alice.org/pkg git https://github.com/alice/pkg"></head>`,
		// Package in sub-diretory.
		"https://alice.org/pkg/sub": `<head> <meta name="go-import" content="alice.org/pkg git https://github.com/alice/pkg"><body>`,
		// Fallback to http.
		"http://alice.org/pkg/http": `<head> <meta name="go-import" content="alice.org/pkg git https://github.com/alice/pkg">`,
		// Meta tag in sub-directory does not match meta tag at root.
		"https://alice.org/pkg/mismatch": `<head> <meta name="go-import" content="alice.org/pkg hg https://github.com/alice/pkg">`,
		// More than one matching meta tag.
		"http://alice.org/pkg/multiple": `<head> ` +
			`<meta name="go-import" content="alice.org/pkg git https://github.com/alice/pkg">` +
			`<meta name="go-import" content="alice.org/pkg git https://github.com/alice/pkg">`,
		// Unknown meta name
		"https://alice.org/pkg/ignore": `<meta name="go-junk" content="alice.org/pkg http://alice.org/pkg http://alice.org/pkg{/dir} http://alice.org/pkg{/dir}?f={file}#Line{line}">` +
			// go-import tag for the package
			`<meta name="go-import" content="alice.org/pkg git https://github.com/alice/pkg">` +
			// go-import with wrong number of fields
			`<meta name="go-import" content="alice.org/pkg https://github.com/alice/pkg">` +
			// go-import with no fields
			`<meta name="go-import" content="">` +
			// meta tag for a different package
			`<meta name="go-import" content="alice.org/other git https://github.com/alice/other">` +
			// meta tag for a different package
			`<meta name="go-import" content="alice.org/other git https://github.com/alice/other">` +
			`</head>` +
			// go-import outside of head
			`<meta name="go-import" content="alice.org/pkg git https://github.com/alice/pkg">`,

		// Package at root of a Git repo.
		"https://bob.com/pkg": `<head> <meta name="go-import" content="bob.com/pkg git https://vcs.net/bob/pkg.git">`,
		// Package at in sub-directory of a Git repo.
		"https://bob.com/pkg/sub": `<head> <meta name="go-import" content="bob.com/pkg git https://vcs.net/bob/pkg.git">`,
	}
	client := &http.Client{Transport: testTransport(pages)}

	for _, tt := range tests {
		dir, err := resolveDynamicImportPath(client, tt.importPath)

		if tt.dir == nil {
			if err == nil {
				t.Errorf("resolveDynamicImportPath(client, %q) did not return expected error", tt.importPath)
			}
			continue
		}

		if err != nil {
			t.Errorf("resolveDynamicImportPath(client, %q) return unexpected error: %v", tt.importPath, err)
			continue
		}

		if !reflect.DeepEqual(dir, tt.dir) {
			t.Errorf("resolveDynamicImportPath(client, %q) =\n     %+v,\nwant %+v", tt.importPath, dir, tt.dir)
		}
	}
}
