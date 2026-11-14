"use strict";

/* Daily Readiness Walk, New Hire checklist, Training Materials, LOTO procedure page */

const WK = {checks: null, items: {}, f: {}, svc: ""};
const WK_STATUS = ["Ready", "Needs Attention", "Corrected", "N/A"];

async function loadWalk() {
  try { WK.checks = (await api("/api/reg/walkchecks")).rows.filter(r => r.active !== "No"); } catch (e) { WK.checks = []; }
  WK.checks.forEach(c => { const n = +c.id.replace(/\D/g, ""); if (!WK.items[n]) WK.items[n] = {status: "Ready", finding: "", priority: "P3 - Routine", owner: "", corrected: "No"}; });
  if (!WK.svc) WK.svc = new Date().getHours() < 11 ? "Breakfast/Lunch" : "Dinner";
  render();
}

function vWalk() {
  if (!WK.checks) { loadWalk(); return `<p class="hint">Loading...</p>`; }
  const f = WK.f, bad = WK.checks.filter(c => (WK.items[+c.id.replace(/\D/g, "")] || {}).status === "Needs Attention").length;
  const yn2 = (k, d) => `<select class="field" data-wf="${k}" style="width:100px">${["No", "Yes"].map(o => `<option ${(f[k] || d || "No") === o ? "selected" : ""}>${o}</option>`).join("")}</select>`;
  return `<p class="hint" style="margin-top:0">Walk the kitchen before service. Everything starts as Ready, so only change the checks that are not. Fix what you can on the spot and mark it Corrected.</p>
  <div class="row"><label>Service:</label><select class="field" data-wf="service" style="width:170px">${["Breakfast/Lunch", "Dinner", "Both", "Prep"].map(o => `<option ${WK.svc === o ? "selected" : ""}>${o}</option>`).join("")}</select><label>Your first name:</label><input class="field" data-wf="by" value="${esc(f.by !== undefined ? f.by : who())}" style="width:180px"><span class="hint">${bad} need attention</span><button class="btn" data-act="wk-reset">Start Over</button></div>
  <div class="tablewrap"><table class="plain"><thead><tr><th class="nosort" style="width:36px">#</th><th class="nosort" style="width:150px">Area</th><th class="nosort" style="width:auto">Check</th><th class="nosort" style="width:170px">Status</th></tr></thead><tbody>
  ${WK.checks.map(c => { const n = +c.id.replace(/\D/g, ""), it = WK.items[n]; return `<tr><td>${n}</td><td><b>${esc(c.area)}</b></td><td>${esc(c.check)}</td><td><select class="field" data-wk="${n}" data-k="status">${WK_STATUS.map(o => `<option ${it.status === o ? "selected" : ""}>${o}</option>`).join("")}</select></td></tr>
    ${it.status === "Needs Attention" || it.status === "Corrected" ? `<tr><td></td><td colspan="3"><div class="row" style="flex-wrap:wrap"><input class="field" placeholder="What is wrong" data-wk="${n}" data-k="finding" value="${esc(it.finding)}" style="width:320px"><select class="field" data-wk="${n}" data-k="priority" style="width:140px">${["P1 - Critical", "P2 - Urgent", "P3 - Routine"].map(o => `<option ${it.priority === o ? "selected" : ""}>${o}</option>`).join("")}</select><input class="field" placeholder="Who will fix it" data-wk="${n}" data-k="owner" value="${esc(it.owner)}" style="width:170px"></div></td></tr>` : ""}`; }).join("")}</tbody></table></div>
  <div class="panel"><h3 style="margin:0 0 8px">Release for service</h3><div class="fields">
    <label>Overall ready for service?</label>${yn2("overall", bad ? "No" : "Yes")}<label>P1 (critical) issue found?</label>${yn2("p1")}<label>Equipment taken out of service?</label>${yn2("oos")}<label>Equipment / work order:</label><input class="field" data-wf="oosAsset" value="${esc(f.oosAsset || "")}" placeholder="Equipment and MaintainX number">
    <label>Sanitation follow-up needed?</label>${yn2("sanit")}<label>Supply shortage?</label>${yn2("supply")}<label>Released by:</label><input class="field" data-wf="release" value="${esc(f.release || "")}"><label>Notes:</label><textarea class="field" rows="2" data-wf="notes">${esc(f.notes || "")}</textarea></div>
  <div class="row" style="margin-top:10px"><button class="btn green big" data-act="wk-save">${icon("save")}Finish Walk</button><button class="btn" data-act="rep-view" data-id="reg:walkchecks">${icon("print")}Print Blank Walk</button></div></div>`;
}

