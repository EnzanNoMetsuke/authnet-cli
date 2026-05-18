package cli

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultTransactionListLimit = 25
	maxTransactionListLimit     = 100
	defaultTransactionLastRange = "24h"
)

var nowFunc = time.Now

func newVersionCommand(build BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show local version and contract metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return renderResult(cmd, commandResult{
				Data: versionData{
					Version:        build.Version,
					SchemaVersion:  build.SchemaVersion,
					ContractStatus: build.ContractStatus,
					Commit:         build.Commit,
					BuiltAt:        build.Date,
				},
				Human: func(writer io.Writer) error {
					_, err := fmt.Fprintf(writer, "authnet %s\nschema version: %s\ncontract status: %s\ncommit: %s\nbuilt: %s\n",
						build.Version,
						build.SchemaVersion,
						build.ContractStatus,
						build.Commit,
						build.Date,
					)
					return err
				},
			})
		},
	}
}

func newPathsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "paths",
		Short: "Show local authnet paths",
		RunE: func(cmd *cobra.Command, _ []string) error {
			configDir, err := authnetConfigDir()
			if err != nil {
				return err
			}
			warnings := []warning{{
				Code:    "no_sensitive_persistence",
				Message: "v1 does not define CLI-controlled sensitive-data persistence paths.",
			}}
			return renderResult(cmd, commandResult{
				Data: pathsData{
					ConfigDirectory:             configDir,
					ProfileConfigFile:           filepath.Join(configDir, profileConfigFileName),
					SensitiveDataPersistence:    "none",
					HasSensitivePersistencePath: false,
				},
				Warnings: warnings,
				Human: func(writer io.Writer) error {
					_, err := fmt.Fprintf(writer, "config directory: %s\nsensitive-data persistence: none\n", configDir)
					return err
				},
			})
		},
	}
}

func newConfigCommand() *cobra.Command {
	config := &cobra.Command{
		Use:   "config",
		Short: "Manage local non-secret profile config",
	}
	config.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate local non-secret profile config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := newProfileStore()
			if err != nil {
				return err
			}
			file, err := store.load()
			if err != nil {
				return err
			}
			result := validateProfileFile(file)
			result.ConfigPath = store.path
			warnings := result.Warnings
			if len(file.Profiles) == 0 {
				warnings = append(warnings, warning{
					Code:    "no_profiles",
					Message: "no profiles are configured.",
				})
			}
			renderErr := renderResult(cmd, commandResult{
				Data:     result,
				Warnings: warnings,
				Errors:   validationErrors(result),
				Human: func(writer io.Writer) error {
					status := "valid"
					if !result.Valid {
						status = "invalid"
					}
					if _, err := fmt.Fprintf(writer, "profile config: %s\nstatus: %s\nprofiles: %d\n", store.path, status, len(file.Profiles)); err != nil {
						return err
					}
					for _, check := range result.Checks {
						if _, err := fmt.Fprintf(writer, "- %s: %s - %s\n", check.Name, check.Status, check.Message); err != nil {
							return err
						}
					}
					return nil
				},
			})
			if renderErr != nil {
				return renderErr
			}
			if !result.Valid {
				if !optionsFromCommand(cmd).JSON {
					return nil
				}
				return renderedError{exitCode: exitUsageOrConfig, message: "profile config validation failed"}
			}
			return nil
		},
	})
	return config
}

func newAuthCommand() *cobra.Command {
	auth := &cobra.Command{
		Use:   "auth",
		Short: "Test Authorize.Net profile authentication",
	}
	auth.AddCommand(&cobra.Command{
		Use:   "test",
		Short: "Test selected profile authentication",
		RunE:  runAuthTest,
	})
	return auth
}

func newProfileCommand() *cobra.Command {
	profile := &cobra.Command{
		Use:   "profile",
		Short: "Manage local profiles",
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List saved non-secret profile metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := newProfileStore()
			if err != nil {
				return err
			}
			file, err := store.load()
			if err != nil {
				return err
			}
			data := profileListDataFromFile(file)
			return renderResult(cmd, commandResult{
				Data: data,
				Human: func(writer io.Writer) error {
					if len(data.Profiles) == 0 {
						_, err := fmt.Fprintln(writer, "no profiles configured")
						return err
					}
					for _, item := range data.Profiles {
						defaultMarker := ""
						if item.Default {
							defaultMarker = " default"
						}
						if item.ProductionMarker != "" {
							if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s%s\n", item.Name, item.Environment, item.ProductionMarker, item.CredentialSource.Type, defaultMarker); err != nil {
								return err
							}
							continue
						}
						if _, err := fmt.Fprintf(writer, "%s\t%s\t%s%s\n", item.Name, item.Environment, item.CredentialSource.Type, defaultMarker); err != nil {
							return err
						}
					}
					return nil
				},
			})
		},
	}
	profile.AddCommand(list)

	setupOptions := &profileSetupOptions{}
	setup := &cobra.Command{
		Use:   "setup",
		Short: "Create or update profile metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runProfileSetup(cmd, setupOptions)
		},
	}
	setup.Flags().StringVar(&setupOptions.Name, "name", "", "profile name")
	setup.Flags().StringVar(&setupOptions.Environment, "environment", "", "environment classification: sandbox or production")
	setup.Flags().StringVar(&setupOptions.APILoginIDEnv, "api-login-id-env", defaultCredentialLoginEnv, "environment variable containing the API login ID")
	setup.Flags().StringVar(&setupOptions.TransactionKeyEnv, "transaction-key-env", defaultCredentialTranKeyEnv, "environment variable containing the transaction key")
	setup.Flags().StringVar(&setupOptions.SecureReference, "secure-local-reference", "", "secure local credential reference")
	setup.Flags().BoolVar(&setupOptions.Default, "default", false, "make this sandbox profile the default")
	profile.AddCommand(setup)

	removeOptions := &profileRemoveOptions{}
	remove := &cobra.Command{
		Use:   "remove",
		Short: "Remove local profile metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runProfileRemove(cmd, removeOptions)
		},
	}
	remove.Flags().StringVar(&removeOptions.Name, "name", "", "profile name to remove")
	profile.AddCommand(remove)

	return profile
}

