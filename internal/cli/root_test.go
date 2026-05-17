package cli

import (
	"bytes"
	"strings"
	"testing"
)

func executeCommand(args ...string) (string, string, error) {
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

	err := command.Execute()
	return stdout.String(), stderr.String(), err
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
	_, _, err := executeCommand("--color=purple", "version")
	if err == nil {
		t.Fatal("expected invalid color to fail")
	}
	assertContains(t, err.Error(), "invalid --color value")
}

func TestAutomationModeAcceptsLocalCommands(t *testing.T) {
	stdout, _, err := executeCommand("--automation", "version")
	if err != nil {
		t.Fatalf("expected automation mode to succeed for local version command: %v", err)
	}
	assertContains(t, stdout, "authnet 0.1.0-test")
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
