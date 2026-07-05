import { useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";

import { api } from "../api/client";
import { CategoryBars, SeverityDonut, WebsiteBars } from "../charts";
import IssuesTable from "../components/IssuesTable";
import { StatusBadge, TriggerBadge } from "../components/badges";
import { ErrorNote, Panel, Spinner, StatCard } from "../components/panels";
import { useAudit, useCancelAudit, useIssues, usePages } from "../hooks/useApi";
import type { Audit, AuditSite, Page } from "../types/api";
import { fmtDateTime, fmtDuration, hostOf, pathOf } from "../utils/format";

const STAGES = [
  { key: "crawling", label: "Crawl" },
  { key: "checking", label: "Rule checks" },
  { key: "ai_analysis", label: "AI analysis" },
  { key: "reporting", label: "Report" },
  { key: "done", label: "Done" },
];

export default function AuditDetailPage() {
  const { id } = useParams();
  const auditId = Number(id);
  const { data: audit, isLoading, error } = useAudit(auditId);

  if (isLoading) return <Spinner label="Loading audit…" />;
  if (error) return <ErrorNote message={`Failed to load audit: ${error.message}`} />;
  if (!audit) return null;

  const running = audit.status === "pending" || audit.status === "running";

  return (
    <div className="space-y-4">
      <AuditHeader audit={audit} />
      {running ? <ProgressView audit={audit} /> : <ResultView audit={audit} />}
    </div>
  );
}

function AuditHeader({ audit }: { audit: Audit }) {
  const cancel = useCancelAudit(audit.id);
  const running = audit.status === "pending" || audit.status === "running";

  return (
    <div className="rise rise-1 flex flex-wrap items-center justify-between gap-3">
      <div className="flex items-center gap-3">
        <h2 className="font-mono text-lg font-semibold">Audit #{audit.id}</h2>
        <StatusBadge status={audit.status} />
        <TriggerBadge trigger={audit.trigger} />
        <span className="text-xs text-ink-400">
          {fmtDateTime(audit.createdAt)} · {audit.params.websites.length} website
          {audit.params.websites.length === 1 ? "" : "s"} · max {audit.params.maxPages}{" "}
          pages/site
          {audit.params.useAI && !audit.stats.aiSkipped ? " · AI on" : ""}
        </span>
      </div>
      <div className="flex items-center gap-2">
        {running ? (
          <button
            onClick={() => cancel.mutate()}
            className="rounded-md border border-critical/40 px-3 py-1.5 text-xs font-semibold text-critical hover:bg-critical/5"
          >
            Cancel audit
          </button>
        ) : (
          <ExportMenu auditId={audit.id} />
        )}
      </div>
    </div>
  );
}

function ExportMenu({ auditId }: { auditId: number }) {
  return (
    <div className="flex items-center gap-1.5">
      <span className="microlabel mr-1">Export</span>
      {(["json", "csv", "html", "pdf"] as const).map((f) => (
        <a
          key={f}
          href={api.exportUrl(auditId, f)}
          className="rounded-md border border-line bg-panel px-2.5 py-1.5 font-mono text-[11px] font-semibold uppercase text-ink-700 transition-colors hover:border-signal-500 hover:text-signal-600"
        >
          {f}
        </a>
      ))}
    </div>
  );
}

function ProgressView({ audit }: { audit: Audit }) {
  const activeIdx = STAGES.findIndex((s) => s.key === audit.stage);

  return (
    <div className="space-y-4">
      <Panel title="Pipeline" className="rise rise-2">
        <div className="flex items-center gap-0 p-5">
          {STAGES.map((stage, i) => {
            const isDone = activeIdx > i || audit.stage === "done";
            const isActive = activeIdx === i && audit.stage !== "done";
            return (
              <div key={stage.key} className="flex flex-1 items-center last:flex-none">
                <div className="flex flex-col items-center gap-1.5">
                  <div
                    className={`flex h-7 w-7 items-center justify-center rounded-full border-2 font-mono text-[11px] font-bold transition-colors ${
                      isDone
                        ? "border-signal-500 bg-signal-500 text-white"
                        : isActive
                          ? "border-signal-500 bg-panel text-signal-600"
                          : "border-line bg-panel text-ink-400"
                    }`}
                  >
                    {isDone ? "✓" : i + 1}
                  </div>
                  <span
                    className={`text-[11px] font-medium ${
                      isActive ? "text-signal-600" : isDone ? "text-ink-900" : "text-ink-400"
                    }`}
                  >
                    {stage.label}
                  </span>
                </div>
                {i < STAGES.length - 1 && (
                  <div className="relative mx-2 mb-5 h-0.5 flex-1 overflow-hidden rounded bg-line">
                    {isDone && <div className="h-full bg-signal-500" />}
                    {isActive && <div className="sweep h-full w-1/3 bg-signal-500/60" />}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </Panel>

      <Panel title="Websites" className="rise rise-3">
        <div className="divide-y divide-line/60">
          {audit.sites.map((site) => (
            <SiteProgressRow key={site.website} site={site} maxPages={audit.params.maxPages} />
          ))}
        </div>
      </Panel>
    </div>
  );
}

const SITE_STATUS_LABEL: Record<AuditSite["status"], string> = {
  pending: "Queued",
  crawling: "Crawling",
  checking: "Running checks",
  ai_analysis: "AI analysis",
  completed: "Completed",
  failed: "Failed",
};

function SiteProgressRow({ site, maxPages }: { site: AuditSite; maxPages: number }) {
  const pct = Math.min(100, Math.round((site.pagesCrawled / Math.max(maxPages, 1)) * 100));
  const active = site.status === "crawling" || site.status === "checking" || site.status === "ai_analysis";

  return (
    <div className="flex items-center gap-4 px-4 py-3">
      <span className="w-40 truncate font-mono text-[13px] font-medium">
        {hostOf(site.website)}
      </span>
      <span
        className={`w-28 text-xs ${
          site.status === "failed"
            ? "text-critical"
            : active
              ? "text-signal-600"
              : "text-ink-400"
        }`}
      >
        {active && <span className="pulse-dot mr-1.5 inline-block h-1.5 w-1.5 rounded-full bg-signal-500" />}
        {SITE_STATUS_LABEL[site.status]}
      </span>
      <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-line">
        <div
          className={`h-full rounded-full transition-all duration-500 ${
            site.status === "failed" ? "bg-critical/60" : "bg-signal-500"
          }`}
          style={{ width: `${site.status === "completed" ? 100 : pct}%` }}
        />
      </div>
      <span className="w-24 text-right font-mono text-[12px] text-ink-400">
        {site.pagesCrawled} pages
      </span>
      {site.error && (
        <span className="max-w-[220px] truncate text-[11px] text-critical" title={site.error}>
          {site.error}
        </span>
      )}
    </div>
  );
}

function ResultView({ audit }: { audit: Audit }) {
  const [website, setWebsite] = useState("");
  const [severity, setSeverity] = useState("");
  const [category, setCategory] = useState("");
  const [source, setSource] = useState("");
  const [search, setSearch] = useState("");

  const { data: issuesData, isFetching } = useIssues(audit.id, {
    website: website || undefined,
    severity: severity || undefined,
    category: category || undefined,
    source: source || undefined,
    search: search || undefined,
  });
  // Crawl inventory, used to show provenance (scrape metadata) per issue.
  const { data: pagesData } = usePages(audit.id);
  const pageByUrl = useMemo(() => {
    const map = new Map<string, Page>();
    for (const p of pagesData?.pages ?? []) {
      map.set(p.url, p);
    }
    return map;
  }, [pagesData]);

  const issues = issuesData?.issues ?? [];
  const stats = audit.stats;

  const select = (
    value: string,
    set: (v: string) => void,
    options: { value: string; label: string }[],
    placeholder: string,
  ) => (
    <select
      value={value}
      onChange={(e) => set(e.target.value)}
      className={`rounded-md border border-line bg-panel px-2.5 py-1.5 text-xs focus:border-signal-500 focus:outline-none ${
        value ? "font-medium text-ink-900" : "text-ink-400"
      }`}
    >
      <option value="">{placeholder}</option>
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  );

  return (
    <div className="space-y-4">
      {audit.error && <ErrorNote message={audit.error} />}
      {stats.aiSkipped && (
        <div className="rounded border border-medium/30 bg-medium/5 px-3 py-2 text-xs text-medium">
          AI analysis was skipped — no API key configured. Rule-based findings only.
        </div>
      )}

      {stats.previousAuditId ? <ChangePanel audit={audit} /> : null}

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-8">
        <StatCard className="rise rise-1" label="Websites" value={stats.totalWebsites} />
        <StatCard className="rise rise-1" label="Pages" value={stats.totalPages} />
        <StatCard className="rise rise-2" label="Issues" value={stats.totalIssues} />
        <StatCard className="rise rise-2" label="Duration" value={fmtDuration(stats.durationMs)} />
        <StatCard className="rise rise-3" label="Critical" value={stats.bySeverity.critical ?? 0} tone="critical" />
        <StatCard className="rise rise-3" label="High" value={stats.bySeverity.high ?? 0} tone="high" />
        <StatCard className="rise rise-4" label="Medium" value={stats.bySeverity.medium ?? 0} tone="medium" />
        <StatCard className="rise rise-4" label="Low" value={stats.bySeverity.low ?? 0} tone="low" />
      </div>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <Panel title="By severity" className="rise rise-2">
          <div className="p-2">
            <SeverityDonut data={stats.bySeverity} />
          </div>
        </Panel>
        <Panel title="By category" className="rise rise-3">
          <div className="p-2">
            <CategoryBars data={stats.byCategory} />
          </div>
        </Panel>
        <Panel title="By website" className="rise rise-3">
          <div className="p-2">
            <WebsiteBars data={stats.byWebsite} />
          </div>
        </Panel>
      </div>

      <Panel
        title={`Issues${isFetching ? " · refreshing…" : ""}`}
        className="rise rise-4"
        action={
          <div className="flex flex-wrap items-center gap-2">
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search issues…"
              className="w-44 rounded-md border border-line bg-panel px-2.5 py-1.5 text-xs placeholder:text-ink-400 focus:border-signal-500 focus:outline-none"
            />
            {select(website, setWebsite,
              audit.params.websites.map((w) => ({ value: w, label: hostOf(w) })),
              "All websites")}
            {select(severity, setSeverity,
              ["critical", "high", "medium", "low"].map((s) => ({ value: s, label: s })),
              "All severities")}
            {select(category, setCategory,
              Object.keys(stats.byCategory).map((c) => ({ value: c, label: c })),
              "All categories")}
            {select(source, setSource,
              [{ value: "rule", label: "Rule-based" }, { value: "ai", label: "AI" }],
              "All sources")}
          </div>
        }
      >
        <IssuesTable issues={issues} pageByUrl={pageByUrl} />
      </Panel>

      <Panel title={`Crawled pages — insights (${pagesData?.pages.length ?? 0})`} className="rise rise-4">
        <PagesInsights pages={pagesData?.pages ?? []} />
      </Panel>
    </div>
  );
}

// ChangePanel summarizes what changed since the previous audit of the same
// sites — the payoff of running audits automatically over time.
function ChangePanel({ audit }: { audit: Audit }) {
  const s = audit.stats;
  const resolved = s.resolvedSummary ?? [];
  return (
    <Panel
      title={`Change since audit #${s.previousAuditId}`}
      className="rise rise-1"
      action={
        <Link
          to={`/audits/${s.previousAuditId}`}
          className="text-xs font-medium text-signal-600 hover:underline"
        >
          Compare →
        </Link>
      }
    >
      <div className="flex flex-wrap items-stretch gap-3 p-4">
        <div className="rounded-md border border-high/30 bg-high/5 px-4 py-2.5">
          <div className="microlabel text-high">New issues</div>
          <div className="mt-0.5 font-mono text-2xl font-semibold text-high">+{s.newCount}</div>
          {s.newBySeverity && (
            <div className="mt-1 text-[11px] text-ink-400">
              {(["critical", "high", "medium", "low"] as const)
                .filter((k) => s.newBySeverity?.[k])
                .map((k) => `${s.newBySeverity?.[k]} ${k}`)
                .join(" · ") || "—"}
            </div>
          )}
        </div>
        <div className="rounded-md border border-low/30 bg-low/5 px-4 py-2.5">
          <div className="microlabel text-low">Resolved</div>
          <div className="mt-0.5 font-mono text-2xl font-semibold text-low">−{s.resolvedCount}</div>
          <div className="mt-1 text-[11px] text-ink-400">since previous run</div>
        </div>
        {resolved.length > 0 && (
          <div className="min-w-[240px] flex-1 rounded-md border border-line px-4 py-2.5">
            <div className="microlabel mb-1">Fixed since last audit</div>
            <ul className="space-y-0.5 text-[12px] text-ink-700">
              {resolved.slice(0, 6).map((r, i) => (
                <li key={i} className="truncate">✓ {r}</li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </Panel>
  );
}

// PagesInsights lists every scraped page with its key health metrics.
function PagesInsights({ pages }: { pages: Page[] }) {
  if (pages.length === 0) {
    return <div className="px-4 py-8 text-center text-xs text-ink-400">No pages recorded.</div>;
  }
  const sorted = [...pages].sort((a, b) => a.depth - b.depth || a.url.localeCompare(b.url));
  return (
    <div className="max-h-[420px] overflow-auto">
      <table className="w-full text-[12.5px]">
        <thead className="sticky top-0 bg-panel">
          <tr className="border-b border-line text-left">
            <th className="microlabel px-4 py-2">Page</th>
            <th className="microlabel px-4 py-2">Status</th>
            <th className="microlabel px-4 py-2 text-right">Depth</th>
            <th className="microlabel px-4 py-2 text-right">Response</th>
            <th className="microlabel px-4 py-2 text-right">Load</th>
            <th className="microlabel px-4 py-2 text-right">Size</th>
            <th className="microlabel px-4 py-2">Title</th>
          </tr>
        </thead>
        <tbody>
          {sorted.map((p) => {
            const bad = !!p.fetchError || p.statusCode >= 400;
            const slow = p.responseTimeMs > 1500;
            return (
              <tr key={p.id} className="border-b border-line/60 last:border-0 hover:bg-surface">
                <td className="max-w-[260px] truncate px-4 py-2 font-mono text-[11px]">
                  <a
                    href={p.url}
                    target="_blank"
                    rel="noreferrer"
                    title={p.url}
                    className="text-signal-600 hover:underline"
                  >
                    {pathOf(p.url) || "/"}
                  </a>
                </td>
                <td className={`px-4 py-2 font-mono ${bad ? "font-semibold text-critical" : "text-low"}`}>
                  {p.fetchError ? "ERR" : p.statusCode}
                </td>
                <td className="px-4 py-2 text-right font-mono">{p.depth}</td>
                <td className={`px-4 py-2 text-right font-mono ${slow ? "font-semibold text-medium" : ""}`}>
                  {p.responseTimeMs} ms
                </td>
                <td className="px-4 py-2 text-right font-mono">
                  {p.loadTimeMs ? `${p.loadTimeMs} ms` : "—"}
                </td>
                <td className="px-4 py-2 text-right font-mono">
                  {p.contentLength ? `${Math.round(p.contentLength / 1024)} KB` : "—"}
                </td>
                <td className="max-w-[280px] truncate px-4 py-2 text-ink-700" title={p.title}>
                  {p.title || <span className="text-critical">— missing —</span>}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
