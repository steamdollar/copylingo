-- CopyLingo Initial Schema (Multi-language Support)
-- 9 tables: users, contents, materials, user_material_progress, questions (with SRS), sessions, session_materials, session_questions, tips

-----------------------------------------------------------
-- users
-----------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id                  BIGINT PRIMARY KEY,                     -- Telegram user ID
    username            VARCHAR(255) NOT NULL DEFAULT '',
    language            VARCHAR(10) NOT NULL DEFAULT 'ja',      -- ISO 639-1: 'ja', 'el', 'en', etc.
    proficiency_level   VARCHAR(10) NOT NULL DEFAULT 'N5',      -- JLPT: N5-N1, CEFR: A1-C2
    streak_days         INT NOT NULL DEFAULT 0,
    streak_last_date    DATE,
    morning_session_time TIME NOT NULL DEFAULT '08:00',
    evening_session_time TIME NOT NULL DEFAULT '21:00',
    timezone            VARCHAR(50) NOT NULL DEFAULT 'Asia/Seoul'
);

-----------------------------------------------------------
-- contents (collected learning materials)
-----------------------------------------------------------
CREATE TABLE IF NOT EXISTS contents (
    id              SERIAL PRIMARY KEY,
    source_type     VARCHAR(20) NOT NULL,                       -- 'news' | 'exam_prep'
    source_url      TEXT NOT NULL DEFAULT '',
    title           TEXT NOT NULL,
    body            TEXT NOT NULL,
    language        VARCHAR(10) NOT NULL DEFAULT 'ja',          -- ISO 639-1
    proficiency_level VARCHAR(10) NOT NULL DEFAULT 'N5',        -- Target level
    difficulty      INT NOT NULL DEFAULT 1 CHECK (difficulty BETWEEN 1 AND 10),
    tags            TEXT[] NOT NULL DEFAULT '{}',
    is_article      BOOLEAN NOT NULL DEFAULT FALSE,
    collected_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source_url)                                         -- Prevent duplicate collection
);

-----------------------------------------------------------
-- materials (study concepts shown before quizzes)
-----------------------------------------------------------
CREATE TABLE IF NOT EXISTS materials (
    id                SERIAL PRIMARY KEY,
    material_key      VARCHAR(255) NOT NULL UNIQUE,              -- '{language}:{domain}:{stable_slug}'
    content_id        INT REFERENCES contents(id) ON DELETE SET NULL,
    category          VARCHAR(30) NOT NULL,                      -- model.MaterialCategory whitelist
    language          VARCHAR(10) NOT NULL,                      -- ISO 639-1
    proficiency_level VARCHAR(10) NOT NULL,                      -- JLPT: N5-N1, CEFR: A1-C2
    title             VARCHAR(512) NOT NULL,
    payload           JSONB NOT NULL DEFAULT '{}',
    difficulty        INT NOT NULL DEFAULT 1 CHECK (difficulty BETWEEN 1 AND 10),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-----------------------------------------------------------
-- user_material_progress (user-specific SRS state for study materials)
-----------------------------------------------------------
CREATE TABLE IF NOT EXISTS user_material_progress (
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    material_id      INT NOT NULL REFERENCES materials(id),
    ease_factor      DOUBLE PRECISION NOT NULL DEFAULT 2.5,
    interval_days    INT NOT NULL DEFAULT 0,
    repetitions      INT NOT NULL DEFAULT 0,
    next_review_at   TIMESTAMPTZ,
    last_studied_at  TIMESTAMPTZ,
    times_studied    INT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, material_id)
);

CREATE INDEX IF NOT EXISTS idx_user_material_progress_due
    ON user_material_progress(user_id, next_review_at)
    WHERE next_review_at IS NOT NULL;

