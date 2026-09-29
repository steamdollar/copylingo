package bot

type botMessages struct {
	// Study cards and one-shot LLM questions.
	sessionPush                        string
	startButton                        string
	startFailed                        string
	alreadyCompleted                   string
	saveProgressFailed                 string
	loadProgressFailed                 string
	saveCompletionFailed               string
	completeFailed                     string
	sessionCompleted                   string
	noMaterials                        string
	autoCompleted                      string
	materialOrderNotFound              string
	studySettingsButton                string
	previousButton                     string
	completeButton                     string
	nextButton                         string
	askButton                          string
	llmQuestionActivationFailed        string
	currentMaterialQuestionUnavailable string
	materialNotFound                   string
	llmQuestionPrompt                  string
	quizLLMQuestionPrompt              string
	vocabularyReadingFormat            string
	vocabularyWritingFormat            string
	meaningFormat                      string
	partOfSpeechFormat                 string
	explanationFormat                  string
	exampleFormat                      string
	readingFormat                      string
	translationFormat                  string
	keyVocabularyFormat                string
	activationUnavailable              string
	modeActivated                      string
	cancelButton                       string
	cancelUnavailable                  string
	modeCancelled                      string
	emptyQuestion                      string
	questionUnavailable                string
	userUnavailable                    string
	answerGenerating                   string
	answerFailed                       string
	answerFormat                       string
	quizContextFormat                  string
	quizQuestionPromptFormat           string
	studyContextFormat                 string
	studyQuestionPromptFormat          string

	// Commands, menu, and statistics.
	unknownCommand          string
	welcomeMessage          string
	mainMenuFormat          string
	studyMenuButton         string
	reviewMenuButtonFormat  string
	statsMenuButton         string
	settingsMenuButton      string
	menuHomeButton          string
	statsCommandFailed      string
	statsOverviewFormat     string
	statsMenuFormat         string
	streakCommandFailed     string
	streakFormat            string
	helpMessage             string
	exitMessage             string
	studyCommandUsageFormat string
	studyCommandBuildFailed string
	studyCommandNoMaterials string
	studyCommandPushFailed  string
	testSessionBuildFailed  string
	testSessionNoQuestions  string
	testSessionPushFailed   string
	languageJapanese        string
	languageGreek           string
	languageEnglish         string
	languageKorean          string

	// Push schedule and timezone settings.
	settingsLoadFailed                 string
	settingsUserLoadFailed             string
	settingsTimezoneInvalid            string
	settingsTimezoneChanged            string
	settingsChangeFailed               string
	settingsNotificationsDisabled      string
	settingsNotificationTimeSetFormat  string
	settingsOverviewFormat             string
	settingsSlotButtonFormat           string
	settingsSlotPickerFormat           string
	settingsTimezoneFormat             string
	settingsMorningStudyLabel          string
	settingsMorningQuizLabel           string
	settingsEveningStudyLabel          string
	settingsEveningQuizLabel           string
	settingsTimezoneChangeButton       string
	settingsDisableNotificationsButton string
	settingsAllTimesButton             string
	settingsDefaultTimesButton         string
	settingsTimezoneSeoulButton        string
	settingsTimezoneTokyoButton        string
	settingsTimezoneNewYorkButton      string
	settingsTimezoneLosAngelesButton   string
	settingsTimezoneLondonButton       string
	settingsTimezoneUTCButton          string
	settingsDisabledSlotLabel          string
	settingsBackButton                 string

	// Linked material preferences.
	materialPreferenceNotice         string
	materialPreferenceSaveFailed     string
	materialPreferenceSavedFormat    string
	materialPreferenceLoadFailed     string
	linkedQuizMaterialTitle          string
	materialMaintenanceButtonLabel   string
	materialExcludedLabel            string
	materialQuizSessionRejected      string
	materialNotInSession             string
	materialPreferenceAppliedNotice  string
	materialPreferenceMenuFormat     string
	materialBackToCardButton         string
	materialAlreadyKnowLabel         string
	materialNormalLabel              string
	materialSettingsListButton       string
	materialPreferenceRestoredNotice string
	materialPreferencesHeadingFormat string
	materialPreferencesEmpty         string
	materialRestoreButtonFormat      string
	materialPageFormat               string
	materialPreferenceFailed         string

	// Quiz sessions and question rendering.
	sessionFetchFailed                  string
	activeSessionUnavailable            string
	pendingSessionsEmpty                string
	studySessionReadyFormat             string
	quizSessionReadyFormat              string
	reviewUserLoadFailed                string
	noReviewQuestions                   string
	reviewSessionBuildFailed            string
	reviewStartButton                   string
	reviewSessionFormat                 string
	handwritingLinkExpired              string
	handwritingLinkUpdated              string
	handwritingSubmitFirst              string
	sessionStatePrepareFailed           string
	questionAnswerPromptFormat          string
	correctAnswerResultFormat           string
	wrongAnswerResultFormat             string
	aiFeedbackFormat                    string
	subjectiveGradingUnavailable        string
	nextQuestionButton                  string
	resultsButton                       string
	questionCompleted                   string
	linkedMaterialSettingsButton        string
	questionFormat                      string
	reviewQuestionMarker                string
	handwritingURLUnavailable           string
	handwritingPrompt                   string
	handwritingAnswerButton             string
	handwritingNextButton               string
	handwritingSentNotice               string
	listeningAudioUnavailable           string
	listeningAnswerPrompt               string
	freeTextAnswerPrompt                string
	chatAnswerPrompt                    string
	sessionCompletionWrongAnswerHeading string
	sessionCompletionKanaAnswerFormat   string
	sessionCompletionAnswerFormat       string
	sessionCompletionFormat             string
	sessionPushFormat                   string
	sessionPushStudyLabel               string
	sessionPushReviewLabel              string

	// Labels reused across session views.
	unansweredLabel           string
	morningStudySessionLabel  string
	eveningReviewSessionLabel string
	reviewSessionLabel        string
	articleSessionLabel       string
	middayStudySessionLabel   string

	// Word order controls and the marker used to update its message.
	wordOrderEmptySelection  string
	wordOrderAssembledPrefix string
	wordOrderUndoButton      string
	wordOrderResetButton     string
	wordOrderSubmitButton    string
	alreadyAnswered          string
}