document.addEventListener("input", e => { const t = e.target; if (t.dataset && t.dataset.wf) WK.f[t.dataset.wf] = t.value; if (t.dataset && t.dataset.wk && t.tagName === "INPUT") WK.items[+t.dataset.wk][t.dataset.k] = t.value; });
document.addEventListener("change", e => { const t = e.target; if (!t.dataset) return; if (t.dataset.wf) { WK.f[t.dataset.wf] = t.value; if (t.dataset.wf === "service") WK.svc = t.value; }
  if (t.dataset.wk && t.tagName === "SELECT") { WK.items[+t.dataset.wk][t.dataset.k] = t.value; if (t.dataset.k === "status") { delete WK.f.overall; render(); } } });

/* New hire checklist */
function nhProgress(r) { const m = {}; (r.progress || "").split("\n").forEach(l => { const [id, d] = l.split("|"); if (id) m[id] = d || ""; }); return m; }

async function nhOpen(id) {
  if (!state.reg.newhireitems) state.reg.newhireitems = await api("/api/reg/newhireitems");
  const r = state.reg.newhire.rows.find(x => x.id === id), items = state.reg.newhireitems.rows.filter(i => i.active !== "No"), done = nhProgress(r);
  let area = "";
  showModal(`<h2>${icon("list")}${esc(r.employee)} - ${esc(r.role)}</h2><div class="body" id="nhbody" data-id="${esc(id)}"><div class="hint">Tick each item when it has been taught. Today's date is saved with it.${r.status === "Complete" ? " <b>This checklist is complete.</b>" : ""}</div>
    ${items.map(i => { const h = i.area !== area ? `<h3 style="margin:12px 0 2px;color:var(--gold)">${esc(area = i.area)}</h3>` : ""; return h + `<label style="display:flex;gap:10px;align-items:center;padding:5px 0;cursor:pointer"><input type="checkbox" data-nhi="${esc(i.id)}" ${i.id in done ? "checked" : ""} style="width:20px;height:20px"><span>${esc(i.task)}${i.required === "No" ? ' <span class="hint">(if it applies)</span>' : ""}${i.id in done && done[i.id] ? ` <span class="hint">${esc(shortDate(done[i.id]))}</span>` : ""}</span></label>`; }).join("")}</div>
    <div class="foot"><button class="btn green big" data-act="nh-save">${icon("save")}Save Progress</button><button class="btn big" data-act="nh-done">Mark Complete</button><button class="btn" data-act="rep-view" data-id="rec:newhire:${esc(id)}">${icon("print")}Print</button><button class="btn big" data-act="close">Close</button></div>`);
}

async function nhSave(complete) {
  const id = $("#nhbody").dataset.id, r = state.reg.newhire.rows.find(x => x.id === id), old = nhProgress(r), lines = [];
  document.querySelectorAll("[data-nhi]").forEach(c => { if (c.checked) lines.push(c.dataset.nhi + "|" + (old[c.dataset.nhi] || ymd(new Date()))); });
  const body = Object.assign({}, r, {progress: lines.join("\n")});
  if (complete) { body.status = "Complete"; body.signDate = ymd(new Date()); body.ready = confirm("Is " + r.employee + " ready to work independently?\n\nOK = Yes, Cancel = No") ? "Yes" : "No"; }
  try { state.reg.newhire = await api("/api/reg/newhire", {method: "POST", body: JSON.stringify(body)}); closeModal(); render(); toast("Saved."); } catch (e) { toast(e.message, true); }
}

