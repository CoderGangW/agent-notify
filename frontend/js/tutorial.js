"use strict";

// ---- onboarding tour: spotlight overlay, first run + help button ----
// The tour runs on MOCK Claude data (see tourEnterMock): a fresh install
// has nothing to show, and every step should point at populated UI.
// Ordered by on-screen position, top → bottom, so the spotlight (and the
// tip card, which sits below targets until it runs out of room) travels
// downward. One deliberate exception: the tour opens on the tabs, then
// steps up to the header bell just above them. Keys keep their i18n
// text; order is array order.
const TUT_STEPS = [
  { sel: "#tabs", key: "s1" }, // whole row: the pin the text mentions sits outside .seg
  { sel: "#mute-btn", key: "s7" },
  { sel: "#limits-card", key: "s2" },
  { sel: "#usage-card", key: "s3" },
  { sel: "#events-card .head-tools", key: "s6", subtab: "sessions" },
  {
    sel: "#events-card", key: "s4", subtab: "sessions",
    legend: [
      { cls: "act work", icon: "sparkles", key: "tut.leg.working" },
      { cls: "act tool", icon: "terminal", key: "tut.leg.tool" },
      { cls: "wait", icon: "bellRing", key: "tut.leg.waiting" },
      { cls: "idle", icon: "check", key: "tut.leg.idle" },
    ],
  },
  { sel: "#events-card", key: "s5", subtab: "events" },
  { sel: "footer", key: "s8" },
];

// ---- tour mock data (Claude) ----
let tourMock = false;
let tourSavedState = null;
let tourSavedTab = null;
let tourSavedSubTab = null;
let tourSavedAgents = null;

// task/message text goes through t(): the mock must read in the UI language
function tourMockState() {
  const now = Date.now();
  const iso = (msAgo) => new Date(now - msAgo).toISOString();
  return {
    setup: { hooks: true, autostart: true },
    unread: { claude: 2 },
    sessions: [
      { id: "tm1", source: "claude", state: "tool", tool: "Bash", detail: "npm run build",
        title: "claude-notify", cwd: "~/Projects/claude-notify", branch: "main",
        task: t("tut.mock.task1"), turnStart: iso(4 * 60e3), lastSeen: iso(0) },
      { id: "tm2", source: "claude", state: "working",
        title: "webapp", cwd: "~/Projects/webapp", branch: "feat/auth",
        task: t("tut.mock.task2"), turnStart: iso(95e3), lastSeen: iso(0) },
      { id: "tm3", source: "claude", state: "waiting",
        title: "api-server", cwd: "~/Projects/api", branch: "main",
        task: t("tut.mock.task3"), turnStart: iso(11 * 60e3), lastSeen: iso(30e3) },
      { id: "tm4", source: "claude", state: "idle",
        title: "dotfiles", cwd: "~/dotfiles", branch: "",
        task: t("tut.mock.task4"), turnStart: iso(30 * 60e3), lastSeen: iso(8 * 60e3) },
    ],
    events: [
      { kind: "done", source: "claude", time: iso(3 * 60e3), title: "webapp",
        message: t("tut.mock.msg1"), cwd: "~/Projects/webapp",
        branch: "feat/auth", model: "opus", durSec: 312, read: false },
      { kind: "attention", source: "claude", time: iso(9 * 60e3), title: "api-server",
        message: t("tut.mock.msg2"), cwd: "~/Projects/api",
        branch: "main", model: "sonnet", durSec: 97, read: false },
      { kind: "done", source: "claude", time: iso(42 * 60e3), title: "claude-notify",
        message: t("tut.mock.msg3"), cwd: "~/Projects/claude-notify",
        branch: "main", model: "opus", durSec: 186, read: true },
    ],
    limits: {
      plan: "Max",
      buckets: [
        { key: "five_hour", utilization: 42, resetsAt: new Date(now + 3 * 3600e3).toISOString() },
        { key: "seven_day", utilization: 18, resetsAt: new Date(now + 5 * 86400e3).toISOString() },
      ],
    },
    usage: {
      today: { input: 184000, output: 52000, cacheRead: 1200000 },
      week: { input: 1900000, output: 410000 },
      todayByModel: [
        { model: "opus", input: 120000, output: 38000 },
        { model: "sonnet", input: 64000, output: 14000 },
      ],
    },
  };
}

