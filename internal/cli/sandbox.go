package cli

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	defaultSandboxCardAlias       = "visa"
	defaultSandboxChargeAmount    = "12.34"
	defaultSandboxDuplicateWindow = 120
	maxSandboxDuplicateWindow     = 28800
)

var sandboxAmountPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]{2})?$`)

type sandboxChargeOptions struct {
	Card         string
	CardExplicit bool
	Amount       string
	Variant      string
	Window       int
}

type sandboxCard struct {
	Alias      string
	Number     string
	CardType   string
	CardCode   string
	PostalCode string
}

type sandboxVariant struct {
	Name         string
	PostalCode   string
	CardCode     string
	ExpectedCode string
	Description  string
}

type sandboxChargePlan struct {
	Scenario    string
	Variant     string
	Card        sandboxCard
	Amount      string
	Window      int
	PostalCode  string
	CardCode    string
	Description string
	OrderID     string
}

type sandboxChargeData struct {
	ProfileName               string                 `json:"profile_name"`
	EnvironmentClassification string                 `json:"environment_classification"`
	Scenario                  string                 `json:"scenario"`
	Variant                   string                 `json:"variant,omitempty"`
	CardAlias                 string                 `json:"card_alias"`
	Amount                    string                 `json:"amount"`
	DuplicateWindowSeconds    int                    `json:"duplicate_window_seconds,omitempty"`
	Attempts                  []sandboxChargeAttempt `json:"attempts"`
	GatewayMessageCode        string                 `json:"gateway_message_code,omitempty"`
	Message                   string                 `json:"message,omitempty"`
}

type sandboxChargeAttempt struct {
	Attempt            int    `json:"attempt"`
	TransactionID      string `json:"transaction_id,omitempty"`
	ResponseCode       string `json:"response_code,omitempty"`
	AuthCode           string `json:"auth_code,omitempty"`
	AVSResponse        string `json:"avs_response,omitempty"`
	CardCodeResponse   string `json:"card_code_response,omitempty"`
	GatewayResultCode  string `json:"gateway_result_code,omitempty"`
	GatewayMessageCode string `json:"gateway_message_code,omitempty"`
	Message            string `json:"message,omitempty"`
	DuplicateDetected  bool   `json:"duplicate_detected,omitempty"`
}

var sandboxCards = map[string]sandboxCard{
	"visa": {
		Alias:      "visa",
		Number:     "4111111111111111",
		CardType:   "Visa",
		CardCode:   "900",
		PostalCode: "46220",
	},
	"mastercard": {
		Alias:      "mastercard",
		Number:     "5424000000000015",
		CardType:   "Mastercard",
		CardCode:   "900",
		PostalCode: "46220",
	},
	"amex": {
		Alias:      "amex",
		Number:     "370000000000002",
		CardType:   "American Express",
		CardCode:   "9000",
		PostalCode: "46220",
	},
	"discover": {
		Alias:      "discover",
		Number:     "6011000000000012",
		CardType:   "Discover",
		CardCode:   "900",
		PostalCode: "46220",
	},
}

var sandboxAVSVariants = map[string]sandboxVariant{
	"match": {
		Name:         "match",
		PostalCode:   "46214",
		ExpectedCode: "X",
		Description:  "address match and 9-digit ZIP match",
	},
	"no-match": {
		Name:         "no-match",
		PostalCode:   "46205",
		ExpectedCode: "N",
		Description:  "address no match and ZIP no match",
	},
	"zip-match": {
		Name:         "zip-match",
		PostalCode:   "46217",
		ExpectedCode: "Z",
		Description:  "ZIP match and address no match",
	},
	"address-match": {
		Name:         "address-match",
		PostalCode:   "46201",
		ExpectedCode: "A",
		Description:  "address match and ZIP no match",
	},
	"unavailable": {
		Name:         "unavailable",
		PostalCode:   "46209",
		ExpectedCode: "U",
		Description:  "address information unavailable",
	},
}

var sandboxCVVVariants = map[string]sandboxVariant{
	"match": {
		Name:         "match",
		CardCode:     "900",
		ExpectedCode: "M",
		Description:  "successful card-code match",
	},
	"no-match": {
		Name:         "no-match",
		CardCode:     "901",
		ExpectedCode: "N",
		Description:  "card code does not match",
	},
	"not-processed": {
		Name:         "not-processed",
		CardCode:     "904",
		ExpectedCode: "P",
		Description:  "card code was not processed",
	},
	"should-be-present": {
		Name:         "should-be-present",
		CardCode:     "902",
		ExpectedCode: "S",
		Description:  "card code should be present",
	},
	"issuer-unavailable": {
		Name:         "issuer-unavailable",
		CardCode:     "903",
		ExpectedCode: "U",
		Description:  "issuer unavailable or not certified",
	},
}

func sandboxScenarioShort(scenario string) string {
	switch scenario {
	case "approved":
		return "Submit an approved sandbox card charge"
	case "declined":
		return "Submit a declined sandbox card charge"
	case "avs":
		return "Submit a sandbox card charge for an AVS variant"
	case "cvv":
		return "Submit a sandbox card charge for a CVV variant"
	case "duplicate":
		return "Submit duplicate sandbox card charges"
	default:
		return "Submit a sandbox card charge"
	}
}

func sandboxVariantHelp(scenario string) string {
	switch scenario {
	case "avs":
		return "AVS variant: match, no-match, zip-match, address-match, unavailable"
	case "cvv":
		return "CVV variant: match, no-match, not-processed, should-be-present, issuer-unavailable"
	default:
		return "variant"
	}
}

func runSandboxCharge(cmd *cobra.Command, scenario string, options *sandboxChargeOptions) error {
	plan, err := newSandboxChargePlan(scenario, options)
	if err != nil {
		return err
	}

	global := optionsFromCommand(cmd)
	profile, err := loadSelectedProfileWithCredentials(global, "sandbox charge "+scenario)
	if err != nil {
		return err
	}
	if profile.Entry.Environment != environmentSandbox {
		return newSafetyDeniedError("sandbox charge helpers require a sandbox-classified profile")
	}

	client, err := newGatewayClient(profile.Entry.Environment)
	if err != nil {
		return err
	}
	request := plan.gatewayRequest()
	attempts := 1
	if plan.Scenario == "duplicate" {
		attempts = 2
	}

	data := sandboxChargeData{
		ProfileName:               profile.Entry.Name,
		EnvironmentClassification: profile.Entry.Environment,
		Scenario:                  plan.Scenario,
		Variant:                   plan.Variant,
		CardAlias:                 plan.Card.Alias,
		Amount:                    plan.Amount,
		DuplicateWindowSeconds:    plan.Window,
		Attempts:                  []sandboxChargeAttempt{},
	}
	if plan.Scenario != "duplicate" {
		data.DuplicateWindowSeconds = 0
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		response, err := client.createTransaction(cmd.Context(), profile.Credentials, request)
		if err != nil {
			return err
		}
		item := sandboxAttemptFromResponse(attempt, response)
		if plan.Scenario == "duplicate" && attempt == 2 {
			item.DuplicateDetected = isDuplicateResponse(item)
		}
		data.Attempts = append(data.Attempts, item)
		if !strings.EqualFold(response.Messages.ResultCode, "Ok") && plan.Scenario != "duplicate" {
			data.GatewayMessageCode = item.GatewayMessageCode
			data.Message = item.Message
			return renderSandboxChargeFailure(cmd, response.Messages.ResultCode, data)
		}
	}

	return renderSandboxChargeResult(cmd, data)
}

func newSandboxChargePlan(scenario string, options *sandboxChargeOptions) (sandboxChargePlan, error) {
	cardAlias := strings.ToLower(strings.TrimSpace(options.Card))
	if cardAlias == "" {
		cardAlias = defaultSandboxCardAlias
	}
	card, ok := sandboxCards[cardAlias]
	if !ok {
		return sandboxChargePlan{}, newUsageError("unknown sandbox card alias %q: expected visa, mastercard, amex, or discover", options.Card)
	}
	amount := strings.TrimSpace(options.Amount)
	if amount == "" {
		amount = defaultSandboxChargeAmount
	}
	if !sandboxAmountPattern.MatchString(amount) {
		return sandboxChargePlan{}, newUsageError("invalid --amount value %q: expected a positive decimal amount such as 12.34", options.Amount)
	}
	plan := sandboxChargePlan{
		Scenario:    scenario,
		Card:        card,
		Amount:      amount,
		Window:      options.Window,
		PostalCode:  card.PostalCode,
		CardCode:    card.CardCode,
		Description: "authnet sandbox " + scenario,
		OrderID:     "authnet-" + scenario,
	}
	switch scenario {
	case "approved":
	case "declined":
		plan.PostalCode = "46282"
	case "avs":
		variant, err := lookupSandboxVariant(options.Variant, sandboxAVSVariants, "avs", "match, no-match, zip-match, address-match, unavailable")
		if err != nil {
			return sandboxChargePlan{}, err
		}
		if variant.Name == "match" && (card.Alias == "visa" || card.Alias == "amex") {
			if options.CardExplicit {
				return sandboxChargePlan{}, newUsageError("AVS variant %q is not applicable to card alias %q; use mastercard or discover", variant.Name, card.Alias)
			}
			card = sandboxCards["mastercard"]
			plan.Card = card
		}
		plan.Variant = variant.Name
		plan.PostalCode = variant.PostalCode
		plan.Description = "authnet sandbox avs " + variant.Name
		plan.OrderID = "authnet-avs-" + variant.Name
	case "cvv":
		variant, err := lookupSandboxVariant(options.Variant, sandboxCVVVariants, "cvv", "match, no-match, not-processed, should-be-present, issuer-unavailable")
		if err != nil {
			return sandboxChargePlan{}, err
		}
		plan.Variant = variant.Name
		plan.CardCode = cvvCodeForCard(card, variant.CardCode)
		plan.Description = "authnet sandbox cvv " + variant.Name
		plan.OrderID = "authnet-cvv-" + variant.Name
	case "duplicate":
		if options.Window < 1 || options.Window > maxSandboxDuplicateWindow {
			return sandboxChargePlan{}, newUsageError("--window must be between 1 and %d seconds", maxSandboxDuplicateWindow)
		}
		plan.Description = "authnet sandbox duplicate"
		plan.OrderID = "an-dup-" + strconv.FormatInt(nowFunc().UnixNano(), 36)
	default:
		return sandboxChargePlan{}, newUsageError("unknown sandbox charge scenario %q", scenario)
	}
	return plan, nil
}

func lookupSandboxVariant(value string, variants map[string]sandboxVariant, scenario string, expected string) (sandboxVariant, error) {
	name := strings.ToLower(strings.TrimSpace(value))
	if name == "" {
		return sandboxVariant{}, newUsageError("sandbox charge %s requires --variant: expected %s", scenario, expected)
	}
	variant, ok := variants[name]
	if !ok {
		return sandboxVariant{}, newUsageError("unknown sandbox charge %s variant %q: expected %s", scenario, value, expected)
	}
	return variant, nil
}

func cvvCodeForCard(card sandboxCard, code string) string {
	if card.Alias == "amex" && len(code) == 3 {
		return code + "0"
	}
	return code
}

func (plan sandboxChargePlan) gatewayRequest() gatewayTransactionRequest {
	request := gatewayTransactionRequest{
		TransactionType: "authCaptureTransaction",
		Amount:          plan.Amount,
		Payment: gatewayRequestPayment{
			CreditCard: gatewayRequestCreditCard{
				CardNumber:     plan.Card.Number,
				ExpirationDate: nowFunc().AddDate(1, 0, 0).Format("2006-01"),
				CardCode:       plan.CardCode,
			},
		},
		Order: gatewayRequestOrder{
			InvoiceNumber: plan.OrderID,
			Description:   plan.Description,
		},
		BillTo: gatewayRequestBillTo{
			Zip: plan.PostalCode,
		},
	}
	if plan.Scenario == "duplicate" {
		request.TransactionSettings = &gatewayTransactionSettings{
			Settings: []gatewayTransactionSetting{{
				Name:  "duplicateWindow",
				Value: fmt.Sprintf("%d", plan.Window),
			}},
		}
	}
	return request
}

func sandboxAttemptFromResponse(attempt int, response createTransactionResponseEnvelope) sandboxChargeAttempt {
	message := firstGatewayMessage(response.Messages.Message)
	transactionMessage := firstTransactionMessage(response.TransactionResponse.Messages)
	if transactionMessage.isEmpty() {
		transactionMessage = firstTransactionMessage(response.TransactionResponse.Errors)
	}
	code := firstNonEmpty(transactionMessage.Code, transactionMessage.ErrorCode, message.Code)
	text := firstNonEmpty(transactionMessage.Description, transactionMessage.ErrorText, transactionMessage.Text, message.Text)
	return sandboxChargeAttempt{
		Attempt:            attempt,
		TransactionID:      response.TransactionResponse.TransactionID,
		ResponseCode:       response.TransactionResponse.ResponseCode,
		AuthCode:           response.TransactionResponse.AuthCode,
		AVSResponse:        response.TransactionResponse.AVSResultCode,
		CardCodeResponse:   response.TransactionResponse.CVVResultCode,
		GatewayResultCode:  response.Messages.ResultCode,
		GatewayMessageCode: code,
		Message:            text,
	}
}

func (message gatewayTransactionMessage) isEmpty() bool {
	return message.Code == "" &&
		message.Description == "" &&
		message.ErrorCode == "" &&
		message.ErrorText == "" &&
		message.Text == ""
}

func firstTransactionMessage(messages []gatewayTransactionMessage) gatewayTransactionMessage {
	if len(messages) == 0 {
		return gatewayTransactionMessage{}
	}
	return messages[0]
}

func isDuplicateResponse(attempt sandboxChargeAttempt) bool {
	text := strings.ToLower(attempt.Message)
	return attempt.GatewayMessageCode == "11" ||
		strings.Contains(text, "duplicate") ||
		strings.Contains(text, "transaction has been submitted")
}

func renderSandboxChargeResult(cmd *cobra.Command, data sandboxChargeData) error {
	data = sanitizeForOutput(data).(sandboxChargeData)
	return renderResult(cmd, commandResult{
		Data: data,
		Human: func(writer io.Writer) error {
			if _, err := fmt.Fprintf(writer, "sandbox charge: %s\nenvironment: %s\ncard: %s\namount: %s\n",
				data.Scenario,
				data.EnvironmentClassification,
				data.CardAlias,
				data.Amount,
			); err != nil {
				return err
			}
			if data.Variant != "" {
				if _, err := fmt.Fprintf(writer, "variant: %s\n", data.Variant); err != nil {
					return err
				}
			}
			if data.DuplicateWindowSeconds > 0 {
				if _, err := fmt.Fprintf(writer, "duplicate window: %ds\n", data.DuplicateWindowSeconds); err != nil {
					return err
				}
			}
			for _, attempt := range data.Attempts {
				if _, err := fmt.Fprintf(writer, "attempt %d: response %s transaction %s message %s\n",
					attempt.Attempt,
					firstNonEmpty(attempt.ResponseCode, "(none)"),
					firstNonEmpty(attempt.TransactionID, "(none)"),
					firstNonEmpty(attempt.Message, "(none)"),
				); err != nil {
					return err
				}
				if attempt.AVSResponse != "" || attempt.CardCodeResponse != "" {
					if _, err := fmt.Fprintf(writer, "attempt %d checks: avs %s cvv %s\n",
						attempt.Attempt,
						firstNonEmpty(attempt.AVSResponse, "(none)"),
						firstNonEmpty(attempt.CardCodeResponse, "(none)"),
					); err != nil {
						return err
					}
				}
				if attempt.DuplicateDetected {
					if _, err := fmt.Fprintf(writer, "attempt %d duplicate: detected\n", attempt.Attempt); err != nil {
						return err
					}
				}
			}
			return nil
		},
	})
}

func renderSandboxChargeFailure(cmd *cobra.Command, resultCode string, data sandboxChargeData) error {
	if data.Message == "" {
		data.Message = "sandbox charge failed"
	}
	data = sanitizeForOutput(data).(sandboxChargeData)
	errorCode, exitCode := gatewayFailureMapping(data.GatewayMessageCode, data.Message, "sandbox_transaction_not_found")
	renderErr := renderResult(cmd, commandResult{
		Data: data,
		Errors: []structuredError{{
			Code:    errorCode,
			Message: data.Message,
		}},
		Human: func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "sandbox charge: %s failed\nresult: %s\nmessage: %s\n",
				data.Scenario,
				resultCode,
				data.Message,
			)
			return err
		},
	})
	if renderErr != nil {
		return renderErr
	}
	return renderedError{exitCode: exitCode, message: data.Message}
}
