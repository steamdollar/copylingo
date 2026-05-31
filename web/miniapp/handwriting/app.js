const tg = window.Telegram?.WebApp;
const canvas = document.getElementById("pad");
const ctx = canvas.getContext("2d");
const clearButton = document.getElementById("clear");
const submitButton = document.getElementById("submit");
const statusEl = document.getElementById("status");
const questionPanel = document.getElementById("questionPanel");
const questionPrompt = document.getElementById("questionPrompt");
const padViewport = document.getElementById("padViewport");
const padScrollControl = document.getElementById("padScrollControl");
const padScroll = document.getElementById("padScroll");
const loadingPanel = document.getElementById("loadingPanel");
const loadingHeader = document.getElementById("loadingHeader");
const tipCard = document.getElementById("tipCard");
const tipEyebrow = document.getElementById("tipEyebrow");
const tipBody = document.getElementById("tipBody");
const params = new URLSearchParams(window.location.search);

// 캔버스 폭을 답안 글자 수(cells)에 비례시킨다. 한 글자당 정사각형 셀 하나.
const PAD_CELL_PX = 320;
const PAD_HEIGHT_PX = 320;
const PAD_MAX_CELLS = 8;

const TIP_INTERVAL_MS = 15000;
const TIP_CACHE_TTL_MS = 24 * 60 * 60 * 1000;
const TIP_CACHE_PREFIX = "copylingo:handwriting:tips:";
const TIP_CATEGORY_DISPLAY = {
	kana_youon: "요음",
	kana_sokuon: "촉음",
	kana_dakuten: "탁점/반탁점",
	kana_chouon: "장음",
	kana_shape: "비슷한 모양",
	kana_stroke: "획순",
	kana_hira_vs_kata: "히라가나/가타카나",
};

const state = {
	drawing: false,
	currentStroke: null,
	strokes: [],
};

const tipState = {
	pool: [], // shuffled, in display order
	idx: 0,
	intervalId: null,
	loadingActive: false,
};

tg?.ready();
tg?.expand();
if (tg?.isVersionAtLeast?.("7.7")) {
	tg.disableVerticalSwipes?.();
}
configurePad();
renderQuestionPrompt();
updatePadScrollRange();
loadTips();

// 답안 글자 수만큼 캔버스 폭(=정사각형 셀 개수)을 잡고 격자를 글자 단위로 맞춘다.
// canvas.width/height 재설정은 2D context 상태를 초기화하므로 stroke 속성은 그 뒤에 다시 적용한다.
function configurePad() {
	const parsed = Number.parseInt(params.get("cells") || "1", 10);
	const cells = Math.min(Math.max(Number.isFinite(parsed) ? parsed : 1, 1), PAD_MAX_CELLS);
	const width = PAD_CELL_PX * cells;

	canvas.width = width;
	canvas.height = PAD_HEIGHT_PX;
	canvas.style.width = `${width}px`;
	canvas.style.height = `${PAD_HEIGHT_PX}px`;
	canvas.style.backgroundSize = `${PAD_CELL_PX}px ${PAD_CELL_PX}px`;

	ctx.lineWidth = 10;
	ctx.lineCap = "round";
	ctx.lineJoin = "round";
	ctx.strokeStyle = "#111811";
}

function renderQuestionPrompt() {
	const prompt = stripPromptHTML(params.get("prompt") || "").trim();
	if (!prompt) return;

	questionPrompt.textContent = prompt;
	questionPanel.hidden = false;
}

function stripPromptHTML(raw) {
	const template = document.createElement("template");
	template.innerHTML = raw;
	template.content.querySelectorAll("br").forEach((br) => br.replaceWith("\n"));
	return template.content.textContent || "";
}

function updatePadScrollRange() {
	const maxScroll = Math.max(padViewport.scrollWidth - padViewport.clientWidth, 0);
	padScroll.max = String(maxScroll);
	padScroll.value = String(Math.min(Number(padScroll.value), maxScroll));
	padScrollControl.hidden = maxScroll === 0;
}

async function loadTips() {
	const language = params.get("language");
	const level = params.get("level");
	if (!language || !level) return;

	const cacheKey = `${TIP_CACHE_PREFIX}${language}:${level}`;
	const cachedTips = readTipCache(cacheKey);
	if (cachedTips) {
		applyTips(cachedTips);
		return;
	}

	try {
		const res = await fetch(`/api/miniapp/tips?language=${encodeURIComponent(language)}&level=${encodeURIComponent(level)}&limit=30`);
		if (!res.ok) return;
		const tips = await res.json();
		if (!Array.isArray(tips)) {
			return;
		}
		writeTipCache(cacheKey, tips);
		applyTips(tips);
	} catch (_) {
		// graceful — tip 없이 spinner 만
	}
}

function readTipCache(cacheKey) {
	try {
		const raw = window.localStorage?.getItem(cacheKey);
		if (!raw) return null;
		const cached = JSON.parse(raw);
		if (!cached || Date.now() > cached.expires_at || !Array.isArray(cached.tips)) {
			window.localStorage?.removeItem(cacheKey);
			return null;
		}
		return cached.tips;
	} catch (_) {
		return null;
	}
}

function writeTipCache(cacheKey, tips) {
	try {
		window.localStorage?.setItem(cacheKey, JSON.stringify({
			expires_at: Date.now() + TIP_CACHE_TTL_MS,
			tips,
		}));
	} catch (_) {
		// cache best-effort
	}
}

