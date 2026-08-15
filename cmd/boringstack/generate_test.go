package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func testCLI(t *testing.T, wd string) (*cli, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	app := newCLI(wd, strings.NewReader(""), &stdout, &stderr)
	app.now = func() time.Time { return time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC) }
	return app, &stdout, &stderr
}

func TestNewCreatesExpectedProject(t *testing.T) {
	root := t.TempDir()
	app, stdout, _ := testCLI(t, root)
	app.lookPath = func(string) (string, error) { return "", errors.New("missing") }

	if err := app.runNew([]string{"demo-app", "--module", "example.com/demo-app"}); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(root, "demo-app")
	got := projectFiles(t, target)
	want := []string{
		".gitignore",
		"AGENTS.md",
		"Makefile",
		"README.md",
		"STACK.md",
		"data/.gitkeep",
		"deploy/Caddyfile",
		"deploy/README.md",
		"deploy/app.service",
		"deploy/deploy.sh",
		"deploy/litestream.service",
		"deploy/litestream.yml",
		"go.mod",
		"internal/.gitkeep",
		"main.go",
		"main_test.go",
		"migrations/.gitkeep",
		"static/app.css",
		"templates/index.html.tmpl",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("generated files:\n%v\nwant:\n%v", got, want)
	}

	goMod := readTestFile(t, filepath.Join(target, "go.mod"))
	if !strings.Contains(goMod, "module example.com/demo-app") {
		t.Fatalf("go.mod does not contain module: %s", goMod)
	}
	stack := readTestFile(t, filepath.Join(target, "STACK.md"))
	if !strings.Contains(stack, "Decided 2026-08-13") {
		t.Fatalf("STACK.md does not contain injected date: %s", stack)
	}
	info, err := os.Stat(filepath.Join(target, "deploy", "deploy.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("deploy.sh mode = %o, want 755", info.Mode().Perm())
	}
	if !strings.Contains(stdout.String(), "Created ") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestGeneratedProjectTestsAndBuilds(t *testing.T) {
	target := t.TempDir()
	files, err := renderProject(projectConfig{Name: "demo-app", Module: "example.com/demo-app", Date: "2026-08-13"})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeProject(target, files); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"test", "./..."}, {"vet", "./..."}, {"build", "-o", filepath.Join(t.TempDir(), "app"), "."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = target
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, output)
		}
	}
}

func TestGeneratedProjectServesHomeAndHealthz(t *testing.T) {
	target := t.TempDir()
	files, err := renderProject(projectConfig{Name: "runtime-demo", Module: "example.com/runtime-demo", Date: "2026-08-13"})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeProject(target, files); err != nil {
		t.Fatal(err)
	}

	binary := filepath.Join(t.TempDir(), "app")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = target
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build generated app: %v\n%s", err, output)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var logs bytes.Buffer
	server := exec.CommandContext(ctx, binary)
	server.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))
	server.Stdout = &logs
	server.Stderr = &logs
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		_ = server.Wait()
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(8 * time.Second)
	for {
		response, requestErr := client.Get(baseURL + "/healthz")
		if requestErr == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("generated app did not become healthy: %v\n%s", requestErr, logs.String())
		}
		time.Sleep(50 * time.Millisecond)
	}

	response, err := client.Get(baseURL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("home status = %d, want %d", response.StatusCode, http.StatusOK)
	}
}

func TestNewDryRunDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	app, stdout, _ := testCLI(t, root)

	if err := app.runNew([]string{"demo", "--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "demo")); !os.IsNotExist(err) {
		t.Fatalf("dry run created target: %v", err)
	}
	if !strings.Contains(stdout.String(), "Would create") || !strings.Contains(stdout.String(), "main.go") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestNewRefusesExistingTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	app, _, _ := testCLI(t, root)

	err := app.runNew([]string{"demo"})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v", err)
	}
}

func TestInitAllowsOnlyGitMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	app, _, _ := testCLI(t, root)
	app.lookPath = func(string) (string, error) { return "", errors.New("missing") }

	if err := app.runInit([]string{"--name", "demo"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
}

func TestInitRefusesUserFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, _, _ := testCLI(t, root)

	err := app.runInit([]string{"--name", "demo"})
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("error = %v", err)
	}
	if got := readTestFile(t, filepath.Join(root, "notes.txt")); got != "keep" {
		t.Fatalf("existing file changed to %q", got)
	}
}

func TestProjectNameValidation(t *testing.T) {
	for _, name := range []string{"", "../demo", "two words", "/tmp/demo", "-demo"} {
		t.Run(name, func(t *testing.T) {
			if err := validateProjectName(name); err == nil {
				t.Fatalf("validateProjectName(%q) succeeded", name)
			}
		})
	}
	for _, name := range []string{"demo", "demo-app", "Demo_2"} {
		if err := validateProjectName(name); err != nil {
			t.Fatalf("validateProjectName(%q): %v", name, err)
		}
	}
}

func projectFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
