"use strict";

const $ = (id) => document.getElementById(id);

// ---- custom tooltip: near-instant, animated, viewport-clamped ----
const tipEl = document.createElement("div");
tipEl.id = "tooltip";
document.addEventListener("DOMContentLoaded", () => document.body.appendChild(tipEl));
let tipTimer = null;
let tipTarget = null;

function showTip(target) {
  const txt = target.dataset.tip;
  if (!txt) return;
  tipEl.textContent = txt;
  tipEl.classList.add("show");
  const r = target.getBoundingClientRect();
  const tw = tipEl.offsetWidth;
  const th = tipEl.offsetHeight;
  let x = r.left + r.width / 2 - tw / 2;
  x = Math.max(6, Math.min(x, window.innerWidth - tw - 6));
  let y = r.top - th - 9;
  tipEl.classList.toggle("below", y < 4);
  if (y < 4) y = r.bottom + 9;
  tipEl.style.left = x + "px";
  tipEl.style.top = y + "px";
  // caret tracks the hovered element's center even when clamped
  const ax = Math.max(10, Math.min(r.left + r.width / 2 - x, tw - 10));
  tipEl.style.setProperty("--ax", ax + "px");
}

document.addEventListener("mouseover", (e) => {
  const target = e.target.closest("[data-tip]");
  if (target === tipTarget) return;
  tipTarget = target;
  clearTimeout(tipTimer);
  if (!target) {
    tipEl.classList.remove("show");
    return;
  }
  tipTimer = setTimeout(() => showTip(target), 150);
});
document.addEventListener("mousedown", () => {
  clearTimeout(tipTimer);
  tipEl.classList.remove("show");
});


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
  chevron: svgWrap('<path d="m6 9 6 6 6-6"/>'),
  loader: svgWrap('<path d="M21 12a9 9 0 1 1-6.219-8.56"/>'),
  wrench: svgWrap('<path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>'),
  clock: svgWrap('<circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>'),
  branch: svgWrap('<line x1="6" x2="6" y1="3" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/>'),
  pin: svgWrap('<path d="M12 17v5"/><path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V6h1a2 2 0 0 0 0-4H8a2 2 0 0 0 0 4h1z"/>'),
  focus: svgWrap('<path d="M7 7h10v10"/><path d="M7 17 17 7"/>'),
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
    "events.readAll": "Mark all read",
    "sessions.tab": "Active sessions",
    "sessions.empty": "No active sessions",
    "sessions.hint": "Sessions appear here while {agent} is running",
    "state.working": "working",
    "state.waiting": "waiting for input",
    "state.idle": "idle",
    "time.ago": "ago",
    "events.empty": "No events yet",
    "events.hint": "You\u2019ll see events here when {agent} sessions finish",
    "bucket.five_hour": "5-hour session",
    "bucket.seven_day": "Weekly (all)",
    "bucket.seven_day_sonnet": "Weekly Sonnet",
    "bucket.seven_day_opus": "Weekly Opus",
    "reset.now": "resets now",
    "tip.connected": "daemon connected",
    "tip.mute": "mute/unmute notifications",
    "tip.quit": "quit",
    "tip.open": "open",
    "tip.pin": "Make this tab the default",
    "events.more": "Show more",
    "events.less": "Show less",
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
    "events.readAll": "모두 읽기",
    "sessions.tab": "활성 세션",
    "sessions.empty": "활성 세션이 없습니다",
    "sessions.hint": "{agent} 세션이 실행되면 여기에 표시됩니다",
    "state.working": "작업 중",
    "state.waiting": "입력 대기",
    "state.idle": "대기",
    "time.ago": "전",
    "events.empty": "아직 이벤트가 없습니다",
    "events.hint": "{agent} 세션이 완료되면 여기에 표시됩니다",
    "bucket.five_hour": "5시간 세션",
    "bucket.seven_day": "주간 전체",
    "bucket.seven_day_sonnet": "주간 Sonnet",
    "bucket.seven_day_opus": "주간 Opus",
    "reset.now": "리셋됨",
    "tip.connected": "데몬 연결됨",
    "tip.mute": "알림 끄기/켜기",
    "tip.quit": "종료",
    "tip.open": "열기",
    "tip.pin": "이 탭을 기본값으로",
    "events.more": "더보기",
    "events.less": "접기",
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
    "events.readAll": "全部已读",
    "sessions.tab": "活跃会话",
    "sessions.empty": "暂无活跃会话",
    "sessions.hint": "{agent} 会话运行时会显示在这里",
    "state.working": "工作中",
    "state.waiting": "等待输入",
    "state.idle": "空闲",
    "time.ago": "前",
    "events.empty": "暂无事件",
    "events.hint": "{agent} 会话完成后会显示在这里",
    "bucket.five_hour": "5小时会话",
    "bucket.seven_day": "每周（全部）",
    "bucket.seven_day_sonnet": "每周 Sonnet",
    "bucket.seven_day_opus": "每周 Opus",
    "reset.now": "已重置",
    "tip.connected": "守护进程已连接",
    "tip.mute": "开/关通知",
    "tip.quit": "退出",
    "tip.open": "打开",
    "tip.pin": "将此标签设为默认",
    "events.more": "展开",
    "events.less": "收起",
    reset: (d, h, m) =>
      d > 0 ? `${d}天${h}h后重置` : h > 0 ? `${h}h ${m}m后重置` : `${m}m后重置`,
  },
};

