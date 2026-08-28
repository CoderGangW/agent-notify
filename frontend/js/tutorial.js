"use strict";

// ---- onboarding tour: spotlight overlay, first run + help button ----
// The tour runs on MOCK Claude data (see tourEnterMock): a fresh install
// has nothing to show, and every step should point at populated UI.
const TUT_STEPS = [
  { sel: "#tabs .seg", key: "s1" },
  { sel: "#limits-card", key: "s2" },
  { sel: "#usage-card", key: "s3" },
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
  { sel: "#events-card .head-tools", key: "s6", subtab: "sessions" },
  { sel: "#mute-btn", key: "s7" },
  { sel: "footer", key: "s8" },
];
let tutStep = -1;
let tutEls = null;

// ---- tour mock data (Claude) ----
let tourMock = false;
let tourSavedState = null;
let tourSavedTab = null;

function tourMockState() {
  const now = Date.now();
  const iso = (msAgo) => new Date(now - msAgo).toISOString();
  return {
    setup: { hooks: true, autostart: true, claudeCLI: true },
    unread: { claude: 2 },
    sessions: [
      { id: "tm1", source: "claude", state: "tool", tool: "Bash", detail: "npm run build",
        title: "claude-notify", cwd: "~/Projects/claude-notify", branch: "main",
        task: "통계 차트에 드래그 팬 추가", turnStart: iso(4 * 60e3), lastSeen: iso(0) },
      { id: "tm2", source: "claude", state: "working",
        title: "webapp", cwd: "~/Projects/webapp", branch: "feat/auth",
        task: "로그인 흐름 리팩토링", turnStart: iso(95e3), lastSeen: iso(0) },
      { id: "tm3", source: "claude", state: "waiting",
        title: "api-server", cwd: "~/Projects/api", branch: "main",
        task: "마이그레이션 계획 검토", turnStart: iso(11 * 60e3), lastSeen: iso(30e3) },
      { id: "tm4", source: "claude", state: "idle",
        title: "dotfiles", cwd: "~/dotfiles", branch: "",
        task: "zsh 설정 정리", turnStart: iso(30 * 60e3), lastSeen: iso(8 * 60e3) },
    ],
    events: [
      { kind: "done", source: "claude", time: iso(3 * 60e3), title: "webapp",
        message: "JWT 갱신 로직 수정, 테스트 12개 통과", cwd: "~/Projects/webapp",
        branch: "feat/auth", model: "opus", durSec: 312, read: false },
      { kind: "attention", source: "claude", time: iso(9 * 60e3), title: "api-server",
        message: "DB 마이그레이션 실행 전 확인이 필요합니다", cwd: "~/Projects/api",
        branch: "main", model: "sonnet", durSec: 97, read: false },
      { kind: "done", source: "claude", time: iso(42 * 60e3), title: "claude-notify",
        message: "릴리스 노트 초안 작성 완료", cwd: "~/Projects/claude-notify",
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
  const m = tourMockState();
  lastState = Object.assign({}, tourSavedState || {}, m);
  currentTab = "claude";
  applyTab();
  renderLimits(m.limits);
  renderUsage(m.usage);
  const b = $("badge-claude");
  if (b) b.textContent = "2";
}

function tourExitMock() {
  if (!tourMock) return;
  tourMock = false;
  lastState = tourSavedState;
  tourSavedState = null;
  // restore the pre-tour tab; a tab whose agent is no longer enabled
  // falls back to the first enabled one (or the pick-agents state)
  const en = enabledAgents();
  currentTab =
    tourSavedTab && en.some((a) => a.id === tourSavedTab) ? tourSavedTab
    : en.length ? en[0].id
    : null;
  tourSavedTab = null;
  applyTab();
  refresh(); // repaint real data (also restores badges/cards)
}

function tutBuild() {
  const block = document.createElement("div");
  block.id = "tut-block"; // swallows clicks on the UI under the tour
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
  document.body.append(block, hole, card);
  card.querySelector("#tut-skip").addEventListener("click", tutEnd);
  card.querySelector("#tut-prev").addEventListener("click", () => tutGo(tutStep - 1));
  card.querySelector("#tut-next").addEventListener("click", () => {
    if (tutStep >= TUT_STEPS.length - 1) tutEnd();
    else tutGo(tutStep + 1);
  });
  return { block, hole, card };
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
  const { block, hole, card } = tutEls;
  block.style.display = "block";
  const first = hole.style.display !== "block"; // entering the tour
  // clamp the spotlight to the viewport: a target flush with a window
  // edge must not push the hole (and its glow ring) off screen
  const hx = Math.max(6, r.left - pad);
  const hy = Math.max(6, r.top - pad);
  const hw = Math.min(r.right + pad, window.innerWidth - 6) - hx;
  const hh = Math.min(r.bottom + pad, window.innerHeight - 6) - hy;
  hole.style.cssText =
    `display:block;left:${hx}px;top:${hy}px;` +
    `width:${hw}px;height:${hh}px;`;
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
  // first frame of the tour eases in instead of popping
  if (first) {
    hole.classList.remove("shown");
    card.classList.remove("shown");
    requestAnimationFrame(() =>
      requestAnimationFrame(() => {
        hole.classList.add("shown");
        card.classList.add("shown");
      })
    );
  } else {
    hole.classList.add("shown");
    card.classList.add("shown");
  }
}

function tutEnd() {
  tutStep = -1;
  tourExitMock();
  if (tutEls) {
    const { block, hole, card } = tutEls;
    // ease out (the shown class carries the opacity transition), then
    // actually hide once the fade has finished
    hole.classList.remove("shown");
    card.classList.remove("shown");
    setTimeout(() => {
      if (tutStep !== -1) return; // a new tour started meanwhile
      block.style.display = "none";
      hole.style.display = "none";
      card.style.display = "none";
    }, 520);
  }
  try {
    localStorage.setItem("tutorialDone", "1");
  } catch (_) {}
}

function tutStart() {
  tourEnterMock();
  tutGo(0);
}

$("help-btn").addEventListener("click", tutStart);

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
        try {
          const res = await (
            await fetch("/api/setup-fix", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ item: r.item }),
            })
          ).json();
          if (!res.ok) throw new Error(res.error || "");
          await refresh();
          renderWelcomeChecks((lastState && lastState.setup) || {});
        } catch (_) {
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
