import { useEffect, useState } from "react";

import { ErrorNote, Panel, Spinner } from "../components/panels";
import { useSaveSettings, useSettings } from "../hooks/useApi";
import type { SettingsUpdate } from "../types/api";
import { hostOf } from "../utils/format";

export default function SettingsPage() {
  const { data, isLoading } = useSettings();
  const save = useSaveSettings();

  // Schedule form state
  const [schedEnabled, setSchedEnabled] = useState(false);
  const [interval, setIntervalH] = useState(24);
  const [schedSites, setSchedSites] = useState<Set<string>>(new Set());

  // AI form state
  const [aiKey, setAiKey] = useState("");
  const [aiModel, setAiModel] = useState("");
  const [aiBaseUrl, setAiBaseUrl] = useState("");

  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (data) {
      setSchedEnabled(data.schedule.enabled);
      setIntervalH(data.schedule.intervalHours || 24);
      setSchedSites(new Set(data.schedule.websites));
      setAiModel(data.ai.model);
      setAiBaseUrl(data.ai.baseUrl);
      setAiKey(""); // never prefill the secret
    }
  }, [data]);

  if (isLoading || !data) return <Spinner label="Loading settings…" />;

  const allSites = data.availableWebsites;
  const allSelected = schedSites.size === 0 || schedSites.size === allSites.length;

  const submit = (update: SettingsUpdate) => {
    setSaved(false);
    save.mutate(update, { onSuccess: () => setSaved(true) });
  };

  const saveSchedule = () => {
    submit({
      scheduleEnabled: schedEnabled,
      scheduleIntervalHours: interval,
      scheduleWebsites: allSelected ? [] : [...schedSites],
    });
  };

  const saveAI = (opts: { clear?: boolean } = {}) => {
    const u: SettingsUpdate = { aiModel, aiBaseUrl };
    if (opts.clear) u.clearAiKey = true;
    else if (aiKey.trim()) u.aiApiKey = aiKey.trim();
    submit(u);
    if (!opts.clear) setAiKey("");
  };

  const toggleSite = (site: string) => {
    setSchedSites((prev) => {
      const next = new Set(prev.size === 0 ? allSites : prev); // start from "all" if empty
      if (next.has(site)) next.delete(site);
      else next.add(site);
      return next;
    });
  };

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      {/* ---- Automatic audits ---- */}
      <Panel
        title="Automatic audits (scheduler)"
        className="rise rise-1"
        action={
          <span
            className={`inline-flex items-center gap-1.5 text-xs font-medium ${
              data.schedule.enabled ? "text-low" : "text-ink-400"
            }`}
          >
            <span
              className={`h-1.5 w-1.5 rounded-full ${
                data.schedule.enabled ? "bg-low" : "bg-ink-400"
              }`}
            />
            {data.schedule.enabled ? "running" : "off"}
          </span>
        }
      >
        <div className="space-y-4 p-4">
          <label className="flex items-start gap-3">
            <input
              type="checkbox"
              checked={schedEnabled}
              onChange={(e) => setSchedEnabled(e.target.checked)}
              className="mt-0.5 h-4 w-4 accent-teal-600"
            />
            <span>
              <span className="block text-[13px] font-medium">Run audits automatically</span>
              <span className="block text-xs text-ink-400">
                The platform will audit the selected sites on its own and compare each run to
                the previous one.
              </span>
            </span>
          </label>

          <label className="block max-w-[220px]">
            <span className="microlabel">Every (hours)</span>
            <input
              type="number"
              min={1}
              max={168}
              value={interval}
              onChange={(e) => setIntervalH(Number(e.target.value))}
              className="mt-1 w-full rounded-md border border-line bg-panel px-3 py-1.5 font-mono text-sm focus:border-signal-500 focus:outline-none"
            />
            <span className="mt-0.5 block text-[10.5px] text-ink-400">1–168 (a week)</span>
          </label>

          <div>
            <span className="microlabel">Websites to audit</span>
            <div className="mt-1.5 grid grid-cols-2 gap-1.5">
              {allSites.map((site) => {
                const on = schedSites.size === 0 || schedSites.has(site);
                return (
                  <label
                    key={site}
                    className={`flex cursor-pointer items-center gap-2 rounded border px-2.5 py-1.5 text-[12px] ${
                      on ? "border-signal-500 bg-signal-500/5" : "border-line"
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={on}
                      onChange={() => toggleSite(site)}
                      className="h-3.5 w-3.5 accent-teal-600"
                    />
                    <span className="font-mono">{hostOf(site)}</span>
                  </label>
                );
              })}
            </div>
            <span className="mt-1 block text-[10.5px] text-ink-400">
              All selected = audit every configured site.
            </span>
          </div>

          <button
            onClick={saveSchedule}
            disabled={save.isPending}
            className="rounded-md bg-ink-900 px-4 py-2 text-xs font-semibold text-white hover:bg-ink-800 disabled:opacity-40"
          >
            {save.isPending ? "Saving…" : "Save schedule"}
          </button>
        </div>
      </Panel>

      {/* ---- AI ---- */}
      <Panel
        title="AI content analysis"
        className="rise rise-2"
        action={
          <span
            className={`inline-flex items-center gap-1.5 text-xs font-medium ${
              data.ai.enabled ? "text-signal-600" : "text-ink-400"
            }`}
          >
            <span
              className={`h-1.5 w-1.5 rounded-full ${data.ai.enabled ? "bg-signal-500" : "bg-ink-400"}`}
            />
            {data.ai.enabled ? "enabled" : "disabled"}
          </span>
        }
      >
        <div className="space-y-4 p-4">
          <label className="block">
            <span className="microlabel">API key</span>
            <input
              type="password"
              autoComplete="off"
              value={aiKey}
              placeholder={
                data.ai.keySet ? `saved (${data.ai.keyPreview}) — type to replace` : "sk-…"
              }
              onChange={(e) => setAiKey(e.target.value)}
              className="mt-1 w-full rounded-md border border-line bg-panel px-3 py-2 font-mono text-sm focus:border-signal-500 focus:outline-none"
            />
            <span className="mt-0.5 block text-[10.5px] text-ink-400">
              Stored server-side; never shown again. Works with OpenAI, Azure, OpenRouter or a
              local Ollama.
            </span>
          </label>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <label className="block">
              <span className="microlabel">Model</span>
              <input
                value={aiModel}
                placeholder="gpt-4o-mini"
                onChange={(e) => setAiModel(e.target.value)}
                className="mt-1 w-full rounded-md border border-line bg-panel px-3 py-1.5 font-mono text-sm focus:border-signal-500 focus:outline-none"
              />
            </label>
            <label className="block">
              <span className="microlabel">Base URL</span>
              <input
                value={aiBaseUrl}
                placeholder="https://api.openai.com/v1"
                onChange={(e) => setAiBaseUrl(e.target.value)}
                className="mt-1 w-full rounded-md border border-line bg-panel px-3 py-1.5 font-mono text-sm focus:border-signal-500 focus:outline-none"
              />
            </label>
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={() => saveAI()}
              disabled={save.isPending}
              className="rounded-md bg-ink-900 px-4 py-2 text-xs font-semibold text-white hover:bg-ink-800 disabled:opacity-40"
            >
              {save.isPending ? "Saving…" : "Save AI settings"}
            </button>
            {data.ai.keySet && (
              <button
                onClick={() => saveAI({ clear: true })}
                disabled={save.isPending}
                className="rounded-md border border-critical/40 px-3 py-2 text-xs font-semibold text-critical hover:bg-critical/5 disabled:opacity-40"
              >
                Remove key
              </button>
            )}
          </div>
        </div>
      </Panel>

      {saved && (
        <div className="rise rise-3 rounded border border-low/30 bg-low/5 px-3 py-2 text-xs font-medium text-low">
          ✓ Saved and applied live — no restart needed.
        </div>
      )}
      {save.error && <ErrorNote message={save.error.message} />}
    </div>
  );
}
