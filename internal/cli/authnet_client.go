package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultGatewayTimeout = 15 * time.Second

var (
	sandboxAPIEndpoint    = "https://apitest.authorize.net/xml/v1/request.api"
	productionAPIEndpoint = "https://api.authorize.net/xml/v1/request.api"
	gatewayHTTPClient     = &http.Client{Timeout: defaultGatewayTimeout}
)

type gatewayClient struct {
	endpoint   string
	httpClient *http.Client
}

type merchantAuthentication struct {
	Name           string `json:"name"`
	TransactionKey string `json:"transactionKey"`
}

type authenticateTestRequestEnvelope struct {
	Request authenticateTestRequest `json:"authenticateTestRequest"`
}

type authenticateTestRequest struct {
	MerchantAuthentication merchantAuthentication `json:"merchantAuthentication"`
}

type authenticateTestResponseEnvelope struct {
	Messages gatewayMessages `json:"messages"`
}

type gatewayMessages struct {
	ResultCode string           `json:"resultCode"`
	Message    []gatewayMessage `json:"message"`
}

type gatewayMessage struct {
	Code string `json:"code"`
	Text string `json:"text"`
}

type authCredentials struct {
	APILoginID     string
	TransactionKey string
}

type selectedProfile struct {
	Entry       profileEntry
	Credentials authCredentials
}

func newGatewayClient(environment string) (gatewayClient, error) {
	endpoint, err := gatewayEndpointForEnvironment(environment)
	if err != nil {
		return gatewayClient{}, err
	}
	return gatewayClient{
		endpoint:   endpoint,
		httpClient: gatewayHTTPClient,
	}, nil
}

func gatewayEndpointForEnvironment(environment string) (string, error) {
	switch environment {
	case environmentSandbox:
		return sandboxAPIEndpoint, nil
	case environmentProduction:
		return productionAPIEndpoint, nil
	default:
		return "", newUsageError("invalid environment classification %q: expected sandbox or production", environment)
	}
}

func (client gatewayClient) authenticate(ctx context.Context, credentials authCredentials) (authenticateTestResponseEnvelope, error) {
	requestBody := authenticateTestRequestEnvelope{
		Request: authenticateTestRequest{
			MerchantAuthentication: merchantAuthentication{
				Name:           credentials.APILoginID,
				TransactionKey: credentials.TransactionKey,
			},
		},
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return authenticateTestResponseEnvelope{}, fmt.Errorf("encode authentication request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(body))
	if err != nil {
		return authenticateTestResponseEnvelope{}, fmt.Errorf("create authentication request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		if isTimeoutError(err) {
			return authenticateTestResponseEnvelope{}, cliError{
				exitCode: exitUnavailable,
				code:     "gateway_unavailable",
				message:  "Authorize.Net authentication request timed out",
			}
		}
		return authenticateTestResponseEnvelope{}, cliError{
			exitCode: exitUnavailable,
			code:     "gateway_unavailable",
			message:  fmt.Sprintf("Authorize.Net authentication request failed: %v", err),
		}
	}
	defer func() {
		_ = response.Body.Close()
	}()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return authenticateTestResponseEnvelope{}, fmt.Errorf("read authentication response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return authenticateTestResponseEnvelope{}, cliError{
			exitCode: exitGatewayFailure,
			code:     "gateway_failure",
			message:  fmt.Sprintf("Authorize.Net returned HTTP %d", response.StatusCode),
		}
	}

	var parsed authenticateTestResponseEnvelope
	if err := json.Unmarshal(bytes.TrimPrefix(responseBody, []byte("\xef\xbb\xbf")), &parsed); err != nil {
		return authenticateTestResponseEnvelope{}, cliError{
			exitCode: exitGatewayFailure,
			code:     "gateway_response_invalid",
			message:  "Authorize.Net returned an invalid authentication response",
		}
	}
	return parsed, nil
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isTimeoutError(urlErr.Err)
	}
	return false
}

func loadSelectedProfileWithCredentials(options *globalOptions) (selectedProfile, error) {
	store, err := newProfileStore()
	if err != nil {
		return selectedProfile{}, err
	}
	file, err := store.load()
	if err != nil {
		return selectedProfile{}, err
	}
	name := strings.TrimSpace(options.Profile)
	if name == "" {
		name = file.DefaultProfile
	}
	if name == "" {
		return selectedProfile{}, newUsageError("auth test requires --profile or a configured default profile")
	}
	for _, profile := range file.Profiles {
		if profile.Name != name {
			continue
		}
		if err := validateProfile(profile); err != nil {
			return selectedProfile{}, err
		}
		credentials, err := credentialsForProfile(profile)
		if err != nil {
			return selectedProfile{}, err
		}
		options.Profile = profile.Name
		options.Environment = profile.Environment
		return selectedProfile{
			Entry:       profile,
			Credentials: credentials,
		}, nil
	}
	return selectedProfile{}, newUsageError("profile %q does not exist", name)
}

func credentialsForProfile(profile profileEntry) (authCredentials, error) {
	switch profile.CredentialSource.Type {
	case credentialSourceEnv:
		loginID := os.Getenv(profile.CredentialSource.APILoginIDEnv)
		transactionKey := os.Getenv(profile.CredentialSource.TransactionKeyEnv)
		missing := []string{}
		if loginID == "" {
			missing = append(missing, profile.CredentialSource.APILoginIDEnv)
		}
		if transactionKey == "" {
			missing = append(missing, profile.CredentialSource.TransactionKeyEnv)
		}
		if len(missing) > 0 {
			return authCredentials{}, newUsageError("missing credential environment variables: %s", strings.Join(missing, ", "))
		}
		return authCredentials{
			APILoginID:     loginID,
			TransactionKey: transactionKey,
		}, nil
	case credentialSourceSecureRef:
		return authCredentials{}, newUsageError("auth test cannot read secure local credential references yet; use env credential source")
	default:
		return authCredentials{}, newUsageError("invalid credential source type %q: expected env or secure-local-reference", profile.CredentialSource.Type)
	}
}
