import type { ReactNode } from 'react';

type BadgeColor = 'slate' | 'indigo' | 'emerald' | 'amber' | 'red' | 'blue' | 'purple';

interface BadgeProps {
  children: ReactNode;
  color?: BadgeColor;
}

const COLOR_MAP: Record<BadgeColor, string> = {
  slate: 'bg-slate-100 text-slate-700',
  indigo: 'bg-indigo-100 text-indigo-700',
  emerald: 'bg-emerald-100 text-emerald-700',
  amber: 'bg-amber-100 text-amber-700',
  red: 'bg-red-100 text-red-700',
  blue: 'bg-blue-100 text-blue-700',
  purple: 'bg-purple-100 text-purple-700',
};

export function Badge({ children, color = 'slate' }: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${COLOR_MAP[color]}`}
    >
      {children}
    </span>
  );
}
