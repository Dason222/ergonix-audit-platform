# How AI Is Used

The platform uses an LLM for exactly one job: judging **content quality** —
the class of problems rule-based checks cannot see. Everything mechanical
(broken links, status codes, missing tags, sizes, timings, redirects) is
handled by deterministic rules; the AI never duplicates them.

## Principles

1. **Never send raw HTML.** The crawler's extraction step reduces each page to
   a compact structured summary (`internal/ai/extract.go`):

   ```json
   {
     "url": "https://ergonix.lt/products/kede",
     "declaredLanguage": "lt",
     "title": "Ergonominė biuro kėdė | Ergonix",
     "metaDescription": "…",
     "h1": ["Ergonominė biuro kėdė"],
     "h2": ["Aprašymas", "Pristatymas"],
     "buttons": ["Į krepšelį", "Pirkti dabar"],
     "formActions": ["/cart/add"],
     "prices": ["129,99 €"],
     "linkTexts": ["Kėdės", "Stalai", "Kontaktai"],
     "visibleText": "…first ~4000 chars of visible text…"
   }
   ```

   The visible-text budget (`AI_MAX_TEXT_CHARS`, default 4000) keeps token
   cost flat per page regardless of page size.

2. **Page selection, not page flooding.** Pages are ranked by a content
   *richness* score (text volume + prices + headings + forms; homepage always
   included) and only the top `AI_MAX_PAGES_PER_SITE` (default 8) per site are
   analysed. A 200-page crawl still costs ≤ 8 LLM calls per site.

3. **Country-aware prompting.** The system prompt pins the expected language
   per storefront — `.lt`→Lithuanian, `.lv`→Latvian, `.ee`→Estonian,
   `.cz`→Czech, `.pl`→Polish — and instructs the model to treat user-facing
   text in any other language as a defect (brand/model names excepted).

4. **Strict JSON out.** The request uses `response_format: json_object` with
   temperature 0.1. The response parser (`internal/ai/parse.go`) tolerates
   code fences and wrapper text, validates every finding against a whitelist
   of 16 type ids, maps them to issue categories, clamps severity/confidence,
   and silently drops anything malformed.

5. **Failure is never fatal.** A failed page is skipped; a site whose AI pass
   fails entirely still ships its rule-based findings, with a note recorded on
   the audit (`stats.aiSkipped` / site error). No API key ⇒ AI stage is
   skipped cleanly.

## What the AI checks (16 dimensions)

| type id | issue category | catches |
|---|---|---|
| `wrong_language` | Translation | Estonian page serving English copy |
| `mixed_language` | Translation | LT description with EN buttons |
| `poor_translation` | Translation | machine-translation artifacts |
| `missing_translation` | Translation | untranslated UI strings |
| `placeholder` | Content | lorem ipsum, `{{var}}`, "translation missing" |
| `unnatural_wording` | Translation | grammatical but awkward phrasing |
| `grammar` | Translation | spelling/grammar errors |
| `confusing_description` | Content | contradictory or nonsensical product copy |
| `missing_buyer_info` | Content | no payment/returns signals |
| `missing_shipping_info` | Content | no delivery info where expected |
| `missing_warranty_info` | Content | no warranty info where expected |
| `suspicious_pricing` | Logic | 0.00 prices, conflicting prices, wrong currency |
| `ux_writing` | UI | unclear CTAs, inconsistent naming |
| `inconsistency` | Content | title vs description contradictions |
| `country_mismatch` | Logic | wrong currency/locale/country references |
| `trust` | Content | anything eroding customer trust |

Each finding returns `severity`, `title`, `description` (quoting the offending
text), `suggestedFix`, and `confidence` (0–1), which the UI renders as a meter
and the exports include verbatim.

## Provider flexibility

The client (`internal/ai/client.go`) speaks the OpenAI chat-completions
protocol against any `OPENAI_BASE_URL` — OpenAI, Azure OpenAI, OpenRouter, or
a local Ollama (`http://localhost:11434/v1`). Model, key, timeout and page
budgets are all `.env` knobs; one retry on 429/5xx/transport errors.