func newTransactionCommand() *cobra.Command {
	transaction := &cobra.Command{
		Use:   "transaction",
		Short: "Inspect Authorize.Net transactions",
	}
	transaction.AddCommand(&cobra.Command{
		Use:   "get TRANSACTION_ID",
		Short: "Inspect one transaction",
		Args:  cobra.ExactArgs(1),
		RunE:  runTransactionGet,
	})
	listOptions := &transactionListOptions{
		Limit: defaultTransactionListLimit,
	}
	list := &cobra.Command{
		Use:   "list",
		Short: "List settled transaction history",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTransactionList(cmd, listOptions)
		},
	}
	list.Flags().StringVar(&listOptions.From, "from", "", "range start as YYYY-MM-DD or RFC3339 timestamp")
	list.Flags().StringVar(&listOptions.To, "to", "", "range end as YYYY-MM-DD or RFC3339 timestamp")
	list.Flags().StringVar(&listOptions.Last, "last", defaultTransactionLastRange, "relative range such as 24h, 7d, or 1w")
	list.Flags().IntVar(&listOptions.Limit, "limit", defaultTransactionListLimit, "maximum transactions to return")
	transaction.AddCommand(list)

	unsettled := &cobra.Command{
		Use:   "unsettled",
		Short: "Inspect unsettled transaction set",
	}
	unsettledOptions := &transactionUnsettledListOptions{
		Limit: defaultTransactionListLimit,
	}
	unsettledList := &cobra.Command{
		Use:   "list",
		Short: "List unsettled transactions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTransactionUnsettledList(cmd, unsettledOptions)
		},
	}
	unsettledList.Flags().IntVar(&unsettledOptions.Limit, "limit", defaultTransactionListLimit, "maximum transactions to return")
	unsettled.AddCommand(unsettledList)
	transaction.AddCommand(unsettled)

	return transaction
}

func newCustomerProfileCommand() *cobra.Command {
	customerProfile := &cobra.Command{
		Use:   "customer-profile",
		Short: "Inspect Authorize.Net customer profiles",
	}
	getOptions := &customerProfileGetOptions{}
	get := &cobra.Command{
		Use:   "get CUSTOMER_PROFILE_ID",
		Short: "Inspect one customer profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCustomerProfileGet(cmd, args, getOptions)
		},
	}
	get.Flags().BoolVar(&getOptions.IncludePaymentProfiles, "include-payment-profiles", false, "include redacted customer payment profile summaries")
	get.Flags().BoolVar(&getOptions.IncludeShippingAddresses, "include-shipping-addresses", false, "include redacted customer shipping address summaries")
	customerProfile.AddCommand(get)
	customerProfile.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List customer profile metadata",
		RunE:  runCustomerProfileList,
	})
	return customerProfile
}

func newResponseCodeCommand() *cobra.Command {
	responseCode := &cobra.Command{
		Use:   "response-code",
		Short: "Explain Authorize.Net response codes",
	}
	responseCode.AddCommand(&cobra.Command{
		Use:   "explain",
		Short: "Explain a gateway or API response code",
		RunE:  notImplemented("response-code explain"),
	})
	return responseCode
}

func newSandboxCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "sandbox",
		Short: "Run sandbox-only test helpers",
		RunE:  notImplemented("sandbox"),
	}
}

type versionData struct {
	Version        string `json:"version"`
	SchemaVersion  string `json:"schema_version"`
	ContractStatus string `json:"contract_status"`
	Commit         string `json:"commit"`
	BuiltAt        string `json:"built_at"`
}

type pathsData struct {
	ConfigDirectory             string `json:"config_directory"`
	ProfileConfigFile           string `json:"profile_config_file"`
	SensitiveDataPersistence    string `json:"sensitive_data_persistence"`
	HasSensitivePersistencePath bool   `json:"has_sensitive_persistence_path"`
}

type profileSetupOptions struct {
	Name              string
	Environment       string
	APILoginIDEnv     string
	TransactionKeyEnv string
	SecureReference   string
	Default           bool
}

type profileRemoveOptions struct {
	Name string
}

type customerProfileGetOptions struct {
	IncludePaymentProfiles   bool
	IncludeShippingAddresses bool
}

type transactionListOptions struct {
	From  string
	To    string
	Last  string
	Limit int
}

type transactionUnsettledListOptions struct {
	Limit int
}

type profileListData struct {
	DefaultProfile string            `json:"default_profile,omitempty"`
	Profiles       []profileListItem `json:"profiles"`
}

type profileListItem struct {
	Name             string           `json:"name"`
	Environment      string           `json:"environment"`
	ProductionMarker string           `json:"production_marker,omitempty"`
	Default          bool             `json:"default"`
	CredentialSource credentialSource `json:"credential_source"`
}

type profileMutationData struct {
	Name        string `json:"name"`
	Environment string `json:"environment,omitempty"`
	ConfigPath  string `json:"config_path"`
	Default     bool   `json:"default,omitempty"`
	Removed     bool   `json:"removed,omitempty"`
}

type authTestData struct {
	Authenticated             bool   `json:"authenticated"`
	ProfileName               string `json:"profile_name"`
	EnvironmentClassification string `json:"environment_classification"`
	CredentialSource          string `json:"credential_source"`
	GatewayResultCode         string `json:"gateway_result_code,omitempty"`
	GatewayMessageCode        string `json:"gateway_message_code,omitempty"`
	Message                   string `json:"message"`
}

