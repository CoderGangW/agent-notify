"use strict";

// Session window: every session one agent left on disk (?agent=codex…),
// with filters, multi-select, protect/exclude and delete. cleaner.go scans
// and deletes (and re-checks running/protected itself); this page filters,
// sorts and asks.

// the agent shown; i18n.js t() resolves {agent} through it
let currentTab = new URLSearchParams(location.search).get("agent") || "claude";
document.body.dataset.agent = currentTab;
const isMac = /Mac/.test(navigator.platform);
const MOD = isMac ? "⌘" : "Ctrl";
const LRM = "‎"; // keeps "~/" in front inside head-truncating (rtl) path cells

let st = null; // last /api/cleaner/state
let filter = "all";
let query = "";
let sortKey = "mtime";
let sortDir = -1;
const selection = new Set();
let anchor = null; // shift-click / arrow-key origin
let visible = []; // filtered + sorted rows, in screen order
let busy = "";
let lastLoad = 0;
// ?tour=1: the onboarding tour's second leg (cleaner-tour.js) — mock
// sessions on screen, rescans and keyboard actions paused
let touring = new URLSearchParams(location.search).has("tour");

// ---------- formatting ----------
const tv = (key, vars) => {
  let s = t(key);
  for (const k in vars) s = s.split("{" + k + "}").join(vars[k]);
  return s;
};
const esc = (s) =>
  String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

function fmtBytes(n) {
  if (n < 1000) return n + " B";
  const units = ["KB", "MB", "GB", "TB"];
  let i = -1;
  do {
    n /= 1000;
    i++;
  } while (n >= 1000 && i < units.length - 1);
  return (n >= 100 ? n.toFixed(0) : n.toFixed(1)) + " " + units[i];
}
const total = (list) => list.reduce((a, s) => a + s.size, 0);
const countSize = (list) => tv("cl.countSize", { n: list.length, size: fmtBytes(total(list)) });

function short(p) {
  const h = st && st.home;
  if (h && (p === h || p.startsWith(h + "/") || p.startsWith(h + "\\"))) return "~" + p.slice(h.length);
  return p;
}
const ltr = (p) => LRM + p + LRM;
const baseName = (p) => p.split(/[\\/]/).filter(Boolean).pop() || p;
const oneLine = (s) => s.split(/\s+/).filter(Boolean).join(" ");
const shq = (s) => "'" + s.replace(/'/g, "'\\''") + "'";

let fmtLang = "";
let rtf, dtf;
function fmts() {
  if (fmtLang !== lang) {
    fmtLang = lang;
    rtf = new Intl.RelativeTimeFormat(lang, { numeric: "auto" });
    dtf = new Intl.DateTimeFormat(lang, { dateStyle: "medium", timeStyle: "short" });
  }
}
const RT = [["year", 31536000], ["month", 2592000], ["week", 604800], ["day", 86400], ["hour", 3600], ["minute", 60]];
function relTime(iso) {
  const ms = Date.parse(iso);
  if (!(ms > 0)) return "–";
  fmts();
  const sec = (ms - Date.now()) / 1000;
  for (const [u, n] of RT) if (Math.abs(sec) >= n) return rtf.format(Math.round(sec / n), u);
  return rtf.format(0, "second");
}
function fullTime(iso) {
  const ms = Date.parse(iso);
  if (!(ms > 0)) return "–";
  fmts();
  return dtf.format(ms);
}
// the two largest units: "3h 12m", "2d 4h"
function fmtDur(ms) {
  if (!(ms >= 0)) return "–";
  const m = Math.round(ms / 60000);
  const parts = m >= 1440 ? [[Math.floor(m / 1440), "day"], [Math.floor(m / 60) % 24, "hour"]]
    : m >= 60 ? [[Math.floor(m / 60), "hour"], [m % 60, "minute"]] : [[Math.max(1, m), "minute"]];
  return parts.filter(([n]) => n).map(([n, unit]) =>
    new Intl.NumberFormat(lang, { style: "unit", unit, unitDisplay: "short" }).format(n)).join(" ");
}

// ---------- session model ----------
const SRC = {
  cli: { icon: "terminal", color: "#3f9d6a" },
  vscode: { icon: "code", color: "#3d80d6" },
  desktop: { icon: "appWindow", color: "#8f5ee8" },
  sdk: { icon: "pkg", color: "#d9822b" },
  orphan: { icon: "ghost", color: "#8a8079" },
};
const SRC_OTHER = { icon: "dashed", color: "#8a8079" };
const LABELED = ["cli", "vscode", "desktop", "sdk", "orphan", "unknown"];
const srcId = (s) => (s.orphan ? "orphan" : s.source || "unknown");
const srcStyle = (id) => SRC[id] || SRC_OTHER;
const srcLabel = (id) =>
  id === "desktop" && currentTab === "antigravity" ? t("cl.src.ide") : LABELED.includes(id) ? t("cl.src." + id) : id;
const srcIcon = (id, cls) =>
  '<span class="src-ic ' + (cls || "") + '" style="--c:' + srcStyle(id).color + '">' + ICONS[srcStyle(id).icon] + "</span>";

const isProtected = (s) => s.pinned || !!s.excluded;
const deletable = (s) => !s.running && !isProtected(s);
const canResume = (s) => !s.running && !!(s.resume && s.resume.length);
// a live agent would write its own title back over the new one
const canRename = (s) => !!(st.can && st.can.rename) && !s.running && !s.orphan;
const hasKey = (k) => !!(dict[k] || dictEn[k]);
const titleOf = (s) => (s.orphan ? t("cl.orphanTitle") : s.title || t("cl.emptyTitle"));
function subOf(s) {
  const p = oneLine(s.prompt || "");
  return !p || p.startsWith(s.title) ? s.sid : p;
}
const searchText = (s) =>
  s._q || (s._q = [s.title, s.project, srcLabel(srcId(s)), s.sid, (s.prompt || "").slice(0, 300)].join(" ").toLowerCase());
function resumeCmd(s) {
  return st.os === "windows"
    ? 'cd /d "' + s.cwd + '" && ' + s.resume.join(" ")
    : "cd " + shq(s.cwd) + " && " + s.resume.map(shq).join(" ");
}

function matches(f, s) {
  switch (f) {
    case "all": return true;
    case "running": return s.running;
    case "protected": return isProtected(s);
    case "empty": return s.empty;
    case "orphan": return s.orphan;
    case "old": return Date.parse(s.mtime) < Date.now() - st.prefs.oldDays * 86400000;
    case "large": return s.size >= st.prefs.largeMB * 1e6;
  }
  if (f.startsWith("src:")) return !s.orphan && srcId(s) === f.slice(4);
  if (f.startsWith("proj:")) return !s.orphan && s.project === f.slice(5);
  return false;
}
const baseList = () => (st.prefs.hideExcluded ? st.sessions.filter((s) => !s.excluded) : st.sessions);
const selectedList = () => visible.filter((s) => selection.has(s.id));
function filterName(f) {
  if (f === "old") return tv("cl.f.old", { n: st.prefs.oldDays });
  if (f === "large") return tv("cl.f.large", { n: st.prefs.largeMB });
  return t("cl.f." + f);
}

// ---------- daemon calls ----------
async function api(url, body) {
  try {
    const r = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body || {}),
    });
    return r.ok ? r : null;
  } catch (_) {
    return null;
  }
}

