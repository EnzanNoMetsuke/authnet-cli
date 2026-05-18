package cli

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

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
	transaction.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List settled transaction history",
		RunE:  notImplemented("transaction list"),
	})

	unsettled := &cobra.Command{
		Use:   "unsettled",
		Short: "Inspect unsettled transaction set",
	}
	unsettled.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List unsettled transactions",
		RunE:  notImplemented("transaction unsettled list"),
	})
	transaction.AddCommand(unsettled)

	return transaction
}

func newCustomerProfileCommand() *cobra.Command {
	customerProfile := &cobra.Command{
		Use:   "customer-profile",
		Short: "Inspect Authorize.Net customer profiles",
	}
	customerProfile.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Inspect one customer profile",
		RunE:  notImplemented("customer-profile get"),
	})
	customerProfile.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List customer profile metadata",
		RunE:  notImplemented("customer-profile list"),
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

func renderTransactionLookupFailure(cmd *cobra.Command, resultCode string, data transactionLookupData) error {
	if data.Message == "" {
		data.Message = "transaction lookup failed"
	}
	data = sanitizeForOutput(data).(transactionLookupData)
	errorCode := "gateway_failure"
	exitCode := exitGatewayFailure
	if data.GatewayMessageCode == "E00040" || strings.Contains(strings.ToLower(data.Message), "not found") {
		errorCode = "transaction_not_found"
		exitCode = exitNotFound
	} else if data.GatewayMessageCode == "E00007" || data.GatewayMessageCode == "E00008" {
		errorCode = "authentication_failed"
		exitCode = exitAuthFailure
	}
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

func firstGatewayMessage(messages []gatewayMessage) gatewayMessage {
	if len(messages) == 0 {
		return gatewayMessage{}
	}
	return messages[0]
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
