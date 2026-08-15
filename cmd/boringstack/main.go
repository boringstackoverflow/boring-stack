package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var version = "dev"

type cli struct {
	stdin    io.Reader
	stdout   io.Writer
	stderr   io.Writer
	cwd      string
	getenv   func(string) string
	lookPath func(string) (string, error)
	now      func() time.Time
}

func main() {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "boringstack:", err)
		os.Exit(1)
	}

	app := newCLI(wd, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(app.run(os.Args[1:]))
}

func newCLI(wd string, stdin io.Reader, stdout, stderr io.Writer) *cli {
	return &cli{
		stdin:    stdin,
		stdout:   stdout,
		stderr:   stderr,
		cwd:      wd,
		getenv:   os.Getenv,
		lookPath: exec.LookPath,
		now:      time.Now,
	}
}

func (c *cli) run(args []string) int {
	if len(args) == 0 {
		c.printHelp()
		return 0
	}
	if args[0] == "help" {
		if len(args) == 1 {
			c.printHelp()
			return 0
		}
		if len(args) == 2 && c.printCommandHelp(args[1], c.stdout) {
			return 0
		}
		fmt.Fprintln(c.stderr, "boringstack: usage: boringstack help [command]")
		return 2
	}
	if len(args) == 2 && (args[1] == "--help" || args[1] == "-h") && c.printCommandHelp(args[0], c.stdout) {
		return 0
	}

	var err error
	switch args[0] {
	case "new":
		err = c.runNew(args[1:])
	case "init":
		err = c.runInit(args[1:])
	case "dev":
		err = c.runDev(args[1:])
	case "doctor":
		err = c.runDoctor(args[1:])
	case "deploy":
		err = c.runDeploy(args[1:])
	case "version", "--version", "-v":
		fmt.Fprintln(c.stdout, version)
		return 0
	case "--help", "-h":
		c.printHelp()
		return 0
	default:
		fmt.Fprintf(c.stderr, "boringstack: unknown command %q\n\n", args[0])
		c.printHelpTo(c.stderr)
		return 2
	}

	if err == nil {
		return 0
	}
	if isUsageError(err) {
		fmt.Fprintln(c.stderr, "boringstack:", err)
		return 2
	}
	fmt.Fprintln(c.stderr, "boringstack:", err)
	return 1
}

func (c *cli) printCommandHelp(command string, w io.Writer) bool {
	usage := map[string]string{
		"new":     "boringstack new <name> [--module <path>] [--dry-run]",
		"init":    "boringstack init [--name <name>] [--module <path>] [--dry-run]",
		"dev":     "boringstack dev",
		"doctor":  "boringstack doctor [--deploy]",
		"deploy":  "boringstack deploy --host <user@host> --healthz <url> [--remote <path>] [--dry-run]",
		"version": "boringstack version",
	}
	line, ok := usage[command]
	if !ok {
		return false
	}
	fmt.Fprintln(w, "Usage:", line)
	return true
}

func (c *cli) printHelp() {
	c.printHelpTo(c.stdout)
}

func (c *cli) printHelpTo(w io.Writer) {
	fmt.Fprint(w, `Boring Stack turns the stack's defaults into a working project.

Usage:
  boringstack new <name> [--module <path>] [--dry-run]
  boringstack init [--name <name>] [--module <path>] [--dry-run]
  boringstack dev
  boringstack doctor [--deploy]
  boringstack deploy --host <user@host> --healthz <url> [--remote <path>] [--dry-run]
  boringstack version

Examples:
  boringstack new myapp
  cd myapp && boringstack dev
  boringstack deploy --host deploy@app.example.com --healthz https://app.example.com/healthz
`)
}

func (c *cli) projectPath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(c.cwd, path)
}