let loading = null;
function load() {
  if (touring && st) return Promise.resolve(); // the tour's mock stays put
  if (!loading) {
    loading = (async () => {
      setBusy("cl.scanning");
      try {
        const next = await (await fetch("/api/cleaner/state?agent=" + encodeURIComponent(currentTab))).json();
        agents = [{ id: next.agent, name: next.name }];
        // dictionary first: renders below bake t() output into the DOM
        if (next.lang !== lang || !Object.keys(dict).length) {
          lang = next.lang || lang;
          await loadLang(lang);
          applyLang();
        }
        applyTheme(next.theme);
        document.title = t("cl.title");
        setText($("title"), t("cl.title"));
        st = next;
        st.sessions = st.sessions || [];
        st.prefs.excluded = st.prefs.excluded || [];
        if (touring) st.sessions = clTourSessions(); // before the first render
        lastLoad = Date.now();
      } catch (_) {}
      setBusy("");
      render();
      const boot = $("boot");
      if (boot && !boot.classList.contains("done")) {
        boot.classList.add("done");
        setTimeout(() => boot.remove(), 400);
      }
    })().finally(() => (loading = null));
  }
  return loading;
}

const setPinned = (list, on) => api("/api/cleaner/prefs", { pin: list.map((s) => s.sid), on }).then(load);
const setExcluded = (paths, on) => api("/api/cleaner/prefs", { exclude: paths, on }).then(load);
const setPref = (body) => api("/api/cleaner/prefs", body).then(load);

async function resume(s) {
  if (!(await api("/api/cleaner/resume", { id: s.id }))) flash(t("cl.err.resume"));
}
const reveal = (s) => api("/api/cleaner/reveal", { id: s.id });

// what the session's files say beyond its row (recent turns, model,
// branch), read when it's selected and kept until its mtime moves
const details = new Map(); // id → { mtime, d }
function detailOf(s) {
  const c = details.get(s.id);
  return c && c.mtime === s.mtime ? c.d : null;
}
async function fetchDetail(s) {
  const r = await api("/api/cleaner/detail", { id: s.id });
  const d = r ? await r.json().catch(() => null) : null;
  if (d) details.set(s.id, { mtime: s.mtime, d });
  return d;
}

async function doRename(s, title) {
  title = oneLine(title);
  if (!title || title === s.title) return;
  if (!(await api("/api/cleaner/rename", { agent: currentTab, id: s.id, title }))) return flash(t("cl.err.rename"));
  await load();
  flash(t("cl.renamed"));
}

function copy(text) {
  if (!navigator.clipboard) return;
  navigator.clipboard.writeText(text).then(() => flash(t("cl.copied")), () => {});
}

// ---------- render ----------
function render() {
  if (!st) return;
  const base = baseList();
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  const key = SORTS[sortKey];
  visible = base
    .filter((s) => matches(filter, s) && words.every((w) => searchText(s).includes(w)))
    .sort((a, b) => {
      const x = key(a), y = key(b);
      return (x < y ? -1 : x > y ? 1 : 0) * sortDir || Date.parse(b.mtime) - Date.parse(a.mtime);
    });
  // selection only ever covers rows on screen
  const ids = new Set(visible.map((s) => s.id));
  for (const id of [...selection]) if (!ids.has(id)) selection.delete(id);
  renderSide(base);
  renderHead();
  renderRows();
  renderSel();
  renderPrefs();
}

// selection changes skip the list rebuild: row classes + side panels only
function renderSel() {
  for (const r of $("rows-in").children) r.classList.toggle("on", selection.has(r.dataset.key));
  renderInsp();
  renderStatus();
  renderToolbar();
}

const VIEWS = [["all", "layers"], ["running", "zap"], ["protected", "lock"], ["empty", "dashed"],
  ["orphan", "ghost"], ["old", "clock"], ["large", "hardDrive"]];

