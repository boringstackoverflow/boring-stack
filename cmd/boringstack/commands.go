package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (c *cli) runDev(args []string) error {
	if len(args) != 0 {
		return usagef("dev takes no arguments")
	}
	if _, err := os.Stat(filepath.Join(c.cwd, "go.mod")); err != nil {
		return fmt.Errorf("go.mod not found; run this command from a generated project")
	}
	goPath, err := c.lookPath("go")
	if err != nil {
		return fmt.Errorf("Go is required for development: %w", err)
	}
	port := c.getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Fprintf(c.stdout, "→ running http://localhost:%s\n", port)
	cmd := exec.Command(goPath, "run", ".")
	cmd.Dir = c.cwd
	cmd.Stdin = c.stdin
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (c *cli) runDoctor(args []string) error {
	deploy := false
	for _, arg := range args {
		switch arg {
		case "--deploy":
			deploy = true
		case "--help", "-h":
			return usagef("usage: boringstack doctor [--deploy]")
		default:
			return usagef("unknown argument %q", arg)
		}
	}

	blocked := false
	tools := []string{"go"}
	if deploy {
		tools = append(tools, "git", "ssh", "scp", "curl", "bash")
	}
	for _, tool := range tools {
		if path, err := c.lookPath(tool); err != nil {
			fmt.Fprintf(c.stdout, "✗ %-12s not found on PATH\n", tool)
			blocked = true
		} else {
			fmt.Fprintf(c.stdout, "✓ %-12s %s\n", tool, path)
		}
	}

	required := []string{"go.mod", "main.go", "STACK.md"}
	if deploy {
		required = append(required, "deploy/deploy.sh", "deploy/Caddyfile", "deploy/app.service", "deploy/litestream.yml")
	}
	for _, relative := range required {
		path := filepath.Join(c.cwd, filepath.FromSlash(relative))
		if _, err := os.Stat(path); err != nil {
			fmt.Fprintf(c.stdout, "✗ %-12s missing\n", relative)
			blocked = true
		} else {
			fmt.Fprintf(c.stdout, "✓ %-12s present\n", relative)
		}
	}

	if deploy {
		for _, relative := range []string{"deploy/Caddyfile", "deploy/litestream.yml"} {
			data, err := os.ReadFile(filepath.Join(c.cwd, filepath.FromSlash(relative)))
			if err == nil && containsPlaceholder(string(data)) {
				fmt.Fprintf(c.stdout, "! %-12s still contains setup placeholders\n", relative)
			}
		}
	}

	c.checkBackups(deploy)

	if blocked {
		return fmt.Errorf("doctor found blocking problems")
	}
	fmt.Fprintln(c.stdout, "✓ ready")
	return nil
}

// borelaDrillURL goes through the backend's tracked redirect rather than
// straight at borela.dev, so the click is counted. See D23 in
// boring-stack-backend/DECISIONS.md.
const borelaDrillURL = "https://api.boringstack.org/r/borela?utm_campaign=doctor"

// checkBackups reports the one thing this tool cannot verify for you.
//
// It can see whether a replica is configured. It cannot see whether restoring
// from that replica has ever worked, and that is the gap principle 7 is about.
// Never blocking: a project with no backups yet is a project at an early
// stage, not a broken one.
func (c *cli) checkBackups(deploy bool) {
	data, err := os.ReadFile(filepath.Join(c.cwd, filepath.FromSlash("deploy/litestream.yml")))
	switch {
	case err != nil:
		// With --deploy this file is already in `required` above, and
		// reporting it twice reads like two separate problems.
		if !deploy {
			fmt.Fprintf(c.stdout, "! %-12s no deploy/litestream.yml; nothing is replicating this database\n", "backups")
		}
	case containsPlaceholder(string(data)):
		fmt.Fprintf(c.stdout, "! %-12s litestream.yml still has placeholders; the replica is not real yet\n", "backups")
	default:
		fmt.Fprintf(c.stdout, "✓ %-12s litestream.yml configured\n", "backups")
	}
	fmt.Fprintf(c.stdout, "  nothing here proves a restore works. drill it yourself, or have it\n")
	fmt.Fprintf(c.stdout, "  drilled every week: %s\n", borelaDrillURL)
	if deploy {
		fmt.Fprintf(c.stdout, "  by hand: litestream restore -o /tmp/drill.db <replica-url> \\\n")
		fmt.Fprintf(c.stdout, "           && sqlite3 /tmp/drill.db 'pragma integrity_check'\n")
	}
}

type deployOptions struct {
	Host    string
	Remote  string
	Healthz string
	DryRun  bool
}

func (c *cli) runDeploy(args []string) error {
	options, err := parseDeployArgs(args)
	if err != nil {
		return err
	}
	if options.Host == "" {
		options.Host = c.getenv("HOST")
	}
	if options.Remote == "" {
		options.Remote = c.getenv("REMOTE")
	}
	if options.Remote == "" {
		options.Remote = "/home/deploy/app"
	}
	if options.Healthz == "" {
		options.Healthz = c.getenv("HEALTHZ_URL")
	}
	if options.Host == "" || containsPlaceholder(options.Host) {
		return usagef("deploy requires --host <user@host> (or HOST)")
	}
	if options.Healthz == "" || containsPlaceholder(options.Healthz) {
		return usagef("deploy requires --healthz <https-url> (or HEALTHZ_URL)")
	}
	if !strings.HasPrefix(options.Healthz, "https://") && !strings.HasPrefix(options.Healthz, "http://") {
		return usagef("healthz must be an http:// or https:// URL")
	}

	script := filepath.Join(c.cwd, "deploy", "deploy.sh")
	if _, err := os.Stat(script); err != nil {
		fallback := filepath.Join(c.cwd, "deploy.sh")
		if _, fallbackErr := os.Stat(fallback); fallbackErr != nil {
			return fmt.Errorf("deploy script not found at deploy/deploy.sh or deploy.sh")
		}
		script = fallback
	}

	fmt.Fprintf(c.stdout, "HOST=%s REMOTE=%s HEALTHZ_URL=%s %s\n", options.Host, options.Remote, options.Healthz, script)
	if options.DryRun {
		return nil
	}
	for _, tool := range []string{"go", "git", "ssh", "scp", "curl", "bash"} {
		if _, err := c.lookPath(tool); err != nil {
			return fmt.Errorf("%s is required to deploy", tool)
		}
	}

	cmd := exec.Command("bash", script)
	cmd.Dir = c.cwd
	cmd.Stdin = c.stdin
	cmd.Stdout = c.stdout
	cmd.Stderr = c.stderr
	cmd.Env = replaceEnv(os.Environ(), map[string]string{
		"HOST": options.Host, "REMOTE": options.Remote, "HEALTHZ_URL": options.Healthz,
	})
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("deploy failed: %w", err)
	}
	return nil
}

