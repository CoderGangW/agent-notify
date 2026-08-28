"use strict";

// ---- session context menu: right-click a row to exclude it ----
const ctxEl = document.createElement("div");
ctxEl.id = "ctxmenu";
document.addEventListener("DOMContentLoaded", () => document.body.appendChild(ctxEl));

function closeCtx() {
  ctxEl.classList.remove("show");
}

function openCtx(e, sid) {
  ctxEl.innerHTML = "";
  const btn = document.createElement("button");
  btn.className = "ctx-item";
  btn.innerHTML = ICONS.eyeOff + "<span></span>";
  btn.querySelector("span").textContent = t("sessions.hide");
  btn.addEventListener("click", () => {
    hideSession(sid);
    closeCtx();
  });
  ctxEl.appendChild(btn);
  ctxEl.classList.add("show");
  const x = Math.max(6, Math.min(e.clientX, window.innerWidth - ctxEl.offsetWidth - 6));
  const y = Math.max(6, Math.min(e.clientY, window.innerHeight - ctxEl.offsetHeight - 6));
  ctxEl.style.left = x + "px";
  ctxEl.style.top = y + "px";
}

document.addEventListener("mousedown", (e) => {
  if (!ctxEl.contains(e.target)) closeCtx();
});
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape") closeCtx();
});
window.addEventListener("blur", closeCtx);
document.addEventListener("scroll", closeCtx, true);

const hiddenPending = new Set(); // filtered locally until the daemon's list confirms

function hideSession(sid) {
  hiddenPending.add(sid);
  post("/api/hide-session", { id: sid });
  const li = document.querySelector('#sessions li[data-sid="' + CSS.escape(sid) + '"]');
  if (!li) return;
  // collapse smoothly: pin the current height, then transition to 0
  li.style.height = li.offsetHeight + "px";
  void li.offsetHeight; // reflow so the transition starts from the pinned value
  li.classList.add("leave");
  li.addEventListener("transitionend", (ev) => {
    if (ev.propertyName !== "height") return; // opacity/transform finish earlier
    li.remove();
    if (lastState) renderSessions(lastState.sessions || []); // empty state may need to appear
  });
}