function renderSide(base) {
  const exOf = new Map();
  for (const s of base) if (s.excluded) exOf.set(s.project, s.excluded);
  const items = [{ h: "cl.sec.views" }];
  for (const [f, icon] of VIEWS) if (f !== "orphan" || currentTab === "claude") items.push({ f, icon });
  const srcs = [...new Set(base.filter((s) => !s.orphan).map(srcId))].sort();
  // one source says nothing: no sidebar section, no column
  $("app").classList.toggle("one-src", srcs.length < 2);
  if (srcs.length > 1) {
    items.push({ h: "cl.sec.sources" });
    for (const id of srcs) items.push({ f: "src:" + id, src: id });
  }
  const projs = new Map();
  for (const s of base) if (!s.orphan) projs.set(s.project, (projs.get(s.project) || 0) + s.size);
  if (projs.size) {
    items.push({ h: "cl.sec.projects" });
    for (const [p] of [...projs].sort((a, b) => b[1] - a[1])) items.push({ f: "proj:" + p, proj: p, ex: exOf.get(p) });
  }
  reconcile(
    $("side"),
    items,
    (it) => it.h || it.f,
    (it) => {
      if (it.h) {
        const h = document.createElement("div");
        h.className = "side-h";
        return h;
      }
      const b = document.createElement("button");
      b.className = "side-row";
      b.innerHTML = '<span class="ic"></span><span class="nm"></span><span class="sz"></span><span class="ct"></span>';
      return b;
    },
    (el, it) => {
      if (it.h) return setText(el, t(it.h));
      const m = base.filter((s) => matches(it.f, s));
      const icon = it.src ? srcStyle(it.src).icon : it.proj ? (it.ex ? "folderMinus" : "folder") : it.icon;
      const ic = el.querySelector(".ic");
      if (ic.dataset.icon !== icon) {
        ic.dataset.icon = icon;
        ic.innerHTML = ICONS[icon];
      }
      ic.style.color = it.src ? srcStyle(it.src).color : "";
      el.classList.toggle("on", filter === it.f);
      el.classList.toggle("excluded", !!it.ex);
      setText(el.querySelector(".nm"), it.src ? srcLabel(it.src) : it.proj ? baseName(it.proj) : filterName(it.f));
      setText(el.querySelector(".sz"), m.length ? fmtBytes(total(m)) : "");
      setText(el.querySelector(".ct"), String(m.length));
      const tip = it.proj ? short(it.proj) + (it.ex ? " · " + t("cl.badge.excluded") : "") : "";
      if (el.dataset.tip !== tip) el.dataset.tip = tip;
    }
  );
}

const COLS = [["title", "cl.col.session"], ["project", "cl.col.project"], ["source", "cl.col.source"],
  ["mtime", "cl.col.mtime"], ["size", "cl.col.size"]];
const SORTS = {
  title: (s) => titleOf(s).toLowerCase(),
  project: (s) => (s.orphan ? "￿" : s.project.toLowerCase()),
  source: (s) => srcLabel(srcId(s)).toLowerCase(),
  mtime: (s) => Date.parse(s.mtime) || 0,
  size: (s) => s.size,
};

function buildHead() {
  $("thead").innerHTML = COLS.map(
    ([k, label]) =>
      '<button data-sort="' + k + '"' + (k === "size" ? ' class="num"' : "") + "><span>" + esc(t(label)) +
      '</span><span class="arr">' + ICONS.chevron + "</span></button>"
  ).join("");
}
function renderHead() {
  for (const b of $("thead").children) {
    b.classList.toggle("on", b.dataset.sort === sortKey);
    b.classList.toggle("asc", b.dataset.sort === sortKey && sortDir > 0);
  }
}

function createRow() {
  const r = document.createElement("div");
  r.className = "row";
  r.innerHTML =
    '<div class="c-title"><span class="src-ic"></span><div class="tt"><div class="t1"><span class="name"></span>' +
    '<span class="bd"></span></div><div class="t2"></div></div></div>' +
    '<div class="c-proj"></div><div class="c-src"></div><div class="c-time"></div><div class="c-size"></div>';
  r.addEventListener("transitionend", () => r.classList.contains("leave") && r.remove());
  return r;
}
function updateRow(r, s) {
  r.classList.remove("leave");
  r.classList.toggle("on", selection.has(s.id));
  const id = srcId(s);
  const sty = srcStyle(id);
  const ic = r.querySelector(".src-ic");
  if (ic.dataset.src !== id) {
    ic.dataset.src = id;
    ic.style.setProperty("--c", sty.color);
    ic.innerHTML = ICONS[sty.icon];
  }
  r.classList.toggle("dim", s.orphan || s.empty);
  setText(r.querySelector(".name"), titleOf(s));
  const bd = r.querySelector(".bd");
  const sig = lang + (s.running ? "r" : "") + (isProtected(s) ? "p" : "");
  if (bd.dataset.sig !== sig) {
    bd.dataset.sig = sig;
    bd.innerHTML =
      (s.running ? '<span class="badge run">' + esc(t("cl.badge.running")) + "</span>" : "") +
      (isProtected(s) ? '<span class="lock" data-tip="' + esc(t("cl.tip.protected")) + '">' + ICONS.lock + "</span>" : "");
  }
  setText(r.querySelector(".t2"), subOf(s));
  const pj = r.querySelector(".c-proj");
  setText(pj, s.orphan ? "–" : ltr(short(s.project)));
  pj.dataset.tip = s.orphan ? "" : s.project;
  const sc = r.querySelector(".c-src");
  setText(sc, srcLabel(id));
  sc.style.setProperty("--c", sty.color);
  const tm = r.querySelector(".c-time");
  setText(tm, relTime(s.mtime));
  tm.dataset.tip = fullTime(s.mtime);
  setText(r.querySelector(".c-size"), fmtBytes(s.size));
}

// Virtualized list: rows are a fixed height, so only the ones in (and a
// little around) the viewport are built; the rest is one tall spacer.
const ROW_H = 46; // = --row-h in cleaner.css
const ROW_PAD = 4; // #rows top padding
const OVERSCAN = 8;
function renderRows() {
  const box = $("rows");
  const inner = $("rows-in");
  inner.style.height = visible.length * ROW_H + "px";
  const top = box.scrollTop - ROW_PAD;
  const first = Math.max(0, Math.floor(top / ROW_H) - OVERSCAN);
  const last = Math.min(visible.length, Math.ceil((top + box.clientHeight) / ROW_H) + OVERSCAN);
  reconcile(inner, visible.slice(first, last), (s) => s.id, createRow, (r, s, i) => {
    updateRow(r, s);
    r.style.setProperty("--y", (first + i) * ROW_H + "px");
  });
  const es = $("empty-state");
  const key = st.notice || (query ? "cl.noMatch" : "cl.none");
  const sig = lang + key;
  if (es.dataset.sig !== sig) {
    es.dataset.sig = sig;
    es.innerHTML = ICONS.inbox + "<span>" + esc(t(key)) + "</span>";
  }
  es.classList.toggle("show", !visible.length);
}

