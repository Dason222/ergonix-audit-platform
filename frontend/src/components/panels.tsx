import type { ReactNode } from "react";

export function Panel({
  title,
  action,
  children,
  className = "",
}: {
  title?: string;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={`rounded-lg border border-line bg-panel ${className}`}>
      {(title || action) && (
        <header className="flex items-center justify-between border-b border-line px-4 py-2.5">
          <h2 className="microlabel">{title}</h2>
          {action}
        </header>
      )}
      {children}
    </section>
  );
}

export function StatCard({
  label,
  value,
  hint,
  tone = "default",
  className = "",
}: {
  label: string;
  value: ReactNode;
  hint?: string;
  tone?: "default" | "critical" | "high" | "medium" | "low";
  className?: string;
}) {
  const toneCls =
    tone === "default"
      ? "text-ink-900"
      : { critical: "text-critical", high: "text-high", medium: "text-medium", low: "text-low" }[tone];
  return (
    <div className={`rounded-lg border border-line bg-panel px-4 py-3 ${className}`}>
      <div className="microlabel">{label}</div>
      <div className={`mt-1 font-mono text-2xl font-semibold leading-none ${toneCls}`}>
        {value}
      </div>
      {hint && <div className="mt-1.5 text-[11px] text-ink-400">{hint}</div>}
    </div>
  );
}

export function EmptyState({
  title,
  subtitle,
  action,
}: {
  title: string;
  subtitle?: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 px-6 py-16 text-center">
      <svg width="36" height="36" viewBox="0 0 32 32" className="opacity-30">
        <rect width="32" height="32" rx="6" fill="#101826" />
        <path
          d="M8 22 L14 10 L18 18 L21 13 L24 22"
          stroke="#2dd4bf"
          strokeWidth="2.5"
          fill="none"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
      <p className="text-sm font-medium text-ink-900">{title}</p>
      {subtitle && <p className="max-w-sm text-xs text-ink-400">{subtitle}</p>}
      {action}
    </div>
  );
}

export function Spinner({ label }: { label?: string }) {
  return (
    <div className="flex items-center justify-center gap-2 py-14 text-ink-400">
      <span className="h-4 w-4 animate-spin rounded-full border-2 border-line border-t-signal-500" />
      {label && <span className="text-xs">{label}</span>}
    </div>
  );
}

export function ErrorNote({ message }: { message: string }) {
  return (
    <div className="m-4 rounded border border-critical/30 bg-critical/5 px-3 py-2 text-xs text-critical">
      {message}
    </div>
  );
}