// swap lastState for the mock: every existing render path (applyTab,
// applySubTab, legend…) then draws the mock for free. refresh() is
// paused while the tour runs so real polls don't overwrite it.
function tourEnterMock() {
  if (tourMock) return;
  tourMock = true;
  tourSavedState = lastState;
  tourSavedTab = currentTab;
  tourSavedSubTab = subTab;
  tourSavedAgents = agents;
  // the tour walks the Claude tab, so it needs one to point at — a fresh
  // install (no agents picked) or a user who dropped Claude has none
  agents = agents.map((a) => (a.id === "claude" ? Object.assign({}, a, { enabled: true }) : a));
  const m = tourMockState();
  lastState = Object.assign({}, tourSavedState || {}, m);
  currentTab = "claude";
  buildTabs();
  applyTab();
  renderLimits(m.limits);
  renderUsage(m.usage);
  const b = $("badge-claude");
  if (b) {
    b.textContent = "2";
    // the badge widens the tab after applyTab measured it: re-fit the ink
    // pill and the carousel (refresh() does the same on badge changes)
    updateInk();
    updateCarousel();
  }
}

function tourExitMock() {
  if (!tourMock) return;
  tourMock = false;
  lastState = tourSavedState;
  tourSavedState = null;
  agents = tourSavedAgents;
  tourSavedAgents = null;
  // the tour flips to the events sub-tab; hand back whichever was open
  subTab = tourSavedSubTab;
  tourSavedSubTab = null;
  applySubTab();
  // restore the pre-tour tab; a tab whose agent is no longer enabled
  // falls back to the first enabled one (or the pick-agents state)
  const en = enabledAgents();
  currentTab =
    tourSavedTab && en.some((a) => a.id === tourSavedTab) ? tourSavedTab
    : en.length ? en[0].id
    : null;
  tourSavedTab = null;
  buildTabs(); // drops the tour-only Claude tab again
  applyTab();
  refresh(); // repaint real data (also restores badges/cards)
}

// make sure the highlighted region is actually visible
function tutEnter(step) {
  if (currentTab !== "claude") {
    currentTab = "claude";
    applyTab();
  }
  if (step.subtab && subTab !== step.subtab) {
    subTab = step.subtab;
    applySubTab();
  }
}

// Skip, Done, or a settings reset. Done (finished) hands the tour on to
// the session window, whose own Skip/Done closes it and brings this
// window back (cleaner-tour.js).
function tutEnd(finished) {
  spotHide();
  tourExitMock();
  try {
    localStorage.setItem("tutorialDone", "1");
  } catch (_) {}
  if (finished === true) post("/api/cleaner/open", { agent: "claude", tour: true });
}

function tutStart() {
  tourEnterMock();
  // the last step reads "Next": the tour carries on in the session window
  spotStart({ steps: TUT_STEPS, enter: tutEnter, end: tutEnd, last: "tut.next" });
}

$("help-btn").addEventListener("click", tutStart);

// true while a fix request is in flight — the welcome poll skips its
// re-render then, so the disabled button isn't stomped mid-request
let welcomeFixing = false;

