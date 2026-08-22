// Build the issue HTML pages, archive index, and RSS feed.
//
// Reads:  ../docs/issues/<slug>.md (markdown with --- YAML frontmatter)
// Writes: ../docs/issues/<slug>.html
//
//	../docs/issues/index.html
//	../docs/feed.xml
//
// Frontmatter:
//
//	---
//	title: "Restore drill #1"
//	date: 2026-05-09
//	summary: "Optional one-liner."
//	draft: false
//	---
//
// `draft: true` adds <meta name="newsletter-draft" content="true"> to the
// rendered page, which BSB's auto-send worker reads and treats as "skip".
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

const (
	issuesDir        = "../docs/issues"
	feedPath         = "../docs/feed.xml"
	indexPath        = "../docs/issues/index.html"
	showcaseDataPath = "../docs/showcase.json" // written by `bsb showcase export`
	showcasePath     = "../docs/showcase.html"
	templateDir      = "templates"
)

// imageDir holds preview images written by `go run ./images`. A var, not a
// const, so tests can point it at a temp dir.
var imageDir = "../docs/showcase"

func setImageDir(d string) { imageDir = d }

type Issue struct {
	Slug      string
	Title     string
	Summary   string
	Draft     bool
	Date      time.Time
	DateHuman string
	DateISO   string
	PubDate   string
	Number    int
	HTMLBody  template.HTML
}

func main() {
	log.SetFlags(0)
	buildIssues()
	buildShowcase()
}

// buildIssues renders the issue pages, the archive index, and the RSS feed.
func buildIssues() {
	mds, err := filepath.Glob(filepath.Join(issuesDir, "*.md"))
	if err != nil {
		log.Fatalf("glob: %v", err)
	}
	sort.Strings(mds)
	if len(mds) == 0 {
		log.Printf("no markdown issues found in %s — skipping issues", issuesDir)
		return
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Footnote, extension.Typographer),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	var issues []*Issue
	for _, path := range mds {
		iss, err := parseIssue(path, md)
		if err != nil {
			log.Fatalf("%s: %v", path, err)
		}
		issues = append(issues, iss)
	}

	// Stabilize pubDates: preserve each issue's pubDate from the existing
	// feed.xml so re-builds don't churn the feed. Issues new to this build
	// get the current UTC time as their first pubDate. This is the right
	// fix for the "auto-send dies 24h past UTC midnight" bug: a fresh issue
	// always enters the feed with pubDate=now, not pubDate=midnight-of-date.
	prior := loadPriorPubDates(feedPath)
	now := time.Now().UTC().Format(time.RFC1123Z)
	for _, iss := range issues {
		if pd, ok := prior[iss.Slug]; ok {
			iss.PubDate = pd
		} else {
			iss.PubDate = now
		}
	}

	// Sort newest first; assign issue numbers in chronological order so
	// older issues keep their stable number when new ones are added.
	sort.Slice(issues, func(i, j int) bool { return issues[i].Date.Before(issues[j].Date) })
	for i, iss := range issues {
		iss.Number = i + 1
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Date.After(issues[j].Date) })

	tIssue := template.Must(template.ParseFiles(filepath.Join(templateDir, "issue.html.tmpl")))
	tIndex := template.Must(template.ParseFiles(filepath.Join(templateDir, "index.html.tmpl")))
	// Use text/template for XML so the <?xml …?> declaration and `+` in
	// timestamps don't get HTML-escaped.
	tFeed := texttemplate.Must(texttemplate.ParseFiles(filepath.Join(templateDir, "feed.xml.tmpl")))

	for _, iss := range issues {
		out := filepath.Join(issuesDir, iss.Slug+".html")
		f, err := os.Create(out)
		if err != nil {
			log.Fatalf("create %s: %v", out, err)
		}
		if err := tIssue.Execute(f, iss); err != nil {
			log.Fatalf("render %s: %v", out, err)
		}
		f.Close()
		log.Printf("wrote %s", out)
	}

	idxF, err := os.Create(indexPath)
	if err != nil {
		log.Fatalf("create %s: %v", indexPath, err)
	}
	if err := tIndex.Execute(idxF, struct{ Issues []*Issue }{issues}); err != nil {
		log.Fatalf("render index: %v", err)
	}
	idxF.Close()
	log.Printf("wrote %s", indexPath)

	feedF, err := os.Create(feedPath)
	if err != nil {
		log.Fatalf("create %s: %v", feedPath, err)
	}
	feedData := struct {
		BuildDate string
		Issues    []*Issue
	}{
		BuildDate: time.Now().UTC().Format(time.RFC1123Z),
		Issues:    issues,
	}
	if err := tFeed.Execute(feedF, feedData); err != nil {
		log.Fatalf("render feed: %v", err)
	}
	feedF.Close()
	log.Printf("wrote %s", feedPath)
	log.Printf("done — %d issue(s)", len(issues))
}