let inspSig = "";
let inspOf = ""; // the selection the panel shows; a new one starts at the top
function renderInsp() {
  const sel = selectedList();
  const sig = lang + "|" + sel.map((s) => [s.id, s.mtime, s.size, s.pinned, s.excluded, s.running, s.title].join(":")).join(",");
  if (sig === inspSig) return;
  inspSig = sig;
  const inner = $("insp").firstElementChild;
  inner.innerHTML = sel.length === 1 ? inspOne(sel[0]) : sel.length ? inspMulti(sel) : inspNone();
  const of = sel.map((s) => s.id).join(",");
  if (of !== inspOf) {
    inspOf = of;
    inner.scrollTop = 0;
  }
  if (sel.length === 1) fillDetail(sel[0]);
}

// a height change eases instead of jumping: measure, change, animate between
function tweenHeight(el, change) {
  const h0 = el.offsetHeight;
  change();
  const h1 = el.offsetHeight;
  if (h0 === h1) return;
  el.style.overflow = "hidden";
  el.animate([{ height: h0 + "px" }, { height: h1 + "px" }], { duration: 300, easing: "cubic-bezier(0.22, 1, 0.36, 1)" })
    .finished.then(() => (el.style.overflow = ""), () => {});
}

// the detail block opens on a placeholder; the read fills it in
async function fillDetail(s) {
  const box = $("insp").querySelector(".i-detail");
  if (!box) return;
  if (detailOf(s)) return markClamped(box);
  const d = (await fetchDetail(s)) || { turns: [] };
  if (!box.isConnected) return; // the inspector moved on meanwhile
  tweenHeight(box, () => (box.innerHTML = '<div class="i-in">' + detailHTML(d) + "</div>"));
  markClamped(box);
}

// long messages fold to a few lines; only the folded ones unfold on click
function markClamped(root) {
  for (const m of root.querySelectorAll(".t-msg")) m.classList.toggle("more", m.scrollHeight > m.clientHeight + 1);
}

const irow = (key, val, cls) => "<dt>" + esc(t(key)) + '</dt><dd class="' + (cls || "") + '">' + esc(val) + "</dd>";
const stat = (val, label, tip) =>
  '<div class="stat"' + (tip ? ' data-tip="' + esc(tip) + '"' : "") + "><b>" + esc(val) + "</b><span>" + esc(label) + "</span></div>";
const tmsg = (cls, text) => '<div class="t-msg ' + cls + ' sel" data-act="msg">' + esc(text) + "</div>";
const turnsBox = (body) => '<div class="i-box"><h4>' + esc(t("cl.turns")) + "</h4>" + body + "</div>";

function detailHTML(d) {
  const meta = (d.model ? irow("cl.k.model", d.model, "mono sel") : "") + (d.branch ? irow("cl.k.branch", d.branch, "mono sel") : "");
  return (
    (meta ? '<dl class="i-grid">' + meta + "</dl>" : "") +
    turnsBox(
      d.turns.length
        ? d.turns.map((x) => '<div class="turn">' + tmsg("t-user", x.user) + (x.assistant ? tmsg("t-bot", x.assistant) : "") + "</div>").join("")
        : '<p class="t-none">' + esc(t("cl.turns.none")) + "</p>"
    )
  );
}

// the project as a chip under the title; a real folder opens on click
function projChip(s) {
  if (s.orphan || !s.project) return "";
  const inner = (s.excluded ? ICONS.folderMinus : ICONS.folder) + "<span>" + esc(baseName(s.project)) + "</span>";
  return s.cwd
    ? '<button class="chip" data-act="open-proj" data-tip="' + esc(short(s.project) + " · " + t("cl.openFolder")) + '">' + inner + "</button>"
    : '<span class="chip" data-tip="' + esc(short(s.project)) + '">' + inner + "</span>";
}

function inspNone() {
  return '<div class="i-empty i-in">' + ICONS.panelRight + "<h3>" + esc(t("cl.insp.none")) + "</h3><p>" +
    esc(tv("cl.insp.noneHint", { mod: MOD })) + "</p></div>";
}

function inspOne(s) {
  const id = srcId(s);
  const loc = s.cwd || s.project;
  const parent = s.root.replace(/[\\/][^\\/]*$/, "");
  const badge = (cls, key) => '<span class="badge ' + cls + '">' + esc(t(key)) + "</span>";
  const d = detailOf(s);
  const files = s.files.map(
    (f) =>
      '<li data-tip="' + esc(f.path) + '">' + (f.dir ? ICONS.folder : ICONS.file) +
      '<span class="fp mono">' + esc(ltr(f.path.slice(parent.length + 1))) + '</span><span class="fs">' +
      fmtBytes(f.size) + "</span></li>"
  ).join("");
  return (
    '<div class="i-in">' +
    '<div class="i-head">' + srcIcon(id, "big") + '<div><h2 class="sel">' + esc(titleOf(s)) + "</h2>" +
    '<div class="i-meta">' + projChip(s) + "<span>" + esc(srcLabel(id)) + "</span>" +
    (s.running ? badge("run", "cl.badge.running") : "") +
    (s.pinned ? badge("pin", "cl.badge.pinned") : "") +
    (s.excluded ? badge("pin", "cl.badge.excluded") : "") + "</div></div></div>" +
    '<div class="i-actions">' +
    (canResume(s) ? '<button class="btn primary" data-act="resume">' + ICONS.terminal + esc(t("cl.resume")) + "</button>" : "") +
    '<button class="btn" data-act="pin">' + (s.pinned ? ICONS.lockOpen : ICONS.lock) + esc(t(s.pinned ? "cl.unpin" : "cl.pin")) + "</button>" +
    (canRename(s) ? '<button class="btn" data-act="rename">' + ICONS.pencil + esc(t("cl.rename")) + "</button>" : "") +
    "</div>" +
    '<div class="i-stats">' +
    stat(relTime(s.mtime), t("cl.col.mtime"), fullTime(s.mtime)) +
    stat(s.started ? fmtDur(Date.parse(s.mtime) - Date.parse(s.started)) : "–", t("cl.stat.dur")) +
    stat(fmtBytes(s.size), tv("cl.stat.files", { n: s.files.length })) +
    "</div>" +
    '<dl class="i-grid">' +
    irow("cl.k.id", s.sid, "mono sel") +
    (loc ? irow("cl.k.loc", short(loc), "sel") : "") +
    (s.started ? irow("cl.k.started", fullTime(s.started)) : "") +
    irow("cl.k.mtime", fullTime(s.mtime)) +
    (s.excluded ? irow("cl.k.excluded", short(s.excluded)) : "") +
    "</dl>" +
    // the tour's mock sessions have nothing on disk to read
    (!touring && !s.orphan && st.can && st.can.detail
      ? '<div class="i-detail">' + (d ? detailHTML(d) : turnsBox('<div class="t-skel"><i></i><i></i><i></i></div>')) + "</div>"
      : "") +
    (s.prompt ?'<div class="i-box"><h4>' + esc(t("cl.prompt")) + '</h4><p class="sel">' + esc(s.prompt) + "</p></div>" : "") +
    '<div class="i-box"><h4>' + esc(t("cl.files")) + '</h4><ul class="i-files">' + files +
    (hasKey("cl.hist." + s.agent) ? '<li class="hist">' + ICONS.list + "<span>" + esc(t("cl.hist." + s.agent)) + "</span></li>" : "") +
    "</ul></div>" +
    '<div class="i-foot">' +
    '<button class="btn sm" data-act="reveal">' + ICONS.folder + esc(t("cl.reveal")) + "</button>" +
    '<button class="btn sm" data-act="copy-id">' + ICONS.copy + esc(t("cl.copyId")) + "</button>" +
    (s.cwd ? '<button class="btn sm" data-act="copy-resume">' + ICONS.terminal + esc(t("cl.copyResume")) + "</button>" : "") +
    "</div></div>"
  );
}

