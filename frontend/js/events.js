"use strict";

function renderEvents(events, unread) {
  const tabUnread = (unread || {})[currentTab || "claude"] || 0;
  $("done-badge").textContent = badgesOn() && tabUnread > 0 ? String(tabUnread) : "";
  if (subTab !== "events") return;
  const list = $("events");
  const empty = $("empty");

  const tab = currentTab || "claude";
  const shown = [];
  let total = 0;
  events.forEach((ev, i) => {
    if ((ev.source || "claude") !== tab) return;
    total++;
    if (!searchOpen || evSearchPass(ev)) shown.push([ev, i]);
  });
  setSearchCount(shown.length, total);
  const filtered = searchOpen && searchActive() && total > 0;
  empty.querySelector("[data-i18n]").textContent =
    t(filtered ? "search.none" : "events.empty");
  empty.querySelectorAll("span")[1].textContent =
    t(filtered ? "search.none.hint" : "events.hint");
  empty.classList.toggle("hidden", shown.length > 0);
  list.classList.toggle("hidden", shown.length === 0);

  reconcile(
    list,
    shown,
    ([ev]) => evKey(ev),
    ([ev]) => {
      const li = document.createElement("li");
      if (knownNewest && ev.time > knownNewest) li.classList.add("new");
      li.innerHTML =
        '<span class="ic"></span>' +
        '<div class="body">' +
        '<div class="head"><span class="name"></span><span class="branch"></span><span class="proj"></span><span class="time"></span></div>' +
        '<div class="meta"></div>' +
        '<div class="msg clamp"></div>' +
        "</div>";
      li.querySelector(".proj").addEventListener("click", (e) => {
        e.stopPropagation(); // chip opens the folder; the row focuses the window
        post("/api/folder", { index: +li.dataset.index });
      });
      li.addEventListener("click", () => {
        li.classList.remove("unread"); // optimistic; server marks it too
        post("/api/open", { index: +li.dataset.index });
      });
      return li;
    },
    (li, [ev, idx]) => {
      li.dataset.index = idx;
      li.classList.toggle("unread", !ev.read);
      li.classList.add("ev");
      const key = evKey(ev);
      const ic = li.querySelector(".ic");
      const kindCls = "ic " + (ev.kind === "attention" ? "attn" : "done");
      if (ic.dataset.kind !== ev.kind) {
        ic.dataset.kind = ev.kind;
        ic.className = kindCls;
        ic.innerHTML = ev.kind === "attention" ? ICONS.bellRing : ICONS.check;
      }
      setText(li.querySelector(".name"),
        ev.title || (ev.cwd || "").split(/[\\/]/).pop() || "claude");
      const when = new Date(ev.time);
      setText(li.querySelector(".time"),
        when.getHours().toString().padStart(2, "0") + ":" +
        when.getMinutes().toString().padStart(2, "0"));
      const br = li.querySelector(".branch");
      if (ev.branch) {
        if (!br.firstChild) br.innerHTML = ICONS.branch + "<span></span>";
        setText(br.querySelector("span"), ev.branch);
        br.style.display = "";
      } else br.style.display = "none";
      const proj = li.querySelector(".proj");
      if (!proj.firstChild) proj.innerHTML = ICONS.focus + "<span></span>";
      setText(proj.querySelector("span"), (ev.cwd || "").split(/[\\/]/).pop() || "");
      proj.dataset.tip = ev.cwd || "";
      const meta = li.querySelector(".meta");
      const bits = [];
      if (ev.durSec > 0) bits.push(ICONS.clock + "<span>" + fmtDur(ev.durSec) + "</span>");
      if (ev.model) bits.push("<span>" + shortModel(ev.model) + "</span>");
      const metaHTML = bits.join('<i class="sep">·</i>');
      if (meta.dataset.html !== metaHTML) {
        meta.dataset.html = metaHTML;
        meta.innerHTML = metaHTML;
      }
      meta.style.display = metaHTML ? "" : "none";
      const msg = li.querySelector(".msg");
      if (ev.message) {
        msg.style.display = "";
        const changed = msg.dataset.txt !== ev.message;
        if (changed) {
          msg.dataset.txt = ev.message;
          msg.textContent = ev.message;
        }
        const isOpen = expanded.has(key);
        msg.classList.toggle("clamp", !isOpen);
        // (re)evaluate the show-more toggle when content changed
        let btn = li.querySelector(".more-btn");
        if (changed || !btn) {
          const overflows = isOpen || msg.scrollHeight > msg.clientHeight + 1;
          if (overflows && !btn) {
            btn = document.createElement("button");
            btn.className = "more-btn" + (isOpen ? " open" : "");
            btn.innerHTML = "<span></span>" + ICONS.chevron;
            btn.querySelector("span").textContent = t(isOpen ? "events.less" : "events.more");
            btn.addEventListener("click", (e) => {
              e.stopPropagation();
              toggleMsg(msg, btn, key);
            });
            li.querySelector(".body").appendChild(btn);
          } else if (!overflows && btn && !isOpen) {
            btn.remove();
          }
        }
      } else {
        msg.style.display = "none";
        const btn = li.querySelector(".more-btn");
        if (btn) btn.remove();
      }
    }
  );
  if (events.length > 0) knownNewest = events[0].time;
}

async function post(url, body) {
  try {
    await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : null,
    });
  } catch (_) {}
  refresh();
}
