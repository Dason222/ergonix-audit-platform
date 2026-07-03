package ai

import (
	"fmt"
	"strings"
)

// languageNames maps expected language codes to human-readable names used in
// the prompt. Keep in sync with checks.ExpectedLanguage.
var languageNames = map[string]string{
	"lt": "Lithuanian",
	"lv": "Latvian",
	"et": "Estonian",
	"cs": "Czech",
	"pl": "Polish",
}

// systemPrompt instructs the model to behave as a strict JSON-emitting
// content auditor for one storefront.
func systemPrompt(website, expectedLang string) string {
	langName := languageNames[expectedLang]
	langClause := "The site's expected primary language is unknown; infer it from the domain and content."
	if langName != "" {
		langClause = fmt.Sprintf(
			"This storefront (%s) MUST serve content in %s (language code %q). "+
				"Any user-facing text in another language (except brand names, product model names, and universally accepted terms) is a defect.",
			website, langName, expectedLang)
	}

	return strings.TrimSpace(fmt.Sprintf(`
You are a meticulous e-commerce content auditor for the Ergonix office-furniture store network.
You receive structured JSON extracted from ONE web page (never raw HTML) and report content problems.

%s

Analyse the page for these problem types:
1.  wrong_language        - page content is in the wrong language for this storefront
2.  mixed_language        - multiple languages mixed in user-facing text
3.  poor_translation      - text reads like low-quality machine translation
4.  missing_translation   - some UI strings/section remain untranslated
5.  placeholder           - placeholder text visible (lorem ipsum, TODO, {{variable}}, "translation missing")
6.  unnatural_wording     - grammatically valid but awkward, unnatural phrasing
7.  grammar               - spelling or grammar mistakes
8.  confusing_description - product description is unclear, contradictory, or nonsensical
9.  missing_buyer_info    - no/insufficient purchase-critical info (payment, returns policy signals)
10. missing_shipping_info - shipping/delivery information absent where expected
11. missing_warranty_info - warranty information absent where a buyer would expect it
12. suspicious_pricing    - pricing text looks wrong (0.00, absurd values, conflicting prices, wrong currency for the country)
13. ux_writing            - button/label/microcopy problems (unclear CTAs, inconsistent naming)
14. inconsistency         - internal contradictions (title vs description vs headings)
15. country_mismatch      - country-specific mistakes (wrong currency symbol, wrong locale formats, references to another country)
16. trust                 - anything that would make a customer distrust the shop

Rules:
- Judge ONLY what is in the provided JSON. Do not invent facts. If the extract is too thin to judge a dimension, stay silent on it.
- Absence of shipping/warranty details on a clearly non-product page (blog, policy page) is NOT an issue.
- Be conservative: report a finding only when you are reasonably confident a native-speaking customer would notice the problem.

Respond with STRICT JSON only — no markdown fences, no commentary — exactly this shape:
{"issues":[{"type":"<one of the 16 type ids>","severity":"critical|high|medium|low","title":"<short summary, max 90 chars>","description":"<what is wrong, quoting the offending text>","suggestedFix":"<concrete fix>","confidence":<0.0-1.0>}]}

If the page has no problems, respond {"issues":[]}.
`, langClause))
}

// userPrompt wraps the page summary JSON.
func userPrompt(summaryJSON string) string {
	return "Audit this page extract:\n" + summaryJSON
}
