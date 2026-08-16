package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	boringstack "github.com/boringstackoverflow/boring-stack"
)

var projectNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

type projectConfig struct {
	Name   string
	Module string
	Date   string
}

type outputFile struct {
	Path string
	Mode fs.FileMode
	Data []byte
}

func (c *cli) runNew(args []string) error {
	name, module, dryRun, err := parseNewArgs(args)
	if err != nil {
		return err
	}
	if err := validateProjectName(name); err != nil {
		return err
	}
	if module == "" {
		module = name
	}
	if err := validateModule(module); err != nil {
		return err
	}

	target := c.projectPath(name)
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("target %s already exists; use 'boringstack init' inside an empty directory", target)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect target: %w", err)
	}

	config := projectConfig{Name: name, Module: module, Date: c.now().Format("2006-01-02")}
	files, err := renderProject(config)
	if err != nil {
		return err
	}
	if dryRun {
		printFilePlan(c.stdout, target, files)
		return nil
	}

	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	temp, err := os.MkdirTemp(parent, "."+filepath.Base(target)+".boringstack-")
	if err != nil {
		return fmt.Errorf("create temporary project: %w", err)
	}
	keepTemp := false
	defer func() {
		if !keepTemp {
			_ = os.RemoveAll(temp)
		}
	}()

	if err := writeProject(temp, files); err != nil {
		return err
	}
	if err := os.Rename(temp, target); err != nil {
		return fmt.Errorf("publish project: %w", err)
	}
	keepTemp = true

	printCreated(c.stdout, name, files)
	return c.reportScaffold("new", c.validateGeneratedProject(target))
}

func (c *cli) runInit(args []string) error {
	name, module, dryRun, err := parseInitArgs(args)
	if err != nil {
		return err
	}
	if name == "" {
		name = filepath.Base(c.cwd)
	}
	if err := validateProjectName(name); err != nil {
		return err
	}
	if module == "" {
		module = name
	}
	if err := validateModule(module); err != nil {
		return err
	}
	if err := ensureInitializable(c.cwd); err != nil {
		return err
	}

	config := projectConfig{Name: name, Module: module, Date: c.now().Format("2006-01-02")}
	files, err := renderProject(config)
	if err != nil {
		return err
	}
	if dryRun {
		printFilePlan(c.stdout, c.cwd, files)
		return nil
	}
	if err := writeProject(c.cwd, files); err != nil {
		return err
	}
	printCreated(c.stdout, ".", files)
	return c.reportScaffold("init", c.validateGeneratedProject(c.cwd))
}

func parseNewArgs(args []string) (name, module string, dryRun bool, err error) {
	positionals := make([]string, 0, 1)
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--dry-run":
			dryRun = true
		case arg == "--module":
			if i+1 >= len(args) {
				return "", "", false, usagef("--module requires a value")
			}
			i++
			module = args[i]
		case strings.HasPrefix(arg, "--module="):
			module = strings.TrimPrefix(arg, "--module=")
		case arg == "--help" || arg == "-h":
			return "", "", false, usagef("usage: boringstack new <name> [--module <path>] [--dry-run]")
		case strings.HasPrefix(arg, "-"):
			return "", "", false, usagef("unknown flag %q", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 1 {
		return "", "", false, usagef("new requires exactly one project name")
	}
	return positionals[0], module, dryRun, nil
}

func parseInitArgs(args []string) (name, module string, dryRun bool, err error) {
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--dry-run":
			dryRun = true
		case arg == "--name" || arg == "--module":
			if i+1 >= len(args) {
				return "", "", false, usagef("%s requires a value", arg)
			}
			i++
			if arg == "--name" {
				name = args[i]
			} else {
				module = args[i]
			}
		case strings.HasPrefix(arg, "--name="):
			name = strings.TrimPrefix(arg, "--name=")
		case strings.HasPrefix(arg, "--module="):
			module = strings.TrimPrefix(arg, "--module=")
		case arg == "--help" || arg == "-h":
			return "", "", false, usagef("usage: boringstack init [--name <name>] [--module <path>] [--dry-run]")
		default:
			return "", "", false, usagef("unknown argument %q", arg)
		}
	}
	return name, module, dryRun, nil
}

func validateProjectName(name string) error {
	if name == "" {
		return usagef("project name must not be empty")
	}
	if !projectNamePattern.MatchString(name) || name == "." || name == ".." {
		return usagef("invalid project name %q; use letters, numbers, '-' or '_', starting with a letter", name)
	}
	return nil
}

