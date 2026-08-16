package main

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Anonymous scaffold telemetry.
//
// Reports that a project was created and whether it passed fmt/test/build. It
// records the installation ID written by install.sh, the CLI version, and
// OS/arch. It deliberately does NOT record the project name, module path,
// target directory, or git remote — see the privacy contract in
// boring-stack-backend's internal/analytics package.
//
// Rules that must survive any edit to this file:
//
//   - Telemetry never changes the exit code. `boringstack new` reports scaffold
//     success, not analytics success.
//   - Telemetry never blocks longer than flushTimeout. A hung endpoint costs
//     half a second on a command that already took seconds.
//   - Telemetry never prints to stdout/stderr on failure. A red error about an
//     analytics box is worse than no data.
const (
	defaultEventsURL = "https://api.boringstack.org/v1/events"
	requestTimeout   = 2 * time.Second
	flushTimeout     = 500 * time.Millisecond
)

type telemetry struct {
	wg sync.WaitGroup
}

// eventsURL allows the test suite (and a self-hosted deployment) to redirect
// events without patching the binary.
func (c *cli) eventsURL() string {
	if v := strings.TrimSpace(c.getenv("BORING_STACK_EVENTS_URL")); v != "" {
		return v
	}
	return defaultEventsURL
}

// installID reads the ID install.sh persisted next to the checkout. Absent that
// file the CLI was installed some other way, and "standalone" keeps those runs
// countable without inventing a per-run identity that would inflate installs.
func (c *cli) installID() string {
	home := strings.TrimSpace(c.getenv("BORING_STACK_HOME"))
	if home == "" {
		h := strings.TrimSpace(c.getenv("HOME"))
		if h == "" {
			return "standalone"
		}
		home = filepath.Join(h, ".boring-stack")
	}
	b, err := os.ReadFile(filepath.Join(home, ".install-id"))
	if err != nil {
		return "standalone"
	}
	id := strings.TrimSpace(string(b))
	if id == "" {
		return "standalone"
	}
	return id
}

// report queues one event. It returns immediately; flush bounds the wait.
func (c *cli) report(event string, props map[string]string) {
	if c.telemetry == nil {
		return
	}
	form := url.Values{}
	form.Set("event", event)
	form.Set("install_id", c.installID())
	form.Set("rev", version)
	form.Set("os", runtime.GOOS)
	form.Set("arch", runtime.GOARCH)
	for k, v := range props {
		if v != "" {
			form.Set(k, v)
		}
	}
	endpoint := c.eventsURL()

	c.telemetry.wg.Add(1)
	go func() {
		defer c.telemetry.wg.Done()
		defer func() { _ = recover() }() // never let analytics panic the CLI

		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := (&http.Client{Timeout: requestTimeout}).Do(req)
		if err != nil {
			return
		}
		_ = resp.Body.Close()
	}()
}

// reportScaffold records that a project was written and whether it then passed
// fmt/test/build, and returns validation's error untouched.
//
// The two events are separate on purpose. project_created says the scaffold
// landed; project_validated says the generated project actually works. Only the
// second is honest enough to report as "projects built" — counting creations
// would include every project that failed to compile.
//
// Threading the error through here rather than reporting at the call site is
// what guarantees the return value is unchanged by instrumentation.
func (c *cli) reportScaffold(mode string, validationErr error) error {
	c.report("project_created", map[string]string{"mode": mode})
	ok := "true"
	if validationErr != nil {
		ok = "false"
	}
	c.report("project_validated", map[string]string{"mode": mode, "ok": ok})
	return validationErr
}

// flush waits briefly for queued events, then gives up.
//
// Waiting at all is a compromise: a bare goroutine dies when main returns, so
// most events would never leave the machine. Waiting without a cap would let a
// hung endpoint stall the command. flushTimeout is the bound.
func (c *cli) flush() {
	if c.telemetry == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		c.telemetry.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(flushTimeout):
	}
}