function inspMulti(sel) {
  const groups = new Map();
  for (const s of sel) {
    const id = srcId(s);
    if (!groups.has(id)) groups.set(id, []);
    groups.get(id).push(s);
  }
  const ok = sel.filter(deletable);
  const skipped = sel.length - ok.length;
  const allPinned = sel.every((s) => s.pinned);
  return (
    '<div class="i-multi i-in"><div class="big">' + ICONS.layers + "</div>" +
    "<h3>" + esc(tv("cl.multi", { n: sel.length })) + '</h3><div class="total">' + fmtBytes(total(sel)) + "</div>" +
    '<ul class="i-groups">' +
    [...groups].sort((a, b) => b[1].length - a[1].length).map(
      ([id, l]) => "<li>" + srcIcon(id, "sm") + "<span>" + esc(srcLabel(id)) + "</span><span>" + esc(countSize(l)) + "</span></li>"
    ).join("") +
    "</ul>" +
    '<button class="btn" data-act="pin-all">' + (allPinned ? ICONS.lockOpen : ICONS.lock) +
    esc(t(allPinned ? "cl.unpinAll" : "cl.pinAll")) + "</button>" +
    (ok.length ? '<button class="btn danger" data-act="delete">' + ICONS.trash + esc(tv("cl.deleteN", { n: ok.length })) + "</button>" : "") +
    (skipped ? '<div class="warn-line">' + ICONS.alert + "<span>" + esc(tv("cl.skipNote", { n: skipped })) + "</span></div>" : "") +
    "</div>"
  );
}

function renderStatus() {
  const base = baseList();
  setText($("st-total"), tv("cl.status.total", { n: base.length, size: fmtBytes(total(base)) }));
  const prot = base.filter(isProtected).length;
  const sp = $("st-prot");
  if (sp.dataset.n !== String(prot)) {
    sp.dataset.n = String(prot);
    sp.innerHTML = prot ? ICONS.lock + "<span>" + prot + "</span>" : "";
  }
  sp.dataset.tip = t("cl.f.protected");
  const sel = selectedList();
  setText($("st-sel"), sel.length ? tv("cl.status.sel", { n: sel.length, size: fmtBytes(total(sel)) }) : "");
}

function renderToolbar() {
  const sel = selectedList();
  const locked = busy === "cl.deleting";
  $("del").disabled = !sel.length || locked;
  setText($("del").querySelector(".lbl"), sel.length ? tv("cl.deleteN", { n: sel.length }) : t("cl.delete"));
  $("bulk").disabled = locked;
  setText($("subtitle"), countSize(visible));
}

let prefsSig = "";
function renderPrefs() {
  const ex = st.prefs.excluded;
  const sig = lang + JSON.stringify(ex);
  if (sig !== prefsSig) {
    prefsSig = sig;
    $("ex-list").innerHTML = ex.length
      ? ex.map(
          (p) =>
            '<li data-p="' + esc(p) + '">' + ICONS.folderMinus + '<span class="p" data-tip="' + esc(p) + '">' +
            esc(ltr(short(p))) + '</span><button class="icon-btn" data-tip="' + esc(t("cl.remove")) + '">' + ICONS.x + "</button></li>"
        ).join("")
      : '<li class="none">' + esc(t("cl.set.noExcluded")) + "</li>";
  }
  $("sw-hide").classList.toggle("on", !!st.prefs.hideExcluded);
  if (document.activeElement !== $("in-old")) $("in-old").value = st.prefs.oldDays;
  if (document.activeElement !== $("in-large")) $("in-large").value = st.prefs.largeMB;
}

// ---------- busy / flash ----------
let busyTimer = 0;
function setBusy(key) {
  busy = key;
  clearTimeout(busyTimer);
  const el = $("st-busy");
  if (!key) el.innerHTML = "";
  // only slow work earns a spinner; a warm rescan is ~50ms
  else busyTimer = setTimeout(() => (el.innerHTML = ICONS.loader + "<span>" + esc(t(key)) + "</span>"), 200);
  if (st) renderToolbar();
}

let flashTimer = 0;
function flash(msg) {
  const el = $("st-msg");
  el.textContent = msg;
  el.classList.add("show");
  clearTimeout(flashTimer);
  flashTimer = setTimeout(() => el.classList.remove("show"), 2200);
}