type transactionLookupData struct {
	TransactionID             string                 `json:"transaction_id"`
	ProfileName               string                 `json:"profile_name"`
	EnvironmentClassification string                 `json:"environment_classification"`
	TransactionStatus         string                 `json:"transaction_status,omitempty"`
	ResponseCode              string                 `json:"response_code,omitempty"`
	ResponseReasonCode        string                 `json:"response_reason_code,omitempty"`
	ResponseReasonDescription string                 `json:"response_reason_description,omitempty"`
	AuthCode                  string                 `json:"auth_code,omitempty"`
	SubmitTimeUTC             string                 `json:"submit_time_utc,omitempty"`
	SubmitTimeLocal           string                 `json:"submit_time_local,omitempty"`
	SettleAmount              string                 `json:"settle_amount,omitempty"`
	SettlementState           string                 `json:"settlement_state,omitempty"`
	SettlementTimeUTC         string                 `json:"settlement_time_utc,omitempty"`
	Payment                   paymentSummary         `json:"payment,omitempty"`
	CustomerProfile           customerProfileSummary `json:"customer_profile,omitempty"`
	GatewayDetails            gatewayDetailsSummary  `json:"gateway_details,omitempty"`
	GatewayMessageCode        string                 `json:"gateway_message_code,omitempty"`
	Message                   string                 `json:"message,omitempty"`
}

type transactionListData struct {
	ProfileName               string                    `json:"profile_name"`
	EnvironmentClassification string                    `json:"environment_classification"`
	ProductionMarker          string                    `json:"production_marker,omitempty"`
	Kind                      string                    `json:"kind"`
	Query                     transactionListQuery      `json:"query"`
	Pagination                transactionListPagination `json:"pagination"`
	BatchCount                int                       `json:"batch_count,omitempty"`
	Transactions              []transactionListItem     `json:"transactions"`
	GatewayMessageCode        string                    `json:"gateway_message_code,omitempty"`
	Message                   string                    `json:"message,omitempty"`
}

type transactionListQuery struct {
	From                  string `json:"from,omitempty"`
	To                    string `json:"to,omitempty"`
	OperatorLocalTimeZone string `json:"operator_local_time_zone,omitempty"`
	RelativeRange         string `json:"relative_range,omitempty"`
}

type transactionListPagination struct {
	RequestedLimit int  `json:"requested_limit"`
	ReturnedCount  int  `json:"returned_count"`
	HasMore        bool `json:"has_more"`
}

type transactionListItem struct {
	TransactionID     string         `json:"transaction_id"`
	TransactionStatus string         `json:"transaction_status,omitempty"`
	SubmitTimeUTC     string         `json:"submit_time_utc,omitempty"`
	SubmitTimeLocal   string         `json:"submit_time_local,omitempty"`
	SettleAmount      string         `json:"settle_amount,omitempty"`
	Payment           paymentSummary `json:"payment,omitempty"`
	BatchID           string         `json:"batch_id,omitempty"`
}

type paymentSummary struct {
	AccountType   string `json:"account_type,omitempty"`
	AccountNumber string `json:"account_number,omitempty"`
	CardType      string `json:"card_type,omitempty"`
}

type customerProfileSummary struct {
	CustomerProfileID        string `json:"customer_profile_id,omitempty"`
	CustomerPaymentProfileID string `json:"customer_payment_profile_id,omitempty"`
}

type gatewayDetailsSummary struct {
	BatchID          string `json:"batch_id,omitempty"`
	AVSResponse      string `json:"avs_response,omitempty"`
	CardCodeResponse string `json:"card_code_response,omitempty"`
	CAVVResponse     string `json:"cavv_response,omitempty"`
}

type customerProfileLookupData struct {
	CustomerProfileID         string                           `json:"customer_profile_id"`
	ProfileName               string                           `json:"profile_name"`
	EnvironmentClassification string                           `json:"environment_classification"`
	ProductionMarker          string                           `json:"production_marker,omitempty"`
	MerchantCustomerID        string                           `json:"merchant_customer_id,omitempty"`
	PaymentProfileCount       int                              `json:"payment_profile_count"`
	ShippingAddressCount      int                              `json:"shipping_address_count"`
	PaymentProfiles           []customerPaymentProfileSummary  `json:"payment_profiles,omitempty"`
	ShippingAddresses         []customerShippingAddressSummary `json:"shipping_addresses,omitempty"`
	GatewayMessageCode        string                           `json:"gateway_message_code,omitempty"`
	Message                   string                           `json:"message,omitempty"`
}

type customerPaymentProfileSummary struct {
	CustomerPaymentProfileID string `json:"customer_payment_profile_id"`
	AccountType              string `json:"account_type,omitempty"`
	AccountNumber            string `json:"account_number,omitempty"`
	CardType                 string `json:"card_type,omitempty"`
}

type customerShippingAddressSummary struct {
	CustomerAddressID string `json:"customer_address_id"`
}

type customerProfileListData struct {
	ProfileName               string   `json:"profile_name"`
	EnvironmentClassification string   `json:"environment_classification"`
	ProductionMarker          string   `json:"production_marker,omitempty"`
	Count                     int      `json:"count"`
	CustomerProfileIDs        []string `json:"customer_profile_ids"`
	GatewayMessageCode        string   `json:"gateway_message_code,omitempty"`
	Message                   string   `json:"message,omitempty"`
}

func runAuthTest(cmd *cobra.Command, _ []string) error {
	options := optionsFromCommand(cmd)
	profile, err := loadSelectedProfileWithCredentials(options, "auth test")
	if err != nil {
		return err
	}
	client, err := newGatewayClient(profile.Entry.Environment)
	if err != nil {
		return err
	}
	response, err := client.authenticate(cmd.Context(), profile.Credentials)
	if err != nil {
		return err
	}

	message := firstGatewayMessage(response.Messages.Message)
	data := authTestData{
		Authenticated:             strings.EqualFold(response.Messages.ResultCode, "Ok"),
		ProfileName:               profile.Entry.Name,
		EnvironmentClassification: profile.Entry.Environment,
		CredentialSource:          profile.Entry.CredentialSource.Type,
		GatewayResultCode:         response.Messages.ResultCode,
		GatewayMessageCode:        message.Code,
		Message:                   message.Text,
	}
	if data.Message == "" {
		data.Message = "authentication response did not include a message"
	}
	data = sanitizeForOutput(data).(authTestData)
	if !data.Authenticated {
		renderErr := renderResult(cmd, commandResult{
			Data: data,
			Errors: []structuredError{{
				Code:    "authentication_failed",
				Message: data.Message,
			}},
			Human: func(writer io.Writer) error {
				_, writeErr := fmt.Fprintf(writer, "profile: %s\nenvironment: %s\nauthentication: failed\nmessage: %s\n",
					data.ProfileName,
					data.EnvironmentClassification,
					data.Message,
				)
				return writeErr
			},
		})
		if renderErr != nil {
			return renderErr
		}
		return renderedError{exitCode: exitAuthFailure, message: data.Message}
	}
	return renderResult(cmd, commandResult{
		Data: data,
		Human: func(writer io.Writer) error {
			_, writeErr := fmt.Fprintf(writer, "profile: %s\nenvironment: %s\nauthentication: ok\nmessage: %s\n",
				data.ProfileName,
				data.EnvironmentClassification,
				data.Message,
			)
			return writeErr
		},
	})
}

