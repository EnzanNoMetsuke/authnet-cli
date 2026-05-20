package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	configEnvName               = "AUTHNET_CONFIG_DIR"
	profileEnvName              = "AUTHNET_PROFILE"
	environmentEnvName          = "AUTHNET_ENVIRONMENT"
	apiLoginIDEnvName           = "AUTHNET_API_LOGIN_ID"
	transactionKeyEnvName       = "AUTHNET_TRANSACTION_KEY"
	profileConfigFileName       = "profiles.json"
	profileConfigVersion        = 1
	environmentSandbox          = "sandbox"
	environmentProduction       = "production"
	credentialSourceEnv         = "env"
	credentialSourceSecureRef   = "secure-local-reference" // #nosec G101 - credential source type label, not a secret.
	productionMarker            = "PRODUCTION"
	defaultCredentialLoginEnv   = apiLoginIDEnvName
	defaultCredentialTranKeyEnv = transactionKeyEnvName
)

type profileStore struct {
	dir  string
	path string
}

type profileFile struct {
	Version        int            `json:"version"`
	DefaultProfile string         `json:"default_profile,omitempty"`
	Profiles       []profileEntry `json:"profiles"`
}

type profileEntry struct {
	Name             string           `json:"name"`
	Environment      string           `json:"environment"`
	CredentialSource credentialSource `json:"credential_source"`
}

type credentialSource struct {
	Type              string `json:"type"`
	APILoginIDEnv     string `json:"api_login_id_env,omitempty"`
	TransactionKeyEnv string `json:"transaction_key_env,omitempty"`
	Reference         string `json:"reference,omitempty"`
}

type validationResult struct {
	ConfigPath string     `json:"config_path"`
	Valid      bool       `json:"valid"`
	Profiles   []string   `json:"profiles"`
	Warnings   []warning  `json:"-"`
	Checks     []checkRow `json:"checks"`
}

type checkRow struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func newProfileStore() (profileStore, error) {
	dir, err := authnetConfigDir()
	if err != nil {
		return profileStore{}, err
	}
	return profileStore{
		dir:  dir,
		path: filepath.Join(dir, profileConfigFileName),
	}, nil
}

func (store profileStore) load() (profileFile, error) {
	data, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return profileFile{Version: profileConfigVersion, Profiles: []profileEntry{}}, nil
	}
	if err != nil {
		return profileFile{}, fmt.Errorf("read profile config: %w", err)
	}

	var file profileFile
	if err := json.Unmarshal(data, &file); err != nil {
		return profileFile{}, newUsageError("profile config is not valid JSON: %v", err)
	}
	if file.Version == 0 {
		file.Version = profileConfigVersion
	}
	if file.Profiles == nil {
		file.Profiles = []profileEntry{}
	}
	return file, nil
}

func (store profileStore) save(file profileFile) error {
	if err := os.MkdirAll(store.dir, 0o700); err != nil {
		return fmt.Errorf("create profile config directory: %w", err)
	}
	file.Version = profileConfigVersion
	sortProfiles(file.Profiles)

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profile config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(store.path, data, 0o600); err != nil {
		return fmt.Errorf("write profile config: %w", err)
	}
	return nil
}

func upsertProfile(file profileFile, profile profileEntry, defaultProfile bool) (profileFile, error) {
	if err := validateProfile(profile); err != nil {
		return file, err
	}
	replaced := false
	for index, existing := range file.Profiles {
		if existing.Name == profile.Name {
			file.Profiles[index] = profile
			replaced = true
			break
		}
	}
	if !replaced {
		file.Profiles = append(file.Profiles, profile)
	}
	if defaultProfile {
		if profile.Environment != environmentSandbox {
			return file, newUsageError("default profile %q must be sandbox-classified", profile.Name)
		}
		file.DefaultProfile = profile.Name
	} else if file.DefaultProfile == profile.Name && profile.Environment != environmentSandbox {
		file.DefaultProfile = ""
	}
	sortProfiles(file.Profiles)
	return file, nil
}

func removeProfile(file profileFile, name string) (profileFile, bool) {
	next := file.Profiles[:0]
	removed := false
	for _, profile := range file.Profiles {
		if profile.Name == name {
			removed = true
			continue
		}
		next = append(next, profile)
	}
	file.Profiles = next
	if file.DefaultProfile == name {
		file.DefaultProfile = ""
	}
	return file, removed
}