function renderSessions(all) {
  const tab = currentTab || "claude";
  // drop locally-hidden rows; forget ids the daemon no longer reports
  const ids = new Set(all.map((s) => s.id));
  for (const sid of hiddenPending) if (!ids.has(sid)) hiddenPending.delete(sid);
  let sessions = all.filter(
    (s) => (s.source || "claude") === tab && !hiddenPending.has(s.id)
  );
  $("sess-badge").textContent = "";
  const busy = sessions.filter((s) => s.state === "tool" || s.state === "working").length;
  if (busy > 0) $("sess-badge").textContent = String(busy);
  if (subTab !== "sessions") return;
  const total = sessions.length; // badge/empty-copy still speak for the full list
  if (searchOpen) sessions = sessions.filter(sessSearchPass);
  setSearchCount(sessions.length, total);
  const list = $("sessions");
  const empty = $("empty");
  // a row mid-collapse keeps the list visible until its transition ends
  const showEmpty = sessions.length === 0 && !list.querySelector(".leave");
  empty.classList.toggle("hidden", !showEmpty);
  list.classList.toggle("hidden", showEmpty);
  if (sessions.length === 0) {
    if (showEmpty) {
      list.innerHTML = "";
      const filtered = searchOpen && searchActive() && total > 0;
      empty.querySelector("[data-i18n]").textContent =
        t(filtered ? "search.none" : "sessions.empty");
      empty.querySelectorAll("span")[1].textContent =
        t(filtered ? "search.none.hint" : "sessions.hint");
    }
    return;
  }

  reconcile(
    list,
    sessions,
    (s) => s.id,
    (s) => {
      const li = document.createElement("li");
      li.innerHTML =
        '<span class="ic"></span>' +
        '<div class="body">' +
        '<div class="head"><span class="name"></span><span class="branch"></span><span class="proj"></span></div>' +
        '<div class="meta"><span class="statetxt"></span><i class="sep">·</i>' +
        '<span class="elapsed" data-ts=""></span></div>' +
        '<div class="task"></div>' +
        "</div>";
      li.querySelector(".proj").addEventListener("click", (e) => {
        e.stopPropagation(); // chip opens the folder; the row focuses the window
        post("/api/folder", { id: li.dataset.sid });
      });
      li.addEventListener("click", () =>
        post("/api/focus-session", { id: li.dataset.sid })
      );
      li.addEventListener("contextmenu", (e) => {
        e.preventDefault();
        openCtx(e, li.dataset.sid);
      });
      return li;
    },
    (li, s) => {
      li.dataset.sid = s.id;
      li.className = "ev sess " + s.state;
      const ic = li.querySelector(".ic");
      // key includes the tool icon: Bash → Read swaps the glyph, same state
      const ikey = s.state + (s.state === "tool" ? ":" + toolIconName(s.tool) : "");
      if (ic.dataset.state !== ikey) {
        ic.dataset.state = ikey;
        if (s.state === "tool") { ic.className = "ic act tool"; ic.innerHTML = ICONS[toolIconName(s.tool)]; }
        else if (s.state === "working") { ic.className = "ic act work"; ic.innerHTML = ICONS.sparkles; }
        else if (s.state === "waiting") { ic.className = "ic wait"; ic.innerHTML = ICONS.bellRing; }
        else { ic.className = "ic idle"; ic.innerHTML = ICONS.check; }
      }
      // outside the state guard: the tool name can change while state stays "tool"
      ic.dataset.tip = sessionStateText(s);
      setText(li.querySelector(".name"),
        s.title || (s.cwd || "").split(/[\\/]/).pop() || "claude");
      const br = li.querySelector(".branch");
      if (s.branch) {
        if (!br.firstChild) br.innerHTML = ICONS.branch + "<span></span>";
        setText(br.querySelector("span"), s.branch);
        br.style.display = "";
      } else br.style.display = "none";
      const proj = li.querySelector(".proj");
      if (!proj.firstChild) proj.innerHTML = ICONS.focus + "<span></span>";
      setText(proj.querySelector("span"), (s.cwd || "").split(/[\\/]/).pop() || "");
      proj.dataset.tip = s.cwd || "";
      setText(li.querySelector(".statetxt"), sessionStateText(s));
      const task = li.querySelector(".task");
      const tkey = "task:" + s.id;
      task.style.display = s.task ? "" : "none";
      const tChanged = task.dataset.txt !== (s.task || "");
      if (tChanged) {
        task.dataset.txt = s.task || "";
        task.textContent = s.task || "";
      }
      if (s.task) {
        const isOpen = expanded.has(tkey);
        task.classList.toggle("clamp", !isOpen);
        let btn = li.querySelector(".more-btn");
        if (tChanged || !btn) {
          const overflows = isOpen || task.scrollHeight > task.clientHeight + 1;
          if (overflows && !btn) {
            btn = document.createElement("button");
            btn.className = "more-btn" + (isOpen ? " open" : "");
            btn.innerHTML = "<span></span>" + ICONS.chevron;
            btn.querySelector("span").textContent = t(isOpen ? "events.less" : "events.more");
            btn.addEventListener("click", (e) => {
              e.stopPropagation();
              toggleMsg(task, btn, tkey, 1);
            });
            li.querySelector(".body").appendChild(btn);
          } else if (!overflows && btn && !isOpen) {
            btn.remove();
          }
        }
      } else {
        const btn = li.querySelector(".more-btn");
        if (btn) btn.remove();
      }
      const el = li.querySelector(".elapsed");
      const busyState = s.state === "tool" || s.state === "working";
      el.dataset.ts = busyState ? s.turnStart : s.lastSeen;
      el.dataset.mode = busyState ? "run" : "ago";
    }
  );
  tickElapsed();
}

function tickElapsed() {
  for (const el of document.querySelectorAll(".elapsed")) {
    const ts = new Date(el.dataset.ts);
    if (isNaN(ts)) continue;
    const sec = Math.max(0, Math.floor((Date.now() - ts) / 1000));
    el.textContent =
      el.dataset.mode === "run" ? fmtDur(sec) : fmtDur(sec) + " " + t("time.ago");
  }
}
setInterval(tickElapsed, 1000);
