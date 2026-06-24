package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"math/rand"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	ja "github.com/lsj/copylingo/cmd/ja/catalog"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/repository"
)

const (
	vocabLanguage         = ja.VocabLanguage
	vocabProficiencyLevel = ja.VocabProficiencyLevel
	vocabDifficulty       = ja.VocabDifficulty
)

type vocabWord = ja.VocabWord
type grammarPoint = ja.GrammarPoint

var n5Words = ja.N5Words
var n5GrammarPoints = ja.N5GrammarPoints

func kanaScriptLabel(kana string) string {
	return ja.ScriptLabel(kana)
}

func kanaDisambiguationHint(kana string) string {
	switch kana {
	case "じ", "ず":
		return "さ행에 탁점"
	case "ぢ", "づ":
		return "た행에 탁점"
	case "ジ", "ズ":
		return "サ행에 탁점"
	case "ヂ", "ヅ":
		return "タ행에 탁점"
	default:
		return ""
	}
}

func appendKanaDisambiguationHint(prompt, kana string) string {
	if hint := kanaDisambiguationHint(kana); hint != "" {
		return fmt.Sprintf("%s<br>힌트: <b>%s</b>", prompt, hint)
	}
	return prompt
}

func initDB(cfg *config.Config) (*sqlx.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.User, cfg.DB.Password, cfg.DB.DBName, cfg.DB.SSLMode)
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := initDB(cfg)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Close()

	repos := repository.NewRepositories(db)
	ctx := context.Background()

	materials := ja.BuildAllMaterials()
	if err := repos.Material.UpsertBatch(ctx, materials); err != nil {
		log.Fatalf("Failed to upsert Japanese materials batch: %v", err)
	}
	log.Printf("Successfully upserted %d Japanese materials.", len(materials))

	materialIDsByKana, err := loadKanaMaterialIDs(ctx, repos.Material, ja.KanaMap)
	if err != nil {
		log.Fatalf("Failed to load kana materials: %v", err)
	}
	materialIDsByWordID, err := loadVocabularyMaterialIDs(ctx, repos.Material, n5Words)
	if err != nil {
		log.Fatalf("Failed to load vocabulary materials: %v", err)
	}
	materialIDsByGrammarID, err := loadGrammarMaterialIDs(ctx, repos.Material, n5GrammarPoints)
	if err != nil {
		log.Fatalf("Failed to load grammar materials: %v", err)
	}

	rng := rand.New(rand.NewSource(1))

	kanaQuestions := buildKanaQuestions(materialIDsByKana)
	vocabularyQuestions := buildVocabularyQuestions(rng, n5Words, materialIDsByWordID)
	grammarQuestions := buildGrammarQuestions(rng, n5GrammarPoints, materialIDsByGrammarID)
	questions := make([]*model.Question, 0, len(kanaQuestions)+len(vocabularyQuestions)+len(grammarQuestions))
	questions = append(questions, kanaQuestions...)
	questions = append(questions, vocabularyQuestions...)
	questions = append(questions, grammarQuestions...)

	if err := repos.Question.UpsertSeedBatch(ctx, questions); err != nil {
		log.Printf("Failed to upsert Japanese questions batch: %v", err)
		return
	}

	log.Printf(
		"Successfully upserted %d Japanese questions. kana=%d vocabulary=%d grammar=%d",
		len(questions),
		len(kanaQuestions),
		len(vocabularyQuestions),
		len(grammarQuestions),
	)
}

