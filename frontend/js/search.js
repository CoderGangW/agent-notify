"use strict";

// ---- events-card sub-tabs (active sessions / events) ----
let subTab = "sessions";

function applySubTab() {
  for (const b of document.querySelectorAll(".subtab")) {
    b.classList.toggle("active", b.dataset.subtab === subTab);
  }
  $("sessions").classList.toggle("hidden", subTab !== "sessions");
  $("events").classList.toggle("hidden", subTab !== "events");
  $("events-actions").classList.toggle("hidden", subTab !== "events");
  if (searchOpen) renderSearchChips(); // each sub-tab has its own filter chips
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

// ---- search & filter: the events card floats out of the flow and glides
// to fill the window (FLIP) so the list gets room while the search UI shows ----
let searchOpen = false;
let searchAnim = false; // ignore toggles while the card is mid-glide
let searchQuery = "";
const sessFilter = new Set(); // empty set = no state filter
const evFilter = new Set();

const CHIP_DEFS = {
  sessions: [
    { k: "working", label: () => t("state.working") },
    { k: "tool", label: () => t("filter.tool") },
    { k: "waiting", label: () => t("state.waiting") },
    { k: "idle", label: () => t("state.idle") },
  ],
  events: [
    { k: "done", label: () => t("notif.done") },
    { k: "attention", label: () => t("notif.attention") },
    { k: "unread", label: () => t("filter.unread") },
  ],
};

function renderSearchChips() {
  const wrap = $("search-chips");
  const set = subTab === "sessions" ? sessFilter : evFilter;
  wrap.innerHTML = "";
  CHIP_DEFS[subTab].forEach((c, i) => {
    const b = document.createElement("button");
    b.className = "f-chip" + (set.has(c.k) ? " on" : "");
    b.style.animationDelay = i * 40 + "ms";
    b.textContent = c.label();
    b.addEventListener("click", () => {
      set.has(c.k) ? set.delete(c.k) : set.add(c.k);
      b.classList.toggle("on");
      rerenderLists();
    });
    wrap.appendChild(b);
  });
}

function rerenderLists() {
  if (!lastState) return;
  renderSessions(lastState.sessions || []);
  renderEvents(lastState.events || [], lastState.unread || {});
}

// any query text or an active chip on the current sub-tab
function searchActive() {
  const set = subTab === "sessions" ? sessFilter : evFilter;
  return searchQuery.trim() !== "" || set.size > 0;
}

function searchMatch(fields) {
  const q = searchQuery.trim().toLowerCase();
  if (!q) return true;
  return fields.some((f) => (f || "").toLowerCase().includes(q));
}

function sessSearchPass(s) {
  const st = ["working", "tool", "waiting"].includes(s.state) ? s.state : "idle";
  if (sessFilter.size && !sessFilter.has(st)) return false;
  return searchMatch([s.title, s.cwd, s.branch, s.tool, s.detail, s.task]);
}

function evSearchPass(ev) {
  const kind = ev.kind === "attention" ? "attention" : "done";
  const kinds = ["done", "attention"].filter((k) => evFilter.has(k));
  if (kinds.length && !kinds.includes(kind)) return false;
  if (evFilter.has("unread") && ev.read) return false;
  return searchMatch([ev.title, ev.cwd, ev.branch, ev.message, ev.model]);
}

function setSearchCount(shown, total) {
  $("search-count").textContent =
    searchOpen && searchActive() ? shown + " / " + total : "";
}

function openSearch() {
  if (searchOpen || searchAnim) return;
  searchOpen = true;
  searchAnim = true;
  const card = $("events-card");
  const r = card.getBoundingClientRect();
  // ghost keeps the flex column from collapsing while the card floats
  const ghost = document.createElement("div");
  ghost.id = "card-ghost";
  card.parentNode.insertBefore(ghost, card);
  card.style.top = r.top + "px";
  card.style.left = r.left + "px";
  card.style.width = r.width + "px";
  card.style.height = r.height + "px";
  card.classList.add("float");
  void card.offsetHeight; // paint the pinned frame first so the glide animates
  card.classList.add("expanded");
  // stop below the header: the util buttons stay reachable while searching
  const headBottom = Math.round(document.querySelector("header").getBoundingClientRect().bottom) + 10;
  card.style.top = headBottom + "px";
  card.style.left = "12px";
  card.style.width = "calc(100vw - 24px)"; // calc keeps it tracking window resizes
  card.style.height = "calc(100vh - " + (headBottom + 12) + "px)";
  card.addEventListener("transitionend", function onEnd(ev) {
    if (ev.target !== card || ev.propertyName !== "height") return;
    card.removeEventListener("transitionend", onEnd);
    searchAnim = false;
  });
  $("search-btn").classList.add("active");
  renderSearchChips();
  $("search-bar").classList.add("open");
  $("search-input").focus();
  rerenderLists();
}

function closeSearch() {
  if (!searchOpen || searchAnim) return;
  searchOpen = false;
  searchAnim = true;
  const card = $("events-card");
  const ghost = $("card-ghost");
  const r = ghost.getBoundingClientRect();
  card.classList.remove("expanded");
  card.style.top = r.top + "px";
  card.style.left = r.left + "px";
  card.style.width = r.width + "px";
  card.style.height = r.height + "px";
  $("search-bar").classList.remove("open");
  $("search-btn").classList.remove("active");
  card.addEventListener("transitionend", function onEnd(ev) {
    if (ev.target !== card || ev.propertyName !== "height") return;
    card.removeEventListener("transitionend", onEnd);
    card.classList.remove("float");
    card.style.top = card.style.left = card.style.width = card.style.height = "";
    ghost.remove();
    searchAnim = false;
  });
  rerenderLists(); // unfiltered rows return while the card glides back
}

// expand: grow the window for long lists; search appears only when tall
let winTall = false;
$("expand-btn").addEventListener("click", () => {
  winTall = !winTall;
  document.body.classList.toggle("tall", winTall);
  $("expand-btn").innerHTML = winTall ? ICONS.collapse : ICONS.expand;
  $("expand-btn").dataset.tip = t(winTall ? "tip.collapse" : "tip.expand");
  if (!winTall && searchOpen) {
    // sequence, don't overlap: gliding the search card home while the
    // window is also shrinking makes both animations land wrong
    closeSearch();
    setTimeout(() => post("/api/expand", { expanded: false }), 400);
    return;
  }
  post("/api/expand", { expanded: winTall });
});

$("search-btn").addEventListener("click", () =>
  searchOpen ? closeSearch() : openSearch()
);
$("search-close").addEventListener("click", closeSearch);
$("search-input").addEventListener("input", () => {
  searchQuery = $("search-input").value;
  $("search-clear").classList.toggle("show", searchQuery !== "");
  rerenderLists();
});
$("search-clear").addEventListener("click", () => {
  $("search-input").value = "";
  searchQuery = "";
  $("search-clear").classList.remove("show");
  $("search-input").focus();
  rerenderLists();
});
document.addEventListener("keydown", (e) => {
  if (e.key !== "Escape" || !searchOpen) return;
  // overlays stacked above the card claim Escape first
  if (settingsOpen || !$("stats-overlay").classList.contains("hidden")) return;
  closeSearch();
});

// which icon a running tool gets; unknown tools fall back to the wrench
function toolIconName(tool) {
  const n = (tool || "").toLowerCase();
  if (n.includes("bash") || n.includes("shell") || n.includes("terminal")) return "terminal";
  if (n.includes("edit") || n.includes("write") || n.includes("notebook")) return "pencil";
  if (n.includes("read")) return "file";
  if (n.includes("web") || n.includes("fetch")) return "globe";
  if (n.includes("grep") || n.includes("glob") || n.includes("search")) return "search";
  if (n.includes("task") || n.includes("agent")) return "bot";
  return "wrench";
}

function stateIcon(s) {
  if (s.state === "tool") return ['<span class="ic act tool">' + ICONS[toolIconName(s.tool)] + "</span>", ""];
  if (s.state === "working") return ['<span class="ic act work">' + ICONS.sparkles + "</span>", ""];
  if (s.state === "waiting") return ['<span class="ic wait">' + ICONS.bellRing + "</span>", ""];
  return ['<span class="ic idle">' + ICONS.check + "</span>", ""];
}

function sessionStateText(s) {
  if (s.state === "tool") return (s.tool || "tool") + (s.detail ? ": " + s.detail : "");
  if (s.state === "working") return t("state.working");
  if (s.state === "waiting") return t("state.waiting");
  return t("state.idle");
}
