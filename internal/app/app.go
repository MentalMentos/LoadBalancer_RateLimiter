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

func NewApp(configPath string) {
	InitLogger()
	ctx := context.Background()
	mylogger, _ := zap.NewDevelopment()
	sugar := mylogger.Sugar()

	go func() {
		sugar.Info(http.ListenAndServe("localhost:6060", nil))
		sugar.Info("Started sugarlog server on localhost:6060")
	}()

	config, err := config.LoadConfig(configPath)
	if err != nil {
		sugar.Fatalf("Error loading config: %v", err)
	}
	sugar.Infof("Configuration loaded from %s", configPath)
	if config.LoadBalancer.Address == "" {
		config.LoadBalancer.Address = ":8080"
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
	sugar.Info("Health checker initialized")

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
		sugar.Infof("Loaded route %s with %d backends", route.Path, len(route.Backends))
	}

	lbMap := loadBalancer.CreateLoadBalancers(routes, backend, hc, Logger)
	sugar.Infof("Creating load balancer map for routes: %v", routes)
	rateLimiter := rateLimiter2.NewTokenBucketLimiter(ctx, config.RateLimiter.Limit, time.Second*30, Logger)
	sugar.Info("Load balancers and rate limiter initialized")

	server := &http.Server{
		Addr:    config.LoadBalancer.Address,
		Handler: routes2.CreateRouter(lbMap, rateLimiter, Logger),
	}
	sugar.Infof("Server created with address %s", config.LoadBalancer.Address)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sugar.Errorf("Server failed: %v", err)
		}
	}()
	sugar.Info(">>>>Server started<<<<")
	rateLimiter.AddClient(&rateLimiter2.ClientConfig{
		Ip:       "127.0.0.1",
		Capacity: config.RateLimiter.Limit,
		Interval: time.Second * 30,
	})
	rateLimiter.StartPeriod(ctx)
	sugar.Info("Rate limiter client added and started")

	go hc.Start()
	sugar.Info("Health checker started")

	go handleShutdown(ctx, server, sugar)
}

func InitLogger() {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var err error
	Logger, err = config.Build()
	if err != nil {
		os.Exit(1)
	}
	defer Logger.Sync()
}

func handleShutdown(ctx context.Context, server *http.Server, sugar *zap.SugaredLogger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	sugar.Info("Received shutdown signal")

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		sugar.Errorf("Shutdown error: %v", err)
	} else {
		sugar.Info("Server stopped gracefully")
	}
}