func runTransactionGet(cmd *cobra.Command, args []string) error {
	transactionID := strings.TrimSpace(args[0])
	if transactionID == "" {
		return newUsageError("transaction get requires a transaction identifier")
	}

	options := optionsFromCommand(cmd)
	profile, err := loadSelectedProfileWithCredentials(options, "transaction get")
	if err != nil {
		return err
	}
	client, err := newGatewayClient(profile.Entry.Environment)
	if err != nil {
		return err
	}
	response, err := client.getTransactionDetails(cmd.Context(), profile.Credentials, transactionID)
	if err != nil {
		return err
	}

	message := firstGatewayMessage(response.Messages.Message)
	data := transactionLookupData{
		TransactionID:             firstNonEmpty(response.Transaction.TransactionID.String(), transactionID),
		ProfileName:               profile.Entry.Name,
		EnvironmentClassification: profile.Entry.Environment,
		GatewayMessageCode:        message.Code,
		Message:                   message.Text,
	}
	if !strings.EqualFold(response.Messages.ResultCode, "Ok") {
		return renderTransactionLookupFailure(cmd, response.Messages.ResultCode, data)
	}

	data = data.withTransaction(response.Transaction)
	data = sanitizeForOutput(data).(transactionLookupData)
	return renderResult(cmd, commandResult{
		Data: data,
		Human: func(writer io.Writer) error {
			if _, writeErr := fmt.Fprintf(writer, "transaction: %s\nstatus: %s\nresponse: %s\n",
				data.TransactionID,
				data.TransactionStatus,
				data.ResponseCode,
			); writeErr != nil {
				return writeErr
			}
			if data.SettleAmount != "" {
				if _, writeErr := fmt.Fprintf(writer, "settle amount: %s\n", data.SettleAmount); writeErr != nil {
					return writeErr
				}
			}
			if data.Payment.AccountNumber != "" || data.Payment.AccountType != "" {
				if _, writeErr := fmt.Fprintf(writer, "payment: %s %s\n", data.Payment.AccountType, data.Payment.AccountNumber); writeErr != nil {
					return writeErr
				}
			}
			if data.SettlementState != "" {
				if _, writeErr := fmt.Fprintf(writer, "settlement: %s\n", data.SettlementState); writeErr != nil {
					return writeErr
				}
			}
			return nil
		},
	})
}

func runTransactionList(cmd *cobra.Command, listOptions *transactionListOptions) error {
	limit, err := normalizedTransactionListLimit(listOptions.Limit)
	if err != nil {
		return err
	}
	resolvedRange, err := resolveTransactionTimeRange(*listOptions, nowFunc())
	if err != nil {
		return err
	}

	options := optionsFromCommand(cmd)
	profile, err := loadSelectedProfileWithCredentials(options, "transaction list")
	if err != nil {
		return err
	}
	client, err := newGatewayClient(profile.Entry.Environment)
	if err != nil {
		return err
	}
	batches, err := client.getSettledBatchList(cmd.Context(), profile.Credentials, resolvedRange.GatewayFrom, resolvedRange.GatewayTo)
	if err != nil {
		return err
	}

	message := firstGatewayMessage(batches.Messages.Message)
	data := transactionListData{
		ProfileName:               profile.Entry.Name,
		EnvironmentClassification: profile.Entry.Environment,
		ProductionMarker:          productionMarkerForEnvironment(profile.Entry.Environment),
		Kind:                      "settled",
		Query:                     resolvedRange.Query,
		Pagination: transactionListPagination{
			RequestedLimit: limit,
		},
		BatchCount:         len(batches.BatchList),
		Transactions:       []transactionListItem{},
		GatewayMessageCode: message.Code,
		Message:            message.Text,
	}
	if !strings.EqualFold(batches.Messages.ResultCode, "Ok") {
		return renderTransactionListFailure(cmd, "transaction list", batches.Messages.ResultCode, data)
	}

	for _, batch := range batches.BatchList {
		if len(data.Transactions) >= limit {
			break
		}
		remaining := limit - len(data.Transactions)
		response, err := client.getTransactionList(cmd.Context(), profile.Credentials, batch.BatchID.String(), gatewayPaging{
			Limit:  remaining,
			Offset: 1,
		})
		if err != nil {
			return err
		}
		message = firstGatewayMessage(response.Messages.Message)
		if !strings.EqualFold(response.Messages.ResultCode, "Ok") {
			data.GatewayMessageCode = message.Code
			data.Message = message.Text
			return renderTransactionListFailure(cmd, "transaction list", response.Messages.ResultCode, data)
		}
		data.Transactions = append(data.Transactions, transactionListItems(response.Transactions, batch.BatchID.String())...)
	}
	data.Pagination.ReturnedCount = len(data.Transactions)
	data.Pagination.HasMore = len(data.Transactions) >= limit && len(data.Transactions) < transactionListPossibleCount(batches.BatchList)
	return renderTransactionListResult(cmd, data)
}

