package service

import "slices"

// proficiencyLevels lists each language's levels in difficulty order, easiest
// first. Every level relation (rank, neighbors, session scope) is read from
// this map; a language or level missing here keeps an exact scope. Japanese NZ
// holds the kana (hiragana/katakana) foundation below N5.
var proficiencyLevels = map[string][]string{
	"ja": {"NZ", "N5", "N4", "N3", "N2", "N1"},
}

// levelRank returns level's position in its language's order and puts
// unknown levels last.
func levelRank(
	language,
	level string,
) int {
	levels := proficiencyLevels[language]
	if rank := slices.Index(
		levels,
		level,
	); rank >= 0 {
		return rank
	}
	return len(levels)
}

// neighborLevels returns the levels one step easier and one step harder than
// level. Either is "" at the ends of the order, and both are "" when the
// language or level has no known order.
func neighborLevels(
	language,
	level string,
) (lower, upper string) {
	levels := proficiencyLevels[language]
	i := slices.Index(
		levels,
		level,
	)
	if i < 0 {
		return "", ""
	}
	if i > 0 {
		lower = levels[i-1]
	}
	if i+1 < len(levels) {
		upper = levels[i+1]
	}
	return lower, upper
}

// sessionLevelsFor returns the Study and Quiz scope for a user's current
// level: the level and its neighbors, easiest first. Levels without a known
// order keep an exact scope.
func sessionLevelsFor(
	language,
	currentLevel string,
) []string {
	lower, upper := neighborLevels(
		language,
		currentLevel,
	)
	var scope []string
	if lower != "" {
		scope = append(
			scope,
			lower,
		)
	}
	scope = append(
		scope,
		currentLevel,
	)
	if upper != "" {
		scope = append(
			scope,
			upper,
		)
	}
	return scope
}
