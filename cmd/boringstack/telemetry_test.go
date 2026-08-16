package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordingEvents is a stand-in for api.boringstack.org.
type recordingEvents struct {
	mu     sync.Mutex
	events []url.Values
	block  chan struct{} // when non-nil, handlers wait on it (simulates a hang)
}

func (r *recordingEvents) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if r.block != nil {
			<-r.block
		}
		_ = req.ParseForm()
		r.mu.Lock()
		r.events = append(r.events, req.Form)
		r.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
}

func (r *recordingEvents) names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, e.Get("event"))
	}
	return out
}

func (r *recordingEvents) find(name string) (url.Values, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events {
		if e.Get("event") == name {
			return e, true
		}
	}
	return nil, false
}

// telemetryCLI wires a cli with telemetry enabled and pointed at a stub server.
func telemetryCLI(t *testing.T, wd, endpoint, home string) *cli {
	t.Helper()
	app, _, _ := testCLI(t, wd)
	app.telemetry = &telemetry{}
	app.lookPath = func(string) (string, error) { return "", errors.New("missing") }
	app.getenv = func(k string) string {
		switch k {
		case "BORING_STACK_EVENTS_URL":
			return endpoint
		case "BORING_STACK_HOME":
			return home
		}
		return ""
	}
	return app
}

func TestReportScaffoldSendsBothEvents(t *testing.T) {
	rec := &recordingEvents{}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".install-id"), []byte("abc123"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := telemetryCLI(t, t.TempDir(), srv.URL, home)

	if err := app.runNew([]string{"demo-app"}); err != nil {
		t.Fatalf("runNew: %v", err)
	}
	app.flush()

	created, ok := rec.find("project_created")
	if !ok {
		t.Fatalf("no project_created event; got %v", rec.names())
	}
	if created.Get("mode") != "new" {
		t.Errorf("mode = %q, want new", created.Get("mode"))
	}
	// The install ID written by install.sh is reused, so a scaffold ties back
	// to the machine's install instead of inventing a per-run identity.
	if created.Get("install_id") != "abc123" {
		t.Errorf("install_id = %q, want abc123", created.Get("install_id"))
	}

	validated, ok := rec.find("project_validated")
	if !ok {
		t.Fatalf("no project_validated event; got %v", rec.names())
	}
	if validated.Get("ok") != "true" {
		t.Errorf("ok = %q, want true", validated.Get("ok"))
	}
}

// The privacy contract forbids recording anything identifying about the
// project. Nothing sent may contain the project name, module, or a path.
func TestReportScaffoldSendsNoProjectIdentity(t *testing.T) {
	rec := &recordingEvents{}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	root := t.TempDir()
	app := telemetryCLI(t, root, srv.URL, t.TempDir())
	if err := app.runNew([]string{"secret-startup", "--module", "github.com/acme/secret-startup"}); err != nil {
		t.Fatalf("runNew: %v", err)
	}
	app.flush()

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.events) == 0 {
		t.Fatal("no events recorded")
	}
	for _, ev := range rec.events {
		for key, vals := range ev {
			for _, v := range vals {
				for _, banned := range []string{"secret-startup", "acme", root, os.TempDir()} {
					if banned != "" && strings.Contains(v, banned) {
						t.Errorf("event field %s=%q leaks %q", key, v, banned)
					}
				}
			}
		}
	}
}

// Best-effort invariant: an unreachable endpoint must not change the exit code.
func TestScaffoldSucceedsWhenTelemetryUnreachable(t *testing.T) {
	// 203.0.113.0/24 is TEST-NET-3: black-holed, so the connection hangs rather
	// than being refused. Refused would return instantly and prove nothing.
	app := telemetryCLI(t, t.TempDir(), "http://203.0.113.1/v1/events", t.TempDir())

	if err := app.runNew([]string{"demo-app"}); err != nil {
		t.Fatalf("runNew returned %v; scaffolding must succeed regardless of telemetry", err)
	}
	if _, err := os.Stat(filepath.Join(app.cwd, "demo-app", "main.go")); err != nil {
		t.Fatalf("project was not written: %v", err)
	}
}

// ...and must not stall the command for more than flushTimeout.
func TestFlushIsBounded(t *testing.T) {
	rec := &recordingEvents{block: make(chan struct{})}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()
	defer close(rec.block)

	app := telemetryCLI(t, t.TempDir(), srv.URL, t.TempDir())
	app.report("project_created", nil)
	app.report("project_validated", nil)

	start := time.Now()
	app.flush()
	elapsed := time.Since(start)

	if elapsed > flushTimeout+250*time.Millisecond {
		t.Fatalf("flush took %s against a hung endpoint, want <= %s", elapsed, flushTimeout)
	}
}

// A nil telemetry field disables reporting outright, which is what keeps the
// rest of the test suite from making network calls.
func TestTelemetryDisabledByDefault(t *testing.T) {
	rec := &recordingEvents{}
	srv := httptest.NewServer(rec.handler())
	defer srv.Close()

	app, _, _ := testCLI(t, t.TempDir())
	app.lookPath = func(string) (string, error) { return "", errors.New("missing") }
	app.getenv = func(k string) string {
		if k == "BORING_STACK_EVENTS_URL" {
			return srv.URL
		}
		return ""
	}
	if app.telemetry != nil {
		t.Fatal("telemetry should be nil unless main() enables it")
	}
	if err := app.runNew([]string{"demo-app"}); err != nil {
		t.Fatal(err)
	}
	app.flush()

	if n := len(rec.names()); n != 0 {
		t.Errorf("recorded %d events with telemetry disabled, want 0", n)
	}
}

func TestInstallIDFallsBackToStandalone(t *testing.T) {
	app := telemetryCLI(t, t.TempDir(), "http://example.invalid", t.TempDir())
	if got := app.installID(); got != "standalone" {
		t.Errorf("installID() = %q with no .install-id file, want standalone", got)
	}
}

func TestEventsURLDefault(t *testing.T) {
	app, _, _ := testCLI(t, t.TempDir())
	app.getenv = func(string) string { return "" }
	if got := app.eventsURL(); got != defaultEventsURL {
		t.Errorf("eventsURL() = %q, want %q", got, defaultEventsURL)
	}
}
