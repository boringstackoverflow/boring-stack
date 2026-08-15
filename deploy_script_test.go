package boringstack

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeployScriptHealthFailureRollsBack(t *testing.T) {
	output, sshLog, err := runDeployScript(t, false)
	if err == nil {
		t.Fatal("deploy unexpectedly succeeded")
	}
	if !strings.Contains(output, "health check failed, rolling back") {
		t.Fatalf("output does not report rollback:\n%s", output)
	}
	if !strings.Contains(sshLog, "mv app.prev app") {
		t.Fatalf("ssh log does not contain rollback command:\n%s", sshLog)
	}
}

func TestDeployScriptHealthyReleaseDoesNotRollBack(t *testing.T) {
	output, sshLog, err := runDeployScript(t, true)
	if err != nil {
		t.Fatalf("deploy failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, "live: abc123") {
		t.Fatalf("output does not report live revision:\n%s", output)
	}
	if strings.Contains(sshLog, "mv app.prev app") {
		t.Fatalf("healthy deploy attempted rollback:\n%s", sshLog)
	}
}

func runDeployScript(t *testing.T, healthy bool) (output, sshLog string, runErr error) {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	scriptData, err := Assets.ReadFile("templates/deploy.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "deploy.sh")
	if err := os.WriteFile(script, scriptData, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "ssh.log")

	fakes := map[string]string{
		"git":   "#!/bin/sh\necho abc123\n",
		"go":    "#!/bin/sh\ntouch app\n",
		"scp":   "#!/bin/sh\nexit 0\n",
		"ssh":   "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$FAKE_SSH_LOG\"\n",
		"sleep": "#!/bin/sh\nexit 0\n",
		"curl":  "#!/bin/sh\n[ \"$FAKE_HEALTHY\" = yes ]\n",
	}
	for name, contents := range fakes {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(contents), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	healthValue := "no"
	if healthy {
		healthValue = "yes"
	}
	cmd := exec.Command("bash", script)
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"PATH="+bin+":"+os.Getenv("PATH"),
		"HOST=deploy@example.com",
		"REMOTE=/home/deploy/app",
		"HEALTHZ_URL=https://example.com/healthz",
		"FAKE_SSH_LOG="+logPath,
		"FAKE_HEALTHY="+healthValue,
	)
	combined, runErr := cmd.CombinedOutput()
	logData, readErr := os.ReadFile(logPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	return string(combined), string(logData), runErr
}
