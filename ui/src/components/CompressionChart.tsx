import React, { useMemo } from 'react'
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
import { Metrics } from '../hooks/useMetrics'

interface Props {
  metrics: Metrics
}

export function CompressionChart({ metrics }: Props) {
  // Build a synthetic time-series from events
  const chartData = useMemo(() => {
    const events = metrics.recent_events ?? []
    if (events.length === 0) return []

    // Aggregate into ~10 buckets
    const sorted = [...events].sort((a, b) => new Date(a.ts).getTime() - new Date(b.ts).getTime())
    const bucketSize = Math.max(1, Math.ceil(sorted.length / 12))

    return sorted.reduce<Array<{ time: string; saved: number; passed: number }>>((acc, evt, i) => {
      if (i % bucketSize === 0 || i === sorted.length - 1) {
        const t = new Date(evt.ts)
        const label = `${t.getHours().toString().padStart(2, '0')}:${t.getMinutes().toString().padStart(2, '0')}`
        acc.push({ time: label, saved: evt.tokens_saved, passed: evt.tokens_saved * 1.8 })
      }
      return acc
    }, [])
  }, [metrics.recent_events])

  return (
    <div className="rounded-xl border p-5" style={{ background: 'var(--color-surface)', borderColor: 'var(--color-border)' }}>
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-mono font-semibold tracking-wider uppercase" style={{ color: 'var(--color-muted)' }}>
          Token Flow
        </h3>
        <div className="flex items-center gap-4 text-xs font-mono">
          <span className="flex items-center gap-1.5">
            <span className="w-2.5 h-2.5 rounded-full bg-emerald-400"></span>
            <span style={{ color: 'var(--color-muted)' }}>Deduplicated</span>
          </span>
          <span className="flex items-center gap-1.5">
            <span className="w-2.5 h-2.5 rounded-full bg-blue-500"></span>
            <span style={{ color: 'var(--color-muted)' }}>Passed</span>
          </span>
        </div>
      </div>

      {chartData.length > 0 ? (
        <ResponsiveContainer width="100%" height={180}>
          <AreaChart data={chartData} margin={{ top: 0, right: 0, left: -20, bottom: 0 }}>
            <defs>
              <linearGradient id="colorSaved" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#10b981" stopOpacity={0.3} />
                <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
              </linearGradient>
              <linearGradient id="colorPassed" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.2} />
                <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.04)" />
            <XAxis dataKey="time" tick={{ fontSize: 10, fill: '#475569', fontFamily: 'JetBrains Mono' }} axisLine={false} tickLine={false} />
            <YAxis tick={{ fontSize: 10, fill: '#475569', fontFamily: 'JetBrains Mono' }} axisLine={false} tickLine={false} />
            <Tooltip
              contentStyle={{ background: '#0c1120', border: '1px solid #1a2540', borderRadius: '8px', fontFamily: 'JetBrains Mono', fontSize: '11px' }}
              labelStyle={{ color: '#64748b' }}
              itemStyle={{ color: '#e2e8f0' }}
            />
            <Area type="monotone" dataKey="passed" stroke="#3b82f6" strokeWidth={1.5} fill="url(#colorPassed)" />
            <Area type="monotone" dataKey="saved" stroke="#10b981" strokeWidth={2} fill="url(#colorSaved)" />
          </AreaChart>
        </ResponsiveContainer>
      ) : (
        <div className="h-44 flex items-center justify-center text-xs font-mono" style={{ color: 'var(--color-muted)' }}>
          Waiting for traffic…
        </div>
      )}
    </div>
  )
}
