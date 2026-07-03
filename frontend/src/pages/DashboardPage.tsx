import { Link } from "react-router-dom";

import { CategoryBars, IssuesOverTime, SeverityDonut, WebsiteBars } from "../charts";
import { StatusBadge } from "../components/badges";
import { EmptyState, ErrorNote, Panel, Spinner, StatCard } from "../components/panels";
import { useDashboard } from "../hooks/useApi";
import { fmtDateTime, fmtDuration } from "../utils/format";

export default function DashboardPage() {
  const { data, isLoading, error } = useDashboard();

  if (isLoading) return <Spinner label="Loading dashboard…" />;
  if (error) return <ErrorNote message={`Failed to load dashboard: ${error.message}`} />;
  if (!data) return null;

  const noData = data.totalAudits === 0;

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-8">
        <StatCard className="rise rise-1" label="Audits run" value={data.totalAudits} />
        <StatCard
          className="rise rise-1"
          label="Websites audited"
          value={data.totalWebsitesAudited}
        />
        <StatCard
          className="rise rise-2"
          label="Pages scanned"
          value={data.totalPagesScanned}
        />
        <StatCard
          className="rise rise-2"
          label="Avg duration"
          value={fmtDuration(data.avgAuditDurationMs)}
        />
        <StatCard className="rise rise-3" label="Total issues" value={data.totalIssues} />
        <StatCard
          className="rise rise-3"
          label="Critical"
          value={data.bySeverity.critical ?? 0}
          tone="critical"
        />
        <StatCard
          className="rise rise-4"
          label="High"
          value={data.bySeverity.high ?? 0}
          tone="high"
        />
        <StatCard
          className="rise rise-4"
          label="Medium / Low"
          value={`${data.bySeverity.medium ?? 0} / ${data.bySeverity.low ?? 0}`}
        />
      </div>

      {noData ? (
        <Panel>
          <EmptyState
            title="No audits yet"
            subtitle="Run your first audit to populate the dashboard with issues, charts and history."
            action={
              <Link
                to="/audits/new"
                className="mt-2 rounded-md bg-ink-900 px-4 py-2 text-xs font-semibold text-white hover:bg-ink-800"
              >
                Create audit
              </Link>
            }
          />
        </Panel>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <Panel title="Issues by severity" className="rise rise-2">
              <div className="p-3">
                <SeverityDonut data={data.bySeverity} />
              </div>
            </Panel>
            <Panel title="Issues over time" className="rise rise-2">
              <div className="p-3">
                <IssuesOverTime data={data.issuesOverTime} />
              </div>
            </Panel>
            <Panel title="Issues by category" className="rise rise-3">
              <div className="p-3">
                <CategoryBars data={data.byCategory} />
              </div>
            </Panel>
            <Panel title="Issues by website" className="rise rise-3">
              <div className="p-3">
                <WebsiteBars data={data.byWebsite} />
              </div>
            </Panel>
          </div>

          <Panel
            title="Recent audits"
            className="rise rise-4"
            action={
              <Link to="/audits" className="text-xs font-medium text-signal-600 hover:underline">
                View all →
              </Link>
            }
          >
            <table className="w-full text-[13px]">
              <thead>
                <tr className="border-b border-line text-left">
                  <th className="microlabel px-4 py-2">ID</th>
                  <th className="microlabel px-4 py-2">Status</th>
                  <th className="microlabel px-4 py-2">Websites</th>
                  <th className="microlabel px-4 py-2 text-right">Pages</th>
                  <th className="microlabel px-4 py-2 text-right">Issues</th>
                  <th className="microlabel px-4 py-2 text-right">Duration</th>
                  <th className="microlabel px-4 py-2 text-right">Created</th>
                </tr>
              </thead>
              <tbody>
                {data.recentAudits.map((a) => (
                  <tr key={a.id} className="border-b border-line/60 last:border-0 hover:bg-surface">
                    <td className="px-4 py-2 font-mono">
                      <Link to={`/audits/${a.id}`} className="text-signal-600 hover:underline">
                        #{a.id}
                      </Link>
                    </td>
                    <td className="px-4 py-2">
                      <StatusBadge status={a.status} />
                    </td>
                    <td className="px-4 py-2 text-ink-400">
                      {a.params.websites.length} site{a.params.websites.length === 1 ? "" : "s"}
                    </td>
                    <td className="px-4 py-2 text-right font-mono">{a.stats.totalPages}</td>
                    <td className="px-4 py-2 text-right font-mono">{a.stats.totalIssues}</td>
                    <td className="px-4 py-2 text-right font-mono">
                      {fmtDuration(a.stats.durationMs)}
                    </td>
                    <td className="px-4 py-2 text-right font-mono text-[12px] text-ink-400">
                      {fmtDateTime(a.createdAt)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Panel>
        </>
      )}
    </div>
  );
}
