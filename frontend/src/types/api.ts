// API types mirroring the Go backend's JSON (camelCase) 1:1.

export type Severity = "critical" | "high" | "medium" | "low";
export type Source = "rule" | "ai";
export type AuditStatus = "pending" | "running" | "completed" | "failed" | "cancelled";

export type Category =
  | "Translation"
  | "Performance"
  | "Accessibility"
  | "SEO"
  | "Security"
  | "Content"
  | "Logic"
  | "UI"
  | "Network"
  | "JavaScript";

export const SEVERITIES: Severity[] = ["critical", "high", "medium", "low"];

export interface AuditParams {
  websites: string[];
  maxPages: number;
  maxDepth: number;
  concurrency: number;
  requestTimeoutSec: number;
  retryCount: number;
  useAI: boolean;
  useBrowser: boolean;
}

export interface AuditSite {
  website: string;
  status: "pending" | "crawling" | "checking" | "ai_analysis" | "completed" | "failed";
  pagesCrawled: number;
  issueCount: number;
  durationMs: number;
  error?: string;
}

export interface AuditStats {
  totalWebsites: number;
  totalPages: number;
  totalIssues: number;
  durationMs: number;
  bySeverity: Partial<Record<Severity, number>>;
  byCategory: Partial<Record<Category, number>>;
  byWebsite: Record<string, number>;
  bySource: Partial<Record<Source, number>>;
  aiSkipped?: boolean;
  notes?: string[];
}

export interface Audit {
  id: number;
  status: AuditStatus;
  stage: string;
  params: AuditParams;
  sites: AuditSite[];
  stats: AuditStats;
  error?: string;
  createdAt: string;
  startedAt?: string;
  finishedAt?: string;
}

export interface Issue {
  id: number;
  auditId: number;
  website: string;
  pageUrl: string;
  category: Category;
  source: Source;
  checkId?: string;
  severity: Severity;
  title: string;
  description: string;
  suggestedFix: string;
  confidence: number;
  screenshot?: string;
  details?: Record<string, unknown>;
  createdAt: string;
}

export interface Page {
  id: number;
  auditId: number;
  website: string;
  url: string;
  finalUrl: string;
  depth: number;
  statusCode: number;
  title: string;
  metaDescription: string;
  language: string;
  responseTimeMs: number;
  loadTimeMs?: number;
  contentLength: number;
  fetchError?: string;
  crawledAt: string;
}

export interface TimePoint {
  auditId: number;
  date: string;
  total: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
}

export interface DashboardData {
  totalAudits: number;
  totalWebsitesAudited: number;
  totalPagesScanned: number;
  totalIssues: number;
  avgAuditDurationMs: number;
  bySeverity: Partial<Record<Severity, number>>;
  byCategory: Partial<Record<Category, number>>;
  byWebsite: Record<string, number>;
  issuesOverTime: TimePoint[];
  recentAudits: Audit[];
}

export interface WebsitesResponse {
  websites: string[];
  defaults: {
    maxPages: number;
    maxDepth: number;
    concurrency: number;
    requestTimeoutSec: number;
    retryCount: number;
    useAI: boolean;
  };
  aiEnabled: boolean;
  aiModel: string;
  browserEnabled: boolean;
  categories: Category[];
}

export interface IssueFilters {
  website?: string;
  severity?: string;
  category?: string;
  source?: string;
  search?: string;
}
