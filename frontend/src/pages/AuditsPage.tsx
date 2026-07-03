import { Link } from "react-router-dom";

import { StatusBadge } from "../components/badges";
import { EmptyState, ErrorNote, Panel, Spinner } from "../components/panels";
import { useAudits, useDeleteAudit } from "../hooks/useApi";
import { fmtDateTime, fmtDuration, hostOf } from "../utils/format";

export default function AuditsPage() {
  const { data, isLoading, error } = useAudits();
  const del = useDeleteAudit();

  if (isLoading) return <Spinner label="Loading audits…" />;
  if (error) return <ErrorNote message={`Failed to load audits: ${error.message}`} />;

  const audits = data?.audits ?? [];

  return (
    <Panel
      title={`All audits (${data?.total ?? 0})`}
      className="rise rise-1"
      action={
        <Link
          to="/audits/new"
          className="rounded-md bg-ink-900 px-3 py-1.5 text-xs font-semibold text-white hover:bg-ink-800"
        >
          + New audit
        </Link>
      }
    >
      {audits.length === 0 ? (
        <EmptyState
          title="No audits recorded"
          subtitle="Every audit you run is stored here with its full report."
        />
      ) : (
        <table className="w-full text-[13px]">
          <thead>
            <tr className="border-b border-line text-left">
              <th className="microlabel px-4 py-2">ID</th>
              <th className="microlabel px-4 py-2">Status</th>
              <th className="microlabel px-4 py-2">Websites</th>
              <th className="microlabel px-4 py-2 text-right">Pages</th>
              <th className="microlabel px-4 py-2 text-right">Issues</th>
              <th className="microlabel px-4 py-2 text-right">Critical</th>
              <th className="microlabel px-4 py-2 text-right">Duration</th>
              <th className="microlabel px-4 py-2 text-right">Created</th>
              <th className="microlabel px-4 py-2 text-right">AI</th>
              <th className="px-2 py-2" />
            </tr>
          </thead>
          <tbody>
            {audits.map((a) => (
              <tr key={a.id} className="border-b border-line/60 last:border-0 hover:bg-surface">
                <td className="px-4 py-2.5 font-mono">
                  <Link to={`/audits/${a.id}`} className="font-medium text-signal-600 hover:underline">
                    #{a.id}
                  </Link>
                </td>
                <td className="px-4 py-2.5">
                  <StatusBadge status={a.status} />
                </td>
                <td className="max-w-[260px] px-4 py-2.5">
                  <div className="flex flex-wrap gap-1">
                    {a.params.websites.map((w) => (
                      <span
                        key={w}
                        className="rounded bg-surface px-1.5 py-0.5 font-mono text-[11px] text-ink-700"
                      >
                        {hostOf(w)}
                      </span>
                    ))}
                  </div>
                </td>
                <td className="px-4 py-2.5 text-right font-mono">{a.stats.totalPages}</td>
                <td className="px-4 py-2.5 text-right font-mono">{a.stats.totalIssues}</td>
                <td className="px-4 py-2.5 text-right font-mono">
                  {(a.stats.bySeverity?.critical ?? 0) > 0 ? (
                    <span className="font-semibold text-critical">
                      {a.stats.bySeverity.critical}
                    </span>
                  ) : (
                    <span className="text-ink-400">0</span>
                  )}
                </td>
                <td className="px-4 py-2.5 text-right font-mono">{fmtDuration(a.stats.durationMs)}</td>
                <td className="px-4 py-2.5 text-right font-mono text-[12px] text-ink-400">
                  {fmtDateTime(a.createdAt)}
                </td>
                <td className="px-4 py-2.5 text-right">
                  {a.params.useAI && !a.stats.aiSkipped ? (
                    <span className="font-mono text-[10px] font-semibold text-signal-600">ON</span>
                  ) : (
                    <span className="font-mono text-[10px] text-ink-400">OFF</span>
                  )}
                </td>
                <td className="px-2 py-2.5 text-right">
                  <button
                    title="Delete audit"
                    onClick={() => {
                      if (confirm(`Delete audit #${a.id} and all its data?`)) {
                        del.mutate(a.id);
                      }
                    }}
                    className="rounded p-1 text-ink-400 hover:bg-critical/10 hover:text-critical"
                  >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round">
                      <path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3" />
                    </svg>
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Panel>
  );
}