func runTransactionUnsettledList(cmd *cobra.Command, listOptions *transactionUnsettledListOptions) error {
	limit, err := normalizedTransactionListLimit(listOptions.Limit)
	if err != nil {
		return err
	}

	options := optionsFromCommand(cmd)
	profile, err := loadSelectedProfileWithCredentials(options, "transaction unsettled list")
	if err != nil {
		return err
	}
	client, err := newGatewayClient(profile.Entry.Environment)
	if err != nil {
		return err
	}
	response, err := client.getUnsettledTransactionList(cmd.Context(), profile.Credentials, gatewayPaging{
		Limit:  limit,
		Offset: 1,
	})
	if err != nil {
		return err
	}

	message := firstGatewayMessage(response.Messages.Message)
	data := transactionListData{
		ProfileName:               profile.Entry.Name,
		EnvironmentClassification: profile.Entry.Environment,
		ProductionMarker:          productionMarkerForEnvironment(profile.Entry.Environment),
		Kind:                      "unsettled",
		Pagination: transactionListPagination{
			RequestedLimit: limit,
			ReturnedCount:  len(response.Transactions),
			HasMore:        len(response.Transactions) >= limit,
		},
		Transactions:       transactionListItems(response.Transactions, ""),
		GatewayMessageCode: message.Code,
		Message:            message.Text,
	}
	if !strings.EqualFold(response.Messages.ResultCode, "Ok") {
		return renderTransactionListFailure(cmd, "transaction unsettled list", response.Messages.ResultCode, data)
	}
	return renderTransactionListResult(cmd, data)
}

func runCustomerProfileGet(cmd *cobra.Command, args []string, getOptions *customerProfileGetOptions) error {
	customerProfileID := strings.TrimSpace(args[0])
	if customerProfileID == "" {
		return newUsageError("customer-profile get requires a customer profile identifier")
	}

	options := optionsFromCommand(cmd)
	profile, err := loadSelectedProfileWithCredentials(options, "customer-profile get")
	if err != nil {
		return err
	}
	client, err := newGatewayClient(profile.Entry.Environment)
	if err != nil {
		return err
	}
	response, err := client.getCustomerProfile(cmd.Context(), profile.Credentials, customerProfileID)
	if err != nil {
		return err
	}

	message := firstGatewayMessage(response.Messages.Message)
	data := customerProfileLookupData{
		CustomerProfileID:         firstNonEmpty(response.Profile.CustomerProfileID.String(), customerProfileID),
		ProfileName:               profile.Entry.Name,
		EnvironmentClassification: profile.Entry.Environment,
		ProductionMarker:          productionMarkerForEnvironment(profile.Entry.Environment),
		GatewayMessageCode:        message.Code,
		Message:                   message.Text,
	}
	if !strings.EqualFold(response.Messages.ResultCode, "Ok") {
		return renderCustomerProfileFailure(cmd, "customer profile lookup", response.Messages.ResultCode, data.CustomerProfileID, data.GatewayMessageCode, data.Message)
	}

	data = data.withCustomerProfile(response.Profile, getOptions)
	data = sanitizeForOutput(data).(customerProfileLookupData)
	return renderResult(cmd, commandResult{
		Data: data,
		Human: func(writer io.Writer) error {
			if _, writeErr := fmt.Fprintf(writer, "customer profile: %s\nmerchant customer: %s\nenvironment: %s%s\npayment profiles: %d\nshipping addresses: %d\n",
				data.CustomerProfileID,
				firstNonEmpty(data.MerchantCustomerID, "(none)"),
				data.EnvironmentClassification,
				productionMarkerSuffix(data.ProductionMarker),
				data.PaymentProfileCount,
				data.ShippingAddressCount,
			); writeErr != nil {
				return writeErr
			}
			for _, item := range data.PaymentProfiles {
				if _, writeErr := fmt.Fprintf(writer, "payment profile: %s %s %s\n", item.CustomerPaymentProfileID, item.AccountType, item.AccountNumber); writeErr != nil {
					return writeErr
				}
			}
			for _, item := range data.ShippingAddresses {
				if _, writeErr := fmt.Fprintf(writer, "shipping address: %s\n", item.CustomerAddressID); writeErr != nil {
					return writeErr
				}
			}
			return nil
		},
	})
}

func runCustomerProfileList(cmd *cobra.Command, _ []string) error {
	options := optionsFromCommand(cmd)
	profile, err := loadSelectedProfileWithCredentials(options, "customer-profile list")
	if err != nil {
		return err
	}
	client, err := newGatewayClient(profile.Entry.Environment)
	if err != nil {
		return err
	}
	response, err := client.getCustomerProfileIDs(cmd.Context(), profile.Credentials)
	if err != nil {
		return err
	}

	message := firstGatewayMessage(response.Messages.Message)
	data := customerProfileListData{
		ProfileName:               profile.Entry.Name,
		EnvironmentClassification: profile.Entry.Environment,
		ProductionMarker:          productionMarkerForEnvironment(profile.Entry.Environment),
		CustomerProfileIDs:        gatewayStrings(response.IDs),
		GatewayMessageCode:        message.Code,
		Message:                   message.Text,
	}
	data.Count = len(data.CustomerProfileIDs)
	if !strings.EqualFold(response.Messages.ResultCode, "Ok") {
		return renderCustomerProfileListFailure(cmd, response.Messages.ResultCode, data)
	}

	data = sanitizeForOutput(data).(customerProfileListData)
	return renderResult(cmd, commandResult{
		Data: data,
		Human: func(writer io.Writer) error {
			if _, writeErr := fmt.Fprintf(writer, "environment: %s%s\ncustomer profiles: %d\n",
				data.EnvironmentClassification,
				productionMarkerSuffix(data.ProductionMarker),
				data.Count,
			); writeErr != nil {
				return writeErr
			}
			for _, customerProfileID := range data.CustomerProfileIDs {
				if _, writeErr := fmt.Fprintf(writer, "%s\n", customerProfileID); writeErr != nil {
					return writeErr
				}
			}
			return nil
		},
	})
}

