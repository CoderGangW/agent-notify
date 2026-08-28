"use strict";

// ---- update footer ----
let updState = "idle"; // idle | checking | latest | available | applying | done
let updLatest = "";

function setUpd(state, label) {
  updState = state;
  const btn = $("upd-btn");
  btn.dataset.state = state;
  btn.querySelector("span").textContent = label;
}

$("upd-btn").addEventListener("click", async () => {
  if (updState === "checking" || updState === "applying" || updState === "done") return;
  if (updState === "available") {
    setUpd("applying", t("update.applying"));
    try {
      const r = await (await fetch("/api/update-apply", { method: "POST" })).json();
      if (r.error) setUpd("idle", t("update.fail"));
      else setUpd("done", r.restarted ? t("update.restarting") : t("update.next"));
    } catch (_) {
      // daemon likely restarted mid-response; that's success
      setUpd("done", t("update.restarting"));
    }
    return;
  }
  setUpd("checking", t("update.checking"));
  try {
    const r = await (await fetch("/api/update-check", { method: "POST" })).json();
    if (r.error) {
      setUpd("idle", t("update.fail"));
    } else if (r.available) {
      updLatest = r.latest;
      setUpd("available", t("update.available").replace("{v}", r.latest));
    } else {
      setUpd("latest", t("update.latest"));
      setTimeout(() => {
        if (updState === "latest") setUpd("idle", t("update.check"));
      }, 3000);
    }
  } catch (_) {
    setUpd("idle", t("update.fail"));
  }
});
