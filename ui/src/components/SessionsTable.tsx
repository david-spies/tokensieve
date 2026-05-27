import React from 'react'
import { SessionStats } from '../hooks/useMetrics'

interface Props {
  sessions: SessionStats[]
}

function relTime(iso: string) {
  const diff = Date.now() - new Date(iso).getTime()
  if (diff < 60_000) return `${Math.round(diff / 1000)}s`
  if (diff < 3_600_000) return `${Math.round(diff / 60_000)}m`
  return `${Math.round(diff / 3_600_000)}h`
}

export function SessionsTable({ sessions }: Props) {
  const sorted = [...sessions].sort((a, b) => b.tokens_saved - a.tokens_saved)

  return (
    <div className="rounded-xl border overflow-hidden" style={{ background: 'var(--color-surface)', borderColor: 'var(--color-border)' }}>
      <div className="px-5 py-4 border-b" style={{ borderColor: 'var(--color-border)' }}>
        <h3 className="text-sm font-mono font-semibold tracking-wider uppercase" style={{ color: 'var(--color-muted)' }}>
          Active Sessions
        </h3>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-sm font-mono">
          <thead>
            <tr style={{ borderBottom: '1px solid var(--color-border)' }}>
              {['Session', 'Provider', 'Requests', 'Cache Hits', 'Tokens Saved', 'Est. Savings', 'Last Active'].map(h => (
                <th key={h} className="px-5 py-3 text-left text-xs font-semibold uppercase tracking-wider"
                  style={{ color: 'var(--color-muted)' }}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {sorted.map((sess) => {
              const efficiency = sess.requests > 0
                ? ((sess.cache_hits / sess.requests) * 100).toFixed(0)
                : '0'
              return (
                <tr
                  key={sess.session_id}
                  className="border-b last:border-b-0 hover:opacity-80 transition-opacity"
                  style={{ borderColor: 'var(--color-border)' }}
                >
                  <td className="px-5 py-3">
                    <span className="px-2 py-0.5 rounded text-xs" style={{ background: 'var(--color-accent-dim)', color: 'var(--color-accent)' }}>
                      {sess.session_id}
                    </span>
                  </td>
                  <td className="px-5 py-3 text-xs capitalize" style={{ color: 'var(--color-muted)' }}>
                    {sess.provider}
                  </td>
                  <td className="px-5 py-3 tabular-nums" style={{ color: 'var(--color-text)' }}>
                    {sess.requests.toLocaleString()}
                  </td>
                  <td className="px-5 py-3">
                    <span className="text-emerald-400">{sess.cache_hits.toLocaleString()}</span>
                    <span className="text-xs ml-1" style={{ color: 'var(--color-muted)' }}>({efficiency}%)</span>
                  </td>
                  <td className="px-5 py-3 tabular-nums text-emerald-400">
                    {sess.tokens_saved.toLocaleString()}
                  </td>
                  <td className="px-5 py-3 tabular-nums text-white font-semibold">
                    ${sess.est_savings_usd.toFixed(4)}
                  </td>
                  <td className="px-5 py-3 text-xs" style={{ color: 'var(--color-muted)' }}>
                    {relTime(sess.last_seen)} ago
                  </td>
                </tr>
              )
            })}
            {sorted.length === 0 && (
              <tr>
                <td colSpan={7} className="px-5 py-8 text-center text-xs" style={{ color: 'var(--color-muted)' }}>
                  No active sessions
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}
