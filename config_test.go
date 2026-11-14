package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGetConfig(t *testing.T) {
	path := writeConfig(t, `
proswitch:
  url: https://192.168.1.1
  user: admin
  password: secret
  ignore_cert: true
server:
  port: 9000
`)

	cfg, err := getConfig(path)
	if err != nil {
		t.Fatalf("getConfig: %v", err)
	}

	want := Config{
		LocalServer: LocalServerConfig{Port: 9000},
		ProSwitch: ProSwitchConfig{
			Url:        "https://192.168.1.1",
			User:       "admin",
			Password:   "secret",
			IgnoreCert: true,
		},
	}
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestGetConfigDefaultPort(t *testing.T) {
	path := writeConfig(t, `
proswitch:
  url: http://192.168.1.1
  user: admin
  password: secret
`)

	cfg, err := getConfig(path)
	if err != nil {
		t.Fatalf("getConfig: %v", err)
	}
	if cfg.LocalServer.Port != 8000 {
		t.Errorf("port = %d, want 8000", cfg.LocalServer.Port)
	}
}

func TestGetConfigExampleFile(t *testing.T) {
	// Keep the shipped example loadable
	if _, err := getConfig("example_config.yml"); err != nil {
		t.Errorf("example_config.yml: %v", err)
	}
}

func TestGetConfigErrors(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		wantErr  string
	}{
		{
			name:     "missing url",
			contents: "proswitch:\n  user: admin\n  password: secret\n",
			wantErr:  "empty server URL",
		},
		{
			name:     "missing user",
			contents: "proswitch:\n  url: http://x\n  password: secret\n",
			wantErr:  "empty user",
		},
		{
			name:     "missing password",
			contents: "proswitch:\n  url: http://x\n  user: admin\n",
			wantErr:  "empty password",
		},
		{
			name:     "bad yaml",
			contents: "proswitch: [this is not\n",
			wantErr:  "yaml",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := getConfig(writeConfig(t, tc.contents))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestGetConfigNoPath(t *testing.T) {
	if _, err := getConfig(""); err == nil {
		t.Errorf("expected error for empty path")
	}
}

func TestGetConfigMissingFile(t *testing.T) {
	if _, err := getConfig(filepath.Join(t.TempDir(), "nope.yml")); err == nil {
		t.Errorf("expected error for missing file")
	}
}
