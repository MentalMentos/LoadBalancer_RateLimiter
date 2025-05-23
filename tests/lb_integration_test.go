package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"lb/internal/app"
	"lb/internal/config"
	routes "lb/internal/modules"
	"lb/internal/modules/backends"
	"lb/internal/modules/backends/models"
	"lb/internal/modules/healthchecker"
	"lb/internal/modules/loadBalancer"
	"lb/internal/modules/rateLimiter"
	rateLimiter2 "lb/internal/modules/rateLimiter"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoadBalancerIntegration(t *testing.T) {
	// 1. Инициализация тестовых бэкендов
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("backend1"))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("backend2"))
	}))
	defer backend2.Close()

	// 2. Создаем тестовую конфигурацию
	testConfig := &config.Config{
		Routes: []config.Route{
			{
				Path: "/api",
				Backends: []config.Backend{
					{URL: backend1.URL, Health: "/health"},
					{URL: backend2.URL, Health: "/health"},
				},
			},
		},
		RateLimiter: config.RateLimiter{
			Limit: 100,
		},
		LoadBalancer: config.LoadBalancer{
			Address: ":8080",
		},
		HealthChecker: config.HealthCheckerTime{
			HealthyServerFrequency:   5 * time.Second,
			UnhealthyServerFrequency: 1 * time.Second,
		},
	}

	// 3. Инициализация компонентов
	logger, _ := zap.NewDevelopment()
	ctx := context.Background()

	// Инициализируем registry и health checker
	registry := backends.NewBackendRegistry()
	hc := healthchecker.NewHealthChecker(
		testConfig.HealthChecker.HealthyServerFrequency,
		testConfig.HealthChecker.UnhealthyServerFrequency,
		registry,
		http.DefaultClient,
		logger,
	)

	// 4. Создаем маршруты для балансировщика
	routers := make([]loadBalancer.RouteConfig, len(testConfig.Routes))
	for i, route := range testConfig.Routes {
		routers[i] = loadBalancer.RouteConfig{
			Path:     route.Path,
			Backends: make([]models.Backend, len(route.Backends)),
		}
		for j, b := range route.Backends {
			routers[i].Backends[j] = models.Backend{
				URL:    b.URL,
				Health: b.Health,
			}
			// Регистрируем бэкенды
			registry.AddBackendToRegistry(models.Backend{
				Id:     uint64(j + 1),
				URL:    b.URL,
				Health: b.Health,
			})
			hc.AddBackend(&models.Backend{
				Id:     uint64(j + 1),
				URL:    b.URL,
				Health: b.Health,
			})
		}
	}

	// 5. Создаем балансировщики и rate limiter
	lbMap := loadBalancer.CreateLoadBalancers(routers, registry, hc, logger)
	rateLimiter := rateLimiter.NewTokenBucketLimiter(ctx, testConfig.RateLimiter.Limit, 30*time.Second, logger)

	// 6. Запускаем health checker
	go hc.Start()

	// 7. Создаем тестовый сервер
	routes := routes.CreateRouter(lbMap, rateLimiter, logger)
	testServer := httptest.NewServer(routes)
	defer testServer.Close()

	// 8. Тестируем балансировку нагрузки
	t.Run("Test request distribution", func(t *testing.T) {
		responses := make(map[string]int)
		for i := 0; i < 100; i++ {
			resp, err := http.Get(testServer.URL + "/api")
			require.NoError(t, err)

			var body bytes.Buffer
			_, err = body.ReadFrom(resp.Body)
			require.NoError(t, err)
			resp.Body.Close()

			responses[body.String()]++
		}

		assert.Greater(t, responses["backend1"], 30, "backend1 received too few requests")
		assert.Greater(t, responses["backend2"], 30, "backend2 received too few requests")
	})

	// 9. Тестируем rate limiting
	t.Run("Test rate limiting", func(t *testing.T) {
		// Добавляем тестового клиента
		rateLimiter.AddClient(&rateLimiter2.ClientConfig{
			Ip:       "127.0.0.1",
			Capacity: 100,
			Interval: time.Second * 30,
		})

		successCount := 0
		for i := 0; i < 15; i++ {
			resp, err := http.Get(testServer.URL + "/api")
			if err == nil && resp.StatusCode == http.StatusOK {
				successCount++
				resp.Body.Close()
			}
		}

		assert.InDelta(t, 10, successCount, 2, "rate limiting not working properly")
	})

	// 10. Тестируем health checking и failover
	t.Run("Test health checking and failover", func(t *testing.T) {
		// Останавливаем первый бэкенд
		backend1.Close()

		// Даем время health checker'у обнаружить проблему
		time.Sleep(2 * time.Second)

		// Делаем несколько запросов - все должны идти на backend2
		for i := 0; i < 10; i++ {
			resp, err := http.Get(testServer.URL + "/api")
			require.NoError(t, err)

			var body bytes.Buffer
			_, err = body.ReadFrom(resp.Body)
			require.NoError(t, err)
			resp.Body.Close()

			assert.Equal(t, "backend2", body.String(), "should only use backend2 after failover")
		}
	})

	// 11. Тестируем управление клиентами rate limiter'а
	t.Run("Test rate limiter client management", func(t *testing.T) {
		// Проверяем список клиентов
		resp, err := http.Get(testServer.URL + "/clients")
		require.NoError(t, err)
		defer resp.Body.Close()

		var clients []*rateLimiter2.ClientConfig
		err = json.NewDecoder(resp.Body).Decode(&clients)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(clients), 1, "should have at least one client")

		// Добавляем нового клиента
		newClient := rateLimiter2.ClientConfig{
			Ip:       "192.168.1.1",
			Capacity: 20,
			Interval: 10 * time.Second,
		}
		body, _ := json.Marshal(newClient)
		resp, err = http.Post(testServer.URL+"/clients", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Проверяем что клиент добавился
		resp, err = http.Get(testServer.URL + "/clients")
		require.NoError(t, err)
		defer resp.Body.Close()

		err = json.NewDecoder(resp.Body).Decode(&clients)
		require.NoError(t, err)
		assert.Equal(t, 2, len(clients), "should have two clients now")
	})
}

func TestAppInitialization(t *testing.T) {
	// Тестируем инициализацию всего приложения
	t.Run("Test full app initialization", func(t *testing.T) {
		// Создаем временный конфиг файл
		//		configContent := `
		//routes:
		//  - path: "/test"
		//    backends:
		//      - url: "http://localhost:8081"
		//        health: "/health"
		//      - url: "http://localhost:8082"
		//        health: "/health"
		//
		//rateLimiter:
		//  limit: 100
		//
		//loadbalancer:
		//  address: ":8080"
		//
		//healthchecker:
		//  healthyserver_freq: 5s
		//  unhealthyserver_freq: 1s
		//`

		// Запускаем приложение
		go func() {
			app.NewApp("test_config")
		}()

		// Даем время на инициализацию
		time.Sleep(1 * time.Second)

		// Проверяем что сервер запустился
		resp, err := http.Get("http://localhost:8080/test")
		if err == nil {
			defer resp.Body.Close()
			assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode,
				"should return 503 when no backends available")
		}
	})
}
