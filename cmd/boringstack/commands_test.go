package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployDryRun(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "deploy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "deploy", "deploy.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	app, stdout, _ := testCLI(t, root)

	err := app.runDeploy([]string{
		"--host", "deploy@app.example.com",
		"--healthz", "https://app.example.com/healthz",
		"--dry-run",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := stdout.String()
	for _, want := range []string{"HOST=deploy@app.example.com", "REMOTE=/home/deploy/app", "HEALTHZ_URL=https://app.example.com/healthz", "deploy/deploy.sh"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output %q does not contain %q", got, want)
		}
	}
}

func TestDeployRequiresExplicitTarget(t *testing.T) {
	app, _, _ := testCLI(t, t.TempDir())
	app.getenv = func(string) string { return "" }

	err := app.runDeploy([]string{"--dry-run"})
	if err == nil || !isUsageError(err) || !strings.Contains(err.Error(), "--host") {
		t.Fatalf("error = %v", err)
	}
}

func TestDoctorReportsMissingToolsAndFiles(t *testing.T) {
	app, stdout, _ := testCLI(t, t.TempDir())
	app.lookPath = func(string) (string, error) { return "", errors.New("missing") }

	err := app.runDoctor([]string{"--deploy"})
	if err == nil {
		t.Fatal("doctor unexpectedly succeeded")
	}
	if !strings.Contains(stdout.String(), "not found on PATH") || !strings.Contains(stdout.String(), "go.mod") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestReplaceEnvRemovesOldValues(t *testing.T) {
	got := replaceEnv([]string{"A=1", "HOST=old", "B=2"}, map[string]string{"HOST": "new"})
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, "HOST=old") || !strings.Contains(joined, "HOST=new") {
		t.Fatalf("environment = %v", got)
	}
}

func TestRunExitCodes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := newCLI(t.TempDir(), strings.NewReader(""), &stdout, &stderr)
	if code := app.run([]string{"wat"}); code != 2 {
		t.Fatalf("unknown command exit = %d, want 2", code)
	}
	stdout.Reset()
	stderr.Reset()
	if code := app.run([]string{"version"}); code != 0 {
		t.Fatalf("version exit = %d, want 0", code)
	}
	stdout.Reset()
	if code := app.run([]string{"new", "--help"}); code != 0 {
		t.Fatalf("new --help exit = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "boringstack new") {
		t.Fatalf("new --help output = %q", stdout.String())
	}
}
