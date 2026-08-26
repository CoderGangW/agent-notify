"use strict";

const $ = (id) => document.getElementById(id);

// ---- i18n ----
const I18N = {
  en: {
    "limits.title": "Plan limits",
    "limits.none": "No limit data",
    "usage.title": "Token usage",
    "usage.local": "from local transcripts",
    "usage.today": "Today",
    "usage.week": "Last 7 days",
    "usage.out": "Output today",
    "usage.cache": "Cache read today",
    "events.title": "Session events",
    "events.clear": "Clear",
    "events.empty": "No events yet",
    "events.hint": "Finished Claude Code sessions show up here",
    "bucket.five_hour": "5-hour session",
    "bucket.seven_day": "Weekly (all)",
    "bucket.seven_day_sonnet": "Weekly Sonnet",
    "bucket.seven_day_opus": "Weekly Opus",
    "reset.now": "resets now",
    "tip.connected": "daemon connected",
    "tip.mute": "mute/unmute notifications",
    "tip.quit": "quit",
    "tip.open": "open",
    reset: (d, h, m) =>
      d > 0 ? `resets in ${d}d ${h}h` : h > 0 ? `resets in ${h}h ${m}m` : `resets in ${m}m`,
  },
  ko: {
    "limits.title": "플랜 한도",
    "limits.none": "한도 정보 없음",
    "usage.title": "토큰 사용량",
    "usage.local": "로컬 트랜스크립트 합산",
    "usage.today": "오늘",
    "usage.week": "최근 7일",
    "usage.out": "오늘 출력",
    "usage.cache": "오늘 캐시 읽기",
    "events.title": "세션 이벤트",
    "events.clear": "비우기",
    "events.empty": "아직 이벤트 없음",
    "events.hint": "Claude Code 세션이 끝나면 여기 뜸",
    "bucket.five_hour": "5시간 세션",
    "bucket.seven_day": "주간 전체",
    "bucket.seven_day_sonnet": "주간 Sonnet",
    "bucket.seven_day_opus": "주간 Opus",
    "reset.now": "리셋됨",
    "tip.connected": "데몬 연결됨",
    "tip.mute": "알림 끄기/켜기",
    "tip.quit": "종료",
    "tip.open": "열기",
    reset: (d, h, m) =>
      d > 0 ? `${d}일 ${h}h 후 리셋` : h > 0 ? `${h}h ${m}m 후 리셋` : `${m}m 후 리셋`,
  },
  zh: {
    "limits.title": "套餐限额",
    "limits.none": "无限额数据",
    "usage.title": "令牌用量",
    "usage.local": "本地转录汇总",
    "usage.today": "今日",
    "usage.week": "近7天",
    "usage.out": "今日输出",
    "usage.cache": "今日缓存读取",
    "events.title": "会话事件",
    "events.clear": "清空",
    "events.empty": "暂无事件",
    "events.hint": "Claude Code 会话结束后显示在这里",
    "bucket.five_hour": "5小时会话",
    "bucket.seven_day": "每周（全部）",
    "bucket.seven_day_sonnet": "每周 Sonnet",
    "bucket.seven_day_opus": "每周 Opus",
    "reset.now": "已重置",
    "tip.connected": "守护进程已连接",
    "tip.mute": "开/关通知",
    "tip.quit": "退出",
    "tip.open": "打开",
    reset: (d, h, m) =>
      d > 0 ? `${d}天${h}h后重置` : h > 0 ? `${h}h ${m}m后重置` : `${m}m后重置`,
  },
};

let lang = "ko";
const t = (key) => I18N[lang][key] || I18N.en[key] || key;

function applyI18n() {
  document.documentElement.lang = lang;
  for (const el of document.querySelectorAll("[data-i18n]")) {
    el.textContent = t(el.dataset.i18n);
  }
  $("live-dot").title = t("tip.connected");
  $("mute-btn").title = t("tip.mute");
  $("quit-btn").title = t("tip.quit");
}


// 8-ray Claude spark: long cardinals, short diagonals, tapered via inner
// valley points. Same geometry the app icon uses.
function sparkPath(cx, cy, rLong, rShort, rValley) {
  const pts = [];
  for (let i = 0; i < 8; i++) {
    const tip = (i * Math.PI) / 4;
    const r = i % 2 === 0 ? rLong : rShort;
    pts.push([cx + r * Math.cos(tip), cy + r * Math.sin(tip)]);
    const v = tip + Math.PI / 8;
    pts.push([cx + rValley * Math.cos(v), cy + rValley * Math.sin(v)]);
  }
  return (
    "M" + pts.map((p) => p[0].toFixed(2) + " " + p[1].toFixed(2)).join("L") + "Z"
  );
}
$("spark-path").setAttribute("d", sparkPath(19.4, 4.6, 4.2, 2.9, 1.15));

function fmtTokens(n) {
  if (n >= 1e9) return (n / 1e9).toFixed(1) + "B";
  if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
  if (n >= 1e3) return (n / 1e3).toFixed(1) + "k";
  return String(n);
}

function fmtReset(iso) {
  const ms = new Date(iso) - Date.now();
  if (isNaN(ms) || ms <= 0) return t("reset.now");
  const h = Math.floor(ms / 3600000);
  const m = Math.floor((ms % 3600000) / 60000);
  return I18N[lang].reset(Math.floor(h / 24), h >= 24 ? h % 24 : h, m);
}

