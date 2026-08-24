const tg = window.Telegram?.WebApp;
const canvas = document.getElementById("pad");
const ctx = canvas.getContext("2d");
const clearButton = document.getElementById("clear");
const undoButton = document.getElementById("undo");
const eraserButton = document.getElementById("eraser");
const submitButton = document.getElementById("submit");
const overview = document.getElementById("overview");
const overviewCtx = overview.getContext("2d");
const statusEl = document.getElementById("status");
const gradeResult = document.getElementById("gradeResult");
const questionPanel = document.getElementById("questionPanel");
const questionPrompt = document.getElementById("questionPrompt");
const padViewport = document.getElementById("padViewport");
const padScrollControl = document.getElementById("padScrollControl");
const overviewPanel = document.getElementById("overviewPanel");
const padScroll = document.getElementById("padScroll");
const loadingPanel = document.getElementById("loadingPanel");
const loadingHeader = document.getElementById("loadingHeader");
const tipCard = document.getElementById("tipCard");
const tipEyebrow = document.getElementById("tipEyebrow");
const tipBody = document.getElementById("tipBody");
const params = new URLSearchParams(window.location.search);

// 캔버스 폭을 답안 글자 수(cells)에 비례시킨다. 화면 크기는 유지하고 내부 좌표 해상도만 2배로 올린다.
// 기존 패드 크기(224x280)의 약 85%로 줄여 작은 화면에서 도구 영역을 함께 보이게 한다.
const PAD_CELL_CSS_PX = 190;
const PAD_HEIGHT_CSS_PX = 238;
const PAD_SCALE = 2;
const PAD_CELL_PX = PAD_CELL_CSS_PX * PAD_SCALE;
const PAD_HEIGHT_PX = PAD_HEIGHT_CSS_PX * PAD_SCALE;
const PAD_MAX_CELLS = 8;
// 지우개 hit-test 반경(캔버스 좌표). 선 두께(10*PAD_SCALE)보다 약간 넉넉하게 잡아 손가락 오차를 흡수한다.
const ERASER_HIT_PX = 14 * PAD_SCALE;

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
	eraserMode: false,
	erasingActive: false,
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
setupDebugExport();

// 답안 글자 수만큼 캔버스 폭(=셀 개수, 셀당 4:5 비율)을 잡고 격자를 글자 단위로 맞춘다.
// canvas.width/height 재설정은 2D context 상태를 초기화하므로 stroke 속성은 그 뒤에 다시 적용한다.
function configurePad() {
	const parsed = Number.parseInt(params.get("cells") || "1", 10);
	const cells = Math.min(Math.max(Number.isFinite(parsed) ? parsed : 1, 1), PAD_MAX_CELLS);
	const width = PAD_CELL_PX * cells;

	canvas.width = width;
	canvas.height = PAD_HEIGHT_PX;
	canvas.style.width = `${PAD_CELL_CSS_PX * cells}px`;
	canvas.style.height = `${PAD_HEIGHT_CSS_PX}px`;
	canvas.style.backgroundSize = `${PAD_CELL_CSS_PX}px ${PAD_HEIGHT_CSS_PX}px`;

	ctx.lineWidth = 10 * PAD_SCALE;
	ctx.lineCap = "round";
	ctx.lineJoin = "round";
	ctx.strokeStyle = "#111811";

	// 미니맵은 메인 캔버스와 동일 해상도로 두고, CSS(width:100%)가 컨테이너 폭에 맞춰 축소한다.
	// background-size 를 셀 폭(%)으로 잡아 글자 단위 분할선을 표시한다.
	overview.width = width;
	overview.height = PAD_HEIGHT_PX;
	overview.style.backgroundSize = `${100 / cells}% 100%`;
	renderOverview();
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
	// 가로 넘침이 없으면 패드가 이미 전부 보이므로 미니맵을 숨겨 세로 공간을 아낀다.
	overviewPanel.hidden = maxScroll === 0;
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

function setGradeResult(message) {
  gradeResult.textContent = message;
  gradeResult.hidden = !message;
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
  renderOverview();
}

// state.strokes 를 단일 진실 소스로 캔버스 전체를 다시 그린다. undo/지우개/clear 후 호출.
function redrawAll() {
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  for (const stroke of state.strokes) {
    drawStroke(stroke);
  }
  renderOverview();
}

function drawStroke(stroke) {
  const pts = stroke.points;
  if (pts.length === 0) return;
  ctx.beginPath();
  ctx.moveTo(pts[0].x, pts[0].y);
  if (pts.length === 1) {
    // 점 하나짜리 획도 둥근 점으로 보이게 0 길이 선을 긋는다(lineCap: round).
    ctx.lineTo(pts[0].x, pts[0].y);
  } else {
    for (let i = 1; i < pts.length; i++) {
      ctx.lineTo(pts[i].x, pts[i].y);
    }
  }
  ctx.stroke();
}

// 메인 캔버스(획만, 배경 투명)를 미니맵에 1:1 복사한다. 격자/배경은 미니맵 CSS가 담당.
function renderOverview() {
  overviewCtx.clearRect(0, 0, overview.width, overview.height);
  overviewCtx.drawImage(canvas, 0, 0);
}

function distToSegment(px, py, a, b) {
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const len2 = dx * dx + dy * dy;
  if (len2 === 0) return Math.hypot(px - a.x, py - a.y);
  let t = ((px - a.x) * dx + (py - a.y) * dy) / len2;
  t = Math.max(0, Math.min(1, t));
  return Math.hypot(px - (a.x + t * dx), py - (a.y + t * dy));
}

function strokeHit(stroke, x, y, radius) {
  const pts = stroke.points;
  if (pts.length === 1) {
    return Math.hypot(pts[0].x - x, pts[0].y - y) <= radius;
  }
  for (let i = 1; i < pts.length; i++) {
    if (distToSegment(x, y, pts[i - 1], pts[i]) <= radius) return true;
  }
  return false;
}

// 위에 그려진 획(배열 뒤쪽)부터 검사해 가장 위 획을 지운다.
function eraseAt(event) {
  event.preventDefault();
  const point = pointFromEvent(event);
  for (let i = state.strokes.length - 1; i >= 0; i--) {
    if (strokeHit(state.strokes[i], point.x, point.y, ERASER_HIT_PX)) {
      state.strokes.splice(i, 1);
      redrawAll();
      tg?.HapticFeedback?.impactOccurred?.("light");
      return;
    }
  }
}

function undoStroke() {
  if (state.strokes.length === 0) {
    setStatus("되돌릴 획이 없습니다.");
    return;
  }
  state.strokes.pop();
  redrawAll();
  setStatus("마지막 획을 지웠습니다.");
}

function toggleEraser() {
  state.eraserMode = !state.eraserMode;
  state.drawing = false;
  state.currentStroke = null;
  eraserButton.classList.toggle("active", state.eraserMode);
  canvas.classList.toggle("erasing", state.eraserMode);
  setStatus(state.eraserMode ? "지우개: 지울 획을 터치하세요." : "다시 쓸 수 있습니다.");
}

function clearPad() {
  state.strokes = [];
  state.currentStroke = null;
  state.drawing = false;
  state.erasingActive = false;
  redrawAll();
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

  setToolsDisabled(true);
  setStatus("");
  setGradeResult("");
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

    const answerText = `정답: ${payload.correct_answer}`;
    if (payload.is_correct) {
      setGradeResult(`✅ ${answerText}`);
      setStatus("");
    } else {
      setGradeResult(`❌ ${answerText}`);
      setStatus(payload.feedback || "");
    }
    tg?.HapticFeedback?.notificationOccurred(payload.is_correct ? "success" : "error");
  } catch (error) {
    setStatus(error.message);
    setToolsDisabled(false);
  } finally {
    stopLoading({ keepTip: true });
  }
}