function renderWelcomeChecks(setup) {
  // OS-level items only. Agent wiring (hooks, CLI installs) is a per-tab
  // choice made through the in-tab setup guides, never automatic.
  const rows = [];
  if (setup.notifPerm !== undefined && setup.notifPerm >= 0) {
    rows.push({
      ok: setup.notifPerm === 1, key: "welcome.notif",
      miss: setup.notifPerm === 0 ? "welcome.notif.denied" : "welcome.notif.miss",
      item: "notifperm", fixKey: "welcome.allow",
    });
  }
  if (setup.automation !== undefined && setup.automation >= 0) {
    rows.push({
      ok: setup.automation === 1, key: "welcome.automation",
      miss: setup.automation === 0 ? "welcome.automation.denied" : "welcome.automation.miss",
      item: "automation", fixKey: "welcome.allow",
    });
  }
  if (setup.diskAccess !== undefined && setup.diskAccess >= 0) {
    rows.push({
      ok: setup.diskAccess === 1, key: "welcome.disk",
      miss: setup.diskAccess === 0 ? "welcome.disk.denied" : "welcome.disk.miss",
      item: "diskaccess", fixKey: "welcome.allow",
    });
  }
  rows.push({ ok: setup.autostart, key: "welcome.autostart", item: "autostart", fixKey: "welcome.hooks.fix" });
  const list = $("welcome-checks");
  list.innerHTML = "";
  for (const r of rows) {
    const li = document.createElement("li");
    li.className = r.ok ? "ok" : "miss";
    li.innerHTML =
      '<span class="ic">' + (r.ok ? ICONS.check : ICONS.bellOff) + "</span>" +
      "<div><span></span>" +
      (!r.ok && r.miss ? "<small>" + t(r.miss) + "</small>" : "") +
      "</div>";
    li.querySelector("div > span").textContent = t(r.key);
    if (!r.ok) {
      const btn = document.createElement("button");
      btn.className = "fix-btn";
      btn.textContent = t(r.fixKey);
      btn.addEventListener("click", async () => {
        btn.disabled = true;
        btn.textContent = t("welcome.fixing");
        welcomeFixing = true;
        try {
          const res = await (
            await fetch("/api/setup-fix", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ item: r.item }),
            })
          ).json();
          if (!res.ok) throw new Error(res.error || "");
          welcomeFixing = false;
          await refresh();
          renderWelcomeChecks((lastState && lastState.setup) || {});
        } catch (_) {
          welcomeFixing = false;
          btn.disabled = false;
          btn.textContent = t("welcome.fixfail") + " · " + t(r.fixKey);
        }
      });
      li.appendChild(btn);
    }
    list.appendChild(li);
  }
}

// Apple-style intro between the welcome screen and the tour: the window
// frosts over, the greeting inks itself in character by character, then
// a call-to-action rises from the bottom; its button starts the tour.
function greetIntro(onDone) {
  const name =
    ((lastState && lastState.limits && lastState.limits.accountName) || "").trim();
  const ov = document.createElement("div");
  ov.id = "greet-ov";
  const lines = [
    name ? t("greet.l1").replace("{name}", name) : "",
    t("greet.l2"),
  ].filter(Boolean);
  let delay = 350; // let the blur settle first
  for (const line of lines) {
    const div = document.createElement("div");
    div.className = "greet-line";
    for (const ch of line) {
      const s = document.createElement("span");
      s.className = "greet-ch";
      s.textContent = ch === " " ? "\u00A0" : ch;
      s.style.animationDelay = delay + "ms";
      delay += 65;
      div.appendChild(s);
    }
    delay += 220; // beat between lines
    ov.appendChild(div);
  }
  const cta = document.createElement("div");
  cta.className = "greet-cta";
  cta.innerHTML = '<p></p><button class="tut-next"></button>';
  cta.querySelector("p").textContent = t("greet.sub");
  const btn = cta.querySelector("button");
  btn.textContent = t("greet.start");
  btn.addEventListener("click", () => {
    ov.classList.add("out"); // whole-screen fade, then the tour begins
    setTimeout(() => {
      ov.remove();
      if (onDone) onDone();
    }, 750);
  });
  ov.appendChild(cta);
  document.body.appendChild(ov);
  setTimeout(() => cta.classList.add("show"), delay + 550);
}

function welcomeClose(startTour) {
  if (startTour) {
    // mount the frost first, hide the welcome a frame later — the main
    // UI must never peek through between the two screens
    greetIntro(tutStart);
    requestAnimationFrame(() =>
      requestAnimationFrame(() => $("welcome-overlay").classList.add("hidden"))
    );
  } else {
    $("welcome-overlay").classList.add("hidden");
    tutEnd(); // marks tutorialDone so it never auto-shows again
  }
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
