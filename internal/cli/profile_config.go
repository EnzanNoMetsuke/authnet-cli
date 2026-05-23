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
	"time"

	"go.yaml.in/yaml/v3"
)

const (
	configEnvName                   = "AUTHNET_CONFIG_DIR"
	profileEnvName                  = "AUTHNET_PROFILE"
	environmentEnvName              = "AUTHNET_ENVIRONMENT"
	apiLoginIDEnvName               = "AUTHNET_API_LOGIN_ID"
	transactionKeyEnvName           = "AUTHNET_TRANSACTION_KEY"
	profileConfigFileName           = "config.yaml"
	legacyProfileConfigFileName     = "profiles.json"
	deprecatedProfileConfigFileName = "DEPRECATED-profiles.json"
	profileConfigVersion            = 1
	environmentSandbox              = "sandbox"
	environmentProduction           = "production"
	credentialSourceEnv             = "env"
	credentialSourceSecureRef       = "secure-local-reference" // #nosec G101 - credential source type label, not a secret.
	productionMarker                = "PRODUCTION"
	defaultCredentialLoginEnv       = apiLoginIDEnvName
	defaultCredentialTranKeyEnv     = transactionKeyEnvName
)

var unixTimestampNow = func() int64 {
	return time.Now().Unix()
}

type profileStore struct {
	dir        string
	path       string
	legacyPath string
	backupPath string
}

type loadedProfileFile struct {
	file profileFile
	path string
}

type profileFile struct {
	Version        int            `json:"version" yaml:"version"`
	DefaultProfile string         `json:"default_profile,omitempty" yaml:"default_profile,omitempty"`
	Profiles       []profileEntry `json:"profiles" yaml:"profiles"`
	Preferences    map[string]any `json:"preferences,omitempty" yaml:"preferences"`
}

type profileEntry struct {
	Name             string           `json:"name" yaml:"name"`
	Environment      string           `json:"environment" yaml:"environment"`
	CredentialSource credentialSource `json:"credential_source" yaml:"credential_source"`
}

type credentialSource struct {
	Type              string `json:"type" yaml:"type"`
	APILoginIDEnv     string `json:"api_login_id_env,omitempty" yaml:"api_login_id_env,omitempty"`
	TransactionKeyEnv string `json:"transaction_key_env,omitempty" yaml:"transaction_key_env,omitempty"`
	Reference         string `json:"reference,omitempty" yaml:"reference,omitempty"`
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
		dir:        dir,
		path:       filepath.Join(dir, profileConfigFileName),
		legacyPath: filepath.Join(dir, legacyProfileConfigFileName),
		backupPath: filepath.Join(dir, deprecatedProfileConfigFileName),
	}, nil
}

func (store profileStore) load() (profileFile, error) {
	loaded, err := store.loadWithSource()
	if err != nil {
		return profileFile{}, err
	}
	return loaded.file, nil
}

func (store profileStore) loadWithSource() (loadedProfileFile, error) {
	data, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return store.loadLegacyWithSource()
	}
	if err != nil {
		return loadedProfileFile{}, fmt.Errorf("read profile config: %w", err)
	}

	var file profileFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return loadedProfileFile{}, newUsageError("profile config is not valid YAML: %v", err)
	}
	return loadedProfileFile{file: normalizeProfileFile(file), path: store.path}, nil
}

func (store profileStore) loadLegacyWithSource() (loadedProfileFile, error) {
	data, err := os.ReadFile(store.legacyPath)
	if errors.Is(err, os.ErrNotExist) {
		return loadedProfileFile{file: newProfileFile(), path: store.path}, nil
	}
	if err != nil {
		return loadedProfileFile{}, fmt.Errorf("read legacy profile config: %w", err)
	}

	var file profileFile
	if err := json.Unmarshal(data, &file); err != nil {
		return loadedProfileFile{}, newUsageError("legacy profile config is not valid JSON: %v", err)
	}
	return loadedProfileFile{file: normalizeProfileFile(file), path: store.legacyPath}, nil
}