function renderLimits(lim) {
  const body = $("limits-body");
  const note = $("limits-note");
  note.textContent = lim.error ? lim.error : "";
  if (!lim.buckets || lim.buckets.length === 0) {
    body.innerHTML =
      '<div class="limits-err">' +
      (lim.error || t("limits.none")) +
      "</div>";
    return;
  }
  for (const b of lim.buckets) {
    const id = "gauge-" + b.key;
    let el = $(id);
    if (!el) {
      el = document.createElement("div");
      el.className = "gauge";
      el.id = id;
      el.innerHTML =
        '<div class="row"><span class="name"></span><span class="meta"></span></div>' +
        '<div class="bar"><i></i></div>';
      body.appendChild(el);
    }
    const pct = Math.min(100, Math.max(0, b.utilization));
    el.querySelector(".name").textContent = t("bucket." + b.key) || b.key;
    el.querySelector(".meta").textContent =
      pct.toFixed(0) + "% · " + fmtReset(b.resetsAt);
    const bar = el.querySelector(".bar > i");
    bar.style.width = pct + "%";
    bar.className = pct >= 90 ? "bad" : pct >= 70 ? "warn" : "";
  }
}

function setStat(id, value) {
  const el = $(id);
  const txt = fmtTokens(value);
  if (el.textContent !== txt) {
    el.textContent = txt;
    el.classList.remove("tick");
    void el.offsetWidth; // restart animation
    el.classList.add("tick");
  }
}

function shortModel(m) {
  return m
    .replace(/^claude-/, "")
    .replace(/-\d{8}$/, "")
    .replace(/-(\d+)-(\d+)$/, " $1.$2");
}

function renderUsage(u) {
  setStat("u-today", u.today.input + u.today.output);
  setStat("u-week", u.week.input + u.week.output);
  setStat("u-out", u.today.output);
  setStat("u-cache", u.today.cacheRead);
  const models = $("models");
  models.innerHTML = "";
  for (const m of (u.todayByModel || []).slice(0, 4)) {
    const chip = document.createElement("span");
    chip.className = "chip";
    chip.innerHTML =
      "<b></b> " + fmtTokens(m.input + m.output);
    chip.querySelector("b").textContent = shortModel(m.model);
    models.appendChild(chip);
  }
}

let knownNewest = null; // Time of newest event we've already rendered

function renderEvents(events, done) {
  $("done-badge").textContent = done > 0 ? String(done) : "";
  const list = $("events");
  const empty = $("empty");
  empty.classList.toggle("hidden", events.length > 0);
  list.classList.toggle("hidden", events.length === 0);

  list.innerHTML = "";
  events.forEach((ev, i) => {
    const li = document.createElement("li");
    li.className = "ev" + (knownNewest && ev.time > knownNewest ? " new" : "");
    li.title = t("tip.open") + " " + ev.cwd;
    const when = new Date(ev.time);
    li.innerHTML =
      '<span class="ic"></span>' +
      '<div class="body">' +
      '<div class="head"><span class="name"></span><span class="time"></span></div>' +
      '<div class="proj"></div>' +
      '<div class="msg"></div>' +
      "</div>";
    li.querySelector(".ic").textContent = ev.kind === "attention" ? "🔔" : "✅";
    li.querySelector(".name").textContent =
      ev.title || (ev.cwd || "").split("/").pop() || "claude";
    li.querySelector(".time").textContent =
      when.getHours().toString().padStart(2, "0") +
      ":" +
      when.getMinutes().toString().padStart(2, "0");
    li.querySelector(".proj").textContent = ev.cwd || "";
    const msg = li.querySelector(".msg");
    if (ev.message) msg.textContent = ev.message;
    else msg.remove();
    li.addEventListener("click", () => post("/api/open", { index: i }));
    list.appendChild(li);
  });
  if (events.length > 0) knownNewest = events[0].time;
}

async function post(url, body) {
  try {
    await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : null,
    });
  } catch (_) {}
  refresh();
}

async function refresh() {
  try {
    const res = await fetch("/api/state");
    const st = await res.json();
    $("live-dot").classList.remove("off");
    if (st.lang && st.lang !== lang) {
      lang = st.lang;
      applyI18n();
    }
    const sel = $("lang-sel");
    const want = st.langSetting || "auto";
    if (document.activeElement !== sel && sel.value !== want) sel.value = want;
    renderLimits(st.limits || {});
    renderUsage(st.usage || { today: {}, week: {} });
    renderEvents(st.events || [], st.done || 0);
    const mute = $("mute-btn");
    mute.textContent = st.muted ? "🔕" : "🔔";
    mute.classList.toggle("muted", st.muted);
  } catch (_) {
    $("live-dot").classList.add("off");
  }
}

$("lang-sel").addEventListener("change", (e) => post("/api/lang", { lang: e.target.value }));
$("mute-btn").addEventListener("click", () => post("/api/mute"));
$("clear-btn").addEventListener("click", () => post("/api/clear"));
$("quit-btn").addEventListener("click", () => post("/api/quit"));

applyI18n();
refresh();
setInterval(refresh, 2500);