// ---------- menus ----------
const ctxEl = $("ctxmenu");
function fillMenu(el, items) {
  el.innerHTML = "";
  for (const it of items) {
    if (it === "sep") {
      el.appendChild(Object.assign(document.createElement("div"), { className: "ctx-sep" }));
      continue;
    }
    const b = document.createElement("button");
    b.className = "ctx-item" + (it.danger ? " danger" : "");
    b.disabled = !!it.disabled;
    b.innerHTML = ICONS[it.icon] + "<span></span>" + (it.note ? "<small></small>" : "");
    b.querySelector("span").textContent = it.label;
    if (it.note) b.querySelector("small").textContent = it.note;
    b.addEventListener("click", () => {
      closeMenus();
      it.act();
    });
    el.appendChild(b);
  }
}
function openCtx(e, items) {
  e.preventDefault();
  closeMenus();
  fillMenu(ctxEl, items);
  ctxEl.classList.add("show");
  ctxEl.style.left = Math.max(6, Math.min(e.clientX, innerWidth - ctxEl.offsetWidth - 6)) + "px";
  ctxEl.style.top = Math.max(6, Math.min(e.clientY, innerHeight - ctxEl.offsetHeight - 6)) + "px";
}
function closeMenus() {
  ctxEl.classList.remove("show");
  for (const d of document.querySelectorAll(".drop.open")) d.classList.remove("open");
}
function toggleDrop(btn, fill) {
  const d = btn.parentElement;
  const open = !d.classList.contains("open");
  closeMenus();
  if (!open) return;
  if (fill) fill();
  d.classList.add("open");
}

function rowMenu(sel) {
  const one = sel.length === 1 ? sel[0] : null;
  const items = [];
  if (one && canResume(one)) {
    items.push(
      { icon: "terminal", label: t("cl.resume"), act: () => resume(one) },
      { icon: "copy", label: t("cl.copyResume"), act: () => copy(resumeCmd(one)) },
      "sep"
    );
  }
  if (one && canRename(one)) items.push({ icon: "pencil", label: t("cl.ctx.rename"), act: () => askRename(one) });
  const allPinned = sel.every((s) => s.pinned);
  items.push({ icon: allPinned ? "lockOpen" : "lock", label: t(allPinned ? "cl.unpin" : "cl.pinLong"), act: () => setPinned(sel, !allPinned) });
  if (one && !one.orphan) {
    items.push(
      one.excluded
        ? { icon: "folder", label: tv("cl.ctx.unexclude", { p: short(one.excluded) }), act: () => setExcluded([one.excluded], false) }
        : { icon: "folderMinus", label: t("cl.ctx.excludeProject"), act: () => setExcluded([one.project], true) }
    );
  }
  if (one) items.push({ icon: "folder", label: t("cl.reveal"), act: () => reveal(one) });
  items.push("sep", { icon: "trash", label: t("cl.ctx.delete"), danger: true, act: () => askDelete(sel) });
  return items;
}

function bulkItems() {
  const base = baseList();
  const kinds = [["cl.bulk.orphan", "orphan"], ["cl.bulk.empty", "empty"], ["cl.bulk.old", "old"]];
  return kinds.filter(([, f]) => f !== "orphan" || currentTab === "claude").map(([key, f]) => {
    const list = base.filter((s) => matches(f, s));
    const ok = list.filter(deletable);
    return { icon: "trash", label: tv(key, { n: st.prefs.oldDays }), note: countSize(ok), disabled: !ok.length, act: () => askDelete(list) };
  });
}

// ---------- modal ----------
let modalOpen = false;
function showModal(title, body, buttons) {
  closeMenus();
  const d = $("modal").firstElementChild;
  d.innerHTML = "<h3></h3>" + body + '<div class="dlg-btns"></div>';
  d.querySelector("h3").textContent = title;
  for (const b of buttons) {
    const el = document.createElement("button");
    el.className = "btn " + (b.kind || "");
    el.textContent = b.label;
    el.addEventListener("click", () => {
      closeModal();
      if (b.act) b.act();
    });
    d.lastElementChild.appendChild(el);
  }
  modalOpen = true;
  $("modal").classList.add("show");
}
function closeModal() {
  modalOpen = false;
  $("modal").classList.remove("show");
}
const okButton = () => [{ label: t("cl.ok"), kind: "primary" }];

function askDelete(targets) {
  if (!targets.length || busy === "cl.deleting") return;
  const ok = targets.filter(deletable);
  const running = targets.filter((s) => s.running).length;
  const prot = targets.filter((s) => !s.running && isProtected(s)).length;
  const notes = [running && tv("cl.skip.running", { n: running }), prot && tv("cl.skip.protected", { n: prot })].filter(Boolean);
  const skipped = notes.length ? tv("cl.skipped", { list: notes.join(", ") }) : "";
  if (!ok.length) return showModal(t("cl.nothing"), skipped ? "<p>" + esc(skipped) + "</p>" : "", okButton());
  const names =
    ok.slice(0, 8).map(
      (s) => "<li>• " + esc(titleOf(s).slice(0, 48)) + " <small>(" + esc(s.orphan ? "–" : baseName(s.project)) + ")</small></li>"
    ).join("") + (ok.length > 8 ? "<li><small>" + esc(tv("cl.more", { n: ok.length - 8 })) + "</small></li>" : "");
  showModal(
    tv("cl.confirm.title", { n: ok.length }),
    '<ul class="names">' + names + "</ul><p>" + esc(tv("cl.confirm.body", { size: fmtBytes(total(ok)) }) + (skipped ? "\n\n" + skipped : "")) + "</p>",
    [
      { label: t("cl.cancel") },
      { label: t("cl.trash"), kind: "primary", act: () => doDelete(ok, false) },
      { label: t("cl.permanent"), kind: "danger", act: () => doDelete(ok, true) },
    ]
  );
}

function askRename(s) {
  showModal(t("cl.rename.title"), '<input id="rn-in" class="dlg-in" type="text" maxlength="200" spellcheck="false" autocomplete="off"/>', [
    { label: t("cl.cancel") },
    { label: t("cl.rename.save"), kind: "primary", act: () => doRename(s, $("rn-in").value) },
  ]);
  const inp = $("rn-in");
  inp.value = s.title;
  inp.placeholder = titleOf(s);
  inp.focus();
  inp.select();
}