func renderCustomerProfileListFailure(cmd *cobra.Command, resultCode string, data customerProfileListData) error {
	if data.Message == "" {
		data.Message = "customer profile list failed"
	}
	data = sanitizeForOutput(data).(customerProfileListData)
	errorCode, exitCode := gatewayFailureMapping(data.GatewayMessageCode, data.Message, "customer_profile_not_found")
	renderErr := renderResult(cmd, commandResult{
		Data: data,
		Errors: []structuredError{{
			Code:    errorCode,
			Message: data.Message,
		}},
		Human: func(writer io.Writer) error {
			_, writeErr := fmt.Fprintf(writer, "customer profile list: failed\nresult: %s\nmessage: %s\n",
				resultCode,
				data.Message,
			)
			return writeErr
		},
	})
	if renderErr != nil {
		return renderErr
	}
	return renderedError{exitCode: exitCode, message: data.Message}
}

func renderTransactionListResult(cmd *cobra.Command, data transactionListData) error {
	data = sanitizeForOutput(data).(transactionListData)
	return renderResult(cmd, commandResult{
		Data: data,
		Human: func(writer io.Writer) error {
			if _, writeErr := fmt.Fprintf(writer, "%s transactions: %d\nenvironment: %s%s\n",
				data.Kind,
				data.Pagination.ReturnedCount,
				data.EnvironmentClassification,
				productionMarkerSuffix(data.ProductionMarker),
			); writeErr != nil {
				return writeErr
			}
			if data.Query.From != "" || data.Query.To != "" {
				if _, writeErr := fmt.Fprintf(writer, "range: %s to %s\n", data.Query.From, data.Query.To); writeErr != nil {
					return writeErr
				}
			}
			for _, item := range data.Transactions {
				if _, writeErr := fmt.Fprintf(writer, "%s\t%s\t%s\t%s %s\n",
					item.TransactionID,
					item.TransactionStatus,
					item.SettleAmount,
					item.Payment.AccountType,
					item.Payment.AccountNumber,
				); writeErr != nil {
					return writeErr
				}
			}
			return nil
		},
	})
}

func renderTransactionListFailure(cmd *cobra.Command, operation string, resultCode string, data transactionListData) error {
	if data.Message == "" {
		data.Message = operation + " failed"
	}
	data = sanitizeForOutput(data).(transactionListData)
	errorCode, exitCode := gatewayFailureMapping(data.GatewayMessageCode, data.Message, "transaction_not_found")
	renderErr := renderResult(cmd, commandResult{
		Data: data,
		Errors: []structuredError{{
			Code:    errorCode,
			Message: data.Message,
		}},
		Human: func(writer io.Writer) error {
			_, writeErr := fmt.Fprintf(writer, "%s: failed\nresult: %s\nmessage: %s\n",
				operation,
				resultCode,
				data.Message,
			)
			return writeErr
		},
	})
	if renderErr != nil {
		return renderErr
	}
	return renderedError{exitCode: exitCode, message: data.Message}
}

func renderCustomerProfileFailure(cmd *cobra.Command, operation string, resultCode string, customerProfileID string, gatewayMessageCode string, message string) error {
	if message == "" {
		message = operation + " failed"
	}
	data := customerProfileLookupData{
		CustomerProfileID:  customerProfileID,
		GatewayMessageCode: gatewayMessageCode,
		Message:            message,
	}
	data = sanitizeForOutput(data).(customerProfileLookupData)
	errorCode, exitCode := gatewayFailureMapping(gatewayMessageCode, message, "customer_profile_not_found")
	renderErr := renderResult(cmd, commandResult{
		Data: data,
		Errors: []structuredError{{
			Code:    errorCode,
			Message: data.Message,
		}},
		Human: func(writer io.Writer) error {
			_, writeErr := fmt.Fprintf(writer, "customer profile: %s\nlookup: failed\nresult: %s\nmessage: %s\n",
				data.CustomerProfileID,
				resultCode,
				data.Message,
			)
			return writeErr
		},
	})
	if renderErr != nil {
		return renderErr
	}
	return renderedError{exitCode: exitCode, message: data.Message}
}

func renderTransactionLookupFailure(cmd *cobra.Command, resultCode string, data transactionLookupData) error {
	if data.Message == "" {
		data.Message = "transaction lookup failed"
	}
	data = sanitizeForOutput(data).(transactionLookupData)
	errorCode, exitCode := gatewayFailureMapping(data.GatewayMessageCode, data.Message, "transaction_not_found")
	renderErr := renderResult(cmd, commandResult{
		Data: data,
		Errors: []structuredError{{
			Code:    errorCode,
			Message: data.Message,
		}},
		Human: func(writer io.Writer) error {
			_, writeErr := fmt.Fprintf(writer, "transaction: %s\nlookup: failed\nresult: %s\nmessage: %s\n",
				data.TransactionID,
				resultCode,
				data.Message,
			)
			return writeErr
		},
	})
	if renderErr != nil {
		return renderErr
	}
	return renderedError{exitCode: exitCode, message: data.Message}
}

func gatewayFailureMapping(gatewayMessageCode string, message string, notFoundCode string) (string, ExitCode) {
	if gatewayMessageCode == "E00040" || strings.Contains(strings.ToLower(message), "not found") || strings.Contains(strings.ToLower(message), "cannot be found") {
		return notFoundCode, exitNotFound
	}
	if gatewayMessageCode == "E00007" || gatewayMessageCode == "E00008" {
		return "authentication_failed", exitAuthFailure
	}
	return "gateway_failure", exitGatewayFailure
}

