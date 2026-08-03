// Package krypticdev is the Kryptic Go SDK. During development startup,
// Inject fetches the current project's secrets from the local Kryptic daemon
// and sets them as environment variables. Outside development it is a no-op.
// It never panics — a missing daemon means the application simply starts with
// the environment it already has.
//
// Protocol: daemon/PROTOCOL.md v1 (newline-delimited JSON over a local socket).
package krypticdev

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const protocolVersion = 1

// Result reports what Inject did.
type Result struct {
	Injected int
	Skipped  bool
	Reason   string
}

type secretEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type daemonResponse struct {
	Ok      bool          `json:"ok"`
	Error   string        `json:"error"`
	Message string        `json:"message"`
	Secrets []secretEntry `json:"secrets"`
}

type krypticJSON struct {
	ProjectID          string `json:"projectId"`
	DefaultEnvironment string `json:"defaultEnvironment"`
}

// Inject fetches secrets from the daemon and sets them with os.Setenv.
// Existing environment variables are never overwritten.
func Inject() Result {
	if reason := shouldSkip(); reason != "" {
		return Result{Skipped: true, Reason: reason}
	}

	config := findKrypticJSON()

	projectID := os.Getenv("KRYPTIC_PROJECT_ID")
	if projectID == "" && config != nil {
		projectID = config.ProjectID
	}
	if projectID == "" {
		warn("no kryptic.json found (and no KRYPTIC_PROJECT_ID set) — nothing to inject.")
		return Result{Skipped: true, Reason: "no_project"}
	}

	environment := os.Getenv("KRYPTIC_ENV")
	if environment == "" && config != nil {
		environment = config.DefaultEnvironment
	}
	if environment == "" {
		environment = "development"
	}

	response, err := request(projectID, environment)
	if err != nil {
		warn(fmt.Sprintf("daemon not reachable (%v) — continuing without injected secrets.", err))
		return Result{Skipped: true, Reason: "daemon_unreachable"}
	}

	if !response.Ok {
		warn(fmt.Sprintf("daemon refused the request (%s): %s", response.Error, response.Message))
		return Result{Skipped: true, Reason: response.Error}
	}

	injected := 0
	for _, secret := range response.Secrets {
		if _, exists := os.LookupEnv(secret.Key); exists {
			continue // real environment always wins
		}
		if err := os.Setenv(secret.Key, secret.Value); err == nil {
			injected++
		}
	}

	return Result{Injected: injected}
}

func shouldSkip() string {
	if os.Getenv("KRYPTIC_DISABLED") == "true" {
		return "disabled"
	}

	// Go has no single convention; honor the common ones.
	for _, variable := range []string{"GO_ENV", "APP_ENV", "ENVIRONMENT", "ENV"} {
		value := strings.ToLower(os.Getenv(variable))
		if value == "production" || value == "prod" || value == "staging" {
			return strings.ToLower(variable) + "_" + value
		}
	}

	return ""
}

func timeout() time.Duration {
	if raw := os.Getenv("KRYPTIC_TIMEOUT_MS"); raw != "" {
		if ms, err := strconv.Atoi(raw); err == nil {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return 2 * time.Second
}

func request(projectID, environment string) (*daemonResponse, error) {
	payload, _ := json.Marshal(map[string]any{
		"v": protocolVersion, "type": "secrets", "projectId": projectID, "environment": environment,
	})

	line, err := roundTrip(append(payload, '\n'), timeout())
	if err != nil {
		return nil, err
	}

	var response daemonResponse
	if err := json.Unmarshal(line, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// findKrypticJSON walks up from the working directory looking for kryptic.json.
func findKrypticJSON() *krypticJSON {
	directory, err := os.Getwd()
	if err != nil {
		return nil
	}

	for {
		candidate := filepath.Join(directory, "kryptic.json")
		if data, err := os.ReadFile(candidate); err == nil {
			var config krypticJSON
			if json.Unmarshal(data, &config) == nil {
				return &config
			}
			warn("could not parse " + candidate + " — ignoring it.")
			return nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return nil
		}
		directory = parent
	}
}

func warn(message string) {
	if os.Getenv("KRYPTIC_SILENT") == "true" {
		return
	}
	fmt.Fprintln(os.Stderr, "[kryptic] "+message)
}
