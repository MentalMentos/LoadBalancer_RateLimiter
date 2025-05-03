package config

import "time"

type Route struct {
	Path     string
	Backends []Backend `mapstructure:"backends"`
}

type Backend struct {
	URL    string `mapstructure:"url"`
	Health string `mapstructure:"health"`
}

type RateLimiter struct {
	Type   string `mapstructure:"type"`
	Limit  int    `mapstructure:"limit"`
	Bucket string `mapstructure:"tokenbucket"` //
}

type LoadBalancer struct {
	Address string `mapstructure:"address" yaml:"address"`
}

type HealthChecker struct {
	HealthyServerFrequency   string `mapstructure:"healthyserver_freq" yaml:"healthyserver_frequency"`
	UnhealthyServerFrequency string `mapstructure:"unhealthyserver_freq" yaml:"unhealthyserver_frequency"`
}

type HealthCheckerTime struct {
	HealthyServerFrequency   time.Duration `mapstructure:"healthyserver_freq" yaml:"healthyserver_freq"`
	UnhealthyServerFrequency time.Duration `mapstructure:"unhealthyserver_freq" yaml:"unhealthyserver_freq"`
}

type Config struct {
	Routes        []Route           `mapstructure:"routes" yaml:"Routes"`
	RateLimiter   RateLimiter       `mapstructure:"rateLimiter" yaml:"RateLimiter"`
	LoadBalancer  LoadBalancer      `mapstructure:"loadbalancer" yaml:"LoadBalancer"`
	HealthChecker HealthCheckerTime `mapstructure:"healthchecker" yaml:"healthchecker"`
}
