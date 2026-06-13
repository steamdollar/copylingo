package model

import "testing"

func TestSkillPtr(t *testing.T) {
	skill := SkillPtr(SkillVocabContext)
	if skill == nil || *skill != SkillVocabContext {
		t.Fatalf("SkillPtr = %v, want %q", skill, SkillVocabContext)
	}
}

func TestSkillTaxonomyIncludesN1Types(t *testing.T) {
	tests := []Skill{
		SkillKanjiReading,
		SkillVocabContext,
		SkillVocabParaphrase,
		SkillVocabUsage,
		SkillGrammarForm,
		SkillSentenceComposition,
		SkillTextGrammar,
		SkillReadingShort,
		SkillReadingMid,
		SkillReadingLong,
		SkillReadingIntegrated,
		SkillReadingThematic,
		SkillInformationRetrieval,
		SkillListeningTask,
		SkillListeningKeyPoint,
		SkillListeningOutline,
		SkillListeningQuickResponse,
		SkillListeningIntegrated,
	}

	for _, skill := range tests {
		if skill == "" {
			t.Fatal("skill must not be empty")
		}
	}
}
