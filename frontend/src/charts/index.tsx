import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import type { Category, Severity, TimePoint } from "../types/api";
import { SEVERITIES } from "../types/api";
import { hostOf } from "../utils/format";

export const SEV_COLORS: Record<Severity, string> = {
  critical: "#dc2626",
  high: "#ea580c",
  medium: "#d97706",
  low: "#65a30d",
};

const AXIS = { fontSize: 11, fill: "#64748b", fontFamily: "IBM Plex Mono, monospace" };
const GRID = "#eceef2";
const TOOLTIP_STYLE = {
  fontSize: 12,
  fontFamily: "IBM Plex Sans, sans-serif",
  border: "1px solid #e4e7ec",
  borderRadius: 8,
  boxShadow: "0 4px 12px rgba(16,24,38,0.08)",
};

export function SeverityDonut({ data }: { data: Partial<Record<Severity, number>> }) {
  const rows = SEVERITIES.map((s) => ({ name: s, value: data[s] ?? 0 })).filter(
    (r) => r.value > 0,
  );
  if (rows.length === 0) return <ChartEmpty />;
  return (
    <ResponsiveContainer width="100%" height={220}>
      <PieChart>
        <Pie
          data={rows}
          dataKey="value"
          nameKey="name"
          innerRadius={55}
          outerRadius={85}
          paddingAngle={2}
          strokeWidth={0}
        >
          {rows.map((r) => (
            <Cell key={r.name} fill={SEV_COLORS[r.name as Severity]} />
          ))}
        </Pie>
        <Tooltip contentStyle={TOOLTIP_STYLE} />
        <Legend
          formatter={(v) => <span style={{ fontSize: 11, color: "#334155" }}>{v}</span>}
        />
      </PieChart>
    </ResponsiveContainer>
  );
}

export function CategoryBars({ data }: { data: Partial<Record<Category, number>> }) {
  const rows = Object.entries(data)
    .map(([name, value]) => ({ name, value: value ?? 0 }))
    .filter((r) => r.value > 0)
    .sort((a, b) => b.value - a.value);
  if (rows.length === 0) return <ChartEmpty />;
  return (
    <ResponsiveContainer width="100%" height={Math.max(180, rows.length * 30 + 40)}>
      <BarChart data={rows} layout="vertical" margin={{ left: 24, right: 16 }}>
        <CartesianGrid horizontal={false} stroke={GRID} />
        <XAxis type="number" tick={AXIS} allowDecimals={false} />
        <YAxis type="category" dataKey="name" tick={AXIS} width={88} />
        <Tooltip contentStyle={TOOLTIP_STYLE} cursor={{ fill: "rgba(20,184,166,0.06)" }} />
        <Bar dataKey="value" fill="#14b8a6" radius={[0, 3, 3, 0]} barSize={14} />
      </BarChart>
    </ResponsiveContainer>
  );
}

export function WebsiteBars({ data }: { data: Record<string, number> }) {
  const rows = Object.entries(data)
    .map(([name, value]) => ({ name: hostOf(name), value }))
    .sort((a, b) => b.value - a.value);
  if (rows.length === 0) return <ChartEmpty />;
  return (
    <ResponsiveContainer width="100%" height={220}>
      <BarChart data={rows} margin={{ left: 0, right: 16 }}>
        <CartesianGrid vertical={false} stroke={GRID} />
        <XAxis dataKey="name" tick={AXIS} interval={0} angle={-14} textAnchor="end" height={48} />
        <YAxis tick={AXIS} allowDecimals={false} width={34} />
        <Tooltip contentStyle={TOOLTIP_STYLE} cursor={{ fill: "rgba(20,184,166,0.06)" }} />
        <Bar dataKey="value" fill="#101826" radius={[3, 3, 0, 0]} barSize={26} />
      </BarChart>
    </ResponsiveContainer>
  );
}

export function IssuesOverTime({ data }: { data: TimePoint[] }) {
  if (data.length === 0) return <ChartEmpty />;
  const rows = data.map((p) => ({
    ...p,
    label: `#${p.auditId}`,
  }));
  return (
    <ResponsiveContainer width="100%" height={220}>
      <AreaChart data={rows} margin={{ left: 0, right: 16, top: 8 }}>
        <defs>
          <linearGradient id="fillTotal" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="#14b8a6" stopOpacity={0.25} />
            <stop offset="100%" stopColor="#14b8a6" stopOpacity={0.02} />
          </linearGradient>
        </defs>
        <CartesianGrid vertical={false} stroke={GRID} />
        <XAxis dataKey="label" tick={AXIS} />
        <YAxis tick={AXIS} allowDecimals={false} width={34} />
        <Tooltip contentStyle={TOOLTIP_STYLE} />
        <Area
          type="monotone"
          dataKey="total"
          stroke="#0d9488"
          strokeWidth={2}
          fill="url(#fillTotal)"
          name="Total issues"
        />
        <Area
          type="monotone"
          dataKey="critical"
          stroke="#dc2626"
          strokeWidth={1.5}
          fill="none"
          name="Critical"
        />
      </AreaChart>
    </ResponsiveContainer>
  );
}

function ChartEmpty() {
  return (
    <div className="flex h-[220px] items-center justify-center text-xs text-ink-400">
      No data yet
    </div>
  );
}
