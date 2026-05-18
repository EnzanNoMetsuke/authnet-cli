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

type getTransactionDetailsRequestEnvelope struct {
	Request getTransactionDetailsRequest `json:"getTransactionDetailsRequest"`
}

type getTransactionDetailsRequest struct {
	MerchantAuthentication merchantAuthentication `json:"merchantAuthentication"`
	TransactionID          string                 `json:"transId"`
}

type authenticateTestResponseEnvelope struct {
	Messages gatewayMessages `json:"messages"`
}

type getTransactionDetailsResponseEnvelope struct {
	Messages    gatewayMessages    `json:"messages"`
	Transaction gatewayTransaction `json:"transaction"`
}

type gatewayMessages struct {
	ResultCode string           `json:"resultCode"`
	Message    []gatewayMessage `json:"message"`
}

type gatewayMessage struct {
	Code string `json:"code"`
	Text string `json:"text"`
}

type gatewayString string

func (value *gatewayString) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*value = ""
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*value = gatewayString(text)
		return nil
	}
	*value = gatewayString(trimmed)
	return nil
}

func (value gatewayString) String() string {
	return string(value)
}

type authCredentials struct {
	APILoginID     string
	TransactionKey string
}

type gatewayTransaction struct {
	TransactionID             gatewayString  `json:"transId"`
	TransactionStatus         gatewayString  `json:"transactionStatus"`
	ResponseCode              gatewayString  `json:"responseCode"`
	ResponseReasonCode        gatewayString  `json:"responseReasonCode"`
	ResponseReasonDescription gatewayString  `json:"responseReasonDescription"`
	AuthCode                  gatewayString  `json:"authCode"`
	AVSResponse               gatewayString  `json:"AVSResponse"`
	CardCodeResponse          gatewayString  `json:"cardCodeResponse"`
	CAVVResponse              gatewayString  `json:"CAVVResponse"`
	SubmitTimeUTC             gatewayString  `json:"submitTimeUTC"`
	SubmitTimeLocal           gatewayString  `json:"submitTimeLocal"`
	SettleAmount              gatewayString  `json:"settleAmount"`
	Batch                     gatewayBatch   `json:"batch"`
	Payment                   gatewayPayment `json:"payment"`
	AccountType               gatewayString  `json:"accountType"`
	AccountNumber             gatewayString  `json:"accountNumber"`
	Profile                   gatewayProfile `json:"profile"`
}

type gatewayBatch struct {
	BatchID           gatewayString `json:"batchId"`
	SettlementState   gatewayString `json:"settlementState"`
	SettlementTimeUTC gatewayString `json:"settlementTimeUTC"`
}

type gatewayPayment struct {
	CreditCard gatewayCreditCard `json:"creditCard"`
}

type gatewayCreditCard struct {
	CardNumber     gatewayString `json:"cardNumber"`
	ExpirationDate gatewayString `json:"expirationDate"`
	CardType       gatewayString `json:"cardType"`
}

type gatewayProfile struct {
	CustomerProfileID        gatewayString `json:"customerProfileId"`
	CustomerPaymentProfileID gatewayString `json:"customerPaymentProfileId"`
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
	responseBody, err := client.post(ctx, requestBody, "authentication")
	if err != nil {
		return authenticateTestResponseEnvelope{}, err
	}

	var parsed authenticateTestResponseEnvelope
	if err := decodeGatewayJSON(responseBody, &parsed); err != nil {
		return authenticateTestResponseEnvelope{}, cliError{
			exitCode: exitGatewayFailure,
			code:     "gateway_response_invalid",
			message:  "Authorize.Net returned an invalid authentication response",
		}
	}
	return parsed, nil
}

func (client gatewayClient) getTransactionDetails(ctx context.Context, credentials authCredentials, transactionID string) (getTransactionDetailsResponseEnvelope, error) {
	requestBody := getTransactionDetailsRequestEnvelope{
		Request: getTransactionDetailsRequest{
			MerchantAuthentication: merchantAuthentication{
				Name:           credentials.APILoginID,
				TransactionKey: credentials.TransactionKey,
			},
			TransactionID: transactionID,
		},
	}
	responseBody, err := client.post(ctx, requestBody, "transaction lookup")
	if err != nil {
		return getTransactionDetailsResponseEnvelope{}, err
	}

	var parsed getTransactionDetailsResponseEnvelope
	if err := decodeGatewayJSON(responseBody, &parsed); err != nil {
		return getTransactionDetailsResponseEnvelope{}, cliError{
			exitCode: exitGatewayFailure,
			code:     "gateway_response_invalid",
			message:  "Authorize.Net returned an invalid transaction lookup response",
		}
	}
	return parsed, nil
}

func (client gatewayClient) post(ctx context.Context, requestBody any, operation string) ([]byte, error) {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("encode %s request: %w", operation, err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create %s request: %w", operation, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		if isTimeoutError(err) {
			return nil, cliError{
				exitCode: exitUnavailable,
				code:     "gateway_unavailable",
				message:  fmt.Sprintf("Authorize.Net %s request timed out", operation),
			}
		}
		return nil, cliError{
			exitCode: exitUnavailable,
			code:     "gateway_unavailable",
			message:  fmt.Sprintf("Authorize.Net %s request failed: %v", operation, err),
		}
	}
	defer func() {
		_ = response.Body.Close()
	}()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", operation, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, cliError{
			exitCode: exitGatewayFailure,
			code:     "gateway_failure",
			message:  fmt.Sprintf("Authorize.Net returned HTTP %d", response.StatusCode),
		}
	}
	return responseBody, nil
}

func decodeGatewayJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	decoder.UseNumber()
	return decoder.Decode(target)
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

func loadSelectedProfileWithCredentials(options *globalOptions, commandName string) (selectedProfile, error) {
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
		return selectedProfile{}, newUsageError("%s requires --profile or a configured default profile", commandName)
	}
	for _, profile := range file.Profiles {
		if profile.Name != name {
			continue
		}
		if err := validateProfile(profile); err != nil {
			return selectedProfile{}, err
		}
		credentials, err := credentialsForProfile(profile, commandName)
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

func credentialsForProfile(profile profileEntry, commandName string) (authCredentials, error) {
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
		return authCredentials{}, newUsageError("%s cannot read secure local credential references yet; use env credential source", commandName)
	default:
		return authCredentials{}, newUsageError("invalid credential source type %q: expected env or secure-local-reference", profile.CredentialSource.Type)
	}
}
