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
  bell: svgWrap('<path d="M10.268 21a2 2 0 0 0 3.464 0"/><path d="M3.262 15.326A1 1 0 0 0 4 17h16a1 1 0 0 0 .74-1.673C19.41 13.956 18 12.499 18 8A6 6 0 0 0 6 8c0 4.499-1.411 5.956-2.738 7.326"/>'),
  bellOff: svgWrap('<path d="M10.268 21a2 2 0 0 0 3.464 0"/><path d="M17.658 17H4a1 1 0 0 1-.74-1.673C4.59 13.956 6 12.499 6 8a6 6 0 0 1 .258-1.742"/><path d="m2 2 20 20"/><path d="M8.668 3.01A6 6 0 0 1 18 8c0 2.687.77 4.653 1.707 6.05"/>'),
  bellRing: svgWrap('<path d="M10.268 21a2 2 0 0 0 3.464 0"/><path d="M22 8c0-2.3-.8-4.3-2-6"/><path d="M3.262 15.326A1 1 0 0 0 4 17h16a1 1 0 0 0 .74-1.673C19.41 13.956 18 12.499 18 8A6 6 0 0 0 6 8c0 4.499-1.411 5.956-2.738 7.326"/><path d="M4 2C2.8 3.7 2 5.7 2 8"/>'),
  check: svgWrap('<circle cx="12" cy="12" r="10"/><path d="m9 12 2 2 4-4"/>'),
  x: svgWrap('<path d="M18 6 6 18"/><path d="m6 6 12 12"/>'),
  chevron: svgWrap('<path d="m6 9 6 6 6-6"/>'),
  // loader spins via SMIL around the exact viewBox center — CSS transforms
  // on svg elements rotate off-axis in WebKit no matter the origin syntax
  loader: svgWrap('<g><animateTransform attributeName="transform" type="rotate" from="0 12 12" to="360 12 12" dur="1.1s" repeatCount="indefinite"/><path d="M21 12a9 9 0 1 1-6.219-8.56"/></g>'),
  clock: svgWrap('<circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>'),
  branch: svgWrap('<line x1="6" x2="6" y1="3" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/>'),
  pin: svgWrap('<path d="M12 17v5"/><path d="M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V6h1a2 2 0 0 0 0-4H8a2 2 0 0 0 0 4h1z"/>'),
  focus: svgWrap('<path d="M7 7h10v10"/><path d="M7 17 17 7"/>'),
  rotate: svgWrap('<path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/>'),
  github: svgWrap('<path d="M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.403 5.403 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4"/><path d="M9 18c-4.51 2-5-2-7-2"/>'),
  gear: svgWrap('<path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/>'),
  help: svgWrap('<circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/>'),
};

// ---- i18n: dictionaries load from /i18n/<lang>.json (shared with Go) ----
let lang = "ko";
let dict = {};
let dictEn = {};
const agentName = () => (currentTab === "codex" ? "Codex" : "Claude Code");
const t = (key) =>
  (dict[key] || dictEn[key] || key).replace("{agent}", agentName());

async function loadLang(l) {
  try {
    if (!Object.keys(dictEn).length) {
      dictEn = await (await fetch("/i18n/en.json")).json();
    }
    dict = l === "en" ? dictEn : await (await fetch("/i18n/" + l + ".json")).json();
  } catch (_) {}
}

function applyI18n() {
  document.documentElement.lang = lang;
  for (const el of document.querySelectorAll("[data-i18n]")) {
    el.textContent = t(el.dataset.i18n);
  }
  $("live-dot").dataset.tip = t("tip.connected");
  $("mute-btn").dataset.tip = t("tip.mute");
  $("quit-btn").dataset.tip = t("tip.quit");
  $("restart-btn").dataset.tip = t("tip.restart");
  $("help-btn").dataset.tip = t("tip.help");
  $("gh-btn").dataset.tip = t("tip.github");
  $("pin-btn").dataset.tip = t("tip.pin");
  $("settings-btn").dataset.tip = t("tip.settings");
  for (const opt of $("set-theme").options) {
    opt.textContent = t("theme." + opt.value);
  }
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
  const tpl = h >= 24 ? t("reset.day") : h > 0 ? t("reset.hour") : t("reset.min");
  return tpl
    .replace("{d}", Math.floor(h / 24))
    .replace("{h}", h >= 24 ? h % 24 : h)
    .replace("{m}", m);
}

