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

var kanaMap = map[string]string{
	// Hiragana basic
	"あ": "a", "い": "i", "う": "u", "え": "e", "お": "o",
	"か": "ka", "き": "ki", "く": "ku", "け": "ke", "こ": "ko",
	"さ": "sa", "し": "shi", "す": "su", "せ": "se", "そ": "so",
	"た": "ta", "ち": "chi", "つ": "tsu", "て": "te", "と": "to",
	"な": "na", "に": "ni", "ぬ": "nu", "ね": "ne", "の": "no",
	"は": "ha", "ひ": "hi", "ふ": "fu", "へ": "he", "ほ": "ho",
	"ま": "ma", "み": "mi", "む": "mu", "め": "me", "も": "mo",
	"や": "ya", "ゆ": "yu", "よ": "yo",
	"ら": "ra", "り": "ri", "る": "ru", "れ": "re", "ろ": "ro",
	"わ": "wa", "を": "wo", "ん": "n",
	// Hiragana dakuten
	"が": "ga", "ぎ": "gi", "ぐ": "gu", "げ": "ge", "ご": "go",
	"ざ": "za", "じ": "ji", "ず": "zu", "ぜ": "ze", "ぞ": "zo",
	"だ": "da", "ぢ": "ji", "づ": "zu", "で": "de", "ど": "do",
	"ば": "ba", "び": "bi", "ぶ": "bu", "べ": "be", "ぼ": "bo",
	"ぱ": "pa", "ぴ": "pi", "ぷ": "pu", "ぺ": "pe", "ぽ": "po",
	// Hiragana yoon
	"きゃ": "kya", "きゅ": "kyu", "きょ": "kyo",
	"しゃ": "sha", "しゅ": "shu", "しょ": "sho",
	"ちゃ": "cha", "ちゅ": "chu", "ちょ": "cho",
	"にゃ": "nya", "にゅ": "nyu", "にょ": "nyo",
	"ひゃ": "hya", "ひゅ": "hyu", "ひょ": "hyo",
	"みゃ": "mya", "みゅ": "myu", "みょ": "myo",
	"りゃ": "rya", "りゅ": "ryu", "りょ": "ryo",
	"ぎゃ": "gya", "ぎゅ": "gyu", "ぎょ": "gyo",
	"じゃ": "ja", "じゅ": "ju", "じょ": "jo",
	"びゃ": "bya", "びゅ": "byu", "びょ": "byo",
	"ぴゃ": "pya", "ぴゅ": "pyu", "ぴょ": "pyo",
	// Katakana basic
	"ア": "a", "イ": "i", "ウ": "u", "エ": "e", "オ": "o",
	"カ": "ka", "キ": "ki", "ク": "ku", "ケ": "ke", "コ": "ko",
	"サ": "sa", "シ": "shi", "ス": "su", "セ": "se", "ソ": "so",
	"タ": "ta", "チ": "chi", "ツ": "tsu", "テ": "te", "ト": "to",
	"ナ": "na", "ニ": "ni", "ヌ": "nu", "ネ": "ne", "ノ": "no",
	"ハ": "ha", "ヒ": "hi", "フ": "fu", "ヘ": "he", "ホ": "ho",
	"マ": "ma", "ミ": "mi", "ム": "mu", "メ": "me", "モ": "mo",
	"ヤ": "ya", "ユ": "yu", "ヨ": "yo",
	"ラ": "ra", "リ": "ri", "ル": "ru", "レ": "re", "ロ": "ro",
	"ワ": "wa", "ヲ": "wo", "ン": "n",
	// Katakana dakuten
	"ガ": "ga", "ギ": "gi", "グ": "gu", "ゲ": "ge", "ゴ": "go",
	"ザ": "za", "ジ": "ji", "ズ": "zu", "ゼ": "ze", "ゾ": "zo",
	"ダ": "da", "ヂ": "ji", "ヅ": "zu", "デ": "de", "ド": "do",
	"バ": "ba", "ビ": "bi", "ブ": "bu", "ベ": "be", "ボ": "bo",
	"パ": "pa", "ピ": "pi", "プ": "pu", "ペ": "pe", "ポ": "po",
	// Katakana yoon
	"キャ": "kya", "キュ": "kyu", "キョ": "kyo",
	"シャ": "sha", "シュ": "shu", "ショ": "sho",
	"チャ": "cha", "チュ": "chu", "チョ": "cho",
	"ニャ": "nya", "ニュ": "nyu", "ニョ": "nyo",
	"ヒャ": "hya", "ヒュ": "hyu", "ヒョ": "hyo",
	"ミャ": "mya", "ミュ": "myu", "ミョ": "myo",
	"リャ": "rya", "リュ": "ryu", "リョ": "ryo",
	"ギャ": "gya", "ギュ": "gyu", "ギョ": "gyo",
	"ジャ": "ja", "ジュ": "ju", "ジョ": "jo",
	"ビャ": "bya", "ビュ": "byu", "ビョ": "byo",
	"ピャ": "pya", "ピュ": "pyu", "ピョ": "pyo",
}

func kanaScriptLabel(kana string) string {
	for _, r := range kana {
		switch {
		case r >= 'ぁ' && r <= 'ゖ':
			return "히라가나"
		case r >= 'ァ' && r <= 'ヺ':
			return "가타카나"
		}
	}

	return "가나"
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

	rand.Seed(time.Now().UnixNano())

	kanaList := make([]string, 0, len(kanaMap))
	romajiList := make([]string, 0, len(kanaMap))
	for k, v := range kanaMap {
		kanaList = append(kanaList, k)
		romajiList = append(romajiList, v)
	}

	questions := make([]*model.Question, 0, len(kanaList)*3)

	// Type 1: Kana -> Romaji (Existing)
	for _, kana := range kanaList {
		romaji := kanaMap[kana]
		questions = append(questions, buildQuestion(kana, romaji, romajiList, true))
	}

	// Type 2: Romaji -> Kana (New)
	for _, kana := range kanaList {
		romaji := kanaMap[kana]
		questions = append(questions, buildQuestion(romaji, kana, kanaList, false))
	}

	// Type 3: Romaji -> Kana handwriting (Mini App)
	for _, kana := range kanaList {
		if !shouldSeedHandwritingQuestion(kana) {
			continue
		}
		romaji := kanaMap[kana]
		questions = append(questions, buildHandwritingQuestion(romaji, kana))
	}

	if err := repos.Question.CreateBatch(ctx, questions); err != nil {
		log.Printf("Failed to insert kana questions batch: %v", err)
		return
	}

	log.Printf("Successfully inserted %d Kana questions.", len(questions))
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
	isFillBlank := rand.Float32() < 0.7

	qType := model.QuestionFillBlank
	var options []string
	if !isFillBlank {
		qType = model.QuestionMultipleChoice
		options = append(options, answerVal)
		for len(options) < 4 {
			wrong := wrongPool[rand.Intn(len(wrongPool))]
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
		rand.Shuffle(len(options), func(i, j int) { options[i], options[j] = options[j], options[i] })
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

func buildHandwritingQuestion(romaji, kana string) *model.Question {
	scriptLabel := kanaScriptLabel(kana)
	prompt := fmt.Sprintf("발음 <b>'%s'</b>에 해당하는 %s 문자를 손글씨로 쓰세요", romaji, scriptLabel)

	return &model.Question{
		Type:             model.QuestionKanaHandwriting,
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
