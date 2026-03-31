package shell

import (
	"bytes"
	"strings"
	"testing"
)

func TestPreferredShell(t *testing.T) {
	t.Setenv("SHELL", "")
	if got := PreferredShell(); got != "zsh" {
		t.Fatalf("PreferredShell() with empty SHELL = %q, want %q", got, "zsh")
	}

	t.Setenv("SHELL", "/bin/bash")
	if got := PreferredShell(); got != "bash" {
		t.Fatalf("PreferredShell() with bash = %q, want %q", got, "bash")
	}

	t.Setenv("SHELL", "/usr/bin/fish")
	if got := PreferredShell(); got != "zsh" {
		t.Fatalf("PreferredShell() with unsupported shell = %q, want %q", got, "zsh")
	}
}

func TestInitScriptSupportsBashAndZsh(t *testing.T) {
	for _, sh := range []string{"bash", "zsh"} {
		script, err := InitScript(sh)
		if err != nil {
			t.Fatalf("InitScript(%q) returned error: %v", sh, err)
		}
		if !strings.Contains(script, "# ft shell integration for "+sh) {
			t.Fatalf("InitScript(%q) missing header, got: %q", sh, script)
		}
		if !strings.Contains(script, "__FT_CD__=") {
			t.Fatalf("InitScript(%q) missing cd marker support", sh)
		}
	}
}

func TestInitScriptRejectsUnsupportedShell(t *testing.T) {
	_, err := InitScript("fish")
	if err == nil {
		t.Fatalf("InitScript(fish) expected an error")
	}
	if !strings.Contains(err.Error(), "unsupported shell") {
		t.Fatalf("InitScript(fish) error = %q, expected unsupported shell message", err.Error())
	}
}

func TestEmitCDOrWarningEmitsMarkerWhenIntegrationActive(t *testing.T) {
	t.Setenv(EmitCDEnv, EmitCDValue)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	EmitCDOrWarning("/tmp/worktree", &stdout, &stderr)

	if got := strings.TrimSpace(stdout.String()); got != CDMarkerPrefix+"/tmp/worktree" {
		t.Fatalf("stdout marker = %q, want %q", got, CDMarkerPrefix+"/tmp/worktree")
	}
	if strings.TrimSpace(stderr.String()) != "" {
		t.Fatalf("stderr should be empty when integration is active, got %q", stderr.String())
	}
}

func TestEmitCDOrWarningPrintsHintWhenIntegrationInactive(t *testing.T) {
	t.Setenv(EmitCDEnv, "")
	t.Setenv("SHELL", "/bin/bash")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	EmitCDOrWarning("/tmp/worktree", &stdout, &stderr)

	if strings.TrimSpace(stdout.String()) != "" {
		t.Fatalf("stdout should be empty when integration is inactive, got %q", stdout.String())
	}
	msg := stderr.String()
	if !strings.Contains(msg, "shell integration not active") {
		t.Fatalf("stderr missing integration warning, got %q", msg)
	}
	if !strings.Contains(msg, "ft init bash") {
		t.Fatalf("stderr missing shell-specific hint, got %q", msg)
	}
}
