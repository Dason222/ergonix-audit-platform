import {
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";

import { api } from "../api/client";
import type { AuditParams, IssueFilters } from "../types/api";

export function useWebsites() {
  return useQuery({ queryKey: ["websites"], queryFn: api.websites, staleTime: 60_000 });
}

export function useDashboard() {
  return useQuery({ queryKey: ["dashboard"], queryFn: api.dashboard, refetchInterval: 5_000 });
}

export function useAudits() {
  return useQuery({
    queryKey: ["audits"],
    queryFn: () => api.audits(100, 0),
    // Keep history fresh while anything is running.
    refetchInterval: (query) =>
      query.state.data?.audits.some((a) => a.status === "pending" || a.status === "running")
        ? 2_000
        : false,
  });
}

export function useAudit(id: number) {
  return useQuery({
    queryKey: ["audit", id],
    queryFn: () => api.audit(id),
    enabled: Number.isFinite(id) && id > 0,
    // Poll every 2s while the audit is in flight.
    refetchInterval: (query) => {
      const s = query.state.data?.status;
      return s === "pending" || s === "running" ? 2_000 : false;
    },
  });
}

export function useIssues(auditId: number, filters: IssueFilters, enabled = true) {
  return useQuery({
    queryKey: ["issues", auditId, filters],
    queryFn: () => api.issues(auditId, filters),
    enabled: enabled && auditId > 0,
    placeholderData: (prev) => prev,
  });
}

export function usePages(auditId: number, enabled = true) {
  return useQuery({
    queryKey: ["pages", auditId],
    queryFn: () => api.pages(auditId),
    enabled: enabled && auditId > 0,
  });
}

export function useCreateAudit() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (params: Partial<AuditParams>) => api.createAudit(params),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["audits"] });
      qc.invalidateQueries({ queryKey: ["dashboard"] });
    },
  });
}

export function useCancelAudit(auditId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.cancelAudit(auditId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["audit", auditId] }),
  });
}

export function useDeleteAudit() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.deleteAudit(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["audits"] });
      qc.invalidateQueries({ queryKey: ["dashboard"] });
    },
  });
}

export function useSettings() {
  return useQuery({ queryKey: ["settings"], queryFn: api.settings });
}

export function useSaveSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (settings: Record<string, string>) => api.saveSettings(settings),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["settings"] }),
  });
}
