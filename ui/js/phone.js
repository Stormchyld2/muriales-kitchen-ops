// Settings > Phone Access: the switch for the phone page on the kitchen Wi-Fi.
async function loadPhone() { try { state.phone = await api("/api/phone"); } catch (e) { state.phone = {on: false, urls: []}; } if (state.view === "settings" && state.tab === "phone") render(); }

function tPhone() {
  const p = state.phone; if (!p) return `<p class="hint">Loading...</p>`;
  return `<div class="panel" style="max-width:860px"><h3 style="margin-top:0">Phone page</h3>
    <p class="hint">Staff can open a simple page on a phone to log temperatures, do the par check, tick the daily workflow, log breakage and leave a handoff note. It only works on the kitchen Wi-Fi, only while this program is open on this computer, and it can reach nothing else in the program. There are no logins, so keep it off the guest Wi-Fi.</p>
    <label style="font-weight:700"><input type="checkbox" data-act="phone-toggle" ${p.on ? "checked" : ""}> Allow phones on the kitchen Wi-Fi to open the phone page</label>
    ${p.error ? `<div class="note warn">${esc(p.error)}</div>` : ""}
    ${p.on && p.running ? `<h3>Open this on the phone</h3><p>In the phone's browser (Chrome), type this address:</p>${p.urls.length ? p.urls.map(u => `<p style="font-size:22px;font-weight:700;margin:6px 0">${esc(u)}</p>`).join("") : `<p class="hint">No kitchen network address was found. Connect this computer to the Wi-Fi or network.</p>`}
      <ol class="hint"><li>Open the address in Chrome on the phone (the phone must be on the kitchen Wi-Fi).</li><li>Tap the three dots at the top right, then <b>Add to Home screen</b>.</li><li>The first time, Windows asks whether to allow this program on the private network. Choose Allow.</li><li>If the address stops working after the router restarts, the computer was given a new address. Ask whoever manages the router to reserve this computer's address.</li></ol>` : ""}
    ${p.on && !p.running && !p.error ? `<p class="hint">Starting...</p>` : ""}
    ${p.on ? `<h3>What the phones sent</h3><p class="hint">The last things phones saved or tried to save, newest first. If a phone says it saved but nothing is here, the phone is not reaching this computer. A line in red shows why a save was refused.</p>
      ${(p.activity || []).length ? `<table class="plain"><thead><tr>${["When", "Phone", "What", "Result"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}</tr></thead><tbody>${p.activity.map(a => `<tr><td>${esc(a.at)}</td><td>${esc(a.from)}</td><td>${esc(a.method + " " + a.path.replace("/api/", ""))}</td><td>${a.code < 400 ? "Saved" : `<b style="color:#a3243b">${a.code}: ${esc(a.note || "refused")}</b>`}</td></tr>`).join("")}</tbody></table>` : `<p class="hint">Nothing yet.</p>`}
      <div class="row"><button class="btn small" data-act="phone-refresh">${icon("down")}Refresh</button></div>` : ""}</div>`;
}

document.addEventListener("change", async e => {
  const el = e.target.closest && e.target.closest("[data-act=phone-toggle]"); if (!el) return;
  try { state.phone = await api("/api/phone", {method: "POST", body: JSON.stringify({on: el.checked})}); } catch (err) { toast(err.message || "Could not change that.", true); await loadPhone(); }
  render();
});

Object.assign(ACT, { async "phone-refresh"() { await loadPhone(); } });

// The computer screen refreshes itself when a phone saves something, so a
// temperature or note entered on a phone shows up without reopening the program.
let phoneStamp = null, phonePending = false;
async function phoneRefresh() {
  const v = state.view;
  if (v === "temps" || v === "tpoints") await loadTemps();
  else if (v === "dash") await loadDash();
  else if (v === "cltoday") { state.today = null; await loadToday(); }
  else if (v === "parcheck" || v === "brk" || v === "dwtoday") ACT.nav({dataset: {v}});
  else if (v === "notes" || v === "todo") { await reload(); if (v === "todo") await refreshTodo(); else render(); }
  else if (v === "settings" && state.tab === "phone") await loadPhone();
  else return;
  toast("Updated with a new entry from a phone.");
}
async function phonePoll() {
  let n; try { n = (await api("/api/phone/stamp")).n; } catch (e) { return; }
  if (phoneStamp === null) { phoneStamp = n; return; }
  if (n !== phoneStamp) phonePending = true;
  if (!phonePending) return;
  const a = document.activeElement;
  if ($("#modal").className || $("#preview").className || (a && /^(INPUT|TEXTAREA|SELECT)$/.test(a.tagName) && $("#view").contains(a))) return;
  phonePending = false; phoneStamp = n;
  try { await phoneRefresh(); } catch (e) {}
}
setInterval(phonePoll, 5000); phonePoll();