func buildKanaQuestions(materialIDsByKana map[string]int) []*model.Question {
	kanaList := make([]string, 0, len(ja.KanaMap))
	romajiList := make([]string, 0, len(ja.KanaMap))
	for k, v := range ja.KanaMap {
		kanaList = append(kanaList, k)
		romajiList = append(romajiList, v)
	}
	sort.Strings(kanaList)
	sort.Strings(romajiList)

	questions := make([]*model.Question, 0, len(kanaList)*3)

	// Type 1: Kana -> Romaji (Existing)
	for _, kana := range kanaList {
		romaji := ja.KanaMap[kana]
		question := buildQuestion(kana, romaji, romajiList, true)
		setQuestionKey(question, kanaQuestionKey(kana, "reading"))
		setQuestionMaterial(question, materialIDsByKana[kana])
		questions = append(questions, question)
	}

	// Type 2: Romaji -> Kana (New)
	for _, kana := range kanaList {
		romaji := ja.KanaMap[kana]
		question := buildQuestion(romaji, kana, kanaList, false)
		setQuestionKey(question, kanaQuestionKey(kana, "recall"))
		setQuestionMaterial(question, materialIDsByKana[kana])
		questions = append(questions, question)
	}

	// Type 3: Romaji -> Kana handwriting (Mini App)
	for _, kana := range kanaList {
		if !shouldSeedHandwritingQuestion(kana) {
			continue
		}
		romaji := ja.KanaMap[kana]
		question := buildHandwritingQuestion(romaji, kana)
		setQuestionKey(question, kanaQuestionKey(kana, "handwriting"))
		setQuestionMaterial(question, materialIDsByKana[kana])
		questions = append(questions, question)
	}

	return questions
}

type kanaMaterialStore interface {
	GetByMaterialKeys(ctx context.Context, keys []string) ([]model.Material, error)
}

func loadKanaMaterialIDs(
	ctx context.Context,
	store kanaMaterialStore,
	kanaMap map[string]string,
) (map[string]int, error) {
	keys := make([]string, 0, len(kanaMap))
	keyByKana := make(map[string]string, len(kanaMap))
	for kana := range kanaMap {
		key := ja.MaterialKeyForKana(kana)
		keys = append(keys, key)
		keyByKana[kana] = key
	}

	materials, err := store.GetByMaterialKeys(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("load kana material ids: %w", err)
	}

	idByKey := make(map[string]int, len(materials))
	for _, material := range materials {
		idByKey[material.MaterialKey] = material.ID
	}

	materialIDsByKana := make(map[string]int, len(kanaMap))
	for kana, key := range keyByKana {
		id, ok := idByKey[key]
		if !ok {
			return nil, fmt.Errorf("missing kana material: %s", key)
		}
		materialIDsByKana[kana] = id
	}
	return materialIDsByKana, nil
}

func setQuestionMaterial(question *model.Question, materialID int) {
	id := materialID
	question.MaterialID = &id
}

func setQuestionKey(question *model.Question, questionKey string) {
	question.QuestionKey = &questionKey
}

func kanaQuestionKey(kana, variant string) string {
	return fmt.Sprintf("%s:%s", ja.MaterialKeyForKana(kana), variant)
}

type vocabularyMaterialStore interface {
	GetByMaterialKeys(ctx context.Context, keys []string) ([]model.Material, error)
}

func loadVocabularyMaterialIDs(
	ctx context.Context,
	store vocabularyMaterialStore,
	words []vocabWord,
) (map[string]int, error) {
	keys := make([]string, 0, len(words))
	keyByWordID := make(map[string]string, len(words))
	for _, word := range words {
		key := vocabularyMaterialKey(word)
		keys = append(keys, key)
		keyByWordID[word.ID] = key
	}

	materials, err := store.GetByMaterialKeys(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("load vocabulary material ids: %w", err)
	}

	idByKey := make(map[string]int, len(materials))
	for _, material := range materials {
		idByKey[material.MaterialKey] = material.ID
	}

	materialIDsByWordID := make(map[string]int, len(words))
	missing := make([]string, 0)
	for _, word := range words {
		key := keyByWordID[word.ID]
		id, ok := idByKey[key]
		if !ok {
			missing = append(missing, key)
			continue
		}
		materialIDsByWordID[word.ID] = id
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing vocabulary materials: %s", strings.Join(missing, ", "))
	}

	return materialIDsByWordID, nil
}

