import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
  type SortingState,
} from "@tanstack/react-table";
import { Fragment, useMemo, useState } from "react";

import type { Issue, Page, Severity } from "../types/api";
import { fmtDateTime, hostOf, pathOf } from "../utils/format";
import { ConfidenceMeter, SeverityBadge, SourceBadge } from "./badges";

const SEV_ORDER: Record<Severity, number> = { critical: 0, high: 1, medium: 2, low: 3 };

function UrlLink({ href, tone = "signal" }: { href: string; tone?: "signal" | "critical" }) {
  // URLs come from crawled third-party HTML — only ever link http(s), so a
  // javascript:/data: URI in audit data can't execute on click.
  const safe = /^https?:\/\//i.test(href);
  const cls = `break-all font-mono text-[11px] ${
    tone === "critical" ? "text-critical" : "text-signal-600"
  }`;
  if (!safe) {
    return <span className={cls}>{href}</span>;
  }
  return (
    <a href={href} target="_blank" rel="noreferrer" className={`${cls} hover:underline`}>
      {href}
    </a>
  );
}

// IssueDetails renders the expanded row: full text plus every URL buried in
// the issue's details (broken-link target, duplicate-content page list) as
// clickable links, and the provenance trail (which check produced the issue,
// from which scraped page).
function IssueDetails({ issue, page }: { issue: Issue; page?: Page }) {
  const details = issue.details ?? {};
  const target = typeof details.target === "string" ? details.target : null;
  const affectedPages = Array.isArray(details.pages)
    ? (details.pages as unknown[]).filter((p): p is string => typeof p === "string")
    : [];
  const elements = Array.isArray(details.elements)
    ? (details.elements as unknown[]).filter((e): e is string => typeof e === "string")
    : typeof details.element === "string"
      ? [details.element]
      : [];

  return (
    <div className="grid gap-3 rounded-md border border-line bg-panel p-4 text-[12.5px] md:grid-cols-2">
      <div>
        <div className="microlabel mb-1">Full description</div>
        <p className="leading-relaxed text-ink-700">{issue.description || "—"}</p>
        {target && (
          <>
            <div className="microlabel mb-1 mt-3">Problem link (open to verify)</div>
            <UrlLink href={target} tone="critical" />
          </>
        )}
        {affectedPages.length > 0 && (
          <>
            <div className="microlabel mb-1 mt-3">Affected pages</div>
            <ul className="space-y-0.5">
              {affectedPages.map((p) => (
                <li key={p}>
                  <UrlLink href={p} />
                </li>
              ))}
            </ul>
          </>
        )}
        {elements.length > 0 && (
          <>
            <div className="microlabel mb-1 mt-3">
              Affected elements (find via DevTools / view-source)
            </div>
            <ul className="space-y-0.5">
              {elements.map((e, i) => (
                <li key={i}>
                  <code className="break-all rounded bg-surface px-1.5 py-0.5 font-mono text-[11px] text-ink-700">
                    {e}
                  </code>
                </li>
              ))}
            </ul>
          </>
        )}
      </div>
      <div>
        <div className="microlabel mb-1">Suggested fix</div>
        <p className="leading-relaxed text-ink-700">{issue.suggestedFix || "—"}</p>
        <div className="microlabel mb-1 mt-3">Found on page</div>
        <UrlLink href={issue.pageUrl} />
        <Provenance issue={issue} page={page} />
      </div>
    </div>
  );
}

// Provenance explains where the finding came from: the check (or AI type +
// model) that produced it, and the crawl that supplied the data.
function Provenance({ issue, page }: { issue: Issue; page?: Page }) {
  const model =
    typeof issue.details?.model === "string" ? (issue.details.model as string) : null;
  const producer =
    issue.source === "ai"
      ? `AI analysis${issue.checkId ? ` · ${issue.checkId}` : ""}${model ? ` · model ${model}` : ""}`
      : `rule check${issue.checkId ? ` · ${issue.checkId}` : ""}`;

  return (
    <div className="mt-3 rounded-md border border-line bg-surface px-3 py-2">
      <div className="microlabel mb-1.5">Where this finding came from</div>
      <dl className="grid grid-cols-[92px_1fr] gap-y-1 font-mono text-[11px] text-ink-700">
        <dt className="text-ink-400">detected by</dt>
        <dd>{producer}</dd>
        {page ? (
          <>
            <dt className="text-ink-400">scraped from</dt>
            <dd className="break-all">
              {page.finalUrl && page.finalUrl !== page.url
                ? `${page.url} → ${page.finalUrl}`
                : page.url}
            </dd>
            <dt className="text-ink-400">crawl data</dt>
            <dd>
              HTTP {page.statusCode || "—"} · depth {page.depth} ·{" "}
              {page.responseTimeMs} ms · {fmtDateTime(page.crawledAt)}
            </dd>
          </>
        ) : (
          <>
            <dt className="text-ink-400">scraped from</dt>
            <dd className="break-all">{issue.pageUrl}</dd>
          </>
        )}
      </dl>
    </div>
  );
}

const col = createColumnHelper<Issue>();

