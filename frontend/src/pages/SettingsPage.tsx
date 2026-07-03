import { useEffect, useState } from "react";

import { ErrorNote, Panel, Spinner } from "../components/panels";
import { useSaveSettings, useSettings, useWebsites } from "../hooks/useApi";

// Settings persisted server-side (informational defaults for the UI).
const FIELDS: { key: string; label: string; hint: string; placeholder: string }[] = [
  { key: "defaultMaxPages", label: "Default max pages", hint: "Prefilled in the audit form", placeholder: "25" },
  { key: "defaultMaxDepth", label: "Default max depth", hint: "Link levels from the homepage", placeholder: "3" },
  { key: "notifyEmail", label: "Report email", hint: "Where exported reports get sent (future)", placeholder: "qa@ergonix.example" },
];

export default function SettingsPage() {
  const { data, isLoading } = useSettings();
  const { data: meta } = useWebsites();
  const save = useSaveSettings();
  const [values, setValues] = useState<Record<string, string>>({});
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (data) setValues(data.settings);
  }, [data]);

  if (isLoading) return <Spinner label="Loading settings…" />;

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <Panel title="Workspace defaults" className="rise rise-1">
        <div className="space-y-4 p-4">
          {FIELDS.map((f) => (
            <label key={f.key} className="block">
              <span className="microlabel">{f.label}</span>
              <input
                value={values[f.key] ?? ""}
                placeholder={f.placeholder}
                onChange={(e) => {
                  setSaved(false);
                  setValues((v) => ({ ...v, [f.key]: e.target.value }));
                }}
                className="mt-1 w-full rounded-md border border-line bg-panel px-3 py-2 text-sm focus:border-signal-500 focus:outline-none"
              />
              <span className="mt-0.5 block text-[11px] text-ink-400">{f.hint}</span>
            </label>
          ))}
          <div className="flex items-center gap-3">
            <button
              onClick={() => save.mutate(values, { onSuccess: () => setSaved(true) })}
              disabled={save.isPending}
              className="rounded-md bg-ink-900 px-4 py-2 text-xs font-semibold text-white hover:bg-ink-800 disabled:opacity-40"
            >
              {save.isPending ? "Saving…" : "Save settings"}
            </button>
            {saved && <span className="text-xs font-medium text-low">✓ Saved</span>}
          </div>
          {save.error && <ErrorNote message={save.error.message} />}
        </div>
      </Panel>

      <Panel title="Runtime configuration (read-only)" className="rise rise-2">
        <div className="p-4">
          <p className="mb-3 text-xs text-ink-400">
            These values come from the backend environment (.env). Restart the backend to
            change them.
          </p>
          <dl className="grid grid-cols-2 gap-x-6 gap-y-2 font-mono text-[12px]">
            <dt className="text-ink-400">AI analysis</dt>
            <dd>{meta?.aiEnabled ? `enabled (${meta.aiModel})` : "disabled — no API key"}</dd>
            <dt className="text-ink-400">Browser checks</dt>
            <dd>{meta?.browserEnabled ? "enabled (Playwright)" : "disabled (HTTP-only)"}</dd>
            <dt className="text-ink-400">Registered websites</dt>
            <dd>{meta?.websites.length ?? "—"}</dd>
            <dt className="text-ink-400">Default concurrency</dt>
            <dd>{meta?.defaults.concurrency ?? "—"}</dd>
            <dt className="text-ink-400">Default timeout</dt>
            <dd>{meta ? `${meta.defaults.requestTimeoutSec}s` : "—"}</dd>
            <dt className="text-ink-400">Default retries</dt>
            <dd>{meta?.defaults.retryCount ?? "—"}</dd>
          </dl>
        </div>
      </Panel>
    </div>
  );
}