func vocabularyMaterialKey(word vocabWord) string {
	return ja.MaterialKeyForVocab(word)
}

func vocabularyQuestionKey(word vocabWord, variant string) string {
	return fmt.Sprintf("%s:%s", vocabularyMaterialKey(word), variant)
}

func buildVocabularyQuestions(
	rng *rand.Rand,
	words []vocabWord,
	materialIDsByWordID map[string]int,
) []*model.Question {
	questions := make([]*model.Question, 0, len(words)*3)
	for _, word := range words {
		wordQuestions := []*model.Question{
			buildKanaToMeaningQuestion(rng, word, words),
			buildMeaningToKanaQuestion(word),
			buildMeaningToKanaHandwritingQuestion(word),
		}
		setQuestionKey(wordQuestions[0], vocabularyQuestionKey(word, "meaning"))
		setQuestionKey(wordQuestions[1], vocabularyQuestionKey(word, "recall"))
		setQuestionKey(wordQuestions[2], vocabularyQuestionKey(word, "handwriting"))
		if materialID, ok := materialIDsByWordID[word.ID]; ok {
			for _, question := range wordQuestions {
				question.MaterialID = &materialID
			}
		}
		questions = append(questions, wordQuestions...)
	}
	return questions
}

func buildKanaToMeaningQuestion(rng *rand.Rand, word vocabWord, wrongPool []vocabWord) *model.Question {
	options := buildMeaningOptions(rng, word, wrongPool)

	return &model.Question{
		Type:             model.QuestionMultipleChoice,
		Skill:            model.SkillPtr(model.SkillVocabMeaning),
		Language:         vocabLanguage,
		ProficiencyLevel: vocabProficiencyLevel,
		Category:         model.CategoryVocabulary,
		Prompt:           fmt.Sprintf("다음 단어의 뜻을 고르세요: %s", formatWordPrompt(word)),
		Options:          mustJSON(options),
		CorrectAnswer:    word.MeaningKo,
		Explanation:      formatExplanation(word),
		Difficulty:       vocabDifficulty,
	}
}

func buildMeaningToKanaQuestion(word vocabWord) *model.Question {
	scriptLabel := vocabularyScriptLabel(word.Kana)
	prompt := fmt.Sprintf("뜻 <b>'%s'</b>에 해당하는 일본어 발음을 %s로 입력하세요", word.MeaningKo, scriptLabel)
	if isCounterWord(word) {
		prompt = fmt.Sprintf("다음 표현의 %s 읽기를 입력하세요: %s", scriptLabel, formatCounterReadingPrompt(word))
	}
	return &model.Question{
		Type:             model.QuestionFillBlank,
		Skill:            model.SkillPtr(model.SkillVocabRecall),
		Language:         vocabLanguage,
		ProficiencyLevel: vocabProficiencyLevel,
		Category:         model.CategoryVocabulary,
		Prompt:           prompt,
		Options:          []byte("[]"),
		CorrectAnswer:    word.Kana,
		Explanation:      formatExplanation(word),
		Difficulty:       vocabDifficulty,
	}
}

func buildMeaningToKanaHandwritingQuestion(word vocabWord) *model.Question {
	scriptLabel := vocabularyScriptLabel(word.Kana)
	prompt := fmt.Sprintf("뜻 <b>'%s'</b>에 해당하는 일본어 단어를 %s로 쓰세요", word.MeaningKo, scriptLabel)
	if isCounterWord(word) {
		prompt = fmt.Sprintf("다음 표현의 %s 읽기를 손글씨로 쓰세요: %s", scriptLabel, formatCounterReadingPrompt(word))
	}
	return &model.Question{
		Type:             model.QuestionKanaHandwriting,
		Skill:            model.SkillPtr(model.SkillVocabHandwriting),
		Language:         vocabLanguage,
		ProficiencyLevel: vocabProficiencyLevel,
		Category:         model.CategoryVocabulary,
		Prompt:           prompt,
		Options:          []byte("[]"),
		CorrectAnswer:    word.Kana,
		Explanation:      formatExplanation(word),
		Difficulty:       vocabDifficulty,
	}
}

