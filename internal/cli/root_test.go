package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

func TestMain(m *testing.M) {
	for _, env := range os.Environ() {
		name, _, _ := strings.Cut(env, "=")
		if strings.HasPrefix(name, "AUTHNET_") {
			_ = os.Unsetenv(name)
		}
	}

	configDir, err := os.MkdirTemp("", "authnet-cli-test-config-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create isolated config directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.Setenv(configEnvName, configDir); err != nil {
		fmt.Fprintf(os.Stderr, "set isolated config directory: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(configDir)
	os.Exit(code)
}

func executeCommand(args ...string) (string, string, error) {
	stdout, stderr, _, err := executeCommandWithInput("", args...)
	return stdout, stderr, err
}

func executeCommandWithExit(args ...string) (string, string, ExitCode, error) {
	return executeCommandWithInput("", args...)
}

func executeCommandWithInput(input string, args ...string) (string, string, ExitCode, error) {
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
	command.SetIn(strings.NewReader(input))
	code := ExecuteWithArgs(command, args)
	var err error
	if code != exitSuccess {
		err = cliError{exitCode: code, code: "command_failed", message: strings.TrimSpace(stderr.String())}
		if strings.TrimSpace(stderr.String()) == "" {
			err = cliError{exitCode: code, code: "command_failed", message: strings.TrimSpace(stdout.String())}
		}
	}
	return stdout.String(), stderr.String(), code, err
}

func writeInvalidColorPreferenceConfig(t *testing.T, color string) string {
	t.Helper()
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	configText := fmt.Sprintf(`version: 1
profiles: []
preferences:
  color: %s
`, color)
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte(configText), 0o600); err != nil {
		t.Fatalf("expected to write profile config fixture: %v", err)
	}
	return configDir
}

func writeTransactionSortPreferenceConfig(t *testing.T, configDir string, sortBy string, sortOrder string) {
	t.Helper()
	configBytes, err := yaml.Marshal(map[string]any{
		"version":  profileConfigVersion,
		"profiles": []profileEntry{},
		"preferences": map[string]any{
			"transaction_list": map[string]any{
				"sort_by":    sortBy,
				"sort_order": sortOrder,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected to marshal profile config fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), configBytes, 0o600); err != nil {
		t.Fatalf("expected to write profile config fixture: %v", err)
	}
}

func writeTransactionFilterPreferenceConfig(t *testing.T, configDir string, status string, amount any, payment string) {
	t.Helper()
	configBytes, err := yaml.Marshal(map[string]any{
		"version":  profileConfigVersion,
		"profiles": []profileEntry{},
		"preferences": map[string]any{
			"transaction_list": map[string]any{
				"filter": map[string]any{
					"status":  status,
					"amount":  amount,
					"payment": payment,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected to marshal profile config fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), configBytes, 0o600); err != nil {
		t.Fatalf("expected to write profile config fixture: %v", err)
	}
}

func writeLegacyProfileConfig(t *testing.T, configDir string) {
	t.Helper()
	legacyConfig := `{
  "version": 1,
  "default_profile": "sandbox-main",
  "profiles": [
    {
      "name": "sandbox-main",
      "environment": "sandbox",
      "credential_source": {
        "type": "env",
        "api_login_id_env": "AUTHNET_API_LOGIN_ID",
        "transaction_key_env": "AUTHNET_TRANSACTION_KEY"
      }
    }
  ]
}
`
	if err := os.WriteFile(filepath.Join(configDir, legacyProfileConfigFileName), []byte(legacyConfig), 0o600); err != nil {
		t.Fatalf("expected to write legacy profile config fixture: %v", err)
	}
}

func writeDeprecatedProfileConfig(t *testing.T, configDir string) {
	t.Helper()
	writeLegacyProfileConfig(t, configDir)
	if err := os.Rename(filepath.Join(configDir, legacyProfileConfigFileName), filepath.Join(configDir, deprecatedProfileConfigFileName)); err != nil {
		t.Fatalf("expected to rename legacy profile config fixture: %v", err)
	}
}

func stubUnixTimestampNow(timestamp int64) func() {
	original := unixTimestampNow
	unixTimestampNow = func() int64 {
		return timestamp
	}
	return func() {
		unixTimestampNow = original
	}
}

func stubUnixTimestampSequence(timestamps ...int64) func() {
	original := unixTimestampNow
	index := 0
	unixTimestampNow = func() int64 {
		if index >= len(timestamps) {
			return timestamps[len(timestamps)-1]
		}
		timestamp := timestamps[index]
		index++
		return timestamp
	}
	return func() {
		unixTimestampNow = original
	}
}

func writeValidProfileConfig(t *testing.T, configDir string) {
	t.Helper()
	configText := `version: 1
default_profile: sandbox-main
profiles:
  - name: sandbox-main
    environment: sandbox
    credential_source:
      type: env
      api_login_id_env: AUTHNET_API_LOGIN_ID
      transaction_key_env: AUTHNET_TRANSACTION_KEY
preferences: {}
`
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte(configText), 0o600); err != nil {
		t.Fatalf("expected to write YAML profile config fixture: %v", err)
	}
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
	assertContains(t, stdout, "--raw-response")
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
	assertContains(t, stdout, "built: 2026-05-17T00:00:00Z")
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
	assertJSONField(t, data, "built_at", "2026-05-17T00:00:00Z")
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
	_, stderr, code, err := executeCommandWithExit("--color=purple", "version")
	if err == nil {
		t.Fatal("expected human invalid color to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d", code)
	}
	assertContains(t, stderr, "invalid --color value")
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
	assertContains(t, stdout, "profile config file:")
	assertContains(t, stdout, "config.yaml")
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

func TestPathsDoesNotRequireValidProfileConfigYAML(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte("profiles: [\n"), 0o600); err != nil {
		t.Fatalf("expected to write malformed YAML config fixture: %v", err)
	}

	stdout, stderr, err := executeCommand("paths")
	if err != nil {
		t.Fatalf("expected paths command to succeed with malformed profile config: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, "config directory: "+configDir)
	assertContains(t, stdout, "profile config file: "+filepath.Join(configDir, profileConfigFileName))
	assertNotContains(t, stderr, "profile config is not valid YAML")
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

func TestInvalidColorPreferenceReportsSourceInAutomationJSON(t *testing.T) {
	configDir := writeInvalidColorPreferenceConfig(t, "pizza")

	stdout, stderr, code, err := executeCommandWithExit("version", "--automation")
	if err == nil {
		t.Fatal("expected invalid color preference to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected automation failure stderr to stay empty, got %q", stderr)
	}
	assertContains(t, stdout, `"schema_version": "0.1.0"`)
	assertContains(t, stdout, `"code": "usage_or_config_error"`)
	assertContains(t, stdout, `preferences.color`)
	assertContains(t, stdout, filepath.Join(configDir, profileConfigFileName))
	assertContains(t, stdout, `pizza`)
	assertNotContains(t, stdout, `invalid --color value`)
}

func TestInvalidColorPreferenceReportsSourceInJSON(t *testing.T) {
	configDir := writeInvalidColorPreferenceConfig(t, "pizza")

	stdout, stderr, code, err := executeCommandWithExit("version", "--json")
	if err == nil {
		t.Fatal("expected invalid color preference to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected JSON failure stderr to stay empty, got %q", stderr)
	}
	assertContains(t, stdout, `"code": "usage_or_config_error"`)
	assertContains(t, stdout, `preferences.color`)
	assertContains(t, stdout, filepath.Join(configDir, profileConfigFileName))
	assertContains(t, stdout, `pizza`)
	assertNotContains(t, stdout, `invalid --color value`)
}

func TestInvalidColorPreferenceIgnoresEmptyColorEnvironmentForSource(t *testing.T) {
	configDir := writeInvalidColorPreferenceConfig(t, "pizza")
	t.Setenv("AUTHNET_COLOR", "")

	stdout, stderr, code, err := executeCommandWithExit("version", "--json")
	if err == nil {
		t.Fatal("expected invalid color preference to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected JSON failure stderr to stay empty, got %q", stderr)
	}
	assertContains(t, stdout, `preferences.color`)
	assertContains(t, stdout, filepath.Join(configDir, profileConfigFileName))
	assertNotContains(t, stdout, `AUTHNET_COLOR`)
}

func TestInvalidNonStringColorPreferenceReportsConfiguredValue(t *testing.T) {
	configDir := writeInvalidColorPreferenceConfig(t, "123")

	stdout, stderr, code, err := executeCommandWithExit("version", "--json")
	if err == nil {
		t.Fatal("expected invalid color preference to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected JSON failure stderr to stay empty, got %q", stderr)
	}
	assertContains(t, stdout, `preferences.color`)
	assertContains(t, stdout, filepath.Join(configDir, profileConfigFileName))
	assertContains(t, stdout, `\"123\"`)
	assertNotContains(t, stdout, `value \"\"`)
}

func TestInvalidColorPreferenceReportsSourceInHumanOutput(t *testing.T) {
	configDir := writeInvalidColorPreferenceConfig(t, "pizza")

	stdout, stderr, code, err := executeCommandWithExit("version")
	if err == nil {
		t.Fatal("expected invalid color preference to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected human failure stdout to stay empty, got %q", stdout)
	}
	assertContains(t, stderr, `preferences.color`)
	assertContains(t, stderr, filepath.Join(configDir, profileConfigFileName))
	assertContains(t, stderr, `"pizza"`)
	assertNotContains(t, stderr, `invalid --color value`)
}

func TestNoColorOverridesInvalidColorPreference(t *testing.T) {
	writeInvalidColorPreferenceConfig(t, "pizza")

	stdout, stderr, code, err := executeCommandWithExit("--no-color", "version")
	if err != nil {
		t.Fatalf("expected --no-color to override invalid color preference: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if code != exitSuccess {
		t.Fatalf("expected success exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, "authnet 0.1.0-test")
	assertNotContains(t, stderr, "preferences.color")
	assertNotContains(t, stdout, "\x1b[")
}

func TestNoColorDoesNotHideInvalidExplicitColorFlag(t *testing.T) {
	_, stderr, code, err := executeCommandWithExit("--color=purple", "--no-color", "version")
	if err == nil {
		t.Fatal("expected human invalid explicit color to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d", code)
	}
	assertContains(t, stderr, `invalid --color value "purple"`)
}

func TestExplicitHelpShowsCommandHelpSuccessfully(t *testing.T) {
	stdout, stderr, code, err := executeCommandWithExit("sandbox", "--help")
	if err != nil {
		t.Fatalf("expected sandbox help to succeed: %v", err)
	}
	if code != exitSuccess {
		t.Fatalf("expected help-only success exit code, got %d", code)
	}
	if stderr != "" {
		t.Fatalf("expected sandbox help stderr to stay empty, got %q", stderr)
	}
	assertContains(t, stdout, "charge")
	assertContains(t, stdout, "Run sandbox-only test helpers")
}

func TestIncompleteCommandGroupsShowHelpAndReturnUsageExitCode(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "root",
			args: []string{},
			want: "Authorize.Net operations CLI",
		},
		{
			name: "config group",
			args: []string{"config"},
			want: "Manage local non-secret profile config",
		},
		{
			name: "auth group",
			args: []string{"auth"},
			want: "Test Authorize.Net profile authentication",
		},
		{
			name: "profile group",
			args: []string{"profile"},
			want: "Manage local profiles",
		},
		{
			name: "transaction group",
			args: []string{"transaction"},
			want: "Inspect Authorize.Net transactions",
		},
		{
			name: "transaction unsettled group",
			args: []string{"transaction", "unsettled"},
			want: "Inspect unsettled transaction set",
		},
		{
			name: "customer profile group",
			args: []string{"customer-profile"},
			want: "Inspect Authorize.Net customer profiles",
		},
		{
			name: "response code group",
			args: []string{"response-code"},
			want: "Explain Authorize.Net response codes",
		},
		{
			name: "sandbox group",
			args: []string{"sandbox"},
			want: "Run sandbox-only test helpers",
		},
		{
			name: "sandbox scenario group",
			args: []string{"sandbox", "charge"},
			want: "Run sandbox card charge scenarios",
		},
		{
			name: "completion group",
			args: []string{"completion"},
			want: "Generate static shell completion scripts",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code, err := executeCommandWithExit(tt.args...)
			if err == nil {
				t.Fatalf("expected %v to fail", tt.args)
			}
			if code != exitUsageOrConfig {
				t.Fatalf("expected %v to exit %d, got %d\nstdout:\n%s\nstderr:\n%s", tt.args, exitUsageOrConfig, code, stdout, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected incomplete command group stderr to stay empty, got %q", stderr)
			}
			assertContains(t, stdout, tt.want)
			assertContains(t, stdout, "Usage:")
			assertContains(t, stdout, "Available Commands:")
		})
	}
}

func TestCommandGroupHelpMarksRequiredSubcommand(t *testing.T) {
	stdout, stderr, code, err := executeCommandWithExit("transaction")
	if err == nil {
		t.Fatal("expected incomplete command group to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d", code)
	}
	if stderr != "" {
		t.Fatalf("expected stderr to stay empty, got %q", stderr)
	}
	assertContains(t, stdout, "Usage:\n  authnet transaction <command> [flags]")
	assertContains(t, stdout, `Use "authnet transaction <command> --help" for more information about a command.`)
	assertNotContains(t, stdout, "authnet transaction [command]")
}

func TestCommandHelpMarksRequiredArguments(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "transaction id",
			args: []string{"transaction", "get", "--help"},
			want: "Usage:\n  authnet transaction get <TRANSACTION_ID> [flags]",
		},
		{
			name: "customer profile id",
			args: []string{"customer-profile", "get", "--help"},
			want: "Usage:\n  authnet customer-profile get <CUSTOMER_PROFILE_ID> [flags]",
		},
		{
			name: "response code",
			args: []string{"response-code", "explain", "--help"},
			want: "Usage:\n  authnet response-code explain <CODE> [flags]",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code, err := executeCommandWithExit(tt.args...)
			if err != nil {
				t.Fatalf("expected help to succeed: %v", err)
			}
			if code != exitSuccess {
				t.Fatalf("expected help success exit code, got %d", code)
			}
			if stderr != "" {
				t.Fatalf("expected stderr to stay empty, got %q", stderr)
			}
			assertContains(t, stdout, tt.want)
		})
	}
}

func TestMissingRequiredArgumentsShowCommandUsage(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		wantError   string
		wantUsage   string
		notExpected string
	}{
		{
			name:        "transaction id",
			args:        []string{"transaction", "get"},
			wantError:   "transaction get requires <TRANSACTION_ID>",
			wantUsage:   "Usage:\n  authnet transaction get <TRANSACTION_ID> [flags]",
			notExpected: "accepts 1 arg(s), received 0",
		},
		{
			name:        "customer profile id",
			args:        []string{"customer-profile", "get"},
			wantError:   "customer-profile get requires <CUSTOMER_PROFILE_ID>",
			wantUsage:   "Usage:\n  authnet customer-profile get <CUSTOMER_PROFILE_ID> [flags]",
			notExpected: "accepts 1 arg(s), received 0",
		},
		{
			name:        "response code",
			args:        []string{"response-code", "explain"},
			wantError:   "response-code explain requires <CODE>",
			wantUsage:   "Usage:\n  authnet response-code explain <CODE> [flags]",
			notExpected: "accepts 1 arg(s), received 0",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code, err := executeCommandWithExit(tt.args...)
			if err == nil {
				t.Fatalf("expected %v to fail", tt.args)
			}
			if code != exitUsageOrConfig {
				t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			assertContains(t, stderr, tt.wantError)
			assertContains(t, stdout, tt.wantUsage)
			assertNotContains(t, stdout+stderr, tt.notExpected)
		})
	}
}

func TestMissingArgumentMessageDerivesRootPrefix(t *testing.T) {
	root := NewRootCommand(BuildInfo{SchemaVersion: "0.1.0"})
	root.Use = "anet"
	cmd, _, err := root.Find([]string{"transaction", "get"})
	if err != nil {
		t.Fatalf("expected to find transaction get command: %v", err)
	}

	message := missingArgumentMessage(cmd, 1, []string{"<TRANSACTION_ID>"})
	if message != "transaction get requires <TRANSACTION_ID>" {
		t.Fatalf("expected root prefix to be removed, got %q", message)
	}
}

func TestMissingRequiredArgumentsReturnStructuredJSONUsageFailure(t *testing.T) {
	stdout, stderr, code, err := executeCommandWithExit("--json", "transaction", "get")
	if err == nil {
		t.Fatal("expected missing JSON argument to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d", code)
	}
	if stderr != "" {
		t.Fatalf("expected JSON failure stderr to stay empty, got %q", stderr)
	}
	assertContains(t, stdout, `"command": "authnet transaction get"`)
	assertContains(t, stdout, `"code": "usage_or_config_error"`)
	assertContains(t, stdout, `"message": "transaction get requires <TRANSACTION_ID>"`)
	assertNotContains(t, stdout, "Usage:")
}

func TestIncompleteCommandGroupsReturnStructuredJSONUsageFailure(t *testing.T) {
	stdout, stderr, code, err := executeCommandWithExit("--json", "transaction")
	if err == nil {
		t.Fatal("expected incomplete JSON command group to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d", code)
	}
	if stderr != "" {
		t.Fatalf("expected JSON failure stderr to stay empty, got %q", stderr)
	}
	assertContains(t, stdout, `"command": "authnet transaction"`)
	assertContains(t, stdout, `"code": "usage_or_config_error"`)
	assertContains(t, stdout, `"message": "authnet transaction requires a subcommand"`)
	assertNotContains(t, stdout, "Available Commands:")
}

func TestHumanFailuresReturnTaxonomyExitCodes(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want ExitCode
	}{
		{
			name: "invalid global option",
			args: []string{"--color=purple", "version"},
			want: exitUsageOrConfig,
		},
		{
			name: "command validation",
			args: []string{"transaction", "list", "--limit", "101"},
			want: exitUsageOrConfig,
		},
		{
			name: "local missing resource",
			args: []string{"response-code", "explain", "ZZZ"},
			want: exitNotFound,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code, err := executeCommandWithExit(tt.args...)
			if err == nil {
				t.Fatalf("expected %v to fail", tt.args)
			}
			if code != tt.want {
				t.Fatalf("expected %v to exit %d, got %d\nstdout:\n%s\nstderr:\n%s", tt.args, tt.want, code, stdout, stderr)
			}
			if strings.TrimSpace(stdout) == "" && strings.TrimSpace(stderr) == "" {
				t.Fatalf("expected %v to render an error or report", tt.args)
			}
		})
	}
}

func TestResponseCodeExplainHumanOutput(t *testing.T) {
	stdout, stderr, err := executeCommand("response-code", "explain", "I00001")
	if err != nil {
		t.Fatalf("expected response-code explain to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, "code: I00001")
	assertContains(t, stdout, "family: api_message")
	assertContains(t, stdout, "meaning: The request was processed successfully.")
	assertContains(t, stdout, "likely causes:")
	assertContains(t, stdout, "recommended next steps:")
	assertContains(t, stdout, "reference: 2026-05-18 reviewed 2026-05-18")
}

func TestResponseCodeExplainJSONGoldenOutput(t *testing.T) {
	stdout, stderr, err := executeCommand("--json", "response-code", "explain", "2", "--family", "transaction_response")
	if err != nil {
		t.Fatalf("expected JSON response-code explain to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	want := `{
  "schema_version": "0.1.0",
  "command": "authnet response-code explain",
  "redacted": true,
  "warnings": [],
  "errors": [],
  "data": {
    "query_code": "2",
    "query_family": "transaction_response",
    "matches": [
      {
        "code": "2",
        "family": "transaction_response",
        "title": "Declined",
        "source_meaning": "The transaction was declined.",
        "likely_causes": [
          "The issuer or processor declined the payment request."
        ],
        "recommended_next_steps": [
          "Do not retry blindly. Ask the operator to review the detailed response reason and use another payment method if needed."
        ],
        "source_links": [
          "https://developer.authorize.net/api/reference/index.html"
        ]
      }
    ],
    "reference": {
      "version": "2026-05-18",
      "reviewed_at": "2026-05-18",
      "maintainer": "authnet-cli curated reference",
      "maintenance": "Review the official Authorize.Net response-code and API reference pages before each release that changes this file, then update the reference version, review date, records, and tests together.",
      "sources": [
        {
          "title": "Authorize.Net response-code tool",
          "url": "https://developer.authorize.net/api/reference/responseCodes.html"
        },
        {
          "title": "Authorize.Net API error and response codes",
          "url": "https://developer.authorize.net/api/reference/features/errorandresponsecodes.html"
        },
        {
          "title": "Authorize.Net API transaction response fields",
          "url": "https://developer.authorize.net/api/reference/index.html"
        }
      ]
    }
  }
}
`
	if stdout != want {
		t.Fatalf("JSON output mismatch\nwant:\n%s\ngot:\n%s", want, stdout)
	}
}

func TestResponseCodeExplainDisambiguatesByFamily(t *testing.T) {
	stdout, stderr, code, err := executeCommandWithExit("--json", "response-code", "explain", "N")
	if err == nil {
		t.Fatal("expected ambiguous response code to fail without family")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "ambiguous_response_code"`)
	assertContains(t, stdout, "response code N is ambiguous")
	assertContains(t, stdout, `"family": "avs"`)
	assertContains(t, stdout, `"family": "cvv"`)

	stdout, stderr, err = executeCommand("--json", "response-code", "explain", "N", "--family", "cvv")
	if err != nil {
		t.Fatalf("expected family-disambiguated lookup to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"query_family": "cvv"`)
	assertContains(t, stdout, `"title": "Card code did not match"`)
	assertNotContains(t, stdout, `"title": "Address and postal code did not match"`)
}

func TestResponseCodeExplainMissingCodeIncludesMetadata(t *testing.T) {
	stdout, stderr, code, err := executeCommandWithExit("--json", "response-code", "explain", "ZZZ")
	if err == nil {
		t.Fatal("expected missing response code to fail")
	}
	if code != exitNotFound {
		t.Fatalf("expected not-found exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "response_code_not_found"`)
	assertContains(t, stdout, `"query_code": "ZZZ"`)
	assertContains(t, stdout, `"matches": []`)
	assertContains(t, stdout, `"reviewed_at": "2026-05-18"`)
}

func TestResponseCodeExplainAmbiguousHumanOutputUsesTable(t *testing.T) {
	stdout, stderr, code, err := executeCommandWithExit("response-code", "explain", "N")
	if err == nil {
		t.Fatal("expected ambiguous response code to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, "response code N is ambiguous")
	assertContains(t, stdout, "Family")
	assertContains(t, stdout, "Title")
	assertContains(t, stdout, "avs")
	assertContains(t, stdout, "cvv")
}

func TestSandboxChargeApprovedUsesAliasAndRedactedOutput(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "sandbox-login")
	t.Setenv(transactionKeyEnvName, "sandbox-key")
	server := newSandboxTransactionTestServer(t, []sandboxRequestExpectation{{
		Want: []string{
			`"createTransactionRequest"`,
			`"transactionType":"authCaptureTransaction"`,
			`"amount":"12.34"`,
			`"cardNumber":"4111111111111111"`,
			`"cardCode":"900"`,
		},
		Body: sandboxApprovedResponse("1000001", "1", "This transaction has been approved.", "Y", "M"),
	}})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "sandbox", "charge", "approved", "--card", "visa", "--amount", "12.34")
	if err != nil {
		t.Fatalf("expected sandbox charge to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"command": "authnet sandbox charge approved"`)
	assertContains(t, stdout, `"scenario": "approved"`)
	assertContains(t, stdout, `"card_alias": "visa"`)
	assertContains(t, stdout, `"transaction_id": "1000001"`)
	assertContains(t, stdout, `"response_code": "1"`)
	assertContains(t, stdout, `"avs_response": "Y"`)
	assertContains(t, stdout, `"card_code_response": "M"`)
	assertNotContains(t, stdout, "4111111111111111")
	assertNotContains(t, stdout, "900")
}

func TestSandboxChargeAVSMatchDefaultsToCompatibleCard(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "sandbox-login")
	t.Setenv(transactionKeyEnvName, "sandbox-key")
	server := newSandboxTransactionTestServer(t, []sandboxRequestExpectation{{
		Want: []string{
			`"cardNumber":"5424000000000015"`,
			`"zip":"46214"`,
		},
		Body: sandboxApprovedResponse("1000002", "1", "This transaction has been approved.", "X", "M"),
	}})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "sandbox", "charge", "avs", "--variant", "match")
	if err != nil {
		t.Fatalf("expected sandbox avs charge to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"variant": "match"`)
	assertContains(t, stdout, `"card_alias": "mastercard"`)
	assertContains(t, stdout, `"avs_response": "X"`)
	assertNotContains(t, stdout, "5424000000000015")
}

func TestSandboxChargeRejectsProductionBeforeGatewayRequest(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "prod-login")
	t.Setenv(transactionKeyEnvName, "prod-key")
	var called atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(server.Close)
	withGatewayTestEndpoint(t, environmentProduction, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "prod-main", "--environment", "production")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--json", "--profile", "prod-main", "sandbox", "charge", "approved")
	if err == nil {
		t.Fatal("expected production sandbox charge to fail")
	}
	if code != exitSafetyDenied {
		t.Fatalf("expected safety denied exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if called.Load() {
		t.Fatal("production sandbox charge unexpectedly contacted gateway")
	}
	assertContains(t, stdout, `"code": "safety_policy_denied"`)
	assertContains(t, stdout, "sandbox charge helpers require a sandbox-classified profile")
}

func TestSandboxChargeDuplicateReportsSecondAttempt(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "sandbox-login")
	t.Setenv(transactionKeyEnvName, "sandbox-key")
	server := newSandboxTransactionTestServer(t, []sandboxRequestExpectation{
		{
			Want: []string{
				`"settingName":"duplicateWindow","settingValue":"120"`,
				`"invoiceNumber":"an-dup-`,
			},
			Body: sandboxApprovedResponse("1000003", "1", "This transaction has been approved.", "Y", "M"),
		},
		{
			Want: []string{
				`"settingName":"duplicateWindow","settingValue":"120"`,
				`"invoiceNumber":"an-dup-`,
			},
			Body: `{"messages":{"resultCode":"Error","message":[{"code":"E00027","text":"The transaction was unsuccessful."}]},"transactionResponse":{"responseCode":"3","transId":"1000003","authCode":"ABC123","avsResultCode":"Y","cvvResultCode":"M","errors":[{"errorCode":"11","errorText":"A duplicate transaction has been submitted."}]}}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "sandbox", "charge", "duplicate", "--amount", "12.34", "--window", "120")
	if err != nil {
		t.Fatalf("expected sandbox duplicate charge to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"scenario": "duplicate"`)
	assertContains(t, stdout, `"duplicate_window_seconds": 120`)
	assertContains(t, stdout, `"attempt": 1`)
	assertContains(t, stdout, `"attempt": 2`)
	assertContains(t, stdout, `"transaction_id": "1000003"`)
	assertContains(t, stdout, `"response_code": "3"`)
	assertContains(t, stdout, `"gateway_message_code": "11"`)
	assertContains(t, stdout, `"message": "A duplicate transaction has been submitted."`)
	assertContains(t, stdout, `"duplicate_detected": true`)
	assertNotContains(t, stdout, "4111111111111111")
	assertNotContains(t, stdout, "900")
}

func TestSandboxChargeDuplicateHumanOutputUsesAttemptTable(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "sandbox-login")
	t.Setenv(transactionKeyEnvName, "sandbox-key")
	server := newSandboxTransactionTestServer(t, []sandboxRequestExpectation{
		{
			Want: []string{
				`"settingName":"duplicateWindow","settingValue":"120"`,
				`"invoiceNumber":"an-dup-`,
			},
			Body: sandboxApprovedResponse("1000003", "1", "This transaction has been approved.", "Y", "M"),
		},
		{
			Want: []string{
				`"settingName":"duplicateWindow","settingValue":"120"`,
				`"invoiceNumber":"an-dup-`,
			},
			Body: `{"messages":{"resultCode":"Error","message":[{"code":"E00027","text":"The transaction was unsuccessful."}]},"transactionResponse":{"responseCode":"3","transId":"1000003","authCode":"ABC123","avsResultCode":"Y","cvvResultCode":"M","errors":[{"errorCode":"11","errorText":"A duplicate transaction has been submitted."}]}}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("sandbox", "charge", "duplicate", "--amount", "12.34", "--window", "120")
	if err != nil {
		t.Fatalf("expected sandbox duplicate charge to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, "sandbox charge: duplicate")
	assertContains(t, stdout, "Attempt")
	assertContains(t, stdout, "Response")
	assertContains(t, stdout, "Transaction")
	assertContains(t, stdout, "Duplicate")
	assertContains(t, stdout, "1000003")
	assertContains(t, stdout, "detected")
	assertNotContains(t, stdout, "4111111111111111")
	assertNotContains(t, stdout, "900")
}

func TestSandboxChargeInvoiceNumbersFitGatewayLimit(t *testing.T) {
	withFixedNow(t, time.Unix(0, 1779119443925440000))

	cases := []struct {
		scenario string
		options  sandboxChargeOptions
	}{
		{scenario: "approved", options: sandboxChargeOptions{Card: "visa", Amount: "1.23"}},
		{scenario: "declined", options: sandboxChargeOptions{Card: "visa", Amount: "1.23"}},
		{scenario: "avs", options: sandboxChargeOptions{Card: "mastercard", Amount: "1.23", Variant: "match"}},
		{scenario: "cvv", options: sandboxChargeOptions{Card: "visa", Amount: "1.23", Variant: "match"}},
		{scenario: "duplicate", options: sandboxChargeOptions{Card: "visa", Amount: "1.23", Window: 120}},
	}
	for _, testCase := range cases {
		plan, err := newSandboxChargePlan(testCase.scenario, &testCase.options)
		if err != nil {
			t.Fatalf("expected %s charge plan to build: %v", testCase.scenario, err)
		}

		invoiceNumber := plan.gatewayRequest().Order.InvoiceNumber
		if len(invoiceNumber) > 20 {
			t.Fatalf("expected %s invoice number to fit Authorize.Net 20 character limit, got %q length %d", testCase.scenario, invoiceNumber, len(invoiceNumber))
		}
		assertContains(t, invoiceNumber, "an-")
	}
}

func TestSandboxChargeRejectsUnknownAliasAndVariant(t *testing.T) {
	stdout, stderr, code, err := executeCommandWithExit("--json", "sandbox", "charge", "approved", "--card", "bad")
	if err == nil {
		t.Fatal("expected unknown alias to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, "unknown sandbox card alias")
	assertContains(t, stdout, "visa, mastercard, amex, or discover")

	stdout, stderr, code, err = executeCommandWithExit("--json", "sandbox", "charge", "cvv", "--variant", "bad")
	if err == nil {
		t.Fatal("expected unknown variant to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, "unknown sandbox charge cvv variant")
	assertContains(t, stdout, "match, no-match, not-processed, should-be-present, issuer-unavailable")
}

func TestSandboxAuthAndChargeApprovedIntegration(t *testing.T) {
	if os.Getenv("AUTHNET_SANDBOX_INTEGRATION") != "1" {
		t.Skip("set AUTHNET_SANDBOX_INTEGRATION=1 with sandbox credentials to run")
	}
	if os.Getenv(apiLoginIDEnvName) == "" || os.Getenv(transactionKeyEnvName) == "" {
		t.Skip("sandbox integration requires AUTHNET_API_LOGIN_ID and AUTHNET_TRANSACTION_KEY")
	}
	t.Setenv(configEnvName, t.TempDir())

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}

	stdout, stderr, err := executeCommand("--json", "auth", "test")
	if err != nil {
		t.Fatalf("expected real sandbox auth test to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"command": "authnet auth test"`)
	assertContains(t, stdout, `"profile_name": "sandbox-main"`)
	assertContains(t, stdout, `"environment_classification": "sandbox"`)
	assertContains(t, stdout, `"authenticated": true`)
	assertNotContains(t, stdout, os.Getenv(apiLoginIDEnvName))
	assertNotContains(t, stdout, os.Getenv(transactionKeyEnvName))

	stdout, stderr, err = executeCommand("--json", "sandbox", "charge", "approved", "--amount", "1.23")
	if err != nil {
		t.Fatalf("expected real sandbox charge to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"scenario": "approved"`)
	assertContains(t, stdout, `"card_alias": "visa"`)
}

func TestAuthTestSucceedsWithMockGateway(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newAuthTestServer(t, http.StatusOK, `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "Successful."}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "auth", "test")
	if err != nil {
		t.Fatalf("expected auth test to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"command": "authnet auth test"`)
	assertContains(t, stdout, `"profile_name": "sandbox-main"`)
	assertContains(t, stdout, `"environment_classification": "sandbox"`)
	assertContains(t, stdout, `"authenticated": true`)
	assertContains(t, stdout, `"gateway_message_code": "I00001"`)
	assertNotContains(t, stdout, "secret-login")
	assertNotContains(t, stdout, "secret-key")
}

func TestAuthTestRawResponseEmitsSandboxGatewayResponse(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newAuthTestServer(t, http.StatusOK, `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "secret-login was accepted."}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "--raw-response", "auth", "test")
	if err != nil {
		t.Fatalf("expected raw auth test to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"command": "authnet auth test"`)
	assertContains(t, stdout, `"redacted": false`)
	assertContains(t, stdout, `"raw_gateway_response": {`)
	assertContains(t, stdout, `"text": "secret-login was accepted."`)
	assertNotContains(t, stdout, `"authenticated": true`)
}

func TestAuthTestRawResponseHumanOutputPrintsGatewayResponse(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newAuthTestServer(t, http.StatusOK, `{"messages":{"resultCode":"Ok","message":[{"code":"I00001","text":"secret-login was accepted."}]}}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--raw-response", "auth", "test")
	if err != nil {
		t.Fatalf("expected raw auth test to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"messages":{"resultCode":"Ok"`)
	assertContains(t, stdout, "secret-login was accepted.")
	assertNotContains(t, stdout, "authentication: ok")
}

func TestAuthTestRawResponsePreservesAuthenticationFailureExit(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newAuthTestServer(t, http.StatusOK, `{
		"messages": {
			"resultCode": "Error",
			"message": [{"code": "E00007", "text": "secret-login rejected."}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--raw-response", "auth", "test")
	if err == nil {
		t.Fatal("expected raw auth failure to return a nonzero exit")
	}
	if code != exitAuthFailure {
		t.Fatalf("expected auth failure exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"resultCode": "Error"`)
	assertContains(t, stdout, "secret-login rejected.")
	assertNotContains(t, stdout, "authentication: failed")
}

func TestAuthTestRawResponseJSONRejectsInvalidGatewayJSON(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newAuthTestServer(t, http.StatusOK, `{"messages":{"resultCode":"Ok","message":[{"code":"I00001","text":"Successful."}]}}<html>gateway error</html>`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--json", "--raw-response", "auth", "test")
	if err == nil {
		t.Fatal("expected invalid raw gateway JSON to fail")
	}
	if code != exitGatewayFailure {
		t.Fatalf("expected gateway failure exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "gateway_response_invalid"`)
	assertContains(t, stdout, "Authorize.Net returned an invalid JSON raw gateway response")
	assertNotContains(t, stdout, "<html>gateway error</html>")
}

func TestAuthTestUsesProductionEndpointForProductionProfile(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	sandboxServer := newAuthTestServer(t, http.StatusInternalServerError, `{"messages":{"resultCode":"Error","message":[{"code":"E99999","text":"wrong endpoint"}]}}`)
	productionServer := newAuthTestServer(t, http.StatusOK, `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "Successful."}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, sandboxServer.URL)
	withGatewayTestEndpoint(t, environmentProduction, productionServer.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "prod-main", "--environment", "production")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "--profile", "prod-main", "auth", "test")
	if err != nil {
		t.Fatalf("expected auth test to use production endpoint: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"profile_name": "prod-main"`)
	assertContains(t, stdout, `"environment_classification": "production"`)
	assertContains(t, stdout, `"authenticated": true`)
}

func TestAuthTestMapsAuthenticationFailure(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newAuthTestServer(t, http.StatusOK, `{
		"messages": {
			"resultCode": "Error",
			"message": [{"code": "E00007", "text": "secret-login rejected."}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--json", "auth", "test")
	if err == nil {
		t.Fatal("expected auth failure")
	}
	if code != exitAuthFailure {
		t.Fatalf("expected auth failure exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"authenticated": false`)
	assertContains(t, stdout, `"code": "authentication_failed"`)
	assertContains(t, stdout, `"gateway_message_code": "E00007"`)
	assertContains(t, stdout, redactedValue)
	assertNotContains(t, stdout, "secret-login")
	assertNotContains(t, stdout, "secret-key")
}

func TestAuthTestRejectsOverlengthTransactionKeyBeforeGatewayRequest(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "prod-login")
	t.Setenv(transactionKeyEnvName, "XXXXXXXXXXXXXXXXX")
	var called atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(server.Close)
	withGatewayTestEndpoint(t, environmentProduction, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "prod-fake", "--environment", "production")
	if err != nil {
		t.Fatalf("expected production profile setup to succeed: %v", err)
	}

	stdout, stderr, code, err := executeCommandWithExit("--json", "--profile", "prod-fake", "auth", "test")
	if err == nil {
		t.Fatal("expected overlength transaction key to fail locally")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if called.Load() {
		t.Fatal("overlength transaction key unexpectedly contacted gateway")
	}
	assertContains(t, stdout, `"code": "usage_or_config_error"`)
	assertContains(t, stdout, "transaction key from AUTHNET_TRANSACTION_KEY is too long")
	assertContains(t, stdout, "expected at most 16 characters")
	assertNotContains(t, stdout, "XXXXXXXXXXXXXXXXX")
}

func TestAuthTestTrimsCredentialEnvValuesBeforeValidationAndRequest(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "\tprod-login\n")
	t.Setenv(transactionKeyEnvName, " SECRETKEY1234567\n")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		text := string(body)
		assertContains(t, text, `"name":"prod-login"`)
		assertContains(t, text, `"transactionKey":"SECRETKEY1234567"`)
		assertNotContains(t, text, "\tprod-login")
		assertNotContains(t, text, " SECRETKEY1234567")
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{
			"messages": {
				"resultCode": "Ok",
				"message": [{"code": "I00001", "text": "Successful."}]
			}
		}`))
	}))
	t.Cleanup(server.Close)
	withGatewayTestEndpoint(t, environmentProduction, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "prod-file-sourced", "--environment", "production")
	if err != nil {
		t.Fatalf("expected production profile setup to succeed: %v", err)
	}

	stdout, stderr, err := executeCommand("--json", "--profile", "prod-file-sourced", "auth", "test")
	if err != nil {
		t.Fatalf("expected trimmed credentials to authenticate: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"authenticated": true`)
}

func TestAuthTestRedactsCredentialEchoesInHumanFailureOutput(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "prod-login")
	t.Setenv(transactionKeyEnvName, "SECRETKEY1234567")
	server := newAuthTestServer(t, http.StatusOK, `{
		"messages": {
			"resultCode": "Error",
			"message": [{"code": "E00003", "text": "The 'transactionKey' element is invalid - The value SECRETKEY1234567 is invalid according to its datatype 'String'."}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentProduction, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "prod-fake", "--environment", "production")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--profile", "prod-fake", "auth", "test")
	if err == nil {
		t.Fatal("expected human auth failure to fail")
	}
	if code != exitAuthFailure {
		t.Fatalf("expected auth failure exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, redactedValue)
	assertNotContains(t, stdout, "SECRETKEY1234567")
}

func TestAuthTestRedactsTrimmedCredentialEchoesInJSONFailureOutput(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "prod-login")
	t.Setenv(transactionKeyEnvName, " SECRETKEY1234567\n")
	server := newAuthTestServer(t, http.StatusOK, `{
		"messages": {
			"resultCode": "Error",
			"message": [{"code": "E00003", "text": "The 'transactionKey' element is invalid - The value SECRETKEY1234567 is invalid according to its datatype 'String'."}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentProduction, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "prod-fake", "--environment", "production")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--json", "--profile", "prod-fake", "auth", "test")
	if err == nil {
		t.Fatal("expected JSON auth failure")
	}
	if code != exitAuthFailure {
		t.Fatalf("expected auth failure exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, redactedValue)
	assertNotContains(t, stdout, "SECRETKEY1234567")
}

func TestAuthTestReportsConfigurationFailure(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "")
	t.Setenv(transactionKeyEnvName, "")

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--json", "auth", "test")
	if err == nil {
		t.Fatal("expected missing credentials to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "usage_or_config_error"`)
	assertContains(t, stdout, "missing credential environment variables")
}

func TestAuthTestMapsNetworkTimeout(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	t.Cleanup(server.Close)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)
	originalClient := gatewayHTTPClient
	gatewayHTTPClient = &http.Client{Timeout: time.Millisecond}
	t.Cleanup(func() {
		gatewayHTTPClient = originalClient
	})

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--json", "auth", "test")
	if err == nil {
		t.Fatal("expected timeout to fail")
	}
	if code != exitUnavailable {
		t.Fatalf("expected unavailable exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "gateway_unavailable"`)
	assertContains(t, stdout, "timed out")
	assertNotContains(t, stdout, "secret-login")
	assertNotContains(t, stdout, "secret-key")
}

func TestTransactionGetSucceedsWithMockGateway(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newTransactionTestServer(t, http.StatusOK, "1234567890", `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "Successful."}]
		},
		"transaction": {
			"transId": "1234567890",
			"transactionStatus": "settledSuccessfully",
			"responseCode": 1,
			"responseReasonCode": 1,
			"responseReasonDescription": "secret-login approved.",
			"authCode": "ABC123",
			"AVSResponse": "Y",
			"cardCodeResponse": "M",
			"CAVVResponse": "2",
			"submitTimeUTC": "2026-05-18T01:02:03Z",
			"submitTimeLocal": "2026-05-17T21:02:03",
			"settleAmount": 12.34,
			"accountType": "Visa",
			"accountNumber": "XXXX1111",
			"profile": {
				"customerProfileId": 1001,
				"customerPaymentProfileId": 2002
			},
			"batch": {
				"batchId": 3003,
				"settlementState": "settledSuccessfully",
				"settlementTimeUTC": "2026-05-18T03:00:00Z"
			},
			"billTo": {
				"firstName": "Customer",
				"email": "customer@example.test"
			}
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "transaction", "get", "1234567890")
	if err != nil {
		t.Fatalf("expected transaction get to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"command": "authnet transaction get"`)
	assertContains(t, stdout, `"profile_name": "sandbox-main"`)
	assertContains(t, stdout, `"environment_classification": "sandbox"`)
	assertContains(t, stdout, `"transaction_id": "1234567890"`)
	assertContains(t, stdout, `"transaction_status": "settledSuccessfully"`)
	assertContains(t, stdout, `"response_code": "1"`)
	assertContains(t, stdout, `"settle_amount": "12.34"`)
	assertContains(t, stdout, `"account_number": "XXXX1111"`)
	assertContains(t, stdout, `"customer_profile_id": "1001"`)
	assertContains(t, stdout, `"batch_id": "3003"`)
	assertContains(t, stdout, redactedValue)
	assertNotContains(t, stdout, "secret-login")
	assertNotContains(t, stdout, "secret-key")
	assertNotContains(t, stdout, "customer@example.test")
}

func TestTransactionGetRawResponseEmitsSandboxGatewayResponse(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newTransactionTestServer(t, http.StatusOK, "1234567890", `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "Successful."}]
		},
		"transaction": {
			"transId": "1234567890",
			"transactionStatus": "settledSuccessfully",
			"billTo": {
				"email": "customer@example.test"
			}
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "--raw-response", "transaction", "get", "1234567890")
	if err != nil {
		t.Fatalf("expected raw transaction get to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"command": "authnet transaction get"`)
	assertContains(t, stdout, `"redacted": false`)
	assertContains(t, stdout, `"raw_gateway_response": {`)
	assertContains(t, stdout, `"email": "customer@example.test"`)
	assertNotContains(t, stdout, `"transaction_status": "settledSuccessfully"`)
}

func TestTransactionGetRawResponsePreservesNotFoundExit(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newTransactionTestServer(t, http.StatusOK, "missing-trans", `{
		"messages": {
			"resultCode": "Error",
			"message": [{"code": "E00040", "text": "The record cannot be found."}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--json", "--raw-response", "transaction", "get", "missing-trans")
	if err == nil {
		t.Fatal("expected raw transaction lookup failure to return a nonzero exit")
	}
	if code != exitNotFound {
		t.Fatalf("expected not found exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"redacted": false`)
	assertContains(t, stdout, `"raw_gateway_response": {`)
	assertContains(t, stdout, `"code": "E00040"`)
	assertContains(t, stdout, "The record cannot be found.")
	assertNotContains(t, stdout, `"code": "transaction_not_found"`)
}

func TestTransactionUnsettledListRawResponseEmitsSandboxGatewayPage(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getUnsettledTransactionListRequest"`,
			AlsoWant: []string{
				`"limit":3`,
				`"offset":1`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "9001", "transactionStatus": "capturedPendingSettlement", "submitTimeUTC": "2026-05-18T01:00:00Z", "customer": {"email": "customer@example.test"}}
				]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "--raw-response", "transaction", "unsettled", "list", "--limit", "3")
	if err != nil {
		t.Fatalf("expected raw unsettled transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"command": "authnet transaction unsettled list"`)
	assertContains(t, stdout, `"redacted": false`)
	assertContains(t, stdout, `"raw_gateway_response": {`)
	assertContains(t, stdout, `"transId": "9001"`)
	assertContains(t, stdout, `"email": "customer@example.test"`)
	assertNotContains(t, stdout, `"returned_count"`)
}

func TestTransactionUnsettledListRawResponseUsesPageAndGatewayControls(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getUnsettledTransactionListRequest"`,
			AlsoWant: []string{
				`"status":"pendingApproval"`,
				`"orderBy":"id"`,
				`"orderDescending":false`,
				`"limit":2`,
				`"offset":2`,
			},
			WantOrder: []string{
				`"status":"pendingApproval"`,
				`"sorting":`,
				`"paging":`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "9201", "transactionStatus": "pendingApproval", "submitTimeUTC": "2026-05-18T01:00:00Z"},
					{"transId": "9202", "transactionStatus": "pendingApproval", "submitTimeUTC": "2026-05-18T02:00:00Z"}
				]
			}`,
		},
		{
			Want: `"getUnsettledTransactionListRequest"`,
			AlsoWant: []string{
				`"status":"pendingApproval"`,
				`"orderBy":"id"`,
				`"orderDescending":false`,
				`"limit":2`,
				`"offset":3`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "9301", "transactionStatus": "pendingApproval", "submitTimeUTC": "2026-05-18T03:00:00Z"}
				]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "--raw-response", "transaction", "unsettled", "list", "--limit", "2", "--page", "2", "--sort-by", "transaction_id", "--sort-order", "ascending", "--status", "pendingApproval")
	if err != nil {
		t.Fatalf("expected raw unsettled transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"raw_gateway_response": {`)
	assertContains(t, stdout, `"transId": "9201"`)
	assertContains(t, stdout, `"transId": "9202"`)
	assertNotContains(t, stdout, `"transId": "9301"`)
	assertContains(t, stdout, `"code": "raw_response_more_pages"`)
	assertContains(t, stdout, "--page 3")
}

func TestTransactionUnsettledListRawResponseHumanMorePagesWarningUsesStderr(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getUnsettledTransactionListRequest"`,
			AlsoWant: []string{
				`"limit":1`,
				`"offset":1`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [{"transId": "9101", "transactionStatus": "capturedPendingSettlement"}]
			}`,
		},
		{
			Want: `"getUnsettledTransactionListRequest"`,
			AlsoWant: []string{
				`"limit":1`,
				`"offset":2`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [{"transId": "9102", "transactionStatus": "capturedPendingSettlement"}]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--raw-response", "transaction", "unsettled", "list", "--limit", "1")
	if err != nil {
		t.Fatalf("expected raw unsettled transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"transId": "9101"`)
	assertNotContains(t, stdout, `"transId": "9102"`)
	assertNotContains(t, stdout, "warning:")
	assertContains(t, stderr, "warning: Another raw gateway page is available")
	assertContains(t, stderr, "--page 2")
}

func TestTransactionUnsettledListRawResponseProductionProfileIsDeniedBeforeGatewayRequest(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "prod-login")
	t.Setenv(transactionKeyEnvName, "prod-key")
	var called atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(server.Close)
	withGatewayTestEndpoint(t, environmentProduction, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "prod-main", "--environment", "production")
	if err != nil {
		t.Fatalf("expected production profile setup to succeed: %v", err)
	}

	stdout, stderr, code, err := executeCommandWithExit("--json", "--raw-response", "--profile", "prod-main", "transaction", "unsettled", "list")
	if err == nil {
		t.Fatal("expected production raw-response unsettled list to fail")
	}
	if code != exitSafetyDenied {
		t.Fatalf("expected safety denied exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if called.Load() {
		t.Fatal("production raw-response unsettled list unexpectedly contacted gateway")
	}
	assertContains(t, stdout, `"code": "safety_policy_denied"`)
	assertContains(t, stdout, "raw response mode is unavailable for production-classified profiles")
}

func TestTransactionUnsettledListRawResponseRejectsUnsupportedControls(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		env         map[string]string
		preferences string
		want        []string
	}{
		{
			name: "flag amount sort",
			args: []string{"--sort-by", "amount"},
			want: []string{`unsupported --sort-by value \"amount\" in raw transaction unsettled list mode`, "normalized mode"},
		},
		{
			name: "flag exact transaction status",
			args: []string{"--status", "declined"},
			want: []string{`unsupported --status value \"declined\" in raw transaction unsettled list mode`, "any and pendingApproval"},
		},
		{
			name: "environment amount filter",
			env: map[string]string{
				transactionFilterAmountEnvName: "12.30",
			},
			want: []string{`unsupported AUTHNET_TX_FILTER_AMOUNT value \"12.30\" in raw transaction unsettled list mode`, "normalized mode"},
		},
		{
			name:        "preference payment filter",
			preferences: "transaction_list:\n    filter:\n      payment: Visa XXXX1111\n",
			want:        []string{"unsupported preferences.transaction_list.filter.payment", `value \"Visa XXXX1111\"`, "normalized mode"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configDir := t.TempDir()
			t.Setenv(configEnvName, configDir)
			t.Setenv(apiLoginIDEnvName, "secret-login")
			t.Setenv(transactionKeyEnvName, "secret-key")
			for name, value := range test.env {
				t.Setenv(name, value)
			}
			if test.preferences == "" {
				writeValidProfileConfig(t, configDir)
			} else {
				configText := fmt.Sprintf(`version: 1
default_profile: sandbox-main
profiles:
  - name: sandbox-main
    environment: sandbox
    credential_source:
      type: env
      api_login_id_env: AUTHNET_API_LOGIN_ID
      transaction_key_env: AUTHNET_TRANSACTION_KEY
preferences:
  %s`, test.preferences)
				if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte(configText), 0o600); err != nil {
					t.Fatalf("expected to write profile config fixture: %v", err)
				}
			}
			var called atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called.Store(true)
			}))
			t.Cleanup(server.Close)
			withGatewayTestEndpoint(t, environmentSandbox, server.URL)

			args := append([]string{"--json", "--raw-response", "transaction", "unsettled", "list"}, test.args...)
			stdout, stderr, code, err := executeCommandWithExit(args...)
			if err == nil {
				t.Fatal("expected unsupported raw unsettled list control to fail")
			}
			if code != exitUsageOrConfig {
				t.Fatalf("expected usage/config exit, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			if called.Load() {
				t.Fatal("unsupported raw unsettled list control unexpectedly contacted gateway")
			}
			for _, want := range test.want {
				assertContains(t, stdout, want)
			}
		})
	}
}

func TestTransactionGetUsesProductionEndpointForExplicitProductionProfile(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	sandboxServer := newTransactionTestServer(t, http.StatusInternalServerError, "1234567890", `{"messages":{"resultCode":"Error","message":[{"code":"E99999","text":"wrong endpoint"}]}}`)
	productionServer := newTransactionTestServer(t, http.StatusOK, "1234567890", `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "Successful."}]
		},
		"transaction": {
			"transId": "1234567890",
			"transactionStatus": "capturedPendingSettlement",
			"responseCode": "1",
			"accountType": "MasterCard",
			"accountNumber": "XXXX2222"
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, sandboxServer.URL)
	withGatewayTestEndpoint(t, environmentProduction, productionServer.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "prod-main", "--environment", "production")
	if err != nil {
		t.Fatalf("expected production profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "--profile", "prod-main", "transaction", "get", "1234567890")
	if err != nil {
		t.Fatalf("expected transaction get to use production endpoint: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"profile_name": "prod-main"`)
	assertContains(t, stdout, `"environment_classification": "production"`)
	assertContains(t, stdout, `"transaction_status": "capturedPendingSettlement"`)
}

func TestTransactionGetMapsNotFound(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newTransactionTestServer(t, http.StatusOK, "missing-trans", `{
		"messages": {
			"resultCode": "Error",
			"message": [{"code": "E00040", "text": "The record cannot be found."}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--json", "transaction", "get", "missing-trans")
	if err == nil {
		t.Fatal("expected missing transaction to fail")
	}
	if code != exitNotFound {
		t.Fatalf("expected not found exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "transaction_not_found"`)
	assertContains(t, stdout, `"gateway_message_code": "E00040"`)
}

func TestTransactionGetMapsGatewayFailure(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newTransactionTestServer(t, http.StatusOK, "1234567890", `{
		"messages": {
			"resultCode": "Error",
			"message": [{"code": "E00027", "text": "Gateway validation failed for secret-key."}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--json", "transaction", "get", "1234567890")
	if err == nil {
		t.Fatal("expected gateway failure")
	}
	if code != exitGatewayFailure {
		t.Fatalf("expected gateway failure exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "gateway_failure"`)
	assertContains(t, stdout, redactedValue)
	assertNotContains(t, stdout, "secret-key")
}

func TestTransactionGetHumanOutput(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newTransactionTestServer(t, http.StatusOK, "1234567890", `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "Successful."}]
		},
		"transaction": {
			"transId": "1234567890",
			"transactionStatus": "settledSuccessfully",
			"responseCode": "1",
			"settleAmount": 12.34,
			"accountType": "Visa",
			"accountNumber": "XXXX1111",
			"batch": {"settlementState": "settledSuccessfully"}
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("transaction", "get", "1234567890")
	if err != nil {
		t.Fatalf("expected transaction get to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, "transaction: 1234567890")
	assertContains(t, stdout, "status: settledSuccessfully")
	assertContains(t, stdout, "response: 1")
	assertContains(t, stdout, "settle amount: 12.34")
	assertContains(t, stdout, "payment: Visa XXXX1111")
	assertContains(t, stdout, "settlement: settledSuccessfully")
}

func TestTransactionListResolvesRelativeRangeAndUsesBoundedPagination(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.FixedZone("operator-local", -4*60*60)))
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getSettledBatchListRequest"`,
			AlsoWant: []string{
				`"firstSettlementDate":"2026-05-11T12:00:00-04:00"`,
				`"lastSettlementDate":"2026-05-18T12:00:00-04:00"`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"batchList": [{"batchId": 3003, "settlementState": "settledSuccessfully", "settlementTimeUTC": "2026-05-18T03:00:00Z"}]
			}`,
		},
		{
			Want: `"getTransactionListRequest"`,
			AlsoWant: []string{
				`"batchId":"3003"`,
				`"limit":2`,
				`"offset":1`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [{
					"transId": "1234567890",
					"transactionStatus": "settledSuccessfully",
					"submitTimeUTC": "2026-05-18T01:02:03Z",
					"settleAmount": 12.34,
					"accountType": "Visa",
					"accountNumber": "XXXX1111",
					"billTo": {"email": "customer@example.test"}
				}]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "transaction", "list", "--last", "7d", "--limit", "2")
	if err != nil {
		t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"command": "authnet transaction list"`)
	assertContains(t, stdout, `"kind": "settled"`)
	assertContains(t, stdout, `"from": "2026-05-11T12:00:00-04:00"`)
	assertContains(t, stdout, `"to": "2026-05-18T12:00:00-04:00"`)
	assertContains(t, stdout, `"relative_range": "7d"`)
	assertContains(t, stdout, `"requested_limit": 2`)
	assertContains(t, stdout, `"returned_count": 1`)
	assertContains(t, stdout, `"transaction_id": "1234567890"`)
	assertContains(t, stdout, `"account_number": "XXXX1111"`)
	assertNotContains(t, stdout, "customer@example.test")
	assertNotContains(t, stdout, "secret-login")
	assertNotContains(t, stdout, "secret-key")
}

func TestTransactionListDefaultsToTimestampDescendingJSONOrder(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.FixedZone("operator-local", -4*60*60)))
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getSettledBatchListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"batchList": [
					{"batchId": 3003, "settlementState": "settledSuccessfully", "settlementTimeUTC": "2026-05-18T03:00:00Z"},
					{"batchId": 3004, "settlementState": "settledSuccessfully", "settlementTimeUTC": "2026-05-18T04:00:00Z"}
				]
			}`,
		},
		{
			Want:     `"getTransactionListRequest"`,
			AlsoWant: []string{`"batchId":"3003"`},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "1001", "transactionStatus": "settledSuccessfully", "submitTimeUTC": "2026-05-18T01:00:00Z", "settleAmount": 1.00},
					{"transId": "1003", "transactionStatus": "settledSuccessfully", "submitTimeUTC": "2026-05-18T03:00:00Z", "settleAmount": 3.00}
				]
			}`,
		},
		{
			Want:     `"getTransactionListRequest"`,
			AlsoWant: []string{`"batchId":"3004"`},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "1002", "transactionStatus": "settledSuccessfully", "submitTimeUTC": "2026-05-18T02:00:00Z", "settleAmount": 2.00}
				]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "transaction", "list", "--last", "7d", "--limit", "3")
	if err != nil {
		t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertTransactionIDs(t, stdout, "1003", "1002", "1001")
}

func TestTransactionListAppliesLimitAfterGlobalSort(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC))
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getSettledBatchListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"batchList": [
					{"batchId": 3003, "settlementState": "settledSuccessfully", "settlementTimeUTC": "2026-05-18T03:00:00Z"},
					{"batchId": 3004, "settlementState": "settledSuccessfully", "settlementTimeUTC": "2026-05-18T04:00:00Z"}
				]
			}`,
		},
		{
			Want: `"getTransactionListRequest"`,
			AlsoWant: []string{
				`"batchId":"3003"`,
				`"limit":2`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "1003", "transactionStatus": "settledSuccessfully", "submitTimeUTC": "2026-05-18T03:00:00Z", "settleAmount": 30.00},
					{"transId": "1004", "transactionStatus": "settledSuccessfully", "submitTimeUTC": "2026-05-18T04:00:00Z", "settleAmount": 40.00}
				]
			}`,
		},
		{
			Want: `"getTransactionListRequest"`,
			AlsoWant: []string{
				`"batchId":"3004"`,
				`"limit":2`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "1001", "transactionStatus": "settledSuccessfully", "submitTimeUTC": "2026-05-18T01:00:00Z", "settleAmount": 10.00},
					{"transId": "1002", "transactionStatus": "settledSuccessfully", "submitTimeUTC": "2026-05-18T02:00:00Z", "settleAmount": 20.00}
				]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "transaction", "list", "--last", "7d", "--limit", "2", "--sort-by", "amount", "--sort-order", "ascending")
	if err != nil {
		t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertTransactionIDs(t, stdout, "1001", "1002")
	assertContains(t, stdout, `"returned_count": 2`)
	assertContains(t, stdout, `"has_more": true`)
}

func TestTransactionListFilterFlagsUseAndSemantics(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC))
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getSettledBatchListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"batchList": [{"batchId": 3003, "settlementState": "settledSuccessfully", "settlementTimeUTC": "2026-05-18T03:00:00Z"}]
			}`,
		},
		{
			Want: `"getTransactionListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "1001", "transactionStatus": "declined", "submitTimeUTC": "2026-05-18T01:00:00Z", "settleAmount": 12.30, "accountType": "Visa", "accountNumber": "XXXX1111"},
					{"transId": "1002", "transactionStatus": "settledSuccessfully", "submitTimeUTC": "2026-05-18T02:00:00Z", "settleAmount": 12.30, "accountType": "Visa", "accountNumber": "XXXX1111"},
					{"transId": "1003", "transactionStatus": "declined", "submitTimeUTC": "2026-05-18T03:00:00Z", "settleAmount": 12.31, "accountType": "Visa", "accountNumber": "XXXX1111"},
					{"transId": "1004", "transactionStatus": "declined", "submitTimeUTC": "2026-05-18T04:00:00Z", "settleAmount": 12.30, "accountType": "MasterCard", "accountNumber": "XXXX2222"}
				]
			}`,
		},
		{
			Want: `"getTransactionListRequest"`,
			AlsoWant: []string{
				`"offset":2`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": []
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "transaction", "list", "--last", "7d", "--limit", "4", "--status", "declined", "--amount", "0012.30", "--payment", "Visa XXXX1111")
	if err != nil {
		t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertTransactionIDs(t, stdout, "1001")
	assertContains(t, stdout, `"returned_count": 1`)
	assertContains(t, stdout, `"has_more": false`)
}

func TestTransactionFiltersUseFlagEnvironmentConfigPrecedence(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, configDir string)
		args      []string
		wantTrans string
	}{
		{
			name: "config",
			setup: func(t *testing.T, configDir string) {
				writeTransactionFilterPreferenceConfig(t, configDir, "declined", 12, "Visa XXXX1111")
			},
			args:      []string{"--json", "transaction", "list", "--last", "7d", "--limit", "4"},
			wantTrans: "1001",
		},
		{
			name: "environment overrides config",
			setup: func(t *testing.T, configDir string) {
				writeTransactionFilterPreferenceConfig(t, configDir, "declined", 12, "Visa XXXX1111")
				t.Setenv(transactionFilterStatusEnvName, "settledSuccessfully")
				t.Setenv(transactionFilterAmountEnvName, "2.50")
				t.Setenv(transactionFilterPaymentEnvName, "MasterCard XXXX2222")
			},
			args:      []string{"--json", "transaction", "list", "--last", "7d", "--limit", "4"},
			wantTrans: "1002",
		},
		{
			name: "flags override environment and config",
			setup: func(t *testing.T, configDir string) {
				writeTransactionFilterPreferenceConfig(t, configDir, "declined", 12, "Visa XXXX1111")
				t.Setenv(transactionFilterStatusEnvName, "settledSuccessfully")
				t.Setenv(transactionFilterAmountEnvName, "2.50")
				t.Setenv(transactionFilterPaymentEnvName, "MasterCard XXXX2222")
			},
			args:      []string{"--json", "transaction", "list", "--last", "7d", "--limit", "4", "--status", "refundSettledSuccessfully", "--amount", "3", "--payment", "Discover XXXX3333"},
			wantTrans: "1003",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			configDir := t.TempDir()
			t.Setenv(configEnvName, configDir)
			t.Setenv(apiLoginIDEnvName, "secret-login")
			t.Setenv(transactionKeyEnvName, "secret-key")
			test.setup(t, configDir)
			withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC))
			server := newReportingTestServer(t, []reportingResponse{
				{
					Want: `"getSettledBatchListRequest"`,
					Body: `{
						"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
						"batchList": [{"batchId": 3003, "settlementState": "settledSuccessfully", "settlementTimeUTC": "2026-05-18T03:00:00Z"}]
					}`,
				},
				{
					Want: `"getTransactionListRequest"`,
					Body: `{
						"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
						"transactions": [
							{"transId": "1001", "transactionStatus": "declined", "submitTimeUTC": "2026-05-18T01:00:00Z", "settleAmount": 12.00, "accountType": "Visa", "accountNumber": "XXXX1111"},
							{"transId": "1002", "transactionStatus": "settledSuccessfully", "submitTimeUTC": "2026-05-18T02:00:00Z", "settleAmount": 2.50, "accountType": "MasterCard", "accountNumber": "XXXX2222"},
							{"transId": "1003", "transactionStatus": "refundSettledSuccessfully", "submitTimeUTC": "2026-05-18T03:00:00Z", "settleAmount": 3.00, "accountType": "Discover", "accountNumber": "XXXX3333"}
						]
					}`,
				},
			})
			withGatewayTestEndpoint(t, environmentSandbox, server.URL)

			_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
			if err != nil {
				t.Fatalf("expected profile setup to succeed: %v", err)
			}
			stdout, stderr, err := executeCommand(test.args...)
			if err != nil {
				t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
			}

			assertTransactionIDs(t, stdout, test.wantTrans)
		})
	}
}

func TestTransactionListFilterFetchesAdditionalSettledPages(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC))
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getSettledBatchListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"batchList": [{"batchId": 3003, "settlementState": "settledSuccessfully", "settlementTimeUTC": "2026-05-18T03:00:00Z"}]
			}`,
		},
		{
			Want: `"getTransactionListRequest"`,
			AlsoWant: []string{
				`"limit":2`,
				`"offset":1`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "1001", "transactionStatus": "settledSuccessfully", "submitTimeUTC": "2026-05-18T01:00:00Z", "settleAmount": 1.00},
					{"transId": "1002", "transactionStatus": "settledSuccessfully", "submitTimeUTC": "2026-05-18T02:00:00Z", "settleAmount": 2.00}
				]
			}`,
		},
		{
			Want: `"getTransactionListRequest"`,
			AlsoWant: []string{
				`"limit":2`,
				`"offset":2`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "1003", "transactionStatus": "declined", "submitTimeUTC": "2026-05-18T03:00:00Z", "settleAmount": 3.00},
					{"transId": "1004", "transactionStatus": "declined", "submitTimeUTC": "2026-05-18T04:00:00Z", "settleAmount": 4.00}
				]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "transaction", "list", "--last", "7d", "--limit", "2", "--status", "declined")
	if err != nil {
		t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertTransactionIDs(t, stdout, "1004", "1003")
	assertContains(t, stdout, `"returned_count": 2`)
}

func TestTransactionUnsettledListFilterFetchesAdditionalPages(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getUnsettledTransactionListRequest"`,
			AlsoWant: []string{
				`"limit":2`,
				`"offset":1`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "9001", "transactionStatus": "capturedPendingSettlement", "submitTimeUTC": "2026-05-18T01:00:00Z", "settleAmount": 1.00},
					{"transId": "9002", "transactionStatus": "capturedPendingSettlement", "submitTimeUTC": "2026-05-18T02:00:00Z", "settleAmount": 2.00}
				]
			}`,
		},
		{
			Want: `"getUnsettledTransactionListRequest"`,
			AlsoWant: []string{
				`"limit":2`,
				`"offset":2`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "9003", "transactionStatus": "declined", "submitTimeUTC": "2026-05-18T03:00:00Z", "settleAmount": 3.00},
					{"transId": "9004", "transactionStatus": "declined", "submitTimeUTC": "2026-05-18T04:00:00Z", "settleAmount": 4.00}
				]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "transaction", "unsettled", "list", "--limit", "2", "--status", "declined")
	if err != nil {
		t.Fatalf("expected unsettled transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertTransactionIDs(t, stdout, "9004", "9003")
	assertContains(t, stdout, `"returned_count": 2`)
}

func TestTransactionUnsettledListFilterReportsNoMoreAfterExhaustion(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getUnsettledTransactionListRequest"`,
			AlsoWant: []string{
				`"limit":2`,
				`"offset":1`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "9001", "transactionStatus": "capturedPendingSettlement", "submitTimeUTC": "2026-05-18T01:00:00Z", "settleAmount": 1.00},
					{"transId": "9002", "transactionStatus": "capturedPendingSettlement", "submitTimeUTC": "2026-05-18T02:00:00Z", "settleAmount": 2.00}
				]
			}`,
		},
		{
			Want: `"getUnsettledTransactionListRequest"`,
			AlsoWant: []string{
				`"limit":2`,
				`"offset":2`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "9003", "transactionStatus": "declined", "submitTimeUTC": "2026-05-18T03:00:00Z", "settleAmount": 3.00}
				]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "transaction", "unsettled", "list", "--limit", "2", "--status", "declined")
	if err != nil {
		t.Fatalf("expected unsettled transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertTransactionIDs(t, stdout, "9003")
	assertContains(t, stdout, `"returned_count": 1`)
	assertContains(t, stdout, `"has_more": false`)
}

func TestTransactionUnsettledListAppliesLimitAfterSort(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newUnsettledSortingLimitTestServer(t)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "transaction", "unsettled", "list", "--limit", "3", "--sort-by", "amount", "--sort-order", "ascending")
	if err != nil {
		t.Fatalf("expected unsettled transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertTransactionIDs(t, stdout, "9006", "9005", "9004")
	assertContains(t, stdout, `"returned_count": 3`)
	assertContains(t, stdout, `"has_more": true`)
}

func TestTransactionUnsettledListFilterPreservesCandidateLimitForSort(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newUnsettledSortingLimitTestServer(t)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "transaction", "unsettled", "list", "--limit", "2", "--status", "declined", "--sort-by", "amount", "--sort-order", "ascending")
	if err != nil {
		t.Fatalf("expected unsettled transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertTransactionIDs(t, stdout, "9005", "9002")
	assertContains(t, stdout, `"returned_count": 2`)
	assertContains(t, stdout, `"has_more": true`)
}

func TestTransactionListDateRangeDefaultsToTimestampDescendingJSONOrder(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC))
	server := newReportingTestServer(t, settledSortingResponses())
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "transaction", "list", "--last", "", "--from", "2026-05-11", "--to", "2026-05-18", "--limit", "3")
	if err != nil {
		t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertTransactionIDs(t, stdout, "1003", "1002", "1001")
}

func TestTransactionListSortFlagsOrderHumanRows(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.FixedZone("operator-local", -4*60*60)))
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getSettledBatchListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"batchList": [{"batchId": 3003, "settlementState": "settledSuccessfully", "settlementTimeUTC": "2026-05-18T03:00:00Z"}]
			}`,
		},
		{
			Want: `"getTransactionListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "1003", "transactionStatus": "settledSuccessfully", "submitTimeLocal": "2026-05-18T03:00:00", "settleAmount": 3.00},
					{"transId": "1001", "transactionStatus": "settledSuccessfully", "submitTimeLocal": "2026-05-18T01:00:00", "settleAmount": 1.00},
					{"transId": "1002", "transactionStatus": "settledSuccessfully", "submitTimeLocal": "2026-05-18T02:00:00", "settleAmount": 2.00}
				]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("transaction", "list", "--last", "7d", "--limit", "3", "--sort-by", "amount", "--sort-order", "ascending")
	if err != nil {
		t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContainsInOrder(t, stdout, "1001", "1002", "1003")
}

func TestTransactionSortUsesEnvironmentAndConfigPreferences(t *testing.T) {
	t.Run("environment", func(t *testing.T) {
		t.Setenv(configEnvName, t.TempDir())
		t.Setenv(apiLoginIDEnvName, "secret-login")
		t.Setenv(transactionKeyEnvName, "secret-key")
		t.Setenv(transactionSortByEnvName, "transaction_id")
		t.Setenv(transactionSortOrderEnvName, "ascending")
		withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC))
		server := newReportingTestServer(t, settledSortingResponses())
		withGatewayTestEndpoint(t, environmentSandbox, server.URL)

		_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
		if err != nil {
			t.Fatalf("expected profile setup to succeed: %v", err)
		}
		stdout, stderr, err := executeCommand("--json", "transaction", "list", "--last", "7d", "--limit", "3")
		if err != nil {
			t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}

		assertTransactionIDs(t, stdout, "1001", "1002", "1003")
	})

	t.Run("config", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv(configEnvName, configDir)
		t.Setenv(apiLoginIDEnvName, "secret-login")
		t.Setenv(transactionKeyEnvName, "secret-key")
		writeTransactionSortPreferenceConfig(t, configDir, "amount", "ascending")
		withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC))
		server := newReportingTestServer(t, settledSortingResponses())
		withGatewayTestEndpoint(t, environmentSandbox, server.URL)

		_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
		if err != nil {
			t.Fatalf("expected profile setup to succeed: %v", err)
		}
		stdout, stderr, err := executeCommand("--json", "transaction", "list", "--last", "7d", "--limit", "3")
		if err != nil {
			t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}

		assertTransactionIDs(t, stdout, "1002", "1003", "1001")
	})

	t.Run("flag precedence", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv(configEnvName, configDir)
		t.Setenv(apiLoginIDEnvName, "secret-login")
		t.Setenv(transactionKeyEnvName, "secret-key")
		t.Setenv(transactionSortByEnvName, "transaction_id")
		t.Setenv(transactionSortOrderEnvName, "ascending")
		writeTransactionSortPreferenceConfig(t, configDir, "amount", "ascending")
		withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC))
		server := newReportingTestServer(t, settledSortingResponses())
		withGatewayTestEndpoint(t, environmentSandbox, server.URL)

		_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
		if err != nil {
			t.Fatalf("expected profile setup to succeed: %v", err)
		}
		stdout, stderr, err := executeCommand("--json", "transaction", "list", "--last", "7d", "--limit", "3", "--sort-by", "timestamp", "--sort-order", "descending")
		if err != nil {
			t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}

		assertTransactionIDs(t, stdout, "1003", "1002", "1001")
	})
}

func TestTransactionUnsettledListDefaultsToTimestampDescendingJSONOrder(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getUnsettledTransactionListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "9001", "transactionStatus": "capturedPendingSettlement", "submitTimeUTC": "2026-05-18T01:00:00Z"},
					{"transId": "9003", "transactionStatus": "capturedPendingSettlement", "submitTimeUTC": "2026-05-18T03:00:00Z"},
					{"transId": "9002", "transactionStatus": "capturedPendingSettlement", "submitTimeUTC": "2026-05-18T02:00:00Z"}
				]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "transaction", "unsettled", "list", "--limit", "3")
	if err != nil {
		t.Fatalf("expected unsettled transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertTransactionIDs(t, stdout, "9003", "9002", "9001")
}

func TestTransactionSortValidationRejectsInvalidSources(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T)
		args    []string
		message string
	}{
		{
			name:    "flag sort field",
			setup:   func(t *testing.T) { t.Setenv(configEnvName, t.TempDir()) },
			args:    []string{"--json", "transaction", "list", "--last", "7d", "--sort-by", "status"},
			message: `invalid --sort-by value \"status\": expected timestamp, transaction_id, or amount`,
		},
		{
			name: "environment sort order",
			setup: func(t *testing.T) {
				t.Setenv(configEnvName, t.TempDir())
				t.Setenv(transactionSortOrderEnvName, "sideways")
			},
			args:    []string{"--json", "transaction", "list", "--last", "7d"},
			message: `invalid AUTHNET_TX_SORT_ORDER value \"sideways\": expected ascending or descending`,
		},
		{
			name: "config sort field",
			setup: func(t *testing.T) {
				configDir := t.TempDir()
				t.Setenv(configEnvName, configDir)
				writeTransactionSortPreferenceConfig(t, configDir, "status", "descending")
			},
			args:    []string{"--json", "transaction", "list", "--last", "7d"},
			message: `preferences.transaction_list.sort_by`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.setup(t)

			stdout, _, code, err := executeCommandWithExit(test.args...)
			if err == nil {
				t.Fatal("expected invalid transaction sort configuration to fail")
			}
			if code != exitUsageOrConfig {
				t.Fatalf("expected usage/config exit, got %d\nstdout:\n%s", code, stdout)
			}
			assertContains(t, stdout, test.message)
		})
	}
}

