"use strict";

// ---- tabs ----
let currentTab = null; // set from defaultTab on first state fetch
let defaultTab = "claude";
let lastState = null;
// 4-level notification mode; badges show only in "on" and "quiet"
let notifyMode = "on";
const badgesOn = () => notifyMode === "on" || notifyMode === "quiet";

function updateInk() {
  const seg = document.querySelector("#tabs .seg");
  const ink = $("tab-ink");
  for (const b of seg.querySelectorAll(".tab")) {
    if (b.dataset.tab !== currentTab) continue;
    ink.style.transform = `translateX(${b.offsetLeft - 3}px)`;
    ink.style.width = b.offsetWidth + "px";
    ink.style.opacity = "";
    return;
  }
  // no active tab (nothing selected): a stale ink pill would float
  // behind the lone "+" button
  ink.style.opacity = "0";
  ink.style.width = "0px";
}

// buildTabs renders one tab per ENABLED agent (picked in settings) plus a
// trailing "+" that opens settings to change the pick. When the row can't
// fit, the strip becomes a carousel: arrows step one tab at a time.
const enabledAgents = () => agents.filter((a) => a.enabled !== false);

function buildTabs() {
  const seg = document.querySelector("#tabs .seg");
  const shown = enabledAgents();
  const sig = shown.map((a) => a.id).join(",");
  if (seg.dataset.sig !== sig) {
    seg.dataset.sig = sig;
    for (const b of seg.querySelectorAll(".tab")) b.remove();
    seg.classList.toggle("many", shown.length > 2);
    for (const a of shown) {
      const b = document.createElement("button");
      b.className = "tab";
      b.dataset.tab = a.id;
      const label = document.createElement("span");
      label.className = "tab-label";
      label.textContent = a.name;
      b.appendChild(label);
      const badge = document.createElement("span");
      badge.className = "tab-badge";
      badge.id = "badge-" + a.id;
      b.appendChild(badge);
      seg.appendChild(b);
    }
    const more = document.createElement("button");
    more.className = "tab more-tab";
    more.textContent = "+";
    more.dataset.tip = t("tip.moreAgents");
    more.addEventListener("click", (e) => {
      e.stopPropagation(); // not a tab switch
      openSettings("agents");
    });
    seg.appendChild(more);
    // the current agent was disabled: fall back to the first enabled tab,
    // or to the no-agents empty state when nothing is left
    if (currentTab && !shown.some((a) => a.id === currentTab)) {
      currentTab = shown.length ? shown[0].id : null;
    }
    applyTab();
  }
  updateCarousel();
}

// carousel: arrows appear only when the strip overflows; each click
// scrolls by one tab, and the active tab is kept in view
function updateCarousel() {
  const seg = document.querySelector("#tabs .seg");
  const over = seg.scrollWidth > seg.clientWidth + 2;
  $("tabs").classList.toggle("carousel", over);
  if (over) scrollTabIntoView();
  updateTabArrows();
}

function updateTabArrows() {
  const seg = document.querySelector("#tabs .seg");
  $("tab-prev").disabled = seg.scrollLeft <= 1;
  $("tab-next").disabled = seg.scrollLeft >= seg.scrollWidth - seg.clientWidth - 1;
}

function stepTabs(dir) {
  const seg = document.querySelector("#tabs .seg");
  const tabs = [...seg.querySelectorAll(".tab")];
  if (!tabs.length) return;
  // first tab not fully visible in the given direction
  const target =
    dir > 0
      ? tabs.find((b) => b.offsetLeft + b.offsetWidth > seg.scrollLeft + seg.clientWidth + 1)
      : [...tabs].reverse().find((b) => b.offsetLeft < seg.scrollLeft - 1);
  if (target) {
    seg.scrollTo({
      left: dir > 0
        ? target.offsetLeft + target.offsetWidth - seg.clientWidth + 4
        : Math.max(0, target.offsetLeft - 4),
      behavior: "smooth",
    });
  }
}

function scrollTabIntoView() {
  const seg = document.querySelector("#tabs .seg");
  const b = seg.querySelector('.tab[data-tab="' + (currentTab || "claude") + '"]');
  if (!b) return;
  if (b.offsetLeft < seg.scrollLeft) {
    seg.scrollTo({ left: Math.max(0, b.offsetLeft - 4), behavior: "smooth" });
  } else if (b.offsetLeft + b.offsetWidth > seg.scrollLeft + seg.clientWidth) {
    seg.scrollTo({ left: b.offsetLeft + b.offsetWidth - seg.clientWidth + 4, behavior: "smooth" });
  }
}

$("tab-prev").addEventListener("click", () => stepTabs(-1));
$("tab-next").addEventListener("click", () => stepTabs(1));
document.querySelector("#tabs .seg").addEventListener("scroll", updateTabArrows);
window.addEventListener("resize", updateCarousel);

