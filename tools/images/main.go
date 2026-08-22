// Fetch a preview image for each showcase listing.
//
//	cd tools && go run ./images [--force] [--only <slug>]
//
// Reads:  ../docs/showcase.json (written by `bsb showcase export`)
// Writes: ../docs/showcase/<slug>.{jpg,png,webp}
//
// For each listing it tries, in order:
//
//  1. the site's own og:image (or twitter:image) meta tag, downloaded
//  2. a headless Chrome screenshot of the homepage at 1200x630
//  3. nothing — the card renders a CSS placeholder, which is fine
//
// # Why the images are downloaded and committed, not hotlinked
//
// Hotlinking would be less code and zero bytes in the repo. It would also mean
// every visitor to boringstack.org/showcase silently sends their IP address to
// every listed project's server — on a page whose own copy, two sections down,
// says we record no IP addresses and no visitor identity. Self-hosting keeps
// that promise true for the whole page, and has the side benefit that a listed
// site cannot swap its og:image for something else after review.
//
// # Why this is a separate command and not part of `go run .`
//
// tools/build.go does no network I/O and is deterministic: `make build` works
// offline, in CI, and for a contributor who has never touched the backend.
// Fetching images is slow, needs the network, and needs Chrome. Keeping it a
// separate opt-in step means a failed fetch can never break a site build.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	showcaseData = "../docs/showcase.json"

	maxPageBytes  = 1 << 20 // 1 MiB of HTML is plenty to find a meta tag
	maxImageBytes = 2 << 20 // 2 MiB — anything larger is not a preview image
	fetchTimeout  = 20 * time.Second
	shotTimeout   = 60 * time.Second

	// The Open Graph standard size. Screenshots match it so cards built from a
	// screenshot and cards built from an og:image crop identically.
	shotWidth  = 1200
	shotHeight = 630
)

// imageDir is a var, not a const, so tests can point it at a temp dir.
var imageDir = "../docs/showcase"

type showcase struct {
	Projects []project `json:"projects"`
}

type project struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

var (
	ogRE = regexp.MustCompile(
		`(?is)<meta[^>]+(?:property|name)\s*=\s*["'](?:og:image(?::url)?|twitter:image(?::src)?)["'][^>]*>`)
	contentRE = regexp.MustCompile(`(?is)content\s*=\s*["']([^"']+)["']`)

	// Extensions we are willing to write, keyed by sniffed content type. A
	// server that says "image/gif" gets .gif; anything unrecognized is skipped
	// rather than guessed at.
	extByType = map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
		"image/gif":  ".gif",
	}
)

func main() {
	log.SetFlags(0)
	force := hasFlag("--force")
	only := flagValue("--only")

	raw, err := os.ReadFile(showcaseData)
	if err != nil {
		log.Fatalf("read %s: %v (run `bsb showcase export` first)", showcaseData, err)
	}
	var sc showcase
	if err := json.Unmarshal(raw, &sc); err != nil {
		log.Fatalf("parse %s: %v", showcaseData, err)
	}
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", imageDir, err)
	}

	chrome := findChrome()
	if chrome == "" {
		log.Printf("note: no Chrome found — screenshot fallback disabled, og:image only")
	}

	var got, skipped, failed int
	for _, p := range sc.Projects {
		if only != "" && p.Slug != only {
			continue
		}
		if existing := existingImage(p.Slug); existing != "" && !force {
			log.Printf("%-14s skip     %s (--force to refetch)", p.Slug, filepath.Base(existing))
			skipped++
			continue
		}
		switch path, how, err := fetchImage(p, chrome); {
		case err != nil:
			log.Printf("%-14s FAILED   %v", p.Slug, err)
			failed++
		default:
			info, _ := os.Stat(path)
			log.Printf("%-14s %-8s %s (%d KB)", p.Slug, how, filepath.Base(path), info.Size()/1024)
			got++
		}
	}
	reportOrphans(sc, only)
	log.Printf("\n%d fetched, %d skipped, %d failed", got, skipped, failed)
	if failed > 0 {
		log.Printf("failed listings render a CSS placeholder — the page is still correct")
	}
}

