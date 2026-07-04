# REST API Reference

Base URL: `http://localhost:8080` (or the frontend origin — nginx/vite proxy `/api`).
All bodies are JSON (camelCase). Errors return `{"error": "message"}` with an
appropriate 4xx/5xx status.

## Audits

### `POST /api/audits` — create and start an audit

```json
{
  "websites": ["https://ergonix.lt", "https://ergonix.lv"],
  "maxPages": 40,
  "maxDepth": 3,
  "concurrency": 5,
  "requestTimeoutSec": 15,
  "retryCount": 2,
  "useAI": true
}
```

Only `websites` is required; other fields fall back to server defaults and are
clamped to safe bounds (pages ≤ 500, depth ≤ 10, concurrency ≤ 16). Bare domains
are upgraded to `https://`. Responds `201` with the created audit immediately;
the pipeline runs in the background — poll `GET /api/audits/{id}`.

### `GET /api/audits?limit=50&offset=0`

`{"audits": [Audit…], "total": n}` — newest first.

### `GET /api/audits/{id}`

Full audit object:

```json
{
  "id": 1,
  "status": "running",           // pending|running|completed|failed|cancelled
  "stage": "crawling",           // queued|crawling|checking|ai_analysis|reporting|done
  "params": { "websites": ["…"], "maxPages": 40, "…": "…" },
  "sites": [
    {"website": "https://ergonix.lt", "status": "crawling",
     "pagesCrawled": 12, "issueCount": 0, "durationMs": 0}
  ],
  "stats": {
    "totalWebsites": 2, "totalPages": 16, "totalIssues": 137, "durationMs": 5333,
    "bySeverity": {"high": 85, "medium": 44, "low": 8},
    "byCategory": {"Network": 99, "Logic": 22, "SEO": 10, "UI": 6},
    "byWebsite": {"https://ergonix.lt": 70, "https://ergonix.pl": 67},
    "bySource": {"rule": 137}
  },
  "createdAt": "2026-07-03T20:46:36Z",
  "startedAt": "…", "finishedAt": "…"
}
```

### `POST /api/audits/{id}/cancel`

Stops a running audit (`200 {"status":"cancelling"}`; `409` if not running).

### `DELETE /api/audits/{id}`

Removes the audit with its pages and issues (`204`).

## Issues & Pages

### `GET /api/audits/{id}/issues`

Query parameters (all optional, combinable):

| param      | values                                    |
|------------|-------------------------------------------|
| `website`  | exact website URL                          |
| `severity` | `critical` `high` `medium` `low`           |
| `category` | `Translation` `Performance` `Accessibility` `SEO` `Security` `Content` `Logic` `UI` `Network` `JavaScript` |
| `source`   | `rule` `ai`                                |
| `search`   | substring over title/description/URL/fix   |
| `limit`, `offset` | pagination (omit for all)           |

Returns `{"issues": [Issue…], "total": n}` ordered most-severe first.
`checkId` names the producer for provenance: a rule check id (`broken-links`,
`empty-button`, …) or `ai:<type>` for AI findings (`ai:wrong_language`), with
the judging model recorded in `details.model`. Issue shape:

```json
{
  "id": 3, "auditId": 1,
  "website": "https://ergonix.lt",
  "pageUrl": "https://ergonix.lt/",
  "category": "Network", "source": "rule", "checkId": "broken-links", "severity": "high",
  "title": "Broken internal link",
  "description": "Link \"Ergonomics explained\" → https://ergonix.lt/blogs/ergonomics is broken (HTTP 404); referenced from https://ergonix.lt/.",
  "suggestedFix": "Fix the target URL, restore the destination page, or remove the link.",
  "confidence": 1,
  "details": {"target": "…", "reason": "HTTP 404", "internal": true},
  "createdAt": "…"
}
```

### `GET /api/audits/{id}/pages`

`{"pages": [Page…]}` — the crawl inventory (status, title, meta, headings,
links, images, forms, buttons, prices, scripts, timing, redirect chain…).

## Export

### `GET /api/audits/{id}/export/{format}`

`format` ∈ `json` (full report incl. pages) · `csv` (issues, BOM for Excel) ·
`html` (standalone styled report) · `pdf`. Served with a
`Content-Disposition: attachment` filename.

## Meta

### `GET /api/websites`

Registry + defaults for the UI:

```json
{
  "websites": ["https://ergonix.lt", "…"],
  "defaults": {"maxPages": 25, "maxDepth": 3, "concurrency": 4,
               "requestTimeoutSec": 15, "retryCount": 2, "useAI": false},
  "aiEnabled": false, "aiModel": "gpt-4o-mini", "browserEnabled": false,
  "categories": ["Translation", "…"]
}
```

### `GET /api/dashboard`

All-time aggregates: totals, severity/category/website breakdowns,
`issuesOverTime` (one point per finished audit), `recentAudits`.

### `GET /api/settings` / `PUT /api/settings`

Free-form key/value store for UI preferences:
`PUT {"settings": {"defaultMaxPages": "30"}}`.

### `GET /healthz`

`{"status":"ok"}` — liveness probe.