let lang = "ko";
const agentName = () => (currentTab === "codex" ? "Codex" : "Claude Code");
const t = (key) =>
  (I18N[lang][key] || I18N.en[key] || key).replace("{agent}", agentName());

function applyI18n() {
  document.documentElement.lang = lang;
  for (const el of document.querySelectorAll("[data-i18n]")) {
    el.textContent = t(el.dataset.i18n);
  }
  $("live-dot").dataset.tip = t("tip.connected");
  $("mute-btn").dataset.tip = t("tip.mute");
  $("quit-btn").dataset.tip = t("tip.quit");
}



function fmtTokens(n) {
  if (n >= 1e9) return (n / 1e9).toFixed(1) + "B";
  if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
  if (n >= 1e3) return (n / 1e3).toFixed(1) + "k";
  return String(n);
}

function fmtDur(sec) {
  if (sec < 60) return sec + "s";
  const m = Math.floor(sec / 60);
  if (m < 60) return m + "m";
  return Math.floor(m / 60) + "h " + (m % 60) + "m";
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
    bar.className = pct >= 90 ? "bad" : pct >= 70 ? "warn" : "";
    // Setting the width in the creation frame skips the transition; a
    // rAF hop lets the 0-width state paint first so the bar rises.
    requestAnimationFrame(() =>
      requestAnimationFrame(() => (bar.style.width = pct + "%"))
    );
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

// reconcile keeps DOM rows keyed and updates them in place — full
// innerHTML rebuilds every poll made the whole card flash and dropped
// hover/animation state.
function reconcile(list, items, keyFn, createFn, updateFn) {
  const byKey = new Map();
  for (const el of [...list.children]) byKey.set(el.dataset.key, el);
  let prev = null;
  items.forEach((it, i) => {
    const k = keyFn(it, i);
    let el = byKey.get(k);
    if (el) byKey.delete(k);
    else {
      el = createFn(it);
      el.dataset.key = k;
    }
    // insert before updating: updateFn may measure layout (overflow checks)
    const want = prev ? prev.nextSibling : list.firstChild;
    if (el !== want) list.insertBefore(el, want);
    updateFn(el, it, i);
    prev = el;
  });
  for (const el of byKey.values()) el.remove();
}

function setText(el, txt) {
  if (el.textContent !== txt) el.textContent = txt;
}

// ---- events-card sub-tabs (active sessions / events) ----
let subTab = "sessions";

function applySubTab() {
  for (const b of document.querySelectorAll(".subtab")) {
    b.classList.toggle("active", b.dataset.subtab === subTab);
  }
  $("sessions").classList.toggle("hidden", subTab !== "sessions");
  $("events").classList.toggle("hidden", subTab !== "events");
  $("events-actions").classList.toggle("hidden", subTab !== "events");
  if (lastState) {
    renderSessions(lastState.sessions || []);
    renderEvents(lastState.events || [], lastState.unread || {});
  }
}
document.querySelectorAll(".subtab").forEach((b) =>
  b.addEventListener("click", () => {
    if (b.dataset.subtab === subTab) return;
    subTab = b.dataset.subtab;
    applySubTab();
  })
);

function stateIcon(s) {
  if (s.state === "tool") return ['<span class="ic spin tool">' + ICONS.loader + "</span>", ""];
  if (s.state === "working") return ['<span class="ic spin work">' + ICONS.loader + "</span>", ""];
  if (s.state === "waiting") return ['<span class="ic wait">' + ICONS.bellRing + "</span>", ""];
  return ['<span class="ic idle">' + ICONS.check + "</span>", ""];
}

function sessionStateText(s) {
  if (s.state === "tool") return (s.tool || "tool");
  if (s.state === "working") return t("state.working");
  if (s.state === "waiting") return t("state.waiting");
  return t("state.idle");
}

function renderSessions(all) {
  const tab = currentTab || "claude";
  const sessions = all.filter((s) => (s.source || "claude") === tab);
  $("sess-badge").textContent = "";
  const busy = sessions.filter((s) => s.state === "tool" || s.state === "working").length;
  if (busy > 0) $("sess-badge").textContent = String(busy);
  if (subTab !== "sessions") return;
  const list = $("sessions");
  const empty = $("empty");
  empty.classList.toggle("hidden", sessions.length > 0);
  list.classList.toggle("hidden", sessions.length === 0);
  if (sessions.length === 0) {
    list.innerHTML = "";
    empty.querySelector("[data-i18n]").textContent = t("sessions.empty");
    empty.querySelectorAll("span")[1].textContent = t("sessions.hint");
    return;
  }

  reconcile(
    list,
    sessions,
    (s) => s.id,
    (s) => {
      const li = document.createElement("li");
      li.innerHTML =
        '<span class="ic"></span>' +
        '<div class="body">' +
        '<div class="head"><span class="name"></span><span class="branch"></span><span class="proj"></span></div>' +
        '<div class="meta"><span class="statetxt"></span><i class="sep">·</i>' +
        '<span class="elapsed" data-ts=""></span></div>' +
        '<div class="task"></div>' +
        "</div>";
      li.querySelector(".proj").addEventListener("click", (e) => {
        e.stopPropagation(); // chip opens the folder; the row focuses the window
        post("/api/folder", { id: li.dataset.sid });
      });
      li.addEventListener("click", () =>
        post("/api/focus-session", { id: li.dataset.sid })
      );
      return li;
    },
    (li, s) => {
      li.dataset.sid = s.id;
      li.className = "ev sess " + s.state;
      const ic = li.querySelector(".ic");
      if (ic.dataset.state !== s.state) {
        ic.dataset.state = s.state;
        if (s.state === "tool") { ic.className = "ic spin tool"; ic.innerHTML = ICONS.loader; }
        else if (s.state === "working") { ic.className = "ic spin work"; ic.innerHTML = ICONS.loader; }
        else if (s.state === "waiting") { ic.className = "ic wait"; ic.innerHTML = ICONS.bellRing; }
        else { ic.className = "ic idle"; ic.innerHTML = ICONS.check; }
      }
      setText(li.querySelector(".name"),
        s.title || (s.cwd || "").split(/[\\/]/).pop() || "claude");
      const br = li.querySelector(".branch");
      if (s.branch) {
        if (!br.firstChild) br.innerHTML = ICONS.branch + "<span></span>";
        setText(br.querySelector("span"), s.branch);
        br.style.display = "";
      } else br.style.display = "none";
      const proj = li.querySelector(".proj");
      if (!proj.firstChild) proj.innerHTML = ICONS.focus + "<span></span>";
      setText(proj.querySelector("span"), (s.cwd || "").split(/[\\/]/).pop() || "");
      proj.dataset.tip = s.cwd || "";
      setText(li.querySelector(".statetxt"), sessionStateText(s));
      const task = li.querySelector(".task");
      setText(task, s.task || "");
      task.style.display = s.task ? "" : "none";
      const el = li.querySelector(".elapsed");
      const busyState = s.state === "tool" || s.state === "working";
      el.dataset.ts = busyState ? s.turnStart : s.lastSeen;
      el.dataset.mode = busyState ? "run" : "ago";
    }
  );
  tickElapsed();
}

function tickElapsed() {
  for (const el of document.querySelectorAll(".elapsed")) {
    const ts = new Date(el.dataset.ts);
    if (isNaN(ts)) continue;
    const sec = Math.max(0, Math.floor((Date.now() - ts) / 1000));
    el.textContent =
      el.dataset.mode === "run" ? fmtDur(sec) : fmtDur(sec) + " " + t("time.ago");
  }
}
setInterval(tickElapsed, 1000);

// ---- tabs ----
let currentTab = null; // set from defaultTab on first state fetch
let defaultTab = "claude";
let lastState = null;

function updateInk() {
  const seg = document.querySelector("#tabs .seg");
  const ink = $("tab-ink");
  for (const b of seg.querySelectorAll(".tab")) {
    if (b.dataset.tab !== currentTab) continue;
    ink.style.transform = `translateX(${b.offsetLeft - 3}px)`;
    ink.style.width = b.offsetWidth + "px";
  }
}

function applyTab() {
  document.body.classList.toggle("tab-codex", currentTab === "codex");
  const seg = document.querySelector("#tabs .seg");
  for (const b of seg.querySelectorAll(".tab")) {
    b.classList.toggle("active", b.dataset.tab === currentTab);
  }
  updateInk();
  const pin = $("pin-btn");
  pin.classList.toggle("pinned", currentTab === defaultTab);
  pin.dataset.tip = t("tip.pin");
  if (lastState) {
    renderSessions(lastState.sessions || []);
    renderEvents(lastState.events || [], lastState.unread || {});
  }
}

document.querySelector("#tabs .seg").addEventListener("click", (e) => {
  const b = e.target.closest(".tab");
  if (!b || b.dataset.tab === currentTab) return;
  currentTab = b.dataset.tab;
  applyTab();
});
$("pin-btn").addEventListener("click", () => {
  if (currentTab) post("/api/tab", { tab: currentTab });
});

let knownNewest = null; // Time of newest event we've already rendered
const expanded = new Set(); // event keys the user opened; survives re-renders

const evKey = (ev) => (ev.session_id || "") + "|" + ev.time;

function toggleMsg(msg, btn, key) {
  const line = parseFloat(getComputedStyle(msg).lineHeight);
  const collapsedPx = Math.round(line * 2);
  if (expanded.has(key)) {
    // collapse: animate current height down to two lines, then re-clamp
    expanded.delete(key);
    btn.classList.remove("open");
    btn.querySelector("span").textContent = t("events.more");
    msg.style.maxHeight = msg.scrollHeight + "px";
    void msg.offsetHeight;
    msg.style.maxHeight = collapsedPx + "px";
    msg.addEventListener(
      "transitionend",
      () => {
        if (!expanded.has(key)) {
          msg.classList.add("clamp");
          msg.style.maxHeight = "";
        }
      },
      { once: true }
    );
  } else {
    // expand: unclamp, animate from two lines to full height
    expanded.add(key);
    btn.classList.add("open");
    btn.querySelector("span").textContent = t("events.less");
    msg.classList.remove("clamp");
    msg.style.maxHeight = collapsedPx + "px";
    void msg.offsetHeight;
    msg.style.maxHeight = msg.scrollHeight + "px";
    msg.addEventListener(
      "transitionend",
      () => {
        if (expanded.has(key)) msg.style.maxHeight = "";
      },
      { once: true }
    );
  }
}

function renderEvents(events, unread) {
  const tabUnread = (unread || {})[currentTab || "claude"] || 0;
  $("done-badge").textContent = tabUnread > 0 ? String(tabUnread) : "";
  if (subTab !== "events") return;
  const list = $("events");
  const empty = $("empty");
  empty.querySelector("[data-i18n]").textContent = t("events.empty");
  empty.querySelectorAll("span")[1].textContent = t("events.hint");

  const tab = currentTab || "claude";
  const shown = [];
  events.forEach((ev, i) => {
    if ((ev.source || "claude") === tab) shown.push([ev, i]);
  });
  empty.classList.toggle("hidden", shown.length > 0);
  list.classList.toggle("hidden", shown.length === 0);

  reconcile(
    list,
    shown,
    ([ev]) => evKey(ev),
    ([ev]) => {
      const li = document.createElement("li");
      if (knownNewest && ev.time > knownNewest) li.classList.add("new");
      li.innerHTML =
        '<span class="ic"></span>' +
        '<div class="body">' +
        '<div class="head"><span class="name"></span><span class="branch"></span><span class="proj"></span><span class="time"></span></div>' +
        '<div class="meta"></div>' +
        '<div class="msg clamp"></div>' +
        "</div>";
      li.querySelector(".proj").addEventListener("click", (e) => {
        e.stopPropagation(); // chip opens the folder; the row focuses the window
        post("/api/folder", { index: +li.dataset.index });
      });
      li.addEventListener("click", () => {
        li.classList.remove("unread"); // optimistic; server marks it too
        post("/api/open", { index: +li.dataset.index });
      });
      return li;
    },
    (li, [ev, idx]) => {
      li.dataset.index = idx;
      li.classList.toggle("unread", !ev.read);
      li.classList.add("ev");
      const key = evKey(ev);
      const ic = li.querySelector(".ic");
      const kindCls = "ic " + (ev.kind === "attention" ? "attn" : "done");
      if (ic.dataset.kind !== ev.kind) {
        ic.dataset.kind = ev.kind;
        ic.className = kindCls;
        ic.innerHTML = ev.kind === "attention" ? ICONS.bellRing : ICONS.check;
      }
      setText(li.querySelector(".name"),
        ev.title || (ev.cwd || "").split(/[\\/]/).pop() || "claude");
      const when = new Date(ev.time);
      setText(li.querySelector(".time"),
        when.getHours().toString().padStart(2, "0") + ":" +
        when.getMinutes().toString().padStart(2, "0"));
      const br = li.querySelector(".branch");
      if (ev.branch) {
        if (!br.firstChild) br.innerHTML = ICONS.branch + "<span></span>";
        setText(br.querySelector("span"), ev.branch);
        br.style.display = "";
      } else br.style.display = "none";
      const proj = li.querySelector(".proj");
      if (!proj.firstChild) proj.innerHTML = ICONS.focus + "<span></span>";
      setText(proj.querySelector("span"), (ev.cwd || "").split(/[\\/]/).pop() || "");
      proj.dataset.tip = ev.cwd || "";
      const meta = li.querySelector(".meta");
      const bits = [];
      if (ev.durSec > 0) bits.push(ICONS.clock + "<span>" + fmtDur(ev.durSec) + "</span>");
      if (ev.model) bits.push("<span>" + shortModel(ev.model) + "</span>");
      const metaHTML = bits.join('<i class="sep">·</i>');
      if (meta.dataset.html !== metaHTML) {
        meta.dataset.html = metaHTML;
        meta.innerHTML = metaHTML;
      }
      meta.style.display = metaHTML ? "" : "none";
      const msg = li.querySelector(".msg");
      if (ev.message) {
        msg.style.display = "";
        const changed = msg.dataset.txt !== ev.message;
        if (changed) {
          msg.dataset.txt = ev.message;
          msg.textContent = ev.message;
        }
        const isOpen = expanded.has(key);
        msg.classList.toggle("clamp", !isOpen);
        // (re)evaluate the show-more toggle when content changed
        let btn = li.querySelector(".more-btn");
        if (changed || !btn) {
          const overflows = isOpen || msg.scrollHeight > msg.clientHeight + 1;
          if (overflows && !btn) {
            btn = document.createElement("button");
            btn.className = "more-btn" + (isOpen ? " open" : "");
            btn.innerHTML = "<span></span>" + ICONS.chevron;
            btn.querySelector("span").textContent = t(isOpen ? "events.less" : "events.more");
            btn.addEventListener("click", (e) => {
              e.stopPropagation();
              toggleMsg(msg, btn, key);
            });
            li.querySelector(".body").appendChild(btn);
          } else if (!overflows && btn && !isOpen) {
            btn.remove();
          }
        }
      } else {
        msg.style.display = "none";
        const btn = li.querySelector(".more-btn");
        if (btn) btn.remove();
      }
    }
  );
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
    lastState = st;
    $("live-dot").classList.remove("off");
    defaultTab = st.defaultTab || "claude";
    if (currentTab === null) {
      currentTab = defaultTab;
      applyTab();
    } else {
      $("pin-btn").classList.toggle("pinned", currentTab === defaultTab);
    }
    if (st.lang && st.lang !== lang) {
      lang = st.lang;
      applyI18n();
    }
    const sel = $("lang-sel");
    const want = st.langSetting || "auto";
    if (document.activeElement !== sel && sel.value !== want) sel.value = want;
    renderLimits(st.limits || {});
    renderUsage(st.usage || { today: {}, week: {} });
    renderSessions(st.sessions || []);
    renderEvents(st.events || [], st.unread || {});
    let badgeChanged = false;
    for (const src of ["claude", "codex"]) {
      const b = $("badge-" + src);
      const n = (st.unread || {})[src] || 0;
      const txt = n > 0 ? String(n) : "";
      if (b.textContent !== txt) {
        b.textContent = txt;
        badgeChanged = true;
      }
    }
    // a badge appearing/disappearing changes the tab's width — resize the ink
    if (badgeChanged && currentTab) updateInk();
    $("plan-chip").textContent = (st.limits && st.limits.plan) || "";
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
$("readall-btn").addEventListener("click", () =>
  post("/api/read-all", { source: currentTab || "claude" })
);
$("quit-btn").addEventListener("click", () => post("/api/quit"));

$("quit-btn").innerHTML = ICONS.x;
$("mute-btn").innerHTML = ICONS.bell;
$("pin-btn").innerHTML = ICONS.pin;
applyI18n();
applySubTab();
refresh();
setInterval(refresh, 2500);
