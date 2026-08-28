"use strict";

const $ = (id) => document.getElementById(id);

// ---- custom tooltip: near-instant, animated, viewport-clamped ----
const tipEl = document.createElement("div");
tipEl.id = "tooltip";
document.addEventListener("DOMContentLoaded", () => document.body.appendChild(tipEl));
let tipTimer = null;
let tipTarget = null;

function showTip(target) {
  const txt = target.dataset.tip;
  if (!txt) return;
  tipEl.textContent = txt;
  tipEl.classList.add("show");
  const r = target.getBoundingClientRect();
  const tw = tipEl.offsetWidth;
  const th = tipEl.offsetHeight;
  let x = r.left + r.width / 2 - tw / 2;
  x = Math.max(6, Math.min(x, window.innerWidth - tw - 6));
  let y = r.top - th - 9;
  tipEl.classList.toggle("below", y < 4);
  if (y < 4) y = r.bottom + 9;
  tipEl.style.left = x + "px";
  tipEl.style.top = y + "px";
  // caret tracks the hovered element's center even when clamped
  const ax = Math.max(10, Math.min(r.left + r.width / 2 - x, tw - 10));
  tipEl.style.setProperty("--ax", ax + "px");
}

document.addEventListener("mouseover", (e) => {
  const target = e.target.closest("[data-tip]");
  if (target === tipTarget) return;
  tipTarget = target;
  clearTimeout(tipTimer);
  if (!target) {
    tipEl.classList.remove("show");
    return;
  }
  tipTimer = setTimeout(() => showTip(target), 150);
});
document.addEventListener("mousedown", () => {
  clearTimeout(tipTimer);
  tipEl.classList.remove("show");
});
