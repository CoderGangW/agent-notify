"use strict";

// macOS popover shape, shared by the dashboard and the tray menu. The
// native window is transparent and square (daemon.go), so the page clips
// itself to a rounded rect with a dome at top center pointing up at the
// tray icon. Re-fits on resize (expand animation, menu auto-size).
// ponytail: the dome is always centered; if the window gets clamped at a
// screen edge it won't sit under the icon — pass the icon x if that bites.
if (/Mac/.test(navigator.platform)) {
  const b = 11, r = 12, bw = 28; // dome base y, corner radius, dome half-width
  document.documentElement.classList.add("mac");
  document.documentElement.style.setProperty("--bump", b + "px");
  const shape = () => {
    const w = innerWidth, h = innerHeight, cx = w / 2;
    // dome: each cubic half leaves the top edge flat (control 16px out)
    // and meets the crest flat but tight (control 9px in). Crest radius
    // = 3·9²/(2·10) ≈ 12px — round, not a plateau (wider controls read
    // as a sliced-off top). Crest sits at y=1 so its antialiasing isn't
    // shaved by the window edge.
    document.body.style.clipPath =
      `path("M${r} ${b}H${cx - bw}C${cx - 16} ${b} ${cx - 9} 1 ${cx} 1` +
      `C${cx + 9} 1 ${cx + 16} ${b} ${cx + bw} ${b}H${w - r}` +
      `A${r} ${r} 0 0 1 ${w} ${b + r}V${h - r}A${r} ${r} 0 0 1 ${w - r} ${h}` +
      `H${r}A${r} ${r} 0 0 1 0 ${h - r}V${b + r}A${r} ${r} 0 0 1 ${r} ${b}Z")`;
  };
  shape();
  addEventListener("resize", shape);

  // scale half of the show/hide animation, pivoted on the dome (the
  // daemon fades the window itself — daemon.go trayWin)
  const body = document.body;
  const play = (cls) => {
    body.classList.remove("win-in", "win-out");
    void body.offsetWidth; // restart even when the same class repeats
    body.classList.add(cls);
  };
  window.winIn = () => play("win-in");
  window.winOut = () => play("win-out");
}
