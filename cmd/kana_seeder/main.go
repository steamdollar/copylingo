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
	"リャ": "rya", "リュ": "ryu", "리ョ": "ryo", // Typo fixed below
	"ギャ": "gya", "ギュ": "gyu", "ギョ": "gyo",
	"ジャ": "ja", "ジュ": "ju", "ジョ": "jo",
	"ビャ": "bya", "ビュ": "byu", "ビョ": "byo",
	"ピャ": "pya", "ピュ": "pyu", "ピョ": "pyo",
	// Fix typo in katakana yoon ('리' is Korean)
	"リョ": "ryo",
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
	
	romajiList := make([]string, 0, len(kanaMap))
	kanaList := make([]string, 0, len(kanaMap))
	for k, v := range kanaMap {
		if k == "리ョ" { continue } // skip typo key
		kanaList = append(kanaList, k)
		romajiList = append(romajiList, v)
	}

	totalInserted := 0
	for _, k := range kanaList {
		romaji := kanaMap[k]
		
		isFillBlank := rand.Float32() < 0.7 // 70% fill blank, 30% multiple choice

		qType := model.QuestionFillBlank
		var options []string
		if isFillBlank {
			options = []string{}
		} else {
			qType = model.QuestionMultipleChoice
			// Generate 3 random incorrect options
			options = append(options, romaji)
			for len(options) < 4 {
				wrong := romajiList[rand.Intn(len(romajiList))]
				// Ensure distinct
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
			// Shuffle options
			rand.Shuffle(len(options), func(i, j int) { options[i], options[j] = options[j], options[i] })
		}

		optBytes, _ := json.Marshal(options)

		prompt := fmt.Sprintf("다음 문자의 올바른 발음을 입력하세요: <b>%s</b>", k)
		if !isFillBlank {
			prompt = fmt.Sprintf("다음 문자의 올바른 발음을 고르시오: <b>%s</b>", k)
		}

		q := &model.Question{
			Type:             qType,
			Language:         "ja",
			ProficiencyLevel: "N5",
			Category:         "kana",
			Prompt:           prompt,
			Options:          optBytes,
			CorrectAnswer:    romaji,
			Explanation:      fmt.Sprintf("%s 발음은 '%s' 입니다.", k, romaji),
			Difficulty:       1,
		}

		if err := repos.Question.Create(ctx, q); err != nil {
			log.Printf("Failed to insert question for %s: %v", k, err)
		} else {
			totalInserted++
		}
	}

	log.Printf("Successfully inserted %d Kana questions.", totalInserted)
}
