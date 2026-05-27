package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tokensieve/pkg/auth"
	"tokensieve/pkg/cache"
	"tokensieve/pkg/diff"
	"tokensieve/pkg/telemetry"
)

// Server wires together all subsystems.
type Server struct {
	proxy       *httputil.ReverseProxy
	windowCache *cache.SlidingWindowCache
	recorder    *telemetry.Recorder
	limiter     *auth.RateLimiter
	provider    string
}

func main() {
	provider := getEnv("LLM_PROVIDER", "anthropic")

	targetURL := resolveUpstream(provider)
	log.Printf("🛡️  TokenSieve v1.0.0 | provider=%s upstream=%s", provider, targetURL.String())

	windowCache := cache.NewSlidingWindowCache(256 * 1024) // 256k entry LRU
	recorder := telemetry.NewRecorder(provider)
	limiter := auth.NewRateLimiter(200, time.Minute)

	srv := &Server{
		windowCache: windowCache,
		recorder:    recorder,
		limiter:     limiter,
		provider:    provider,
	}

	srv.proxy = buildProxy(targetURL)

	mux := http.NewServeMux()

	// LLM API pass-through routes with semantic compression
	mux.HandleFunc("/v1/messages", srv.handleMessages)              // Anthropic
	mux.HandleFunc("/v1/chat/completions", srv.handleMessages)      // OpenAI-compat
	mux.HandleFunc("/v1beta/models/", srv.handleMessages)           // Gemini

	// Telemetry & admin endpoints
	mux.HandleFunc("/api/metrics", recorder.HTTPHandler())
	mux.HandleFunc("/api/health", srv.handleHealth)
	mux.HandleFunc("/api/cache/stats", srv.handleCacheStats)

	// Apply auth + rate-limiting middleware
	handler := auth.Middleware(limiter)(mux)

	httpSrv := &http.Server{
		Addr:         ":" + getEnv("PORT", "8080"),
		Handler:      handler,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("🚀 Proxy listening on %s", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	<-quit
	log.Println("⏳ Shutting down gracefully…")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Fatalf("Shutdown error: %v", err)
	}
	log.Println("✅ TokenSieve stopped.")
}

// ── Request Pipeline ─────────────────────────────────────────────────────────

// MessageRequest represents the canonical upstream payload structure.
type MessageRequest struct {
	Model    string         `json:"model"`
	Messages []diff.Message `json:"messages"`
	System   string         `json:"system,omitempty"`
	Stream   bool           `json:"stream"`
	MaxTokens int           `json:"max_tokens,omitempty"`
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.proxy.ServeHTTP(w, r)
		return
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 32*1024*1024)) // 32MB cap
	if err != nil {
		http.Error(w, `{"error":"request read failure"}`, http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	var msgReq MessageRequest
	if err := json.Unmarshal(bodyBytes, &msgReq); err != nil {
		// Non-parseable: pass through untouched to avoid breaking workflows
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		s.proxy.ServeHTTP(w, r)
		return
	}

	sessionID := deriveSession(r)

	// ── Core semantic compression pass ────────────────────────────────────
	result := diff.CompressContext(sessionID, msgReq.Messages, s.windowCache)
	msgReq.Messages = result.Messages

	// Inline system prompt deduplication
	if msgReq.System != "" {
		hash := s.windowCache.ComputeHash(msgReq.System)
		if _, found := s.windowCache.Get(hash); found {
			tokensSaved := int64(len(msgReq.System) / 4)
			result.TokensSaved += tokensSaved
			msgReq.System = fmt.Sprintf("[TS:SYS_CACHE ref=%s saved≈%dtok]", hash[:12], tokensSaved)
		} else {
			s.windowCache.Put(hash, msgReq.System, sessionID)
		}
	}
	// ─────────────────────────────────────────────────────────────────────

	newBodyBytes, err := json.Marshal(msgReq)
	if err != nil {
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		s.proxy.ServeHTTP(w, r)
		return
	}

	tokensPassed := int64(len(newBodyBytes) / 4)

	s.recorder.Record(
		sessionID,
		result.Strategy,
		result.TokensSaved,
		tokensPassed,
		result.BlocksMatched,
	)

	// Rehydrate the request with the compressed body
	r.Body = io.NopCloser(bytes.NewBuffer(newBodyBytes))
	r.ContentLength = int64(len(newBodyBytes))
	r.Header.Set("Content-Length", fmt.Sprintf("%d", len(newBodyBytes)))
	r.Header.Set("X-TokenSieve-Session", sessionID)
	r.Header.Set("X-TokenSieve-Saved", fmt.Sprintf("%d", result.TokensSaved))

	if msgReq.Stream {
		s.recorder.IncrementStreams()
		defer s.recorder.DecrementStreams()
	}

	s.proxy.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"version":   "1.0.0",
		"provider":  s.provider,
		"timestamp": time.Now().UTC(),
	})
}

func (s *Server) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	stats := s.windowCache.Stats()
	json.NewEncoder(w).Encode(stats)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func buildProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	original := proxy.Director

	proxy.Director = func(req *http.Request) {
		original(req)
		req.Header.Set("X-TokenSieve-Proxy", "v1.0.0")

		// Forward upstream API key securely
		if key := os.Getenv("UPSTREAM_API_KEY"); key != "" {
			req.Header.Set("x-api-key", key)
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy error: %v", err)
		http.Error(w, `{"error":"upstream_unavailable"}`, http.StatusBadGateway)
	}

	return proxy
}

func resolveUpstream(provider string) *url.URL {
	switch provider {
	case "gemini":
		u, _ := url.Parse("https://generativelanguage.googleapis.com")
		return u
	case "openai":
		u, _ := url.Parse("https://api.openai.com")
		return u
	default:
		u, _ := url.Parse("https://api.anthropic.com")
		return u
	}
}

// deriveSession builds a stable session ID from auth headers + IP,
// respecting any explicit X-Session-ID the client provides.
func deriveSession(r *http.Request) string {
	if id := r.Header.Get("X-Session-ID"); id != "" {
		return id
	}
	h := sha256.New()
	h.Write([]byte(r.Header.Get("Authorization")))
	h.Write([]byte(r.Header.Get("x-api-key")))
	h.Write([]byte(r.RemoteAddr))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
