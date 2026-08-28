"use strict";

// ---- usage statistics modal ----
let statsData = null;
let statsGran = "day"; // default daily; wheel zooms hour ↔ day ↔ month
const GRANS = ["hour", "day", "month"];

function statsLabel(pt, gran) {
  if (gran === "hour") {
    const [d, h] = pt.t.split("T");
    return d.slice(5).replace("-", "/") + " " + h + ":00";
  }
  if (gran === "day") return pt.t.slice(5).replace("-", "/");
  return pt.t.replace("-", "/");
}

// short two-line tick labels so the axis stays readable when buckets are dense
function tickParts(pt, gran) {
  if (gran === "hour") {
    const [d, h] = pt.t.split("T");
    const [, m, dd] = d.split("-");
    return { main: h + ":00", sub: +m + "/" + +dd };
  }
  if (gran === "day") {
    const [, m, dd] = pt.t.split("-");
    return { main: +m + "/" + +dd };
  }
  return { main: pt.t.replace("-", "/") };
}

let statsPts = [];
// visible window per granularity; older buckets reachable by drag / shift-wheel
const VIEW = { hour: 24, day: 14, month: 12 };
let statsOff = 0; // buckets panned back from the latest

function renderStats(zoomDir, noAnim) {
  if (!statsData) return;
  for (const b of document.querySelectorAll("#stats-gran .subtab")) {
    b.classList.toggle("active", b.dataset.gran === statsGran);
  }
  const all =
    statsGran === "hour" ? statsData.hourly
    : statsGran === "day" ? statsData.daily
    : statsData.monthly;
  const vc = Math.min(VIEW[statsGran], all.length);
  statsOff = Math.max(0, Math.min(statsOff, all.length - vc));
  const pts = all.slice(all.length - vc - statsOff, all.length - statsOff);
  statsPts = pts;
  $("chart-latest").classList.toggle("show", statsOff > 0);
  const chart = $("chart");
  const xs = $("chart-x");
  const grid = $("chart-grid");
  $("chart-wrap").classList.remove("hovering");
  hotCol = null;
  chart.innerHTML = "";
  xs.innerHTML = "";
  grid.innerHTML = "";
  const max = Math.max(1, ...pts.map((p) => p.in + p.out));

  // faint reference lines at max and half-max
  for (const [topPct, v] of [[0, max], [50, max / 2]]) {
    const gl = document.createElement("i");
    gl.className = "gl";
    gl.style.top = topPct + "%";
    const gv = document.createElement("span");
    gv.className = "gv";
    gv.style.top = topPct + "%";
    gv.textContent = fmtTokens(Math.round(v));
    grid.append(gl, gv);
  }

  const grow = [];
  pts.forEach((p, i) => {
    const col = document.createElement("div");
    col.className = "cbar" + (p.in + p.out === 0 ? " zero" : "");
    col.innerHTML = '<i class="seg in"></i><i class="seg out"></i>';
    const [sin, sout] = col.children;
    const hIn = (p.in / max) * 100;
    const hOut = (p.out / max) * 100;
    if (noAnim) {
      // panning: bars must track the pointer frame-by-frame, no grow
      sin.style.transition = "none";
      sout.style.transition = "none";
      sin.style.height = hIn + "%";
      sout.style.height = hOut + "%";
    } else {
      const d = Math.min(i * 4, 160) + "ms";
      sin.style.transitionDelay = d;
      sout.style.transitionDelay = d;
      sin.style.height = "0%";
      sout.style.height = "0%";
      grow.push([sin, hIn, sout, hOut]);
    }
    chart.appendChild(col);
  });
  // double rAF: heights must paint at 0 first so the grow transition runs
  if (grow.length) requestAnimationFrame(() => requestAnimationFrame(() => {
    for (const [a, ha, b, hb] of grow) {
      a.style.height = ha + "%";
      b.style.height = hb + "%";
    }
  }));

  // width-aware ticks: labels only as dense as the panel can fit
  const w = chart.clientWidth || 320;
  const est = statsGran === "hour" ? 56 : statsGran === "day" ? 48 : 64;
  const step = Math.max(1, Math.ceil(pts.length / Math.max(3, Math.floor(w / est))));
  xs.classList.toggle("two", statsGran === "hour");
  for (let i = 0; i < pts.length; i += step) {
    const { main, sub } = tickParts(pts[i], statsGran);
    const lab = document.createElement("span");
    lab.textContent = main;
    if (sub) {
      const s = document.createElement("small");
      s.textContent = sub;
      lab.appendChild(s);
    }
    const c = ((i + 0.5) / pts.length) * 100;
    if (c < 6) {
      lab.classList.add("edge-l");
      lab.style.left = "0";
    } else if (c > 94) {
      lab.classList.add("edge-r");
      lab.style.left = "100%";
    } else {
      lab.style.left = c + "%";
    }
    xs.appendChild(lab);
  }

  if (zoomDir) {
    for (const el of [chart, xs]) {
      el.classList.remove("zoom-in", "zoom-out");
      void el.offsetWidth;
      el.classList.add("zoom-" + zoomDir);
    }
  }
}

