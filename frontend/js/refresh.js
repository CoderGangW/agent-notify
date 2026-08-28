"use strict";

async function refresh() {
  if (tourMock) return; // the tour is showing mock data; don't overwrite
  try {
    // welcome open: bypass the daemon's setup cache so a permission
    // granted in System Settings ticks the checklist within one poll
    const welcomeOpen = !$("welcome-overlay").classList.contains("hidden");
    const res = await fetch("/api/state" + (welcomeOpen ? "?fresh=1" : ""));
    const st = await res.json();
    lastState = st;
    $("live-dot").classList.remove("off");
    $("dev-chip").classList.toggle("hidden", !st.dev);
    agents = st.agents || [
      { id: "claude", name: "Claude" },
      { id: "codex", name: "Codex", beta: true },
    ];
    // load the dictionary before anything renders: buildTabs bakes t() output
    // into the DOM and is signature-memoized, so a pre-dict render sticks
    if ((st.lang && st.lang !== lang) || !Object.keys(dict).length) {
      lang = st.lang || lang;
      await loadLang(lang);
      applyI18n();
    }
    buildTabs();
    const enabled = enabledAgents();
    defaultTab = st.defaultTab || (enabled.length ? enabled[0].id : "");
    if (currentTab === null && enabled.length) {
      currentTab = enabled.some((a) => a.id === defaultTab) ? defaultTab : enabled[0].id;
      applyTab();
    } else {
      $("pin-btn").classList.toggle("pinned", currentTab === defaultTab);
    }
    renderSettings(st.settings);
    renderLimits(st.limits || {});
    renderAgyQuota(st.agyQuota || {});
    renderOcUsage(st.ocUsage || {});
    renderUsage(st.usage || { today: {}, week: {} });
    renderAgentSetup();
    if (welcomeOpen && !welcomeFixing) renderWelcomeChecks(st.setup || {});
    renderSessions(st.sessions || []);
    renderEvents(st.events || [], st.unread || {});
    let badgeChanged = false;
    for (const a of agents) {
      const b = $("badge-" + a.id);
      if (!b) continue;
      const n = (st.unread || {})[a.id] || 0;
      const txt = badgesOn() && n > 0 ? String(n) : "";
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
    notifyMode = st.notifyMode || (st.muted ? "quiet" : "on");
    if (mute.dataset.state !== notifyMode) {
      mute.dataset.state = notifyMode;
      mute.innerHTML =
        notifyMode === "silent" ? ICONS.bellOff
        : notifyMode === "quiet" ? ICONS.bellDot
        : notifyMode === "alerts" ? ICONS.bellRing
        : ICONS.bell;
      mute.classList.toggle("muted", notifyMode === "silent");
      mute.classList.toggle("quiet", notifyMode === "quiet");
      mute.classList.toggle("alerts", notifyMode === "alerts");
    }
    mute.dataset.tip = t("tip.mute") + ": " + t("notify." + notifyMode);
    // first real state has rendered: fade the boot overlay away
    const bl = $("boot-loading");
    if (bl && !bl.classList.contains("done")) {
      bl.classList.add("done");
      setTimeout(() => bl.remove(), 420);
    }
  } catch (_) {
    $("live-dot").classList.add("off");
  }
}
