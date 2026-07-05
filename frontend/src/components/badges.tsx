import type { AuditStatus, Severity, Source } from "../types/api";

const SEV_STYLES: Record<Severity, string> = {
  critical: "bg-critical/10 text-critical border-critical/30",
  high: "bg-high/10 text-high border-high/30",
  medium: "bg-medium/10 text-medium border-medium/30",
  low: "bg-low/10 text-low border-low/30",
};

export function SeverityBadge({ severity }: { severity: Severity }) {
  return (
    <span
      className={`inline-flex items-center rounded border px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider ${SEV_STYLES[severity]}`}
    >
      {severity}
    </span>
  );
}

const STATUS_STYLES: Record<AuditStatus, { cls: string; label: string; pulse?: boolean }> = {
  pending: { cls: "bg-ink-400", label: "Pending", pulse: true },
  running: { cls: "bg-signal-500", label: "Running", pulse: true },
  completed: { cls: "bg-low", label: "Completed" },
  failed: { cls: "bg-critical", label: "Failed" },
  cancelled: { cls: "bg-ink-400", label: "Cancelled" },
};

export function StatusBadge({ status }: { status: AuditStatus }) {
  const s = STATUS_STYLES[status];
  return (
    <span className="inline-flex items-center gap-1.5 text-xs font-medium">
      <span className={`h-1.5 w-1.5 rounded-full ${s.cls} ${s.pulse ? "pulse-dot" : ""}`} />
      {s.label}
    </span>
  );
}

export function SourceBadge({ source }: { source: Source }) {
  return source === "ai" ? (
    <span className="inline-flex items-center rounded border border-signal-600/30 bg-signal-500/10 px-1.5 py-0.5 font-mono text-[10px] font-semibold text-signal-600">
      AI
    </span>
  ) : (
    <span className="inline-flex items-center rounded border border-ink-400/30 bg-ink-400/10 px-1.5 py-0.5 font-mono text-[10px] font-semibold text-ink-400">
      RULE
    </span>
  );
}

export function TriggerBadge({ trigger }: { trigger?: string }) {
  if (trigger !== "scheduled") return null;
  return (
    <span
      title="Started automatically by the scheduler"
      className="inline-flex items-center gap-1 rounded border border-signal-600/30 bg-signal-500/10 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-signal-600"
    >
      <svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round">
        <path d="M12 6v6l4 2M12 2a10 10 0 100 20 10 10 0 000-20z" />
      </svg>
      auto
    </span>
  );
}

// NewBadge marks a finding that appeared since the previous audit.
export function NewBadge() {
  return (
    <span className="inline-flex items-center rounded bg-high/15 px-1.5 py-0.5 text-[9.5px] font-bold uppercase tracking-wider text-high">
      new
    </span>
  );
}

export function ConfidenceMeter({ value }: { value: number }) {
  const pct = Math.round(value * 100);
  return (
    <span className="inline-flex items-center gap-1.5">
      <span className="h-1 w-10 overflow-hidden rounded-full bg-line">
        <span
          className="block h-full rounded-full bg-signal-500"
          style={{ width: `${pct}%` }}
        />
      </span>
      <span className="font-mono text-[11px] text-ink-400">{pct}%</span>
    </span>
  );
}