/* Training materials */
async function loadMats() { try { state.mats = await api("/api/materials"); } catch (e) { state.mats = {files: [], folder: ""}; } render(); }
function vMaterials() {
  if (!state.mats) { loadMats(); return `<p class="hint">Loading...</p>`; }
  const m = state.mats, sz = n => n > 1048576 ? (n / 1048576).toFixed(1) + " MB" : Math.max(1, Math.round(n / 1024)) + " KB";
  return `<div class="row"><button class="btn green big" data-act="mat-folder">${icon("list")}Open Training Materials Folder</button><button class="btn" data-act="mat-refresh">Refresh</button><span class="hint">${m.files.length} files</span></div>
  <p class="hint" style="margin-top:0">Copy the training slide decks, videos and handouts into the Training Materials folder, which sits next to this program. They show up here and open in whatever program normally opens them.</p>
  ${!m.files.length ? `<div class="empty"><b>The folder is empty</b>Open the folder and copy your files into it.</div>` : `<div class="tablewrap"><table class="plain"><thead><tr><th class="nosort" style="width:auto">File</th><th class="nosort" style="width:auto">Size</th><th class="nosort" style="width:auto">Changed</th><th class="nosort"></th></tr></thead><tbody>${m.files.map(f => `<tr><td><b>${esc(f.name)}</b></td><td>${sz(f.size)}</td><td>${esc(shortDate(f.modified))}</td><td style="text-align:right"><button class="btn small green" data-act="mat-open" data-n="${esc(f.name)}">Open</button></td></tr>`).join("")}</tbody></table></div>`}`;
}

/* General LOTO procedure page */
async function loadLotoGen() { try { state.lotogen = await api("/api/guides/GDE-102"); } catch (e) { state.lotogen = {html: "<p>The general procedure was not found. It is in Station Guides under Kitchen-Wide.</p>"}; } render(); }
function vLotoGen() {
  if (!state.lotogen) { loadLotoGen(); return `<p class="hint">Loading...</p>`; }
  return `<div class="row"><button class="btn" data-act="gd-print" data-id="GDE-102">${icon("print")}Print</button><button class="btn" data-act="reg-open" data-c="guides" data-id="GDE-102">Edit</button></div><div class="panel gmd">${state.lotogen.html}</div>`;
}