func newProfileFile() profileFile {
	return profileFile{
		Version:     profileConfigVersion,
		Profiles:    []profileEntry{},
		Preferences: map[string]any{},
	}
}

func normalizeProfileFile(file profileFile) profileFile {
	if file.Version == 0 {
		file.Version = profileConfigVersion
	}
	if file.Profiles == nil {
		file.Profiles = []profileEntry{}
	}
	if file.Preferences == nil {
		file.Preferences = map[string]any{}
	}
	return file
}

func loadPreferencesFile(path string) (profileFile, error) {
	data, err := os.ReadFile(path) // #nosec G304 - path is the resolved CLI-controlled config file path.
	if errors.Is(err, os.ErrNotExist) {
		return newProfileFile(), nil
	}
	if err != nil {
		return profileFile{}, fmt.Errorf("read profile config preferences: %w", err)
	}

	var file profileFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return profileFile{}, newUsageError("profile config is not valid YAML: %v", err)
	}
	return normalizeProfileFile(file), nil
}

func stringPreference(preferences map[string]any, key string) (string, bool) {
	value, ok := preferences[key]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return strings.TrimSpace(fmt.Sprint(value)), true
	}
	return strings.TrimSpace(text), true
}

func nestedStringPreference(preferences map[string]any, keys ...string) (string, bool) {
	if len(keys) == 0 {
		return "", false
	}
	current := preferences
	for _, key := range keys[:len(keys)-1] {
		value, ok := current[key]
		if !ok {
			return "", false
		}
		next, ok := value.(map[string]any)
		if !ok {
			return "", false
		}
		current = next
	}
	return stringPreference(current, keys[len(keys)-1])
}

func (store profileStore) save(file profileFile) error {
	if err := os.MkdirAll(store.dir, 0o700); err != nil {
		return fmt.Errorf("create profile config directory: %w", err)
	}
	file.Version = profileConfigVersion
	if file.Preferences == nil {
		file.Preferences = map[string]any{}
	}
	sortProfiles(file.Profiles)

	data, err := yaml.Marshal(file)
	if err != nil {
		return fmt.Errorf("encode profile config: %w", err)
	}
	if err := os.WriteFile(store.path, data, 0o600); err != nil {
		return fmt.Errorf("write profile config: %w", err)
	}
	return nil
}

func (store profileStore) renameLegacyBackup() (string, warning) {
	if _, err := os.Stat(store.backupPath); err == nil {
		return store.renameLegacyBackupWithTimestamp()
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", warning{
			Code:    "config_migration_backup_rename_failed",
			Message: fmt.Sprintf("Could not inspect DEPRECATED-profiles.json before renaming profiles.json: %v", err),
		}
	}
	if err := os.Rename(store.legacyPath, store.backupPath); err != nil {
		return "", warning{
			Code:    "config_migration_backup_rename_failed",
			Message: fmt.Sprintf("Could not rename profiles.json to DEPRECATED-profiles.json: %v", err),
		}
	}
	return store.backupPath, warning{}
}