function applyTab() {
  // "" = no agent chosen: agent-scoped cards hide, the pick prompt shows
  document.body.dataset.agent = currentTab || "";
  const seg = document.querySelector("#tabs .seg");
  for (const b of seg.querySelectorAll(".tab")) {
    b.classList.toggle("active", b.dataset.tab === currentTab);
  }
  updateInk();
  scrollTabIntoView();
  const pin = $("pin-btn");
  pin.classList.toggle("pinned", currentTab === defaultTab);
  pin.dataset.tip = t("tip.pin");
  renderAgentSetup();
  if (lastState) {
    renderSessions(lastState.sessions || []);
    renderEvents(lastState.events || [], lastState.unread || {});
  }
}

// renderAgentSetup swaps the sessions/events lists for a setup guide
// when the current tab's agent isn't ready: not installed → install
// command, not logged in → terminal login, no hook → one-click register.
function renderAgentSetup() {
  const panel = $("agent-setup");
  const card = $("events-card");
  if (typeof tourMock !== "undefined" && tourMock) {
    // the tour shows a populated mock — never the setup guide
    card.classList.remove("setup-mode");
    panel.classList.add("hidden");
    $("beta-chip").classList.add("hidden");
    return;
  }
  const a = agentById(currentTab);
  // claude's hook state comes from the daemon's setup probe, not the
  // agent registry (its registry Hooked is always true)
  const hooked =
    a && a.id === "claude"
      ? !lastState || !lastState.setup || lastState.setup.hooks !== false
      : a && a.hooked;
  const mode =
    !a ? (enabledAgents().length ? "" : "pick") : // nothing chosen → prompt
    !a.installed ? "install" :
    !a.loggedIn ? "login" :
    !hooked ? "hook" : "";
  card.classList.toggle("setup-mode", !!mode);
  panel.classList.toggle("hidden", !mode);
  // every non-Claude agent is beta: tag the card, not the tab button
  $("beta-chip").classList.toggle("hidden", !a || a.id === "claude");
  if (!mode) {
    panel.dataset.mode = "";
    return;
  }
  const key = mode + ":" + (a ? a.id : "-") + ":" + lang;
  if (panel.dataset.mode === key) return;
  panel.dataset.mode = key;
  panel.innerHTML = "";

  if (mode === "pick") {
    const icon = document.createElement("span");
    icon.className = "setup-ic";
    icon.innerHTML = ICONS.sparkles;
    const title = document.createElement("div");
    title.className = "setup-title";
    title.textContent = t("agent.pick");
    const hint = document.createElement("div");
    hint.className = "setup-hint";
    hint.textContent = t("agent.pick.d");
    const btn = document.createElement("button");
    btn.className = "setup-btn";
    btn.textContent = t("agent.pick.btn");
    btn.addEventListener("click", () => openSettings("agents"));
    panel.append(icon, title, hint, btn);
    return;
  }

  const icon = document.createElement("span");
  icon.className = "setup-ic";
  icon.innerHTML =
    mode === "install" ? ICONS.download : mode === "login" ? ICONS.key : ICONS.bellRing;
  const title = document.createElement("div");
  title.className = "setup-title";
  title.textContent = t("agent." + mode).replace("{name}", a.name);
  if (a.id !== "claude") {
    const bc = document.createElement("span");
    bc.className = "beta-chip";
    bc.textContent = "BETA";
    title.appendChild(bc);
  }
  const hint = document.createElement("div");
  hint.className = "setup-hint";
  hint.textContent = t("agent." + mode + ".d").replace("{name}", a.name);
  panel.append(icon, title, hint);

  if (mode === "install") {
    const row = document.createElement("div");
    row.className = "setup-cmd";
    const code = document.createElement("code");
    code.textContent = a.installCmd;
    const cp = document.createElement("button");
    cp.className = "icon-btn";
    cp.innerHTML = ICONS.copy;
    cp.dataset.tip = t("agent.copy");
    cp.addEventListener("click", () => {
      if (navigator.clipboard) navigator.clipboard.writeText(a.installCmd);
      cp.innerHTML = ICONS.check;
      setTimeout(() => (cp.innerHTML = ICONS.copy), 1200);
    });
    row.append(code, cp);
    panel.appendChild(row);
  } else {
    const btn = document.createElement("button");
    btn.className = "setup-btn";
    btn.textContent = t(mode === "login" ? "agent.login.btn" : "agent.hook.btn");
    btn.addEventListener("click", () => {
      btn.disabled = true;
      if (mode === "hook" && a.id === "claude") {
        // claude's hooks are the native install path, not an adapter
        post("/api/setup-fix", { item: "hooks" });
      } else {
        post(mode === "login" ? "/api/agent-login" : "/api/agent-hook", { id: a.id });
      }
      setTimeout(() => (btn.disabled = false), 1500);
    });
    panel.appendChild(btn);
  }
}

document.querySelector("#tabs .seg").addEventListener("click", (e) => {
  const b = e.target.closest(".tab");
  if (!b || !b.dataset.tab || b.dataset.tab === currentTab) return;
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