// User-facing copy and locale-specific LLM context instructions share one map.
var botMessagesByLocale = map[string]botMessages{
	botDefaultLocale: {
		sessionPush: "📚 <b>Study Session이 도착했습니다!</b>\n\n" +
			"현재 레벨에 맞춘 Study Material을 짧게 훑고 가세요.",
		startButton:          "▶️ 시작하기",
		startFailed:          "❌ Study Session을 시작하지 못했습니다.",
		alreadyCompleted:     "✅ 이미 완료한 Study Session입니다.",
		saveProgressFailed:   "❌ Study 진행 상태를 저장하지 못했습니다.",
		loadProgressFailed:   "❌ Study 진행 상태를 불러오지 못했습니다.",
		saveCompletionFailed: "❌ Study 완료 상태를 저장하지 못했습니다.",
		completeFailed:       "❌ Study Session을 완료하지 못했습니다.",
		sessionCompleted: "✅ <b>Study Session 완료!</b>\n\n" +
			"학습한 Material 이력이 저장됐습니다.",
		noMaterials:                        "⚠️ 표시할 Study Material이 없습니다.",
		autoCompleted:                      "✅ Study Session을 완료했습니다.",
		materialOrderNotFound:              "⚠️ Study Material 순서를 찾지 못했습니다.",
		studySettingsButton:                "⚙️ 학습 설정",
		previousButton:                     "← 이전",
		completeButton:                     "✅ 완료",
		nextButton:                         "다음 →",
		askButton:                          "🤖 질문",
		llmQuestionActivationFailed:        "❌ LLM 질문을 활성화할 수 없습니다.",
		currentMaterialQuestionUnavailable: "❌ 현재 Study Material의 질문을 준비할 수 없습니다.",
		materialNotFound:                   "❌ Study Material을 찾을 수 없습니다.",
		llmQuestionPrompt:                  "🤖 이 Study Material에 대해 궁금한 점을 입력해 주세요. 다음 메시지 1개를 AI에게 보냅니다.",
		quizLLMQuestionPrompt:              "🤖 이 문제에 대해 궁금한 점을 입력해 주세요. 다음 메시지 1개를 AI에게 보냅니다.",
		vocabularyReadingFormat:            "읽기: <b>%s</b>",
		vocabularyWritingFormat:            "표기: <b>%s</b>",
		meaningFormat:                      "의미: <b>%s</b>",
		partOfSpeechFormat:                 "품사: <b>%s</b>",
		explanationFormat:                  "설명: %s",
		exampleFormat:                      "예문: <b>%s</b>",
		readingFormat:                      "읽기: %s",
		translationFormat:                  "해석: %s",
		keyVocabularyFormat:                "핵심 어휘:\n%s",
		activationUnavailable:              "❌ LLM mode를 활성화할 수 없습니다.",
		modeActivated:                      "🤖 <b>LLM mode 활성화</b>\n질문을 입력해 주세요. 다음 메시지 1개를 AI에게 보냅니다.",
		cancelButton:                       "❌ 취소",
		cancelUnavailable:                  "❌ LLM mode를 취소할 수 없습니다.",
		modeCancelled:                      "✅ LLM mode를 취소했습니다.",
		emptyQuestion:                      "⚠️ 질문 내용이 비어 있습니다. /llm 으로 다시 시작해 주세요.",
		questionUnavailable:                "❌ LLM 질문 기능이 준비되지 않았습니다.",
		userUnavailable:                    "❌ 사용자 정보를 확인할 수 없습니다.",
		answerGenerating:                   "🤖 AI가 답변을 생성 중입니다...",
		answerFailed:                       "❌ AI 답변 생성 중 오류가 발생했습니다. 다시 질문하려면 /llm 을 입력해 주세요.",
		answerFormat:                       "<b>AI 답변</b>\n\n%s",
		quizContextFormat: `다음은 사용자가 방금 푼 일본어 학습 문제입니다.
[문제] %s
[정답] %s
[해설] %s
[사용자가 제출한 답] %s`,
		quizQuestionPromptFormat: "\n\n위 문제에 대한 사용자의 질문에 답해 주세요.\n[질문] %s",
		studyContextFormat: `다음은 사용자가 방금 보고 있는 일본어 Study Material입니다.
[카테고리] %s
[제목] %s
[학습 내용] %s`,
		studyQuestionPromptFormat: "\n\n위 Study Material에 대한 사용자의 질문에 답해 주세요.\n[질문] %s",

		unknownCommand: "❓ 알 수 없는 명령어입니다. /help 를 입력해 보세요.",
		welcomeMessage: `🎌 <b>CopyLingo에 오신 것을 환영합니다!</b>

일본어를 마스터하기 위한 여정을 시작합니다.
JLPT N5부터 N1까지, 매일 조금씩 실력을 키워갑니다.

📚 <b>학습 방식:</b>
• 매일 오전/오후 학습 세션이 전송됩니다
• 뉴스, 시험 대비 자료를 기반으로 문제가 생성됩니다
• 틀린 문제는 간격 반복(SRS)으로 자동 복습됩니다
• 주말에는 아티클 읽기 + AI 대화도 제공됩니다

/menu 를 눌러 시작하세요! 🚀`,
		mainMenuFormat:         "🎌 <b>CopyLingo</b>\n\n%s 스트릭: <b>%d일</b> 연속\n🌐 언어: <b>%s</b>\n📈 레벨: <b>%s</b>",
		studyMenuButton:        "📚 학습하기",
		reviewMenuButtonFormat: "🔄 복습하기 (%d개)",
		statsMenuButton:        "📊 내 통계",
		settingsMenuButton:     "⚙️ 설정",
		menuHomeButton:         "🏠 메뉴로",
		statsCommandFailed:     "❌ 통계를 불러오는 데 실패했습니다.",
		statsOverviewFormat: `📊 <b>학습 통계</b>

📅 오늘: %d문제 풀음 (정답률 %.0f%%)
🔥 스트릭: %d일 연속

<b>카테고리별 정답률:</b>
📝 어휘: %.0f%%
📖 문법: %.0f%%
🈲 한자: %.0f%%
📚 독해: %.0f%%
🎧 청해: %.0f%%`,
		statsMenuFormat: `📊 <b>학습 통계</b>

📅 오늘: %d문제 (정답률 %.0f%%)
🔥 스트릭: %d일

📝 어휘: %.0f%% | 📖 문법: %.0f%%
🈲 한자: %.0f%% | 📚 독해: %.0f%%
🎧 청해: %.0f%%`,
		streakCommandFailed: "❌ 스트릭 정보를 불러올 수 없습니다.",
		streakFormat:        "🔥 현재 스트릭: <b>%d일</b> 연속 학습 중!",
		helpMessage: `📖 <b>CopyLingo 도움말</b>

<b>명령어:</b>
/menu - 메인 메뉴
/study [개수] - Study Material 세션 즉시 생성
/llm - LLM 질문 mode 활성화
/stats - 학습 통계
/streak - 스트릭 확인
/settings - 알림 시각 및 시간대 설정
/exit - 현재 입력 취소 (세션은 보존, /menu 에서 재개)
/help - 도움말

<b>학습 흐름:</b>
1. 설정한 시각에 맞춰 맞춤형 학습/퀴즈 세션이 전송됩니다 (설정: /settings)
2. 인라인 버튼으로 문제를 풀어주세요
3. 틀린 문제는 SRS로 자동 복습됩니다
4. /menu → 복습하기로 수동 복습도 가능합니다`,
		exitMessage:             "🚪 현재 입력을 취소했습니다. /menu 에서 언제든 이어서 진행할 수 있어요.",
		studyCommandUsageFormat: "❓ 사용법: /study [1-%d]\n예: /study 20",
		studyCommandBuildFailed: "❌ Study Session 생성 중 오류가 발생했습니다.",
		studyCommandNoMaterials: "⚠️ 현재 학습 가능한 Study Material이 없습니다.",
		studyCommandPushFailed:  "❌ Study Session 발송에 실패했습니다.",
		testSessionBuildFailed:  "❌ 세션 생성 중 오류가 발생했습니다.",
		testSessionNoQuestions:  "⚠️ 현재 사용 가능한 문제가 없습니다. 컨텐츠가 수집되었는지 확인해 주세요.",
		testSessionPushFailed:   "❌ 세션 발송에 실패했습니다.",
		languageJapanese:        "일본어",
		languageGreek:           "그리스어",
		languageEnglish:         "영어",
		languageKorean:          "한국어",

		settingsLoadFailed:                "❌ 설정 정보를 불러오지 못했습니다.",
		settingsUserLoadFailed:            "❌ 사용자 정보를 불러오지 못했습니다.",
		settingsTimezoneInvalid:           "❌ 유효하지 않은 시간대입니다.",
		settingsTimezoneChanged:           "✅ 시간대가 변경되었습니다.",
		settingsChangeFailed:              "❌ 설정 변경에 실패했습니다.",
		settingsNotificationsDisabled:     "🔕 알림이 비활성화되었습니다.",
		settingsNotificationTimeSetFormat: "✅ %s(으)로 설정되었습니다.",
		settingsOverviewFormat: `⚙️ <b>푸시 알림 및 스케줄 설정</b>

현재 시간대: <b>%s</b>
각 슬롯별 알림 시각을 변경하거나 끌 수 있습니다.

🌅 %s: <b>%s</b>
📝 %s: <b>%s</b>
🌆 %s: <b>%s</b>
🌙 %s: <b>%s</b>

💡 30분 단위로 시각을 지정할 수 있습니다.`,
		settingsSlotButtonFormat:           "%s %s (%s)",
		settingsSlotPickerFormat:           "⚙️ <b>%s %s 시각 설정</b>\n\n현재 설정: <b>%s</b>\n원하는 시각을 선택하세요 (30분 단위):",
		settingsTimezoneFormat:             "🌍 <b>시간대(Timezone) 설정</b>\n\n현재 설정: <b>%s</b>\n알림 발송 기준이 되는 시간대를 선택하세요:",
		settingsMorningStudyLabel:          "오전 학습",
		settingsMorningQuizLabel:           "오전 퀴즈",
		settingsEveningStudyLabel:          "오후 학습",
		settingsEveningQuizLabel:           "저녁 퀴즈",
		settingsTimezoneChangeButton:       "🌍 시간대 변경",
		settingsDisableNotificationsButton: "🔕 알림 끄기 (OFF)",
		settingsAllTimesButton:             "🕒 전체 시간 (24h)",
		settingsDefaultTimesButton:         "🕒 기본 시간대",
		settingsTimezoneSeoulButton:        "🇰🇷 서울 (Asia/Seoul)",
		settingsTimezoneTokyoButton:        "🇯🇵 도쿄 (Asia/Tokyo)",
		settingsTimezoneNewYorkButton:      "🇺🇸 뉴욕 (America/New_York)",
		settingsTimezoneLosAngelesButton:   "🇺🇸 LA (America/Los_Angeles)",
		settingsTimezoneLondonButton:       "🇬🇧 런던 (Europe/London)",
		settingsTimezoneUTCButton:          "🌐 UTC",
		settingsDisabledSlotLabel:          "🔕 꺼짐",
		settingsBackButton:                 "⬅️ 설정 목록으로",

		materialPreferenceNotice:         "새로 만드는 세션부터 적용됩니다. 현재 세션은 그대로 진행합니다.",
		materialPreferenceSaveFailed:     "❌ 연결 자료 설정을 저장하지 못했습니다. 다시 시도해 주세요.",
		materialPreferenceSavedFormat:    "✅ %s로 설정했습니다.\n\n%s\n원래 Quiz 메시지에서 계속하세요.",
		materialPreferenceLoadFailed:     "❌ 연결 자료 설정을 불러오지 못했습니다. 다시 시도해 주세요.",
		linkedQuizMaterialTitle:          "이 문제에 연결된 학습 자료",
		materialMaintenanceButtonLabel:   "유지 복습",
		materialExcludedLabel:            "학습에서 제외",
		materialQuizSessionRejected:      "❌ 이 Study Session의 설정을 변경할 수 없습니다.",
		materialNotInSession:             "❌ 이 세션에 없는 학습 항목입니다.",
		materialPreferenceAppliedNotice:  "✅ 새로 만드는 세션부터 적용됩니다.",
		materialPreferenceMenuFormat:     "⚙️ <b>학습 설정</b>\n\n<b>%s</b>\n현재: %s\n\n유지 복습은 30일 간격으로 시작합니다. 정답이면 60→120→최대 180일, 오답이면 일반 학습으로 돌아갑니다.\n학습에서 제외: 새 학습·연결 퀴즈에서 제외합니다.\n\n%s",
		materialBackToCardButton:         "← 학습 카드로",
		materialAlreadyKnowLabel:         "이미 알아요 · 드물게 복습",
		materialNormalLabel:              "일반 학습",
		materialSettingsListButton:       "📚 이미 아는 항목 · 제외 목록",
		materialPreferenceRestoredNotice: "✅ 일반 학습으로 복원했습니다. 새로 만드는 세션부터 적용됩니다.",
		materialPreferencesHeadingFormat: "📚 <b>이미 아는 항목 · 제외 목록</b>\n\n%s\n\n",
		materialPreferencesEmpty:         "이 페이지에 설정한 항목이 없습니다.\n",
		materialRestoreButtonFormat:      "↩️ %d번 일반 학습으로 복원",
		materialPageFormat:               "\n%d페이지",
		materialPreferenceFailed:         "❌ 학습 설정을 처리하지 못했습니다. 다시 시도해 주세요.",

		sessionFetchFailed:                  "❌ 세션 정보를 불러오지 못했습니다. 잠시 후 다시 시도해 주세요.",
		activeSessionUnavailable:            "⚠️ 진행 중 세션 상태가 만료되었습니다. 새 세션을 다시 시작해 주세요.",
		pendingSessionsEmpty:                "📚 현재 대기 중인 학습 세션이 없습니다.\n다음 세션이 자동으로 전송될 예정입니다!",
		studySessionReadyFormat:             "☀️ <b>Study Session 준비됨</b>\n\n총 %d개 Material\n\n준비되면 시작 버튼을 누르세요!",
		quizSessionReadyFormat:              "📚 <b>학습 세션 준비됨</b>\n\n총 %d문제\n유형: %s\n\n준비되면 시작 버튼을 누르세요!",
		reviewUserLoadFailed:                "❌ 사용자 정보를 불러오는 데 실패했습니다.",
		noReviewQuestions:                   "✅ 복습할 문제가 없습니다! 훌륭합니다 🎉",
		reviewSessionBuildFailed:            "❌ 복습 세션 생성에 실패했습니다.",
		reviewStartButton:                   "▶️ 복습 시작",
		reviewSessionFormat:                 "🔄 <b>복습 세션</b>\n\n복습 대상: %d문제\n\n준비되면 시작!",
		handwritingLinkExpired:              "🔄 손글씨 링크가 만료되어 같은 문제를 새 링크로 다시 보냅니다.",
		handwritingLinkUpdated:              "🔄 손글씨 링크가 갱신되었습니다. 아래 버튼으로 다시 진행해 주세요.",
		handwritingSubmitFirst:              "✍️ 먼저 손글씨 답안을 제출해 주세요.",
		sessionStatePrepareFailed:           "❌ 세션 상태를 준비하지 못했습니다. 잠시 후 다시 시도해 주세요.",
		questionAnswerPromptFormat:          "📝 %s\n\n",
		correctAnswerResultFormat:           "✅ <b>정답!</b>\n\n%s",
		wrongAnswerResultFormat:             "❌ <b>오답</b>\n\n입력/선택: %s\n정답: <b>%s</b>\n\n%s",
		aiFeedbackFormat:                    "\n\n🤖 <b>AI 피드백:</b>\n%s",
		subjectiveGradingUnavailable:        "⚠️ 시스템 설정 문제로 현재 AI 주관식 채점이 불가능합니다. 임시로 오답 처리하고 넘어갑니다.",
		nextQuestionButton:                  "다음 문제 →",
		resultsButton:                       "📊 결과 보기",
		questionCompleted:                   "✅ 모든 문제를 풀었습니다!",
		linkedMaterialSettingsButton:        "⚙️ 연결 자료 설정",
		questionFormat:                      "📝 <b>문제 %d/%d</b>%s\n\n%s",
		reviewQuestionMarker:                " 🔄",
		handwritingURLUnavailable:           "\n\n⚠️ 손글씨 Mini App URL 설정이 필요합니다. `COPYLINGO_SERVER_PUBLIC_BASE_URL`을 설정해 주세요.",
		handwritingPrompt:                   "\n\n✍️ 아래 버튼을 눌러 화면에 글자를 써 주세요.\n제출 후 이 채팅으로 돌아와 다음 문제를 진행하면 됩니다.",
		handwritingAnswerButton:             "✍️ 손글씨로 답하기",
		handwritingNextButton:               "제출 후 다음 문제 →",
		handwritingSentNotice:               "✍️ 손글씨 문항을 새 메시지로 보냈습니다.",
		listeningAudioUnavailable:           "\n\n⚠️ 이 청해 문항의 음성을 준비하지 못했습니다.",
		listeningAnswerPrompt:               "\n\n🎧 위 음성을 듣고 정답을 선택하세요.",
		freeTextAnswerPrompt:                "\n\n⌨️ 정답을 자유롭게 텍스트로 입력해 주세요",
		chatAnswerPrompt:                    "\n\n⌨️ 채팅창에 답안을 입력해 주세요",
		sessionCompletionWrongAnswerHeading: "\n\n<b>틀린 문제:</b>\n",
		sessionCompletionKanaAnswerFormat:   "❌ %s (정답: %s)\n",
		sessionCompletionAnswerFormat:       "❌ %s → %s (정답: %s)\n",
		sessionCompletionFormat:             "🎉 <b>세션 완료!</b>\n\n정답률: <b>%d/%d (%.0f%%)</b>%s",
		sessionPushFormat:                   "%s <b>%s 세션이 도착했습니다!</b>\n\n아래 버튼을 눌러 시작하세요.",
		sessionPushStudyLabel:               "학습",
		sessionPushReviewLabel:              "복습",

		unansweredLabel:           "미응답",
		morningStudySessionLabel:  "🌅 오전 학습",
		eveningReviewSessionLabel: "🌙 오후 복습",
		reviewSessionLabel:        "🔄 복습",
		articleSessionLabel:       "📖 아티클",
		middayStudySessionLabel:   "☀️ 정오 학습",

		wordOrderEmptySelection:  "(아직 선택한 조각 없음)",
		wordOrderAssembledPrefix: "\n\n🧩 조립:",
		wordOrderUndoButton:      "↩️ 되돌리기",
		wordOrderResetButton:     "초기화",
		wordOrderSubmitButton:    "제출",
		alreadyAnswered:          "이미 답변한 문제입니다.",
	},
}