-----------------------------------------------------------
-- questions (generated learning questions + SRS state)
-----------------------------------------------------------
CREATE TABLE IF NOT EXISTS questions (
    id              SERIAL PRIMARY KEY,
    content_id      INT REFERENCES contents(id) ON DELETE SET NULL,
    type            VARCHAR(30) NOT NULL,                       -- multiple_choice, fill_blank, etc.
    item_type       VARCHAR(64),                                -- model.QuestionItemType whitelist; nullable for legacy rows
    language        VARCHAR(10) NOT NULL DEFAULT 'ja',          -- ISO 639-1
    proficiency_level VARCHAR(10) NOT NULL DEFAULT 'N5',        -- Target level
    category        VARCHAR(30) NOT NULL,                       -- vocabulary, grammar, kanji, reading, listening
    prompt          TEXT NOT NULL,
    options         JSONB,                                      -- Array of option strings (for multiple choice)
    correct_answer  TEXT NOT NULL,
    explanation     TEXT NOT NULL DEFAULT '',
    audio_path      TEXT,                                       -- Path to TTS audio file
    difficulty      INT NOT NULL DEFAULT 1 CHECK (difficulty BETWEEN 1 AND 10),
    times_served    INT NOT NULL DEFAULT 0,
    times_correct   INT NOT NULL DEFAULT 0,
    -- SRS (SM-2) state
    ease_factor     DOUBLE PRECISION NOT NULL DEFAULT 2.5,
    interval_days   INT NOT NULL DEFAULT 0,
    repetitions     INT NOT NULL DEFAULT 0,
    next_review_at  TIMESTAMPTZ,
    last_reviewed_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_questions_next_review ON questions(next_review_at)
    WHERE next_review_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_questions_language_level ON questions(language, proficiency_level);

-----------------------------------------------------------
-- sessions (learning sessions)
-----------------------------------------------------------
CREATE TABLE IF NOT EXISTS sessions (
    id              SERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type            VARCHAR(20) NOT NULL,                       -- morning, evening, review, article, study
    mode            VARCHAR(20) NOT NULL,                       -- quiz | study (application-owned enum)
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',     -- pending, in_progress, completed, expired
    total_questions INT NOT NULL DEFAULT 0,
    correct_count   INT NOT NULL DEFAULT 0,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at);

-----------------------------------------------------------
-- session_materials (join: session <-> material + study progress)
-----------------------------------------------------------
CREATE TABLE IF NOT EXISTS session_materials (
    id              SERIAL PRIMARY KEY,
    session_id      INT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    material_id     INT NOT NULL REFERENCES materials(id),
    material_order  INT NOT NULL,
    studied_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (session_id, material_id),
    UNIQUE (session_id, material_order)
);

CREATE INDEX IF NOT EXISTS idx_session_materials_session_id ON session_materials(session_id);

-----------------------------------------------------------
-- session_questions (join: session <-> question + user answer)
-----------------------------------------------------------
CREATE TABLE IF NOT EXISTS session_questions (
    id              SERIAL PRIMARY KEY,
    session_id      INT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    question_id     INT NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    question_order  INT NOT NULL,
    is_review       BOOLEAN NOT NULL DEFAULT FALSE,
    user_answer     TEXT,                                       -- NULL = not answered yet
    is_correct      BOOLEAN                                     -- NULL = not answered yet
);

CREATE INDEX IF NOT EXISTS idx_session_questions_session_id ON session_questions(session_id);

-----------------------------------------------------------
-- tips (short learning notes shown while waiting for grading)
-----------------------------------------------------------
CREATE TABLE IF NOT EXISTS tips (
    id                SERIAL PRIMARY KEY,
    language          VARCHAR(10)  NOT NULL,                       -- ISO 639-1: 'ja', 'el', 'en'
    proficiency_level VARCHAR(10)  NOT NULL,                       -- JLPT: N5-N1, CEFR: A1-C2
    category          VARCHAR(64)  NOT NULL,                       -- model.TipCategory whitelist
    body              VARCHAR(500) NOT NULL,                       -- 1-2 short sentences

    source_model      VARCHAR(64),                                 -- e.g. 'gemini-2.5-flash'
    source_prompt_ver VARCHAR(32),                                 -- prompt template version tag
    is_active         BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tips_lang_level_active
    ON tips(language, proficiency_level) WHERE is_active;
