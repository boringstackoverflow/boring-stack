// Build the issue HTML pages, archive index, and RSS feed.
//
// Reads:  ../docs/issues/<slug>.md (markdown with --- YAML frontmatter)
// Writes: ../docs/issues/<slug>.html
//         ../docs/issues/index.html
//         ../docs/feed.xml
//
// Frontmatter:
//   ---
//   title: "Restore drill #1"
//   date: 2026-05-09
//   summary: "Optional one-liner."
//   draft: false
//   ---
//
// `draft: true` adds <meta name="newsletter-draft" content="true"> to the
// rendered page, which BSB's auto-send worker reads and treats as "skip".
package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
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
	issuesDir   = "../docs/issues"
	feedPath    = "../docs/feed.xml"
	indexPath   = "../docs/issues/index.html"
	templateDir = "templates"
)

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

	mds, err := filepath.Glob(filepath.Join(issuesDir, "*.md"))
	if err != nil {
		log.Fatalf("glob: %v", err)
	}
	sort.Strings(mds)
	if len(mds) == 0 {
		log.Printf("no markdown issues found in %s — wrote nothing", issuesDir)
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
		PubDate:   date.UTC().Format(time.RFC1123Z),
		HTMLBody:  template.HTML(buf.String()),
	}, nil
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
