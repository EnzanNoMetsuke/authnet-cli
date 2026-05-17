package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func executeCommand(args ...string) (string, string, error) {
	stdout, stderr, _, err := executeCommandWithExit(args...)
	return stdout, stderr, err
}

func executeCommandWithExit(args ...string) (string, string, ExitCode, error) {
	command := NewRootCommand(BuildInfo{
		Version:        "0.1.0-test",
		Commit:         "test-commit",
		Date:           "2026-05-17T00:00:00Z",
		SchemaVersion:  "0.1.0",
		ContractStatus: "test",
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs(args)

	code := Execute(command)
	var err error
	if code != exitSuccess {
		err = cliError{exitCode: code, code: "command_failed", message: strings.TrimSpace(stderr.String())}
		if strings.TrimSpace(stderr.String()) == "" {
			err = cliError{exitCode: code, code: "command_failed", message: strings.TrimSpace(stdout.String())}
		}
	}
	return stdout.String(), stderr.String(), code, err
}

func TestRootStartsAndShowsHelp(t *testing.T) {
	stdout, _, err := executeCommand("--help")
	if err != nil {
		t.Fatalf("expected help to succeed: %v", err)
	}

	assertContains(t, stdout, "Authorize.Net operations CLI")
	assertContains(t, stdout, "customer-profile")
	assertContains(t, stdout, "response-code")
	assertContains(t, stdout, "--automation")
	assertContains(t, stdout, "--color")
}

func TestVersionFlagPrintsConciseLocalVersion(t *testing.T) {
	stdout, _, err := executeCommand("--version")
	if err != nil {
		t.Fatalf("expected version flag to succeed: %v", err)
	}

	got := strings.TrimSpace(stdout)
	want := "authnet 0.1.0-test"
	if got != want {
		t.Fatalf("version output mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestVersionCommandPrintsContractMetadata(t *testing.T) {
	stdout, _, err := executeCommand("version")
	if err != nil {
		t.Fatalf("expected version command to succeed: %v", err)
	}

	assertContains(t, stdout, "authnet 0.1.0-test")
	assertContains(t, stdout, "schema version: 0.1.0")
	assertContains(t, stdout, "contract status: test")
	assertContains(t, stdout, "commit: test-commit")
}

func TestVersionCommandPrintsStableJSONEnvelope(t *testing.T) {
	stdout, stderr, err := executeCommand("--json", "version")
	if err != nil {
		t.Fatalf("expected JSON version command to succeed: %v\nstderr:\n%s", err, stderr)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON: %v\noutput:\n%s", err, stdout)
	}

	assertJSONField(t, got, "schema_version", "0.1.0")
	assertJSONField(t, got, "command", "authnet version")
	assertJSONField(t, got, "redacted", true)
	assertJSONListLength(t, got, "warnings", 0)
	assertJSONListLength(t, got, "errors", 0)

	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected object data field, got %#v", got["data"])
	}
	assertJSONField(t, data, "version", "0.1.0-test")
	assertJSONField(t, data, "schema_version", "0.1.0")
	assertJSONField(t, data, "contract_status", "test")
	assertJSONField(t, data, "commit", "test-commit")
	assertNotContains(t, stdout, "\x1b[")
}

func TestCompletionCommandIsStatic(t *testing.T) {
	stdout, _, err := executeCommand("completion", "bash")
	if err != nil {
		t.Fatalf("expected bash completion to succeed: %v", err)
	}

	assertContains(t, stdout, "authnet")
	assertContains(t, stdout, "customer-profile")
	assertContains(t, stdout, "response-code")
	assertNotContains(t, stdout, "profile name completion")
	assertNotContains(t, stdout, "__complete")
}

func TestGlobalColorValidation(t *testing.T) {
	_, _, code, err := executeCommandWithExit("--color=purple", "version")
	if err == nil {
		t.Fatal("expected invalid color to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d", code)
	}
	assertContains(t, err.Error(), "invalid --color value")
}

func TestAutomationModeAcceptsLocalCommands(t *testing.T) {
	stdout, _, err := executeCommand("--automation", "version")
	if err != nil {
		t.Fatalf("expected automation mode to succeed for local version command: %v", err)
	}
	assertContains(t, stdout, `"schema_version": "0.1.0"`)
	assertContains(t, stdout, `"version": "0.1.0-test"`)
	assertNotContains(t, stdout, "\x1b[")
}

func TestPathsWarningsRenderForHumanAndJSON(t *testing.T) {
	stdout, stderr, err := executeCommand("paths")
	if err != nil {
		t.Fatalf("expected paths command to succeed: %v", err)
	}
	assertContains(t, stdout, "config directory:")
	assertContains(t, stdout, "sensitive-data persistence: none")
	assertContains(t, stderr, "warning: v1 does not define CLI-controlled sensitive-data persistence paths.")

	stdout, stderr, err = executeCommand("--json", "paths")
	if err != nil {
		t.Fatalf("expected JSON paths command to succeed: %v\nstderr:\n%s", err, stderr)
	}
	assertContains(t, stdout, `"warnings": [`)
	assertContains(t, stdout, `"code": "no_sensitive_persistence"`)
	assertContains(t, stdout, `"sensitive_data_persistence": "none"`)
	assertNotContains(t, stderr, "warning:")
}

func TestJSONFailuresAreStructuredOnStdout(t *testing.T) {
	stdout, stderr, code, err := executeCommandWithExit("--json", "--color=purple", "version")
	if err == nil {
		t.Fatal("expected invalid color to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d", code)
	}
	if stderr != "" {
		t.Fatalf("expected JSON failure stderr to stay empty, got %q", stderr)
	}
	assertContains(t, stdout, `"schema_version": "0.1.0"`)
	assertContains(t, stdout, `"code": "usage_or_config_error"`)
	assertContains(t, stdout, `"message": "invalid --color value \"purple\": expected auto, always, or never"`)
}

func TestNotImplementedUsesExitTaxonomy(t *testing.T) {
	stdout, stderr, code, err := executeCommandWithExit("auth", "test")
	if err == nil {
		t.Fatal("expected scaffold command to fail")
	}
	if code != exitGeneralFailure {
		t.Fatalf("expected general failure exit code, got %d", code)
	}
	if stdout != "" {
		t.Fatalf("expected non-JSON failure stdout to stay empty, got %q", stdout)
	}
	assertContains(t, stderr, "auth test is not implemented in this scaffold")
}

func TestExplicitColorCanApplyToHumanWarningsAndJSON(t *testing.T) {
	_, stderr, err := executeCommand("--color=always", "paths")
	if err != nil {
		t.Fatalf("expected paths command to succeed: %v", err)
	}
	assertContains(t, stderr, "\x1b[33mwarning\x1b[0m")

	stdout, _, err := executeCommand("--json", "--color=always", "version")
	if err != nil {
		t.Fatalf("expected JSON version command to succeed: %v", err)
	}
	assertContains(t, stdout, "\x1b[36m")
}

func assertContains(t *testing.T, text string, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("expected output to contain %q\noutput:\n%s", want, text)
	}
}

func assertNotContains(t *testing.T, text string, unwanted string) {
	t.Helper()
	if strings.Contains(text, unwanted) {
		t.Fatalf("expected output not to contain %q\noutput:\n%s", unwanted, text)
	}
}

func assertJSONField(t *testing.T, object map[string]any, field string, want any) {
	t.Helper()
	got, ok := object[field]
	if !ok {
		t.Fatalf("expected JSON field %q in %#v", field, object)
	}
	if got != want {
		t.Fatalf("JSON field %q mismatch\nwant: %#v\n got: %#v", field, want, got)
	}
}

func assertJSONListLength(t *testing.T, object map[string]any, field string, want int) {
	t.Helper()
	got, ok := object[field].([]any)
	if !ok {
		t.Fatalf("expected JSON array field %q in %#v", field, object)
	}
	if len(got) != want {
		t.Fatalf("JSON array field %q length mismatch\nwant: %d\n got: %d", field, want, len(got))
	}
}