function renderLimits(lim) {
  const body = $("limits-body");
  const note = $("limits-note");
  note.textContent = lim.error ? lim.error : lim.account || "";
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
      const tkey = "task:" + s.id;
      task.style.display = s.task ? "" : "none";
      const tChanged = task.dataset.txt !== (s.task || "");
      if (tChanged) {
        task.dataset.txt = s.task || "";
        task.textContent = s.task || "";
      }
      if (s.task) {
        const isOpen = expanded.has(tkey);
        task.classList.toggle("clamp", !isOpen);
        let btn = li.querySelector(".more-btn");
        if (tChanged || !btn) {
          const overflows = isOpen || task.scrollHeight > task.clientHeight + 1;
          if (overflows && !btn) {
            btn = document.createElement("button");
            btn.className = "more-btn" + (isOpen ? " open" : "");
            btn.innerHTML = "<span></span>" + ICONS.chevron;
            btn.querySelector("span").textContent = t(isOpen ? "events.less" : "events.more");
            btn.addEventListener("click", (e) => {
              e.stopPropagation();
              toggleMsg(task, btn, tkey, 1);
            });
            li.querySelector(".body").appendChild(btn);
          } else if (!overflows && btn && !isOpen) {
            btn.remove();
          }
        }
      } else {
        const btn = li.querySelector(".more-btn");
        if (btn) btn.remove();
      }
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

function toggleMsg(msg, btn, key, lines = 2) {
  const line = parseFloat(getComputedStyle(msg).lineHeight);
  const collapsedPx = Math.round(line * lines);
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
    if ((st.lang && st.lang !== lang) || !Object.keys(dict).length) {
      lang = st.lang || lang;
      await loadLang(lang);
      applyI18n();
    }
    renderSettings(st.settings);
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
    setText($("ver"), st.version ? "v" + st.version : "");
    if (st.updateAvail && (updState === "idle" || updState === "latest")) {
      updLatest = st.updateAvail;
      setUpd("available", t("update.available").replace("{v}", st.updateAvail));
    }
    tutMaybeAutoStart();
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

// ---- theme ----
let themeSetting = "auto";
const lightMQ = window.matchMedia("(prefers-color-scheme: light)");
function applyTheme() {
  const resolved =
    themeSetting === "light" || themeSetting === "dark"
      ? themeSetting
      : lightMQ.matches
        ? "light"
        : "dark";
  document.documentElement.dataset.theme = resolved;
}
lightMQ.addEventListener("change", applyTheme);
applyTheme();
$("set-theme").addEventListener("change", (e) => {
  themeSetting = e.target.value;
  applyTheme();
  post("/api/settings", { theme: themeSetting });
});

// ---- settings panel ----
let settingsOpen = false;
function toggleSettings(open) {
  settingsOpen = open === undefined ? !settingsOpen : open;
  $("settings-overlay").classList.toggle("hidden", !settingsOpen);
  $("settings-btn").classList.toggle("active", settingsOpen);
}
$("settings-btn").addEventListener("click", () => toggleSettings());
$("settings-close").addEventListener("click", () => toggleSettings(false));
$("settings-overlay").addEventListener("click", (e) => {
  if (e.target === $("settings-overlay")) toggleSettings(false);
});
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && settingsOpen) toggleSettings(false);
});

function renderSettings(cfg) {
  if (!cfg) return;
  const th = $("set-theme");
  if (document.activeElement !== th) th.value = cfg.theme || "auto";
  if ((cfg.theme || "auto") !== themeSetting) {
    themeSetting = cfg.theme || "auto";
    applyTheme();
  }
  const sel = $("lang-sel");
  if (document.activeElement !== sel) sel.value = cfg.lang || "auto";
  const tab = $("set-tab");
  if (document.activeElement !== tab) tab.value = cfg.defaultTab || "claude";
  $("sw-mute").classList.toggle("on", !!cfg.muted);
  $("sw-ai").classList.toggle("on", !cfg.disableAISummary);
  $("sw-live").classList.toggle("on", !cfg.disableLiveStatus);
  $("sw-update").classList.toggle("on", !cfg.disableAutoUpdate);
  $("sw-autostart").classList.toggle("on", !cfg.disableAutostart);
}

function bindSwitch(id, build) {
  $(id).addEventListener("click", () => {
    const el = $(id);
    el.classList.toggle("on");
    post("/api/settings", build(el.classList.contains("on")));
  });
}
bindSwitch("sw-mute", (on) => ({ muted: on }));
bindSwitch("sw-ai", (on) => ({ disableAISummary: !on }));
bindSwitch("sw-live", (on) => ({ disableLiveStatus: !on }));
bindSwitch("sw-update", (on) => ({ disableAutoUpdate: !on }));
bindSwitch("sw-autostart", (on) => ({ autostart: on }));
$("set-tab").addEventListener("change", (e) => post("/api/settings", { defaultTab: e.target.value }));

