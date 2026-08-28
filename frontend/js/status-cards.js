"use strict";

function renderLimits(lim) {
  const body = $("limits-body");
  const note = $("limits-note");
  note.textContent = lim.error ? lim.error : lim.account || "";
  if (!lim.buckets || lim.buckets.length === 0) {
    body.innerHTML =
      '<div class="limits-err">' +
      (lim.error || t("limits.none")) +
      "</div>";
    return;
  }
  for (const b of lim.buckets) {
    const id = "gauge-" + b.key + (b.model ? "-" + b.model : "");
    let el = $(id);
    if (!el) {
      el = document.createElement("div");
      el.className = "gauge";
      el.id = id;
      el.innerHTML =
        '<div class="row"><span class="name"></span><span class="meta"></span></div>' +
        '<div class="bar"><i></i></div>';
      body.appendChild(el);
    }
    const pct = Math.min(100, Math.max(0, b.utilization));
    el.querySelector(".name").textContent = b.model
      ? (t("bucket.weekly_model") || "Weekly {model}").replace("{model}", b.model)
      : t("bucket." + b.key) || b.key;
    el.querySelector(".meta").textContent =
      pct.toFixed(0) + "% · " + fmtReset(b.resetsAt);
    const bar = el.querySelector(".bar > i");
    bar.className = pct >= 90 ? "bad" : pct >= 70 ? "warn" : "";
    // Setting the width in the creation frame skips the transition; a
    // rAF hop lets the 0-width state paint first so the bar rises.
    requestAnimationFrame(() =>
      requestAnimationFrame(() => (bar.style.width = pct + "%"))
    );
  }
}

// renderAgyQuota draws the Antigravity model-quota gauges. Unlike the
// Claude limits card (which shows utilization), the bar here is REMAINING
// headroom — full bar = lots left — so the color inverts: a nearly-empty
// pool goes warn/bad. Claude and GPT share one Vertex pool; Gemini its own.
function renderAgyQuota(q) {
  const body = $("agy-quota-body");
  const note = $("agy-quota-note");
  const pools = q.pools || [];
  note.textContent = q.error ? q.error : pools.length ? t("agyquota.pooled") : "";
  if (pools.length === 0) {
    body.innerHTML =
      '<div class="limits-err">' + (q.error || t("agyquota.none")) + "</div>";
    return;
  }
  const live = new Set();
  for (const p of pools) {
    const id = "agy-gauge-" + p.key;
    live.add(id);
    let el = $(id);
    if (!el) {
      el = document.createElement("div");
      el.className = "gauge";
      el.id = id;
      el.innerHTML =
        '<div class="row"><span class="name"></span><span class="meta"></span></div>' +
        '<div class="bar"><i></i></div>';
      body.appendChild(el);
    }
    const pct = Math.min(100, Math.max(0, p.fraction * 100));
    el.querySelector(".name").textContent = p.label;
    el.querySelector(".meta").textContent =
      t("agyquota.left").replace("{pct}", pct.toFixed(0)) +
      (p.resetsAt ? " · " + fmtReset(p.resetsAt) : "");
    const bar = el.querySelector(".bar > i");
    bar.className = pct <= 10 ? "bad" : pct <= 30 ? "warn" : "";
    requestAnimationFrame(() =>
      requestAnimationFrame(() => (bar.style.width = pct + "%"))
    );
  }
  // Drop gauges whose pool vanished (e.g. account/model change).
  for (const el of [...body.children]) {
    if (el.id && el.id.startsWith("agy-gauge-") && !live.has(el.id)) el.remove();
  }
}

// rollTo renders txt into el as per-character slots. A changed digit rolls
// like an odometer: increased digits exit up / enter from below, decreased
// digits exit down / enter from above. Non-digit chars (units, dots)
// follow the overall direction of the value.
function rollTo(el, txt, rawVal) {
  const old = el.dataset.txt !== undefined ? el.dataset.txt : el.textContent;
  if (old === txt) return;
  const prevVal = Number(el.dataset.val);
  const up = isNaN(prevVal) || rawVal >= prevVal; // overall direction
  el.dataset.txt = txt;
  el.dataset.val = String(rawVal);
  el.innerHTML = "";
  el.classList.add("roll");

  // Align by place value: compare characters from the right.
  const n = Math.max(old.length, txt.length);
  const slots = [];
  for (let i = n - 1; i >= 0; i--) {
    const oldCh = old[old.length - 1 - i];
    const newCh = txt[txt.length - 1 - i];
    const slot = document.createElement("span");
    slot.className = "slot";
    if (newCh === undefined) continue; // value got shorter; char just drops
    if (oldCh === newCh) {
      slot.textContent = newCh;
    } else {
      let dirUp = up;
      if (oldCh >= "0" && oldCh <= "9" && newCh >= "0" && newCh <= "9") {
        dirUp = newCh > oldCh;
      }
      const enter = document.createElement("span");
      enter.className = "ch " + (dirUp ? "in-up" : "in-down");
      enter.textContent = newCh;
      slot.appendChild(enter);
      if (oldCh !== undefined) {
        const exit = document.createElement("span");
        exit.className = "ch out " + (dirUp ? "out-up" : "out-down");
        exit.textContent = oldCh;
        exit.addEventListener("animationend", () => exit.remove(), { once: true });
        slot.appendChild(exit);
      }
    }
    slots.push(slot);
  }
  // The loop walks i = n-1 → 0, i.e. leftmost slot first: append in order.
  for (const s of slots) el.appendChild(s);
}

function setStat(id, value) {
  rollTo($(id), fmtTokens(value), value);
}

function shortModel(m) {
  return m
    .replace(/^claude-/, "")
    .replace(/-\d{8}$/, "")
    .replace(/-(\d+)-(\d+)$/, " $1.$2");
}

function renderUsage(u) {
  setStat("u-today", u.today.input + u.today.output);
  setStat("u-week", u.week.input + u.week.output);
  setStat("u-out", u.today.output);
  setStat("u-cache", u.today.cacheRead);
  // Diff the model chips in place: rebuilding them every poll replays
  // the enter animation and reads as flicker.
  const models = $("models");
  const want = (u.todayByModel || []).slice(0, 4);
  const seen = new Set();
  for (const m of want) {
    seen.add(m.model);
    let chip = models.querySelector(
      '.chip[data-model="' + CSS.escape(m.model) + '"]'
    );
    if (chip && chip.classList.contains("leave")) {
      chip.classList.remove("leave"); // came back mid-exit
      chip.classList.add("enter");
    }
    if (!chip) {
      chip = document.createElement("span");
      chip.className = "chip enter";
      chip.dataset.model = m.model;
      chip.innerHTML = "<b></b> <span class=\"val\"></span>";
      chip.querySelector("b").textContent = shortModel(m.model);
      models.appendChild(chip);
    }
    rollTo(chip.querySelector(".val"), fmtTokens(m.input + m.output), m.input + m.output);
  }
  for (const chip of [...models.querySelectorAll(".chip")]) {
    if (!seen.has(chip.dataset.model) && !chip.classList.contains("leave")) {
      chip.classList.remove("enter");
      chip.classList.add("leave");
      chip.addEventListener("animationend", () => chip.remove(), { once: true });
    }
  }
}