// --- showcase ---

// Showcase mirrors the JSON that `bsb showcase export` writes. Only the fields
// the page renders are here: the exporter is the allowlist that decides what
// becomes public, and this struct is deliberately not a superset of it.
type Showcase struct {
	GeneratedAt     string            `json:"generated_at"`
	WindowDays      int               `json:"window_days"`
	MeasuredThrough string            `json:"measured_through"`
	Count           int               `json:"count"`
	Projects        []ShowcaseProject `json:"projects"`
}

type ShowcaseProject struct {
	Slug             string   `json:"slug"`
	Name             string   `json:"name"`
	URL              string   `json:"url"`
	RepoURL          string   `json:"repo_url"`
	Stack            []string `json:"stack"`
	Builder          string   `json:"builder"`
	Lesson           string   `json:"lesson"`
	MonthlyCost      string   `json:"monthly_cost"`
	MonthlyCostLabel string   `json:"monthly_cost_label"`
	BadgeViews       int64    `json:"badge_views"`
	BadgeViewsLabel  string   `json:"badge_views_label"`
	ListedOn         string   `json:"listed_on"`

	// Filled in by loadShowcase, not by the exporter.
	//
	// Image is the filename under docs/showcase/ if `go run ./images` has
	// fetched one — a purely local filesystem check, so `make build` stays
	// offline and deterministic. Empty means the card renders a placeholder
	// instead, which is a normal state, not an error.
	Image string `json:"-"`
	// Host is the bare hostname, shown in that placeholder.
	Host string `json:"-"`
	// BuilderURL is the X profile for Builder, or "" to render it as plain
	// text. See builderProfileURL for why it is not always set.
	BuilderURL string `json:"-"`
}

// loadShowcase reads the file `bsb showcase export` produces.
//
// Tolerant by design, the same posture as loadPriorPubDates: a missing or
// unreadable file means "no showcase data here", not a broken build. A
// contributor who has never run the exporter must still be able to `make
// build` without touching the backend.
func loadShowcase(path string) (*Showcase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Showcase
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if s.WindowDays == 0 {
		s.WindowDays = 30
	}
	// Trust the array, not the exporter's own count field.
	s.Count = len(s.Projects)

	for i := range s.Projects {
		s.Projects[i].Host = hostOf(s.Projects[i].URL)
		s.Projects[i].Image = findShowcaseImage(s.Projects[i].Slug)
		s.Projects[i].BuilderURL = builderProfileURL(s.Projects[i].Builder)
	}
	return &s, nil
}

// findShowcaseImage returns the filename of a fetched preview image, or "".
// Extensions are tried in the order `go run ./images` prefers to write them.
func findShowcaseImage(slug string) string {
	for _, ext := range []string{".jpg", ".png", ".webp", ".gif"} {
		if _, err := os.Stat(filepath.Join(imageDir, slug+ext)); err == nil {
			return slug + ext
		}
	}
	return ""
}

// xUsername is stricter than the backend's handle sanitizer, which allows
// "." and "-" so a handle stays readable for any platform. X usernames are
// letters, digits and underscore only, up to 15 characters.
var xUsername = regexp.MustCompile(`^[A-Za-z0-9_]{1,15}$`)

