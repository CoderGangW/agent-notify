"use strict";

$("mute-btn").addEventListener("click", () => post("/api/mute"));
$("clear-btn").addEventListener("click", () => post("/api/clear"));
$("readall-btn").addEventListener("click", () =>
  post("/api/read-all", { source: currentTab || "claude" })
);
$("quit-btn").addEventListener("click", () => post("/api/quit"));
$("restart-btn").addEventListener("click", () => {
  fetch("/api/restart", { method: "POST" }).catch(() => {});
  $("live-dot").classList.add("off"); // window dies with the daemon; dot for the beat before it does
});

$("quit-btn").innerHTML = ICONS.power;
$("help-btn").innerHTML = ICONS.help;
$("settings-btn").innerHTML = ICONS.gear;
$("stats-btn").innerHTML = ICONS.chart;
$("stats-close").innerHTML = ICONS.x;
$("settings-close").innerHTML = ICONS.x;
$("restart-btn").innerHTML = ICONS.rotate;
$("gh-btn").innerHTML = ICONS.github;
$("gh-btn").addEventListener("click", () => post("/api/github"));
$("mute-btn").innerHTML = ICONS.bell;
$("pin-btn").innerHTML = ICONS.pin;
$("search-btn").innerHTML = ICONS.search;
$("expand-btn").innerHTML = ICONS.expand;
$("search-close").innerHTML = ICONS.x;
$("search-clear").innerHTML = ICONS.x;
document.querySelector("#search-bar .s-ic").innerHTML = ICONS.search;
// pin the boot overlay exactly under the header (CSS guesses 52px)
{
  const bl = $("boot-loading");
  const head = document.querySelector("header");
  if (bl && head) {
    bl.style.top = Math.round(head.getBoundingClientRect().bottom) + 2 + "px";
  }
}
applySubTab();
refresh();
setInterval(refresh, 2500);