func (data transactionLookupData) withTransaction(transaction gatewayTransaction) transactionLookupData {
	data.TransactionStatus = transaction.TransactionStatus.String()
	data.ResponseCode = transaction.ResponseCode.String()
	data.ResponseReasonCode = transaction.ResponseReasonCode.String()
	data.ResponseReasonDescription = transaction.ResponseReasonDescription.String()
	data.AuthCode = transaction.AuthCode.String()
	data.SubmitTimeUTC = transaction.SubmitTimeUTC.String()
	data.SubmitTimeLocal = transaction.SubmitTimeLocal.String()
	data.SettleAmount = transaction.SettleAmount.String()
	data.SettlementState = transaction.Batch.SettlementState.String()
	data.SettlementTimeUTC = transaction.Batch.SettlementTimeUTC.String()
	data.Payment = paymentSummary{
		AccountType:   firstNonEmpty(transaction.AccountType.String(), transaction.Payment.CreditCard.CardType.String()),
		AccountNumber: firstNonEmpty(transaction.AccountNumber.String(), transaction.Payment.CreditCard.CardNumber.String()),
		CardType:      transaction.Payment.CreditCard.CardType.String(),
	}
	data.CustomerProfile = customerProfileSummary{
		CustomerProfileID:        transaction.Profile.CustomerProfileID.String(),
		CustomerPaymentProfileID: transaction.Profile.CustomerPaymentProfileID.String(),
	}
	data.GatewayDetails = gatewayDetailsSummary{
		BatchID:          transaction.Batch.BatchID.String(),
		AVSResponse:      transaction.AVSResponse.String(),
		CardCodeResponse: transaction.CardCodeResponse.String(),
		CAVVResponse:     transaction.CAVVResponse.String(),
	}
	return data
}

func transactionListItems(transactions []gatewayTransaction, batchID string) []transactionListItem {
	items := make([]transactionListItem, 0, len(transactions))
	for _, transaction := range transactions {
		items = append(items, transactionListItem{
			TransactionID:     transaction.TransactionID.String(),
			TransactionStatus: transaction.TransactionStatus.String(),
			SubmitTimeUTC:     transaction.SubmitTimeUTC.String(),
			SubmitTimeLocal:   transaction.SubmitTimeLocal.String(),
			SettleAmount:      transaction.SettleAmount.String(),
			Payment: paymentSummary{
				AccountType:   firstNonEmpty(transaction.AccountType.String(), transaction.Payment.CreditCard.CardType.String()),
				AccountNumber: firstNonEmpty(transaction.AccountNumber.String(), transaction.Payment.CreditCard.CardNumber.String()),
				CardType:      transaction.Payment.CreditCard.CardType.String(),
			},
			BatchID: firstNonEmpty(batchID, transaction.Batch.BatchID.String()),
		})
	}
	return items
}

func transactionListPossibleCount(batches []gatewayBatch) int {
	return len(batches) * maxTransactionListLimit
}

func (data customerProfileLookupData) withCustomerProfile(profile gatewayCustomerProfile, options *customerProfileGetOptions) customerProfileLookupData {
	data.CustomerProfileID = firstNonEmpty(profile.CustomerProfileID.String(), data.CustomerProfileID)
	data.MerchantCustomerID = profile.MerchantCustomerID.String()
	data.PaymentProfileCount = len(profile.PaymentProfiles)
	data.ShippingAddressCount = len(profile.ShipToList)
	if options.IncludePaymentProfiles {
		for _, paymentProfile := range profile.PaymentProfiles {
			data.PaymentProfiles = append(data.PaymentProfiles, customerPaymentProfileSummary{
				CustomerPaymentProfileID: paymentProfile.CustomerPaymentProfileID.String(),
				AccountType:              firstNonEmpty(paymentProfile.Payment.CreditCard.CardType.String()),
				AccountNumber:            paymentProfile.Payment.CreditCard.CardNumber.String(),
				CardType:                 paymentProfile.Payment.CreditCard.CardType.String(),
			})
		}
	}
	if options.IncludeShippingAddresses {
		for _, address := range profile.ShipToList {
			data.ShippingAddresses = append(data.ShippingAddresses, customerShippingAddressSummary{
				CustomerAddressID: address.CustomerAddressID.String(),
			})
		}
	}
	return data
}

func firstGatewayMessage(messages []gatewayMessage) gatewayMessage {
	if len(messages) == 0 {
		return gatewayMessage{}
	}
	return messages[0]
}

func gatewayStrings(values []gatewayString) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text := strings.TrimSpace(value.String()); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func productionMarkerForEnvironment(environment string) string {
	if environment == environmentProduction {
		return productionMarker
	}
	return ""
}

func productionMarkerSuffix(marker string) string {
	if marker == "" {
		return ""
	}
	return " " + marker
}

func normalizedTransactionListLimit(limit int) (int, error) {
	if limit < 1 {
		return 0, newUsageError("--limit must be at least 1")
	}
	if limit > maxTransactionListLimit {
		return 0, newUsageError("--limit must be at most %d", maxTransactionListLimit)
	}
	return limit, nil
}

type resolvedTransactionTimeRange struct {
	GatewayFrom string
	GatewayTo   string
	Query       transactionListQuery
}

func resolveTransactionTimeRange(options transactionListOptions, now time.Time) (resolvedTransactionTimeRange, error) {
	if strings.TrimSpace(options.Last) != "" && (strings.TrimSpace(options.From) != "" || strings.TrimSpace(options.To) != "") {
		return resolvedTransactionTimeRange{}, newUsageError("--last cannot be combined with --from or --to")
	}
	location := now.Location()
	to := now
	var from time.Time
	relative := strings.TrimSpace(options.Last)
	if relative != "" {
		duration, err := parseRelativeDuration(relative)
		if err != nil {
			return resolvedTransactionTimeRange{}, err
		}
		from = to.Add(-duration)
	} else {
		var err error
		from, err = parseOperatorTime(options.From, location, false)
		if err != nil {
			return resolvedTransactionTimeRange{}, err
		}
		to, err = parseOperatorTime(options.To, location, true)
		if err != nil {
			return resolvedTransactionTimeRange{}, err
		}
	}
	if from.IsZero() {
		return resolvedTransactionTimeRange{}, newUsageError("transaction list requires --last or both --from and --to")
	}
	if !from.Before(to) {
		return resolvedTransactionTimeRange{}, newUsageError("transaction list requires --from to be before --to")
	}
	return resolvedTransactionTimeRange{
		GatewayFrom: from.Format(time.RFC3339),
		GatewayTo:   to.Format(time.RFC3339),
		Query: transactionListQuery{
			From:                  from.Format(time.RFC3339),
			To:                    to.Format(time.RFC3339),
			OperatorLocalTimeZone: location.String(),
			RelativeRange:         relative,
		},
	}, nil
}

