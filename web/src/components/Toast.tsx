import { useEffect } from 'react';
import { CheckCircle, AlertTriangle, Info, XCircle, X } from 'lucide-react';

export type ToastVariant = 'success' | 'error' | 'info' | 'warning';

interface ToastProps {
  message: string;
  variant: ToastVariant;
  onDismiss: () => void;
  duration?: number;
}

const VARIANT_STYLES: Record<ToastVariant, { bg: string; icon: typeof Info }> = {
  success: { bg: 'bg-emerald-50 border-emerald-300 text-emerald-800', icon: CheckCircle },
  error: { bg: 'bg-red-50 border-red-300 text-red-800', icon: XCircle },
  warning: { bg: 'bg-amber-50 border-amber-300 text-amber-800', icon: AlertTriangle },
  info: { bg: 'bg-blue-50 border-blue-300 text-blue-800', icon: Info },
};

export function Toast({ message, variant, onDismiss, duration = 4000 }: ToastProps) {
  useEffect(() => {
    const timer = setTimeout(onDismiss, duration);
    return () => clearTimeout(timer);
  }, [onDismiss, duration]);

  const { bg, icon: Icon } = VARIANT_STYLES[variant];

  return (
    <div
      className={`flex items-center gap-2 rounded-lg border px-4 py-3 shadow-lg animate-in slide-in-from-right ${bg}`}
      role="alert"
    >
      <Icon className="h-4 w-4 shrink-0" />
      <span className="text-sm font-medium">{message}</span>
      <button
        onClick={onDismiss}
        className="ml-2 shrink-0 rounded p-0.5 opacity-60 hover:opacity-100 transition-opacity"
      >
        <X className="h-3.5 w-3.5" />
      </button>
    </div>
  );
}
