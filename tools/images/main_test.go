package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindOGImage(t *testing.T) {
	const page = "https://site.example/some/page"
	tests := []struct {
		name, html, want string
	}{
		{"og:image absolute",
			`<meta property="og:image" content="https://cdn.example/a.png">`,
			"https://cdn.example/a.png"},
		{"og:image relative resolves against the page",
			`<meta property="og:image" content="/img/a.png">`,
			"https://site.example/img/a.png"},
		{"og:image relative to the current directory",
			`<meta property="og:image" content="a.png">`,
			"https://site.example/some/a.png"},
		{"og:image:url variant",
			`<meta property="og:image:url" content="https://cdn.example/b.png">`,
			"https://cdn.example/b.png"},
		{"name= instead of property=",
			`<meta name="og:image" content="https://cdn.example/c.png">`,
			"https://cdn.example/c.png"},
		{"twitter:image fallback",
			`<meta name="twitter:image" content="https://cdn.example/d.png">`,
			"https://cdn.example/d.png"},
		{"content before property",
			`<meta content="https://cdn.example/e.png" property="og:image">`,
			"https://cdn.example/e.png"},
		{"single quotes",
			`<meta property='og:image' content='https://cdn.example/f.png'>`,
			"https://cdn.example/f.png"},
		{"entities decoded",
			`<meta property="og:image" content="https://cdn.example/g.png?a=1&amp;b=2">`,
			"https://cdn.example/g.png?a=1&b=2"},
		{"no tag at all", `<html><body>hi</body></html>`, ""},
		{"tag with empty content", `<meta property="og:image" content="">`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findOGImage([]byte(tt.html), page)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestImageExt(t *testing.T) {
	pad := func(b []byte) []byte { return append(b, bytes.Repeat([]byte{0}, 2048)...) }

	png := pad([]byte("\x89PNG\r\n\x1a\n"))
	jpg := pad([]byte("\xff\xd8\xff\xe0"))
	gif := pad([]byte("GIF89a"))

	for _, tt := range []struct {
		name, want string
		body       []byte
	}{
		{"png", ".png", png},
		{"jpeg", ".jpg", jpg},
		{"gif", ".gif", gif},
	} {
		got, err := imageExt(tt.body, "image/whatever")
		if err != nil || got != tt.want {
			t.Errorf("%s: got (%q, %v), want %q", tt.name, got, err, tt.want)
		}
	}

	// A server claiming image/png while serving an HTML error page is the case
	// this exists for: sniffed bytes win over the declared header.
	html := pad([]byte("<!DOCTYPE html><html><body>404 not found"))
	if _, err := imageExt(html, "image/png"); err == nil {
		t.Error("accepted an HTML body declared as image/png")
	}
	if _, err := imageExt([]byte("\x89PNG\r\n\x1a\n"), "image/png"); err == nil {
		t.Error("accepted a 8-byte 'image' — almost certainly a tracking pixel or an error")
	}
}

// The URLs here come from submitters and this runs on a maintainer's laptop,
// which is usually on a network with interesting things at 192.168.x.x.
func TestGet_RefusesNonPublicAddresses(t *testing.T) {
	for _, target := range []string{
		"http://127.0.0.1:80/",
		"http://localhost/",
		"http://[::1]/",
	} {
		_, _, err := get(context.Background(), target, 1024)
		if err == nil {
			t.Errorf("%s: fetch succeeded, want refusal", target)
			continue
		}
		if !strings.Contains(err.Error(), "non-public address") &&
			!strings.Contains(err.Error(), "refusing") {
			t.Logf("%s: refused for another reason (%v) — acceptable", target, err)
		}
	}
	if _, _, err := get(context.Background(), "ftp://example.com/x", 1024); err == nil {
		t.Error("accepted a non-http scheme")
	}
}

// A dead domain must fail outright, never fall through to a screenshot: Chrome
// happily renders its own "This site can't be reached" page and writing that to
// disk turns a dead listing into a card that looks like a broken product.
func TestFetchImage_UnreachableSiteNeverScreenshots(t *testing.T) {
	_, _, err := fetchImage(
		project{Slug: "dead", Name: "Dead", URL: "https://nx-does-not-resolve.example/"},
		"/nonexistent/chrome-that-would-fail-loudly-if-called",
	)
	if err == nil {
		t.Fatal("expected an error for an unreachable site")
	}
	if !strings.Contains(err.Error(), "site unreachable") {
		t.Errorf("err = %v, want it to stop at the page fetch (not reach the screenshot path)", err)
	}
}

// A slug rename or a rejection leaves an image file behind that no page
// references. It is invisible in the rendered site and accumulates in the repo,
// so the tool has to name it.
func TestReportOrphans_PrunesOnlyUnreferencedImages(t *testing.T) {
	dir := t.TempDir()
	orig := imageDir
	imageDir = dir
	defer func() { imageDir = orig }()

	for _, name := range []string{"live.png", "renamed-away.png", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	sc := showcase{Projects: []project{{Slug: "live"}}}

	// Without --prune nothing is touched.
	os.Args = []string{"images"}
	reportOrphans(sc, "")
	if _, err := os.Stat(filepath.Join(dir, "renamed-away.png")); err != nil {
		t.Error("orphan was deleted without --prune")
	}

	os.Args = []string{"images", "--prune"}
	reportOrphans(sc, "")

	if _, err := os.Stat(filepath.Join(dir, "renamed-away.png")); !os.IsNotExist(err) {
		t.Error("orphan survived --prune")
	}
	if _, err := os.Stat(filepath.Join(dir, "live.png")); err != nil {
		t.Error("--prune deleted an image that has a listing")
	}
	// Non-images are none of this tool's business.
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Error("--prune deleted a non-image file")
	}
}

// A --only run sees one slug and must not conclude every other image is an
// orphan.
func TestReportOrphans_SkippedForSingleSlugRuns(t *testing.T) {
	dir := t.TempDir()
	orig := imageDir
	imageDir = dir
	defer func() { imageDir = orig }()
	if err := os.WriteFile(filepath.Join(dir, "other.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	os.Args = []string{"images", "--prune", "--only", "one"}
	reportOrphans(showcase{Projects: []project{{Slug: "one"}}}, "one")

	if _, err := os.Stat(filepath.Join(dir, "other.png")); err != nil {
		t.Error("a --only run pruned images belonging to other listings")
	}
}
