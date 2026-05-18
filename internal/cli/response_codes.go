package cli

import (
	"fmt"
	"strings"
)

const (
	responseCodeFamilyAPIMessage          = "api_message"
	responseCodeFamilyValidation          = "validation"
	responseCodeFamilyTransactionResponse = "transaction_response"
	responseCodeFamilyAVS                 = "avs"
	responseCodeFamilyCVV                 = "cvv"
)

var responseCodeFamilyAliases = map[string]string{
	"api":                  responseCodeFamilyAPIMessage,
	"api-message":          responseCodeFamilyAPIMessage,
	"api_message":          responseCodeFamilyAPIMessage,
	"message":              responseCodeFamilyAPIMessage,
	"validation":           responseCodeFamilyValidation,
	"transaction":          responseCodeFamilyTransactionResponse,
	"transaction-response": responseCodeFamilyTransactionResponse,
	"transaction_response": responseCodeFamilyTransactionResponse,
	"avs":                  responseCodeFamilyAVS,
	"cvv":                  responseCodeFamilyCVV,
	"card-code":            responseCodeFamilyCVV,
	"card_code":            responseCodeFamilyCVV,
}

var responseCodeReference = responseCodeReferenceData{
	Version:     "2026-05-18",
	Reviewed:    "2026-05-18",
	Maintainer:  "authnet-cli curated reference",
	Maintenance: "Review the official Authorize.Net response-code and API reference pages before each release that changes this file, then update the reference version, review date, records, and tests together.",
	Sources: []responseCodeSource{
		{
			Title: "Authorize.Net response-code tool",
			URL:   "https://developer.authorize.net/api/reference/responseCodes.html",
		},
		{
			Title: "Authorize.Net API error and response codes",
			URL:   "https://developer.authorize.net/api/reference/features/errorandresponsecodes.html",
		},
		{
			Title: "Authorize.Net API transaction response fields",
			URL:   "https://developer.authorize.net/api/reference/index.html",
		},
	},
}

