"use strict";

/* Cleaning and sanitation */

async function loadToday() { state.today = await api("/api/clean/today"); if (state.view === "cltoday") render(); }

function vClToday() {
  const d = state.today; if (!d) { loadToday(); return `<p class="hint">Loading...</p>`; }
  const done = d.tasks.filter(t => t.state === "Done Today").length, open = d.tasks.filter(t => t.state === "Overdue" || t.state === "Due Today"), table = (rows, kind) => `<div class="tablewrap"><table class="plain"><thead><tr>${["Area", kind === "task" ? "Task" : "Check", "When", "Who", "Status"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}<th class="nosort"></th></tr></thead><tbody>
    ${rows.map(t => `<tr><td><b>${esc(t.area)}</b></td><td>${esc(t.task || t.check)}</td><td>${esc(t.timing || t.frequency)}</td><td>${esc(t.role || t.owner)}</td><td>${tag(t.state)}</td><td style="text-align:right"><div class="acts">${t.state === "Done Today" ? "" : `<button class="btn small green" data-act="${kind === "task" ? "cl-done" : "mgr-done"}" data-id="${esc(t.id)}">Done</button>`}</div></td></tr>`).join("")}</tbody></table></div>`;
  const chk = d.checks.filter(t => t.state !== "Upcoming");
  return `<div class="row"><span class="hint"><b>${open.length}</b> to do, <b>${done}</b> done today</span><button class="btn" data-act="reg-print" data-id="linecook-weekly">${icon("print")}Print Line Cook Schedule</button></div>
  <h3>Cleaning</h3>${open.length || done ? table(d.tasks.filter(t => t.state !== "Upcoming"), "task") : `<div class="empty"><b>Nothing due</b></div>`}
  ${d.pending.length ? `<h3>Waiting to be verified</h3><div class="tablewrap"><table class="plain"><thead><tr>${["ID", "Date", "Area", "Task", "Done by"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}<th class="nosort"></th></tr></thead><tbody>${d.pending.map(p => `<tr><td><b>${esc(p.id)}</b></td><td>${esc(p.date)}</td><td>${esc(p.area)}</td><td>${esc(p.task)}</td><td>${esc(p.completedBy)}</td><td style="text-align:right"><div class="acts"><button class="btn small" data-act="cl-verify" data-id="${esc(p.id)}">Verify</button></div></td></tr>`).join("")}</tbody></table></div>` : ""}
  ${(d.deep || []).length ? `<h3>Weekly deep clean</h3><div class="tablewrap"><table class="plain"><thead><tr>${["Day", "Area", "Task", "Who", "Status"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}<th class="nosort"></th></tr></thead><tbody>${d.deep.map(t => `<tr><td>${esc(t.day)}</td><td><b>${esc(t.area)}</b></td><td>${esc(t.task)}</td><td>${esc(t.role)}</td><td>${tag(t.state)}</td><td style="text-align:right"><div class="acts">${t.state === "Done Today" ? "" : `<button class="btn small green" data-act="deep-done" data-id="${esc(t.id)}">Done</button>`}</div></td></tr>`).join("")}</tbody></table></div>` : ""}
  <h3>Manager inspections</h3>${chk.length ? table(chk, "check") : `<div class="empty"><b>No manager checks due</b></div>`}`;
}

function cleanDone(id) {
  const go = async () => { if (!state.today) await loadToday(); cleanDone(id); }; if (!state.today) { go(); return; }
  const t = state.today.tasks.find(x => x.id === id); if (!t) return toast("That task is not on the schedule.", true);
  showModal(`<h2>${icon("save")}Mark Done</h2><div class="body" id="cdbody" data-id="${esc(id)}"><p><b>${esc(t.area)}</b>: ${esc(t.task)}</p>${t.verify === "Yes" ? `<p class="hint">This task needs a manager to verify it afterwards.</p>` : ""}<div class="fields">
    <label>Your name or initials: <span class="req">*</span></label><input class="field" id="cdby" value="${esc(who())}"><label>Shift:</label><input class="field" id="cdshift" value="${esc(t.timing || "")}" style="width:220px">
    <label></label><label style="font-weight:400"><input type="checkbox" id="cdissue"> I found a problem (adds it to Missed &amp; Overdue)</label><label>Notes:</label><textarea class="field" rows="2" id="cdnotes"></textarea></div></div>
    <div class="foot"><button class="btn green big" data-act="cl-done-go">${icon("save")}Save</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

function deepDone(id) {
  const t = ((state.reg.deepclean || {}).rows || []).find(x => x.id === id) || (state.today.deep || []).find(x => x.id === id); if (!t) return toast("That job was not found.", true);
  showModal(`<h2>${icon("save")}Mark Deep Clean Done</h2><div class="body" id="ddbody" data-id="${esc(id)}"><p><b>${esc(t.area)}</b>: ${esc(t.task)}</p><div class="fields"><label>Your name or initials: <span class="req">*</span></label><input class="field" id="ddby" value="${esc(who())}"><label>Notes:</label><textarea class="field" rows="2" id="ddnotes"></textarea></div></div>
    <div class="foot"><button class="btn green big" data-act="deep-done-go">${icon("save")}Save</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

function verifyAsk(id) {
  showModal(`<h2>${icon("save")}Verify Cleaning</h2><div class="body" id="vfbody" data-id="${esc(id)}"><div class="fields"><label>Result:</label><div><label style="font-weight:400"><input type="radio" name="vfok" value="ok" checked> Done to standard</label> &nbsp; <label style="font-weight:400"><input type="radio" name="vfok" value="fail"> Not to standard</label></div>
    <label>Verified by: <span class="req">*</span></label><input class="field" id="vfby" value="${esc(who())}"><label>Notes:</label><textarea class="field" rows="2" id="vfnotes" placeholder="If not to standard, say what is wrong"></textarea></div></div>
    <div class="foot"><button class="btn green big" data-act="cl-verify-go">${icon("save")}Save</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

function mgrDone(id) {
  if (!state.today) { loadToday().then(() => mgrDone(id)); return; }
  const c = state.today.checks.find(x => x.id === id); if (!c) return toast("That inspection is not on the schedule.", true);
  showModal(`<h2>${icon("save")}Record Inspection</h2><div class="body" id="mdbody" data-id="${esc(id)}"><p><b>${esc(c.check)}</b>${c.area ? " · " + esc(c.area) : ""}</p>${c.whatCheck ? `<p class="hint">${esc(c.whatCheck)}</p>` : ""}<div class="fields">
    <label>Result:</label><select class="field" id="mdres" style="width:200px"><option>Pass</option><option>Needs Attention</option><option>N/A</option></select><label>Checked by: <span class="req">*</span></label><input class="field" id="mdby" value="${esc(who())}"><label>Notes:</label><textarea class="field" rows="2" id="mdnotes"></textarea></div>
    <p class="hint">If it needs attention, open the Manager Inspection Log afterwards and use Corrective Action to start a form.</p></div>
    <div class="foot"><button class="btn green big" data-act="mgr-done-go">${icon("save")}Save</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

function vClWeek() {
  return `<div class="row"><button class="btn big" data-act="reg-print" data-id="linecook-weekly">${icon("print")}Line Cook Weekly Cleaning Schedule</button><button class="btn big" data-act="reg-print" data-id="clean-weekly">${icon("print")}Weekly Schedule to Post</button></div>
  <div class="panel" style="margin:8px 0"><b>Line Cook Weekly Cleaning Schedule</b><div>The nightly close for every station plus a different detail job each day, Monday to Sunday, with the cleaning expectations. Print it and post it on the line.</div></div>
  <div class="panel" style="margin:8px 0"><b>Weekly Schedule to Post</b><div>The week's deep-clean jobs with spaces for who it is assigned to, completed, initials and manager verification, and a spare line each day for an extra job.</div></div>
  <p class="hint">Both open ready to print. The wording of the daily jobs comes from your Cleaning Schedule and the line cook document.</p>`;
}

Object.assign(ACT, {
  "cl-done"(el) { cleanDone(el.dataset.id); },
  async "cl-done-go"() { const by = $("#cdby").value.trim(); if (!by) return toast("Enter your name or initials.", true); rememberWho(by);
    try { state.today = await api("/api/clean/done", {method: "POST", body: JSON.stringify({id: $("#cdbody").dataset.id, by, shift: $("#cdshift").value, issue: $("#cdissue").checked, notes: $("#cdnotes").value})}); state.reg = {}; closeModal(); if (state.view === "todo") await refreshTodo(); render(); toast("Recorded."); } catch (e) { toast(e.message, true); } },
  "deep-done"(el) { if (!state.today) { loadToday().then(() => deepDone(el.dataset.id)); return; } deepDone(el.dataset.id); },
  async "deep-done-go"() { const by = $("#ddby").value.trim(); if (!by) return toast("Enter your name or initials.", true); rememberWho(by);
    try { await api("/api/clean/deepdone", {method: "POST", body: JSON.stringify({id: $("#ddbody").dataset.id, by, notes: $("#ddnotes").value})}); state.reg = {}; state.today = null; closeModal(); render(); toast("Recorded."); } catch (e) { toast(e.message, true); } },
  "cl-verify"(el) { verifyAsk(el.dataset.id); },
  async "cl-verify-go"() { const by = $("#vfby").value.trim(); if (!by) return toast("Enter who is verifying.", true); rememberWho(by);
    try { state.today = await api("/api/clean/verify", {method: "POST", body: JSON.stringify({id: $("#vfbody").dataset.id, by, ok: el_val("vfok") === "ok", notes: $("#vfnotes").value})}); state.reg = {}; closeModal(); render(); toast("Saved."); } catch (e) { toast(e.message, true); } },
  "mgr-done"(el) { mgrDone(el.dataset.id); },
  async "mgr-done-go"() { const by = $("#mdby").value.trim(); if (!by) return toast("Enter your name.", true); rememberWho(by); const result = $("#mdres").value;
    try { state.today = await api("/api/mgr/done", {method: "POST", body: JSON.stringify({id: $("#mdbody").dataset.id, by, result, notes: $("#mdnotes").value})}); state.reg = {}; closeModal(); if (state.view === "todo") await refreshTodo(); render(); toast("Recorded.");
      if (result === "Needs Attention") { const c = state.today.checks.find(x => x.id === $("#mdbody")?.dataset.id) || {}; } } catch (e) { toast(e.message, true); } },
  "mgr-ca"(el) { const r = (state.reg.mgrlog || {rows: []}).rows.find(x => x.id === el.dataset.id); if (!r) return; caForm("", {area: "Food Safety", priority: "P2 - Urgent", issue: `${r.check}${r.area ? " (" + r.area + ")" : ""}: ${r.notes || "needs attention"}`, related: "Manager Inspection Log " + r.id}); },
});
