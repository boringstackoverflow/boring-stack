package boringstack

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerFreshAndIdempotent(t *testing.T) {
	for _, tool := range []string{"bash", "git", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available", tool)
		}
	}

	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	fixture := t.TempDir()
	source := filepath.Join(fixture, "source")
	if err := copyRepository(root, source); err != nil {
		t.Fatal(err)
	}
	runTestCommand(t, source, "git", "init", "-q")
	runTestCommand(t, source, "git", "add", "-A")
	commit := exec.Command("git", "-c", "user.name=Installer Test", "-c", "user.email=test@example.com", "commit", "-qm", "fixture")
	commit.Dir = source
	if output, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("commit fixture: %v\n%s", err, output)
	}

	home := filepath.Join(fixture, "home")
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(withoutEnv(os.Environ(), "HOME", "BORING_STACK_REPO", "BORING_STACK_BIN_DIR"),
		"HOME="+home,
		"BORING_STACK_REPO="+source,
		"BORING_STACK_BIN_DIR="+bin,
	)
	installer := filepath.Join(source, "docs", "install.sh")

	for attempt := 1; attempt <= 2; attempt++ {
		cmd := exec.Command("bash", installer)
		cmd.Env = env
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("installer attempt %d: %v\n%s", attempt, err, output)
		}
	}

	cli := filepath.Join(bin, "boringstack")
	cmd := exec.Command(cli, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("installed CLI: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) == "" || strings.TrimSpace(string(output)) == "dev" {
		t.Fatalf("installed version = %q, want Git revision", output)
	}
}

func copyRepository(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == ".git" || strings.HasPrefix(relative, ".git"+string(filepath.Separator)) || relative == "boringstack" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

func runTestCommand(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, output)
	}
}

func withoutEnv(environment []string, keys ...string) []string {
	omit := make(map[string]bool, len(keys))
	for _, key := range keys {
		omit[key] = true
	}
	result := make([]string, 0, len(environment))
	for _, item := range environment {
		key, _, found := strings.Cut(item, "=")
		if !found || !omit[key] {
			result = append(result, item)
		}
	}
	return result
}
