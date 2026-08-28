"use strict";

// ---- custom dropdown ----
const dropdowns = [];
function makeDropdown(id, getOptions, onChange) {
  const root = $(id);
  root.innerHTML =
    '<button class="dd-btn"><span class="dd-val"></span>' + ICONS.chevron + "</button>" +
    '<div class="dd-menu"></div>';
  const btn = root.querySelector(".dd-btn");
  const menu = root.querySelector(".dd-menu");
  const dd = {
    root,
    value: null,
    set(v) {
      dd.value = v;
      dd.render();
    },
    render() {
      const opts = getOptions();
      const cur = opts.find((o) => o.v === dd.value) || opts[0];
      root.querySelector(".dd-val").textContent = cur ? cur.label : "";
      menu.innerHTML = "";
      for (const o of opts) {
        const item = document.createElement("button");
        item.className = "dd-item" + (o.v === dd.value ? " sel" : "");
        item.innerHTML = "<span></span>" + (o.v === dd.value ? ICONS.check : "");
        item.querySelector("span").textContent = o.label;
        item.addEventListener("click", (e) => {
          e.stopPropagation();
          dd.close();
          if (o.v !== dd.value) {
            dd.set(o.v);
            onChange(o.v);
          }
        });
        menu.appendChild(item);
      }
    },
    open() {
      for (const other of dropdowns) if (other !== dd) other.close();
      dd.render();
      root.classList.add("open");
    },
    close() {
      root.classList.remove("open");
    },
  };
  btn.addEventListener("click", (e) => {
    e.stopPropagation();
    root.classList.contains("open") ? dd.close() : dd.open();
  });
  dropdowns.push(dd);
  return dd;
}
document.addEventListener("click", () => dropdowns.forEach((d) => d.close()));
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") dropdowns.forEach((d) => d.close());
});

const ddLang = makeDropdown(
  "dd-lang",
  () => [
    { v: "auto", label: t("lang.auto") },
    { v: "en", label: "English" },
    { v: "ko", label: "한국어" },
    { v: "zh", label: "中文" },
  ],
  (v) => post("/api/settings", { lang: v })
);
const ddTheme = makeDropdown(
  "dd-theme",
  () => [
    { v: "auto", label: t("theme.auto") },
    { v: "light", label: t("theme.light") },
    { v: "dark", label: t("theme.dark") },
  ],
  (v) => {
    themeSetting = v;
    applyTheme();
    post("/api/settings", { theme: v });
  }
);
// the row's description explains whatever mode is currently picked
function updateNotifyDesc(v) {
  const el = document.querySelector('[data-i18n="set.notify.d"]');
  if (el) el.textContent = t("notify." + v + ".d");
}
const ddNotify = makeDropdown(
  "dd-notify",
  () => ["on", "alerts", "quiet", "silent"].map((v) => ({ v, label: t("notify." + v) })),
  (v) => {
    updateNotifyDesc(v);
    post("/api/settings", { notifyMode: v });
  }
);
const ddTab = makeDropdown(
  "dd-tab",
  () => enabledAgents().map((a) => ({ v: a.id, label: a.name })),
  (v) => post("/api/settings", { defaultTab: v })
);

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


// ---- settings panel ----
let settingsOpen = false;
function toggleSettings(open) {
  settingsOpen = open === undefined ? !settingsOpen : open;
  $("settings-overlay").classList.toggle("hidden", !settingsOpen);
  $("settings-btn").classList.toggle("active", settingsOpen);
}

// openSettings("agents") opens the panel with the agent row spotlighted —
// the "+" tab lands here
function openSettings(section) {
  toggleSettings(true);
  if (section === "agents") {
    const row = $("agents-row");
    row.classList.remove("flash");
    void row.offsetWidth;
    row.classList.add("flash");
  }
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
  if ((cfg.theme || "auto") !== themeSetting) {
    themeSetting = cfg.theme || "auto";
    applyTheme();
  }
  ddTheme.set(cfg.theme || "auto");
  ddLang.set(cfg.lang || "auto");
  // no agents chosen → nothing to default to: empty + disabled
  const en = enabledAgents();
  const tabRoot = $("dd-tab");
  tabRoot.classList.toggle("disabled", !en.length);
  tabRoot.querySelector(".dd-btn").disabled = !en.length;
  if (en.length) {
    ddTab.set(
      en.some((a) => a.id === cfg.defaultTab) ? cfg.defaultTab : en[0].id
    );
  } else {
    tabRoot.querySelector(".dd-val").textContent = "–";
  }
  renderAgentPicks();
  const nm = cfg.notifyMode || (cfg.muted ? "quiet" : "on");
  ddNotify.set(nm);
  updateNotifyDesc(nm);
  $("sw-ai").classList.toggle("on", !cfg.disableAISummary);
  $("sw-live").classList.toggle("on", !cfg.disableLiveStatus);
  $("sw-update").classList.toggle("on", !cfg.disableAutoUpdate);
  $("sw-autostart").classList.toggle("on", !cfg.disableAutostart);
}

// agent chip picker: toggle which agents get a tab; claude is locked on
function renderAgentPicks() {
  const box = $("agent-picks");
  const sig = agents.map((a) => a.id + ":" + (a.enabled !== false)).join(",") + ":" + lang;
  if (box.dataset.sig === sig) return;
  box.dataset.sig = sig;
  box.innerHTML = "";
  for (const a of agents) {
    const chip = document.createElement("button");
    chip.className = "agent-pick" + (a.enabled !== false ? " on" : "");
    chip.dataset.agent = a.id;
    chip.textContent = a.name;
    chip.addEventListener("click", () => {
      chip.classList.toggle("on");
      const picked = [...box.querySelectorAll(".agent-pick.on")].map((c) => c.dataset.agent);
      // optimistic local update so the tab strip reacts instantly
      for (const ag of agents) ag.enabled = picked.includes(ag.id);
      buildTabs();
      post("/api/settings", { agents: picked });
    });
    box.appendChild(chip);
  }
}

function bindSwitch(id, build) {
  $(id).addEventListener("click", () => {
    const el = $(id);
    el.classList.toggle("on");
    post("/api/settings", build(el.classList.contains("on")));
  });
}
bindSwitch("sw-ai", (on) => ({ disableAISummary: !on }));
bindSwitch("sw-live", (on) => ({ disableLiveStatus: !on }));
bindSwitch("sw-update", (on) => ({ disableAutoUpdate: !on }));
bindSwitch("sw-autostart", (on) => ({ autostart: on }));
$("reset-onboarding").addEventListener("click", () => {
  try {
    localStorage.removeItem("tutorialDone");
  } catch (_) {}
  toggleSettings(false);
  tutEnd(); // clear any running tour state (also re-sets the flag —
  try {
    localStorage.removeItem("tutorialDone"); // — so clear it again)
  } catch (_) {}
  renderWelcomeChecks((lastState && lastState.setup) || {});
  $("welcome-overlay").classList.remove("hidden");
});