func validateProfile(profile profileEntry) error {
	if strings.TrimSpace(profile.Name) == "" {
		return newUsageError("profile name is required")
	}
	if err := validateEnvironment(profile.Environment); err != nil {
		return err
	}
	switch profile.CredentialSource.Type {
	case credentialSourceEnv:
		if strings.TrimSpace(profile.CredentialSource.APILoginIDEnv) == "" {
			return newUsageError("API login ID environment variable name is required")
		}
		if strings.TrimSpace(profile.CredentialSource.TransactionKeyEnv) == "" {
			return newUsageError("transaction key environment variable name is required")
		}
	case credentialSourceSecureRef:
		if strings.TrimSpace(profile.CredentialSource.Reference) == "" {
			return newUsageError("secure local credential reference is required")
		}
	default:
		return newUsageError("invalid credential source type %q: expected env or secure-local-reference", profile.CredentialSource.Type)
	}
	return nil
}

func validateEnvironment(environment string) error {
	switch environment {
	case environmentSandbox, environmentProduction:
		return nil
	default:
		return newUsageError("invalid environment classification %q: expected sandbox or production", environment)
	}
}

func validateProfileFile(file profileFile) validationResult {
	result := validationResult{
		Valid:    true,
		Profiles: []string{},
		Checks:   []checkRow{},
	}
	names := map[string]bool{}
	defaultExists := file.DefaultProfile == ""

	for _, profile := range file.Profiles {
		result.Profiles = append(result.Profiles, profile.Name)
		if names[profile.Name] {
			result.Valid = false
			result.Checks = append(result.Checks, checkRow{"profile " + profile.Name, "failed", "profile name is duplicated"})
		}
		names[profile.Name] = true
		if profile.Name == file.DefaultProfile {
			defaultExists = true
			if profile.Environment != environmentSandbox {
				result.Valid = false
				result.Checks = append(result.Checks, checkRow{"default profile", "failed", "default profile must be sandbox-classified"})
			}
		}
		if err := validateProfile(profile); err != nil {
			result.Valid = false
			result.Checks = append(result.Checks, checkRow{"profile " + profile.Name, "failed", err.Error()})
			continue
		}
		result.Checks = append(result.Checks, checkRow{"profile " + profile.Name, "passed", "profile metadata is valid"})
		result = validateCredentialAvailability(result, profile)
	}
	if !defaultExists {
		result.Valid = false
		result.Checks = append(result.Checks, checkRow{"default profile", "failed", "default profile does not exist"})
	}
	sort.Strings(result.Profiles)
	return result
}

func validateCredentialAvailability(result validationResult, profile profileEntry) validationResult {
	if profile.CredentialSource.Type != credentialSourceEnv {
		result.Checks = append(result.Checks, checkRow{"credential source " + profile.Name, "passed", "secure local credential reference is configured"})
		return result
	}
	loginPresent := configuredEnvironmentValue(profile.CredentialSource.APILoginIDEnv) != ""
	keyPresent := configuredEnvironmentValue(profile.CredentialSource.TransactionKeyEnv) != ""
	if loginPresent && keyPresent {
		result.Checks = append(result.Checks, checkRow{"credential source " + profile.Name, "passed", "credential environment variables are available"})
		return result
	}
	result.Valid = false
	missing := []string{}
	if !loginPresent {
		missing = append(missing, profile.CredentialSource.APILoginIDEnv)
	}
	if !keyPresent {
		missing = append(missing, profile.CredentialSource.TransactionKeyEnv)
	}
	result.Checks = append(result.Checks, checkRow{"credential source " + profile.Name, "failed", "missing credential environment variables: " + strings.Join(missing, ", ")})
	return result
}

func resolveProfileEnvironment(options *globalOptions) error {
	if options.Environment != "" {
		return nil
	}
	if options.Profile == "" && !options.RawResponse {
		return nil
	}
	store, err := newProfileStore()
	if err != nil {
		return err
	}
	file, err := store.load()
	if err != nil {
		return err
	}
	selected := options.Profile
	if selected == "" {
		selected = file.DefaultProfile
	}
	if selected == "" {
		return nil
	}
	for _, profile := range file.Profiles {
		if profile.Name == selected {
			options.Profile = profile.Name
			options.Environment = profile.Environment
			if profile.Environment == environmentProduction && options.RawResponse {
				return newSafetyDeniedError("raw response mode is unavailable for production-classified profiles")
			}
			return nil
		}
	}
	if options.Profile != "" {
		return newUsageError("profile %q does not exist", options.Profile)
	}
	return nil
}

func promptForMissing(scanner *bufio.Scanner, writer io.Writer, label string, value *string) error {
	if strings.TrimSpace(*value) != "" {
		return nil
	}
	if _, err := fmt.Fprintf(writer, "%s: ", label); err != nil {
		return err
	}
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return err
		}
		return newUsageError("%s is required", label)
	}
	*value = strings.TrimSpace(scanner.Text())
	return nil
}

func sortProfiles(profiles []profileEntry) {
	sort.SliceStable(profiles, func(left, right int) bool {
		return profiles[left].Name < profiles[right].Name
	})
}

func authnetConfigDir() (string, error) {
	return configuredAuthnetConfigDir()
}
