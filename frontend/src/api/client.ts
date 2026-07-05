import type {
  Audit,
  AuditParams,
  DashboardData,
  Issue,
  IssueFilters,
  Page,
  RuntimeSettings,
  SettingsUpdate,
  WebsitesResponse,
} from "../types/api";

// Same-origin by default (vite proxy in dev, nginx proxy in Docker);
// VITE_API_URL overrides for split deployments.
const BASE = import.meta.env.VITE_API_URL ?? "";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const body = (await res.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      /* non-JSON error body */
    }
    throw new ApiError(res.status, message);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  websites: () => request<WebsitesResponse>("/api/websites"),

  dashboard: () => request<DashboardData>("/api/dashboard"),

  audits: (limit = 50, offset = 0) =>
    request<{ audits: Audit[]; total: number }>(
      `/api/audits?limit=${limit}&offset=${offset}`,
    ),

  audit: (id: number) => request<Audit>(`/api/audits/${id}`),

  createAudit: (params: Partial<AuditParams>) =>
    request<Audit>("/api/audits", {
      method: "POST",
      body: JSON.stringify(params),
    }),

  cancelAudit: (id: number) =>
    request<{ status: string }>(`/api/audits/${id}/cancel`, { method: "POST" }),

  deleteAudit: (id: number) =>
    request<void>(`/api/audits/${id}`, { method: "DELETE" }),

  issues: (auditId: number, filters: IssueFilters = {}) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(filters)) {
      if (v) qs.set(k, v);
    }
    const suffix = qs.size ? `?${qs.toString()}` : "";
    return request<{ issues: Issue[]; total: number }>(
      `/api/audits/${auditId}/issues${suffix}`,
    );
  },

  pages: (auditId: number) =>
    request<{ pages: Page[] }>(`/api/audits/${auditId}/pages`),

  settings: () => request<RuntimeSettings>("/api/settings"),

  saveSettings: (update: SettingsUpdate) =>
    request<RuntimeSettings>("/api/settings", {
      method: "PUT",
      body: JSON.stringify(update),
    }),

  exportUrl: (auditId: number, format: "json" | "csv" | "html" | "pdf") =>
    `${BASE}/api/audits/${auditId}/export/${format}`,
};
