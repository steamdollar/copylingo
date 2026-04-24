const tg = window.Telegram?.WebApp;
const canvas = document.getElementById("pad");
const ctx = canvas.getContext("2d");
const clearButton = document.getElementById("clear");
const submitButton = document.getElementById("submit");
const statusEl = document.getElementById("status");
const params = new URLSearchParams(window.location.search);

const state = {
  drawing: false,
  currentStroke: null,
  strokes: [],
};

tg?.ready();
tg?.expand();

ctx.lineWidth = 10;
ctx.lineCap = "round";
ctx.lineJoin = "round";
ctx.strokeStyle = "#111811";

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
  setStatus("채점 중입니다...");

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

    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      throw new Error(payload.error || "채점 요청에 실패했습니다.");
    }

    const prefix = payload.is_correct ? "정답입니다." : `오답입니다. 정답은 ${payload.correct_answer} 입니다.`;
    setStatus(`${prefix} ${payload.feedback || payload.explanation || ""}`.trim());
    tg?.HapticFeedback?.notificationOccurred(payload.is_correct ? "success" : "error");
  } catch (error) {
    setStatus(error.message);
    submitButton.disabled = false;
    clearButton.disabled = false;
  }
}

canvas.addEventListener("pointerdown", beginStroke);
canvas.addEventListener("pointermove", moveStroke);
canvas.addEventListener("pointerup", endStroke);
canvas.addEventListener("pointercancel", endStroke);
clearButton.addEventListener("click", clearPad);
submitButton.addEventListener("click", submitAnswer);
