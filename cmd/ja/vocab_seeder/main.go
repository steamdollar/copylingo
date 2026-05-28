package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/repository"
)

const (
	vocabLanguage         = "ja"
	vocabProficiencyLevel = "N5"
	vocabDifficulty       = 2
)

type vocabWord struct {
	ID           string
	Kana         string
	Kanji        string
	MeaningKo    string
	PartOfSpeech string
}

var n5Words = []vocabWord{
	{ID: "n5_word_001", Kana: "わたし", Kanji: "私", MeaningKo: "나", PartOfSpeech: "pronoun"},
	{ID: "n5_word_002", Kana: "あなた", Kanji: "あなた", MeaningKo: "당신", PartOfSpeech: "pronoun"},
	{ID: "n5_word_003", Kana: "ひと", Kanji: "人", MeaningKo: "사람", PartOfSpeech: "noun"},
	{ID: "n5_word_004", Kana: "おとこ", Kanji: "男", MeaningKo: "남자", PartOfSpeech: "noun"},
	{ID: "n5_word_005", Kana: "おんな", Kanji: "女", MeaningKo: "여자", PartOfSpeech: "noun"},
	{ID: "n5_word_006", Kana: "こども", Kanji: "子供", MeaningKo: "아이", PartOfSpeech: "noun"},
	{ID: "n5_word_007", Kana: "ともだち", Kanji: "友達", MeaningKo: "친구", PartOfSpeech: "noun"},
	{ID: "n5_word_008", Kana: "せんせい", Kanji: "先生", MeaningKo: "선생님", PartOfSpeech: "noun"},
	{ID: "n5_word_009", Kana: "がくせい", Kanji: "学生", MeaningKo: "학생", PartOfSpeech: "noun"},
	{ID: "n5_word_010", Kana: "がっこう", Kanji: "学校", MeaningKo: "학교", PartOfSpeech: "noun"},
	{ID: "n5_word_011", Kana: "だいがく", Kanji: "大学", MeaningKo: "대학교", PartOfSpeech: "noun"},
	{ID: "n5_word_012", Kana: "いえ", Kanji: "家", MeaningKo: "집", PartOfSpeech: "noun"},
	{ID: "n5_word_013", Kana: "へや", Kanji: "部屋", MeaningKo: "방", PartOfSpeech: "noun"},
	{ID: "n5_word_014", Kana: "えき", Kanji: "駅", MeaningKo: "역", PartOfSpeech: "noun"},
	{ID: "n5_word_015", Kana: "みせ", Kanji: "店", MeaningKo: "가게", PartOfSpeech: "noun"},
	{ID: "n5_word_016", Kana: "かいしゃ", Kanji: "会社", MeaningKo: "회사", PartOfSpeech: "noun"},
	{ID: "n5_word_017", Kana: "びょういん", Kanji: "病院", MeaningKo: "병원", PartOfSpeech: "noun"},
	{ID: "n5_word_018", Kana: "ほん", Kanji: "本", MeaningKo: "책", PartOfSpeech: "noun"},
	{ID: "n5_word_019", Kana: "じしょ", Kanji: "辞書", MeaningKo: "사전", PartOfSpeech: "noun"},
	{ID: "n5_word_020", Kana: "てがみ", Kanji: "手紙", MeaningKo: "편지", PartOfSpeech: "noun"},
	{ID: "n5_word_021", Kana: "くるま", Kanji: "車", MeaningKo: "자동차", PartOfSpeech: "noun"},
	{ID: "n5_word_022", Kana: "でんしゃ", Kanji: "電車", MeaningKo: "전철", PartOfSpeech: "noun"},
	{ID: "n5_word_023", Kana: "じてんしゃ", Kanji: "自転車", MeaningKo: "자전거", PartOfSpeech: "noun"},
	{ID: "n5_word_024", Kana: "みず", Kanji: "水", MeaningKo: "물", PartOfSpeech: "noun"},
	{ID: "n5_word_025", Kana: "おちゃ", Kanji: "お茶", MeaningKo: "차", PartOfSpeech: "noun"},
	{ID: "n5_word_026", Kana: "ごはん", Kanji: "ご飯", MeaningKo: "밥", PartOfSpeech: "noun"},
	{ID: "n5_word_027", Kana: "さかな", Kanji: "魚", MeaningKo: "생선", PartOfSpeech: "noun"},
	{ID: "n5_word_028", Kana: "にく", Kanji: "肉", MeaningKo: "고기", PartOfSpeech: "noun"},
	{ID: "n5_word_029", Kana: "たまご", Kanji: "卵", MeaningKo: "달걀", PartOfSpeech: "noun"},
	{ID: "n5_word_030", Kana: "くだもの", Kanji: "果物", MeaningKo: "과일", PartOfSpeech: "noun"},
	{ID: "n5_word_031", Kana: "ねこ", Kanji: "猫", MeaningKo: "고양이", PartOfSpeech: "noun"},
	{ID: "n5_word_032", Kana: "いぬ", Kanji: "犬", MeaningKo: "개", PartOfSpeech: "noun"},
	{ID: "n5_word_033", Kana: "やま", Kanji: "山", MeaningKo: "산", PartOfSpeech: "noun"},
	{ID: "n5_word_034", Kana: "かわ", Kanji: "川", MeaningKo: "강", PartOfSpeech: "noun"},
	{ID: "n5_word_035", Kana: "あめ", Kanji: "雨", MeaningKo: "비", PartOfSpeech: "noun"},
	{ID: "n5_word_036", Kana: "ゆき", Kanji: "雪", MeaningKo: "눈", PartOfSpeech: "noun"},
	{ID: "n5_word_037", Kana: "ひ", Kanji: "日", MeaningKo: "해", PartOfSpeech: "noun"},
	{ID: "n5_word_038", Kana: "つき", Kanji: "月", MeaningKo: "달", PartOfSpeech: "noun"},
	{ID: "n5_word_039", Kana: "ひ", Kanji: "火", MeaningKo: "불", PartOfSpeech: "noun"},
	{ID: "n5_word_040", Kana: "き", Kanji: "木", MeaningKo: "나무", PartOfSpeech: "noun"},
	{ID: "n5_word_041", Kana: "きん", Kanji: "金", MeaningKo: "금", PartOfSpeech: "noun"},
	{ID: "n5_word_042", Kana: "つち", Kanji: "土", MeaningKo: "흙", PartOfSpeech: "noun"},
	{ID: "n5_word_043", Kana: "きょう", Kanji: "今日", MeaningKo: "오늘", PartOfSpeech: "noun"},
	{ID: "n5_word_044", Kana: "あした", Kanji: "明日", MeaningKo: "내일", PartOfSpeech: "noun"},
	{ID: "n5_word_045", Kana: "きのう", Kanji: "昨日", MeaningKo: "어제", PartOfSpeech: "noun"},
	{ID: "n5_word_046", Kana: "あさ", Kanji: "朝", MeaningKo: "아침", PartOfSpeech: "noun"},
	{ID: "n5_word_047", Kana: "ひる", Kanji: "昼", MeaningKo: "낮", PartOfSpeech: "noun"},
	{ID: "n5_word_048", Kana: "よる", Kanji: "夜", MeaningKo: "밤", PartOfSpeech: "noun"},
	{ID: "n5_word_049", Kana: "いま", Kanji: "今", MeaningKo: "지금", PartOfSpeech: "noun"},
	{ID: "n5_word_050", Kana: "まいにち", Kanji: "毎日", MeaningKo: "매일", PartOfSpeech: "noun"},
	{ID: "n5_word_051", Kana: "うえ", Kanji: "上", MeaningKo: "위", PartOfSpeech: "noun"},
	{ID: "n5_word_052", Kana: "した", Kanji: "下", MeaningKo: "아래", PartOfSpeech: "noun"},
	{ID: "n5_word_053", Kana: "なか", Kanji: "中", MeaningKo: "안", PartOfSpeech: "noun"},
	{ID: "n5_word_054", Kana: "まえ", Kanji: "前", MeaningKo: "앞", PartOfSpeech: "noun"},
	{ID: "n5_word_055", Kana: "うしろ", Kanji: "後ろ", MeaningKo: "뒤", PartOfSpeech: "noun"},
	{ID: "n5_word_056", Kana: "みぎ", Kanji: "右", MeaningKo: "오른쪽", PartOfSpeech: "noun"},
	{ID: "n5_word_057", Kana: "ひだり", Kanji: "左", MeaningKo: "왼쪽", PartOfSpeech: "noun"},
	{ID: "n5_word_058", Kana: "いち", Kanji: "一", MeaningKo: "하나", PartOfSpeech: "numeral"},
	{ID: "n5_word_059", Kana: "に", Kanji: "二", MeaningKo: "둘", PartOfSpeech: "numeral"},
	{ID: "n5_word_060", Kana: "さん", Kanji: "三", MeaningKo: "셋", PartOfSpeech: "numeral"},
	{ID: "n5_word_061", Kana: "よん", Kanji: "四", MeaningKo: "넷", PartOfSpeech: "numeral"},
	{ID: "n5_word_062", Kana: "ご", Kanji: "五", MeaningKo: "다섯", PartOfSpeech: "numeral"},
	{ID: "n5_word_063", Kana: "ろく", Kanji: "六", MeaningKo: "여섯", PartOfSpeech: "numeral"},
	{ID: "n5_word_064", Kana: "なな", Kanji: "七", MeaningKo: "일곱", PartOfSpeech: "numeral"},
	{ID: "n5_word_065", Kana: "はち", Kanji: "八", MeaningKo: "여덟", PartOfSpeech: "numeral"},
	{ID: "n5_word_066", Kana: "きゅう", Kanji: "九", MeaningKo: "아홉", PartOfSpeech: "numeral"},
	{ID: "n5_word_067", Kana: "じゅう", Kanji: "十", MeaningKo: "열", PartOfSpeech: "numeral"},
	{ID: "n5_word_068", Kana: "ひゃく", Kanji: "百", MeaningKo: "백", PartOfSpeech: "numeral"},
	{ID: "n5_word_069", Kana: "せん", Kanji: "千", MeaningKo: "천", PartOfSpeech: "numeral"},
	{ID: "n5_word_070", Kana: "まん", Kanji: "万", MeaningKo: "만", PartOfSpeech: "numeral"},
	{ID: "n5_word_071", Kana: "えん", Kanji: "円", MeaningKo: "엔", PartOfSpeech: "noun"},
	{ID: "n5_word_072", Kana: "いく", Kanji: "行く", MeaningKo: "가다", PartOfSpeech: "verb"},
	{ID: "n5_word_073", Kana: "くる", Kanji: "来る", MeaningKo: "오다", PartOfSpeech: "verb"},
	{ID: "n5_word_074", Kana: "みる", Kanji: "見る", MeaningKo: "보다", PartOfSpeech: "verb"},
	{ID: "n5_word_075", Kana: "たべる", Kanji: "食べる", MeaningKo: "먹다", PartOfSpeech: "verb"},
	{ID: "n5_word_076", Kana: "のむ", Kanji: "飲む", MeaningKo: "마시다", PartOfSpeech: "verb"},
	{ID: "n5_word_077", Kana: "よむ", Kanji: "読む", MeaningKo: "읽다", PartOfSpeech: "verb"},
	{ID: "n5_word_078", Kana: "かく", Kanji: "書く", MeaningKo: "쓰다", PartOfSpeech: "verb"},
	{ID: "n5_word_079", Kana: "きく", Kanji: "聞く", MeaningKo: "듣다", PartOfSpeech: "verb"},
	{ID: "n5_word_080", Kana: "はなす", Kanji: "話す", MeaningKo: "말하다", PartOfSpeech: "verb"},
	{ID: "n5_word_081", Kana: "かう", Kanji: "買う", MeaningKo: "사다", PartOfSpeech: "verb"},
	{ID: "n5_word_082", Kana: "ある", Kanji: "ある", MeaningKo: "있다", PartOfSpeech: "verb"},
	{ID: "n5_word_083", Kana: "いる", Kanji: "いる", MeaningKo: "있다", PartOfSpeech: "verb"},
	{ID: "n5_word_084", Kana: "する", Kanji: "する", MeaningKo: "하다", PartOfSpeech: "verb"},
	{ID: "n5_word_085", Kana: "おおきい", Kanji: "大きい", MeaningKo: "크다", PartOfSpeech: "adjective"},
	{ID: "n5_word_086", Kana: "ちいさい", Kanji: "小さい", MeaningKo: "작다", PartOfSpeech: "adjective"},
	{ID: "n5_word_087", Kana: "たかい", Kanji: "高い", MeaningKo: "비싸다/높다", PartOfSpeech: "adjective"},
	{ID: "n5_word_088", Kana: "やすい", Kanji: "安い", MeaningKo: "싸다", PartOfSpeech: "adjective"},
	{ID: "n5_word_089", Kana: "あたらしい", Kanji: "新しい", MeaningKo: "새롭다", PartOfSpeech: "adjective"},
	{ID: "n5_word_090", Kana: "ふるい", Kanji: "古い", MeaningKo: "오래되다", PartOfSpeech: "adjective"},
	{ID: "n5_word_091", Kana: "あつい", Kanji: "暑い", MeaningKo: "덥다", PartOfSpeech: "adjective"},
	{ID: "n5_word_092", Kana: "さむい", Kanji: "寒い", MeaningKo: "춥다", PartOfSpeech: "adjective"},
	{ID: "n5_word_093", Kana: "いい", Kanji: "良い", MeaningKo: "좋다", PartOfSpeech: "adjective"},
	{ID: "n5_word_094", Kana: "わるい", Kanji: "悪い", MeaningKo: "나쁘다", PartOfSpeech: "adjective"},
	{ID: "n5_word_095", Kana: "テレビ", Kanji: "テレビ", MeaningKo: "TV", PartOfSpeech: "noun"},
	{ID: "n5_word_096", Kana: "コーヒー", Kanji: "コーヒー", MeaningKo: "커피", PartOfSpeech: "noun"},
	{ID: "n5_word_097", Kana: "パン", Kanji: "パン", MeaningKo: "빵", PartOfSpeech: "noun"},
	{ID: "n5_word_098", Kana: "タクシー", Kanji: "タクシー", MeaningKo: "택시", PartOfSpeech: "noun"},
	{ID: "n5_word_099", Kana: "ホテル", Kanji: "ホテル", MeaningKo: "호텔", PartOfSpeech: "noun"},
	{ID: "n5_word_100", Kana: "トイレ", Kanji: "トイレ", MeaningKo: "화장실", PartOfSpeech: "noun"},
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
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	questions := buildVocabularyQuestions(rng, n5Words)

	if err := repos.Question.CreateBatch(context.Background(), questions); err != nil {
		log.Printf("Failed to insert vocabulary questions batch: %v", err)
		return
	}

	log.Printf("Successfully inserted %d vocabulary questions.", len(questions))
}

