package config

import (
	"fmt"
	"github.com/spf13/viper"
	"time"
)

// LoadConfig загружает конфигурацию из файла с использованием Viper
// configFile - имя конфигурационного файла (без расширения)
func LoadConfig(configFile string) (*Config, error) {
	v := viper.New()
	v.SetConfigName(configFile)
	v.AddConfigPath("./")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("config file not found: %w", err)
		} else {
			return nil, fmt.Errorf("unable to parse config.file: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Устанавливаем дефолтные значения для health checker'а,
	// если они не были указаны в конфиге
	if config.HealthChecker.HealthyServerFrequency == 0 {
		config.HealthChecker.HealthyServerFrequency = 5 * time.Second
	}
	if config.HealthChecker.UnhealthyServerFrequency == 0 {
		config.HealthChecker.UnhealthyServerFrequency = 10 * time.Second
	}
	return &config, nil
}

//func parseDuration(str string) (time.Duration, error) {
//	return time.ParseDuration(str)
//}
