import React from 'react'
import { LucideIcon } from 'lucide-react'

interface MetricCardProps {
  label: string
  value: string
  sub?: string
  icon: LucideIcon
  accent?: 'green' | 'blue' | 'amber' | 'slate' | 'rose'
  pulse?: boolean
}

const ACCENTS = {
  green: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20',
  blue:  'text-blue-400 bg-blue-500/10 border-blue-500/20',
  amber: 'text-amber-400 bg-amber-500/10 border-amber-500/20',
  slate: 'text-slate-400 bg-slate-800 border-slate-700',
  rose:  'text-rose-400 bg-rose-500/10 border-rose-500/20',
}

const VALUE_COLOR = {
  green: 'text-emerald-400',
  blue:  'text-white',
  amber: 'text-amber-400',
  slate: 'text-slate-300',
  rose:  'text-rose-400',
}

export function MetricCard({ label, value, sub, icon: Icon, accent = 'slate', pulse }: MetricCardProps) {
  return (
    <div className="relative rounded-xl border overflow-hidden" style={{ background: 'var(--color-surface)', borderColor: 'var(--color-border)' }}>
      {/* Subtle grid pattern */}
      <div className="absolute inset-0 grid-bg opacity-40 pointer-events-none" />

      <div className="relative p-6">
        <div className="flex justify-between items-start">
          <p className="text-xs font-mono uppercase tracking-widest" style={{ color: 'var(--color-muted)' }}>
            {label}
          </p>
          <div className={`p-2.5 rounded-lg border ${ACCENTS[accent]}`}>
            <Icon className="h-4 w-4" />
          </div>
        </div>

        <div className="mt-3 flex items-end gap-2">
          <span className={`text-3xl font-mono font-bold tabular-nums ${VALUE_COLOR[accent]} ${pulse ? 'cursor-blink' : ''}`}>
            {value}
          </span>
        </div>

        {sub && (
          <p className="mt-2 text-xs font-mono" style={{ color: 'var(--color-muted)' }}>
            {sub}
          </p>
        )}
      </div>
    </div>
  )
}
