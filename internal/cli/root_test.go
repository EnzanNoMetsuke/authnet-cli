package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
	stdout, stderr, code, err := executeCommandWithExit("transaction", "list")
	if err == nil {
		t.Fatal("expected scaffold command to fail")
	}
	if code != exitGeneralFailure {
		t.Fatalf("expected general failure exit code, got %d", code)
	}
	if stdout != "" {
		t.Fatalf("expected non-JSON failure stdout to stay empty, got %q", stdout)
	}
	assertContains(t, stderr, "transaction list is not implemented in this scaffold")
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
	assertContains(t, stdout, "sandbox-main\tsandbox\tenv default")
	assertContains(t, stdout, "prod-main\tproduction\tPRODUCTION\tenv")

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
	assertContains(t, configText, `"api_login_id_env": "AUTHNET_API_LOGIN_ID"`)
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

func TestRawResponseModeRequiresSandboxClassification(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv(configEnvName, configDir)
	t.Setenv(apiLoginIDEnvName, "dummy-login")
	t.Setenv(transactionKeyEnvName, "dummy-key")

	_, _, err := executeCommand("--automation", "profile", "setup", "--name", "sandbox-main", "--environment", "sandbox", "--default")
	if err != nil {
		t.Fatalf("expected sandbox profile setup to succeed: %v", err)
	}
	_, _, err = executeCommand("--automation", "profile", "setup", "--name", "prod-main", "--environment", "production")
	if err != nil {
		t.Fatalf("expected production profile setup to succeed: %v", err)
	}

	stdout, _, err := executeCommand("--json", "--raw-response", "--profile", "sandbox-main", "version")
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