func isCounterWord(word vocabWord) bool {
	return word.PartOfSpeech == "counter"
}

func vocabularyScriptLabel(kana string) string {
	for _, r := range kana {
		if r >= 'ァ' && r <= 'ヺ' {
			return "가타카나"
		}
	}
	return "히라가나"
}

func buildMeaningOptions(rng *rand.Rand, word vocabWord, wrongPool []vocabWord) []string {
	options := []string{word.MeaningKo}
	seen := map[string]bool{word.MeaningKo: true}

	wrongMeanings := make([]string, 0, len(wrongPool))
	for _, candidate := range wrongPool {
		if candidate.MeaningKo == word.MeaningKo || seen[candidate.MeaningKo] {
			continue
		}
		seen[candidate.MeaningKo] = true
		wrongMeanings = append(wrongMeanings, candidate.MeaningKo)
	}

	rng.Shuffle(len(wrongMeanings), func(i, j int) {
		wrongMeanings[i], wrongMeanings[j] = wrongMeanings[j], wrongMeanings[i]
	})
	for _, wrong := range wrongMeanings {
		if len(options) >= 4 {
			break
		}
		options = append(options, wrong)
	}

	rng.Shuffle(len(options), func(i, j int) {
		options[i], options[j] = options[j], options[i]
	})
	return options
}

func formatWordPrompt(word vocabWord) string {
	if word.Kanji != word.Kana {
		return fmt.Sprintf("<b>%s</b> (<b>%s</b>)", word.Kana, word.Kanji)
	}
	return fmt.Sprintf("<b>%s</b>", word.Kana)
}

func formatCounterReadingPrompt(word vocabWord) string {
	if word.Kanji != "" && word.Kanji != word.Kana {
		return fmt.Sprintf("<b>%s</b> (%s)", word.Kanji, word.MeaningKo)
	}
	return fmt.Sprintf("<b>%s</b> (%s)", word.Kana, word.MeaningKo)
}

func formatExplanation(word vocabWord) string {
	return fmt.Sprintf("<b>%s</b> / <b>%s</b> = %s", word.Kana, word.Kanji, word.MeaningKo)
}

type grammarMaterialStore interface {
	GetByMaterialKeys(ctx context.Context, keys []string) ([]model.Material, error)
}

func loadGrammarMaterialIDs(
	ctx context.Context,
	store grammarMaterialStore,
	points []grammarPoint,
) (map[string]int, error) {
	keys := make([]string, 0, len(points))
	keyByGrammarID := make(map[string]string, len(points))
	for _, point := range points {
		key := grammarMaterialKey(point)
		keys = append(keys, key)
		keyByGrammarID[point.ID] = key
	}

	materials, err := store.GetByMaterialKeys(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("load grammar material ids: %w", err)
	}

	idByKey := make(map[string]int, len(materials))
	for _, material := range materials {
		idByKey[material.MaterialKey] = material.ID
	}

	materialIDsByGrammarID := make(map[string]int, len(points))
	missing := make([]string, 0)
	for _, point := range points {
		key := keyByGrammarID[point.ID]
		id, ok := idByKey[key]
		if !ok {
			missing = append(missing, key)
			continue
		}
		materialIDsByGrammarID[point.ID] = id
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing grammar materials: %s", strings.Join(missing, ", "))
	}

	return materialIDsByGrammarID, nil
}

func grammarMaterialKey(point grammarPoint) string {
	return ja.MaterialKeyForGrammar(point)
}

func grammarQuestionKey(point grammarPoint, variant string) string {
	return fmt.Sprintf("%s:%s", grammarMaterialKey(point), variant)
}

