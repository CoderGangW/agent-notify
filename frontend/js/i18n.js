"use strict";

// ---- i18n: dictionaries load from /i18n/<lang>.json (shared with Go) ----
let lang = "ko";
let dict = {};
let dictEn = {};
let agents = []; // agent registry + setup status, from /api/state
const agentById = (id) => agents.find((a) => a.id === id);
const agentName = () => {
  if (currentTab === "claude" || !currentTab) return "Claude Code";
  const a = agentById(currentTab);
  return a ? a.name : currentTab;
};
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
  $("stats-btn").dataset.tip = t("tip.stats");
  $("search-btn").dataset.tip = t("tip.search");
  $("expand-btn").dataset.tip = t(winTall ? "tip.collapse" : "tip.expand");
  $("search-close").dataset.tip = t("search.close");
  $("search-input").placeholder = t("search.placeholder");
  const moreTab = document.querySelector("#tabs .more-tab");
  if (moreTab) moreTab.dataset.tip = t("tip.moreAgents");
  if (searchOpen) renderSearchChips(); // chip labels follow the language
  ddTheme.render();
}