// reportOrphans finds image files with no matching listing.
//
// Slugs change (a rename re-mints them) and listings get rejected, and the
// image left behind is invisible in the rendered page — it just sits in the
// repo forever. `make showcase` passes --prune so the committed images always
// match the export; a bare run only reports, because deleting files nobody
// asked about is not a build tool's job.
func reportOrphans(sc showcase, only string) {
	if only != "" {
		return // a single-slug run has no view of the whole set
	}
	live := make(map[string]bool, len(sc.Projects))
	for _, p := range sc.Projects {
		live[p.Slug] = true
	}
	entries, err := os.ReadDir(imageDir)
	if err != nil {
		return
	}
	prune := hasFlag("--prune")
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)
		if !isImageExt(ext) || live[strings.TrimSuffix(name, ext)] {
			continue
		}
		if prune {
			if err := os.Remove(filepath.Join(imageDir, name)); err == nil {
				log.Printf("%-14s pruned   %s (no listing)", strings.TrimSuffix(name, ext), name)
			}
			continue
		}
		log.Printf("%-14s ORPHAN   %s has no listing (--prune to remove)", strings.TrimSuffix(name, ext), name)
	}
}

func isImageExt(ext string) bool {
	for _, e := range []string{".jpg", ".png", ".webp", ".gif"} {
		if strings.EqualFold(ext, e) {
			return true
		}
	}
	return false
}

// fetchImage returns the written path and which strategy produced it.
//
// The page is fetched FIRST, and a failure there is fatal for this listing —
// we never fall through to a screenshot. Chrome will happily render its own
// "This site can't be reached" page and write that to disk, which is how a
// dead domain ends up as a card image that looks like a broken product. A
// reachable page is the precondition for a screenshot being worth anything.
func fetchImage(p project, chrome string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()

	page, err := get2(ctx, p.URL, maxPageBytes)
	if err != nil {
		return "", "", fmt.Errorf("site unreachable: %v", err)
	}

	if src, err := findOGImage(page, p.URL); err != nil {
		log.Printf("%-14s og:image unusable (%v), trying screenshot", p.Slug, err)
	} else if src != "" {
		if path, err := download(ctx, p.Slug, src); err == nil {
			return path, "og:image", nil
		} else {
			log.Printf("%-14s og:image download failed (%v), trying screenshot", p.Slug, err)
		}
	}

	if chrome == "" {
		return "", "", fmt.Errorf("no og:image and no Chrome for a screenshot")
	}
	path, err := screenshot(chrome, p.Slug, p.URL)
	if err != nil {
		return "", "", err
	}
	return path, "screenshot", nil
}

// get2 is get() without the content type, for callers that only want bytes.
func get2(ctx context.Context, raw string, maxBytes int64) ([]byte, error) {
	body, _, err := get(ctx, raw, maxBytes)
	return body, err
}