func validateModule(module string) error {
	if module == "" || strings.ContainsAny(module, " \\") || strings.HasPrefix(module, "/") || strings.Contains(module, "..") {
		return usagef("invalid Go module path %q", module)
	}
	return nil
}

func ensureInitializable(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read current directory: %w", err)
	}
	for _, entry := range entries {
		if entry.Name() == ".git" {
			continue
		}
		return fmt.Errorf("current directory is not empty (%s exists); refusing to overwrite files", entry.Name())
	}
	return nil
}

func renderProject(config projectConfig) ([]outputFile, error) {
	var files []outputFile
	if err := fs.WalkDir(boringstack.Assets, "scaffold/base", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := boringstack.Assets.ReadFile(path)
		if err != nil {
			return err
		}
		outPath := strings.TrimPrefix(path, "scaffold/base/")
		outPath = strings.TrimSuffix(outPath, ".tmpl")
		files = append(files, outputFile{Path: outPath, Mode: 0o644, Data: renderTokens(data, config)})
		return nil
	}); err != nil {
		return nil, fmt.Errorf("read base scaffold: %w", err)
	}

	canonical := map[string]string{
		"templates/index.html.tmpl":    "templates/index.html.tmpl",
		"templates/static/app.css":     "static/app.css",
		"templates/deploy.sh":          "deploy/deploy.sh",
		"templates/Caddyfile":          "deploy/Caddyfile",
		"templates/app.service":        "deploy/app.service",
		"templates/litestream.service": "deploy/litestream.service",
		"templates/litestream.yml":     "deploy/litestream.yml",
	}
	for source, destination := range canonical {
		data, err := boringstack.Assets.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("read canonical template %s: %w", source, err)
		}
		mode := fs.FileMode(0o644)
		if destination == "deploy/deploy.sh" {
			mode = 0o755
		}
		files = append(files, outputFile{Path: destination, Mode: mode, Data: renderTokens(data, config)})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func renderTokens(data []byte, config projectConfig) []byte {
	replacer := strings.NewReplacer(
		"__PROJECT_NAME__", config.Name,
		"__GO_MODULE__", config.Module,
		"__DATE__", config.Date,
	)
	return []byte(replacer.Replace(string(data)))
}

func writeProject(root string, files []outputFile) error {
	for _, file := range files {
		destination := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return fmt.Errorf("create directory for %s: %w", file.Path, err)
		}
		if err := os.WriteFile(destination, file.Data, file.Mode); err != nil {
			return fmt.Errorf("write %s: %w", file.Path, err)
		}
		if err := os.Chmod(destination, file.Mode); err != nil {
			return fmt.Errorf("set mode on %s: %w", file.Path, err)
		}
	}
	return nil
}

func printFilePlan(w interface{ Write([]byte) (int, error) }, target string, files []outputFile) {
	fmt.Fprintf(w, "Would create %s:\n", target)
	for _, file := range files {
		fmt.Fprintf(w, "  %s\n", file.Path)
	}
}

func printCreated(w interface{ Write([]byte) (int, error) }, target string, files []outputFile) {
	fmt.Fprintf(w, "Created %s (%d files).\n", target, len(files))
	fmt.Fprintln(w, "Next:")
	fmt.Fprintf(w, "  cd %s\n", target)
	fmt.Fprintln(w, "  boringstack dev")
}

func (c *cli) validateGeneratedProject(root string) error {
	if _, err := c.lookPath("go"); err != nil {
		fmt.Fprintln(c.stderr, "! Go is not installed; skipped format, test, and build checks.")
		return nil
	}

	buildOutput, err := os.CreateTemp("", "boringstack-build-*")
	if err != nil {
		return fmt.Errorf("create temporary build output: %w", err)
	}
	buildPath := buildOutput.Name()
	if err := buildOutput.Close(); err != nil {
		return fmt.Errorf("close temporary build output: %w", err)
	}
	defer os.Remove(buildPath)

	checks := [][]string{{"go", "fmt", "./..."}, {"go", "test", "./..."}, {"go", "build", "-o", buildPath, "."}}
	for _, check := range checks {
		fmt.Fprintf(c.stdout, "→ %s\n", strings.Join(check, " "))
		cmd := exec.Command(check[0], check[1:]...)
		cmd.Dir = root
		cmd.Stdout = c.stdout
		cmd.Stderr = c.stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("generated project kept at %s; %s failed: %w", root, strings.Join(check, " "), err)
		}
	}
	fmt.Fprintln(c.stdout, "✓ generated project builds and tests cleanly")
	return nil
}
