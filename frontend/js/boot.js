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
// macOS popover shape: the native window is transparent (daemon.go), so
// the page clips itself to a rounded rect with a bump at top center
// pointing up at the tray icon. Re-fit on resize (expand animation).
// ponytail: bump is always centered; if the window gets clamped at a
// screen edge it won't sit under the icon — pass the icon x if that bites.
if (/Mac/.test(navigator.platform)) {
  const b = 9, r = 12, bw = 22; // bump height, corner radius, bump half-width
  document.documentElement.classList.add("mac");
  document.documentElement.style.setProperty("--bump", b + "px");
  const shape = () => {
    const w = innerWidth, h = innerHeight, cx = w / 2;
    document.body.style.clipPath =
      `path("M${r} ${b}H${cx - bw}C${cx - bw / 2} ${b} ${cx - 5} 0 ${cx} 0` +
      `C${cx + 5} 0 ${cx + bw / 2} ${b} ${cx + bw} ${b}H${w - r}` +
      `A${r} ${r} 0 0 1 ${w} ${b + r}V${h - r}A${r} ${r} 0 0 1 ${w - r} ${h}` +
      `H${r}A${r} ${r} 0 0 1 0 ${h - r}V${b + r}A${r} ${r} 0 0 1 ${r} ${b}Z")`;
  };
  shape();
  addEventListener("resize", shape);
}
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
