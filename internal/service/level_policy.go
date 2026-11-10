package service

import "slices"

// proficiencyLevels lists levels easiest first; a language or level missing here keeps an exact scope.
var proficiencyLevels = map[string][]string{
	"ja": {"NZ", "N5", "N4", "N3", "N2", "N1"},
}

// levelRank puts unknown levels last.
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

// Either is "" at the ends of the order or when the level has no known order.
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