// ---- update footer ----
let updState = "idle"; // idle | checking | latest | available | applying | done
let updLatest = "";

function setUpd(state, label) {
  updState = state;
  const btn = $("upd-btn");
  btn.dataset.state = state;
  btn.querySelector("span").textContent = label;
}

$("upd-btn").addEventListener("click", async () => {
  if (updState === "checking" || updState === "applying" || updState === "done") return;
  if (updState === "available") {
    setUpd("applying", t("update.applying"));
    try {
      const r = await (await fetch("/api/update-apply", { method: "POST" })).json();
      if (r.error) setUpd("idle", t("update.fail"));
      else setUpd("done", r.restarted ? t("update.restarting") : t("update.next"));
    } catch (_) {
      // daemon likely restarted mid-response; that's success
      setUpd("done", t("update.restarting"));
    }
    return;
  }
  setUpd("checking", t("update.checking"));
  try {
    const r = await (await fetch("/api/update-check", { method: "POST" })).json();
    if (r.error) {
      setUpd("idle", t("update.fail"));
    } else if (r.available) {
      updLatest = r.latest;
      setUpd("available", t("update.available").replace("{v}", r.latest));
    } else {
      setUpd("latest", t("update.latest"));
      setTimeout(() => {
        if (updState === "latest") setUpd("idle", t("update.check"));
      }, 3000);
    }
  } catch (_) {
    setUpd("idle", t("update.fail"));
  }
});


// ---- onboarding tour: spotlight overlay, first run + help button ----
const TUT_STEPS = [
  { sel: "#tabs .seg", key: "s1" },
  { sel: "#limits-card", key: "s2" },
  { sel: "#usage-card", key: "s3" },
  {
    sel: "#events-card", key: "s4", subtab: "sessions",
    legend: [
      { cls: "spin work", icon: "loader", key: "tut.leg.working" },
      { cls: "spin tool", icon: "loader", key: "tut.leg.tool" },
      { cls: "wait", icon: "bellRing", key: "tut.leg.waiting" },
      { cls: "idle", icon: "check", key: "tut.leg.idle" },
    ],
  },
  { sel: "#events-card", key: "s5", subtab: "events" },
  { sel: "footer", key: "s6" },
];
let tutStep = -1;
let tutEls = null;

function tutBuild() {
  const hole = document.createElement("div");
  hole.id = "tut-hole";
  const card = document.createElement("div");
  card.id = "tut-card";
  card.innerHTML =
    '<h3></h3><p></p><ul class="tut-legend"></ul>' +
    '<div class="row"><span class="dots"></span><span class="btns">' +
    '<button class="text-btn" id="tut-skip"></button>' +
    '<button class="text-btn" id="tut-prev"></button>' +
    '<button class="tut-next" id="tut-next"></button></span></div>';
  document.body.append(hole, card);
  card.querySelector("#tut-skip").addEventListener("click", tutEnd);
  card.querySelector("#tut-prev").addEventListener("click", () => tutGo(tutStep - 1));
  card.querySelector("#tut-next").addEventListener("click", () => {
    if (tutStep >= TUT_STEPS.length - 1) tutEnd();
    else tutGo(tutStep + 1);
  });
  return { hole, card };
}

