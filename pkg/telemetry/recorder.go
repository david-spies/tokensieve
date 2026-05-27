package telemetry

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ProviderCosts holds per-provider cost models (per million tokens, USD).
var ProviderCosts = map[string]ProviderPricing{
	"anthropic": {InputCostPer1M: 3.00, OutputCostPer1M: 15.00, CachedReadPer1M: 0.30},
	"gemini":    {InputCostPer1M: 1.25, OutputCostPer1M: 5.00, CachedReadPer1M: 0.31},
	"openai":    {InputCostPer1M: 2.50, OutputCostPer1M: 10.00, CachedReadPer1M: 0.50},
}

// ProviderPricing defines the cost structure for an LLM provider.
type ProviderPricing struct {
	InputCostPer1M  float64
	OutputCostPer1M float64
	CachedReadPer1M float64
}

// SessionStats holds per-session telemetry.
type SessionStats struct {
	SessionID       string    `json:"session_id"`
	TokensSaved     int64     `json:"tokens_saved"`
	TokensPassed    int64     `json:"tokens_passed"`
	Requests        int64     `json:"requests"`
	CacheHits       int64     `json:"cache_hits"`
	EstSavingsUSD   float64   `json:"est_savings_usd"`
	LastSeen        time.Time `json:"last_seen"`
	Provider        string    `json:"provider"`
}

// Recorder collects and exposes telemetry data.
type Recorder struct {
	mu sync.RWMutex

	// Atomic global counters for hot-path performance
	totalTokensSaved  int64
	totalTokensPassed int64
	totalRequests     int64
	totalCacheHits    int64
	activeStreams      int64

	sessions map[string]*SessionStats
	events   []Event
	maxEvents int

	startTime time.Time
	provider  string
}

// Event represents a single deduplication event for the activity log.
type Event struct {
	Timestamp   time.Time `json:"ts"`
	SessionID   string    `json:"session_id"`
	Strategy    string    `json:"strategy"`
	TokensSaved int64     `json:"tokens_saved"`
	BlocksHit   int       `json:"blocks_hit"`
}

// GlobalMetrics is the full metrics payload exposed via /api/metrics.
type GlobalMetrics struct {
	TotalTokensSaved  int64          `json:"total_tokens_saved"`
	TotalTokensPassed int64          `json:"total_tokens_passed"`
	TotalRequests     int64          `json:"total_requests"`
	TotalCacheHits    int64          `json:"total_cache_hits"`
	ActiveStreams      int64          `json:"active_streams"`
	EstSavingsUSD     float64        `json:"est_savings_usd"`
	CompressionRatio  float64        `json:"compression_ratio_pct"`
	UptimeSeconds     float64        `json:"uptime_seconds"`
	Provider          string         `json:"provider"`
	Sessions          []*SessionStats `json:"sessions"`
	RecentEvents      []Event        `json:"recent_events"`
}

// NewRecorder initializes a telemetry recorder.
func NewRecorder(provider string) *Recorder {
	return &Recorder{
		sessions:  make(map[string]*SessionStats),
		events:    make([]Event, 0, 500),
		maxEvents: 500,
		startTime: time.Now(),
		provider:  provider,
	}
}

// Record logs a compression outcome for a session.
func (r *Recorder) Record(sessionID, strategy string, tokensSaved, tokensPassed int64, blocksHit int) {
	atomic.AddInt64(&r.totalTokensSaved, tokensSaved)
	atomic.AddInt64(&r.totalTokensPassed, tokensPassed)
	atomic.AddInt64(&r.totalRequests, 1)
	if blocksHit > 0 {
		atomic.AddInt64(&r.totalCacheHits, int64(blocksHit))
	}

	pricing := ProviderCosts[r.provider]
	savingsUSD := float64(tokensSaved) / 1_000_000 * pricing.InputCostPer1M

	r.mu.Lock()
	defer r.mu.Unlock()

	sess, ok := r.sessions[sessionID]
	if !ok {
		sess = &SessionStats{SessionID: sessionID, Provider: r.provider}
		r.sessions[sessionID] = sess
	}
	sess.TokensSaved += tokensSaved
	sess.TokensPassed += tokensPassed
	sess.Requests++
	sess.CacheHits += int64(blocksHit)
	sess.EstSavingsUSD += savingsUSD
	sess.LastSeen = time.Now()

	// Append event with ring-buffer eviction
	evt := Event{
		Timestamp:   time.Now(),
		SessionID:   sessionID,
		Strategy:    strategy,
		TokensSaved: tokensSaved,
		BlocksHit:   blocksHit,
	}
	if len(r.events) >= r.maxEvents {
		r.events = r.events[1:]
	}
	r.events = append(r.events, evt)
}

// IncrementStreams atomically tracks active streaming connections.
func (r *Recorder) IncrementStreams() { atomic.AddInt64(&r.activeStreams, 1) }
func (r *Recorder) DecrementStreams() { atomic.AddInt64(&r.activeStreams, -1) }

// Snapshot returns an immutable copy of current global metrics.
func (r *Recorder) Snapshot() GlobalMetrics {
	saved := atomic.LoadInt64(&r.totalTokensSaved)
	passed := atomic.LoadInt64(&r.totalTokensPassed)
	pricing := ProviderCosts[r.provider]

	ratio := 0.0
	total := saved + passed
	if total > 0 {
		ratio = float64(saved) / float64(total) * 100
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	sessions := make([]*SessionStats, 0, len(r.sessions))
	for _, s := range r.sessions {
		sessions = append(sessions, s)
	}

	recentEvents := make([]Event, len(r.events))
	copy(recentEvents, r.events)

	return GlobalMetrics{
		TotalTokensSaved:  saved,
		TotalTokensPassed: passed,
		TotalRequests:     atomic.LoadInt64(&r.totalRequests),
		TotalCacheHits:    atomic.LoadInt64(&r.totalCacheHits),
		ActiveStreams:      atomic.LoadInt64(&r.activeStreams),
		EstSavingsUSD:     float64(saved) / 1_000_000 * pricing.InputCostPer1M,
		CompressionRatio:  ratio,
		UptimeSeconds:     time.Since(r.startTime).Seconds(),
		Provider:          r.provider,
		Sessions:          sessions,
		RecentEvents:      recentEvents,
	}
}

// HTTPHandler returns a metrics HTTP handler for /api/metrics.
func (r *Recorder) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		snap := r.Snapshot()
		json.NewEncoder(w).Encode(snap)
	}
}
