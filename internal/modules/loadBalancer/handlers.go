package loadBalancer

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"io"
	"lb/internal/modules/backends"
	"lb/internal/modules/backends/models"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type LoadBalancerHandler struct {
	lb         Loadbalancer
	client     *http.Client
	bufferPool *sync.Pool
	mu         sync.RWMutex
	logger     *zap.Logger
}

func NewLBHandler(registry *backends.BackendRegistry, healthChannels []<-chan models.BackendStatus, logger *zap.Logger) *LoadBalancerHandler {
	return &LoadBalancerHandler{
		lb:     *NewLoadBalancer(registry, healthChannels, logger),
		logger: logger,
		client: &http.Client{
			Transport: &http2.Transport{
				AllowHTTP: true,
				DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
					return net.Dial(network, addr)
				},
			},
			Timeout: 10 * time.Second,
		},
		bufferPool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 32<<10) // буффер 32KB
			},
		},
	}
}

func (h *LoadBalancerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	startTime := time.Now()

	backends := h.lb.getHealthyBackends()
	if len(backends) == 0 {
		h.handleError(w, r, errors.New("no healthy backends available"), http.StatusServiceUnavailable, startTime)
		return
	}

	backend, err := h.lb.Algorithm.GetNextBackend(backends)
	if err != nil {
		h.handleError(w, r, err, http.StatusServiceUnavailable, startTime)
		return
	}

	h.proxyRequest(ctx, w, r, backend, startTime)
}

func (h *LoadBalancerHandler) proxyRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, backend *models.Backend, startTime time.Time) {
	targetURL := buildTargetURL(backend.URL, r.URL.Path, r.URL.RawQuery)

	req, body, err := cloneRequest(r, targetURL)
	if err != nil {
		h.handleError(w, r, err, http.StatusInternalServerError, startTime)
		return
	}

	resp, err := h.executeWithRetries(ctx, req, body, 3)
	if err != nil {
		h.handleError(w, r, err, http.StatusBadGateway, startTime)
		return
	}
	defer resp.Body.Close()
	
	h.copyResponse(w, resp)

	h.logger.Debug("Request proxied successfully",
		zap.String("backend", backend.URL),
		zap.Int("status", resp.StatusCode),
		zap.Duration("duration", time.Since(startTime)),
	)
}

func (h *LoadBalancerHandler) executeWithRetries(ctx context.Context, req *http.Request, body []byte, maxRetries int) (*http.Response, error) {
	var resp *http.Response
	var err error

	for i := 0; i < maxRetries; i++ {
		req.Body = io.NopCloser(bytes.NewReader(body))

		resp, err = h.client.Do(req.WithContext(ctx))
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return resp, nil
			}
			// 429 (Too Many Requests)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
				break
			}
		}

		if i < maxRetries-1 {
			backoff := time.Duration(i)*time.Second + time.Duration(rand.Intn(100))*time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
	}

	if err != nil {
		h.logger.Error("Request to backend failed",
			zap.String("url", req.URL.String()),
			zap.Error(err),
		)
	} else if resp != nil {
		h.logger.Error("Backend returned error status",
			zap.String("url", req.URL.String()),
			zap.Int("status", resp.StatusCode),
		)
	}

	return resp, err
}

func (h *LoadBalancerHandler) copyResponse(w http.ResponseWriter, resp *http.Response) {
	for k, v := range resp.Header {
		w.Header()[k] = v
	}

	w.WriteHeader(resp.StatusCode)

	// копируем body
	buf := h.bufferPool.Get().([]byte)
	defer h.bufferPool.Put(buf)
	io.CopyBuffer(w, resp.Body, buf)
}

func (h *LoadBalancerHandler) handleError(w http.ResponseWriter, r *http.Request, err error, statusCode int, startTime time.Time) {
	h.logger.Error("Request processing failed",
		zap.String("path", r.URL.Path),
		zap.Error(err),
		zap.Duration("duration", time.Since(startTime)),
	)
	http.Error(w, err.Error(), statusCode)
}

//---------helpers----------------

func buildTargetURL(baseURL, path, query string) string {
	var sb strings.Builder
	sb.WriteString(baseURL)
	sb.WriteString(path)
	if query != "" {
		sb.WriteString("?")
		sb.WriteString(query)
	}
	return sb.String()
}

func cloneRequest(r *http.Request, targetURL string) (*http.Request, []byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, nil, err
	}
	r.Body.Close()

	req, err := http.NewRequest(r.Method, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}

	req.Header = r.Header.Clone()
	return req, body, nil
}