function setToolsDisabled(disabled) {
  submitButton.disabled = disabled;
  clearButton.disabled = disabled;
  undoButton.disabled = disabled;
  eraserButton.disabled = disabled;
}

// ?debug=1 일 때만: 서버 RenderPNG()와 정합성을 비교하기 위한 개발용 export 버튼을 단다.
// client.png(현재 canvas) + strokes.json(서버 입력과 동일한 stroke 배열)을 내보낸다.
// 운영 UI에는 노출하지 않는다. docs/todos/handwriting_rebuild_parity_verification.md 참고.
function setupDebugExport() {
  if (params.get("debug") !== "1") return;

  const bar = document.createElement("div");
  bar.style.cssText = "display:flex;gap:8px;margin-top:8px";

  const pngButton = document.createElement("button");
  pngButton.type = "button";
  pngButton.textContent = "debug: client.png";
  pngButton.addEventListener("click", exportClientPNG);

  const jsonButton = document.createElement("button");
  jsonButton.type = "button";
  jsonButton.textContent = "debug: strokes.json";
  jsonButton.addEventListener("click", exportStrokesJSON);

  bar.append(pngButton, jsonButton);
  document.body.append(bar);
}

function exportClientPNG() {
  canvas.toBlob((blob) => {
    if (!blob) return;
    downloadBlob(blob, "client.png");
  }, "image/png");
}

function exportStrokesJSON() {
  // 서버 RenderPNG()에 들어가는 stroke 배열을 그대로 직렬화한다.
  // canvas_width/height/line_width 는 사람이 정합성 맥락을 읽기 위한 메타데이터.
  const payload = {
    canvas_width: canvas.width,
    canvas_height: canvas.height,
    line_width: 10 * PAD_SCALE,
    strokes: state.strokes,
  };
  const blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" });
  downloadBlob(blob, "strokes.json");
}

function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}

canvas.addEventListener("pointerdown", (event) => {
  if (state.eraserMode) {
    state.erasingActive = true;
    eraseAt(event);
    return;
  }
  beginStroke(event);
});
canvas.addEventListener("pointermove", (event) => {
  if (state.eraserMode) {
    if (state.erasingActive) eraseAt(event);
    return;
  }
  moveStroke(event);
});
canvas.addEventListener("pointerup", (event) => {
  if (state.eraserMode) {
    state.erasingActive = false;
    return;
  }
  endStroke(event);
});
canvas.addEventListener("pointercancel", (event) => {
  if (state.eraserMode) {
    state.erasingActive = false;
    return;
  }
  endStroke(event);
});
clearButton.addEventListener("click", clearPad);
undoButton.addEventListener("click", undoStroke);
eraserButton.addEventListener("click", toggleEraser);
submitButton.addEventListener("click", submitAnswer);
padScroll.addEventListener("input", () => {
  padViewport.scrollLeft = Number(padScroll.value);
});
padViewport.addEventListener("scroll", () => {
  padScroll.value = String(padViewport.scrollLeft);
});
window.addEventListener("resize", updatePadScrollRange);