export default function IssuesTable({
  issues,
  pageByUrl,
}: {
  issues: Issue[];
  pageByUrl?: Map<string, Page>;
}) {
  const [sorting, setSorting] = useState<SortingState>([{ id: "severity", desc: false }]);
  const [expanded, setExpanded] = useState<number | null>(null);

  const columns = useMemo(
    () => [
      col.accessor("severity", {
        header: "Severity",
        sortingFn: (a, b) =>
          SEV_ORDER[a.original.severity] - SEV_ORDER[b.original.severity],
        cell: (info) => <SeverityBadge severity={info.getValue()} />,
        size: 90,
      }),
      col.accessor("website", {
        header: "Website",
        cell: (info) => (
          <span className="font-mono text-[12px]">{hostOf(info.getValue())}</span>
        ),
        size: 110,
      }),
      col.accessor("category", {
        header: "Category",
        cell: (info) => <span className="text-[12px]">{info.getValue()}</span>,
        size: 100,
      }),
      col.accessor("source", {
        header: "Source",
        cell: (info) => (
          <div>
            <SourceBadge source={info.getValue()} />
            {info.row.original.checkId && (
              <div className="mt-0.5 max-w-[90px] truncate font-mono text-[9.5px] text-ink-400"
                   title={info.row.original.checkId}>
                {info.row.original.checkId}
              </div>
            )}
          </div>
        ),
        size: 100,
      }),
      col.accessor("pageUrl", {
        header: "Page",
        cell: (info) =>
          /^https?:\/\//i.test(info.getValue()) ? (
            <a
              href={info.getValue()}
              target="_blank"
              rel="noreferrer"
              onClick={(e) => e.stopPropagation()}
              title={info.getValue()}
              className="block max-w-[180px] truncate font-mono text-[11px] text-signal-600 hover:underline"
            >
              {pathOf(info.getValue()) || "/"}
            </a>
          ) : (
            <span
              title={info.getValue()}
              className="block max-w-[180px] truncate font-mono text-[11px] text-ink-400"
            >
              {pathOf(info.getValue()) || "/"}
            </span>
          ),
        size: 190,
      }),
      col.accessor("title", {
        header: "Description",
        cell: (info) => (
          <div>
            <div className="text-[13px] font-medium leading-snug">{info.getValue()}</div>
            <div className="line-clamp-1 text-[12px] text-ink-400">
              {info.row.original.description}
            </div>
          </div>
        ),
        size: 380,
      }),
      col.accessor("confidence", {
        header: "Conf.",
        cell: (info) => <ConfidenceMeter value={info.getValue()} />,
        size: 90,
      }),
    ],
    [],
  );

  const table = useReactTable({
    data: issues,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageSize: 25 } },
  });

  if (issues.length === 0) {
    return (
      <div className="px-4 py-12 text-center text-xs text-ink-400">
        No issues match the current filters.
      </div>
    );
  }

  return (
    <div>
      <div className="overflow-x-auto">
        <table className="w-full text-[13px]">
          <thead>
            {table.getHeaderGroups().map((hg) => (
              <tr key={hg.id} className="border-b border-line text-left">
                {hg.headers.map((h) => (
                  <th
                    key={h.id}
                    style={{ width: h.getSize() }}
                    className="microlabel cursor-pointer select-none px-3 py-2 hover:text-ink-900"
                    onClick={h.column.getToggleSortingHandler()}
                  >
                    <span className="inline-flex items-center gap-1">
                      {flexRender(h.column.columnDef.header, h.getContext())}
                      {{ asc: "▲", desc: "▼" }[h.column.getIsSorted() as string] ?? ""}
                    </span>
                  </th>
                ))}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.map((row) => (
              <Fragment key={row.id}>
                <tr
                  onClick={() =>
                    setExpanded(expanded === row.original.id ? null : row.original.id)
                  }
                  className={`cursor-pointer border-b border-line/60 align-top transition-colors hover:bg-surface ${
                    expanded === row.original.id ? "bg-surface" : ""
                  }`}
                >
                  {row.getVisibleCells().map((cell) => (
                    <td key={cell.id} className="px-3 py-2.5">
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </td>
                  ))}
                </tr>
                {expanded === row.original.id && (
                  <tr className="border-b border-line/60 bg-surface">
                    <td colSpan={columns.length} className="px-5 pb-4 pt-1">
                      <IssueDetails
                        issue={row.original}
                        page={pageByUrl?.get(row.original.pageUrl)}
                      />
                    </td>
                  </tr>
                )}
              </Fragment>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between border-t border-line px-4 py-2 text-xs">
        <span className="text-ink-400">
          {issues.length} issue{issues.length === 1 ? "" : "s"} · page{" "}
          {table.getState().pagination.pageIndex + 1} of {Math.max(table.getPageCount(), 1)}
        </span>
        <div className="flex gap-1">
          <button
            onClick={() => table.previousPage()}
            disabled={!table.getCanPreviousPage()}
            className="rounded border border-line px-2.5 py-1 font-medium hover:bg-surface disabled:opacity-40"
          >
            ← Prev
          </button>
          <button
            onClick={() => table.nextPage()}
            disabled={!table.getCanNextPage()}
            className="rounded border border-line px-2.5 py-1 font-medium hover:bg-surface disabled:opacity-40"
          >
            Next →
          </button>
        </div>
      </div>
    </div>
  );
}