async function doDelete(list, permanent) {
  setBusy("cl.deleting");
  // rows fold away while the daemon works; a failed one comes back on reload
  for (const s of list) {
    const r = $("rows").querySelector('.row[data-key="' + CSS.escape(s.id) + '"]');
    if (r) r.classList.add("leave");
  }
  $("rows").classList.add("settle");
  const r = await api("/api/cleaner/delete", { agent: currentTab, ids: list.map((s) => s.id), permanent });
  const res = r ? await r.json().catch(() => null) : null;
  selection.clear();
  anchor = null;
  await load();
  setTimeout(() => $("rows").classList.remove("settle"), 300);
  if (!res) return flash(t("cl.err.generic"));
  if (res.errors && res.errors.length) {
    showModal(t("cl.errTitle"), '<p class="errs mono">' + esc(res.errors.slice(0, 15).join("\n")) + "</p>", okButton());
  } else {
    flash(tv("cl.done", { n: res.deleted }));
  }
}

// ---------- theme / language ----------
const lightMQ = matchMedia("(prefers-color-scheme: light)");
let themeSetting = "";
function applyTheme(th) {
  if (th !== undefined) themeSetting = th;
  document.documentElement.dataset.theme =
    themeSetting === "light" || themeSetting === "dark" ? themeSetting : lightMQ.matches ? "light" : "dark";
}
lightMQ.addEventListener("change", () => applyTheme());

function applyLang() {
  document.documentElement.lang = lang;
  document.title = t("cl.title");
  for (const el of document.querySelectorAll("[data-i18n]")) el.textContent = t(el.dataset.i18n);
  $("q").placeholder = t("cl.search");
  $("refresh").dataset.tip = t("cl.refresh") + " (" + MOD + "R)";
  $("gear").dataset.tip = t("cl.settings");
  $("help-btn").dataset.tip = t("tip.help");
  $("insp-btn").dataset.tip = t("cl.inspector");
  buildHead();
  inspSig = prefsSig = "";
}

// ---------- events ----------
$("q").addEventListener("input", (e) => {
  query = e.target.value;
  e.target.parentElement.classList.toggle("has", !!query);
  render();
});
$("q-clear").addEventListener("click", () => {
  $("q").value = "";
  $("q").dispatchEvent(new Event("input"));
  $("q").focus();
});
$("refresh").addEventListener("click", () => load());
$("bulk").addEventListener("click", () => toggleDrop($("bulk"), () => fillMenu($("bulk-menu"), bulkItems())));
$("gear").addEventListener("click", () => toggleDrop($("gear")));
$("del").addEventListener("click", () => askDelete(selectedList()));
$("insp-btn").addEventListener("click", () => {
  const off = $("app").classList.toggle("no-insp");
  $("insp-btn").classList.toggle("active", !off);
  try { localStorage.setItem("cleaner.insp", off ? "off" : "on"); } catch (_) {}
});
try {
  if (localStorage.getItem("cleaner.insp") === "off") {
    $("app").classList.add("no-insp");
    $("insp-btn").classList.remove("active");
  }
} catch (_) {}

$("thead").addEventListener("click", (e) => {
  const b = e.target.closest("button");
  if (!b) return;
  const k = b.dataset.sort;
  if (k === sortKey) sortDir = -sortDir;
  else {
    sortKey = k;
    sortDir = k === "mtime" || k === "size" ? -1 : 1;
  }
  render();
});

$("side").addEventListener("click", (e) => {
  const b = e.target.closest(".side-row");
  if (!b || b.dataset.key === filter) return;
  filter = b.dataset.key;
  selection.clear();
  anchor = null;
  render();
  $("rows").scrollTop = 0;
});
$("side").addEventListener("contextmenu", (e) => {
  const b = e.target.closest(".side-row");
  if (!b || !b.dataset.key.startsWith("proj:")) return;
  const p = b.dataset.key.slice(5);
  const hit = st.sessions.find((s) => s.project === p && s.excluded);
  const ex = hit && hit.excluded;
  openCtx(e, [
    ex
      ? { icon: "folder", label: tv("cl.ctx.unexclude", { p: short(ex) }), act: () => setExcluded([ex], false) }
      : { icon: "folderMinus", label: t("cl.ctx.exclude"), act: () => setExcluded([p], true) },
    { icon: "folder", label: t("cl.openFolder"), act: () => api("/api/cleaner/reveal", { path: p }) },
  ]);
});

$("rows").addEventListener("mousedown", (e) => {
  if (e.button !== 0) return;
  const mod = e.metaKey || e.ctrlKey;
  const r = e.target.closest(".row");
  if (!r) {
    if (!mod && !e.shiftKey && selection.size) {
      selection.clear();
      renderSel();
    }
    return;
  }
  const id = r.dataset.key;
  const a = visible.findIndex((s) => s.id === anchor);
  if (e.shiftKey && a >= 0) {
    const b = visible.findIndex((s) => s.id === id);
    if (!mod) selection.clear();
    for (let i = Math.min(a, b); i <= Math.max(a, b); i++) selection.add(visible[i].id);
  } else if (mod) {
    if (selection.has(id)) selection.delete(id);
    else selection.add(id);
    anchor = id;
  } else {
    selection.clear();
    selection.add(id);
    anchor = id;
  }
  renderSel();
});
$("rows").addEventListener("dblclick", (e) => {
  const r = e.target.closest(".row");
  const s = r && visible.find((x) => x.id === r.dataset.key);
  if (s) reveal(s);
});
$("rows").addEventListener("contextmenu", (e) => {
  const r = e.target.closest(".row");
  if (!r) return;
  if (!selection.has(r.dataset.key)) {
    selection.clear();
    selection.add(r.dataset.key);
    anchor = r.dataset.key;
    renderSel();
  }
  openCtx(e, rowMenu(selectedList()));
});

