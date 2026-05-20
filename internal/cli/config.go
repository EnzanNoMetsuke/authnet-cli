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
)

type cliConfig struct {
	viper               *viper.Viper
	colorPreferenceSet  bool
	colorPreferencePath string
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