func TestTransactionFilterValidationRejectsInvalidSources(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T)
		args    []string
		message string
	}{
		{
			name:    "flag amount",
			setup:   func(t *testing.T) { t.Setenv(configEnvName, t.TempDir()) },
			args:    []string{"--json", "transaction", "list", "--last", "7d", "--amount", "$1.23"},
			message: `invalid --amount value \"$1.23\": expected positive integer or decimal amount with up to two decimal places`,
		},
		{
			name: "environment payment",
			setup: func(t *testing.T) {
				t.Setenv(configEnvName, t.TempDir())
				t.Setenv(transactionFilterPaymentEnvName, "Visa 1111")
			},
			args:    []string{"--json", "transaction", "list", "--last", "7d"},
			message: `invalid AUTHNET_TX_FILTER_PAYMENT value \"Visa 1111\": expected <CardType> XXXXdddd`,
		},
		{
			name: "official status value",
			setup: func(t *testing.T) {
				t.Setenv(configEnvName, t.TempDir())
			},
			args:    []string{"--json", "transaction", "list", "--last", "7d", "--status", "pendingFinalSettlement"},
			message: `invalid --status`,
		},
		{
			name: "config invalid status",
			setup: func(t *testing.T) {
				configDir := t.TempDir()
				t.Setenv(configEnvName, configDir)
				writeTransactionFilterPreferenceConfig(t, configDir, "pendingReview", "1.23", "Visa XXXX1111")
			},
			args:    []string{"--json", "transaction", "list", "--last", "7d"},
			message: `preferences.transaction_list.filter.status`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.setup(t)

			stdout, _, code, err := executeCommandWithExit(test.args...)
			if test.name == "official status value" {
				if err == nil {
					t.Fatal("expected command to fail before gateway call due to missing profile, after accepting the status")
				}
				assertNotContains(t, stdout, test.message)
				return
			}
			if err == nil {
				t.Fatal("expected invalid transaction filter configuration to fail")
			}
			if code != exitUsageOrConfig {
				t.Fatalf("expected usage/config exit, got %d\nstdout:\n%s", code, stdout)
			}
			assertContains(t, stdout, test.message)
		})
	}
}

