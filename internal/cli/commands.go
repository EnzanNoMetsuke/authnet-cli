package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newVersionCommand(build BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show local version and contract metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "authnet %s\nschema version: %s\ncontract status: %s\ncommit: %s\nbuilt: %s\n",
				build.Version,
				build.SchemaVersion,
				build.ContractStatus,
				build.Commit,
				build.Date,
			)
			return err
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
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "config directory: %s\nsensitive-data persistence: none\n", configDir)
			return err
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
		RunE:  notImplemented("config validate"),
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
		RunE:  notImplemented("auth test"),
	})
	return auth
}

func newProfileCommand() *cobra.Command {
	profile := &cobra.Command{
		Use:   "profile",
		Short: "Manage local profiles",
	}
	profile.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List saved non-secret profile metadata",
		RunE:  notImplemented("profile list"),
	})
	profile.AddCommand(&cobra.Command{
		Use:   "setup",
		Short: "Create or update profile metadata",
		RunE:  notImplemented("profile setup"),
	})
	profile.AddCommand(&cobra.Command{
		Use:   "remove",
		Short: "Remove local profile metadata",
		RunE:  notImplemented("profile remove"),
	})
	return profile
}

func newTransactionCommand() *cobra.Command {
	transaction := &cobra.Command{
		Use:   "transaction",
		Short: "Inspect Authorize.Net transactions",
	}
	transaction.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Inspect one transaction",
		RunE:  notImplemented("transaction get"),
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

func authnetConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "authnet-cli"), nil
}
