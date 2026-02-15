package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasSSHConfigHostAliasSupportsInclude(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(filepath.Join(sshDir, "config.d"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	mainCfg := `# main ssh config
Include config.d/*.conf
Host direct
  HostName 127.0.0.1
`
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(mainCfg), 0o644); err != nil {
		t.Fatalf("write main config failed: %v", err)
	}

	includeCfg := `Host orangepi
  HostName 192.168.1.50
`
	if err := os.WriteFile(filepath.Join(sshDir, "config.d", "hosts.conf"), []byte(includeCfg), 0o644); err != nil {
		t.Fatalf("write include config failed: %v", err)
	}

	if !hasSSHConfigHostAlias("direct") {
		t.Fatalf("expected direct host alias to be found")
	}
	if !hasSSHConfigHostAlias("orangepi") {
		t.Fatalf("expected included host alias to be found")
	}
	if hasSSHConfigHostAlias("missing-host") {
		t.Fatalf("did not expect missing host alias to be found")
	}
}

func TestResolveSSHConfigHostEndpointKeepsAliasVerbatim(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	cfg := `Host orangepi
  HostName 192.168.1.55
  User pi
  Port 2222
`
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	endpoint, ok := resolveSSHConfigHostEndpoint("orangepi")
	if !ok {
		t.Fatalf("expected alias to resolve")
	}
	if endpoint != "ssh://orangepi" {
		t.Fatalf("unexpected endpoint: %q", endpoint)
	}
}

func TestHostPatternsMatchAliasSupportsWildcardAndNegation(t *testing.T) {
	if !hostPatternsMatchAlias([]string{"*.lan"}, "orangepi.lan") {
		t.Fatalf("expected wildcard host pattern to match")
	}
	if hostPatternsMatchAlias([]string{"*.lan", "!orangepi.lan"}, "orangepi.lan") {
		t.Fatalf("expected negated host pattern to exclude alias")
	}
	if hostPatternsMatchAlias([]string{"!*.lan"}, "orangepi.lan") {
		t.Fatalf("negated-only pattern should not match")
	}
}
