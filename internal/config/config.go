package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type ServerConfig struct {
	Port    int    `mapstructure:"port"`
	DataDir string `mapstructure:"data_dir"`
}

type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

type LLMConfig struct {
	Provider    string  `mapstructure:"provider"`
	APIKey      string  `mapstructure:"api_key"`
	Model       string  `mapstructure:"model"`
	Temperature float64 `mapstructure:"temperature"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	LLM      LLMConfig      `mapstructure:"llm"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".nura"
	}
	return filepath.Join(home, ".nura")
}

func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	v.SetDefault("server.port", 8000)
	v.SetDefault("server.data_dir", defaultDataDir())
	v.SetDefault("database.path", "nura.db")
	v.SetDefault("llm.provider", "mock")
	v.SetDefault("llm.api_key", "")
	v.SetDefault("llm.model", "")
	v.SetDefault("llm.temperature", 0.1)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "console")

	v.SetEnvPrefix("NURA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