func buildVocabularyQuestions(rng *rand.Rand, words []vocabWord) []*model.Question {
	questions := make([]*model.Question, 0, len(words)*3)
	for _, word := range words {
		questions = append(questions,
			buildKanaToMeaningQuestion(rng, word, words),
			buildMeaningToKanaQuestion(word),
			buildMeaningToKanaHandwritingQuestion(word),
		)
	}
	return questions
}

func buildKanaToMeaningQuestion(rng *rand.Rand, word vocabWord, wrongPool []vocabWord) *model.Question {
	options := buildMeaningOptions(rng, word, wrongPool)

	return &model.Question{
		Type:             model.QuestionMultipleChoice,
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
	scriptLabel := kanaScriptLabel(word.Kana)
	return &model.Question{
		Type:             model.QuestionFillBlank,
		Language:         vocabLanguage,
		ProficiencyLevel: vocabProficiencyLevel,
		Category:         model.CategoryVocabulary,
		Prompt:           fmt.Sprintf("뜻 <b>'%s'</b>에 해당하는 일본어 발음을 %s로 입력하세요", word.MeaningKo, scriptLabel),
		Options:          []byte("[]"),
		CorrectAnswer:    word.Kana,
		Explanation:      formatExplanation(word),
		Difficulty:       vocabDifficulty,
	}
}

func buildMeaningToKanaHandwritingQuestion(word vocabWord) *model.Question {
	scriptLabel := kanaScriptLabel(word.Kana)
	return &model.Question{
		Type:             model.QuestionKanaHandwriting,
		Language:         vocabLanguage,
		ProficiencyLevel: vocabProficiencyLevel,
		Category:         model.CategoryVocabulary,
		Prompt:           fmt.Sprintf("뜻 <b>'%s'</b>에 해당하는 일본어 단어를 %s로 쓰세요", word.MeaningKo, scriptLabel),
		Options:          []byte("[]"),
		CorrectAnswer:    word.Kana,
		Explanation:      formatExplanation(word),
		Difficulty:       vocabDifficulty,
	}
}

func kanaScriptLabel(kana string) string {
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

func formatExplanation(word vocabWord) string {
	return fmt.Sprintf("<b>%s</b> / <b>%s</b> = %s", word.Kana, word.Kanji, word.MeaningKo)
}

func mustJSON(values []string) json.RawMessage {
	b, err := json.Marshal(values)
	if err != nil {
		panic(fmt.Sprintf("marshal options: %v", err))
	}
	return b
}
