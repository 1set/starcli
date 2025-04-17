package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// InitConfig initializes the configuration with the given path.
func InitConfig(configPath string) error {
	if configPath != "" {
		// use the provided config file
		viper.SetConfigFile(configPath)
	} else {
		// search config in current directory
		viper.AddConfigPath(".")

		// try to find home and config directories
		if home, err := os.UserHomeDir(); err == nil {
			configDir := filepath.Join(home, ".config", AppName)
			viper.AddConfigPath(configDir)
		}

		// search config in current and config directory with name app name (without extension)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	// set default values
	viper.SetTypeByDefaultValue(true)
	SetDefaults()

	// read in environment variables that match
	viper.AutomaticEnv()
	replacer := strings.NewReplacer("-", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.SetEnvPrefix(`star`)

	// read the configuration file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			return err
		}
	}

	// done
	return nil
}