var curatedResponseCodes = []responseCodeRecord{
	{
		Code:       "1",
		Family:     responseCodeFamilyTransactionResponse,
		Title:      "Approved",
		Meaning:    "The transaction was approved.",
		Causes:     []string{"The issuer or processor approved the authorization or payment request."},
		NextSteps:  []string{"Treat the transaction as approved and inspect the transaction status for settlement progress."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/index.html"},
	},
	{
		Code:       "2",
		Family:     responseCodeFamilyTransactionResponse,
		Title:      "Declined",
		Meaning:    "The transaction was declined.",
		Causes:     []string{"The issuer or processor declined the payment request."},
		NextSteps:  []string{"Do not retry blindly. Ask the operator to review the detailed response reason and use another payment method if needed."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/index.html"},
	},
	{
		Code:       "3",
		Family:     responseCodeFamilyTransactionResponse,
		Title:      "Error",
		Meaning:    "The transaction encountered an error.",
		Causes:     []string{"Request data, merchant configuration, processor configuration, or gateway validation can prevent transaction processing."},
		NextSteps:  []string{"Inspect the detailed response reason and API message code before retrying."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/index.html"},
	},
	{
		Code:       "4",
		Family:     responseCodeFamilyTransactionResponse,
		Title:      "Held for Review",
		Meaning:    "The transaction was held for review.",
		Causes:     []string{"Fraud filters or review settings can hold an otherwise processable transaction."},
		NextSteps:  []string{"Review the transaction in Authorize.Net before fulfillment or follow-up action."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/index.html"},
	},
	{
		Code:       "I00001",
		Family:     responseCodeFamilyAPIMessage,
		Title:      "Successful",
		Meaning:    "The request was processed successfully.",
		Causes:     []string{"The API request passed gateway validation and completed."},
		NextSteps:  []string{"Continue with the command result and inspect any command-specific data."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/features/errorandresponsecodes.html"},
	},
	{
		Code:       "I00004",
		Family:     responseCodeFamilyAPIMessage,
		Title:      "No records found",
		Meaning:    "No records matched the request.",
		Causes:     []string{"The identifier, date range, or query criteria did not match any records."},
		NextSteps:  []string{"Verify the selected profile and query inputs before widening the search."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/features/errorandresponsecodes.html"},
	},
	{
		Code:       "E00003",
		Family:     responseCodeFamilyValidation,
		Title:      "Request parse or schema error",
		Meaning:    "The request could not be parsed or did not satisfy the API schema.",
		Causes:     []string{"Malformed JSON/XML, an unsupported field, or a field with an invalid shape can trigger this validation failure."},
		NextSteps:  []string{"Check the command inputs and, if this came from authnet, file a bug with the sanitized request context."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/features/errorandresponsecodes.html"},
	},
	{
		Code:       "E00027",
		Family:     responseCodeFamilyAPIMessage,
		Title:      "Transaction unsuccessful",
		Meaning:    "The transaction was unsuccessful.",
		Causes:     []string{"Gateway validation, processor response, or merchant configuration can cause the payment request to fail."},
		NextSteps:  []string{"Inspect the transaction response errors and response reason code to determine whether operator action or a request correction is needed."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/features/errorandresponsecodes.html"},
	},
	{
		Code:       "A",
		Family:     responseCodeFamilyAVS,
		Title:      "Street address matched",
		Meaning:    "The street address matched, but the postal code did not.",
		Causes:     []string{"Billing street address and postal code verification produced a partial AVS match."},
		NextSteps:  []string{"Follow the merchant account AVS policy and verify billing details before retrying or fulfillment."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/index.html"},
	},
	{
		Code:       "N",
		Family:     responseCodeFamilyAVS,
		Title:      "Address and postal code did not match",
		Meaning:    "Neither the street address nor postal code matched.",
		Causes:     []string{"The billing address supplied with the payment does not match issuer records."},
		NextSteps:  []string{"Ask the customer to verify billing details or use another payment method according to merchant policy."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/index.html"},
	},
	{
		Code:       "Y",
		Family:     responseCodeFamilyAVS,
		Title:      "Address and postal code matched",
		Meaning:    "The street address and postal code matched.",
		Causes:     []string{"The supplied billing address matched issuer AVS data."},
		NextSteps:  []string{"Use this as one risk signal together with transaction status and merchant fraud settings."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/index.html"},
	},
	{
		Code:       "M",
		Family:     responseCodeFamilyCVV,
		Title:      "Card code matched",
		Meaning:    "The card code matched.",
		Causes:     []string{"The supplied CVV/card-code value matched issuer records."},
		NextSteps:  []string{"Use this as one risk signal together with the authorization result and merchant policy."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/index.html"},
	},
	{
		Code:       "N",
		Family:     responseCodeFamilyCVV,
		Title:      "Card code did not match",
		Meaning:    "The card code did not match.",
		Causes:     []string{"The supplied CVV/card-code value does not match issuer records."},
		NextSteps:  []string{"Do not store or echo the card code. Ask the customer to re-enter payment details or use another method according to merchant policy."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/index.html"},
	},
	{
		Code:       "P",
		Family:     responseCodeFamilyCVV,
		Title:      "Card code not processed",
		Meaning:    "The card code was not processed.",
		Causes:     []string{"The processor or issuer did not process card-code verification for this transaction."},
		NextSteps:  []string{"Use the merchant risk policy to decide whether additional verification is needed."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/index.html"},
	},
	{
		Code:       "S",
		Family:     responseCodeFamilyCVV,
		Title:      "Card code should be present",
		Meaning:    "The card code should be present, but was not provided.",
		Causes:     []string{"The request omitted card-code data for a card that supports it."},
		NextSteps:  []string{"Use a payment collection flow that gathers card-code data without logging or persisting it."},
		SourceURLs: []string{"https://developer.authorize.net/api/reference/index.html"},
	},
}

type responseCodeExplainOptions struct {
	Family string
}

type responseCodeReferenceData struct {
	Version     string               `json:"version"`
	Reviewed    string               `json:"reviewed_at"`
	Maintainer  string               `json:"maintainer"`
	Maintenance string               `json:"maintenance"`
	Sources     []responseCodeSource `json:"sources"`
}

type responseCodeSource struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type responseCodeRecord struct {
	Code       string   `json:"code"`
	Family     string   `json:"family"`
	Title      string   `json:"title"`
	Meaning    string   `json:"source_meaning"`
	Causes     []string `json:"likely_causes"`
	NextSteps  []string `json:"recommended_next_steps"`
	SourceURLs []string `json:"source_links"`
}

type responseCodeExplainData struct {
	QueryCode   string                    `json:"query_code"`
	QueryFamily string                    `json:"query_family,omitempty"`
	Matches     []responseCodeRecord      `json:"matches"`
	Reference   responseCodeReferenceData `json:"reference"`
}

func normalizeResponseCodeFamily(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "", nil
	}
	value = strings.ReplaceAll(value, " ", "-")
	family, ok := responseCodeFamilyAliases[value]
	if !ok {
		return "", newUsageError("unknown response-code family %q: expected api_message, validation, transaction_response, avs, or cvv", value)
	}
	return family, nil
}

func explainResponseCode(code string, family string) responseCodeExplainData {
	code = strings.ToUpper(strings.TrimSpace(code))
	matches := []responseCodeRecord{}
	for _, record := range curatedResponseCodes {
		if record.Code != code {
			continue
		}
		if family != "" && record.Family != family {
			continue
		}
		matches = append(matches, record)
	}
	return responseCodeExplainData{
		QueryCode:   code,
		QueryFamily: family,
		Matches:     matches,
		Reference:   responseCodeReference,
	}
}

func ambiguousResponseCodeMessage(data responseCodeExplainData) string {
	families := make([]string, 0, len(data.Matches))
	for _, match := range data.Matches {
		families = append(families, match.Family)
	}
	return fmt.Sprintf("response code %s is ambiguous; rerun with --family %s", data.QueryCode, strings.Join(families, "|"))
}