// builderProfileURL turns "@name" into an X profile link, or returns "" so the
// handle renders as plain text.
//
// The showcase stores a bare handle with no platform attached, so linking it
// anywhere is an assumption. X is the one the rest of this site makes — the
// footer has linked @boringstack there since the beginning — and a handle that
// cannot be an X username is left unlinked rather than pointed at a 404.
//
// The handle is already stripped to [A-Za-z0-9_.-] by the backend before it is
// ever exported; this narrowing is belt-and-braces, and means nothing that
// reaches an href was chosen by a submitter.
func builderProfileURL(builder string) string {
	name := strings.TrimPrefix(builder, "@")
	if !xUsername.MatchString(name) {
		return ""
	}
	return "https://x.com/" + name
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// buildShowcase renders docs/showcase.html from docs/showcase.json.
//
// html/template, never text/template: every field below is submitter-supplied.
// Escaping in the URL attribute context is what turns a javascript: href into
// #ZgotmplZ instead of stored XSS on the apex domain.
func buildShowcase() {
	data, err := loadShowcase(showcaseDataPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("no %s — skipping %s (run `bsb showcase export`)", showcaseDataPath, showcasePath)
			return
		}
		log.Fatalf("showcase: %v", err)
	}

	t := template.Must(template.ParseFiles(filepath.Join(templateDir, "showcase.html.tmpl")))
	f, err := os.Create(showcasePath)
	if err != nil {
		log.Fatalf("create %s: %v", showcasePath, err)
	}
	defer f.Close()
	if err := t.Execute(f, data); err != nil {
		log.Fatalf("render showcase: %v", err)
	}
	log.Printf("wrote %s — %d listing(s)", showcasePath, data.Count)
}

func parseIssue(path string, md goldmark.Markdown) (*Issue, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	frontmatter, body, err := splitFrontmatter(raw)
	if err != nil {
		return nil, fmt.Errorf("frontmatter: %w", err)
	}
	meta, err := parseFrontmatter(frontmatter)
	if err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	title, ok := meta["title"]
	if !ok || title == "" {
		return nil, fmt.Errorf("missing title")
	}
	dateStr, ok := meta["date"]
	if !ok {
		return nil, fmt.Errorf("missing date")
	}
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("date %q: %w", dateStr, err)
	}

	slug := strings.TrimSuffix(filepath.Base(path), ".md")

	var buf bytes.Buffer
	if err := md.Convert(body, &buf); err != nil {
		return nil, fmt.Errorf("md: %w", err)
	}

	return &Issue{
		Slug:      slug,
		Title:     title,
		Summary:   meta["summary"],
		Draft:     meta["draft"] == "true",
		Date:      date,
		DateHuman: date.Format("January 2, 2006"),
		DateISO:   date.Format("2006-01-02"),
		// PubDate intentionally left empty — assigned in main() from
		// the existing feed.xml (preserves history) or time.Now() (new).
		HTMLBody: template.HTML(buf.String()),
	}, nil
}

// loadPriorPubDates reads the existing feed.xml (if present) and returns a
// map of slug → pubDate string for every <item> it can parse. Returns nil
// (which is fine — a nil map looks empty to callers) if the feed is missing,
// unreadable, or has no recognizable items. Tolerant by design: any parsing
// hiccup just means "this slug is new", which is the safe fallback.
func loadPriorPubDates(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	re := regexp.MustCompile(
		`<guid[^>]*>https://boringstack\.org/issues/([^<]+)\.html</guid>\s*<pubDate>([^<]+)</pubDate>`,
	)
	out := make(map[string]string)
	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		out[m[1]] = m[2]
	}
	return out
}

// splitFrontmatter extracts the YAML-ish block delimited by '---' lines at
// the top of the file. Returns (frontmatter, body, error).
func splitFrontmatter(raw []byte) ([]byte, []byte, error) {
	const delim = "---\n"
	if !bytes.HasPrefix(raw, []byte(delim)) {
		return nil, nil, fmt.Errorf("file must start with '---' frontmatter delimiter")
	}
	rest := raw[len(delim):]
	end := bytes.Index(rest, []byte("\n"+delim))
	if end < 0 {
		// Try without trailing newline (last line)
		end = bytes.Index(rest, []byte("\n---"))
		if end < 0 {
			return nil, nil, fmt.Errorf("missing closing '---' delimiter")
		}
	}
	frontmatter := rest[:end]
	body := rest[end:]
	if i := bytes.Index(body, []byte(delim)); i >= 0 {
		body = body[i+len(delim):]
	}
	return frontmatter, body, nil
}

// parseFrontmatter handles a deliberately tiny subset of YAML: `key: value`
// per line, optional double-quoted values, no nesting. That's all we need.
func parseFrontmatter(b []byte) (map[string]string, error) {
	out := make(map[string]string)
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			return nil, fmt.Errorf("malformed line: %q", line)
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])
		if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
			val = val[1 : len(val)-1]
		}
		out[key] = val
	}
	return out, nil
}