function setGran(g) {
  if (g === statsGran || !GRANS.includes(g)) return;
  const dir = GRANS.indexOf(g) < GRANS.indexOf(statsGran) ? "in" : "out";
  statsGran = g;
  statsOff = 0;
  renderStats(dir);
}

// instant hover: highlight band + tooltip card that glides between buckets
let hotCol = null;
// drag / horizontal-wheel pan through older buckets
let dragX = null, dragOff = 0, panAcc = 0;
document.addEventListener("DOMContentLoaded", () => {
  const wrap = $("chart-wrap");

  wrap.addEventListener("mousedown", (e) => {
    if (e.button !== 0 || !statsPts.length || e.target.closest("#chart-latest")) return;
    dragX = e.clientX;
    dragOff = statsOff;
    wrap.classList.add("dragging");
    wrap.classList.remove("hovering");
    if (hotCol) { hotCol.classList.remove("hot"); hotCol = null; }
    e.preventDefault();
  });
  window.addEventListener("mousemove", (e) => {
    if (dragX === null) return;
    const r = $("chart").getBoundingClientRect();
    const bw = r.width / Math.max(1, statsPts.length);
    // dragging right pulls older buckets in from the left
    const d = Math.round((e.clientX - dragX) / bw);
    const next = dragOff + d;
    if (next !== statsOff) {
      statsOff = next; // renderStats clamps
      renderStats(null, true);
    }
  });
  window.addEventListener("mouseup", () => {
    if (dragX === null) return;
    dragX = null;
    wrap.classList.remove("dragging");
  });
  wrap.addEventListener("dblclick", () => {
    if (!statsOff) return;
    statsOff = 0;
    renderStats(null);
  });
  $("chart-latest").addEventListener("click", () => {
    statsOff = 0;
    renderStats(null);
  });

  wrap.addEventListener("mousemove", (e) => {
    const n = statsPts.length;
    if (!n || dragX !== null) return;
    const r = $("chart").getBoundingClientRect();
    let i = Math.floor(((e.clientX - r.left) / r.width) * n);
    i = Math.max(0, Math.min(n - 1, i));
    const p = statsPts[i];
    wrap.classList.add("hovering");
    const band = $("chart-band");
    band.style.left = (i / n) * 100 + "%";
    band.style.width = 100 / n + "%";
    const cols = $("chart").children;
    if (hotCol !== cols[i]) {
      if (hotCol) hotCol.classList.remove("hot");
      hotCol = cols[i];
      hotCol.classList.add("hot");
    }
    const tip = $("chart-tip");
    tip.innerHTML =
      '<div class="ct-t">' + statsLabel(p, statsGran) + "</div>" +
      '<div class="ct-r"><i class="sw out"></i>' + t("stats.out") +
      "<b>" + fmtTokens(p.out) + "</b></div>" +
      '<div class="ct-r"><i class="sw in"></i>' + t("stats.in") +
      "<b>" + fmtTokens(p.in) + "</b></div>";
    const tw = tip.offsetWidth;
    const cx = ((i + 0.5) / n) * r.width;
    tip.style.left = Math.max(2, Math.min(cx - tw / 2, r.width - tw - 2)) + "px";
  });
  wrap.addEventListener("mouseleave", () => {
    wrap.classList.remove("hovering");
    if (hotCol) {
      hotCol.classList.remove("hot");
      hotCol = null;
    }
  });
});

async function openStats() {
  $("stats-overlay").classList.remove("hidden");
  try {
    statsData = await (await fetch("/api/stats")).json();
  } catch (_) {}
  renderStats();
}
$("stats-btn").addEventListener("click", openStats);
$("stats-close").addEventListener("click", () => $("stats-overlay").classList.add("hidden"));
$("stats-overlay").addEventListener("click", (e) => {
  if (e.target === $("stats-overlay")) $("stats-overlay").classList.add("hidden");
});
document.querySelectorAll("#stats-gran .subtab").forEach((b) =>
  b.addEventListener("click", () => setGran(b.dataset.gran))
);
// wheel: vertical zooms hour ↔ day ↔ month, horizontal pans through time
let wheelLock = 0;
$("chart-wrap").addEventListener("wheel", (e) => {
  e.preventDefault();
  if (Math.abs(e.deltaX) > Math.abs(e.deltaY)) {
    const n = Math.max(1, statsPts.length);
    const bw = ($("chart").clientWidth || 320) / n;
    panAcc += e.deltaX;
    const d = Math.trunc(panAcc / bw);
    if (d) {
      panAcc -= d * bw;
      // swiping left (deltaX > 0) advances toward the latest bucket
      const next = Math.max(0, statsOff - d);
      if (next !== statsOff) {
        statsOff = next;
        renderStats(null, true);
      }
    }
    return;
  }
  const now = Date.now();
  if (now - wheelLock < 380) return;
  wheelLock = now;
  const i = GRANS.indexOf(statsGran);
  const j = e.deltaY < 0 ? Math.max(0, i - 1) : Math.min(GRANS.length - 1, i + 1);
  setGran(GRANS[j]);
}, { passive: false });
