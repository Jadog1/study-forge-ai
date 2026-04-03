import type { ReactNode } from 'react';

interface CardProps {
  children: ReactNode;
  header?: ReactNode;
  footer?: ReactNode;
  className?: string;
  onClick?: () => void;
}

export function Card({ children, header, footer, className = '', onClick }: CardProps) {
  const interactive = !!onClick;
  return (
    <div
      className={`rounded-xl border border-slate-200 bg-white shadow-sm ${
        interactive ? 'cursor-pointer hover:shadow-md hover:border-slate-300 transition-all' : ''
      } ${className}`}
      onClick={onClick}
      role={interactive ? 'button' : undefined}
      tabIndex={interactive ? 0 : undefined}
      onKeyDown={interactive ? (e) => { if (e.key === 'Enter') onClick?.(); } : undefined}
    >
      {header && (
        <div className="border-b border-slate-200 px-5 py-3">{header}</div>
      )}
      <div className="px-5 py-4">{children}</div>
      {footer && (
        <div className="border-t border-slate-200 px-5 py-3">{footer}</div>
      )}
    </div>
  );
}