function applyTips(tips) {
	tipState.pool = shuffle(tips);
	if (tipState.loadingActive) {
		startTipRotation();
	}
}

function shuffle(arr) {
	const a = arr.slice();
	for (let i = a.length - 1; i > 0; i--) {
		const j = Math.floor(Math.random() * (i + 1));
		[a[i], a[j]] = [a[j], a[i]];
	}
	return a;
}

function startLoading() {
	tipState.loadingActive = true;
	loadingPanel.hidden = false;
	loadingHeader.hidden = false;
	startTipRotation();
}

function startTipRotation() {
	if (tipState.pool.length === 0) {
		tipCard.hidden = true;
		return;
	}
	if (tipState.intervalId) {
		return;
	}
	tipState.idx = 0;
	renderCurrentTip();
	tipCard.hidden = false;
	tipState.intervalId = setInterval(() => {
		tipState.idx = (tipState.idx + 1) % tipState.pool.length;
		renderCurrentTip();
	}, TIP_INTERVAL_MS);
}

function renderCurrentTip() {
	const t = tipState.pool[tipState.idx];
	if (!t) return;
	tipCard.style.opacity = "0";
	requestAnimationFrame(() => {
		tipEyebrow.textContent = TIP_CATEGORY_DISPLAY[t.category] || t.category;
		tipBody.textContent = t.body;
		tipCard.style.opacity = "1";
	});
}

function stopLoading({ keepTip = false } = {}) {
	tipState.loadingActive = false;
	if (tipState.intervalId) {
		clearInterval(tipState.intervalId);
		tipState.intervalId = null;
	}
	loadingHeader.hidden = true;
	if (!keepTip || tipCard.hidden) {
		loadingPanel.hidden = true;
	}
}

function setStatus(message) {
  statusEl.textContent = message;
}

function pointFromEvent(event) {
  const rect = canvas.getBoundingClientRect();
  return {
    x: ((event.clientX - rect.left) / rect.width) * canvas.width,
    y: ((event.clientY - rect.top) / rect.height) * canvas.height,
    time_ms: Date.now(),
    drawing: state.drawing,
  };
}

function beginStroke(event) {
  event.preventDefault();
  state.drawing = true;
  state.currentStroke = { points: [] };
  const point = pointFromEvent(event);
  state.currentStroke.points.push(point);
  ctx.beginPath();
  ctx.moveTo(point.x, point.y);
}

function moveStroke(event) {
  if (!state.drawing || !state.currentStroke) return;
  event.preventDefault();
  const point = pointFromEvent(event);
  state.currentStroke.points.push(point);
  ctx.lineTo(point.x, point.y);
  ctx.stroke();
}

function endStroke(event) {
  if (!state.drawing || !state.currentStroke) return;
  event.preventDefault();
  state.drawing = false;
  if (state.currentStroke.points.length > 0) {
    state.strokes.push(state.currentStroke);
  }
  state.currentStroke = null;
}

function clearPad() {
  state.strokes = [];
  state.currentStroke = null;
  state.drawing = false;
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  setStatus("다시 쓸 준비가 됐습니다.");
}

async function submitAnswer() {
  const sessionID = Number(params.get("session_id"));
  const questionID = Number(params.get("question_id"));
  const initData = tg?.initData || "";

  if (!sessionID || !questionID) {
    setStatus("문항 정보가 없습니다. 텔레그램에서 다시 열어 주세요.");
    return;
  }
  if (!initData) {
    setStatus("Telegram 인증 정보가 없습니다. Mini App으로 다시 열어 주세요.");
    return;
  }
  if (state.strokes.length === 0) {
    setStatus("먼저 글자를 써 주세요.");
    return;
  }

  submitButton.disabled = true;
  clearButton.disabled = true;
  setStatus("");
  startLoading();

  try {
    const response = await fetch("/api/miniapp/handwriting/submit", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        init_data: initData,
        session_id: sessionID,
        question_id: questionID,
        strokes: state.strokes,
      }),
    });

    // TODO: 이거 타입 도입 가능한지 체크
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error(payload.error || "채점 요청에 실패했습니다.");
    }

    if (payload.is_correct) {
      setStatus("정답입니다.");
    } else {
      const prefix = `오답입니다. 정답은 ${payload.correct_answer} 입니다.`;
      setStatus(`${prefix} ${payload.feedback || ""}`.trim());
    }
    tg?.HapticFeedback?.notificationOccurred(payload.is_correct ? "success" : "error");
  } catch (error) {
    setStatus(error.message);
    submitButton.disabled = false;
    clearButton.disabled = false;
  } finally {
    stopLoading({ keepTip: true });
  }
}

canvas.addEventListener("pointerdown", beginStroke);
canvas.addEventListener("pointermove", moveStroke);
canvas.addEventListener("pointerup", endStroke);
canvas.addEventListener("pointercancel", endStroke);
clearButton.addEventListener("click", clearPad);
submitButton.addEventListener("click", submitAnswer);
padScroll.addEventListener("input", () => {
  padViewport.scrollLeft = Number(padScroll.value);
});
padViewport.addEventListener("scroll", () => {
  padScroll.value = String(padViewport.scrollLeft);
});
window.addEventListener("resize", updatePadScrollRange);
