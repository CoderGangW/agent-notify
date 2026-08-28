"use strict";

function fmtTokens(n) {
  if (n >= 1e9) return (n / 1e9).toFixed(1) + "B";
  if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
  if (n >= 1e3) return (n / 1e3).toFixed(1) + "k";
  return String(n);
}

function fmtDur(sec) {
  if (sec < 60) return sec + "s";
  const m = Math.floor(sec / 60);
  if (m < 60) return m + "m";
  return Math.floor(m / 60) + "h " + (m % 60) + "m";
}

function fmtReset(iso) {
  const ms = new Date(iso) - Date.now();
  if (isNaN(ms) || ms <= 0) return t("reset.now");
  const h = Math.floor(ms / 3600000);
  const m = Math.floor((ms % 3600000) / 60000);
  const tpl = h >= 24 ? t("reset.day") : h > 0 ? t("reset.hour") : t("reset.min");
  return tpl
    .replace("{d}", Math.floor(h / 24))
    .replace("{h}", h >= 24 ? h % 24 : h)
    .replace("{m}", m);
}

// reconcile keeps DOM rows keyed and updates them in place — full
// innerHTML rebuilds every poll made the whole card flash and dropped
// hover/animation state.
function reconcile(list, items, keyFn, createFn, updateFn) {
  const byKey = new Map();
  for (const el of [...list.children]) byKey.set(el.dataset.key, el);
  let prev = null;
  items.forEach((it, i) => {
    const k = keyFn(it, i);
    let el = byKey.get(k);
    if (el) byKey.delete(k);
    else {
      el = createFn(it);
      el.dataset.key = k;
    }
    // insert before updating: updateFn may measure layout (overflow checks)
    const want = prev ? prev.nextSibling : list.firstChild;
    if (el !== want) list.insertBefore(el, want);
    updateFn(el, it, i);
    prev = el;
  });
  // rows animating out remove themselves on transitionend
  for (const el of byKey.values()) if (!el.classList.contains("leave")) el.remove();
}

function setText(el, txt) {
  if (el.textContent !== txt) el.textContent = txt;
}
