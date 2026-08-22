package main

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeShowcaseJSON(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "showcase.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// renderShowcase runs the real template against data, the same way
// buildShowcase does, so these tests exercise the shipped markup.
func renderShowcase(t *testing.T, data *Showcase) string {
	t.Helper()
	tmpl, err := template.ParseFiles(filepath.Join(templateDir, "showcase.html.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// A contributor who has never run `bsb showcase export` must still be able to
// `make build`, so a missing file is not an error.
func TestLoadShowcase_MissingFileIsNotFatal(t *testing.T) {
	_, err := loadShowcase(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("err = %v, want a not-exist error so buildShowcase can skip cleanly", err)
	}
}

func TestLoadShowcase_ParsesExport(t *testing.T) {
	path := writeShowcaseJSON(t, `{
	  "generated_at": "2026-08-17T00:00:00Z",
	  "window_days": 30,
	  "measured_through": "2026-08-17",
	  "count": 99,
	  "projects": [
	    {"slug":"tinycrm","name":"TinyCRM","url":"https://tinycrm.example",
	     "stack":["go","sqlite"],"builder":"@foo","lesson":"Restore drill first.",
	     "monthly_cost":"5.84","monthly_cost_label":"$5.84/mo",
	     "badge_views":42137,"badge_views_label":"42,137","listed_on":"2026-08-01"}
	  ]
	}`)
	s, err := loadShowcase(path)
	if err != nil {
		t.Fatal(err)
	}
	// Count comes from the array, not the file's own (here deliberately wrong) field.
	if s.Count != 1 {
		t.Errorf("Count = %d, want 1", s.Count)
	}
	if s.Projects[0].BadgeViewsLabel != "42,137" || s.Projects[0].MonthlyCostLabel != "$5.84/mo" {
		t.Errorf("labels not parsed: %+v", s.Projects[0])
	}
}

func TestLoadShowcase_RejectsGarbage(t *testing.T) {
	if _, err := loadShowcase(writeShowcaseJSON(t, "not json")); err == nil {
		t.Error("expected a parse error")
	}
}

// Everything on this page is submitter-supplied. If html/template were ever
// swapped for text/template, or a field wrapped in template.HTML, this is the
// test that catches it before it becomes stored XSS on the apex domain.
func TestRenderShowcase_EscapesHostileFields(t *testing.T) {
	out := renderShowcase(t, &Showcase{
		WindowDays:      30,
		MeasuredThrough: "2026-08-17",
		Count:           1,
		Projects: []ShowcaseProject{{
			Slug:            "evil",
			Name:            `<script>alert(1)</script>`,
			URL:             `javascript:alert(1)`,
			RepoURL:         `javascript:alert(2)`,
			Stack:           []string{`<img onerror=alert(3)>`},
			Builder:         `"><script>alert(4)</script>`,
			Lesson:          `</p><script>alert(5)</script><p>`,
			BadgeViewsLabel: "1",
			ListedOn:        "2026-08-01",
		}},
	})

	if strings.Contains(out, "<script>alert(") {
		t.Error("unescaped <script> reached the page")
	}
	for _, frag := range []string{"alert(1)", "alert(2)", "alert(3)", "alert(4)", "alert(5)"} {
		if strings.Contains(out, `"`+frag) || strings.Contains(out, ">"+frag+"<") {
			t.Errorf("%s escaped its context", frag)
		}
	}
	// html/template neutralizes a javascript: href to #ZgotmplZ.
	if strings.Contains(out, `href="javascript:`) || strings.Contains(out, `src="javascript:`) {
		t.Error("a javascript: URL survived into an attribute")
	}
	if !strings.Contains(out, "ZgotmplZ") {
		t.Error("expected html/template to neutralize the javascript: URLs")
	}
}

// The page ships before anyone is listed, so the empty state is the first
// thing real visitors will see.
func TestRenderShowcase_EmptyStateAndBadgeSnippet(t *testing.T) {
	out := renderShowcase(t, &Showcase{WindowDays: 30, MeasuredThrough: "2026-08-17"})

	if !strings.Contains(out, "No listings yet") {
		t.Error("no empty state rendered")
	}
	// The badge section must work with zero listings — it's the whole loop.
	if !strings.Contains(out, "https://api.boringstack.org/badge.svg") {
		t.Error("badge snippet missing")
	}
	if !strings.Contains(out, `data-copy-event="showcase_badge_copy"`) {
		t.Error("badge copy button would report install_copy and corrupt the adoption funnel")
	}
	if !strings.Contains(out, "/v1/showcase/submit") {
		t.Error("submit form missing")
	}
}

func TestRenderShowcase_LinksAndPixel(t *testing.T) {
	out := renderShowcase(t, &Showcase{WindowDays: 30, MeasuredThrough: "2026-08-17"})
	for _, want := range []string{
		`href="showcase.html"`,        // nav marks the current page
		`p=/showcase`,                 // no-JS pixel has its own path
		`<script src="analytics.js">`, // its own element…
		`<script src="copy.js">`,      // …and so is the copy button code
		`name="website"`,              // honeypot
	} {
		if !strings.Contains(out, want) {
			t.Errorf("page is missing %q", want)
		}
	}
}

// The nav is duplicated across two hand-written pages and three templates.
// This is the only thing standing between that and a page quietly losing a
// link during an unrelated edit.
func TestNav_EveryPageLinksShowcase(t *testing.T) {
	pages := map[string]string{
		"../docs/index.html":           `href="showcase.html"`,
		"../docs/manifesto.html":       `href="showcase.html"`,
		"templates/index.html.tmpl":    `href="../showcase.html"`,
		"templates/issue.html.tmpl":    `href="../showcase.html"`,
		"templates/showcase.html.tmpl": `href="showcase.html"`,
	}
	for path, want := range pages {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !strings.Contains(string(b), want) {
			t.Errorf("%s has no showcase nav link (%s)", path, want)
		}
	}
}

// Every class the showcase template uses must actually exist in style.css.
//
// This exists because `p.eyebrow` was written into the template before it was
// written into the stylesheet. An undefined class doesn't fail anything: the
// element silently falls back to default paragraph styling, which spans the
// full 960px body while the hero and subhead are centered in a 720px column.
// The page just looks subtly wrong, and nothing tells you.
func TestShowcase_TemplateClassesAreStyled(t *testing.T) {
	tmpl, err := os.ReadFile(filepath.Join(templateDir, "showcase.html.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	css, err := os.ReadFile("../docs/style.css")
	if err != nil {
		t.Fatal(err)
	}

	classRE := regexp.MustCompile(`class="([^"{}]+)"`)
	seen := map[string]bool{}
	for _, m := range classRE.FindAllStringSubmatch(string(tmpl), -1) {
		for _, c := range strings.Fields(m[1]) {
			seen[c] = true
		}
	}
	if len(seen) == 0 {
		t.Fatal("no classes found in the template — the regex broke")
	}

	for c := range seen {
		if !strings.Contains(string(css), "."+c) {
			t.Errorf("class %q is used in showcase.html.tmpl but never defined in docs/style.css", c)
		}
	}
}

// --- preview images ---

// The image is a filesystem fact, not an export field: `go run ./images` writes
// it and the generator just looks. This keeps `make build` offline.
func TestLoadShowcase_FindsPreviewImageOnDisk(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "showcase.json")
	if err := os.WriteFile(data, []byte(`{"projects":[
	  {"slug":"withimg","name":"With","url":"https://with.example"},
	  {"slug":"noimg","name":"Without","url":"https://www.Without.Example/app"}
	]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Point the generator's image dir at a temp dir holding one image.
	orig := imageDir
	imageDirOverride := filepath.Join(dir, "images")
	if err := os.MkdirAll(imageDirOverride, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imageDirOverride, "withimg.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	setImageDir(imageDirOverride)
	defer setImageDir(orig)

	s, err := loadShowcase(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.Projects[0].Image != "withimg.png" {
		t.Errorf("Image = %q, want withimg.png", s.Projects[0].Image)
	}
	if s.Projects[1].Image != "" {
		t.Errorf("Image = %q for a listing with no file, want empty", s.Projects[1].Image)
	}
	// Host feeds the placeholder tile, so it must be bare and lowercased.
	if s.Projects[1].Host != "without.example" {
		t.Errorf("Host = %q, want without.example", s.Projects[1].Host)
	}
}

// A listing with no preview must render the placeholder tile, not a broken
// <img>. The tile holds the card's aspect ratio so the grid stays even.
func TestRenderShowcase_PlaceholderWhenNoImage(t *testing.T) {
	out := renderShowcase(t, &Showcase{
		WindowDays: 30, MeasuredThrough: "2026-08-17", Count: 2,
		Projects: []ShowcaseProject{
			{Slug: "a", Name: "A", URL: "https://a.example", Image: "a.png", Host: "a.example",
				Stack: []string{"go"}, Lesson: "l", BadgeViewsLabel: "1", ListedOn: "2026-01-01"},
			{Slug: "b", Name: "B", URL: "https://b.example", Image: "", Host: "b.example",
				Stack: []string{"go"}, Lesson: "l", BadgeViewsLabel: "1", ListedOn: "2026-01-01"},
		},
	})
	if !strings.Contains(out, `src="showcase/a.png"`) {
		t.Error("card with an image did not render it")
	}
	if strings.Contains(out, `src="showcase/"`) {
		t.Error("card with no image rendered an empty src — that's a broken image icon")
	}
	if !strings.Contains(out, "showcase-shot-none") || !strings.Contains(out, "b.example") {
		t.Error("card with no image did not render the hostname placeholder")
	}
	// Lazy loading matters: six 1200px images above the fold would be the
	// heaviest page on the site.
	if !strings.Contains(out, `loading="lazy"`) {
		t.Error("preview images are not lazy-loaded")
	}
}

func TestBuilderProfileURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"@boringstack", "https://x.com/boringstack"},
		{"boringstack", "https://x.com/boringstack"},
		{"@a_b123", "https://x.com/a_b123"},
		{"", ""},
		// Allowed in a stored handle, but not a valid X username — better an
		// unlinked handle than a link to a 404.
		{"@has.dot", ""},
		{"@has-dash", ""},
		{"@" + strings.Repeat("a", 16), ""},
	}
	for _, tt := range tests {
		if got := builderProfileURL(tt.in); got != tt.want {
			t.Errorf("builderProfileURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRenderShowcase_LinksBuilderHandle(t *testing.T) {
	out := renderShowcase(t, &Showcase{
		WindowDays: 30, MeasuredThrough: "2026-08-22", Count: 2,
		Projects: []ShowcaseProject{
			{Slug: "a", Name: "A", URL: "https://a.example", Stack: []string{"go"}, Lesson: "l",
				Builder: "@boringstack", BuilderURL: "https://x.com/boringstack",
				BadgeViewsLabel: "1", ListedOn: "2026-01-01"},
			{Slug: "b", Name: "B", URL: "https://b.example", Stack: []string{"go"}, Lesson: "l",
				Builder: "@not.an.x.handle", BuilderURL: "",
				BadgeViewsLabel: "1", ListedOn: "2026-01-01"},
		},
	})
	if !strings.Contains(out, `<a href="https://x.com/boringstack" rel="noopener">@boringstack</a>`) {
		t.Error("a linkable handle did not render as a link")
	}
	if !strings.Contains(out, `<span>@not.an.x.handle</span>`) {
		t.Error("an unlinkable handle should render as plain text, not a broken link")
	}
	if strings.Contains(out, `href="https://x.com/not.an.x.handle"`) {
		t.Error("built an X link for something that cannot be an X username")
	}
}