func buildGrammarQuestions(
	rng *rand.Rand,
	points []grammarPoint,
	materialIDsByGrammarID map[string]int,
) []*model.Question {
	questions := make([]*model.Question, 0, len(points)*2)
	for _, point := range points {
		pointQuestions := []*model.Question{
			buildGrammarMeaningQuestion(rng, point, points),
			buildGrammarFormQuestion(point),
		}
		setQuestionKey(pointQuestions[0], grammarQuestionKey(point, "meaning"))
		setQuestionKey(pointQuestions[1], grammarQuestionKey(point, "form"))
		if materialID, ok := materialIDsByGrammarID[point.ID]; ok {
			for _, question := range pointQuestions {
				question.MaterialID = &materialID
			}
		}
		questions = append(questions, pointQuestions...)
	}
	return questions
}

func buildGrammarMeaningQuestion(
	rng *rand.Rand,
	point grammarPoint,
	wrongPool []grammarPoint,
) *model.Question {
	return &model.Question{
		Type:             model.QuestionMultipleChoice,
		Skill:            model.SkillPtr(model.SkillGrammarForm),
		Language:         vocabLanguage,
		ProficiencyLevel: vocabProficiencyLevel,
		Category:         model.CategoryGrammar,
		Prompt: fmt.Sprintf(
			"다음 문법의 핵심 의미를 고르세요: <b>%s</b><br>예문: <b>%s</b>",
			point.Pattern,
			point.Example,
		),
		Options:       mustJSON(buildGrammarMeaningOptions(rng, point, wrongPool)),
		CorrectAnswer: point.MeaningKo,
		Explanation:   formatGrammarExplanation(point),
		Difficulty:    ja.GrammarDifficulty,
	}
}

func buildGrammarFormQuestion(point grammarPoint) *model.Question {
	return &model.Question{
		Type:             model.QuestionMultipleChoice,
		Skill:            model.SkillPtr(model.SkillGrammarForm),
		Language:         vocabLanguage,
		ProficiencyLevel: vocabProficiencyLevel,
		Category:         model.CategoryGrammar,
		Prompt:           fmt.Sprintf("빈칸에 들어갈 알맞은 표현을 고르세요: <b>%s</b>", point.ClozePrompt),
		Options:          mustJSON(point.FormOptions),
		CorrectAnswer:    point.CorrectAnswer,
		Explanation:      formatGrammarExplanation(point),
		Difficulty:       ja.GrammarDifficulty,
	}
}

func buildGrammarMeaningOptions(
	rng *rand.Rand,
	point grammarPoint,
	wrongPool []grammarPoint,
) []string {
	options := []string{point.MeaningKo}
	seen := map[string]bool{point.MeaningKo: true}

	wrongMeanings := make([]string, 0, len(wrongPool))
	for _, candidate := range wrongPool {
		if candidate.MeaningKo == point.MeaningKo || seen[candidate.MeaningKo] {
			continue
		}
		seen[candidate.MeaningKo] = true
		wrongMeanings = append(wrongMeanings, candidate.MeaningKo)
	}

	rng.Shuffle(len(wrongMeanings), func(i, j int) {
		wrongMeanings[i], wrongMeanings[j] = wrongMeanings[j], wrongMeanings[i]
	})
	for _, wrong := range wrongMeanings {
		if len(options) >= 4 {
			break
		}
		options = append(options, wrong)
	}

	rng.Shuffle(len(options), func(i, j int) {
		options[i], options[j] = options[j], options[i]
	})
	return options
}

func formatGrammarExplanation(point grammarPoint) string {
	return fmt.Sprintf(
		"<b>%s</b>: %s<br>%s<br>예문: <b>%s</b> (%s)",
		point.Pattern,
		point.MeaningKo,
		point.ExplanationKo,
		point.Example,
		point.TranslationKo,
	)
}

func mustJSON(values []string) json.RawMessage {
	b, err := json.Marshal(values)
	if err != nil {
		panic(fmt.Sprintf("marshal options: %v", err))
	}
	return b
}

func shouldSeedHandwritingQuestion(kana string) bool {
	switch kana {
	case "ユ", "ヲ":
		return false
	default:
		return true
	}
}

