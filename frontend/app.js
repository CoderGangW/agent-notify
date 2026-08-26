"use strict";

const $ = (id) => document.getElementById(id);


// ---- Lucide-style stroke icons (24x24, MIT) ----
const svgWrap = (inner) =>
  '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" ' +
  'stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' + inner + "</svg>";
const ICONS = {
  bell: svgWrap('<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/>'),
  bellOff: svgWrap('<path d="M8.7 3A6 6 0 0 1 18 8a21.3 21.3 0 0 0 .6 5.4"/><path d="M17.3 17.3H3s3-2 3-9a4.67 4.67 0 0 1 .3-1.7"/><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/><line x1="2" x2="22" y1="2" y2="22"/>'),
  bellRing: svgWrap('<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a1.94 1.94 0 0 0 3.4 0"/><path d="M4 2C2.8 3.7 2 5.7 2 8"/><path d="M22 8c0-2.3-.8-4.3-2-6"/>'),
  check: svgWrap('<circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/>'),
  x: svgWrap('<path d="M18 6 6 18"/><path d="m6 6 12 12"/>'),
};

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

// rollTo renders txt into el as per-character slots. A changed digit rolls
// like an odometer: increased digits exit up / enter from below, decreased
// digits exit down / enter from above. Non-digit chars (units, dots)
// follow the overall direction of the value.
function rollTo(el, txt, rawVal) {
  const old = el.dataset.txt !== undefined ? el.dataset.txt : el.textContent;
  if (old === txt) return;
  const prevVal = Number(el.dataset.val);
  const up = isNaN(prevVal) || rawVal >= prevVal; // overall direction
  el.dataset.txt = txt;
  el.dataset.val = String(rawVal);
  el.innerHTML = "";
  el.classList.add("roll");

  // Align by place value: compare characters from the right.
  const n = Math.max(old.length, txt.length);
  const slots = [];
  for (let i = n - 1; i >= 0; i--) {
    const oldCh = old[old.length - 1 - i];
    const newCh = txt[txt.length - 1 - i];
    const slot = document.createElement("span");
    slot.className = "slot";
    if (newCh === undefined) continue; // value got shorter; char just drops
    if (oldCh === newCh) {
      slot.textContent = newCh;
    } else {
      let dirUp = up;
      if (oldCh >= "0" && oldCh <= "9" && newCh >= "0" && newCh <= "9") {
        dirUp = newCh > oldCh;
      }
      const enter = document.createElement("span");
      enter.className = "ch " + (dirUp ? "in-up" : "in-down");
      enter.textContent = newCh;
      slot.appendChild(enter);
      if (oldCh !== undefined) {
        const exit = document.createElement("span");
        exit.className = "ch out " + (dirUp ? "out-up" : "out-down");
        exit.textContent = oldCh;
        exit.addEventListener("animationend", () => exit.remove(), { once: true });
        slot.appendChild(exit);
      }
    }
    slots.push(slot);
  }
  // The loop walks i = n-1 → 0, i.e. leftmost slot first: append in order.
  for (const s of slots) el.appendChild(s);
}

function setStat(id, value) {
  rollTo($(id), fmtTokens(value), value);
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
  // Diff the model chips in place: rebuilding them every poll replays
  // the enter animation and reads as flicker.
  const models = $("models");
  const want = (u.todayByModel || []).slice(0, 4);
  const seen = new Set();
  for (const m of want) {
    seen.add(m.model);
    let chip = models.querySelector(
      '.chip[data-model="' + CSS.escape(m.model) + '"]'
    );
    if (chip && chip.classList.contains("leave")) {
      chip.classList.remove("leave"); // came back mid-exit
      chip.classList.add("enter");
    }
    if (!chip) {
      chip = document.createElement("span");
      chip.className = "chip enter";
      chip.dataset.model = m.model;
      chip.innerHTML = "<b></b> <span class=\"val\"></span>";
      chip.querySelector("b").textContent = shortModel(m.model);
      models.appendChild(chip);
    }
    rollTo(chip.querySelector(".val"), fmtTokens(m.input + m.output), m.input + m.output);
  }
  for (const chip of [...models.querySelectorAll(".chip")]) {
    if (!seen.has(chip.dataset.model) && !chip.classList.contains("leave")) {
      chip.classList.remove("enter");
      chip.classList.add("leave");
      chip.addEventListener("animationend", () => chip.remove(), { once: true });
    }
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
    const ic = li.querySelector(".ic");
    ic.className = "ic " + (ev.kind === "attention" ? "attn" : "done");
    ic.innerHTML = ev.kind === "attention" ? ICONS.bellRing : ICONS.check;
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
    const mstate = st.muted ? "off" : "on";
    if (mute.dataset.state !== mstate) {
      mute.dataset.state = mstate;
      mute.innerHTML = st.muted ? ICONS.bellOff : ICONS.bell;
      mute.classList.toggle("muted", st.muted);
    }
  } catch (_) {
    $("live-dot").classList.add("off");
  }
}

$("lang-sel").addEventListener("change", (e) => post("/api/lang", { lang: e.target.value }));
$("mute-btn").addEventListener("click", () => post("/api/mute"));
$("clear-btn").addEventListener("click", () => post("/api/clear"));
$("quit-btn").addEventListener("click", () => post("/api/quit"));

$("quit-btn").innerHTML = ICONS.x;
$("mute-btn").innerHTML = ICONS.bell;
applyI18n();
refresh();
setInterval(refresh, 2500);
