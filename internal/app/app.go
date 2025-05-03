package app

import (
	"context"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"lb/internal/config"
	routes2 "lb/internal/modules"
	"lb/internal/modules/backends"
	"lb/internal/modules/backends/models"
	"lb/internal/modules/healthchecker"
	"lb/internal/modules/loadBalancer"
	rateLimiter2 "lb/internal/modules/rateLimiter"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var Logger *zap.Logger

func NewApp(configPath string) *http.Server {
	InitLogger()
	ctx := context.Background()
	mylogger, _ := zap.NewDevelopment()
	sugar := mylogger.Sugar()

	go func() {
		sugar.Info(http.ListenAndServe("localhost:6060", nil))
	}()

	config, err := config.LoadConfig(configPath)
	if err != nil {
		sugar.Fatalf("Error loading config: %v", err)
	}

	transport := &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   false,
	}
	pooledClient := &http.Client{
		Transport: transport,
		Timeout:   3 * time.Second,
	}

	backend := backends.NewBackendRegistry()
	hc := healthchecker.NewHealthChecker(config.HealthChecker.HealthyServerFrequency, config.HealthChecker.UnhealthyServerFrequency, backend, pooledClient, mylogger)

	routes := make([]loadBalancer.RouteConfig, len(config.Routes))
	for i, route := range config.Routes {
		routes[i] = loadBalancer.RouteConfig{
			Path:     route.Path,
			Backends: make([]models.Backend, len(route.Backends)),
		}
		for j, b := range route.Backends {
			routes[i].Backends[j] = models.Backend{
				URL:    b.URL,
				Health: b.Health,
			}
		}
	}

	lbMap := loadBalancer.CreateLoadBalancers(routes, backend, hc, Logger)
	rateLimiter := rateLimiter2.NewTokenBucketLimiter(ctx, config.RateLimiter.Limit, time.Second*30, Logger)

	rateLimiter.AddClient(&rateLimiter2.ClientConfig{
		Ip:       "127.0.0.1",
		Capacity: config.RateLimiter.Limit,
		Interval: time.Second * 30,
	})

	rateLimiter.StartPeriod(ctx)

	go hc.Start()

	server := &http.Server{
		Addr:    config.LoadBalancer.Address,
		Handler: routes2.CreateRouter(lbMap, rateLimiter),
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			sugar.Errorf("Server shutdown error: %v", err)
		}
	}()

	return server
}

func InitLogger() {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var err error
	Logger, err = config.Build()
	if err != nil {
		// Fallback to standard logger in case zap initialization fails
		os.Exit(1)
	}
	defer Logger.Sync()
}
