package client

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExtractScriptsWritesEmbeddedFilesPrivately(t *testing.T) {
	dir, err := extractScripts(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"auth.ps1", "transport_rule.ps1"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s not extracted: %v", name, err)
		}
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(dir)
		if info.Mode().Perm() != 0o700 {
			t.Errorf("script dir permissions = %o, want 700", info.Mode().Perm())
		}
	}
}

func TestExtractScriptsRepairsTamperedFile(t *testing.T) {
	base := t.TempDir()
	dir, err := extractScripts(base)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "auth.ps1")
	if err := os.WriteFile(target, []byte("malicious"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractScripts(base); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(target)
	if string(data) == "malicious" {
		t.Fatal("tampered script was not replaced by the embedded copy")
	}
}

func TestSecretsAreNeverOnTheCommandLine(t *testing.T) {
	c := &Client{config: Config{Server: "mail.contoso.local", Username: "CONTOSO\\u", Password: "s3cr3t-pass"}}
	_, args := c.buildArgs("/scripts/x.ps1", map[string]string{"Action": "Get"})
	if joined := strings.Join(args, " "); strings.Contains(joined, "s3cr3t") {
		t.Errorf("secret leaked into args: %s", joined)
	}
	if env := strings.Join(c.buildEnv(), "\n"); !strings.Contains(env, "s3cr3t") {
		t.Error("secret missing from child environment")
	}
}

func TestRoleIsPassedToTheBridge(t *testing.T) {
	for role, want := range map[string]string{"": "EXCHANGE_BRIDGE_ROLE=Mailbox", RoleEdge: "EXCHANGE_BRIDGE_ROLE=Edge"} {
		c := &Client{config: Config{Role: role}}
		if env := strings.Join(c.buildEnv(), "\n"); !strings.Contains(env, want) {
			t.Errorf("role %q: %s missing from the child environment", role, want)
		}
	}
}

// With integrated authentication the process identity connects: a password
// set by mistake must never reach PowerShell.
func TestAuthMethodIsPassedToTheBridge(t *testing.T) {
	// A value left in Terraform's own environment (manual bridge tests) must
	// not pass through to PowerShell either.
	t.Setenv(EnvBridgePassword, "inherited-secret")
	integrated := &Client{config: Config{AuthMethod: AuthIntegrated, Password: "pw-secret"}}
	env := strings.Join(integrated.buildEnv(), "\n")
	if !strings.Contains(env, "EXCHANGE_BRIDGE_AUTH_METHOD=integrated") {
		t.Error("EXCHANGE_BRIDGE_AUTH_METHOD=integrated missing from the child environment")
	}
	if strings.Contains(env, "pw-secret") || strings.Contains(env, "inherited-secret") {
		t.Error("a password reached the child environment with integrated authentication")
	}

	password := &Client{config: Config{Password: "pw-secret"}}
	env = strings.Join(password.buildEnv(), "\n")
	for _, want := range []string{"EXCHANGE_BRIDGE_AUTH_METHOD=password", "EXCHANGE_BRIDGE_PASSWORD=pw-secret"} {
		if !strings.Contains(env, want) {
			t.Errorf("default method: %s missing from the child environment", want)
		}
	}
}

// The domain controller reaches auth.ps1 only when it is set: a value left in
// Terraform's own environment must not pin the sessions.
func TestDomainControllerIsPassedToTheBridge(t *testing.T) {
	t.Setenv(EnvBridgeDomainController, "inherited.contoso.local")
	pinned := &Client{config: Config{DomainController: "dc01.contoso.local"}}
	env := strings.Join(pinned.buildEnv(), "\n")
	if !strings.Contains(env, EnvBridgeDomainController+"=dc01.contoso.local") || strings.Contains(env, "inherited") {
		t.Errorf("pinned client: child environment has the wrong domain controller:\n%s", env)
	}
	if env := strings.Join((&Client{}).buildEnv(), "\n"); strings.Contains(env, EnvBridgeDomainController) {
		t.Error("a domain controller reached the child environment although none is set")
	}
}

func TestBuildArgsIsDeterministic(t *testing.T) {
	c := &Client{}
	params := map[string]string{"B": "2", "A": "1", "C": "3"}
	_, first := c.buildArgs("x.ps1", params)
	for i := 0; i < 20; i++ {
		_, again := c.buildArgs("x.ps1", params)
		if strings.Join(again, " ") != strings.Join(first, " ") {
			t.Fatal("argument order changes between calls")
		}
	}
	if !strings.HasSuffix(strings.Join(first, " "), "-A 1 -B 2 -C 3") {
		t.Errorf("params not sorted: %v", first)
	}
}

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(nil) || !IsNotFound([]byte(" \r\n")) {
		t.Error("empty output must mean not found")
	}
	if IsNotFound([]byte("WARNING: something")) || IsNotFound([]byte(`{"Name":"x"}`)) {
		t.Error("non-empty output must never mean not found")
	}
}

func TestEnvParsing(t *testing.T) {
	t.Setenv(EnvScriptTimeout, "")
	if d, _ := durationFromEnv(EnvScriptTimeout, time.Minute); d != time.Minute {
		t.Errorf("default not applied: %s", d)
	}
	t.Setenv(EnvScriptTimeout, "nope")
	if _, err := durationFromEnv(EnvScriptTimeout, time.Minute); err == nil {
		t.Error("invalid duration accepted")
	}
	t.Setenv(EnvMaxConcurrency, "0")
	if _, err := intFromEnv(EnvMaxConcurrency, 4); err == nil {
		t.Error("zero concurrency accepted")
	}
}

// fakePwsh puts a fake "powershell.exe" executable first on PATH. It runs the
// given shell body, which lets us test RunScript without PowerShell or Exchange.
func fakePwsh(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake powershell.exe uses a POSIX shell script")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "powershell.exe"), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func newTestClient(timeout time.Duration) *Client {
	return &Client{scriptDir: "/scripts", timeout: timeout, sem: make(chan struct{}, 1)}
}

func TestRunScriptReturnsStdout(t *testing.T) {
	fakePwsh(t, `echo '{"ok":true}'`)
	out, err := newTestClient(time.Minute).RunScript(context.Background(), "x.ps1", nil)
	if err != nil || strings.TrimSpace(string(out)) != `{"ok":true}` {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestRunScriptSurfacesStderrOnFailure(t *testing.T) {
	fakePwsh(t, `echo "throttled by Exchange" >&2; exit 1`)
	_, err := newTestClient(time.Minute).RunScript(context.Background(), "x.ps1", nil)
	if err == nil || !strings.Contains(err.Error(), "throttled by Exchange") {
		t.Fatalf("expected stderr in error, got %v", err)
	}
}

func TestRunScriptTimesOut(t *testing.T) {
	fakePwsh(t, `exec sleep 30`)
	start := time.Now()
	_, err := newTestClient(300*time.Millisecond).RunScript(context.Background(), "x.ps1", nil)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if time.Since(start) > 10*time.Second {
		t.Fatal("timeout did not stop the process")
	}
}

func TestRunScriptPassesSecretsThroughEnvironment(t *testing.T) {
	fakePwsh(t, `echo "$`+EnvBridgePassword+`"`)
	c := newTestClient(time.Minute)
	c.config.Password = "from-env"
	out, err := c.RunScript(context.Background(), "x.ps1", nil)
	if err != nil || strings.TrimSpace(string(out)) != "from-env" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