// buildQuestion constructs a Kana question for batch insertion.
// promptVal is the value shown in the prompt (e.g., 'あ' or 'a').
// answerVal is the correct answer (e.g., 'a' or '아').
// wrongPool is the list of values to pick incorrect options from.
// isToRomaji indicates if the answer is in Romaji (true) or Kana (false).
func buildQuestion(promptVal, answerVal string, wrongPool []string, isToRomaji bool) *model.Question {
	rng := rand.New(rand.NewSource(deterministicSeed(promptVal + "|" + answerVal)))
	isFillBlank := rng.Float32() < 0.7

	qType := model.QuestionFillBlank
	skill := model.SkillKanaRecall
	if isToRomaji {
		skill = model.SkillKanaReading
	}
	var options []string
	if !isFillBlank {
		qType = model.QuestionMultipleChoice
		options = append(options, answerVal)
		for len(options) < 4 {
			wrong := wrongPool[rng.Intn(len(wrongPool))]
			if wrong == answerVal {
				continue
			}
			exists := false
			for _, o := range options {
				if o == wrong {
					exists = true
					break
				}
			}
			if !exists {
				options = append(options, wrong)
			}
		}
		rng.Shuffle(len(options), func(i, j int) { options[i], options[j] = options[j], options[i] })
	}

	optBytes, _ := json.Marshal(options)

	var prompt string
	scriptLabel := ""
	if !isToRomaji {
		scriptLabel = kanaScriptLabel(answerVal)
	}

	if isToRomaji {
		if isFillBlank {
			prompt = fmt.Sprintf("다음 문자의 올바른 발음을 입력하세요: <b>%s</b>", promptVal)
		} else {
			prompt = fmt.Sprintf("다음 문자의 올바른 발음을 고르시오: <b>%s</b>", promptVal)
		}
	} else {
		if isFillBlank {
			prompt = fmt.Sprintf("발음 <b>'%s'</b>에 해당하는 %s 문자를 입력하세요", promptVal, scriptLabel)
		} else {
			prompt = fmt.Sprintf("발음 <b>'%s'</b>에 해당하는 %s 문자를 고르시오", promptVal, scriptLabel)
		}
		prompt = appendKanaDisambiguationHint(prompt, answerVal)
	}

	var explanation string
	if isToRomaji {
		explanation = fmt.Sprintf("<b>%s</b>의 발음은 <b>'%s'</b>입니다.", promptVal, answerVal)
	} else {
		explanation = fmt.Sprintf("발음 <b>'%s'</b>에 해당하는 %s 문자는 <b>%s</b>입니다.", promptVal, scriptLabel, answerVal)
	}

	return &model.Question{
		Type:             qType,
		Skill:            model.SkillPtr(skill),
		Language:         "ja",
		ProficiencyLevel: "N5",
		Category:         "kana",
		Prompt:           prompt,
		Options:          optBytes,
		CorrectAnswer:    answerVal,
		Explanation:      explanation,
		Difficulty:       1,
	}
}

func deterministicSeed(value string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(value))
	return int64(h.Sum64())
}

func buildHandwritingQuestion(romaji, kana string) *model.Question {
	scriptLabel := kanaScriptLabel(kana)
	prompt := fmt.Sprintf("발음 <b>'%s'</b>에 해당하는 %s 문자를 손글씨로 쓰세요", romaji, scriptLabel)

	return &model.Question{
		Type:             model.QuestionKanaHandwriting,
		Skill:            model.SkillPtr(model.SkillKanaHandwriting),
		Language:         "ja",
		ProficiencyLevel: "N5",
		Category:         "handwriting",
		Prompt:           appendKanaDisambiguationHint(prompt, kana),
		Options:          []byte("[]"),
		CorrectAnswer:    kana,
		Explanation:      fmt.Sprintf("발음 <b>'%s'</b>에 해당하는 %s 문자는 <b>%s</b>입니다.", romaji, scriptLabel, kana),
		Difficulty:       1,
	}
}