func (store profileStore) renameLegacyBackupWithTimestamp() (string, warning) {
	var lastErr error
	for range 3 {
		backupPath := filepath.Join(store.dir, fmt.Sprintf("DEPRECATED-%d-profiles.json", unixTimestampNow()))
		if _, err := os.Stat(backupPath); err == nil {
			lastErr = fmt.Errorf("%s already exists", filepath.Base(backupPath))
			continue
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			lastErr = err
			continue
		}
		if err := os.Rename(store.legacyPath, backupPath); err != nil {
			lastErr = err
			continue
		}
		return backupPath, warning{}
	}
	return "", warning{
		Code:    "config_migration_backup_rename_failed",
		Message: fmt.Sprintf("Could not rename profiles.json to a timestamped deprecated backup after 3 attempts: %v; retained profiles.json for manual cleanup.", lastErr),
	}
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
	result = validatePreferences(result, file.Preferences)
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

func validatePreferences(result validationResult, preferences map[string]any) validationResult {
	color, ok := stringPreference(preferences, preferenceKeyColor)
	if ok {
		switch color {
		case "auto", "always", "never":
			result.Checks = append(result.Checks, checkRow{"preference color", "passed", "color preference is valid"})
		default:
			result.Valid = false
			result.Checks = append(result.Checks, checkRow{"preference color", "failed", fmt.Sprintf("invalid preference color %q: expected auto, always, or never", color)})
		}
	}
	sortBy, ok := nestedStringPreference(preferences, "transaction_list", "sort_by")
	if ok {
		switch sortBy {
		case "timestamp", "transaction_id", "amount":
			result.Checks = append(result.Checks, checkRow{"preference transaction list sort field", "passed", "transaction list sort field preference is valid"})
		default:
			result.Valid = false
			result.Checks = append(result.Checks, checkRow{"preference transaction list sort field", "failed", fmt.Sprintf("invalid preference transaction_list.sort_by %q: expected timestamp, transaction_id, or amount", sortBy)})
		}
	}
	sortOrder, ok := nestedStringPreference(preferences, "transaction_list", "sort_order")
	if ok {
		switch sortOrder {
		case "ascending", "descending":
			result.Checks = append(result.Checks, checkRow{"preference transaction list sort order", "passed", "transaction list sort order preference is valid"})
		default:
			result.Valid = false
			result.Checks = append(result.Checks, checkRow{"preference transaction list sort order", "failed", fmt.Sprintf("invalid preference transaction_list.sort_order %q: expected ascending or descending", sortOrder)})
		}
	}
	filterStatus, ok := nestedStringPreference(preferences, "transaction_list", "filter", "status")
	if ok {
		if validTransactionFilterStatus(filterStatus) {
			result.Checks = append(result.Checks, checkRow{"preference transaction list filter status", "passed", "transaction list filter status preference is valid"})
		} else {
			result.Valid = false
			result.Checks = append(result.Checks, checkRow{"preference transaction list filter status", "failed", fmt.Sprintf("invalid preference transaction_list.filter.status %q: expected Authorize.Net transactionStatusEnum value", filterStatus)})
		}
	}
	filterAmount, ok := nestedStringPreference(preferences, "transaction_list", "filter", "amount")
	if ok {
		if _, err := normalizeTransactionFilterAmount(filterAmount); err == nil {
			result.Checks = append(result.Checks, checkRow{"preference transaction list filter amount", "passed", "transaction list filter amount preference is valid"})
		} else {
			result.Valid = false
			result.Checks = append(result.Checks, checkRow{"preference transaction list filter amount", "failed", fmt.Sprintf("invalid preference transaction_list.filter.amount %q: expected positive integer or decimal amount with up to two decimal places", filterAmount)})
		}
	}
	filterPayment, ok := nestedStringPreference(preferences, "transaction_list", "filter", "payment")
	if ok {
		if validTransactionFilterPayment(filterPayment) {
			result.Checks = append(result.Checks, checkRow{"preference transaction list filter payment", "passed", "transaction list filter payment preference is valid"})
		} else {
			result.Valid = false
			result.Checks = append(result.Checks, checkRow{"preference transaction list filter payment", "failed", fmt.Sprintf("invalid preference transaction_list.filter.payment %q: expected <CardType> XXXXdddd for Visa, MasterCard, AmericanExpress, Discover, DinersClub, or JCB", filterPayment)})
		}
	}
	return result
}

func validateCredentialAvailability(result validationResult, profile profileEntry) validationResult {
	if profile.CredentialSource.Type != credentialSourceEnv {
		result.Checks = append(result.Checks, checkRow{"credential source " + profile.Name, "passed", "secure local credential reference is configured"})
		return result
	}
	loginPresent := strings.TrimSpace(configuredEnvironmentValue(profile.CredentialSource.APILoginIDEnv)) != ""
	keyPresent := strings.TrimSpace(configuredEnvironmentValue(profile.CredentialSource.TransactionKeyEnv)) != ""
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
				return newSafetyDeniedError(rawResponseProductionSafetyMessage)
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