Object.assign(ACT, {
  "wk-reset"() { WK.items = {}; WK.f = {}; WK.checks = null; render(); },
  async "wk-save"() {
    const f = WK.f, by = (f.by !== undefined ? f.by : who()).trim(); if (!by) { toast("Type your first name.", true); return; }
    const bad = WK.checks.filter(c => WK.items[+c.id.replace(/\D/g, "")].status === "Needs Attention");
    const items = WK.checks.map(c => { const n = +c.id.replace(/\D/g, ""); return Object.assign({n, area: c.area, check: c.check}, WK.items[n]); });
    try {
      const r = await api("/api/walk", {method: "POST", body: JSON.stringify({service: WK.svc, by, notes: f.notes || "", overall: f.overall || (bad.length ? "No" : "Yes"), p1: f.p1 || "No", oos: f.oos || "No", oosAsset: f.oosAsset || "", sanit: f.sanit || "No", supply: f.supply || "No", release: f.release || "", items})});
      WK.items = {}; WK.f = {}; WK.checks = null; delete state.reg.walks; state.today = null; toast("Walk saved.");
      if (r.need.length && confirm(r.need.length + " item(s) still need attention.\n\nCreate a corrective action for them now?")) caForm("", {area: "Maintenance", priority: "P2 - Urgent", issue: "Readiness walk " + r.id + ":\n" + r.need.join("\n")}); else render();
    } catch (e) { toast(e.message, true); }
  },
  "nh-open"(el) { nhOpen(el.dataset.id); },
  "nh-save"() { nhSave(false); },
  "nh-done"() { nhSave(true); },
  async "inc-ca"(el) { const r = state.reg.incidents.rows.find(x => x.id === el.dataset.id); if (r) caForm("", {area: "Safety", priority: r.severity === "Minor" ? "P3 - Routine" : "P2 - Urgent", issue: `${r.type} (${r.id}) on ${r.date}${r.area ? " in " + r.area : ""}: ${r.what}`, immediate: r.immediate || ""}); },
  async "loto-off"(el) { const r = state.reg.lotolog.rows.find(x => x.id === el.dataset.id); if (!r) return; const d = new Date(); try { state.reg.lotolog = await api("/api/reg/lotolog", {method: "POST", body: JSON.stringify(Object.assign({}, r, {removed: ymd(d) + " " + d.toLocaleTimeString([], {hour: "numeric", minute: "2-digit"})}))}); render(); toast("Marked removed."); } catch (e) { toast(e.message, true); } },
  "mtg-ca"(el) { const r = state.reg.meetings.rows.find(x => x.id === el.dataset.id); if (r) caForm("", {area: "Training", priority: "P3 - Routine", issue: `${r.type} ${r.id} on ${r.date}: ${r.topic}${r.notes ? ". " + r.notes : ""}`}); },
  "mat-refresh"() { state.mats = null; render(); },
  async "mat-folder"() { await api("/api/materials/folder", {method: "POST", body: "{}"}); },
  async "mat-open"(el) { try { await api("/api/materials/open", {method: "POST", body: JSON.stringify({name: el.dataset.n})}); } catch (e) { toast(e.message, true); } },
});
document.addEventListener("change", e => { const c = e.target.closest && e.target.closest("[data-nhi]"); if (c) { /* progress is read when saved */ } });

/* Policies & Reference library */
async function loadRefs() { try { state.refs = await api("/api/reference"); } catch (e) { state.refs = {files: [], folder: ""}; } render(); }
function vReference() {
  if (!state.refs) { loadRefs(); return `<p class="hint">Loading...</p>`; }
  const m = state.refs, sz = n => n > 1048576 ? (n / 1048576).toFixed(1) + " MB" : Math.max(1, Math.round(n / 1024)) + " KB";
  return `<div class="row"><button class="btn green big" data-act="ref-folder">${icon("list")}Open Policies &amp; Reference Folder</button><button class="btn" data-act="ref-refresh">Refresh</button><span class="hint">${m.files.length} files</span></div>
  <p class="hint" style="margin-top:0">The FDA Food Code and the West Virginia food establishment rule are kept here. Copy your own policies, handbooks and permits into the folder, which sits next to this program, and they show up here and open in whatever program normally opens them. A plain-language summary of the 2026 rule change is in Station Guides under Kitchen-Wide.</p>
  ${!m.files.length ? `<div class="empty"><b>The folder is empty</b>Open the folder and copy your files into it.</div>` : `<div class="tablewrap"><table class="plain"><thead><tr><th class="nosort" style="width:auto">File</th><th class="nosort" style="width:auto">Size</th><th class="nosort" style="width:auto">Changed</th><th class="nosort"></th></tr></thead><tbody>${m.files.map(f => `<tr><td><b>${esc(f.name)}</b></td><td>${sz(f.size)}</td><td>${esc(shortDate(f.modified))}</td><td style="text-align:right"><button class="btn small green" data-act="ref-open" data-n="${esc(f.name)}">Open</button></td></tr>`).join("")}</tbody></table></div>`}`;
}
Object.assign(ACT, {
  "ref-refresh"() { state.refs = null; render(); },
  async "ref-folder"() { await api("/api/reference/folder", {method: "POST", body: "{}"}); },
  async "ref-open"(el) { try { await api("/api/reference/open", {method: "POST", body: JSON.stringify({name: el.dataset.n})}); } catch (e) { toast(e.message, true); } },
});
