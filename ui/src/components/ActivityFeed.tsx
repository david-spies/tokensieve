import React from 'react'
import { MetricsEvent } from '../hooks/useMetrics'
import { Zap, FileCode, Layers } from 'lucide-react'

const STRATEGY_META: Record<string, { label: string; Icon: React.FC<{ className?: string }>; color: string }> = {
  'multi-pass': { label: 'Multi-Pass', Icon: Layers, color: 'text-emerald-400' },
  'file-block': { label: 'File Block', Icon: FileCode, color: 'text-blue-400' },
  'blob-dedup': { label: 'Blob Dedup', Icon: Zap, color: 'text-amber-400' },
}

function relTime(iso: string) {
  const diff = Date.now() - new Date(iso).getTime()
  if (diff < 60_000) return `${Math.round(diff / 1000)}s ago`
  if (diff < 3_600_000) return `${Math.round(diff / 60_000)}m ago`
  return `${Math.round(diff / 3_600_000)}h ago`
}

interface Props {
  events: MetricsEvent[]
}

export function ActivityFeed({ events }: Props) {
  return (
    <div className="rounded-xl border overflow-hidden" style={{ background: 'var(--color-surface)', borderColor: 'var(--color-border)' }}>
      <div className="px-5 py-4 border-b flex items-center justify-between" style={{ borderColor: 'var(--color-border)' }}>
        <h3 className="text-sm font-mono font-semibold tracking-wider uppercase" style={{ color: 'var(--color-muted)' }}>
          Compression Events
        </h3>
        <span className="text-xs font-mono px-2 py-0.5 rounded" style={{ background: 'var(--color-accent-dim)', color: 'var(--color-accent)', border: '1px solid var(--color-accent-border)' }}>
          Live
        </span>
      </div>

      <div className="overflow-y-auto" style={{ maxHeight: '320px' }}>
        {events.length === 0 ? (
          <div className="p-8 text-center text-sm font-mono" style={{ color: 'var(--color-muted)' }}>
            No events yet — awaiting requests…
          </div>
        ) : (
          [...events].reverse().map((evt, i) => {
            const meta = STRATEGY_META[evt.strategy] ?? STRATEGY_META['multi-pass']
            const IconComp = meta.Icon
            return (
              <div
                key={i}
                className="px-5 py-3 flex items-center gap-4 border-b last:border-b-0 hover:opacity-80 transition-opacity"
                style={{ borderColor: 'var(--color-border)' }}
              >
                <div className={`flex-shrink-0 ${meta.color}`}>
                  <IconComp className="h-4 w-4" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className={`text-xs font-mono font-semibold ${meta.color}`}>{meta.label}</span>
                    <span className="text-xs font-mono" style={{ color: 'var(--color-muted)' }}>
                      sess:{evt.session_id.slice(0, 8)}
                    </span>
                  </div>
                  <div className="mt-0.5 text-xs font-mono" style={{ color: 'var(--color-muted)' }}>
                    {evt.blocks_hit} block{evt.blocks_hit !== 1 ? 's' : ''} hit · saved ≈{evt.tokens_saved.toLocaleString()} tok
                  </div>
                </div>
                <div className="text-xs font-mono flex-shrink-0" style={{ color: 'var(--color-muted)' }}>
                  {relTime(evt.ts)}
                </div>
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}
