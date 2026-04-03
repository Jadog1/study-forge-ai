interface LoadingSpinnerProps {
  size?: 'sm' | 'md' | 'lg';
  label?: string;
}

const SIZE_MAP = {
  sm: 'h-4 w-4 border-2',
  md: 'h-8 w-8 border-2',
  lg: 'h-12 w-12 border-3',
} as const;

export function LoadingSpinner({ size = 'md', label }: LoadingSpinnerProps) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-8">
      <div
        className={`${SIZE_MAP[size]} animate-spin rounded-full border-slate-200 border-t-indigo-600`}
        role="status"
        aria-label={label ?? 'Loading'}
      />
      {label && <span className="text-sm text-slate-500">{label}</span>}
    </div>
  );
}
