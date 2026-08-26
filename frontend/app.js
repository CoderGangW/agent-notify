"use strict";

const $ = (id) => document.getElementById(id);

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
  if (isNaN(ms) || ms <= 0) return "리셋됨";
  const h = Math.floor(ms / 3600000);
  const m = Math.floor((ms % 3600000) / 60000);
  if (h >= 24) return Math.floor(h / 24) + "일 " + (h % 24) + "h 후 리셋";
  if (h > 0) return h + "h " + m + "m 후 리셋";
  return m + "m 후 리셋";
}

const BUCKET_NAMES = {
  five_hour: "5시간 세션",
  seven_day: "주간 전체",
  seven_day_sonnet: "주간 Sonnet",
  seven_day_opus: "주간 Opus",
};

function renderLimits(lim) {
  const body = $("limits-body");
  const note = $("limits-note");
  note.textContent = lim.error ? lim.error : "";
  if (!lim.buckets || lim.buckets.length === 0) {
    body.innerHTML =
      '<div class="limits-err">' +
      (lim.error || "한도 정보 없음") +
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
    el.querySelector(".name").textContent = BUCKET_NAMES[b.key] || b.key;
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
    li.title = ev.cwd + " 열기";
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

$("mute-btn").addEventListener("click", () => post("/api/mute"));
$("clear-btn").addEventListener("click", () => post("/api/clear"));
$("quit-btn").addEventListener("click", () => post("/api/quit"));

refresh();
setInterval(refresh, 2500);
