"use strict";

// ---- onboarding tour, session-window leg ----
// The dashboard tour's Done opens this window with ?tour=1. Like the
// dashboard leg it runs on MOCK sessions (load() swaps them in before the
// first render, so real ones never flash); Skip/Done closes the window and
// brings the dashboard back (/api/cleaner/tour-end). The ? button replays
// it over an open window and puts the real list back afterwards. Body
// left → right, then the toolbar left → right.
const CL_TOUR_STEPS = [
  { sel: "#side", key: "c1" },
  { sel: "#main", key: "c2", at: "bottom" }, // rows sit at the top of the list
  { sel: "#insp", key: "c3" },
  { sel: "#bar .search", key: "c4" },
  { sel: "#bulk", key: "c5" },
  { sel: "#del", key: "c6" },
  { sel: "#gear", key: "c7" },
];

// one of each kind the filters know: large, running, protected, old,
// empty, orphan — titles reuse the dashboard mock's tasks
function clTourSessions() {
  const now = Date.now();
  const ago = (min) => new Date(now - min * 60e3).toISOString();
  const home = st.home || "~";
  const root = home + "/.claude";
  const mk = (sid, { dir, min, ...o }) => {
    const proj = o.project === undefined ? home + "/Projects/" + dir : o.project;
    const tr = root + "/projects/" + proj.replace(/[\\/.]/g, "-") + "/" + sid + ".jsonl";
    const hist = root + "/file-history/" + sid;
    const files = o.orphan
      ? [{ path: hist, size: o.size, dir: true }]
      : [{ path: tr, size: Math.round(o.size * 0.8) }, { path: hist, size: Math.round(o.size * 0.2), dir: true }];
    return Object.assign(
      { id: o.orphan ? "orphan:" + root + ":" + sid : tr, agent: "claude", sid, root, title: "",
        prompt: "", cwd: proj, project: proj, source: "cli", started: ago(min + 20), mtime: ago(min),
        files, running: false, empty: false, orphan: false, pinned: false,
        resume: o.orphan ? null : ["claude", "--resume", sid] },
      o
    );
  };
  const old = (st.prefs.oldDays + 15) * 1440;
  return [
    mk("7c1e2f4a-1b3d-4e5f-8a9b-0c1d2e3f4a5b", { dir: "claude-notify", min: 3, title: t("tut.mock.task1"),
      prompt: t("tut.mock.task1"), size: (st.prefs.largeMB + 12) * 1e6 }),
    mk("2a9f7e1c-5d4b-4c3a-9e8f-7a6b5c4d3e2f", { dir: "webapp", min: 1, source: "vscode", running: true,
      title: t("tut.mock.task2"), prompt: t("tut.mock.task2"), size: 4.2e6 }),
    mk("9d8c7b6a-5f4e-4d3c-2b1a-0f9e8d7c6b5a", { dir: "api", min: 2 * 1440, source: "desktop", pinned: true,
      title: t("tut.mock.task3"), prompt: t("tut.mock.task3"), size: 1.8e6 }),
    mk("4b5c6d7e-8f9a-4b0c-1d2e-3f4a5b6c7d8e", { project: home + "/dotfiles", min: old,
      title: t("tut.mock.task4"), prompt: t("tut.mock.task4"), size: 3.1e5 }),
    mk("6e7f8a9b-0c1d-4e2f-3a4b-5c6d7e8f9a0b", { dir: "webapp", min: 12 * 1440, empty: true, size: 2100 }),
    mk("1f2e3d4c-5b6a-4978-8695-a4b3c2d1e0f9", { project: "", cwd: "", min: 20 * 1440, orphan: true,
      source: "", started: "", size: 1.8e7 }),
  ];
}

let clHandoff = touring; // opened by the dashboard tour: Done closes the window
let clSaved = null; // the real view a ? replay borrowed

function clTourStart() {
  if (!st) return clTourEnd(); // no state, nothing to point at
  // the inspector step needs the inspector out and a session in it
  $("app").classList.remove("no-insp");
  $("insp-btn").classList.add("active");
  selection.clear();
  selection.add(st.sessions[0].id);
  anchor = st.sessions[0].id;
  renderSel();
  spotStart({ steps: CL_TOUR_STEPS, end: clTourEnd });
}

// swap the view under a fade (the agent switch's), so rows never pop;
// swap may return a promise (a rescan) the fade-in waits for
function clFade(swap, then) {
  $("app").classList.add("switching");
  setTimeout(async () => {
    await swap();
    $("app").classList.remove("switching");
    if (then) setTimeout(then, 400); // inspector slide + fade-in settle first
  }, 180);
}

function clTourReplay() {
  if (touring || !st) return;
  touring = true;
  closeMenus();
  clSaved = { filter, query, anchor, sel: [...selection], insp: !$("app").classList.contains("no-insp"),
    scroll: $("rows").scrollTop };
  clFade(() => {
    filter = "all";
    query = $("q").value = "";
    $("q").parentElement.classList.remove("has");
    st.sessions = clTourSessions();
    render();
    $("rows").scrollTop = 0;
  }, clTourStart);
}

function clTourEnd() {
  spotHide();
  touring = false;
  if (clHandoff) {
    clHandoff = false;
    return api("/api/cleaner/tour-end");
  }
  // replay: back to the real list as it was
  const s = clSaved;
  clSaved = null;
  if (!s) return load();
  clFade(() => {
    filter = s.filter;
    query = $("q").value = s.query;
    $("q").parentElement.classList.toggle("has", !!s.query);
    $("app").classList.toggle("no-insp", !s.insp);
    $("insp-btn").classList.toggle("active", s.insp);
    selection.clear();
    for (const id of s.sel) selection.add(id);
    anchor = s.anchor;
    return load().then(() => ($("rows").scrollTop = s.scroll));
  });
}

$("help-btn").addEventListener("click", clTourReplay);

// let the boot cover fade out before the spotlight eases in
if (touring) load().then(() => setTimeout(clTourStart, 450));