// findOGImage returns an absolute og:image URL from already-fetched page HTML.
func findOGImage(body []byte, pageURL string) (string, error) {
	tag := ogRE.Find(body)
	if tag == nil {
		return "", nil
	}
	m := contentRE.FindSubmatch(tag)
	if m == nil {
		return "", nil
	}
	src := strings.TrimSpace(html(string(m[1])))
	if src == "" {
		return "", nil
	}
	// og:image is allowed to be relative; resolve it against the page.
	base, err := url.Parse(pageURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(src)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}

func download(ctx context.Context, slug, src string) (string, error) {
	body, ctype, err := get(ctx, src, maxImageBytes)
	if err != nil {
		return "", err
	}
	ext, err := imageExt(body, ctype)
	if err != nil {
		return "", err
	}
	return write(slug, ext, body)
}

// imageExt decides what extension a downloaded body earns, or refuses it.
//
// The sniffed bytes beat the declared Content-Type: a server that says
// image/png while serving an HTML error page must not put a .png of a 404 into
// the repo, and that is a thing servers do.
func imageExt(body []byte, declared string) (string, error) {
	sniffed := http.DetectContentType(body)
	ext, ok := extByType[strings.ToLower(strings.TrimSpace(strings.Split(sniffed, ";")[0]))]
	if !ok {
		return "", fmt.Errorf("not an image (declared %q, sniffed %q)", declared, sniffed)
	}
	if len(body) < 1024 {
		return "", fmt.Errorf("suspiciously small image (%d bytes)", len(body))
	}
	return ext, nil
}

func screenshot(chrome, slug, pageURL string) (string, error) {
	tmp, err := os.MkdirTemp("", "showcase-shot")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	out := filepath.Join(tmp, "shot.png")

	ctx, cancel := context.WithTimeout(context.Background(), shotTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, chrome,
		"--headless=new", "--disable-gpu", "--hide-scrollbars", "--no-first-run",
		"--no-default-browser-check", "--disable-extensions",
		fmt.Sprintf("--window-size=%d,%d", shotWidth, shotHeight),
		"--screenshot="+out,
		"--virtual-time-budget=8000",
		pageURL,
	)
	// Chrome is noisy on stderr even when it succeeds; the written file is the
	// only signal that matters.
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	_ = cmd.Run()

	body, err := os.ReadFile(out)
	if err != nil {
		return "", fmt.Errorf("screenshot produced no file (site unreachable?)")
	}
	if len(body) < 1024 {
		return "", fmt.Errorf("screenshot came out empty")
	}
	return write(slug, ".png", body)
}

// write replaces any previously fetched image for this slug so a refetch that
// changes format doesn't leave two files fighting over the same card.
func write(slug, ext string, body []byte) (string, error) {
	if old := existingImage(slug); old != "" {
		_ = os.Remove(old)
	}
	path := filepath.Join(imageDir, slug+ext)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func existingImage(slug string) string {
	for _, ext := range []string{".jpg", ".png", ".webp", ".gif"} {
		p := filepath.Join(imageDir, slug+ext)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// get performs an SSRF-guarded HTTPS fetch with a hard size cap.
//
// Every URL here comes from a submitter, so the dialer refuses loopback,
// link-local, and private destinations. This runs on a maintainer's laptop,
// which is usually on a network with interesting things on 192.168.x.x.
func get(ctx context.Context, raw string, maxBytes int64) ([]byte, string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, "", err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, "", fmt.Errorf("refusing scheme %q", u.Scheme)
	}
	client := &http.Client{
		Timeout: fetchTimeout,
		Transport: &http.Transport{
			DialContext: guardedDial,
		},
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "boringstack-showcase-images/0.1 (+https://boringstack.org/showcase.html)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("Content-Type"), nil
}

func guardedDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if !ip.IP.IsGlobalUnicast() || ip.IP.IsPrivate() || ip.IP.IsLoopback() || ip.IP.IsLinkLocalUnicast() {
			return nil, fmt.Errorf("refusing to fetch a non-public address (%s)", ip.IP)
		}
	}
	return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, addr)
}

func findChrome() string {
	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	}
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if p, err := exec.LookPath(name); err == nil {
			candidates = append(candidates, p)
		}
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

// html decodes the handful of entities that actually turn up in a meta content
// attribute. Not a parser, and doesn't need to be.
func html(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&#38;", "&", "&quot;", `"`, "&#39;", "'", "&apos;", "'")
	return r.Replace(s)
}

func hasFlag(name string) bool {
	for _, a := range os.Args[1:] {
		if a == name {
			return true
		}
	}
	return false
}

func flagValue(name string) string {
	args := os.Args[1:]
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(a, name+"=") {
			return strings.TrimPrefix(a, name+"=")
		}
	}
	return ""
}
