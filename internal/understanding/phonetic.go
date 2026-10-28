package understanding

import (
	"regexp"
	"strings"
)

type phoneticRule struct {
	pattern   *regexp.Regexp
	canonical string
}

var phoneticRules = []phoneticRule{
	{
		pattern: regexp.MustCompile(
			`(?i)(^|[^\p{L}\p{N}])anh\s+trai\s+` +
				`(?:c\s*2|say\s+hai|xe\s+hai|say\s+hi)([^\p{L}\p{N}]|$)`,
		),
		canonical: "Anh Trai Say Hi",
	},
	{
		pattern: regexp.MustCompile(
			`(?i)(^|[^\p{L}\p{N}])(?:a\s+ven\s+dơ|ơ\s+ven\s+gơ|` +
				`aven\s+gơ|aven\s+dơ|a\s+ven\s+gơ|ơ\s+ven\s+dơ)([^\p{L}\p{N}]|$)`,
		),
		canonical: "Avengers",
	},
	{
		pattern: regexp.MustCompile(
			`(?i)(^|[^\p{L}\p{N}])(?:en\s+gêm|end\s+gêm|` +
				`en\s+game|end\s+game)([^\p{L}\p{N}]|$)`,
		),
		canonical: "Endgame",
	},
	{
		pattern: regexp.MustCompile(
			`(?i)(^|[^\p{L}\p{N}])(?:úp|ốp|op)\s+pen\s+` +
				`(?:hay|hai)\s+(?:mơ|mo)([^\p{L}\p{N}]|$)`,
		),
		canonical: "Oppenheimer",
	},
	{
		pattern: regexp.MustCompile(
			`(?i)(^|[^\p{L}\p{N}])ai\s+(?:dơn|don)\s+` +
				`(?:men|man)([^\p{L}\p{N}]|$)`,
		),
		canonical: "Iron Man",
	},
	{
		pattern: regexp.MustCompile(
			`(?i)(^|[^\p{L}\p{N}])(?:xì|xi)\s+pai\s+` +
				`(?:đơ|do)\s+(?:men|man)([^\p{L}\p{N}]|$)`,
		),
		canonical: "Spider-Man",
	},
	{
		pattern: regexp.MustCompile(
			`(?i)(^|[^\p{L}\p{N}])(?:đa|dra|da)\s+` +
				`(?:gôn|gon)\s+(?:bôn|bon)([^\p{L}\p{N}]|$)`,
		),
		canonical: "Dragon Ball",
	},
}

func NormalizePhonetics(rawText string) string {
	normalized := rawText
	for _, rule := range phoneticRules {
		normalized = rule.pattern.ReplaceAllString(
			normalized,
			"${1}"+rule.canonical+"${2}",
		)
	}

	return strings.Join(strings.Fields(normalized), " ")
}
