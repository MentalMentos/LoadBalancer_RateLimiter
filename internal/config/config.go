package config

import (
	"fmt"
	"github.com/spf13/viper"
	"time"
)

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
	//var healthCheckerConfig HealthChecker
	//if err := v.Unmarshal(&healthCheckerConfig); err != nil {
	//	return nil, fmt.Errorf("unable to parse config.file: %w", err)
	//}

	// конверт строк в time.Duration
	//hf, err := parseDuration(healthCheckerConfig.HealthyServerFrequency)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to parse healthy server frequency: %w", err)
	//}
	//unhf, err := parseDuration(healthCheckerConfig.UnhealthyServerFrequency)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to parse unhealthy server frequency: %w", err)
	//}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	config.HealthChecker.HealthyServerFrequency = 5 * time.Second
	config.HealthChecker.UnhealthyServerFrequency = 10 * time.Second

	return &config, nil
}

func parseDuration(str string) (time.Duration, error) {
	return time.ParseDuration(str)
}
