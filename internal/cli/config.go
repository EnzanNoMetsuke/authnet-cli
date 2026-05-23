package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	configKeyJSON        = "json"
	configKeyAutomation  = "automation"
	configKeyProfile     = "profile"
	configKeyEnvironment = "environment"
	configKeyRawResponse = "raw-response"
	configKeyColor       = "color"
	configKeyNoColor     = "no-color"
	configKeyConfigDir   = "config-dir"
	preferenceKeyColor   = "color"
	preferenceModeAlways = "always"
	preferenceModeNever  = "never"
)

type cliConfig struct {
	viper                     *viper.Viper
	configPath                string
	colorPreferenceSet        bool
	colorPreferencePath       string
	jsonPreferenceSet         bool
	jsonPreferenceValue       string
	automationPreferenceSet   bool
	automationPreferenceValue string
}

func newCLIConfig(flags *pflag.FlagSet) (*cliConfig, error) {
	config := &cliConfig{viper: viper.New()}
	config.viper.SetEnvPrefix("AUTHNET")
	config.viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	config.viper.AutomaticEnv()

	config.viper.SetDefault(configKeyJSON, false)
	config.viper.SetDefault(configKeyAutomation, false)
	config.viper.SetDefault(configKeyRawResponse, false)
	config.viper.SetDefault(configKeyColor, "auto")
	config.viper.SetDefault(configKeyNoColor, false)

	envBindings := map[string]string{
		configKeyConfigDir:   configEnvName,
		configKeyProfile:     profileEnvName,
		configKeyEnvironment: environmentEnvName,
	}
	for key, envName := range envBindings {
		if err := config.viper.BindEnv(key, envName); err != nil {
			return nil, fmt.Errorf("bind %s environment config: %w", key, err)
		}
	}

	if flags != nil {
		for _, key := range []string{
			configKeyJSON,
			configKeyAutomation,
			configKeyProfile,
			configKeyRawResponse,
			configKeyColor,
			configKeyNoColor,
		} {
			if err := config.viper.BindPFlag(key, flags.Lookup(key)); err != nil {
				return nil, fmt.Errorf("bind --%s config flag: %w", key, err)
			}
		}
	}
	return config, nil
}

func (config *cliConfig) applyPreferences() error {
	dir, err := configDirFromViper(config.viper)
	if err != nil {
		return err
	}
	file, err := loadPreferencesFile(filepath.Join(dir, profileConfigFileName))
	if err != nil {
		return err
	}
	if color, ok := stringPreference(file.Preferences, preferenceKeyColor); ok {
		config.viper.SetDefault(configKeyColor, color)
		config.colorPreferenceSet = true
		config.colorPreferencePath = filepath.Join(dir, profileConfigFileName)
	}
	config.configPath = filepath.Join(dir, profileConfigFileName)
	if value, ok := stringPreference(file.Preferences, configKeyJSON); ok {
		config.jsonPreferenceSet = true
		config.jsonPreferenceValue = value
		if value == preferenceModeAlways {
			config.viper.SetDefault(configKeyJSON, true)
		}
		if value == preferenceModeNever {
			config.viper.SetDefault(configKeyJSON, false)
		}
	}
	if value, ok := stringPreference(file.Preferences, configKeyAutomation); ok {
		config.automationPreferenceSet = true
		config.automationPreferenceValue = value
		if value == preferenceModeAlways {
			config.viper.SetDefault(configKeyAutomation, true)
		}
		if value == preferenceModeNever {
			config.viper.SetDefault(configKeyAutomation, false)
		}
	}
	if config.jsonPreferenceValue == preferenceModeAlways && config.automationPreferenceValue == preferenceModeAlways {
		config.viper.SetDefault(configKeyAutomation, true)
	}
	return nil
}

func (config *cliConfig) applyGlobalOptions(options *globalOptions) {
	options.JSON = config.viper.GetBool(configKeyJSON)
	options.Automation = config.viper.GetBool(configKeyAutomation)
	options.Profile = strings.TrimSpace(config.viper.GetString(configKeyProfile))
	options.Environment = strings.TrimSpace(config.viper.GetString(configKeyEnvironment))
	options.RawResponse = config.viper.GetBool(configKeyRawResponse)
	options.Color = strings.TrimSpace(config.viper.GetString(configKeyColor))
	options.NoColor = config.viper.GetBool(configKeyNoColor)
	options.PreferenceWarnings = config.outputModePreferenceWarnings()
}

func (config *cliConfig) outputModePreferenceWarnings() []warning {
	if config.jsonPreferenceValue != preferenceModeAlways || config.automationPreferenceValue != preferenceModeAlways {
		return nil
	}
	return []warning{{
		Code:    "output_mode_preference_conflict",
		Message: "preferences.json and preferences.automation are both always in config.yaml; they are mutually exclusive at the preference layer, automation takes precedence at that layer, and one preference should be removed.",
	}}
}

func (config *cliConfig) rawResponsePreferenceWarnings() []warning {
	ignored := []string{}
	if config.jsonPreferenceValue == preferenceModeNever {
		ignored = append(ignored, "preferences.json")
	}
	if config.automationPreferenceValue == preferenceModeNever {
		ignored = append(ignored, "preferences.automation")
	}
	if len(ignored) == 0 {
		return nil
	}
	verb := "is"
	if len(ignored) > 1 {
		verb = "are"
	}
	return []warning{{
		Code:    "raw_response_preference_ignored",
		Message: strings.Join(ignored, " and ") + " " + verb + " set to never in config.yaml but ignored so the raw gateway JSON can be presented accurately.",
	}}
}

func configuredAuthnetConfigDir() (string, error) {
	config, err := newCLIConfig(nil)
	if err != nil {
		return "", err
	}
	return configDirFromViper(config.viper)
}

func configDirFromViper(config *viper.Viper) (string, error) {
	if override := strings.TrimSpace(config.GetString(configKeyConfigDir)); override != "" {
		return override, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "authnet-cli"), nil
}

func configuredEnvironmentValue(envName string) string {
	config := viper.New()
	if err := config.BindEnv("value", envName); err != nil {
		return ""
	}
	return config.GetString("value")
}
