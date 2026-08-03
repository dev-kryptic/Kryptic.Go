package krypticdev

// Tests run against a mock daemon: a unix-socket listener speaking PROTOCOL.md v1.

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func startMockDaemon(t *testing.T, handler func(request map[string]any) any) {
	t.Helper()
	// Unix socket paths are capped (~104 chars on macOS); t.TempDir() is too deep.
	dir, err := os.MkdirTemp("/tmp", "kd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "d.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	t.Setenv("KRYPTIC_SOCKET_PATH", socket)

	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				line, err := bufio.NewReader(c).ReadBytes('\n')
				if err != nil {
					return
				}
				var request map[string]any
				if json.Unmarshal(line, &request) != nil {
					return
				}
				response, _ := json.Marshal(handler(request))
				c.Write(append(response, '\n'))
			}(connection)
		}
	}()
}

func setupProject(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "kryptic.json"), []byte(`{"projectId":"proj_test123456"}`), 0o644)
	t.Chdir(dir)
	t.Setenv("KRYPTIC_SILENT", "true")
	os.Unsetenv("KRYPTIC_DISABLED")
	os.Unsetenv("KRYPTIC_PROJECT_ID")
	os.Unsetenv("KRYPTIC_ENV")
	os.Unsetenv("INJECTED_KEY")
	os.Unsetenv("EXISTING_KEY")
	os.Unsetenv("GO_ENV")
}

func TestInjectsSecrets(t *testing.T) {
	setupProject(t)
	startMockDaemon(t, func(request map[string]any) any {
		if request["projectId"] != "proj_test123456" || request["environment"] != "development" {
			t.Errorf("unexpected request: %v", request)
		}
		return map[string]any{"v": 1, "ok": true, "secrets": []map[string]string{{"key": "INJECTED_KEY", "value": "from-daemon"}}}
	})

	result := Inject()

	if result.Skipped || result.Injected != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if os.Getenv("INJECTED_KEY") != "from-daemon" {
		t.Fatalf("secret not injected")
	}
}

func TestNeverOverwritesExistingVariables(t *testing.T) {
	setupProject(t)
	t.Setenv("EXISTING_KEY", "real-env-wins")
	startMockDaemon(t, func(map[string]any) any {
		return map[string]any{"v": 1, "ok": true, "secrets": []map[string]string{{"key": "EXISTING_KEY", "value": "x"}}}
	})

	result := Inject()

	if result.Injected != 0 || os.Getenv("EXISTING_KEY") != "real-env-wins" {
		t.Fatalf("existing variable was overwritten: %+v", result)
	}
}

func TestNoopWhenDaemonMissing(t *testing.T) {
	setupProject(t)
	t.Setenv("KRYPTIC_SOCKET_PATH", filepath.Join(t.TempDir(), "missing.sock"))

	result := Inject()

	if !result.Skipped || result.Reason != "daemon_unreachable" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestNoopInProduction(t *testing.T) {
	setupProject(t)
	t.Setenv("GO_ENV", "production")

	result := Inject()

	if !result.Skipped || result.Reason != "go_env_production" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestNoopWhenDisabled(t *testing.T) {
	setupProject(t)
	t.Setenv("KRYPTIC_DISABLED", "true")

	result := Inject()

	if !result.Skipped || result.Reason != "disabled" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestHandlesErrorResponses(t *testing.T) {
	setupProject(t)
	startMockDaemon(t, func(map[string]any) any {
		return map[string]any{"v": 1, "ok": false, "error": "access_denied"}
	})

	result := Inject()

	if !result.Skipped || result.Reason != "access_denied" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestEnvOverridesWin(t *testing.T) {
	setupProject(t)
	t.Setenv("KRYPTIC_PROJECT_ID", "proj_override0001")
	t.Setenv("KRYPTIC_ENV", "staging")

	var seen map[string]any
	startMockDaemon(t, func(request map[string]any) any {
		seen = request
		return map[string]any{"v": 1, "ok": true, "secrets": []map[string]string{}}
	})

	Inject()

	if seen["projectId"] != "proj_override0001" || seen["environment"] != "staging" {
		t.Fatalf("overrides not honored: %v", seen)
	}
}