func parseOperatorTime(value string, location *time.Location, endOfDay bool) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, newUsageError("transaction list requires --last or both --from and --to")
	}
	if parsed, err := time.ParseInLocation("2006-01-02", value, location); err == nil {
		if endOfDay {
			return parsed.AddDate(0, 0, 1).Add(-time.Nanosecond), nil
		}
		return parsed, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02T15:04:05", value, location); err == nil {
		return parsed, nil
	}
	return time.Time{}, newUsageError("invalid time %q: expected YYYY-MM-DD, RFC3339, or local YYYY-MM-DDTHH:MM:SS", value)
}

func parseRelativeDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, newUsageError("--last requires a duration")
	}
	multiplier := time.Hour
	number := strings.TrimSuffix(value, "h")
	switch {
	case strings.HasSuffix(value, "d"):
		multiplier = 24 * time.Hour
		number = strings.TrimSuffix(value, "d")
	case strings.HasSuffix(value, "w"):
		multiplier = 7 * 24 * time.Hour
		number = strings.TrimSuffix(value, "w")
	case strings.HasSuffix(value, "h"):
	default:
		return 0, newUsageError("invalid --last value %q: expected duration ending in h, d, or w", value)
	}
	count, err := parsePositiveInteger(number)
	if err != nil {
		return 0, newUsageError("invalid --last value %q: expected positive duration", value)
	}
	return time.Duration(count) * multiplier, nil
}

func parsePositiveInteger(value string) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("missing integer")
	}
	result := 0
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, fmt.Errorf("invalid integer")
		}
		result = result*10 + int(char-'0')
	}
	if result < 1 {
		return 0, fmt.Errorf("invalid integer")
	}
	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func runProfileSetup(cmd *cobra.Command, options *profileSetupOptions) error {
	global := optionsFromCommand(cmd)
	if global.Automation && (strings.TrimSpace(options.Name) == "" || strings.TrimSpace(options.Environment) == "") {
		return newUsageError("profile setup requires --name and --environment in automation mode")
	}
	if !global.Automation {
		scanner := bufio.NewScanner(cmd.InOrStdin())
		if err := promptForMissing(scanner, cmd.ErrOrStderr(), "profile name", &options.Name); err != nil {
			return err
		}
		if err := promptForMissing(scanner, cmd.ErrOrStderr(), "environment", &options.Environment); err != nil {
			return err
		}
	}

	source := credentialSource{
		Type:              credentialSourceEnv,
		APILoginIDEnv:     options.APILoginIDEnv,
		TransactionKeyEnv: options.TransactionKeyEnv,
	}
	if strings.TrimSpace(options.SecureReference) != "" {
		source = credentialSource{
			Type:      credentialSourceSecureRef,
			Reference: strings.TrimSpace(options.SecureReference),
		}
	}
	entry := profileEntry{
		Name:             strings.TrimSpace(options.Name),
		Environment:      strings.TrimSpace(options.Environment),
		CredentialSource: source,
	}

	store, err := newProfileStore()
	if err != nil {
		return err
	}
	file, err := store.load()
	if err != nil {
		return err
	}
	file, err = upsertProfile(file, entry, options.Default)
	if err != nil {
		return err
	}
	if err := store.save(file); err != nil {
		return err
	}
	return renderResult(cmd, commandResult{
		Data: profileMutationData{
			Name:        entry.Name,
			Environment: entry.Environment,
			ConfigPath:  store.path,
			Default:     file.DefaultProfile == entry.Name,
		},
		Human: func(writer io.Writer) error {
			defaultText := ""
			if file.DefaultProfile == entry.Name {
				defaultText = " default"
			}
			_, err := fmt.Fprintf(writer, "profile saved: %s (%s)%s\n", entry.Name, entry.Environment, defaultText)
			return err
		},
	})
}

func runProfileRemove(cmd *cobra.Command, options *profileRemoveOptions) error {
	global := optionsFromCommand(cmd)
	if global.Automation && strings.TrimSpace(options.Name) == "" {
		return newUsageError("profile remove requires --name in automation mode")
	}
	if !global.Automation {
		scanner := bufio.NewScanner(cmd.InOrStdin())
		if err := promptForMissing(scanner, cmd.ErrOrStderr(), "profile name", &options.Name); err != nil {
			return err
		}
	}
	name := strings.TrimSpace(options.Name)
	if name == "" {
		return newUsageError("profile name is required")
	}
	store, err := newProfileStore()
	if err != nil {
		return err
	}
	file, err := store.load()
	if err != nil {
		return err
	}
	file, removed := removeProfile(file, name)
	if !removed {
		return newUsageError("profile %q does not exist", name)
	}
	if err := store.save(file); err != nil {
		return err
	}
	return renderResult(cmd, commandResult{
		Data: profileMutationData{
			Name:       name,
			ConfigPath: store.path,
			Removed:    true,
		},
		Human: func(writer io.Writer) error {
			_, err := fmt.Fprintf(writer, "profile removed: %s\n", name)
			return err
		},
	})
}

func profileListDataFromFile(file profileFile) profileListData {
	data := profileListData{
		DefaultProfile: file.DefaultProfile,
		Profiles:       []profileListItem{},
	}
	for _, profile := range file.Profiles {
		item := profileListItem{
			Name:             profile.Name,
			Environment:      profile.Environment,
			Default:          profile.Name == file.DefaultProfile,
			CredentialSource: profile.CredentialSource,
		}
		if profile.Environment == environmentProduction {
			item.ProductionMarker = productionMarker
		}
		data.Profiles = append(data.Profiles, item)
	}
	return data
}

func validationErrors(result validationResult) []structuredError {
	if result.Valid {
		return nil
	}
	return []structuredError{{
		Code:    "profile_config_invalid",
		Message: "profile config validation failed",
	}}
}
