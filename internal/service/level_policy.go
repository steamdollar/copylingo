package service

import "slices"

// japaneseProficiencyLevels defines Japanese level order once. NZ holds the
// kana (hiragana/katakana) foundation below N5, so it follows the same
// adjacency as JLPT levels. Session adjacency is derived from the learner's
// index instead of being duplicated per level.
var japaneseProficiencyLevels = []string{"NZ", "N5", "N4", "N3", "N2", "N1"}

// jlptLevelRank orders Japanese levels from NZ to N1 and puts unknown levels last.
func jlptLevelRank(level string) int {
	if rank := slices.Index(
		japaneseProficiencyLevels,
		level,
	); rank >= 0 {
		return rank
	}
	return len(japaneseProficiencyLevels)
}

// sessionLevelsFor returns the Study and Quiz scope for a user's current
// level. Only Japanese levels have a known adjacency; all other scopes
// remain exact until their ordering is explicitly defined.
func sessionLevelsFor(
	language,
	currentLevel string,
) []string {
	if language != "ja" {
		return []string{currentLevel}
	}

	for i, level := range japaneseProficiencyLevels {
		if level != currentLevel {
			continue
		}
		start := max(
			0,
			i-1,
		)
		end := min(
			len(japaneseProficiencyLevels),
			i+2,
		)
		return append(
			[]string(nil),
			japaneseProficiencyLevels[start:end]...,
		)
	}

	return []string{currentLevel}
}
