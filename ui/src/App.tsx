import React from 'react'
import {
  Shield, Zap, CircleDollarSign, ArrowUpRight,
  Activity, Database, Cpu, RefreshCw
} from 'lucide-react'
import { useMetrics, useCacheStats } from './hooks/useMetrics'
import { MetricCard } from './components/MetricCard'
import { ActivityFeed } from './components/ActivityFeed'
import { SessionsTable } from './components/SessionsTable'
import { CompressionChart } from './components/CompressionChart'

function fmt(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(2) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'k'
  return n.toString()
}

function uptime(seconds: number) {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  return `${h}h ${m}m ${s}s`
}

export default function App() {
  const { metrics, connected, error } = useMetrics(2000)
  const cacheStats = useCacheStats(5000)

  const ratio = metrics.compression_ratio_pct ?? 0

  return (
    <div className="min-h-screen grid-bg" style={{ background: 'var(--color-bg)' }}>
      {/* ── Header ── */}
      <header className="sticky top-0 z-50 border-b" style={{ background: 'rgba(5,8,16,0.95)', backdropFilter: 'blur(12px)', borderColor: 'var(--color-border)' }}>
        <div className="max-w-screen-2xl mx-auto px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg" style={{ background: 'var(--color-accent-dim)', border: '1px solid var(--color-accent-border)' }}>
              <Shield className="h-5 w-5" style={{ color: 'var(--color-accent)' }} />
            </div>
            <div>
              <h1 className="text-lg font-display font-bold tracking-tight leading-none glow-accent" style={{ color: 'var(--color-accent)' }}>
                TokenSieve
              </h1>
              <p className="text-xs font-mono mt-0.5" style={{ color: 'var(--color-muted)' }}>
                Context-Deduplicating RAG Proxy · v1.0.0
              </p>
            </div>
          </div>

          <div className="flex items-center gap-4">
            {/* Uptime */}
            <div className="hidden md:flex items-center gap-2 text-xs font-mono px-3 py-1.5 rounded-lg" style={{ background: 'var(--color-surface)', border: '1px solid var(--color-border)', color: 'var(--color-muted)' }}>
              <Cpu className="h-3.5 w-3.5" />
              <span>up {uptime(metrics.uptime_seconds)}</span>
            </div>

            {/* Provider badge */}
            <div className="text-xs font-mono px-3 py-1.5 rounded-lg capitalize" style={{ background: 'var(--color-surface)', border: '1px solid var(--color-border)', color: 'var(--color-text)' }}>
              {metrics.provider}
            </div>

            {/* Connection status */}
            <div className="flex items-center gap-2 text-xs font-mono px-3 py-1.5 rounded-lg" style={{ background: 'var(--color-surface)', border: '1px solid var(--color-border)' }}>
              <div className={`h-2 w-2 rounded-full ${connected ? 'bg-emerald-400' : 'bg-amber-400'}`}
                style={connected ? { animation: 'pulse-glow 2s ease infinite' } : { animation: 'blink 1.5s step-end infinite' }}
              />
              <span style={{ color: 'var(--color-muted)' }}>
                {connected ? 'Proxy connected' : error ? 'Demo mode' : 'Connecting…'}
              </span>
            </div>
          </div>
        </div>
      </header>

      <main className="max-w-screen-2xl mx-auto px-6 py-8 space-y-6">

        {/* ── KPI Row ── */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <MetricCard
            label="Tokens Deduplicated"
            value={fmt(metrics.total_tokens_saved)}
            sub={`Compression ratio: ${ratio.toFixed(1)}%`}
            icon={Zap}
            accent="green"
            pulse
          />
          <MetricCard
            label="Estimated Savings"
            value={`$${metrics.est_savings_usd.toFixed(4)}`}
            sub={`$3/M blended input rate`}
            icon={CircleDollarSign}
            accent="blue"
          />
          <MetricCard
            label="Tokens Passed Through"
            value={fmt(metrics.total_tokens_passed)}
            sub={`${metrics.total_requests.toLocaleString()} total requests`}
            icon={ArrowUpRight}
            accent="slate"
          />
          <MetricCard
            label="Active Streams"
            value={String(metrics.active_streams)}
            sub={`${metrics.total_cache_hits.toLocaleString()} total cache hits`}
            icon={Activity}
            accent={metrics.active_streams > 0 ? 'amber' : 'slate'}
            pulse={metrics.active_streams > 0}
          />
        </div>

        {/* ── Cache Stats Banner ── */}
        {cacheStats && (
          <div className="rounded-xl border px-5 py-4 flex flex-wrap gap-6 items-center" style={{ background: 'var(--color-surface)', borderColor: 'var(--color-border)' }}>
            <div className="flex items-center gap-2">
              <Database className="h-4 w-4" style={{ color: 'var(--color-accent)' }} />
              <span className="text-xs font-mono uppercase tracking-wider" style={{ color: 'var(--color-muted)' }}>
                Sliding Window Cache
              </span>
            </div>
            {[
              { label: 'Entries', value: cacheStats.total_entries.toLocaleString() },
              { label: 'Capacity', value: `${((cacheStats.total_entries / cacheStats.max_size) * 100).toFixed(1)}%` },
              { label: 'Hit Rate', value: `${cacheStats.hit_rate_pct.toFixed(1)}%` },
              { label: 'Total Hits', value: cacheStats.total_hits.toLocaleString() },
              { label: 'Evictions', value: cacheStats.total_evicts.toLocaleString() },
            ].map(({ label, value }) => (
              <div key={label} className="flex flex-col">
                <span className="text-xs font-mono" style={{ color: 'var(--color-muted)' }}>{label}</span>
                <span className="text-sm font-mono font-semibold tabular-nums" style={{ color: 'var(--color-text)' }}>{value}</span>
              </div>
            ))}
          </div>
        )}

        {/* ── Chart + Activity Feed ── */}
        <div className="grid grid-cols-1 lg:grid-cols-5 gap-4">
          <div className="lg:col-span-3">
            <CompressionChart metrics={metrics} />
          </div>
          <div className="lg:col-span-2">
            <ActivityFeed events={metrics.recent_events ?? []} />
          </div>
        </div>

        {/* ── Sessions Table ── */}
        <SessionsTable sessions={metrics.sessions ?? []} />

        {/* ── Architecture Info ── */}
        <div className="rounded-xl border p-6" style={{ background: 'var(--color-surface)', borderColor: 'var(--color-border)' }}>
          <div className="flex items-center gap-2 mb-3">
            <RefreshCw className="h-4 w-4" style={{ color: 'var(--color-accent)' }} />
            <h4 className="text-sm font-mono font-semibold uppercase tracking-wider" style={{ color: 'var(--color-muted)' }}>
              How TokenSieve Manages Context Pipelines
            </h4>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 text-xs font-mono leading-relaxed" style={{ color: 'var(--color-muted)' }}>
            <div className="p-4 rounded-lg" style={{ background: 'var(--color-bg)', border: '1px solid var(--color-border)' }}>
              <p className="font-semibold mb-1" style={{ color: 'var(--color-accent)' }}>① Intercept</p>
              <p>Every API request passes through the proxy. Payloads are parsed, session IDs derived, and the message array decomposed into semantic segments.</p>
            </div>
            <div className="p-4 rounded-lg" style={{ background: 'var(--color-bg)', border: '1px solid var(--color-border)' }}>
              <p className="font-semibold mb-1" style={{ color: 'var(--color-accent)' }}>② Deduplicate</p>
              <p>File blocks, stack traces, and large blobs are SHA-256 fingerprinted. Cache hits replace entire segments with compact reference tokens, preventing re-transmission.</p>
            </div>
            <div className="p-4 rounded-lg" style={{ background: 'var(--color-bg)', border: '1px solid var(--color-border)' }}>
              <p className="font-semibold mb-1" style={{ color: 'var(--color-accent)' }}>③ Forward</p>
              <p>The compressed payload is forwarded to the upstream LLM. Savings are logged, telemetry updated, and the upstream response streams back unmodified.</p>
            </div>
          </div>
        </div>
      </main>

      <footer className="border-t mt-8 px-6 py-4" style={{ borderColor: 'var(--color-border)' }}>
        <p className="text-xs font-mono text-center" style={{ color: 'var(--color-muted)' }}>
          TokenSieve · MIT License · Enterprise Context Deduplication Layer
        </p>
      </footer>
    </div>
  )
}