$("insp").addEventListener("click", (e) => {
  const b = e.target.closest("[data-act]");
  const sel = selectedList();
  if (!b || !sel.length) return;
  const s = sel[0];
  switch (b.dataset.act) {
    case "resume": return resume(s);
    case "pin": return setPinned([s], !s.pinned);
    case "pin-all": return setPinned(sel, !sel.every((x) => x.pinned));
    case "reveal": return reveal(s);
    case "copy-id": return copy(s.sid);
    case "copy-resume": return copy(resumeCmd(s));
    case "delete": return askDelete(sel);
    case "rename": return askRename(s);
    case "open-proj": return api("/api/cleaner/reveal", { path: s.project });
    case "msg":
      // a drag that selected text isn't a click to unfold
      if ((b.classList.contains("more") || b.classList.contains("open")) && !String(getSelection())) {
        tweenHeight(b, () => b.classList.toggle("open"));
      }
  }
});

$("ex-list").addEventListener("click", (e) => {
  const li = e.target.closest("button") && e.target.closest("li");
  if (li) setExcluded([li.dataset.p], false);
});
$("sw-hide").addEventListener("click", () => setPref({ hideExcluded: !st.prefs.hideExcluded }));
$("in-old").addEventListener("change", (e) => setPref({ oldDays: Math.round(+e.target.value) }));
$("in-large").addEventListener("change", (e) => setPref({ largeMB: Math.round(+e.target.value) }));

$("modal").addEventListener("mousedown", (e) => e.target === $("modal") && closeModal());
document.addEventListener("mousedown", (e) => {
  if (!ctxEl.contains(e.target)) ctxEl.classList.remove("show");
  for (const d of document.querySelectorAll(".drop.open")) if (!d.contains(e.target)) d.classList.remove("open");
});
document.addEventListener("contextmenu", (e) => e.target.tagName !== "INPUT" && e.preventDefault());
window.addEventListener("blur", closeMenus);
let scrollQueued = false;
$("rows").addEventListener("scroll", () => {
  ctxEl.classList.remove("show");
  if (scrollQueued) return;
  scrollQueued = true;
  requestAnimationFrame(() => {
    scrollQueued = false;
    if (st) renderRows();
  });
});
// only the height decides which rows exist; width changes (the inspector
// sliding) must not rebuild rows every frame
let rowsH = 0;
new ResizeObserver(() => {
  const h = $("rows").clientHeight;
  if (h !== rowsH && st) {
    rowsH = h;
    renderRows();
  }
}).observe($("rows"));
// coming back to the window picks up sessions started or ended meanwhile
window.addEventListener("focus", () => Date.now() - lastLoad > 4000 && load());

document.addEventListener("keydown", (e) => {
  if (touring) return; // the tour's overlay only blocks the mouse
  if (modalOpen) {
    if (e.isComposing || e.keyCode === 229) return; // IME: Return commits the syllable
    // Return cancels a confirm — a destructive default is one keypress too
    // easy — but saves from the rename field
    if (e.key === "Enter" && e.target.id === "rn-in") {
      e.preventDefault();
      $("modal").querySelector(".btn.primary").click();
    } else if (e.key === "Escape" || e.key === "Enter") {
      e.preventDefault();
      closeModal();
    }
    return;
  }
  const mod = e.metaKey || e.ctrlKey;
  const k = e.key.toLowerCase();
  const typing = e.target.tagName === "INPUT";
  if (mod && k === "f") {
    e.preventDefault();
    $("q").focus();
    $("q").select();
  } else if ((mod && k === "r") || e.key === "F5") {
    e.preventDefault();
    load();
  } else if (e.key === "Escape") {
    if (ctxEl.classList.contains("show") || document.querySelector(".drop.open")) closeMenus();
    else if (typing) e.target.blur();
    else if (selection.size) {
      selection.clear();
      renderSel();
    }
  } else if (typing) {
    return;
  } else if (mod && k === "a") {
    e.preventDefault();
    for (const s of visible) selection.add(s.id);
    renderSel();
  } else if (e.key === "Delete" || e.key === "Backspace") {
    e.preventDefault();
    askDelete(selectedList());
  } else if ((e.key === "Enter" || e.key === "F2") && e.target.tagName !== "BUTTON") { // Finder's and Explorer's rename keys
    const sel = selectedList();
    if (sel.length === 1 && canRename(sel[0])) {
      e.preventDefault();
      askRename(sel[0]);
    }
  } else if ((e.key === "ArrowDown" || e.key === "ArrowUp") && visible.length) {
    e.preventDefault();
    let i = visible.findIndex((s) => s.id === anchor);
    i = i < 0 ? 0 : Math.max(0, Math.min(visible.length - 1, i + (e.key === "ArrowDown" ? 1 : -1)));
    if (!e.shiftKey) selection.clear();
    anchor = visible[i].id;
    selection.add(anchor);
    renderSel();
    // the target row may not be built yet: scroll by arithmetic
    const box = $("rows");
    const y = i * ROW_H + ROW_PAD;
    if (y < box.scrollTop) box.scrollTop = y;
    else if (y + ROW_H > box.scrollTop + box.clientHeight) box.scrollTop = y + ROW_H - box.clientHeight + 12;
  }
});

// the daemon reuses one window: another tab's button switches it over
window.switchAgent = (agent) => {
  if (agent === currentTab || touring) return;
  $("app").classList.add("switching");
  setTimeout(async () => {
    currentTab = agent;
    document.body.dataset.agent = agent;
    history.replaceState(null, "", "?agent=" + encodeURIComponent(agent));
    filter = "all";
    query = $("q").value = "";
    $("q").parentElement.classList.remove("has");
    selection.clear();
    anchor = null;
    st = null;
    details.clear();
    $("rows-in").innerHTML = "";
    inspSig = "";
    await load();
    $("rows").scrollTop = 0;
    $("app").classList.remove("switching");
  }, 180);
};

// ---------- boot ----------
document.querySelector(".search .s-ic").innerHTML = ICONS.search;
$("q-clear").innerHTML = ICONS.x;
$("refresh").innerHTML = ICONS.rotate;
$("bulk").querySelector(".ic").innerHTML = ICONS.sparkles;
$("bulk").querySelector(".chev").innerHTML = ICONS.chevron;
$("del").querySelector(".ic").innerHTML = ICONS.trash;
$("gear").innerHTML = ICONS.gear;
$("help-btn").innerHTML = ICONS.help;
$("insp-btn").innerHTML = ICONS.panelRight;
load();