func TestUnknownFlagRespectsStructuredOutputMode(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{
			name:    "json after unknown flag",
			args:    []string{"transaction", "unsettled", "list", "--pizza", "--sort-by", "amount", "--sort-order", "ascending", "--json", "--limit", "5"},
			message: `"message": "unknown flag: --pizza"`,
		},
		{
			name:    "automation after unsupported unsettled date range flag",
			args:    []string{"transaction", "unsettled", "list", "--last", "7d", "--automation", "--limit", "5"},
			message: `"message": "no gateway support: --last is incompatible with unsettled transaction list API (no date/time range allowed)"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(configEnvName, t.TempDir())

			stdout, stderr, code, err := executeCommandWithExit(test.args...)
			if err == nil {
				t.Fatal("expected unknown flag to fail")
			}
			if code != exitUsageOrConfig {
				t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected structured output mode to keep stderr empty, got %q", stderr)
			}
			assertContains(t, stdout, `"command": "authnet transaction unsettled list"`)
			assertContains(t, stdout, `"code": "usage_or_config_error"`)
			assertContains(t, stdout, test.message)
		})
	}
}

func TestUnknownFlagExitsWithUsageErrorInHumanMode(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())

	stdout, stderr, code, err := executeCommandWithExit("transaction", "unsettled", "list", "--pizza")
	if err == nil {
		t.Fatal("expected unknown flag to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected human error output to stay on stderr, got stdout:\n%s", stdout)
	}
	assertContains(t, stderr, "unknown flag: --pizza")
}

func TestTransactionSortFallsBackToTransactionIDDescending(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC))
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getSettledBatchListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"batchList": [{"batchId": 3003, "settlementState": "settledSuccessfully", "settlementTimeUTC": "2026-05-18T03:00:00Z"}]
			}`,
		},
		{
			Want: `"getTransactionListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "1001", "transactionStatus": "settledSuccessfully"},
					{"transId": "1003", "transactionStatus": "settledSuccessfully"},
					{"transId": "1002", "transactionStatus": "settledSuccessfully"}
				]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "transaction", "list", "--last", "7d", "--limit", "3")
	if err != nil {
		t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertTransactionIDs(t, stdout, "1003", "1002", "1001")
}

func TestTransactionSortOrdersMissingPrimaryValuesTransitively(t *testing.T) {
	timestampItems := []transactionListItem{
		{TransactionID: "1001", SubmitTimeUTC: "2026-05-18T03:00:00Z"},
		{TransactionID: "1003"},
		{TransactionID: "1002", SubmitTimeUTC: "2026-05-18T01:00:00Z"},
	}
	sortTransactionListItems(timestampItems, transactionSortOptions{By: "timestamp", Order: "descending"})
	assertTransactionItemIDs(t, timestampItems, "1001", "1002", "1003")

	amountItems := []transactionListItem{
		{TransactionID: "2001", SettleAmount: "3.00"},
		{TransactionID: "2003", SettleAmount: "not-a-number"},
		{TransactionID: "2002", SettleAmount: "1.00"},
	}
	sortTransactionListItems(amountItems, transactionSortOptions{By: "amount", Order: "descending"})
	assertTransactionItemIDs(t, amountItems, "2001", "2002", "2003")
}

func TestTransactionListHumanOutputShowsLocalTimestamp(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.FixedZone("operator-local", -4*60*60)))
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getSettledBatchListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"batchList": [{"batchId": 3003, "settlementState": "settledSuccessfully", "settlementTimeUTC": "2026-05-18T03:00:00Z"}]
			}`,
		},
		{
			Want: `"getTransactionListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [{
					"transId": "1234567890",
					"transactionStatus": "settledSuccessfully",
					"submitTimeUTC": "2026-05-18T01:02:03Z",
					"submitTimeLocal": "2026-05-17T21:02:03",
					"settleAmount": 12.34,
					"accountType": "Visa",
					"accountNumber": "XXXX1111"
				}]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("transaction", "list", "--last", "7d", "--limit", "1")
	if err != nil {
		t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, "Transaction")
	assertContains(t, stdout, "Timestamp")
	assertContains(t, stdout, "Status")
	assertContainsInOrder(t, stdout, "Transaction", "Timestamp", "Status")
	assertContains(t, stdout, "1234567890")
	assertContains(t, stdout, "2026-05-17T21:02:03")
	assertNotContains(t, stdout, "2026-05-18T01:02:03Z")
}

func TestTransactionListHumanOutputCanShowUTCTimestamp(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.FixedZone("operator-local", -4*60*60)))
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getSettledBatchListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"batchList": [{"batchId": 3003, "settlementState": "settledSuccessfully", "settlementTimeUTC": "2026-05-18T03:00:00Z"}]
			}`,
		},
		{
			Want: `"getTransactionListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [{
					"transId": "1234567890",
					"transactionStatus": "settledSuccessfully",
					"submitTimeUTC": "2026-05-18T01:02:03Z",
					"submitTimeLocal": "2026-05-17T21:02:03",
					"settleAmount": 12.34,
					"accountType": "Visa",
					"accountNumber": "XXXX1111"
				}]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("transaction", "list", "--last", "7d", "--limit", "1", "--utc")
	if err != nil {
		t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, "Timestamp")
	assertContains(t, stdout, "2026-05-18T01:02:03Z")
	assertNotContains(t, stdout, "2026-05-17T21:02:03")
}

func TestTransactionListUsesOperatorLocalDateRange(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.FixedZone("operator-local", -4*60*60)))
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getSettledBatchListRequest"`,
			AlsoWant: []string{
				`"firstSettlementDate":"2026-05-01T00:00:00-04:00"`,
				`"lastSettlementDate":"2026-05-02T23:59:59-04:00"`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "No records found."}]},
				"batchList": []
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "transaction", "list", "--last", "", "--from", "2026-05-01", "--to", "2026-05-02")
	if err != nil {
		t.Fatalf("expected transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"from": "2026-05-01T00:00:00-04:00"`)
	assertContains(t, stdout, `"to": "2026-05-02T23:59:59-04:00"`)
	assertContains(t, stdout, `"operator_local_time_zone": "operator-local"`)
	assertContains(t, stdout, `"returned_count": 0`)
}

func TestTransactionUnsettledListUsesDistinctGatewayRequest(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getUnsettledTransactionListRequest"`,
			AlsoWant: []string{
				`"limit":1`,
				`"offset":1`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [{
					"transId": "9001",
					"transactionStatus": "capturedPendingSettlement",
					"submitTimeUTC": "2026-05-18T01:02:03Z",
					"submitTimeLocal": "2026-05-17T21:02:03",
					"accountType": "MasterCard",
					"accountNumber": "XXXX2222"
				}]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("transaction", "unsettled", "list", "--limit", "1")
	if err != nil {
		t.Fatalf("expected unsettled transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, "unsettled transactions: 1")
	assertContains(t, stdout, "Transaction")
	assertContains(t, stdout, "Timestamp")
	assertContains(t, stdout, "Status")
	assertContainsInOrder(t, stdout, "Transaction", "Timestamp", "Status")
	assertContains(t, stdout, "Amount")
	assertContains(t, stdout, "Payment")
	assertContains(t, stdout, "9001")
	assertContains(t, stdout, "2026-05-17T21:02:03")
	assertNotContains(t, stdout, "2026-05-18T01:02:03Z")
	assertContains(t, stdout, "capturedPendingSettlement")
	assertContains(t, stdout, "MasterCard XXXX2222")
}

func TestTransactionUnsettledListCanDisplayUTCTimestamps(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newReportingTestServer(t, []reportingResponse{
		{
			Want: `"getUnsettledTransactionListRequest"`,
			AlsoWant: []string{
				`"limit":1`,
				`"offset":1`,
			},
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [{
					"transId": "9001",
					"transactionStatus": "capturedPendingSettlement",
					"submitTimeUTC": "2026-05-18T01:02:03Z",
					"submitTimeLocal": "2026-05-17T21:02:03",
					"accountType": "MasterCard",
					"accountNumber": "XXXX2222"
				}]
			}`,
		},
	})
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("transaction", "unsettled", "list", "--limit", "1", "--utc")
	if err != nil {
		t.Fatalf("expected unsettled transaction list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, "Timestamp")
	assertContains(t, stdout, "2026-05-18T01:02:03Z")
	assertNotContains(t, stdout, "2026-05-17T21:02:03")
}

func settledSortingResponses() []reportingResponse {
	return []reportingResponse{
		{
			Want: `"getSettledBatchListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"batchList": [{"batchId": 3003, "settlementState": "settledSuccessfully", "settlementTimeUTC": "2026-05-18T03:00:00Z"}]
			}`,
		},
		{
			Want: `"getTransactionListRequest"`,
			Body: `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "1003", "transactionStatus": "settledSuccessfully", "submitTimeUTC": "2026-05-18T03:00:00Z", "settleAmount": 2.00},
					{"transId": "1001", "transactionStatus": "settledSuccessfully", "submitTimeUTC": "2026-05-18T01:00:00Z", "settleAmount": 3.00},
					{"transId": "1002", "transactionStatus": "settledSuccessfully", "submitTimeUTC": "2026-05-18T02:00:00Z", "settleAmount": 1.00}
				]
			}`,
		},
	}
}

func TestTransactionListUsesProductionEndpointForExplicitProductionProfile(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	withFixedNow(t, time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC))
	sandboxServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		t.Errorf("production transaction list unexpectedly used sandbox endpoint")
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(sandboxServer.Close)
	productionServer := newReportingTestServer(t, []reportingResponse{{
		Want: `"getSettledBatchListRequest"`,
		Body: `{"messages":{"resultCode":"Ok","message":[{"code":"I00001","text":"No records found."}]},"batchList":[]}`,
	}})
	withGatewayTestEndpoint(t, environmentSandbox, sandboxServer.URL)
	withGatewayTestEndpoint(t, environmentProduction, productionServer.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "prod-main", "--environment", "production")
	if err != nil {
		t.Fatalf("expected production profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "--profile", "prod-main", "transaction", "list", "--last", "1d")
	if err != nil {
		t.Fatalf("expected transaction list to use production endpoint: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"environment_classification": "production"`)
	assertContains(t, stdout, `"production_marker": "PRODUCTION"`)
}

func TestTransactionListRejectsInvalidRangeAndLimit(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--json", "transaction", "list", "--limit", "101")
	if err == nil {
		t.Fatal("expected invalid limit to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "usage_or_config_error"`)
	assertContains(t, stdout, "--limit must be at most 100")

	stdout, stderr, code, err = executeCommandWithExit("--json", "transaction", "list", "--last", "7d", "--from", "2026-05-01")
	if err == nil {
		t.Fatal("expected conflicting time range to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, "--last cannot be combined with --from or --to")
}

func TestCustomerProfileGetDefaultsToMetadataOnly(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newCustomerProfileGetTestServer(t, http.StatusOK, "1001", `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "Successful."}]
		},
		"profile": {
			"customerProfileId": 1001,
			"merchantCustomerId": "merchant-1001",
			"description": "Customer name should not render",
			"email": "customer@example.test",
			"paymentProfiles": [{
				"customerPaymentProfileId": 2002,
				"payment": {"creditCard": {"cardNumber": "XXXX1111", "expirationDate": "XXXX", "cardType": "Visa"}},
				"billTo": {"firstName": "Customer", "email": "billing@example.test"}
			}],
			"shipToList": [{
				"customerAddressId": 3003,
				"firstName": "Ship",
				"address": "123 Main Street"
			}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "customer-profile", "get", "1001")
	if err != nil {
		t.Fatalf("expected customer-profile get to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"command": "authnet customer-profile get"`)
	assertContains(t, stdout, `"profile_name": "sandbox-main"`)
	assertContains(t, stdout, `"environment_classification": "sandbox"`)
	assertContains(t, stdout, `"customer_profile_id": "1001"`)
	assertContains(t, stdout, `"merchant_customer_id": "merchant-1001"`)
	assertContains(t, stdout, `"payment_profile_count": 1`)
	assertContains(t, stdout, `"shipping_address_count": 1`)
	assertNotContains(t, stdout, "customer@example.test")
	assertNotContains(t, stdout, "billing@example.test")
	assertNotContains(t, stdout, "Customer name should not render")
	assertNotContains(t, stdout, "XXXX1111")
	assertNotContains(t, stdout, "123 Main Street")
	assertNotContains(t, stdout, "secret-login")
	assertNotContains(t, stdout, "secret-key")
}

func TestCustomerProfileGetIncludesExplicitNestedRedactedSummaries(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newCustomerProfileGetTestServer(t, http.StatusOK, "1001", `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "Successful."}]
		},
		"profile": {
			"customerProfileId": "1001",
			"merchantCustomerId": "merchant-1001",
			"paymentProfiles": [{
				"customerPaymentProfileId": 2002,
				"payment": {"creditCard": {"cardNumber": "XXXX1111", "cardType": "Visa"}}
			}],
			"shipToList": [{"customerAddressId": 3003, "firstName": "Ship"}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "customer-profile", "get", "1001", "--include-payment-profiles", "--include-shipping-addresses")
	if err != nil {
		t.Fatalf("expected customer-profile get to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	assertContains(t, stdout, `"customer_payment_profile_id": "2002"`)
	assertContains(t, stdout, `"account_number": "XXXX1111"`)
	assertContains(t, stdout, `"customer_address_id": "3003"`)
	assertNotContains(t, stdout, "Ship")
}

func TestCustomerProfileListSucceedsWithMockGateway(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newCustomerProfileListTestServer(t, http.StatusOK, `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "Successful."}]
		},
		"ids": [1001, "1002"]
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("customer-profile", "list")
	if err != nil {
		t.Fatalf("expected human customer-profile list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, "customer profiles: 2")
	assertContains(t, stdout, "Customer Profile")
	assertContains(t, stdout, "1001")
	assertContains(t, stdout, "1002")

	stdout, stderr, err = executeCommand("--json", "customer-profile", "list")
	if err != nil {
		t.Fatalf("expected customer-profile list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"command": "authnet customer-profile list"`)
	assertContains(t, stdout, `"count": 2`)
	assertContains(t, stdout, `"customer_profile_ids": [`)
	assertContains(t, stdout, `"1001"`)
	assertContains(t, stdout, `"1002"`)
}

func TestCustomerProfileGetUsesProductionEndpointForExplicitProductionProfile(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	sandboxServer := newCustomerProfileGetTestServer(t, http.StatusInternalServerError, "1001", `{"messages":{"resultCode":"Error","message":[{"code":"E99999","text":"wrong endpoint"}]}}`)
	productionServer := newCustomerProfileGetTestServer(t, http.StatusOK, "1001", `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "Successful."}]
		},
		"profile": {"customerProfileId": "1001", "merchantCustomerId": "merchant-1001"}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, sandboxServer.URL)
	withGatewayTestEndpoint(t, environmentProduction, productionServer.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "prod-main", "--environment", "production")
	if err != nil {
		t.Fatalf("expected production profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("--json", "--profile", "prod-main", "customer-profile", "get", "1001")
	if err != nil {
		t.Fatalf("expected customer-profile get to use production endpoint: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"profile_name": "prod-main"`)
	assertContains(t, stdout, `"environment_classification": "production"`)
	assertContains(t, stdout, `"production_marker": "PRODUCTION"`)
}

func TestCustomerProfileGetMapsNotFound(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newCustomerProfileGetTestServer(t, http.StatusOK, "missing-profile", `{
		"messages": {
			"resultCode": "Error",
			"message": [{"code": "E00040", "text": "The record cannot be found."}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--json", "customer-profile", "get", "missing-profile")
	if err == nil {
		t.Fatal("expected missing customer profile to fail")
	}
	if code != exitNotFound {
		t.Fatalf("expected not found exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "customer_profile_not_found"`)
	assertContains(t, stdout, `"gateway_message_code": "E00040"`)
}

func TestCustomerProfileGetHumanOutput(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newCustomerProfileGetTestServer(t, http.StatusOK, "1001", `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "Successful."}]
		},
		"profile": {
			"customerProfileId": "1001",
			"merchantCustomerId": "merchant-1001",
			"paymentProfiles": [{"customerPaymentProfileId": "2002"}],
			"shipToList": [{"customerAddressId": "3003"}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v", err)
	}
	stdout, stderr, err := executeCommand("customer-profile", "get", "1001", "--include-payment-profiles", "--include-shipping-addresses")
	if err != nil {
		t.Fatalf("expected customer-profile get to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, "customer profile: 1001")
	assertContains(t, stdout, "merchant customer: merchant-1001")
	assertContains(t, stdout, "payment profiles: 1")
	assertContains(t, stdout, "shipping addresses: 1")
	assertContains(t, stdout, "Payment Profile")
	assertContains(t, stdout, "Shipping Address")
	assertContains(t, stdout, "2002")
	assertContains(t, stdout, "3003")
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
	assertContains(t, stdout, "\x1b[")
	assertNotContains(t, stdout, "\x1b[36m{")
	if len(uniqueANSISequences(stdout)) < 2 {
		t.Fatalf("expected JSON syntax highlighting to use multiple ANSI styles, got:\n%q", stdout)
	}
	assertContains(t, stripANSI(stdout), `"version": "0.1.0-test"`)
}

func TestPersistedColorPreferenceAppliesToJSONOutput(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	configText := `version: 1
profiles: []
preferences:
  color: always
`
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte(configText), 0o600); err != nil {
		t.Fatalf("expected to write profile config fixture: %v", err)
	}

	stdout, _, err := executeCommand("--json", "version")
	if err != nil {
		t.Fatalf("expected persisted color preference to apply: %v", err)
	}
	assertContains(t, stdout, "\x1b[")
	assertContains(t, stripANSI(stdout), `"version": "0.1.0-test"`)
}

func TestColorPreferencePrecedenceUsesFlagsEnvironmentConfigAndAutomation(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	configText := `version: 1
profiles: []
preferences:
  color: always
`
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte(configText), 0o600); err != nil {
		t.Fatalf("expected to write profile config fixture: %v", err)
	}

	t.Setenv("AUTHNET_COLOR", "never")
	stdout, _, err := executeCommand("--json", "version")
	if err != nil {
		t.Fatalf("expected environment color override to succeed: %v", err)
	}
	assertNotContains(t, stdout, "\x1b[")

	stdout, _, err = executeCommand("--json", "--color=always", "version")
	if err != nil {
		t.Fatalf("expected explicit color flag to override environment: %v", err)
	}
	assertContains(t, stdout, "\x1b[")

	stdout, _, err = executeCommand("--json", "--color=always", "--no-color", "version")
	if err != nil {
		t.Fatalf("expected no-color flag to override color preference: %v", err)
	}
	assertNotContains(t, stdout, "\x1b[")

	t.Setenv("AUTHNET_COLOR", "")
	stdout, _, err = executeCommand("--automation", "version")
	if err != nil {
		t.Fatalf("expected automation mode to override color preference: %v", err)
	}
	assertNotContains(t, stdout, "\x1b[")
	assertContains(t, stdout, `"version": "0.1.0-test"`)
}

func TestOutputModePreferencesUseConfigEnvironmentAndFlags(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	configText := `version: 1
profiles: []
preferences:
  json: always
  automation: never
`
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte(configText), 0o600); err != nil {
		t.Fatalf("expected to write profile config fixture: %v", err)
	}

	stdout, stderr, err := executeCommand("version")
	if err != nil {
		t.Fatalf("expected persisted JSON preference to apply: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"command": "authnet version"`)
	assertContains(t, stdout, `"version": "0.1.0-test"`)
	assertNotContains(t, stdout, "\x1b[")
	if stderr != "" {
		t.Fatalf("expected stderr to stay empty, got %q", stderr)
	}

	t.Setenv("AUTHNET_JSON", "false")
	stdout, stderr, err = executeCommand("version")
	if err != nil {
		t.Fatalf("expected environment JSON override to apply: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, "authnet 0.1.0-test")
	assertNotContains(t, stdout, `"command": "authnet version"`)

	stdout, stderr, err = executeCommand("--json", "version")
	if err != nil {
		t.Fatalf("expected --json to override environment false: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"command": "authnet version"`)

	t.Setenv("AUTHNET_JSON", "")
	t.Setenv("AUTHNET_AUTOMATION", "true")
	stdout, stderr, err = executeCommand("version")
	if err != nil {
		t.Fatalf("expected environment automation override to apply: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"command": "authnet version"`)
	assertNotContains(t, stdout, "\x1b[")
}

func TestJSONOverridesAutomationPreferenceAtHigherPrecedenceLayers(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	configText := `version: 1
profiles: []
preferences:
  color: always
  automation: always
`
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte(configText), 0o600); err != nil {
		t.Fatalf("expected to write profile config fixture: %v", err)
	}

	stdout, stderr, err := executeCommand("--json", "version")
	if err != nil {
		t.Fatalf("expected explicit JSON flag to override automation preference: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stripANSI(stdout), `"command": "authnet version"`)
	assertContains(t, stdout, "\x1b[")
	if stderr != "" {
		t.Fatalf("expected stderr to stay empty, got %q", stderr)
	}

	t.Setenv("AUTHNET_JSON", "true")
	stdout, stderr, err = executeCommand("version")
	if err != nil {
		t.Fatalf("expected JSON environment override to override automation preference: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stripANSI(stdout), `"command": "authnet version"`)
	assertContains(t, stdout, "\x1b[")
	if stderr != "" {
		t.Fatalf("expected stderr to stay empty, got %q", stderr)
	}
}

func TestOutputModePreferenceConflictWarnsAndAutomationWins(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	configText := `version: 1
profiles: []
preferences:
  json: always
  automation: always
`
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte(configText), 0o600); err != nil {
		t.Fatalf("expected to write profile config fixture: %v", err)
	}

	stdout, stderr, err := executeCommand("version")
	if err != nil {
		t.Fatalf("expected conflicting output preferences to warn but succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"command": "authnet version"`)
	assertContains(t, stdout, `"code": "output_mode_preference_conflict"`)
	assertContains(t, stdout, "preferences.json and preferences.automation are both always")
	if stderr != "" {
		t.Fatalf("expected JSON preference warning on stdout only, got stderr %q", stderr)
	}

	t.Setenv("AUTHNET_JSON", "false")
	t.Setenv("AUTHNET_AUTOMATION", "false")
	stdout, stderr, err = executeCommand("version")
	if err != nil {
		t.Fatalf("expected env overrides to produce human output with conflict warning: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, "authnet 0.1.0-test")
	assertNotContains(t, stdout, `"command": "authnet version"`)
	assertContains(t, stderr, "warning: preferences.json and preferences.automation are both always")
}

func TestConfigValidateChecksOutputModePreferences(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	configText := `version: 1
profiles: []
preferences:
  json: sometimes
  automation: sideways
`
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte(configText), 0o600); err != nil {
		t.Fatalf("expected to write profile config fixture: %v", err)
	}

	stdout, stderr, code, err := executeCommandWithExit("--json", "config", "validate")
	if err == nil {
		t.Fatal("expected config validate to fail for invalid output mode preferences")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected config validate usage exit, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `invalid preference json \"sometimes\": expected always or never`)
	assertContains(t, stdout, `invalid preference automation \"sideways\": expected always or never`)
}

func TestInvalidOutputModePreferencesFailFastForNormalCommands(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	configText := `version: 1
profiles: []
preferences:
  json: sometimes
  automation: never
`
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte(configText), 0o600); err != nil {
		t.Fatalf("expected to write profile config fixture: %v", err)
	}

	stdout, stderr, code, err := executeCommandWithExit("--json", "version")
	if err == nil {
		t.Fatal("expected invalid JSON preference to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `invalid preferences.json value \"sometimes\"`)
}

func TestInvalidOutputModeEnvironmentOverridesFailFast(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv("AUTHNET_AUTOMATION", "sideways")

	stdout, stderr, code, err := executeCommandWithExit("--json", "version")
	if err == nil {
		t.Fatal("expected invalid automation environment override to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `invalid AUTHNET_AUTOMATION value \"sideways\": expected true or false`)
}

func TestRawResponseWarningsFollowOutputContracts(t *testing.T) {
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	server := newAuthTestServer(t, http.StatusOK, `{"messages":{"resultCode":"Ok","message":[{"code":"I00001","text":"secret-login was accepted."}]}}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	t.Run("bare raw response warns on stderr and preserves stdout", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv(configEnvName, configDir)
		configText := `version: 1
default_profile: sandbox-main
profiles:
  - name: sandbox-main
    environment: sandbox
    credential_source:
      type: env
      api_login_id_env: AUTHNET_API_LOGIN_ID
      transaction_key_env: AUTHNET_TRANSACTION_KEY
preferences:
  json: never
  automation: never
`
		if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte(configText), 0o600); err != nil {
			t.Fatalf("expected to write profile config fixture: %v", err)
		}

		stdout, stderr, err := executeCommand("--raw-response", "auth", "test")
		if err != nil {
			t.Fatalf("expected raw response to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}
		assertContains(t, stdout, `"messages":{"resultCode":"Ok"`)
		assertNotContains(t, stdout, `"warnings"`)
		assertContains(t, stderr, "warning: preferences.json and preferences.automation are set to never in config.yaml but ignored")
	})

	t.Run("json-preferred raw response uses envelope warning", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv(configEnvName, configDir)
		configText := `version: 1
default_profile: sandbox-main
profiles:
  - name: sandbox-main
    environment: sandbox
    credential_source:
      type: env
      api_login_id_env: AUTHNET_API_LOGIN_ID
      transaction_key_env: AUTHNET_TRANSACTION_KEY
preferences:
  json: always
  automation: never
`
		if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte(configText), 0o600); err != nil {
			t.Fatalf("expected to write profile config fixture: %v", err)
		}

		stdout, stderr, err := executeCommand("--raw-response", "auth", "test")
		if err != nil {
			t.Fatalf("expected raw response to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
		}
		assertContains(t, stdout, `"redacted": false`)
		assertContains(t, stdout, `"raw_gateway_response": {`)
		assertContains(t, stdout, `"code": "raw_response_preference_ignored"`)
		assertContains(t, stdout, "preferences.automation is set to never")
		if stderr != "" {
			t.Fatalf("expected JSON raw-response warning on stdout only, got stderr %q", stderr)
		}
	})
}

func TestProfileSetupListValidateAndRemove(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")
	t.Setenv("PROD_LOGIN", "prod-login")
	t.Setenv("PROD_KEY", "prod-key")

	stdout, stderr, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected sandbox profile setup to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"name": "sandbox-main"`)
	assertContains(t, stdout, `"default": true`)

	stdout, stderr, err = executeCommand("--automation", "profile", "setup", "--name", "prod-main", "--environment", "production", "--api-login-id-env", "PROD_LOGIN", "--transaction-key-env", "PROD_KEY")
	if err != nil {
		t.Fatalf("expected production profile setup to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	stdout, _, err = executeCommand("profile", "list")
	if err != nil {
		t.Fatalf("expected profile list to succeed: %v", err)
	}
	assertContains(t, stdout, "Name")
	assertContains(t, stdout, "Environment")
	assertContains(t, stdout, "Credential Source")
	assertContains(t, stdout, "Default")
	assertContains(t, stdout, "sandbox-main")
	assertContains(t, stdout, "default")
	assertContains(t, stdout, "prod-main")
	assertContains(t, stdout, "PRODUCTION")

	stdout, stderr, err = executeCommand("--json", "config", "validate")
	if err != nil {
		t.Fatalf("expected config validate to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"valid": true`)
	assertContains(t, stdout, `"profiles": [`)

	configBytes, err := os.ReadFile(filepath.Join(configDir, profileConfigFileName)) // #nosec G304 - test reads the command output from a t.TempDir config root.
	if err != nil {
		t.Fatalf("expected profile config file to exist: %v", err)
	}
	configText := string(configBytes)
	assertContains(t, configText, "api_login_id_env: AUTHNET_API_LOGIN_ID")
	assertNotContains(t, configText, "secret-login")
	assertNotContains(t, configText, "secret-key")
	assertNotContains(t, configText, "prod-login")
	assertNotContains(t, configText, "prod-key")

	stdout, stderr, err = executeCommand("--automation", "profile", "remove", "--name", "prod-main")
	if err != nil {
		t.Fatalf("expected profile remove to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"removed": true`)
	stdout, _, err = executeCommand("profile", "list")
	if err != nil {
		t.Fatalf("expected profile list after remove to succeed: %v", err)
	}
	assertNotContains(t, stdout, "prod-main")
}

func TestConfigValidateChecksTransactionSortPreferences(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	writeTransactionSortPreferenceConfig(t, configDir, "status", "sideways")

	stdout, stderr, code, err := executeCommandWithExit("--json", "config", "validate")
	if err == nil {
		t.Fatal("expected config validate to fail for invalid transaction sort preferences")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected config validate usage exit, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `invalid preference transaction_list.sort_by \"status\"`)
	assertContains(t, stdout, `invalid preference transaction_list.sort_order \"sideways\"`)
}

func TestConfigValidateChecksTransactionFilterPreferences(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	writeTransactionFilterPreferenceConfig(t, configDir, "pendingReview", "$1.23", "Visa 1111")

	stdout, stderr, code, err := executeCommandWithExit("--json", "config", "validate")
	if err == nil {
		t.Fatal("expected config validate to fail for invalid transaction filter preferences")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected config validate usage exit, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `invalid preference transaction_list.filter.status \"pendingReview\"`)
	assertContains(t, stdout, `invalid preference transaction_list.filter.amount \"$1.23\"`)
	assertContains(t, stdout, `invalid preference transaction_list.filter.payment \"Visa 1111\"`)
}

func TestProfileSetupWritesUnifiedYAMLConfigWithoutSecrets(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "secret-login")
	t.Setenv(transactionKeyEnvName, "secret-key")

	stdout, stderr, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected profile setup to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"config_path": "`+filepath.Join(configDir, "config.yaml")+`"`)

	configBytes, err := os.ReadFile(filepath.Join(configDir, "config.yaml")) // #nosec G304 - test reads the command output from a t.TempDir config root.
	if err != nil {
		t.Fatalf("expected unified YAML config file to exist: %v", err)
	}
	configText := string(configBytes)
	assertContains(t, configText, "version: 1")
	assertContains(t, configText, "default_profile: sandbox-main")
	assertContains(t, configText, "profiles:")
	assertContains(t, configText, "api_login_id_env: AUTHNET_API_LOGIN_ID")
	assertContains(t, configText, "preferences:")
	assertNotContains(t, configText, "secret-login")
	assertNotContains(t, configText, "secret-key")
	if _, err := os.Stat(filepath.Join(configDir, "profiles.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected new writes to avoid profiles.json, stat error: %v", err)
	}
}

func TestLegacyProfilesJSONIsReadAndMigratedOnNextWrite(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "sandbox-secret-login")
	t.Setenv(transactionKeyEnvName, "sandbox-secret-key")
	t.Setenv("PROD_LOGIN", "prod-secret-login")
	t.Setenv("PROD_KEY", "prod-secret-key")

	legacyConfig := `{
  "version": 1,
  "default_profile": "sandbox-main",
  "profiles": [
    {
      "name": "sandbox-main",
      "environment": "sandbox",
      "credential_source": {
        "type": "env",
        "api_login_id_env": "AUTHNET_API_LOGIN_ID",
        "transaction_key_env": "AUTHNET_TRANSACTION_KEY"
      }
    }
  ]
}
`
	if err := os.WriteFile(filepath.Join(configDir, "profiles.json"), []byte(legacyConfig), 0o600); err != nil {
		t.Fatalf("expected to write legacy profile config fixture: %v", err)
	}

	stdout, stderr, err := executeCommand("--json", "profile", "list")
	if err != nil {
		t.Fatalf("expected legacy profile list to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"name": "sandbox-main"`)
	assertContains(t, stdout, `"default": true`)

	stdout, stderr, err = executeCommand("--automation", "profile", "setup", "--name", "prod-main", "--environment", "production", "--api-login-id-env", "PROD_LOGIN", "--transaction-key-env", "PROD_KEY")
	if err != nil {
		t.Fatalf("expected profile setup to migrate legacy config: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"config_path": "`+filepath.Join(configDir, "config.yaml")+`"`)

	configBytes, err := os.ReadFile(filepath.Join(configDir, "config.yaml")) // #nosec G304 - test reads the command output from a t.TempDir config root.
	if err != nil {
		t.Fatalf("expected migrated YAML profile config to exist: %v", err)
	}
	configText := string(configBytes)
	assertContains(t, configText, "default_profile: sandbox-main")
	assertContains(t, configText, "name: sandbox-main")
	assertContains(t, configText, "name: prod-main")
	assertContains(t, configText, "api_login_id_env: PROD_LOGIN")
	assertContains(t, configText, "preferences:")
	assertNotContains(t, configText, "sandbox-secret-login")
	assertNotContains(t, configText, "sandbox-secret-key")
	assertNotContains(t, configText, "prod-secret-login")
	assertNotContains(t, configText, "prod-secret-key")
	assertContains(t, stdout, `"code": "config_migrated"`)
	if _, err := os.Stat(filepath.Join(configDir, legacyProfileConfigFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected legacy profiles.json to be renamed, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, deprecatedProfileConfigFileName)); err != nil {
		t.Fatalf("expected retained legacy backup to exist: %v", err)
	}
}

func TestConfigMigrateMigratesLegacyProfilesJSON(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "sandbox-secret-login")
	t.Setenv(transactionKeyEnvName, "sandbox-secret-key")
	writeLegacyProfileConfig(t, configDir)

	stdout, stderr, err := executeCommand("config", "migrate")
	if err != nil {
		t.Fatalf("expected config migrate to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, "config migration completed")
	assertContains(t, stdout, "active config: "+filepath.Join(configDir, profileConfigFileName))
	assertContains(t, stdout, "legacy backup: "+filepath.Join(configDir, deprecatedProfileConfigFileName))
	assertContains(t, stderr, "warning: Migrated legacy profiles.json to config.yaml; config.yaml is active going forward and DEPRECATED-profiles.json is a retained legacy backup that can be deleted.")

	configBytes, err := os.ReadFile(filepath.Join(configDir, profileConfigFileName)) // #nosec G304 - test reads the command output from a t.TempDir config root.
	if err != nil {
		t.Fatalf("expected migrated YAML profile config to exist: %v", err)
	}
	configText := string(configBytes)
	assertContains(t, configText, "default_profile: sandbox-main")
	assertContains(t, configText, "name: sandbox-main")
	assertNotContains(t, configText, "sandbox-secret-login")
	assertNotContains(t, configText, "sandbox-secret-key")
	if _, err := os.Stat(filepath.Join(configDir, legacyProfileConfigFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected legacy profiles.json to be renamed, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, deprecatedProfileConfigFileName)); err != nil {
		t.Fatalf("expected retained legacy backup to exist: %v", err)
	}
}

func TestConfigMigrateJSONIncludesMetadataAndWarnings(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "sandbox-secret-login")
	t.Setenv(transactionKeyEnvName, "sandbox-secret-key")
	writeLegacyProfileConfig(t, configDir)

	stdout, stderr, err := executeCommand("--json", "config", "migrate")
	if err != nil {
		t.Fatalf("expected JSON config migrate to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertNotContains(t, stderr, "warning:")

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON: %v\noutput:\n%s", err, stdout)
	}
	assertJSONField(t, got, "command", "authnet config migrate")
	assertContains(t, stdout, `"code": "config_migrated"`)
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected object data field, got %#v", got["data"])
	}
	assertJSONField(t, data, "result", "completed")
	assertJSONField(t, data, "message", "Migrated legacy profiles.json to config.yaml")
	assertJSONField(t, data, "original_path", filepath.Join(configDir, legacyProfileConfigFileName))
	assertJSONField(t, data, "active_config", filepath.Join(configDir, profileConfigFileName))
	assertJSONField(t, data, "migrated_path", filepath.Join(configDir, profileConfigFileName))
	assertJSONField(t, data, "backup_path", filepath.Join(configDir, deprecatedProfileConfigFileName))
}

func TestConfigMigrateWithoutConfigReportsNoLegacySource(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)

	stdout, stderr, err := executeCommand("config", "migrate")
	if err != nil {
		t.Fatalf("expected config migrate to succeed without config files: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, "No migration performed: no legacy profiles.json found")
	assertContains(t, stdout, "run authnet profile setup to create config.yaml")
	assertNotContains(t, stdout, "config.yaml already exists")
	assertNotContains(t, stderr, "warning:")
}

func TestConfigMigrateJSONUsesStableResultAndMessage(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)

	stdout, stderr, err := executeCommand("--json", "config", "migrate")
	if err != nil {
		t.Fatalf("expected JSON config migrate to succeed without config files: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertNotContains(t, stderr, "warning:")

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON: %v\noutput:\n%s", err, stdout)
	}
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected object data field, got %#v", got["data"])
	}
	assertJSONField(t, data, "result", "not_needed")
	assertJSONField(t, data, "message", "No migration performed: no legacy profiles.json found; run authnet profile setup to create config.yaml.")
	assertJSONField(t, data, "active_config", "")
	assertJSONField(t, data, "original_path", "")
	assertJSONField(t, data, "migrated_path", "")
	assertJSONField(t, data, "backup_path", "")
}

func TestConfigMigrateWithExistingConfigRenamesStaleLegacyJSON(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	writeValidProfileConfig(t, configDir)
	writeLegacyProfileConfig(t, configDir)
	before, err := os.ReadFile(filepath.Join(configDir, profileConfigFileName)) // #nosec G304 - test reads the command output from a t.TempDir config root.
	if err != nil {
		t.Fatalf("expected to read YAML config fixture: %v", err)
	}

	stdout, stderr, err := executeCommand("config", "migrate")
	if err != nil {
		t.Fatalf("expected config migrate to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, "Migration not needed: config.yaml already exists; consider deleting DEPRECATED-profiles.json")
	assertNotContains(t, stderr, "warning:")
	after, err := os.ReadFile(filepath.Join(configDir, profileConfigFileName)) // #nosec G304 - test reads the command output from a t.TempDir config root.
	if err != nil {
		t.Fatalf("expected to read YAML config fixture: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("expected config.yaml not to be rewritten\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if _, err := os.Stat(filepath.Join(configDir, legacyProfileConfigFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected stale profiles.json to be renamed, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, deprecatedProfileConfigFileName)); err != nil {
		t.Fatalf("expected retained legacy backup to exist: %v", err)
	}
}

func TestConfigMigrateWithExistingConfigAndNoLegacyOmitsLegacyPaths(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	writeValidProfileConfig(t, configDir)

	stdout, stderr, err := executeCommand("config", "migrate")
	if err != nil {
		t.Fatalf("expected config migrate to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, "Migration not needed: config.yaml already exists")
	assertContains(t, stdout, "active config: "+filepath.Join(configDir, profileConfigFileName))
	assertNotContains(t, stdout, "legacy backup:")
	assertNotContains(t, stdout, deprecatedProfileConfigFileName)
	assertNotContains(t, stderr, "warning:")

	stdout, stderr, err = executeCommand("--automation", "config", "migrate")
	if err != nil {
		t.Fatalf("expected automation config migrate to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertNotContains(t, stderr, "warning:")

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("expected valid JSON: %v\noutput:\n%s", err, stdout)
	}
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected object data field, got %#v", got["data"])
	}
	assertJSONField(t, data, "result", "not_needed")
	assertJSONField(t, data, "message", "Migration not needed: config.yaml already exists")
	assertJSONField(t, data, "active_config", filepath.Join(configDir, profileConfigFileName))
	assertJSONField(t, data, "original_path", "")
	assertJSONField(t, data, "migrated_path", "")
	assertJSONField(t, data, "backup_path", "")
}

func TestConfigMigratePreservesExistingDeprecatedBackup(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "sandbox-secret-login")
	t.Setenv(transactionKeyEnvName, "sandbox-secret-key")
	writeLegacyProfileConfig(t, configDir)
	const retainedBackup = `{"profiles":[],"preferences":{"retained":true}}`
	if err := os.WriteFile(filepath.Join(configDir, deprecatedProfileConfigFileName), []byte(retainedBackup), 0o600); err != nil {
		t.Fatalf("expected to write retained deprecated backup fixture: %v", err)
	}
	restoreTimestamp := stubUnixTimestampNow(1779487408)
	defer restoreTimestamp()

	stdout, stderr, err := executeCommand("config", "migrate")
	if err != nil {
		t.Fatalf("expected config migrate to preserve existing backup: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, "config migration completed")
	assertContains(t, stdout, "legacy backup: "+filepath.Join(configDir, "DEPRECATED-1779487408-profiles.json"))
	assertNotContains(t, stderr, "config_migration_backup_rename_failed")

	backupBytes, err := os.ReadFile(filepath.Join(configDir, deprecatedProfileConfigFileName)) // #nosec G304 - test reads the command output from a t.TempDir config root.
	if err != nil {
		t.Fatalf("expected retained deprecated backup to remain readable: %v", err)
	}
	if string(backupBytes) != retainedBackup {
		t.Fatalf("expected retained deprecated backup not to be overwritten\nwant:\n%s\ngot:\n%s", retainedBackup, backupBytes)
	}
	if _, err := os.Stat(filepath.Join(configDir, legacyProfileConfigFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected legacy profiles.json to be renamed, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "DEPRECATED-1779487408-profiles.json")); err != nil {
		t.Fatalf("expected timestamped legacy backup to exist: %v", err)
	}
}

func TestConfigMigrateRetriesTimestampedBackupNameCollision(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "sandbox-secret-login")
	t.Setenv(transactionKeyEnvName, "sandbox-secret-key")
	writeLegacyProfileConfig(t, configDir)
	if err := os.WriteFile(filepath.Join(configDir, deprecatedProfileConfigFileName), []byte(`{"profiles":[]}`), 0o600); err != nil {
		t.Fatalf("expected to write retained deprecated backup fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "DEPRECATED-1779487408-profiles.json"), []byte(`collision`), 0o600); err != nil {
		t.Fatalf("expected to write colliding timestamped backup fixture: %v", err)
	}
	restoreTimestamp := stubUnixTimestampSequence(1779487408, 1779487409)
	defer restoreTimestamp()

	stdout, stderr, err := executeCommand("config", "migrate")
	if err != nil {
		t.Fatalf("expected config migrate to retry timestamped backup collision: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, "legacy backup: "+filepath.Join(configDir, "DEPRECATED-1779487409-profiles.json"))
	assertNotContains(t, stderr, "config_migration_backup_rename_failed")
	if _, err := os.Stat(filepath.Join(configDir, "DEPRECATED-1779487409-profiles.json")); err != nil {
		t.Fatalf("expected retried timestamped legacy backup to exist: %v", err)
	}
	collisionBytes, err := os.ReadFile(filepath.Join(configDir, "DEPRECATED-1779487408-profiles.json")) // #nosec G304 - test reads the command output from a t.TempDir config root.
	if err != nil {
		t.Fatalf("expected colliding timestamped backup to remain readable: %v", err)
	}
	if string(collisionBytes) != "collision" {
		t.Fatalf("expected colliding timestamped backup not to be overwritten, got %q", collisionBytes)
	}
}

func TestConfigMigrateWarnsWhenTimestampedBackupRetriesFail(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "sandbox-secret-login")
	t.Setenv(transactionKeyEnvName, "sandbox-secret-key")
	writeLegacyProfileConfig(t, configDir)
	if err := os.WriteFile(filepath.Join(configDir, deprecatedProfileConfigFileName), []byte(`{"profiles":[]}`), 0o600); err != nil {
		t.Fatalf("expected to write retained deprecated backup fixture: %v", err)
	}
	for _, timestamp := range []int64{1779487408, 1779487409, 1779487410} {
		name := fmt.Sprintf("DEPRECATED-%d-profiles.json", timestamp)
		if err := os.WriteFile(filepath.Join(configDir, name), []byte(name), 0o600); err != nil {
			t.Fatalf("expected to write colliding timestamped backup fixture: %v", err)
		}
	}
	restoreTimestamp := stubUnixTimestampSequence(1779487408, 1779487409, 1779487410)
	defer restoreTimestamp()

	stdout, stderr, err := executeCommand("config", "migrate")
	if err != nil {
		t.Fatalf("expected config migrate to finish with manual-cleanup warning: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stderr, "warning: Could not rename profiles.json to a timestamped deprecated backup after 3 attempts")
	assertContains(t, stderr, "retained profiles.json for manual cleanup")
	if _, err := os.Stat(filepath.Join(configDir, legacyProfileConfigFileName)); err != nil {
		t.Fatalf("expected legacy profiles.json to remain for manual cleanup: %v", err)
	}
}

func TestConfigMigrateRecoveryRequiresAutomationYes(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "sandbox-secret-login")
	t.Setenv(transactionKeyEnvName, "sandbox-secret-key")
	writeDeprecatedProfileConfig(t, configDir)

	stdout, stderr, code, err := executeCommandWithExit("--automation", "config", "migrate")
	if err == nil {
		t.Fatal("expected automation recovery migration without approval to fail")
	}
	if code != exitSafetyDenied {
		t.Fatalf("expected safety denied exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, "authnet --automation --yes config migrate")
	assertContains(t, stdout, `"code": "config_migration_recovery_requires_approval"`)
	if _, err := os.Stat(filepath.Join(configDir, profileConfigFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected unapproved recovery to avoid creating config.yaml, stat error: %v", err)
	}

	stdout, stderr, err = executeCommand("--automation", "--yes", "config", "migrate")
	if err != nil {
		t.Fatalf("expected approved automation recovery migration to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"result": "recovered"`)
	assertContains(t, stdout, `"message": "Recovered config.yaml from DEPRECATED-profiles.json"`)
	assertContains(t, stdout, `"backup_path": "`+filepath.Join(configDir, deprecatedProfileConfigFileName)+`"`)
	assertContains(t, stdout, `"code": "config_recovered"`)
	assertNotContains(t, stdout, "Migrated legacy profiles.json to config.yaml")
	if _, err := os.Stat(filepath.Join(configDir, profileConfigFileName)); err != nil {
		t.Fatalf("expected approved recovery to create config.yaml: %v", err)
	}
}

func TestConfigMigrateWithDeprecatedBackupAndInvalidConfigFailsForConfig(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	writeDeprecatedProfileConfig(t, configDir)
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte("profiles: [\n"), 0o600); err != nil {
		t.Fatalf("expected to write invalid YAML config fixture: %v", err)
	}

	stdout, stderr, code, err := executeCommandWithExit("--automation", "config", "migrate")
	if err == nil {
		t.Fatal("expected migration to fail for invalid config.yaml")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"result": "invalid_existing_config"`)
	assertContains(t, stdout, `"message": "Migration not needed: config.yaml exists but is invalid, check the file"`)
	assertContains(t, stdout, `"code": "config_migration_not_needed_invalid_yaml"`)
	assertNotContains(t, stderr, "warning:")
	if _, err := os.Stat(filepath.Join(configDir, deprecatedProfileConfigFileName)); err != nil {
		t.Fatalf("expected deprecated backup presence not to cause failure or deletion: %v", err)
	}
}

func TestProfileSetupWarnsWhenDeprecatedBackupCanBeRecovered(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "sandbox-secret-login")
	t.Setenv(transactionKeyEnvName, "sandbox-secret-key")
	writeDeprecatedProfileConfig(t, configDir)

	stdout, stderr, err := executeCommand("--automation", "profile", "setup", "--name", "prod-main", "--environment", "production", "--api-login-id-env", "PROD_LOGIN", "--transaction-key-env", "PROD_KEY")
	if err != nil {
		t.Fatalf("expected profile setup to succeed with recoverable backup warning: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "config_migration_recovery_available"`)
	assertContains(t, stdout, "DEPRECATED-profiles.json exists but config.yaml was missing")
	assertContains(t, stdout, "delete config.yaml and run authnet --automation --yes config migrate")
	assertNotContains(t, stderr, "warning:")
	if _, err := os.Stat(filepath.Join(configDir, profileConfigFileName)); err != nil {
		t.Fatalf("expected profile setup to create config.yaml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, deprecatedProfileConfigFileName)); err != nil {
		t.Fatalf("expected deprecated backup to remain available: %v", err)
	}
}

func TestConfigValidateReportsLegacyProfilesJSONSource(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "sandbox-secret-login")
	t.Setenv(transactionKeyEnvName, "sandbox-secret-key")

	legacyConfig := `{
  "version": 1,
  "default_profile": "sandbox-main",
  "profiles": [
    {
      "name": "sandbox-main",
      "environment": "sandbox",
      "credential_source": {
        "type": "env",
        "api_login_id_env": "AUTHNET_API_LOGIN_ID",
        "transaction_key_env": "AUTHNET_TRANSACTION_KEY"
      }
    }
  ]
}
`
	if err := os.WriteFile(filepath.Join(configDir, legacyProfileConfigFileName), []byte(legacyConfig), 0o600); err != nil {
		t.Fatalf("expected to write legacy profile config fixture: %v", err)
	}

	stdout, stderr, err := executeCommand("config", "validate")
	if err != nil {
		t.Fatalf("expected config validate to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, "profile config: "+filepath.Join(configDir, legacyProfileConfigFileName))
	assertNotContains(t, stdout, "profile config: "+filepath.Join(configDir, profileConfigFileName))
	assertContains(t, stderr, "warning: legacy profiles.json is active; run authnet config migrate or update a profile with authnet profile setup to migrate to config.yaml.")
	if _, err := os.Stat(filepath.Join(configDir, profileConfigFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config validate to avoid creating config.yaml, stat error: %v", err)
	}

	stdout, stderr, err = executeCommand("--json", "config", "validate")
	if err != nil {
		t.Fatalf("expected JSON config validate to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"config_path": "`+filepath.Join(configDir, legacyProfileConfigFileName)+`"`)
	assertContains(t, stdout, `"code": "legacy_profile_config_active"`)
	assertNotContains(t, stdout, `"config_path": "`+filepath.Join(configDir, profileConfigFileName)+`"`)
	if _, err := os.Stat(filepath.Join(configDir, profileConfigFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected JSON config validate to avoid creating config.yaml, stat error: %v", err)
	}
}

func TestConfigValidateReportsYAMLConfigSource(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "sandbox-secret-login")
	t.Setenv(transactionKeyEnvName, "sandbox-secret-key")

	configText := `version: 1
default_profile: sandbox-main
profiles:
  - name: sandbox-main
    environment: sandbox
    credential_source:
      type: env
      api_login_id_env: AUTHNET_API_LOGIN_ID
      transaction_key_env: AUTHNET_TRANSACTION_KEY
preferences: {}
`
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte(configText), 0o600); err != nil {
		t.Fatalf("expected to write YAML profile config fixture: %v", err)
	}

	stdout, stderr, err := executeCommand("config", "validate")
	if err != nil {
		t.Fatalf("expected config validate to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, "profile config: "+filepath.Join(configDir, profileConfigFileName))

	stdout, stderr, err = executeCommand("--json", "config", "validate")
	if err != nil {
		t.Fatalf("expected JSON config validate to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stdout, `"config_path": "`+filepath.Join(configDir, profileConfigFileName)+`"`)
}

func TestConfigValidateRejectsMissingActiveConfig(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	if err := os.WriteFile(filepath.Join(configDir, deprecatedProfileConfigFileName), []byte(`{"profiles":[]}`), 0o600); err != nil {
		t.Fatalf("expected to write deprecated profile config fixture: %v", err)
	}

	stdout, stderr, code, err := executeCommandWithExit("config", "validate")
	if err == nil {
		t.Fatal("expected config validate to fail without config.yaml or profiles.json")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, "profile config: "+filepath.Join(configDir, profileConfigFileName)+" (missing)")
	assertContains(t, stdout, "status: invalid")
	assertContains(t, stdout, "expected config.yaml was not found")
	assertNotContains(t, stderr, "warning: no profiles are configured.")

	stdout, stderr, code, err = executeCommandWithExit("--json", "config", "validate")
	if err == nil {
		t.Fatal("expected JSON config validate to fail without config.yaml or profiles.json")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "profile_config_invalid"`)
	assertContains(t, stdout, `"config_path": "`+filepath.Join(configDir, profileConfigFileName)+`"`)
	assertContains(t, stdout, "expected config.yaml was not found")
}

func TestInteractiveProfileSetupPromptsForMissingValues(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())

	stdout, stderr, _, err := executeCommandWithInput("interactive-sandbox\nsandbox\n", "profile", "setup", "--default")
	if err != nil {
		t.Fatalf("expected interactive setup to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertContains(t, stderr, "profile name:")
	assertContains(t, stderr, "environment:")
	assertContains(t, stdout, "profile saved: interactive-sandbox (sandbox) default")
}

func TestProfileSetupRejectsProductionDefault(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())

	stdout, stderr, code, err := executeCommandWithExit("--automation", "profile", "setup", "--name", "prod", "--environment", "production", "--default")
	if err == nil {
		t.Fatal("expected production default setup to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, err.Error(), "must be sandbox-classified")
}

func TestProfileSetupRejectsInvalidEnvironment(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())

	_, _, code, err := executeCommandWithExit("--automation", "profile", "setup", "--name", "bad", "--environment", "staging")
	if err == nil {
		t.Fatal("expected invalid environment to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d", code)
	}
	assertContains(t, err.Error(), "expected sandbox or production")
}

func TestConfigValidateRejectsMalformedYAMLConfig(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("profiles: [\n"), 0o600); err != nil {
		t.Fatalf("expected to write malformed YAML config fixture: %v", err)
	}

	stdout, stderr, code, err := executeCommandWithExit("--json", "config", "validate")
	if err == nil {
		t.Fatal("expected config validate to fail for malformed YAML")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "usage_or_config_error"`)
	assertContains(t, stdout, "profile config is not valid YAML")
}

func TestConfigValidateReportsMissingCredentialSources(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--api-login-id-env", "MISSING_LOGIN", "--transaction-key-env", "MISSING_KEY")
	if err != nil {
		t.Fatalf("expected setup with credential references to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--json", "config", "validate")
	if err == nil {
		t.Fatal("expected config validate to fail with missing credential sources")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "profile_config_invalid"`)
	assertContains(t, stdout, "MISSING_LOGIN, MISSING_KEY")
}

func TestConfigValidateRejectsInvalidColorPreference(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "SENTINEL_LOGIN_VALUE")
	t.Setenv(transactionKeyEnvName, "SENTINEL_TRANSACTION_KEY")
	configText := `version: 1
profiles: []
preferences:
  color: purple
`
	if err := os.WriteFile(filepath.Join(configDir, profileConfigFileName), []byte(configText), 0o600); err != nil {
		t.Fatalf("expected to write profile config fixture: %v", err)
	}

	stdout, stderr, code, err := executeCommandWithExit("--json", "config", "validate")
	if err == nil {
		t.Fatal("expected config validate to fail for invalid color preference")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "profile_config_invalid"`)
	assertContains(t, stdout, "invalid preference color")
	assertNotContains(t, stdout, "SENTINEL_LOGIN_VALUE")
	assertNotContains(t, stdout, "SENTINEL_TRANSACTION_KEY")
}

func TestConfigValidateIgnoresEmptyColorEnvironmentForPreferenceValidation(t *testing.T) {
	configDir := writeInvalidColorPreferenceConfig(t, "123")
	t.Setenv("AUTHNET_COLOR", "")

	stdout, stderr, code, err := executeCommandWithExit("--json", "config", "validate")
	if err == nil {
		t.Fatal("expected config validate to fail for invalid color preference")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "profile_config_invalid"`)
	assertContains(t, stdout, `invalid preference color \"123\"`)
	assertContains(t, stdout, filepath.Join(configDir, profileConfigFileName))
	assertNotContains(t, stdout, "AUTHNET_COLOR")
}

func TestConfigValidateTreatsWhitespaceOnlyCredentialSourcesAsMissing(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv("BLANK_LOGIN", " \n\t")
	t.Setenv("BLANK_KEY", "    ")

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--api-login-id-env", "BLANK_LOGIN", "--transaction-key-env", "BLANK_KEY")
	if err != nil {
		t.Fatalf("expected setup with credential references to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("--json", "config", "validate")
	if err == nil {
		t.Fatal("expected config validate to fail with whitespace-only credential sources")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "profile_config_invalid"`)
	assertContains(t, stdout, "BLANK_LOGIN, BLANK_KEY")
}

func TestConfigValidateHumanInvalidReturnsUsageConfigExitCode(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--api-login-id-env", "MISSING_LOGIN", "--transaction-key-env", "MISSING_KEY")
	if err != nil {
		t.Fatalf("expected setup with credential references to succeed: %v", err)
	}
	stdout, stderr, code, err := executeCommandWithExit("config", "validate")
	if err == nil {
		t.Fatal("expected human config validate to fail with invalid config")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage/config exit code for human config validate report, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, "status: invalid")
	assertContains(t, stdout, "Check")
	assertContains(t, stdout, "Status")
	assertContains(t, stdout, "Message")
	assertContains(t, stdout, "missing credential environment variables: MISSING_LOGIN, MISSING_KEY")
}

func TestPathsUsesConfigDirectoryOverride(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)

	stdout, _, err := executeCommand("--json", "paths")
	if err != nil {
		t.Fatalf("expected paths to succeed: %v", err)
	}
	assertContains(t, stdout, `"config_directory": "`+configDir+`"`)
	assertContains(t, stdout, `"profile_config_file": "`+filepath.Join(configDir, profileConfigFileName)+`"`)
}

func TestEnvironmentOverridesAppearInJSONEnvelope(t *testing.T) {
	t.Setenv(profileEnvName, "env-profile")
	t.Setenv(environmentEnvName, environmentSandbox)

	stdout, _, err := executeCommand("--json", "version")
	if err != nil {
		t.Fatalf("expected version with environment overrides to succeed: %v", err)
	}
	assertContains(t, stdout, `"profile_name": "env-profile"`)
	assertContains(t, stdout, `"environment_classification": "sandbox"`)
}

func TestGlobalProfilePrecedenceUsesFlagsEnvironmentProfileConfigAndDefaults(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "dummy-login")
	t.Setenv(transactionKeyEnvName, "dummy-key")
	sandboxServer := newAuthTestServer(t, http.StatusOK, `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "Successful."}]
		}
	}`)
	productionServer := newAuthTestServer(t, http.StatusOK, `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "Successful."}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, sandboxServer.URL)
	withGatewayTestEndpoint(t, environmentProduction, productionServer.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-default", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected sandbox default setup to succeed: %v", err)
	}
	_, _, err = executeCommand("--automation", "profile", "setup", "--name", "sandbox-env", "--environment", "sandbox")
	if err != nil {
		t.Fatalf("expected sandbox env setup to succeed: %v", err)
	}
	_, _, err = executeCommand("--automation", "profile", "setup", "--name", "prod-flag", "--environment", "production")
	if err != nil {
		t.Fatalf("expected production flag setup to succeed: %v", err)
	}

	stdout, _, err := executeCommand("--json", "auth", "test")
	if err != nil {
		t.Fatalf("expected profile config default to resolve: %v", err)
	}
	assertContains(t, stdout, `"profile_name": "sandbox-default"`)
	assertContains(t, stdout, `"environment_classification": "sandbox"`)

	t.Setenv(profileEnvName, "sandbox-env")
	stdout, _, err = executeCommand("--json", "auth", "test")
	if err != nil {
		t.Fatalf("expected environment profile override to resolve: %v", err)
	}
	assertContains(t, stdout, `"profile_name": "sandbox-env"`)

	stdout, _, err = executeCommand("--json", "--profile", "prod-flag", "auth", "test")
	if err != nil {
		t.Fatalf("expected command-line profile override to resolve: %v", err)
	}
	assertContains(t, stdout, `"profile_name": "prod-flag"`)
	assertContains(t, stdout, `"environment_classification": "production"`)
}

func TestRawResponseProductionProfileIsDeniedBeforeGatewayRequest(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "prod-login")
	t.Setenv(transactionKeyEnvName, "prod-key")
	var called atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(server.Close)
	withGatewayTestEndpoint(t, environmentProduction, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "prod-main", "--environment", "production")
	if err != nil {
		t.Fatalf("expected production profile setup to succeed: %v", err)
	}

	stdout, stderr, code, err := executeCommandWithExit("--json", "--raw-response", "--profile", "prod-main", "auth", "test")
	if err == nil {
		t.Fatal("expected production raw-response auth test to fail")
	}
	if code != exitSafetyDenied {
		t.Fatalf("expected safety denied exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if called.Load() {
		t.Fatal("production raw-response request unexpectedly contacted gateway")
	}
	assertContains(t, stdout, `"code": "safety_policy_denied"`)
	assertContains(t, stdout, "raw response mode is unavailable for production-classified profiles")
}

func TestRawResponseModeRequiresSandboxClassification(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "dummy-login")
	t.Setenv(transactionKeyEnvName, "dummy-key")
	server := newAuthTestServer(t, http.StatusOK, `{
		"messages": {
			"resultCode": "Ok",
			"message": [{"code": "I00001", "text": "Successful."}]
		}
	}`)
	withGatewayTestEndpoint(t, environmentSandbox, server.URL)

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected sandbox profile setup to succeed: %v", err)
	}
	_, _, err = executeCommand("--automation", "profile", "setup", "--name", "prod-main", "--environment", "production")
	if err != nil {
		t.Fatalf("expected production profile setup to succeed: %v", err)
	}

	stdout, _, err := executeCommand("--json", "--raw-response", "--profile", "sandbox-main", "auth", "test")
	if err != nil {
		t.Fatalf("expected sandbox raw-response request to pass safety gate: %v", err)
	}
	assertContains(t, stdout, `"profile_name": "sandbox-main"`)
	assertContains(t, stdout, `"environment_classification": "sandbox"`)

	stdout, stderr, code, err := executeCommandWithExit("--json", "--raw-response", "--profile", "prod-main", "version")
	if err == nil {
		t.Fatal("expected production raw-response request to fail")
	}
	if code != exitSafetyDenied {
		t.Fatalf("expected safety denied exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "safety_policy_denied"`)
	assertContains(t, stdout, "raw response mode is unavailable for production-classified profiles")
}

func TestRawResponseExplicitProductionEnvironmentUsesProductionSpecificSafetyMessage(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(environmentEnvName, environmentProduction)
	t.Setenv(apiLoginIDEnvName, "dummy-login")
	t.Setenv(transactionKeyEnvName, "dummy-key")

	stdout, stderr, code, err := executeCommandWithExit("--json", "--raw-response", "auth", "test")
	if err == nil {
		t.Fatal("expected production raw-response request to fail")
	}
	if code != exitSafetyDenied {
		t.Fatalf("expected safety denied exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "safety_policy_denied"`)
	assertContains(t, stdout, "raw response mode is unavailable for production-classified profiles")
}

func TestRawResponseUnsupportedCommandFailsClearly(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "dummy-login")
	t.Setenv(transactionKeyEnvName, "dummy-key")

	stdout, stderr, code, err := executeCommandWithExit("--json", "--raw-response", "version")
	if err == nil {
		t.Fatal("expected unsupported raw-response command without profile to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "usage_or_config_error"`)
	assertContains(t, stdout, "raw response mode is not supported for authnet version")

	_, _, err = executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected sandbox profile setup to succeed: %v", err)
	}

	stdout, stderr, code, err = executeCommandWithExit("--json", "--raw-response", "--profile", "sandbox-main", "version")
	if err == nil {
		t.Fatal("expected unsupported raw-response command to fail")
	}
	if code != exitUsageOrConfig {
		t.Fatalf("expected usage exit code, got %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	assertContains(t, stdout, `"code": "usage_or_config_error"`)
	assertContains(t, stdout, "raw response mode is not supported for authnet version")
}

func TestRedactionRemovesSyntheticSentinelsFromOutputs(t *testing.T) {
	t.Setenv(configEnvName, t.TempDir())
	t.Setenv(apiLoginIDEnvName, "SENTINEL_LOGIN_VALUE")
	t.Setenv(transactionKeyEnvName, "SENTINEL_TRANSACTION_KEY")

	stdout, stderr, err := executeCommand("--automation", "profile", "setup", "--name", "safe", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected setup to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertNotContains(t, stdout, "SENTINEL_LOGIN_VALUE")
	assertNotContains(t, stdout, "SENTINEL_TRANSACTION_KEY")

	stdout, stderr, err = executeCommand("--json", "config", "validate")
	if err != nil {
		t.Fatalf("expected validate to succeed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	assertNotContains(t, stdout, "SENTINEL_LOGIN_VALUE")
	assertNotContains(t, stdout, "SENTINEL_TRANSACTION_KEY")

	stdout, _, _, err = executeCommandWithExit("--json", "--color=SENTINEL_BAD_COLOR", "version")
	if err == nil {
		t.Fatal("expected invalid color to fail")
	}
	assertContains(t, stdout, redactedValue)
	assertNotContains(t, stdout, "SENTINEL_BAD_COLOR")
}

func TestRedactionRemovesSyntheticSentinelsFromWarnings(t *testing.T) {
	command := NewRootCommand(BuildInfo{SchemaVersion: "0.1.0"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)

	err := renderResult(command, commandResult{
		Warnings: []warning{{
			Code:    "synthetic",
			Message: "warning contains SENTINEL_CUSTOMER_PII",
		}},
		Human: func(writer io.Writer) error {
			_, writeErr := fmt.Fprintln(writer, "safe human output")
			return writeErr
		},
	})
	if err != nil {
		t.Fatalf("expected render to succeed: %v", err)
	}
	assertContains(t, stderr.String(), redactedValue)
	assertNotContains(t, stderr.String(), "SENTINEL_CUSTOMER_PII")
}

func newAuthTestServer(t *testing.T, status int, responseBody string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", request.Method)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected JSON content type, got %q", got)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		text := string(body)
		if !strings.Contains(text, `"authenticateTestRequest"`) {
			t.Errorf("expected authenticateTestRequest body, got %s", text)
		}
		if strings.Contains(text, "SENTINEL") {
			t.Errorf("request body unexpectedly contained sentinel test value: %s", text)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(responseBody))
	}))
	t.Cleanup(server.Close)
	return server
}

func newTransactionTestServer(t *testing.T, status int, transactionID string, responseBody string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", request.Method)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected JSON content type, got %q", got)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		text := string(body)
		if !strings.Contains(text, `"getTransactionDetailsRequest"`) {
			t.Errorf("expected getTransactionDetailsRequest body, got %s", text)
		}
		if !strings.Contains(text, `"transId":"`+transactionID+`"`) {
			t.Errorf("expected transaction ID %q in body, got %s", transactionID, text)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(responseBody))
	}))
	t.Cleanup(server.Close)
	return server
}

func newCustomerProfileGetTestServer(t *testing.T, status int, customerProfileID string, responseBody string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", request.Method)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected JSON content type, got %q", got)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		text := string(body)
		if !strings.Contains(text, `"getCustomerProfileRequest"`) {
			t.Errorf("expected getCustomerProfileRequest body, got %s", text)
		}
		if !strings.Contains(text, `"customerProfileId":"`+customerProfileID+`"`) {
			t.Errorf("expected customer profile ID %q in body, got %s", customerProfileID, text)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(responseBody))
	}))
	t.Cleanup(server.Close)
	return server
}

func newCustomerProfileListTestServer(t *testing.T, status int, responseBody string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", request.Method)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected JSON content type, got %q", got)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		text := string(body)
		if !strings.Contains(text, `"getCustomerProfileIdsRequest"`) {
			t.Errorf("expected getCustomerProfileIdsRequest body, got %s", text)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(responseBody))
	}))
	t.Cleanup(server.Close)
	return server
}

type reportingResponse struct {
	Want      string
	AlsoWant  []string
	WantOrder []string
	Body      string
}

type sandboxRequestExpectation struct {
	Want []string
	Body string
}

func newSandboxTransactionTestServer(t *testing.T, expectations []sandboxRequestExpectation) *httptest.Server {
	t.Helper()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", request.Method)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected JSON content type, got %q", got)
		}
		if requests >= len(expectations) {
			t.Errorf("unexpected extra sandbox transaction request")
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		expectation := expectations[requests]
		requests++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		text := string(body)
		for _, want := range expectation.Want {
			if !strings.Contains(text, want) {
				t.Errorf("expected request body to contain %s, got %s", want, text)
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(expectation.Body))
	}))
	t.Cleanup(func() {
		server.Close()
		if requests != len(expectations) {
			t.Errorf("expected %d sandbox transaction requests, got %d", len(expectations), requests)
		}
	})
	return server
}

func sandboxApprovedResponse(transactionID string, responseCode string, message string, avs string, cvv string) string {
	return fmt.Sprintf(`{"messages":{"resultCode":"Ok","message":[{"code":"I00001","text":"Successful."}]},"transactionResponse":{"responseCode":%q,"transId":%q,"authCode":"ABC123","avsResultCode":%q,"cvvResultCode":%q,"messages":[{"code":"1","description":%q}]}}`, responseCode, transactionID, avs, cvv, message)
}

func newReportingTestServer(t *testing.T, responses []reportingResponse) *httptest.Server {
	t.Helper()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", request.Method)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected JSON content type, got %q", got)
		}
		if requests >= len(responses) {
			t.Errorf("unexpected extra reporting request")
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		response := responses[requests]
		requests++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		text := string(body)
		if !strings.Contains(text, response.Want) {
			t.Errorf("expected request body to contain %s, got %s", response.Want, text)
		}
		for _, want := range response.AlsoWant {
			if !strings.Contains(text, want) {
				t.Errorf("expected request body to contain %s, got %s", want, text)
			}
		}
		previousIndex := -1
		for _, want := range response.WantOrder {
			index := strings.Index(text, want)
			if index < 0 {
				t.Errorf("expected request body to contain ordered token %s, got %s", want, text)
				continue
			}
			if index <= previousIndex {
				t.Errorf("expected request body token %s after previous ordered token, got %s", want, text)
			}
			previousIndex = index
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(response.Body))
	}))
	t.Cleanup(func() {
		server.Close()
		if requests != len(responses) {
			t.Errorf("expected %d reporting requests, got %d", len(responses), requests)
		}
	})
	return server
}

func newUnsettledSortingLimitTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("expected POST request, got %s", request.Method)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected JSON content type, got %q", got)
		}
		requests++
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		text := string(body)
		if !strings.Contains(text, `"getUnsettledTransactionListRequest"`) {
			t.Errorf("expected getUnsettledTransactionListRequest body, got %s", text)
		}

		responseBody := `{
			"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
			"transactions": [
				{"transId": "9006", "transactionStatus": "capturedPendingSettlement", "submitTimeUTC": "2026-05-20T15:21:17Z", "settleAmount": 2.00},
				{"transId": "9002", "transactionStatus": "declined", "submitTimeUTC": "2026-05-20T15:19:41Z", "settleAmount": 5.55},
				{"transId": "9001", "transactionStatus": "declined", "submitTimeUTC": "2026-05-20T15:19:29Z", "settleAmount": 5.55}
			]
		}`
		if strings.Contains(text, `"limit":100`) {
			responseBody = `{
				"messages": {"resultCode": "Ok", "message": [{"code": "I00001", "text": "Successful."}]},
				"transactions": [
					{"transId": "9006", "transactionStatus": "capturedPendingSettlement", "submitTimeUTC": "2026-05-20T15:21:17Z", "settleAmount": 2.00},
					{"transId": "9002", "transactionStatus": "declined", "submitTimeUTC": "2026-05-20T15:19:41Z", "settleAmount": 5.55},
					{"transId": "9001", "transactionStatus": "declined", "submitTimeUTC": "2026-05-20T15:19:29Z", "settleAmount": 5.55},
					{"transId": "9003", "transactionStatus": "capturedPendingSettlement", "submitTimeUTC": "2026-05-20T15:17:34Z", "settleAmount": 4.44},
					{"transId": "9005", "transactionStatus": "declined", "submitTimeUTC": "2026-05-20T15:17:12Z", "settleAmount": 3.21},
					{"transId": "9004", "transactionStatus": "capturedPendingSettlement", "submitTimeUTC": "2026-05-20T15:16:50Z", "settleAmount": 3.21}
				]
			}`
		}

		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(responseBody))
	}))
	t.Cleanup(func() {
		server.Close()
		if requests != 1 {
			t.Errorf("expected 1 unsettled transaction request, got %d", requests)
		}
	})
	return server
}

func withFixedNow(t *testing.T, now time.Time) {
	t.Helper()
	original := nowFunc
	nowFunc = func() time.Time {
		return now
	}
	t.Cleanup(func() {
		nowFunc = original
	})
}

func withGatewayTestEndpoint(t *testing.T, environment string, endpoint string) {
	t.Helper()
	switch environment {
	case environmentSandbox:
		original := sandboxAPIEndpoint
		sandboxAPIEndpoint = endpoint
		t.Cleanup(func() {
			sandboxAPIEndpoint = original
		})
	case environmentProduction:
		original := productionAPIEndpoint
		productionAPIEndpoint = endpoint
		t.Cleanup(func() {
			productionAPIEndpoint = original
		})
	default:
		t.Fatalf("unsupported test endpoint environment %q", environment)
	}
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

func assertContainsInOrder(t *testing.T, text string, values ...string) {
	t.Helper()
	offset := 0
	for _, value := range values {
		index := strings.Index(text[offset:], value)
		if index < 0 {
			t.Fatalf("expected output to contain %q after offset %d\noutput:\n%s", value, offset, text)
		}
		offset += index + len(value)
	}
}

func assertTransactionIDs(t *testing.T, output string, want ...string) {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		t.Fatalf("expected valid JSON output: %v\noutput:\n%s", err, output)
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected JSON data object in %#v", envelope["data"])
	}
	transactions, ok := data["transactions"].([]any)
	if !ok {
		t.Fatalf("expected JSON transactions array in %#v", data["transactions"])
	}
	got := make([]string, 0, len(transactions))
	for _, transaction := range transactions {
		item, ok := transaction.(map[string]any)
		if !ok {
			t.Fatalf("expected JSON transaction object, got %#v", transaction)
		}
		id, ok := item["transaction_id"].(string)
		if !ok {
			t.Fatalf("expected JSON transaction_id string in %#v", item)
		}
		got = append(got, id)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("transaction ID order mismatch\nwant: %v\n got: %v\noutput:\n%s", want, got, output)
	}
}

func assertTransactionItemIDs(t *testing.T, items []transactionListItem, want ...string) {
	t.Helper()
	got := make([]string, 0, len(items))
	for _, item := range items {
		got = append(got, item.TransactionID)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("transaction ID order mismatch\nwant: %v\n got: %v", want, got)
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
