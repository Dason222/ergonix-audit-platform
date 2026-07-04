package checks

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// stopwords are compact, high-frequency function/commerce words per
// language, used for rule-based language detection (no AI needed).
var stopwords = map[string][]string{
	"lt": {"ir", "yra", "kad", "kaip", "apie", "mūsų", "jūsų", "prekės", "kaina", "krepšelį", "pristatymas", "arba", "daugiau", "visos", "į"},
	"lv": {"un", "ka", "kā", "par", "mūsu", "jūsu", "cena", "grozu", "piegāde", "preces", "vai", "vairāk", "visas", "no"},
	"et": {"ja", "et", "või", "kuidas", "meie", "teie", "hind", "ostukorvi", "tarne", "tooted", "rohkem", "kõik", "ning"},
	"cs": {"je", "že", "ale", "jak", "naše", "vaše", "cena", "košíku", "doprava", "zboží", "nebo", "více", "všechny", "pro"},
	"pl": {"jest", "że", "ale", "jak", "nasze", "cena", "koszyka", "dostawa", "produkty", "się", "nie", "lub", "więcej", "wszystkie"},
	"en": {"the", "and", "is", "of", "to", "for", "with", "your", "our", "price", "cart", "shipping", "all", "more", "from"},
	"de": {"und", "der", "die", "das", "ist", "für", "mit", "ihre", "unsere", "preis", "warenkorb", "versand", "alle", "mehr"},
	"ru": {"и", "в", "на", "что", "для", "наш", "ваш", "цена", "корзину", "доставка", "или", "все", "это"},
}

var wordRe = regexp.MustCompile(`[\p{L}]+`)

// detectLanguage scores text against each stopword list and returns the
// best language with its hit count.
func detectLanguage(text string) (lang string, hits int, scores map[string]int) {
	words := wordRe.FindAllString(strings.ToLower(text), 4000)
	counts := map[string]int{}
	sets := map[string]map[string]bool{}
	for l, list := range stopwords {
		set := make(map[string]bool, len(list))
		for _, w := range list {
			set[w] = true
		}
		sets[l] = set
	}
	for _, w := range words {
		for l, set := range sets {
			if set[w] {
				counts[l]++
			}
		}
	}
	best, bestN := "", 0
	for l, n := range counts {
		if n > bestN {
			best, bestN = l, n
		}
	}
	return best, bestN, counts
}

// WrongLanguageCheck flags pages whose visible text reads in a different
// language than the storefront's country requires — without needing AI.
// Conservative thresholds keep false positives out; the AI layer refines
// further (mixed language, poor translation) when enabled.
type WrongLanguageCheck struct{}

func (WrongLanguageCheck) Name() string { return "wrong-language" }

func (WrongLanguageCheck) CheckPage(sc *SiteContext, p *models.Page) []models.Issue {
	expected := ExpectedLanguage(sc.Website)
	if expected == "" || len(p.VisibleText) < 300 {
		return nil
	}
	detected, hits, scores := detectLanguage(p.VisibleText)
	if detected == "" || detected == expected {
		return nil
	}
	expectedHits := scores[expected]
	// Flag only on a clear verdict: plenty of foreign stopwords AND at
	// least twice as many as the expected language's.
	if hits < 10 || hits < 2*max(expectedHits, 1) {
		return nil
	}
	is := issue(p, models.CategoryTranslation, models.SeverityHigh,
		"Page content in wrong language",
		fmt.Sprintf("The visible text reads as %q (%d stopword hits) but this storefront must serve %q (%d hits). Customers get a page they may not understand.",
			detected, hits, expected, expectedHits),
		"Serve the correct translation for this market.")
	is.Confidence = 0.8
	is.Details = map[string]any{"detected": detected, "expected": expected, "scores": scores}
	return []models.Issue{is}
}
