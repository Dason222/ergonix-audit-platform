package checks

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// NoindexCheck flags internally-linked pages that carry a noindex directive:
// customers can reach them but search engines are told to drop them.
type NoindexCheck struct{}

func (NoindexCheck) Name() string { return "noindex" }

func (NoindexCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	if !strings.Contains(p.MetaRobots, "noindex") {
		return nil
	}
	return []models.Issue{issue(p, models.CategorySEO, models.SeverityHigh,
		"Page excluded from search engines (noindex)",
		fmt.Sprintf("The page declares <meta name=\"robots\" content=%q>. It is linked from within the store but search engines are instructed not to index it.", p.MetaRobots),
		"Remove the noindex directive if this page should rank; if the exclusion is intentional, consider not linking it prominently.")}
}

// OGTagsCheck flags pages missing basic Open Graph tags, which control how
// shared links look on social media and messengers.
type OGTagsCheck struct{}

func (OGTagsCheck) Name() string { return "open-graph" }

func (OGTagsCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	has := map[string]bool{}
	for _, prop := range p.OGProperties {
		has[prop] = true
	}
	var missing []string
	for _, want := range []string{"og:title", "og:image"} {
		if !has[want] {
			missing = append(missing, want)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return []models.Issue{issue(p, models.CategorySEO, models.SeverityLow,
		"Missing Open Graph tags",
		fmt.Sprintf("The page lacks %s. Links shared on social media and messengers will render without a proper title/preview image.",
			strings.Join(missing, " and ")),
		"Add the missing og: meta tags to the page head.")}
}

// expectedCurrency maps a storefront TLD to the currency customers must see.
var expectedCurrency = map[string]string{
	".lt": "€", ".lv": "€", ".ee": "€", ".cz": "Kč", ".pl": "zł",
}

// foreignCurrencyTokens lists the tokens that indicate a specific currency.
var currencyTokens = map[string][]string{
	"€":  {"€", "EUR"},
	"Kč": {"Kč", "CZK"},
	"zł": {"zł", "PLN"},
}

// CurrencyCheck flags prices displayed in the wrong currency for the
// storefront's country (e.g. zł on ergonix.lt) — a classic multi-country
// copy/paste or misconfiguration bug.
type CurrencyCheck struct{}

func (CurrencyCheck) Name() string { return "wrong-currency" }

func (CurrencyCheck) CheckPage(sc *SiteContext, p *models.Page) []models.Issue {
	host := sc.Website
	if u, err := url.Parse(sc.Website); err == nil && u.Host != "" {
		host = strings.ToLower(strings.TrimPrefix(u.Host, "www."))
	}
	var expected string
	for tld, cur := range expectedCurrency {
		if strings.HasSuffix(host, tld) {
			expected = cur
			break
		}
	}
	if expected == "" || len(p.Prices) == 0 {
		return nil
	}

	var offending []string
	for cur, tokens := range currencyTokens {
		if cur == expected {
			continue
		}
		for _, price := range p.Prices {
			for _, tok := range tokens {
				if strings.Contains(price, tok) {
					offending = append(offending, price)
				}
			}
		}
	}
	if len(offending) == 0 {
		return nil
	}
	offending = capStrings(uniqueStr(offending), 5)
	is := issue(p, models.CategoryLogic, models.SeverityHigh,
		"Price in wrong currency for this country",
		fmt.Sprintf("This %s storefront should display prices in %s, but the page shows: %s.",
			host, expected, strings.Join(offending, "; ")),
		"Fix the price display / market configuration so customers see their local currency.")
	is.Confidence = 0.85 // legal fine print may legitimately mention other currencies
	is.Details = map[string]any{"expectedCurrency": expected, "prices": offending}
	return []models.Issue{is}
}

// zeroPriceRe matches a standalone 0,00 / 0.00 amount (not 10,00 etc.).
var zeroPriceRe = regexp.MustCompile(`(^|[^\d.,])0[.,]0{1,2}([^\d]|$)`)

// ZeroPriceCheck flags products displayed at 0.00 — almost always a broken
// price feed, and it invites checkout chaos.
type ZeroPriceCheck struct{}

func (ZeroPriceCheck) Name() string { return "zero-price" }

// cartContextWords mark places where a zero amount is legitimate: the empty
// cart / order-summary widgets rendered into every page (all 5 locales).
var cartContextWords = []string{
	"tarpinė suma", "iš viso", "krepšel", "cart", "subtotal", "total",
	"kopā", "grozs", "celkem", "mezisouč", "košík", "razem", "koszyk",
	"kokku", "vahesumma", "ostukorv", "suma",
}

// zeroPriceLegit reports whether every occurrence of price in the page text
// sits in cart/summary context (empty-cart subtotals are not price bugs).
func zeroPriceLegit(text, price string) bool {
	found := false
	for idx := strings.Index(text, price); idx >= 0; {
		found = true
		start := idx - 60
		if start < 0 {
			start = 0
		}
		ctx := strings.ToLower(text[start:idx])
		inCart := false
		for _, w := range cartContextWords {
			if strings.Contains(ctx, w) {
				inCart = true
				break
			}
		}
		if !inCart {
			return false // at least one occurrence looks like a real price
		}
		next := strings.Index(text[idx+1:], price)
		if next < 0 {
			break
		}
		idx = idx + 1 + next
	}
	return found // all occurrences were cart context (or price not in text → false)
}

func (ZeroPriceCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	var offending []string
	for _, price := range p.Prices {
		if zeroPriceRe.MatchString(price) && !zeroPriceLegit(p.VisibleText, price) {
			offending = append(offending, price)
		}
	}
	if len(offending) == 0 {
		return nil
	}
	is := issue(p, models.CategoryLogic, models.SeverityHigh,
		"Product price displayed as zero",
		fmt.Sprintf("The page shows zero amounts: %s. This is almost always a broken price feed or misconfigured market.",
			strings.Join(capStrings(uniqueStr(offending), 5), "; ")),
		"Fix the product's price configuration for this market.")
	is.Confidence = 0.9 // "free shipping from 0,00" style strings are rare but possible
	is.Details = map[string]any{"prices": offending}
	return []models.Issue{is}
}

func uniqueStr(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// HreflangCheck (site-level): a multi-country store network should declare
// hreflang alternates so search engines send users to the right country site.
type HreflangCheck struct{}

func (HreflangCheck) Name() string { return "hreflang" }

func (HreflangCheck) CheckSite(sc *SiteContext) []models.Issue {
	total, withHreflang := 0, 0
	for _, p := range sc.Pages {
		if !p.OK() || !p.IsHTML() {
			continue
		}
		total++
		if len(p.Hreflangs) > 0 {
			withHreflang++
		}
	}
	if total == 0 || withHreflang > 0 {
		// Partial coverage is a refinement, not a defect worth alarming on.
		return nil
	}
	return []models.Issue{{
		PageURL:  sc.Website,
		Category: models.CategorySEO,
		Severity: models.SeverityMedium,
		Title:    "No hreflang alternates between country stores",
		Description: fmt.Sprintf(
			"None of the %d crawled pages declare <link rel=\"alternate\" hreflang> tags. With five country storefronts (.lt/.lv/.ee/.cz/.pl), search engines may rank the wrong country's site for a market or treat the stores as duplicate content.",
			total),
		SuggestedFix: "Add hreflang link tags on every page referencing the equivalent URL on each sibling storefront (and x-default).",
	}}
}
