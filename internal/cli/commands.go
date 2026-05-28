package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

const (
	defaultTransactionListLimit     = 25
	maxTransactionListLimit         = 100
	maxRawUnsettledTransactionLimit = 1000
	defaultTransactionLastRange     = "24h"
	defaultTransactionSortBy        = "timestamp"
	defaultTransactionSortOrder     = "descending"
	transactionSortByEnvName        = "AUTHNET_TX_SORT_BY"
	transactionSortOrderEnvName     = "AUTHNET_TX_SORT_ORDER"
	transactionFilterStatusEnvName  = "AUTHNET_TX_FILTER_STATUS"
	transactionFilterAmountEnvName  = "AUTHNET_TX_FILTER_AMOUNT"
	transactionFilterPaymentEnvName = "AUTHNET_TX_FILTER_PAYMENT"
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
					_, err := fmt.Fprintf(writer, "config directory: %s\nprofile config file: %s\nsensitive-data persistence: none\n", configDir, filepath.Join(configDir, profileConfigFileName))
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
	requireSubcommandFor(config)
	config.AddCommand(&cobra.Command{
		Use:   "migrate",
		Short: "Migrate legacy profile config to config.yaml",
		RunE:  runConfigMigrate,
	})
	config.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate local non-secret profile config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := newProfileStore()
			if err != nil {
				return err
			}
			loaded, err := store.loadWithSource()
			if err != nil {
				return err
			}
			file := loaded.file
			result := validateProfileFile(file)
			result.ConfigPath = loaded.path
			if !loaded.exists {
				result.Valid = false
				result.Checks = append(result.Checks, checkRow{
					Name:    "profile config",
					Status:  "failed",
					Message: "expected config.yaml was not found",
				})
			}
			warnings := result.Warnings
			if loaded.exists && len(file.Profiles) == 0 {
				warnings = append(warnings, warning{
					Code:    "no_profiles",
					Message: "no profiles are configured.",
				})
			}
			if loaded.path == store.legacyPath {
				warnings = append(warnings, warning{
					Code:    "legacy_profile_config_active",
					Message: "legacy profiles.json is active; run authnet config migrate or update a profile with authnet profile setup to migrate to config.yaml.",
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
					displayPath := loaded.path
					if !loaded.exists {
						displayPath += " (missing)"
					}
					if _, err := fmt.Fprintf(writer, "profile config: %s\nstatus: %s\nprofiles: %d\n", displayPath, status, len(file.Profiles)); err != nil {
						return err
					}
					rows := make([][]string, 0, len(result.Checks))
					for _, check := range result.Checks {
						rows = append(rows, []string{check.Name, check.Status, check.Message})
					}
					return writeHumanTable(writer, []string{"Check", "Status", "Message"}, rows, colorEnabled(optionsFromCommand(cmd)))
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

func runConfigMigrate(cmd *cobra.Command, _ []string) error {
	store, err := newProfileStore()
	if err != nil {
		return err
	}
	if _, err := os.Stat(store.path); err == nil {
		return runConfigMigrationWithExistingConfig(cmd, store)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect profile config: %w", err)
	}
	if _, err := os.Stat(store.legacyPath); err == nil {
		return runConfigMigrationFromPath(cmd, store, store.legacyPath, "completed")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect legacy profile config: %w", err)
	}
	if _, err := os.Stat(store.backupPath); err == nil {
		return runConfigRecoveryMigration(cmd, store)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect deprecated legacy profile config: %w", err)
	}
	return renderConfigMigrationResult(cmd, configMigrationData{
		Result:  "not_needed",
		Message: "No migration performed: no legacy profiles.json found; run authnet profile setup to create config.yaml.",
	}, nil)
}

func runConfigMigrationWithExistingConfig(cmd *cobra.Command, store profileStore) error {
	data, err := os.ReadFile(store.path) // #nosec G304 - path is the resolved CLI-controlled config file path.
	valid := true
	if err != nil {
		return fmt.Errorf("read profile config: %w", err)
	}
	var file profileFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		valid = false
	}
	if !valid {
		message := "Migration not needed: config.yaml exists but is invalid, check the file"
		originalPath, backupPath, err := existingLegacyMigrationPaths(store)
		if err != nil {
			return err
		}
		result := configMigrationData{
			Result:       "invalid_existing_config",
			Message:      message,
			OriginalPath: originalPath,
			ActiveConfig: store.path,
			BackupPath:   backupPath,
		}
		if renderErr := renderConfigMigrationResult(cmd, result, []warning{{
			Code:    "config_migration_not_needed_invalid_yaml",
			Message: message,
		}}); renderErr != nil {
			return renderErr
		}
		return renderedError{exitCode: exitUsageOrConfig, message: "profile config is not valid YAML"}
	}

	warnings := []warning{}
	originalPath := ""
	backupPath := ""
	if _, err := os.Stat(store.legacyPath); err == nil {
		originalPath = store.legacyPath
		renamedPath, backupWarning := store.renameLegacyBackup()
		if renamedPath != "" {
			backupPath = renamedPath
		}
		if backupWarning.Code != "" {
			warnings = append(warnings, backupWarning)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect legacy profile config: %w", err)
	}
	resultMessage := "Migration not needed: config.yaml already exists"
	if _, err := os.Stat(store.backupPath); err == nil {
		if backupPath == "" {
			backupPath = store.backupPath
		}
		resultMessage = "Migration not needed: config.yaml already exists; consider deleting DEPRECATED-profiles.json"
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect deprecated legacy profile config: %w", err)
	}
	return renderConfigMigrationResult(cmd, configMigrationData{
		Result:       "not_needed",
		Message:      resultMessage,
		OriginalPath: originalPath,
		ActiveConfig: store.path,
		BackupPath:   backupPath,
	}, warnings)
}

func existingLegacyMigrationPaths(store profileStore) (string, string, error) {
	originalPath := ""
	if _, err := os.Stat(store.legacyPath); err == nil {
		originalPath = store.legacyPath
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("inspect legacy profile config: %w", err)
	}

	backupPath := ""
	if _, err := os.Stat(store.backupPath); err == nil {
		backupPath = store.backupPath
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("inspect deprecated legacy profile config: %w", err)
	}

	return originalPath, backupPath, nil
}

func runConfigRecoveryMigration(cmd *cobra.Command, store profileStore) error {
	global := optionsFromCommand(cmd)
	if !global.Yes {
		message := "Recovery migration requires explicit approval; re-run with authnet --automation --yes config migrate to approve using DEPRECATED-profiles.json"
		if !global.Automation {
			scanner := bufio.NewScanner(cmd.InOrStdin())
			if _, err := fmt.Fprint(cmd.ErrOrStderr(), "recreate config.yaml from DEPRECATED-profiles.json? type yes to continue: "); err != nil {
				return err
			}
			if scanner.Scan() && strings.EqualFold(strings.TrimSpace(scanner.Text()), "yes") {
				return runConfigMigrationFromPath(cmd, store, store.backupPath, "recovered")
			}
			if err := scanner.Err(); err != nil {
				return err
			}
			message = "Recovery migration requires explicit approval before using DEPRECATED-profiles.json"
		}
		result := configMigrationData{
			Result:       "approval_required",
			Message:      message,
			OriginalPath: store.backupPath,
			BackupPath:   store.backupPath,
		}
		if renderErr := renderConfigMigrationResult(cmd, result, []warning{{
			Code:    "config_migration_recovery_requires_approval",
			Message: message,
		}}); renderErr != nil {
			return renderErr
		}
		return renderedError{exitCode: exitSafetyDenied, message: message}
	}
	return runConfigMigrationFromPath(cmd, store, store.backupPath, "recovered")
}

func runConfigMigrationFromPath(cmd *cobra.Command, store profileStore, sourcePath string, result string) error {
	data, err := os.ReadFile(sourcePath) // #nosec G304 - path is the resolved CLI-controlled config file path.
	if err != nil {
		return fmt.Errorf("read legacy profile config: %w", err)
	}
	var file profileFile
	if err := json.Unmarshal(data, &file); err != nil {
		return newUsageError("legacy profile config is not valid JSON: %v", err)
	}
	if err := store.save(normalizeProfileFile(file)); err != nil {
		return err
	}
	message := "Migrated legacy profiles.json to config.yaml"
	backupPath := store.backupPath
	warnings := []warning{configMigrationAdvisoryWarning()}
	switch sourcePath {
	case store.legacyPath:
		renamedPath, backupWarning := store.renameLegacyBackup()
		if renamedPath != "" {
			backupPath = renamedPath
		}
		if backupWarning.Code != "" {
			warnings = append(warnings, backupWarning)
		}
	case store.backupPath:
		message = "Recovered config.yaml from DEPRECATED-profiles.json"
		warnings = []warning{configRecoveryAdvisoryWarning()}
	}
	return renderConfigMigrationResult(cmd, configMigrationData{
		Result:       result,
		Message:      message,
		OriginalPath: sourcePath,
		ActiveConfig: store.path,
		MigratedPath: store.path,
		BackupPath:   backupPath,
	}, warnings)
}

func renderConfigMigrationResult(cmd *cobra.Command, data configMigrationData, warnings []warning) error {
	return renderResult(cmd, commandResult{
		Data:     data,
		Warnings: warnings,
		Human: func(writer io.Writer) error {
			if _, err := fmt.Fprintf(writer, "config migration %s\n", data.humanResult()); err != nil {
				return err
			}
			if data.ActiveConfig != "" {
				if _, err := fmt.Fprintf(writer, "active config: %s\n", data.ActiveConfig); err != nil {
					return err
				}
			}
			if data.BackupPath == "" {
				return nil
			}
			_, err := fmt.Fprintf(writer, "legacy backup: %s\n", data.BackupPath)
			return err
		},
	})
}

func configMigrationAdvisoryWarning() warning {
	return warning{
		Code:    "config_migrated",
		Message: "Migrated legacy profiles.json to config.yaml; config.yaml is active going forward and DEPRECATED-profiles.json is a retained legacy backup that can be deleted.",
	}
}

func configRecoveryAdvisoryWarning() warning {
	return warning{
		Code:    "config_recovered",
		Message: "Recovered config.yaml from DEPRECATED-profiles.json; config.yaml is active going forward and the retained backup can be deleted when no longer needed.",
	}
}

func configMigrationRecoveryAvailableWarning() warning {
	return warning{
		Code:    "config_migration_recovery_available",
		Message: "DEPRECATED-profiles.json exists but config.yaml was missing; authnet profile setup created a new config.yaml. To recover profiles from the backup instead, delete config.yaml and run authnet --automation --yes config migrate.",
	}
}

func newAuthCommand() *cobra.Command {
	auth := &cobra.Command{
		Use:   "auth",
		Short: "Test Authorize.Net profile authentication",
	}
	requireSubcommandFor(auth)
	auth.AddCommand(&cobra.Command{
		Use:         "test",
		Short:       "Test selected profile authentication",
		Annotations: map[string]string{rawResponseSupportAnnotation: "supported"},
		RunE:        runAuthTest,
	})
	return auth
}

func newProfileCommand() *cobra.Command {
	profile := &cobra.Command{
		Use:   "profile",
		Short: "Manage local profiles",
	}
	requireSubcommandFor(profile)

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
					rows := make([][]string, 0, len(data.Profiles))
					for _, item := range data.Profiles {
						defaultMarker := ""
						if item.Default {
							defaultMarker = "default"
						}
						rows = append(rows, []string{item.Name, item.Environment, item.ProductionMarker, item.CredentialSource.Type, defaultMarker})
					}
					return writeHumanTable(writer, []string{"Name", "Environment", "Marker", "Credential Source", "Default"}, rows, colorEnabled(optionsFromCommand(cmd)))
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
	requireSubcommandFor(transaction)
	transaction.AddCommand(&cobra.Command{
		Use:         "get <TRANSACTION_ID>",
		Short:       "Inspect one transaction",
		Args:        requireExactArgs(1, "<TRANSACTION_ID>"),
		Annotations: map[string]string{rawResponseSupportAnnotation: "supported"},
		RunE:        runTransactionGet,
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
	list.Flags().BoolVar(&listOptions.UTC, "utc", false, "show transaction timestamps in UTC")
	list.Flags().StringVar(&listOptions.SortBy, "sort-by", "", "sort transactions by timestamp, transaction_id, or amount")
	list.Flags().StringVar(&listOptions.SortOrder, "sort-order", "", "sort transactions ascending or descending")
	list.Flags().StringVar(&listOptions.FilterStatus, "status", "", "filter transactions by exact normalized status")
	list.Flags().StringVar(&listOptions.FilterAmount, "amount", "", "filter transactions by exact settled amount")
	list.Flags().StringVar(&listOptions.FilterPayment, "payment", "", "filter transactions by exact redacted card summary")
	transaction.AddCommand(list)

	unsettled := &cobra.Command{
		Use:   "unsettled",
		Short: "Inspect unsettled transaction set",
	}
	requireSubcommandFor(unsettled)
	unsettledOptions := &transactionUnsettledListOptions{
		Limit: defaultTransactionListLimit,
		Page:  1,
	}
	unsettledList := &cobra.Command{
		Use:         "list",
		Short:       "List unsettled transactions",
		Annotations: map[string]string{rawResponseSupportAnnotation: "supported"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTransactionUnsettledList(cmd, unsettledOptions)
		},
	}
	unsettledList.Flags().IntVar(&unsettledOptions.Limit, "limit", defaultTransactionListLimit, "maximum transactions to return")
	unsettledList.Flags().IntVar(&unsettledOptions.Page, "page", 1, "raw response page number to request")
	unsettledList.Flags().BoolVar(&unsettledOptions.UTC, "utc", false, "show transaction timestamps in UTC")
	unsettledList.Flags().StringVar(&unsettledOptions.SortBy, "sort-by", "", "sort transactions by timestamp, transaction_id, or amount")
	unsettledList.Flags().StringVar(&unsettledOptions.SortOrder, "sort-order", "", "sort transactions ascending or descending")
	unsettledList.Flags().StringVar(&unsettledOptions.FilterStatus, "status", "", "filter transactions by exact normalized status")
	unsettledList.Flags().StringVar(&unsettledOptions.FilterAmount, "amount", "", "filter transactions by exact settled amount")
	unsettledList.Flags().StringVar(&unsettledOptions.FilterPayment, "payment", "", "filter transactions by exact redacted card summary")
	unsettled.AddCommand(unsettledList)
	transaction.AddCommand(unsettled)

	return transaction
}

func newCustomerProfileCommand() *cobra.Command {
	customerProfile := &cobra.Command{
		Use:   "customer-profile",
		Short: "Inspect Authorize.Net customer profiles",
	}
	requireSubcommandFor(customerProfile)
	getOptions := &customerProfileGetOptions{}
	get := &cobra.Command{
		Use:   "get <CUSTOMER_PROFILE_ID>",
		Short: "Inspect one customer profile",
		Args:  requireExactArgs(1, "<CUSTOMER_PROFILE_ID>"),
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
	requireSubcommandFor(responseCode)
	explainOptions := &responseCodeExplainOptions{}
	explain := &cobra.Command{
		Use:   "explain <CODE>",
		Short: "Explain a gateway or API response code",
		Args:  requireExactArgs(1, "<CODE>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResponseCodeExplain(cmd, args, explainOptions)
		},
	}
	explain.Flags().StringVar(&explainOptions.Family, "family", "", "code family: api_message, validation, transaction_response, avs, or cvv")
	responseCode.AddCommand(explain)
	return responseCode
}

func runResponseCodeExplain(cmd *cobra.Command, args []string, options *responseCodeExplainOptions) error {
	family, err := normalizeResponseCodeFamily(options.Family)
	if err != nil {
		return err
	}
	data := explainResponseCode(args[0], family)
	result := commandResult{
		Data: data,
		Human: func(writer io.Writer) error {
			if len(data.Matches) == 0 {
				_, err := fmt.Fprintf(writer, "response code %s: no local explanation found\nreference: %s reviewed %s\n", data.QueryCode, data.Reference.Version, data.Reference.Reviewed)
				return err
			}
			if len(data.Matches) > 1 {
				if _, err := fmt.Fprintln(writer, ambiguousResponseCodeMessage(data)); err != nil {
					return err
				}
				rows := make([][]string, 0, len(data.Matches))
				for _, match := range data.Matches {
					rows = append(rows, []string{match.Family, match.Title})
				}
				return writeHumanTable(writer, []string{"Family", "Title"}, rows, colorEnabled(optionsFromCommand(cmd)))
			}
			match := data.Matches[0]
			if _, err := fmt.Fprintf(writer, "code: %s\nfamily: %s\nmeaning: %s\n", match.Code, match.Family, match.Meaning); err != nil {
				return err
			}
			if len(match.Causes) > 0 {
				if _, err := fmt.Fprintln(writer, "likely causes:"); err != nil {
					return err
				}
				for _, cause := range match.Causes {
					if _, err := fmt.Fprintf(writer, "- %s\n", cause); err != nil {
						return err
					}
				}
			}
			if len(match.NextSteps) > 0 {
				if _, err := fmt.Fprintln(writer, "recommended next steps:"); err != nil {
					return err
				}
				for _, step := range match.NextSteps {
					if _, err := fmt.Fprintf(writer, "- %s\n", step); err != nil {
						return err
					}
				}
			}
			_, err := fmt.Fprintf(writer, "reference: %s reviewed %s\n", data.Reference.Version, data.Reference.Reviewed)
			return err
		},
	}
	if len(data.Matches) == 0 {
		result.Errors = []structuredError{{
			Code:    "response_code_not_found",
			Message: fmt.Sprintf("response code %s was not found in the local curated reference", data.QueryCode),
		}}
		if err := renderResult(cmd, result); err != nil {
			return err
		}
		return renderedError{exitCode: exitNotFound, message: result.Errors[0].Message}
	}
	if len(data.Matches) > 1 && family == "" {
		result.Errors = []structuredError{{
			Code:    "ambiguous_response_code",
			Message: ambiguousResponseCodeMessage(data),
		}}
		if err := renderResult(cmd, result); err != nil {
			return err
		}
		return renderedError{exitCode: exitUsageOrConfig, message: result.Errors[0].Message}
	}
	return renderResult(cmd, result)
}

func newSandboxCommand() *cobra.Command {
	sandbox := &cobra.Command{
		Use:   "sandbox",
		Short: "Run sandbox-only test helpers",
	}
	requireSubcommandFor(sandbox)
	charge := &cobra.Command{
		Use:   "charge",
		Short: "Run sandbox card charge scenarios",
	}
	requireSubcommandFor(charge)
	for _, scenario := range []string{"approved", "declined", "avs", "cvv", "duplicate"} {
		options := sandboxChargeOptions{
			Card:   defaultSandboxCardAlias,
			Amount: defaultSandboxChargeAmount,
			Window: defaultSandboxDuplicateWindow,
		}
		command := &cobra.Command{
			Use:   scenario,
			Short: sandboxScenarioShort(scenario),
			RunE: func(cmd *cobra.Command, _ []string) error {
				options.CardExplicit = cmd.Flags().Changed("card")
				return runSandboxCharge(cmd, scenario, &options)
			},
		}
		command.Flags().StringVar(&options.Card, "card", defaultSandboxCardAlias, "sandbox card alias: visa, mastercard, amex, discover")
		command.Flags().StringVar(&options.Amount, "amount", defaultSandboxChargeAmount, "charge amount")
		if scenario == "avs" || scenario == "cvv" {
			command.Flags().StringVar(&options.Variant, "variant", "", sandboxVariantHelp(scenario))
		}
		if scenario == "duplicate" {
			command.Flags().IntVar(&options.Window, "window", defaultSandboxDuplicateWindow, "duplicate window in seconds")
		}
		charge.AddCommand(command)
	}
	sandbox.AddCommand(charge)
	return sandbox
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
	From          string
	To            string
	Last          string
	Limit         int
	UTC           bool
	SortBy        string
	SortOrder     string
	FilterStatus  string
	FilterAmount  string
	FilterPayment string
}

type transactionUnsettledListOptions struct {
	Limit         int
	Page          int
	UTC           bool
	SortBy        string
	SortOrder     string
	FilterStatus  string
	FilterAmount  string
	FilterPayment string
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

type configMigrationData struct {
	Result       string `json:"result"`
	Message      string `json:"message"`
	ActiveConfig string `json:"active_config"`
	OriginalPath string `json:"original_path"`
	MigratedPath string `json:"migrated_path"`
	BackupPath   string `json:"backup_path"`
}

func (data configMigrationData) humanResult() string {
	if data.Message == "" || data.Result == "completed" || data.Result == "recovered" {
		return data.Result
	}
	return data.Message
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

type rawGatewayResponseData struct {
	RawGatewayResponse json.RawMessage `json:"raw_gateway_response"`
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
	timestampsInUTC           bool
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
	response, rawResponse, err := client.authenticate(cmd.Context(), profile.Credentials)
	if err != nil {
		return err
	}

	message := firstGatewayMessage(response.Messages.Message)
	if options.RawResponse {
		if renderErr := renderRawGatewayResponse(cmd, rawResponse); renderErr != nil {
			return renderErr
		}
		if !strings.EqualFold(response.Messages.ResultCode, "Ok") {
			failureMessage := message.Text
			if failureMessage == "" {
				failureMessage = "authentication response did not include a message"
			}
			return renderedError{exitCode: exitAuthFailure, message: failureMessage}
		}
		return nil
	}

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

func renderRawGatewayResponse(cmd *cobra.Command, rawResponse []byte, warnings ...warning) error {
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(rawResponse, []byte("\xef\xbb\xbf")))
	if len(trimmed) == 0 {
		return cliError{
			exitCode: exitGatewayFailure,
			code:     "gateway_response_invalid",
			message:  "Authorize.Net returned an empty raw gateway response",
		}
	}
	if optionsFromCommand(cmd).JSON {
		if !json.Valid(trimmed) {
			return cliError{
				exitCode: exitGatewayFailure,
				code:     "gateway_response_invalid",
				message:  "Authorize.Net returned an invalid JSON raw gateway response",
			}
		}
		return renderResult(cmd, commandResult{
			Data: rawGatewayResponseData{
				RawGatewayResponse: json.RawMessage(trimmed),
			},
			Warnings: warnings,
			Redacted: boolPointer(false),
		})
	}
	humanWarnings := append([]warning{}, optionsFromCommand(cmd).PreferenceWarnings...)
	humanWarnings = append(humanWarnings, warnings...)
	if err := renderHumanWarnings(cmd.ErrOrStderr(), humanWarnings, colorEnabled(optionsFromCommand(cmd))); err != nil {
		return err
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), string(trimmed))
	return err
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
	response, rawResponse, err := client.getTransactionDetails(cmd.Context(), profile.Credentials, transactionID)
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
	if options.RawResponse {
		if renderErr := renderRawGatewayResponse(cmd, rawResponse); renderErr != nil {
			return renderErr
		}
		if !strings.EqualFold(response.Messages.ResultCode, "Ok") {
			if data.Message == "" {
				data.Message = "transaction lookup failed"
			}
			_, exitCode := gatewayFailureMapping(data.GatewayMessageCode, data.Message, "transaction_not_found")
			return renderedError{exitCode: exitCode, message: data.Message}
		}
		return nil
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
	sortOptions, err := resolveTransactionSortOptions(cmd, listOptions.SortBy, listOptions.SortOrder)
	if err != nil {
		return err
	}
	filterOptions, err := resolveTransactionFilterOptions(cmd, listOptions.FilterStatus, listOptions.FilterAmount, listOptions.FilterPayment)
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
		timestampsInUTC:    listOptions.UTC,
	}
	if !strings.EqualFold(batches.Messages.ResultCode, "Ok") {
		return renderTransactionListFailure(cmd, "transaction list", batches.Messages.ResultCode, data)
	}

	hasMoreCandidates := false
	for _, batch := range batches.BatchList {
		pageLimit := limit
		if !filterOptions.empty() {
			pageLimit = transactionListCandidateLimit(limit, sortOptions)
		}
		for offset := 1; ; offset++ {
			response, err := client.getTransactionList(cmd.Context(), profile.Credentials, batch.BatchID.String(), gatewayPaging{
				Limit:  pageLimit,
				Offset: offset,
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
			data.Transactions = append(data.Transactions, filterTransactionListItems(transactionListItems(response.Transactions, batch.BatchID.String()), filterOptions)...)
			canStopAfterFilteredLimit := transactionListCanStopAfterFilteredLimit(data.Transactions, limit, sortOptions)
			if filterOptions.empty() || len(response.Transactions) < pageLimit || canStopAfterFilteredLimit {
				if len(response.Transactions) >= pageLimit && (filterOptions.empty() || canStopAfterFilteredLimit) {
					hasMoreCandidates = true
				}
				break
			}
		}
	}
	sortTransactionListItems(data.Transactions, sortOptions)
	hasMoreCandidates = hasMoreCandidates || len(data.Transactions) > limit
	if len(data.Transactions) > limit {
		data.Transactions = data.Transactions[:limit]
	}
	data.Pagination.ReturnedCount = len(data.Transactions)
	data.Pagination.HasMore = hasMoreCandidates
	return renderTransactionListResult(cmd, data)
}

func runTransactionUnsettledList(cmd *cobra.Command, listOptions *transactionUnsettledListOptions) error {
	options := optionsFromCommand(cmd)
	limit, err := transactionUnsettledListLimit(listOptions.Limit, options.RawResponse)
	if err != nil {
		return err
	}
	sortOptions, err := resolveTransactionSortOptions(cmd, listOptions.SortBy, listOptions.SortOrder)
	if err != nil {
		return err
	}

	if !options.RawResponse && cmd.Flags().Lookup("page").Changed {
		return newUsageError("--page is only supported with --raw-response for transaction unsettled list")
	}
	rawRequestOptions := gatewayUnsettledTransactionListRequestOptions{}
	if options.RawResponse {
		rawRequestOptions, err = resolveRawUnsettledTransactionListRequestOptions(cmd, listOptions, limit, sortOptions)
		if err != nil {
			return err
		}
	}
	profile, err := loadSelectedProfileWithCredentials(options, "transaction unsettled list")
	if err != nil {
		return err
	}
	client, err := newGatewayClient(profile.Entry.Environment)
	if err != nil {
		return err
	}
	if options.RawResponse {
		response, rawResponse, err := client.getUnsettledTransactionListRaw(cmd.Context(), profile.Credentials, rawRequestOptions)
		if err != nil {
			return err
		}
		warnings := []warning{}
		if strings.EqualFold(response.Messages.ResultCode, "Ok") {
			if rawUnsettledTransactionListMayHaveNextPage(response, rawRequestOptions) {
				warnings = append(warnings, rawResponseMorePagesWarning(rawRequestOptions.Paging.Offset+1))
			}
		}
		if renderErr := renderRawGatewayResponse(cmd, rawResponse, warnings...); renderErr != nil {
			return renderErr
		}
		if !strings.EqualFold(response.Messages.ResultCode, "Ok") {
			message := firstGatewayMessage(response.Messages.Message)
			failureMessage := message.Text
			if failureMessage == "" {
				failureMessage = "unsettled transaction list failed"
			}
			_, exitCode := gatewayFailureMapping(message.Code, failureMessage, "gateway_failure")
			return renderedError{exitCode: exitCode, message: failureMessage}
		}
		return nil
	}
	filterOptions, err := resolveTransactionFilterOptions(cmd, listOptions.FilterStatus, listOptions.FilterAmount, listOptions.FilterPayment)
	if err != nil {
		return err
	}
	data := transactionListData{
		ProfileName:               profile.Entry.Name,
		EnvironmentClassification: profile.Entry.Environment,
		ProductionMarker:          productionMarkerForEnvironment(profile.Entry.Environment),
		Kind:                      "unsettled",
		Pagination: transactionListPagination{
			RequestedLimit: limit,
		},
		Transactions:    []transactionListItem{},
		timestampsInUTC: listOptions.UTC,
	}
	candidateLimit := transactionListCandidateLimit(limit, sortOptions)
	hasMoreCandidates := false
	for offset := 1; ; offset++ {
		response, err := client.getUnsettledTransactionList(cmd.Context(), profile.Credentials, gatewayPaging{
			Limit:  candidateLimit,
			Offset: offset,
		})
		if err != nil {
			return err
		}
		message := firstGatewayMessage(response.Messages.Message)
		data.GatewayMessageCode = message.Code
		data.Message = message.Text
		if !strings.EqualFold(response.Messages.ResultCode, "Ok") {
			return renderTransactionListFailure(cmd, "transaction unsettled list", response.Messages.ResultCode, data)
		}
		data.Transactions = append(data.Transactions, filterTransactionListItems(transactionListItems(response.Transactions, ""), filterOptions)...)
		canStopAfterFilteredLimit := transactionListCanStopAfterFilteredLimit(data.Transactions, limit, sortOptions)
		if filterOptions.empty() || len(response.Transactions) < candidateLimit || canStopAfterFilteredLimit {
			if len(response.Transactions) >= candidateLimit && (filterOptions.empty() || canStopAfterFilteredLimit) {
				hasMoreCandidates = true
			}
			break
		}
	}
	sortTransactionListItems(data.Transactions, sortOptions)
	data.Pagination.HasMore = len(data.Transactions) > limit || hasMoreCandidates
	if len(data.Transactions) > limit {
		data.Transactions = data.Transactions[:limit]
	}
	data.Pagination.ReturnedCount = len(data.Transactions)
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
			paymentProfileRows := make([][]string, 0, len(data.PaymentProfiles))
			for _, item := range data.PaymentProfiles {
				paymentProfileRows = append(paymentProfileRows, []string{
					item.CustomerPaymentProfileID,
					firstNonEmpty(item.AccountType, "(none)"),
					firstNonEmpty(item.AccountNumber, "(none)"),
				})
			}
			if writeErr := writeHumanTable(writer, []string{"Payment Profile", "Type", "Account"}, paymentProfileRows, colorEnabled(optionsFromCommand(cmd))); writeErr != nil {
				return writeErr
			}
			shippingAddressRows := make([][]string, 0, len(data.ShippingAddresses))
			for _, item := range data.ShippingAddresses {
				shippingAddressRows = append(shippingAddressRows, []string{item.CustomerAddressID})
			}
			return writeHumanTable(writer, []string{"Shipping Address"}, shippingAddressRows, colorEnabled(optionsFromCommand(cmd)))
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
			rows := make([][]string, 0, len(data.CustomerProfileIDs))
			for _, customerProfileID := range data.CustomerProfileIDs {
				rows = append(rows, []string{customerProfileID})
			}
			return writeHumanTable(writer, []string{"Customer Profile"}, rows, colorEnabled(optionsFromCommand(cmd)))
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
	timestampsInUTC := data.timestampsInUTC
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
			rows := make([][]string, 0, len(data.Transactions))
			for _, item := range data.Transactions {
				timestamp := item.SubmitTimeLocal
				if timestampsInUTC {
					timestamp = item.SubmitTimeUTC
				}
				rows = append(rows, []string{
					item.TransactionID,
					firstNonEmpty(timestamp, "(none)"),
					item.TransactionStatus,
					firstNonEmpty(item.SettleAmount, "(none)"),
					strings.TrimSpace(firstNonEmpty(item.Payment.AccountType, "") + " " + firstNonEmpty(item.Payment.AccountNumber, "")),
				})
			}
			for i := range rows {
				if rows[i][4] == "" {
					rows[i][4] = "(none)"
				}
			}
			return writeHumanTable(writer, []string{"Transaction", "Timestamp", "Status", "Amount", "Payment"}, rows, colorEnabled(optionsFromCommand(cmd)))
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

func transactionUnsettledListLimit(limit int, rawResponse bool) (int, error) {
	if rawResponse {
		return rawUnsettledTransactionListLimit(limit)
	}
	return normalizedTransactionListLimit(limit)
}

func rawUnsettledTransactionListLimit(limit int) (int, error) {
	if limit < 1 {
		return 0, newUsageError("--limit must be at least 1")
	}
	if limit > maxRawUnsettledTransactionLimit {
		return 0, newUsageError("--limit must be at most %d in raw response mode", maxRawUnsettledTransactionLimit)
	}
	return limit, nil
}

func transactionListCandidateLimit(limit int, sortOptions transactionSortOptions) int {
	if sortOptions.By == defaultTransactionSortBy && sortOptions.Order == defaultTransactionSortOrder {
		return limit
	}
	return maxTransactionListLimit
}

func transactionListCanStopAfterFilteredLimit(items []transactionListItem, limit int, sortOptions transactionSortOptions) bool {
	return len(items) >= limit && sortOptions.By == defaultTransactionSortBy && sortOptions.Order == defaultTransactionSortOrder
}

type transactionSortOptions struct {
	By          string
	Order       string
	BySource    string
	OrderSource string
}

type transactionSortPreferences struct {
	SortBy              string
	SortBySource        string
	SortOrder           string
	SortOrderSource     string
	FilterStatus        string
	FilterStatusSource  string
	FilterAmount        string
	FilterAmountSource  string
	FilterPayment       string
	FilterPaymentSource string
}

type transactionFilterOptions struct {
	Status        string
	StatusSource  string
	Amount        string
	AmountSource  string
	Payment       string
	PaymentSource string
}

func (options transactionFilterOptions) empty() bool {
	return options.Status == "" && options.Amount == "" && options.Payment == ""
}

func resolveTransactionSortOptions(cmd *cobra.Command, sortBy string, sortOrder string) (transactionSortOptions, error) {
	var preferences transactionSortPreferences
	preferencesLoaded := false
	loadPreferences := func() (transactionSortPreferences, error) {
		if preferencesLoaded {
			return preferences, nil
		}
		var err error
		preferences, err = loadTransactionSortPreferences()
		preferencesLoaded = true
		return preferences, err
	}

	sortBy, sortBySource, err := resolveTransactionSortValue(cmd, "sort-by", sortBy, transactionSortByEnvName, defaultTransactionSortBy, loadPreferences)
	if err != nil {
		return transactionSortOptions{}, err
	}
	sortOrder, sortOrderSource, err := resolveTransactionSortValue(cmd, "sort-order", sortOrder, transactionSortOrderEnvName, defaultTransactionSortOrder, loadPreferences)
	if err != nil {
		return transactionSortOptions{}, err
	}
	options := transactionSortOptions{
		By:          strings.TrimSpace(sortBy),
		Order:       strings.TrimSpace(sortOrder),
		BySource:    sortBySource,
		OrderSource: sortOrderSource,
	}
	switch options.By {
	case "timestamp", "transaction_id", "amount":
	default:
		return transactionSortOptions{}, invalidTransactionSortByError(options.By, sortBySource)
	}
	switch options.Order {
	case "ascending", "descending":
	default:
		return transactionSortOptions{}, invalidTransactionSortOrderError(options.Order, sortOrderSource)
	}
	return options, nil
}

func resolveTransactionFilterOptions(cmd *cobra.Command, status string, amount string, payment string) (transactionFilterOptions, error) {
	options, err := resolveTransactionFilterValues(cmd, status, amount, payment)
	if err != nil {
		return transactionFilterOptions{}, err
	}
	if options.Status != "" && !validTransactionFilterStatus(options.Status) {
		return transactionFilterOptions{}, invalidTransactionFilterStatusError(options.Status, options.StatusSource)
	}
	if options.Amount != "" {
		normalized, err := normalizeTransactionFilterAmount(options.Amount)
		if err != nil {
			return transactionFilterOptions{}, invalidTransactionFilterAmountError(options.Amount, options.AmountSource)
		}
		options.Amount = normalized
	}
	if options.Payment != "" && !validTransactionFilterPayment(options.Payment) {
		return transactionFilterOptions{}, invalidTransactionFilterPaymentError(options.Payment, options.PaymentSource)
	}
	return options, nil
}

func resolveTransactionFilterValues(cmd *cobra.Command, status string, amount string, payment string) (transactionFilterOptions, error) {
	var preferences transactionSortPreferences
	preferencesLoaded := false
	loadPreferences := func() (transactionSortPreferences, error) {
		if preferencesLoaded {
			return preferences, nil
		}
		var err error
		preferences, err = loadTransactionSortPreferences()
		preferencesLoaded = true
		return preferences, err
	}

	resolvedStatus, statusSource, err := resolveTransactionFilterValue(cmd, "status", status, transactionFilterStatusEnvName, loadPreferences)
	if err != nil {
		return transactionFilterOptions{}, err
	}
	resolvedAmount, amountSource, err := resolveTransactionFilterValue(cmd, "amount", amount, transactionFilterAmountEnvName, loadPreferences)
	if err != nil {
		return transactionFilterOptions{}, err
	}
	resolvedPayment, paymentSource, err := resolveTransactionFilterValue(cmd, "payment", payment, transactionFilterPaymentEnvName, loadPreferences)
	if err != nil {
		return transactionFilterOptions{}, err
	}

	options := transactionFilterOptions{
		Status:        strings.TrimSpace(resolvedStatus),
		StatusSource:  statusSource,
		Amount:        strings.TrimSpace(resolvedAmount),
		AmountSource:  amountSource,
		Payment:       strings.TrimSpace(resolvedPayment),
		PaymentSource: paymentSource,
	}
	return options, nil
}

func resolveRawUnsettledTransactionListRequestOptions(cmd *cobra.Command, listOptions *transactionUnsettledListOptions, limit int, sortOptions transactionSortOptions) (gatewayUnsettledTransactionListRequestOptions, error) {
	if listOptions.Page < 1 {
		return gatewayUnsettledTransactionListRequestOptions{}, newUsageError("--page must be at least 1")
	}
	filterOptions, err := resolveTransactionFilterValues(cmd, listOptions.FilterStatus, listOptions.FilterAmount, listOptions.FilterPayment)
	if err != nil {
		return gatewayUnsettledTransactionListRequestOptions{}, err
	}
	if err := validateRawUnsettledTransactionListControls(sortOptions, filterOptions); err != nil {
		return gatewayUnsettledTransactionListRequestOptions{}, err
	}

	requestOptions := gatewayUnsettledTransactionListRequestOptions{
		Sorting: &gatewaySorting{
			OrderBy:         rawUnsettledTransactionListSortField(sortOptions.By),
			OrderDescending: sortOptions.Order == "descending",
		},
		Paging: gatewayPaging{
			Limit:  limit,
			Offset: listOptions.Page,
		},
	}
	if filterOptions.Status != "" {
		requestOptions.Status = filterOptions.Status
	}
	return requestOptions, nil
}

func validateRawUnsettledTransactionListControls(sortOptions transactionSortOptions, filterOptions transactionFilterOptions) error {
	if sortOptions.By == "amount" {
		return unsupportedRawUnsettledTransactionListControlError(sortOptions.BySource, sortOptions.By, "amount sorting is not gateway-native")
	}
	if filterOptions.Status != "" {
		switch filterOptions.Status {
		case "any", "pendingApproval":
		default:
			return unsupportedRawUnsettledTransactionListControlError(filterOptions.StatusSource, filterOptions.Status, "only any and pendingApproval are gateway-native status values")
		}
	}
	if filterOptions.Amount != "" {
		return unsupportedRawUnsettledTransactionListControlError(filterOptions.AmountSource, filterOptions.Amount, "amount filtering requires normalized transaction output")
	}
	if filterOptions.Payment != "" {
		return unsupportedRawUnsettledTransactionListControlError(filterOptions.PaymentSource, filterOptions.Payment, "payment filtering requires normalized transaction output")
	}
	return nil
}

func rawUnsettledTransactionListSortField(sortBy string) string {
	switch sortBy {
	case "transaction_id":
		return "id"
	default:
		return "submitTimeUTC"
	}
}

func unsupportedRawUnsettledTransactionListControlError(source string, value string, reason string) error {
	return newUsageError("unsupported %s value %q in raw transaction unsettled list mode: %s; exact transaction-status, amount, and payment filtering remain available in normalized mode", source, value, reason)
}

func rawUnsettledTransactionListMayHaveNextPage(response getUnsettledTransactionListResponseEnvelope, requestOptions gatewayUnsettledTransactionListRequestOptions) bool {
	if requestOptions.Paging.Limit <= 0 {
		return false
	}
	return len(response.Transactions) >= requestOptions.Paging.Limit
}

func rawResponseMorePagesWarning(nextPage int) warning {
	return warning{
		Code:    "raw_response_more_pages",
		Message: fmt.Sprintf("Another raw gateway page may be available; rerun with --page %d and the same --limit and raw-mode controls to retrieve it.", nextPage),
	}
}

func resolveTransactionFilterValue(cmd *cobra.Command, flagName string, flagValue string, envName string, loadPreferences func() (transactionSortPreferences, error)) (string, string, error) {
	if cmd.Flags().Lookup(flagName).Changed {
		return strings.TrimSpace(flagValue), "--" + flagName, nil
	}
	if envValue := strings.TrimSpace(os.Getenv(envName)); envValue != "" {
		return envValue, envName, nil
	}
	preferences, err := loadPreferences()
	if err != nil {
		return "", "", err
	}
	switch flagName {
	case "status":
		if preferences.FilterStatus != "" {
			return preferences.FilterStatus, preferences.FilterStatusSource, nil
		}
	case "amount":
		if preferences.FilterAmount != "" {
			return preferences.FilterAmount, preferences.FilterAmountSource, nil
		}
	case "payment":
		if preferences.FilterPayment != "" {
			return preferences.FilterPayment, preferences.FilterPaymentSource, nil
		}
	}
	return "", "default", nil
}

func validTransactionFilterStatus(value string) bool {
	switch value {
	case "approvedReview",
		"authorizedPendingCapture",
		"authorizedPendingRelease",
		"capturedPendingSettlement",
		"chargeback",
		"chargebackReversal",
		"communicationError",
		"couldNotVoid",
		"declined",
		"expired",
		"failedReview",
		"FDSPendingReview",
		"FDSAuthorizedPendingReview",
		"generalError",
		"pendingFinalSettlement",
		"pendingSettlement",
		"refundPendingSettlement",
		"refundSettledSuccessfully",
		"returnedItem",
		"settledSuccessfully",
		"settlementError",
		"underReview",
		"updatingSettlement",
		"voided":
		return true
	default:
		return false
	}
}

func normalizeTransactionFilterAmount(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "$") {
		return "", fmt.Errorf("invalid amount")
	}
	whole, fraction, hasFraction := strings.Cut(value, ".")
	if whole == "" || strings.HasPrefix(whole, "+") || strings.HasPrefix(whole, "-") {
		return "", fmt.Errorf("invalid amount")
	}
	if !allDigits(whole) {
		return "", fmt.Errorf("invalid amount")
	}
	whole = strings.TrimLeft(whole, "0")
	if whole == "" {
		whole = "0"
	}
	if !hasFraction {
		if amount, err := strconv.ParseFloat(whole, 64); err != nil || amount <= 0 {
			return "", fmt.Errorf("invalid amount")
		}
		return whole + ".00", nil
	}
	if fraction == "" || len(fraction) > 2 || !allDigits(fraction) {
		return "", fmt.Errorf("invalid amount")
	}
	normalized := whole + "." + fraction + strings.Repeat("0", 2-len(fraction))
	if amount, err := strconv.ParseFloat(normalized, 64); err != nil || amount <= 0 {
		return "", fmt.Errorf("invalid amount")
	}
	return normalized, nil
}

func allDigits(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return value != ""
}

func validTransactionFilterPayment(value string) bool {
	cardType, accountNumber, ok := strings.Cut(strings.TrimSpace(value), " ")
	if !ok || strings.Contains(accountNumber, " ") {
		return false
	}
	switch cardType {
	case "Visa", "MasterCard", "AmericanExpress", "Discover", "DinersClub", "JCB":
	default:
		return false
	}
	if len(accountNumber) != 8 || !strings.HasPrefix(accountNumber, "XXXX") {
		return false
	}
	return allDigits(accountNumber[4:])
}

func invalidTransactionFilterStatusError(value string, source string) error {
	return newUsageError("invalid %s value %q: expected Authorize.Net transactionStatusEnum value", source, value)
}

func invalidTransactionFilterAmountError(value string, source string) error {
	return newUsageError("invalid %s value %q: expected positive integer or decimal amount with up to two decimal places", source, value)
}

func invalidTransactionFilterPaymentError(value string, source string) error {
	return newUsageError("invalid %s value %q: expected <CardType> XXXXdddd for Visa, MasterCard, AmericanExpress, Discover, DinersClub, or JCB", source, value)
}

func resolveTransactionSortValue(cmd *cobra.Command, flagName string, flagValue string, envName string, defaultValue string, loadPreferences func() (transactionSortPreferences, error)) (string, string, error) {
	if cmd.Flags().Lookup(flagName).Changed {
		return strings.TrimSpace(flagValue), "--" + flagName, nil
	}
	if envValue := strings.TrimSpace(os.Getenv(envName)); envValue != "" {
		return envValue, envName, nil
	}
	preferences, err := loadPreferences()
	if err != nil {
		return "", "", err
	}
	switch flagName {
	case "sort-order":
		if preferences.SortOrder != "" {
			return preferences.SortOrder, preferences.SortOrderSource, nil
		}
	default:
		if preferences.SortBy != "" {
			return preferences.SortBy, preferences.SortBySource, nil
		}
	}
	return defaultValue, "default", nil
}

func loadTransactionSortPreferences() (transactionSortPreferences, error) {
	dir, err := authnetConfigDir()
	if err != nil {
		return transactionSortPreferences{}, err
	}
	configPath := filepath.Join(dir, profileConfigFileName)
	file, err := loadPreferencesFile(configPath)
	if err != nil {
		return transactionSortPreferences{}, err
	}
	preferences := transactionSortPreferences{}
	if value, ok := nestedStringPreference(file.Preferences, "transaction_list", "sort_by"); ok {
		preferences.SortBy = value
		preferences.SortBySource = "preferences.transaction_list.sort_by in " + configPath
	}
	if value, ok := nestedStringPreference(file.Preferences, "transaction_list", "sort_order"); ok {
		preferences.SortOrder = value
		preferences.SortOrderSource = "preferences.transaction_list.sort_order in " + configPath
	}
	if value, ok := nestedStringPreference(file.Preferences, "transaction_list", "filter", "status"); ok {
		preferences.FilterStatus = value
		preferences.FilterStatusSource = "preferences.transaction_list.filter.status in " + configPath
	}
	if value, ok := nestedStringPreference(file.Preferences, "transaction_list", "filter", "amount"); ok {
		preferences.FilterAmount = value
		preferences.FilterAmountSource = "preferences.transaction_list.filter.amount in " + configPath
	}
	if value, ok := nestedStringPreference(file.Preferences, "transaction_list", "filter", "payment"); ok {
		preferences.FilterPayment = value
		preferences.FilterPaymentSource = "preferences.transaction_list.filter.payment in " + configPath
	}
	return preferences, nil
}

func invalidTransactionSortByError(value string, source string) error {
	switch source {
	case "--sort-by":
		return newUsageError("invalid --sort-by value %q: expected timestamp, transaction_id, or amount", value)
	case transactionSortByEnvName:
		return newUsageError("invalid AUTHNET_TX_SORT_BY value %q: expected timestamp, transaction_id, or amount", value)
	default:
		return newExitingUsageError("invalid %s value %q: expected timestamp, transaction_id, or amount", source, value)
	}
}

func invalidTransactionSortOrderError(value string, source string) error {
	switch source {
	case "--sort-order":
		return newUsageError("invalid --sort-order value %q: expected ascending or descending", value)
	case transactionSortOrderEnvName:
		return newUsageError("invalid AUTHNET_TX_SORT_ORDER value %q: expected ascending or descending", value)
	default:
		return newExitingUsageError("invalid %s value %q: expected ascending or descending", source, value)
	}
}

func sortTransactionListItems(items []transactionListItem, options transactionSortOptions) {
	sort.SliceStable(items, func(leftIndex, rightIndex int) bool {
		left := items[leftIndex]
		right := items[rightIndex]
		cmp := compareTransactionPrimarySort(left, right, options.By)
		if cmp == 0 {
			cmp = strings.Compare(left.TransactionID, right.TransactionID)
			return cmp > 0
		}
		if options.Order == "ascending" {
			return cmp < 0
		}
		return cmp > 0
	})
}

func filterTransactionListItems(items []transactionListItem, options transactionFilterOptions) []transactionListItem {
	if options.empty() {
		return items
	}
	filtered := make([]transactionListItem, 0, len(items))
	for _, item := range items {
		if options.Status != "" && item.TransactionStatus != options.Status {
			continue
		}
		if options.Amount != "" && normalizedTransactionItemAmount(item.SettleAmount) != options.Amount {
			continue
		}
		if options.Payment != "" && transactionItemPaymentSummary(item.Payment) != options.Payment {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func normalizedTransactionItemAmount(value string) string {
	normalized, err := normalizeTransactionFilterAmount(value)
	if err != nil {
		return strings.TrimSpace(value)
	}
	return normalized
}

func transactionItemPaymentSummary(payment paymentSummary) string {
	return strings.TrimSpace(firstNonEmpty(payment.AccountType, payment.CardType) + " " + payment.AccountNumber)
}

func compareTransactionPrimarySort(left transactionListItem, right transactionListItem, sortBy string) int {
	switch sortBy {
	case "transaction_id":
		return strings.Compare(left.TransactionID, right.TransactionID)
	case "amount":
		return compareOptionalFloat(left.SettleAmount, right.SettleAmount)
	default:
		return compareOptionalTime(firstNonEmpty(left.SubmitTimeUTC, left.SubmitTimeLocal), firstNonEmpty(right.SubmitTimeUTC, right.SubmitTimeLocal))
	}
}

func compareOptionalTime(left string, right string) int {
	leftTime, leftOK := parseTransactionSortTime(left)
	rightTime, rightOK := parseTransactionSortTime(right)
	switch {
	case !leftOK && !rightOK:
		return 0
	case !leftOK:
		return -1
	case !rightOK:
		return 1
	case leftTime.Before(rightTime):
		return -1
	case leftTime.After(rightTime):
		return 1
	default:
		return 0
	}
}

func parseTransactionSortTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func compareOptionalFloat(left string, right string) int {
	leftAmount, leftOK := parseTransactionSortAmount(left)
	rightAmount, rightOK := parseTransactionSortAmount(right)
	switch {
	case !leftOK && !rightOK:
		return 0
	case !leftOK:
		return -1
	case !rightOK:
		return 1
	case leftAmount < rightAmount:
		return -1
	case leftAmount > rightAmount:
		return 1
	default:
		return 0
	}
}

func parseTransactionSortAmount(value string) (float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	amount, err := strconv.ParseFloat(value, 64)
	return amount, err == nil
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
	configExists := true
	if _, err := os.Stat(store.path); errors.Is(err, os.ErrNotExist) {
		configExists = false
	} else if err != nil {
		return fmt.Errorf("inspect profile config: %w", err)
	}
	recoverableBackupExists := false
	if !configExists {
		if _, err := os.Stat(store.backupPath); err == nil {
			recoverableBackupExists = true
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect deprecated legacy profile config: %w", err)
		}
	}
	loaded, err := store.loadWithSource()
	if err != nil {
		return err
	}
	file := loaded.file
	file, err = upsertProfile(file, entry, options.Default)
	if err != nil {
		return err
	}
	if err := store.save(file); err != nil {
		return err
	}
	warnings := []warning{}
	if loaded.path == store.legacyPath {
		warnings = append(warnings, configMigrationAdvisoryWarning())
		_, backupWarning := store.renameLegacyBackup()
		if backupWarning.Code != "" {
			warnings = append(warnings, backupWarning)
		}
	} else if !configExists && recoverableBackupExists {
		warnings = append(warnings, configMigrationRecoveryAvailableWarning())
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
		Warnings: warnings,
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