function tutGo(i) {
  const step = TUT_STEPS[i];
  if (!step) return tutEnd();
  tutStep = i;
  if (!tutEls) tutEls = tutBuild();
  // make sure the highlighted region is actually visible
  if (currentTab !== "claude") {
    currentTab = "claude";
    applyTab();
  }
  if (step.subtab && subTab !== step.subtab) {
    subTab = step.subtab;
    applySubTab();
  }
  const target = document.querySelector(step.sel);
  if (!target) return tutEnd();
  const r = target.getBoundingClientRect();
  const pad = 6;
  const { hole, card } = tutEls;
  hole.style.cssText =
    `display:block;left:${r.left - pad}px;top:${r.top - pad}px;` +
    `width:${r.width + pad * 2}px;height:${r.height + pad * 2}px;`;
  card.style.display = "block";
  card.querySelector("h3").textContent = t("tut." + step.key + ".t");
  card.querySelector("p").textContent = t("tut." + step.key + ".b");
  const legend = card.querySelector(".tut-legend");
  if (step.legend) {
    legend.innerHTML = step.legend
      .map(
        (l) =>
          '<li><span class="ic ' + l.cls + '">' + ICONS[l.icon] + "</span><span>" +
          t(l.key) + "</span></li>"
      )
      .join("");
    legend.style.display = "";
  } else {
    legend.style.display = "none";
  }
  card.querySelector(".dots").innerHTML = TUT_STEPS.map(
    (_, d) => '<i class="' + (d === i ? "on" : "") + '"></i>'
  ).join("");
  card.querySelector("#tut-skip").textContent = t("tut.skip");
  const prev = card.querySelector("#tut-prev");
  prev.textContent = t("tut.prev");
  prev.style.visibility = i === 0 ? "hidden" : "visible";
  card.querySelector("#tut-next").textContent =
    i === TUT_STEPS.length - 1 ? t("tut.done") : t("tut.next");
  // place the card above or below the hole, clamped to the window
  const ch = card.offsetHeight;
  let y = r.bottom + pad + 10;
  if (y + ch > window.innerHeight - 8) y = r.top - pad - ch - 10;
  card.style.top = Math.max(8, y) + "px";
}

function tutEnd() {
  tutStep = -1;
  if (tutEls) {
    tutEls.hole.style.display = "none";
    tutEls.card.style.display = "none";
  }
  try {
    localStorage.setItem("tutorialDone", "1");
  } catch (_) {}
}

function tutStart() {
  tutGo(0);
}

$("help-btn").addEventListener("click", tutStart);

function renderWelcomeChecks(setup) {
  const rows = [
    { ok: setup.hooks, key: "welcome.hooks" },
    { ok: setup.autostart, key: "welcome.autostart" },
    { ok: setup.terminalNotifier, key: "welcome.notifier", miss: "welcome.notifier.miss" },
    { ok: setup.claudeCLI, key: "welcome.cli", miss: "welcome.cli.miss" },
  ];
  $("welcome-checks").innerHTML = rows
    .map((r) => {
      const icon = r.ok ? ICONS.check : ICONS.bellOff;
      const cls = r.ok ? "ok" : "miss";
      const hint = !r.ok && r.miss ? "<small>" + t(r.miss) + "</small>" : "";
      return (
        '<li class="' + cls + '"><span class="ic">' + icon + "</span>" +
        "<div><span>" + t(r.key) + "</span>" + hint + "</div></li>"
      );
    })
    .join("");
}

function welcomeClose(startTour) {
  $("welcome-overlay").classList.add("hidden");
  if (startTour) tutStart();
  else tutEnd(); // marks tutorialDone so it never auto-shows again
}
$("welcome-start").addEventListener("click", () => welcomeClose(true));
$("welcome-skip").addEventListener("click", () => welcomeClose(false));

let tutAutoChecked = false;
function tutMaybeAutoStart() {
  if (tutAutoChecked) return;
  tutAutoChecked = true;
  let done = "1";
  try {
    done = localStorage.getItem("tutorialDone") || "";
  } catch (_) {}
  if (!done) {
    renderWelcomeChecks((lastState && lastState.setup) || {});
    setTimeout(() => $("welcome-overlay").classList.remove("hidden"), 500);
  }
}

$("lang-sel").addEventListener("change", (e) => post("/api/settings", { lang: e.target.value }));
$("mute-btn").addEventListener("click", () => post("/api/mute"));
$("clear-btn").addEventListener("click", () => post("/api/clear"));
$("readall-btn").addEventListener("click", () =>
  post("/api/read-all", { source: currentTab || "claude" })
);
$("quit-btn").addEventListener("click", () => post("/api/quit"));
$("restart-btn").addEventListener("click", () => {
  fetch("/api/restart", { method: "POST" }).catch(() => {});
  $("live-dot").classList.add("off"); // window dies with the daemon; dot for the beat before it does
});

$("quit-btn").innerHTML = ICONS.x;
$("help-btn").innerHTML = ICONS.help;
$("settings-btn").innerHTML = ICONS.gear;
$("settings-close").innerHTML = ICONS.x;
$("restart-btn").innerHTML = ICONS.rotate;
$("gh-btn").innerHTML = ICONS.github;
$("gh-btn").addEventListener("click", () => post("/api/github"));
$("mute-btn").innerHTML = ICONS.bell;
$("pin-btn").innerHTML = ICONS.pin;
applySubTab();
refresh();
setInterval(refresh, 2500);
