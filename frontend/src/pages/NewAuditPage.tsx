import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";

import { ErrorNote, Panel, Spinner } from "../components/panels";
import { useCreateAudit, useWebsites } from "../hooks/useApi";
import { hostOf } from "../utils/format";

export default function NewAuditPage() {
  const navigate = useNavigate();
  const { data: meta, isLoading } = useWebsites();
  const create = useCreateAudit();

  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [maxPages, setMaxPages] = useState(25);
  const [maxDepth, setMaxDepth] = useState(3);
  const [concurrency, setConcurrency] = useState(4);
  const [timeoutSec, setTimeoutSec] = useState(15);
  const [retryCount, setRetryCount] = useState(2);
  const [useAI, setUseAI] = useState(true);
  const [showAdvanced, setShowAdvanced] = useState(false);

  useEffect(() => {
    if (meta) {
      setMaxPages(meta.defaults.maxPages);
      setMaxDepth(meta.defaults.maxDepth);
      setConcurrency(meta.defaults.concurrency);
      setTimeoutSec(meta.defaults.requestTimeoutSec);
      setRetryCount(meta.defaults.retryCount);
      setUseAI(meta.aiEnabled);
    }
  }, [meta]);

  if (isLoading || !meta) return <Spinner label="Loading configuration…" />;

  const toggle = (site: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(site)) next.delete(site);
      else next.add(site);
      return next;
    });
  };

  const launch = async (websites: string[]) => {
    const audit = await create.mutateAsync({
      websites,
      maxPages,
      maxDepth,
      concurrency,
      requestTimeoutSec: timeoutSec,
      retryCount,
      useAI,
    });
    navigate(`/audits/${audit.id}`);
  };

  const numField = (
    label: string,
    value: number,
    set: (n: number) => void,
    min: number,
    max: number,
    hint: string,
  ) => (
    <label className="block">
      <span className="microlabel">{label}</span>
      <input
        type="number"
        value={value}
        min={min}
        max={max}
        onChange={(e) => set(Number(e.target.value))}
        className="mt-1 w-full rounded-md border border-line bg-panel px-3 py-1.5 font-mono text-sm focus:border-signal-500 focus:outline-none"
      />
      <span className="mt-0.5 block text-[10.5px] text-ink-400">{hint}</span>
    </label>
  );

  return (
    <div className="mx-auto max-w-3xl space-y-4">
      <Panel title="Select websites" className="rise rise-1">
        <div className="grid grid-cols-1 gap-2 p-4 sm:grid-cols-2">
          {meta.websites.map((site) => {
            const checked = selected.has(site);
            const tld = hostOf(site).split(".").pop()?.toUpperCase();
            return (
              <label
                key={site}
                className={`flex cursor-pointer items-center gap-3 rounded-md border px-3.5 py-3 transition-colors ${
                  checked
                    ? "border-signal-500 bg-signal-500/5 shadow-[inset_0_0_0_1px_#14b8a6]"
                    : "border-line hover:border-ink-400/40"
                }`}
              >
                <input
                  type="checkbox"
                  checked={checked}
                  onChange={() => toggle(site)}
                  className="h-4 w-4 accent-teal-600"
                />
                <span className="flex-1">
                  <span className="block font-mono text-[13px] font-medium">
                    {hostOf(site)}
                  </span>
                  <span className="block text-[11px] text-ink-400">{site}</span>
                </span>
                <span className="rounded bg-ink-900 px-1.5 py-0.5 font-mono text-[10px] font-semibold text-signal-300">
                  {tld}
                </span>
              </label>
            );
          })}
        </div>
        <div className="flex items-center justify-between border-t border-line px-4 py-2">
          <span className="text-xs text-ink-400">
            {selected.size} of {meta.websites.length} selected
          </span>
          <div className="flex gap-2">
            <button
              onClick={() => setSelected(new Set(meta.websites))}
              className="text-xs font-medium text-signal-600 hover:underline"
            >
              Select all
            </button>
            <span className="text-line">·</span>
            <button
              onClick={() => setSelected(new Set())}
              className="text-xs font-medium text-ink-400 hover:underline"
            >
              Clear
            </button>
          </div>
        </div>
      </Panel>

      <Panel title="Analysis options" className="rise rise-2">
        <div className="space-y-3 p-4">
          <label className="flex items-start gap-3">
            <input
              type="checkbox"
              checked={useAI}
              onChange={(e) => setUseAI(e.target.checked)}
              disabled={!meta.aiEnabled}
              className="mt-0.5 h-4 w-4 accent-teal-600"
            />
            <span>
              <span className="block text-[13px] font-medium">
                AI content analysis
                {!meta.aiEnabled && (
                  <span className="ml-2 rounded bg-medium/10 px-1.5 py-0.5 text-[10px] font-semibold text-medium">
                    NO API KEY CONFIGURED
                  </span>
                )}
              </span>
              <span className="block text-xs text-ink-400">
                Language, translation quality, pricing sanity, missing buyer info —
                analysed by {meta.aiModel || "LLM"} on extracted content.
              </span>
            </span>
          </label>

          <button
            onClick={() => setShowAdvanced((v) => !v)}
            className="text-xs font-medium text-signal-600"
          >
            {showAdvanced ? "▾ Hide" : "▸ Show"} crawler settings
          </button>

          {showAdvanced && (
            <div className="grid grid-cols-2 gap-3 rounded-md border border-line bg-surface p-3 sm:grid-cols-3">
              {numField("Max pages / site", maxPages, setMaxPages, 1, 500, "1–500")}
              {numField("Max depth", maxDepth, setMaxDepth, 1, 10, "link levels from home")}
              {numField("Concurrency", concurrency, setConcurrency, 1, 16, "parallel requests")}
              {numField("Timeout (s)", timeoutSec, setTimeoutSec, 1, 120, "per request")}
              {numField("Retries", retryCount, setRetryCount, 0, 5, "on failure")}
            </div>
          )}
        </div>
      </Panel>

      {create.error && <ErrorNote message={create.error.message} />}

      <div className="rise rise-3 flex items-center gap-3">
        <button
          onClick={() => launch([...selected])}
          disabled={selected.size === 0 || create.isPending}
          className="rounded-md bg-ink-900 px-5 py-2.5 text-sm font-semibold text-white transition-colors hover:bg-ink-800 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {create.isPending ? "Starting…" : `Audit Selected (${selected.size})`}
        </button>
        <button
          onClick={() => launch(meta.websites)}
          disabled={create.isPending}
          className="rounded-md border border-ink-900 px-5 py-2.5 text-sm font-semibold text-ink-900 transition-colors hover:bg-ink-900 hover:text-white disabled:opacity-40"
        >
          Audit All Websites
        </button>
      </div>
    </div>
  );
}