func parseDeployArgs(args []string) (deployOptions, error) {
	var options deployOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dry-run":
			options.DryRun = true
		case arg == "--host" || arg == "--remote" || arg == "--healthz":
			if i+1 >= len(args) {
				return deployOptions{}, usagef("%s requires a value", arg)
			}
			i++
			setDeployOption(&options, arg, args[i])
		case strings.HasPrefix(arg, "--host="):
			options.Host = strings.TrimPrefix(arg, "--host=")
		case strings.HasPrefix(arg, "--remote="):
			options.Remote = strings.TrimPrefix(arg, "--remote=")
		case strings.HasPrefix(arg, "--healthz="):
			options.Healthz = strings.TrimPrefix(arg, "--healthz=")
		case arg == "--help" || arg == "-h":
			return deployOptions{}, usagef("usage: boringstack deploy --host <user@host> --healthz <url> [--remote <path>] [--dry-run]")
		default:
			return deployOptions{}, usagef("unknown argument %q", arg)
		}
	}
	return options, nil
}

func setDeployOption(options *deployOptions, name, value string) {
	switch name {
	case "--host":
		options.Host = value
	case "--remote":
		options.Remote = value
	case "--healthz":
		options.Healthz = value
	}
}

func containsPlaceholder(value string) bool {
	return strings.Contains(value, "your-vps.example.com") ||
		strings.Contains(value, "your-domain.example.com") ||
		strings.Contains(value, "<account>") || strings.Contains(value, "<bucket>")
}

func replaceEnv(base []string, replacements map[string]string) []string {
	result := make([]string, 0, len(base)+len(replacements))
	for _, item := range base {
		key, _, found := strings.Cut(item, "=")
		if found {
			if _, replace := replacements[key]; replace {
				continue
			}
		}
		result = append(result, item)
	}
	for key, value := range replacements {
		result = append(result, key+"="+value)
	}
	return result
}
