package crawler

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// robotsRules holds the Allow/Disallow rules that apply to our user agent.
type robotsRules struct {
	allows    []string
	disallows []string
}

// fetchRobots downloads and parses robots.txt for the site. A missing or
// unreadable robots.txt yields permissive rules (allow everything).
func fetchRobots(ctx context.Context, client *http.Client, root *url.URL, userAgent string) *robotsRules {
	robotsURL := root.Scheme + "://" + root.Host + "/robots.txt"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return &robotsRules{}
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return &robotsRules{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &robotsRules{}
	}
	return parseRobots(io.LimitReader(resp.Body, 512*1024), userAgent)
}

// parseRobots extracts the rule group matching userAgent (longest UA prefix
// wins; "*" is the fallback group).
func parseRobots(r io.Reader, userAgent string) *robotsRules {
	uaToken := strings.ToLower(strings.Split(userAgent, "/")[0]) // "ErgonixAuditBot/1.0" -> "ergonixauditbot"

	type group struct {
		agents []string
		rules  robotsRules
	}
	var (
		groups  []*group
		current *group
		lastWasUA bool
	)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)

		switch key {
		case "user-agent":
			if current == nil || !lastWasUA {
				current = &group{}
				groups = append(groups, current)
			}
			current.agents = append(current.agents, strings.ToLower(val))
			lastWasUA = true
		case "allow":
			if current != nil && val != "" {
				current.rules.allows = append(current.rules.allows, val)
			}
			lastWasUA = false
		case "disallow":
			if current != nil && val != "" {
				current.rules.disallows = append(current.rules.disallows, val)
			}
			lastWasUA = false
		default:
			lastWasUA = false
		}
	}

	var (
		best    *robotsRules
		bestLen = -1
	)
	for _, g := range groups {
		for _, agent := range g.agents {
			if agent == "*" && bestLen < 0 {
				r := g.rules
				best = &r
			} else if agent != "*" && strings.Contains(uaToken, agent) && len(agent) > bestLen {
				r := g.rules
				best, bestLen = &r, len(agent)
			}
		}
	}
	if best == nil {
		return &robotsRules{}
	}
	return best
}

// Allowed applies longest-match-wins semantics between Allow and Disallow.
func (r *robotsRules) Allowed(u string) bool {
	parsed, err := url.Parse(u)
	if err != nil {
		return true
	}
	path := parsed.Path
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}

	matchLen := func(pattern string) int {
		if robotsMatch(pattern, path) {
			return len(pattern)
		}
		return -1
	}

	bestAllow, bestDisallow := -1, -1
	for _, p := range r.allows {
		if l := matchLen(p); l > bestAllow {
			bestAllow = l
		}
	}
	for _, p := range r.disallows {
		if l := matchLen(p); l > bestDisallow {
			bestDisallow = l
		}
	}
	return bestDisallow < 0 || bestAllow >= bestDisallow
}

// robotsMatch implements the subset of robots.txt pattern matching that
// matters in practice: '*' wildcards and a '$' end anchor.
func robotsMatch(pattern, path string) bool {
	anchored := strings.HasSuffix(pattern, "$")
	if anchored {
		pattern = strings.TrimSuffix(pattern, "$")
	}
	parts := strings.Split(pattern, "*")

	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(path[pos:], part)
		if idx < 0 {
			return false
		}
		if i == 0 && idx != 0 {
			return false // first literal must match at the start
		}
		pos += idx + len(part)
	}
	if anchored {
		return pos == len(path)
	}
	return true
}
