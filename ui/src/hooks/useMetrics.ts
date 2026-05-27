import { useEffect, useRef, useState, useCallback } from 'react'

export interface SessionStats {
  session_id: string
  tokens_saved: number
  tokens_passed: number
  requests: number
  cache_hits: number
  est_savings_usd: number
  last_seen: string
  provider: string
}

export interface MetricsEvent {
  ts: string
  session_id: string
  strategy: string
  tokens_saved: number
  blocks_hit: number
}

export interface Metrics {
  total_tokens_saved: number
  total_tokens_passed: number
  total_requests: number
  total_cache_hits: number
  active_streams: number
  est_savings_usd: number
  compression_ratio_pct: number
  uptime_seconds: number
  provider: string
  sessions: SessionStats[]
  recent_events: MetricsEvent[]
}

export interface CacheStats {
  total_entries: number
  max_size: number
  hit_rate_pct: number
  total_hits: number
  total_misses: number
  total_evicts: number
}

const PROXY_BASE = import.meta.env.VITE_PROXY_URL || 'http://localhost:8080'

// Mock data for demo / when proxy is offline
const MOCK_METRICS: Metrics = {
  total_tokens_saved: 142_830,
  total_tokens_passed: 289_440,
  total_requests: 384,
  total_cache_hits: 217,
  active_streams: 2,
  est_savings_usd: 0.43,
  compression_ratio_pct: 33.1,
  uptime_seconds: 7240,
  provider: 'anthropic',
  sessions: [
    { session_id: 'a3f8c21b', tokens_saved: 62_100, tokens_passed: 134_200, requests: 182, cache_hits: 91, est_savings_usd: 0.19, last_seen: new Date().toISOString(), provider: 'anthropic' },
    { session_id: 'd9e14c77', tokens_saved: 48_930, tokens_passed: 99_810, requests: 142, cache_hits: 78, est_savings_usd: 0.15, last_seen: new Date(Date.now() - 120_000).toISOString(), provider: 'anthropic' },
    { session_id: 'f2a70b5e', tokens_saved: 31_800, tokens_passed: 55_430, requests: 60, cache_hits: 48, est_savings_usd: 0.09, last_seen: new Date(Date.now() - 600_000).toISOString(), provider: 'anthropic' },
  ],
  recent_events: Array.from({ length: 20 }, (_, i) => ({
    ts: new Date(Date.now() - i * 18_000).toISOString(),
    session_id: ['a3f8c21b', 'd9e14c77', 'f2a70b5e'][i % 3],
    strategy: ['multi-pass', 'file-block', 'blob-dedup'][i % 3],
    tokens_saved: Math.floor(Math.random() * 4000 + 200),
    blocks_hit: Math.floor(Math.random() * 8 + 1),
  })),
}

export function useMetrics(intervalMs = 2000) {
  const [metrics, setMetrics] = useState<Metrics>(MOCK_METRICS)
  const [connected, setConnected] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const historyRef = useRef<Array<{ t: number; saved: number; passed: number }>>([])

  const fetchMetrics = useCallback(async () => {
    try {
      const res = await fetch(`${PROXY_BASE}/api/metrics`, { signal: AbortSignal.timeout(3000) })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data: Metrics = await res.json()
      setMetrics(data)
      setConnected(true)
      setError(null)

      // Append to history ring buffer (keep last 60 points)
      const now = Date.now()
      historyRef.current = [
        ...historyRef.current.slice(-59),
        { t: now, saved: data.total_tokens_saved, passed: data.total_tokens_passed },
      ]
    } catch (e: unknown) {
      setConnected(false)
      setError((e as Error).message)
      // Animate mock data when proxy is offline
      setMetrics(prev => ({
        ...MOCK_METRICS,
        total_tokens_saved: prev.total_tokens_saved + Math.floor(Math.random() * 300),
        total_tokens_passed: prev.total_tokens_passed + Math.floor(Math.random() * 600),
        active_streams: Math.random() > 0.7 ? 1 : 0,
      }))
    }
  }, [])

  useEffect(() => {
    fetchMetrics()
    const id = setInterval(fetchMetrics, intervalMs)
    return () => clearInterval(id)
  }, [fetchMetrics, intervalMs])

  return { metrics, connected, error, history: historyRef.current }
}

export function useCacheStats(intervalMs = 5000) {
  const [stats, setStats] = useState<CacheStats | null>(null)

  useEffect(() => {
    const fetch_ = async () => {
      try {
        const res = await fetch(`${PROXY_BASE}/api/cache/stats`, { signal: AbortSignal.timeout(3000) })
        if (res.ok) setStats(await res.json())
      } catch {
        setStats({
          total_entries: 8_423,
          max_size: 262_144,
          hit_rate_pct: 56.4,
          total_hits: 217,
          total_misses: 167,
          total_evicts: 12,
        })
      }
    }
    fetch_()
    const id = setInterval(fetch_, intervalMs)
    return () => clearInterval(id)
  }, [intervalMs])

  return stats
}
