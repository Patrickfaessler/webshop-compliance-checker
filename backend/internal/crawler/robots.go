package crawler

import (
	"strings"
)

// Robots is a minimal robots.txt matcher: it uses the group for our product token if present,
// else "*", and applies the longest matching Allow/Disallow path prefix (Allow wins ties).
// Spec: specs/05-crawler.md (C-1: obey robots.txt).
type Robots struct {
	rules []robotsRule
}

type robotsRule struct {
	allow bool
	path  string
}

// ParseRobots parses robots.txt content for the given product token (e.g. "ComplianceCheckerBot").
func ParseRobots(content, token string) *Robots {
	token = strings.ToLower(token)
	type group struct {
		agents []string
		rules  []robotsRule
	}
	var groups []group
	var cur *group
	lastWasAgent := false

	for _, line := range strings.Split(content, "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key, val = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(val)
		switch key {
		case "user-agent":
			if !lastWasAgent || cur == nil {
				groups = append(groups, group{})
				cur = &groups[len(groups)-1]
			}
			cur.agents = append(cur.agents, strings.ToLower(val))
			lastWasAgent = true
		case "allow", "disallow":
			lastWasAgent = false
			if cur == nil {
				continue
			}
			cur.rules = append(cur.rules, robotsRule{allow: key == "allow", path: val})
		default:
			lastWasAgent = false
		}
	}

	pick := func(match func(agent string) bool) *group {
		for i := range groups {
			for _, a := range groups[i].agents {
				if match(a) {
					return &groups[i]
				}
			}
		}
		return nil
	}
	g := pick(func(a string) bool { return a != "*" && a != "" && strings.Contains(token, a) })
	if g == nil {
		g = pick(func(a string) bool { return a == "*" })
	}
	r := &Robots{}
	if g != nil {
		r.rules = g.rules
	}
	return r
}

// Allowed reports whether path (with optional query) may be fetched.
func (r *Robots) Allowed(path string) bool {
	if r == nil {
		return true
	}
	if path == "" {
		path = "/"
	}
	bestLen, allowed := -1, true
	for _, rule := range r.rules {
		if rule.path == "" { // empty Disallow = allow everything; empty Allow = no-op
			continue
		}
		if !matchRobots(rule.path, path) {
			continue
		}
		l := len(rule.path)
		if l > bestLen || (l == bestLen && rule.allow) {
			bestLen, allowed = l, rule.allow
		}
	}
	return allowed
}

// matchRobots matches a robots.txt path pattern (supports * and trailing $).
func matchRobots(pattern, path string) bool {
	anchored := strings.HasSuffix(pattern, "$")
	pattern = strings.TrimSuffix(pattern, "$")
	parts := strings.Split(pattern, "*")
	pos := 0
	for i, part := range parts {
		idx := strings.Index(path[pos:], part)
		if idx < 0 || (i == 0 && idx != 0) {
			return false
		}
		pos += idx + len(part)
	}
	if anchored {
		last := parts[len(parts)-1]
		return strings.HasSuffix(path, last) && pos == len(path)
	}
	return true
}
