"use strict";

// ---- spotlight tour engine, shared by the dashboard and the session window ----
// A tour is { steps, enter, end, last }: each step is { sel, key, legend?, at? }
// with its text at tut.<key>.t / .b (at: "bottom" pins a card that fits
// nowhere to the window bottom, not the top); enter(step) readies the page
// before the target is measured; end(finished) runs on Skip (false) or on
// Next past the last step (true) and must call spotHide(); last is the
// final button's i18n key (default tut.done).
let spotTour = null;
let spotStep = -1;
let spotEls = null;

function spotBuild() {
  const block = document.createElement("div");
  block.id = "tut-block"; // swallows clicks on the UI under the tour
  const hole = document.createElement("div");
  hole.id = "tut-hole";
  const card = document.createElement("div");
  card.id = "tut-card";
  card.innerHTML =
    '<h3></h3><p></p><ul class="tut-legend"></ul>' +
    '<div class="tut-row"><span class="dots"></span><span class="btns">' +
    '<button class="text-btn" id="tut-skip"></button>' +
    '<button class="text-btn" id="tut-prev"></button>' +
    '<button class="tut-next" id="tut-next"></button></span></div>';
  document.body.append(block, hole, card);
  card.querySelector("#tut-skip").addEventListener("click", () => spotTour.end(false));
  card.querySelector("#tut-prev").addEventListener("click", () => spotGo(spotStep - 1));
  card.querySelector("#tut-next").addEventListener("click", () => {
    if (spotStep >= spotTour.steps.length - 1) spotTour.end(true);
    else spotGo(spotStep + 1);
  });
  return { block, hole, card };
}

function spotStart(tour) {
  spotTour = tour;
  spotGo(0);
}

function spotGo(i) {
  const steps = spotTour.steps;
  const step = steps[i];
  if (!step) return spotTour.end(false);
  spotStep = i;
  if (!spotEls) spotEls = spotBuild();
  if (spotTour.enter) spotTour.enter(step);
  const target = document.querySelector(step.sel);
  if (!target) return spotTour.end(false);
  const r = target.getBoundingClientRect();
  const pad = 6;
  const { block, hole, card } = spotEls;
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
  card.querySelector(".dots").innerHTML = steps.map(
    (_, d) => '<i class="' + (d === i ? "on" : "") + '"></i>'
  ).join("");
  card.querySelector("#tut-skip").textContent = t("tut.skip");
  const prev = card.querySelector("#tut-prev");
  prev.textContent = t("tut.prev");
  prev.style.visibility = i === 0 ? "hidden" : "visible";
  card.querySelector("#tut-next").textContent =
    i === steps.length - 1 ? t(spotTour.last || "tut.done") : t("tut.next");
  // card below the hole, else above, else beside it (a narrow card next to
  // a tall target), else pinned to the top (or bottom, step.at). A
  // full-width card (dashboard) never fits beside, and its clamped x is
  // just the CSS inset.
  const ch = card.offsetHeight;
  const cw = card.offsetWidth;
  const gap = pad + 10;
  let x = Math.min(Math.max(16, r.left), window.innerWidth - cw - 16);
  let y = r.bottom + gap;
  if (y + ch > window.innerHeight - 8) y = r.top - gap - ch;
  if (y < 8) {
    const side =
      r.right + gap + cw <= window.innerWidth - 16 ? r.right + gap
      : r.left - gap - cw >= 16 ? r.left - gap - cw
      : null;
    if (side === null) y = step.at === "bottom" ? window.innerHeight - ch - 8 : 8;
    else {
      x = side;
      y = Math.min(Math.max(8, r.top), window.innerHeight - ch - 8);
    }
  }
  card.style.left = x + "px";
  card.style.top = y + "px";
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

function spotHide() {
  spotTour = null;
  spotStep = -1;
  if (!spotEls) return;
  const { block, hole, card } = spotEls;
  // ease out (the shown class carries the opacity transition), then
  // actually hide once the fade has finished
  hole.classList.remove("shown");
  card.classList.remove("shown");
  setTimeout(() => {
    if (spotStep !== -1) return; // a new tour started meanwhile
    block.style.display = "none";
    hole.style.display = "none";
    card.style.display = "none";
  }, 520);
}
