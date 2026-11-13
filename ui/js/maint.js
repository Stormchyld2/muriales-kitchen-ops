"use strict";

/* Maintenance: equipment, issues, PM, assets, replacements, findings, summary */

const EQ_TYPES = ["Walk-in Cooler", "Walk-in Freezer", "Reach-in Refrigerator", "Freezer", "Fryer", "Range / Oven", "Grill / Flat Top", "Mixer", "Dish Machine", "Ice Machine", "Hood / Exhaust Fan", "Fire Suppression System", "HVAC / Rooftop Unit", "Make-up Air Unit", "Water Heater", "Grease Trap", "Water Filter", "Other"];

var PM_TEMPLATES = [
  ["HVAC: change filters", 3, "months", "Typical starting point. Check the unit's manual; dusty kitchens may need it monthly."],
  ["HVAC: service and clean coils", 6, "months", "Typically spring and fall, by a technician."],
  ["Hood / exhaust: clean", 3, "months", "How often depends on cooking volume and local code. Confirm with your inspector or hood cleaner."],
  ["Fire suppression: inspection", 6, "months", "Usually a licensed technician twice a year. Confirm with your local fire code."],
  ["Walk-in: clean condenser coil", 3, "months", ""],
  ["Walk-in / cooler: check temperature log and door seals", 1, "months", ""],
  ["Ice machine: clean and sanitize", 6, "months", "Follow the manufacturer's schedule."],
  ["Fryer: boil out", 1, "months", ""],
  ["Dish machine: delime", 1, "months", ""],
  ["Grease trap: pump out", 3, "months", "Frequency is set by your local rules."],
  ["Water filter: replace cartridge", 6, "months", ""],
  ["Thermometer: calibrate", 1, "months", ""],
];

var CLEAN_TEMPLATES = [
  ["Sanitize prep surfaces and cutting boards", 1, "days", ""],
  ["Sweep and mop kitchen floors", 1, "days", ""],
  ["Clean and sanitize restrooms", 1, "days", ""],
  ["Empty and clean floor drains", 1, "weeks", ""],
  ["Deep clean walk-in shelves and floor", 1, "weeks", ""],
  ["Clean behind and under cooking equipment", 1, "weeks", ""],
  ["Clean and sanitize reach-in refrigerators", 1, "weeks", ""],
  ["Clean walls and ceiling tiles in prep areas", 1, "months", ""],
  ["Clean light fixtures and vents", 1, "months", ""],
  ["Clean dry storage shelving", 1, "months", ""],
  ["Deep clean kitchen (hood, walls, ceilings)", 1, "years", ""],
];

var FAC_TEMPLATES = [
  ["Pest control service", 1, "months", "Keep the service report with your records."],
  ["Parking lot and grounds walk (litter, lighting)", 1, "weeks", ""],
  ["Dumpster area clean", 1, "weeks", ""],
  ["Check emergency lights and exit signs", 1, "months", ""],
  ["Fire extinguisher inspection", 1, "months", "Monthly visual check; annual professional service."],
  ["Clean gutters and downspouts", 6, "months", ""],
  ["Roof inspection", 1, "years", ""],
  ["Backflow preventer test", 1, "years", "Confirm requirements with your local authority."],
];

function vEquip() {
  const m = state.maint; if (!m) { loadMaint().then(render); return `<p class="hint">Loading...</p>`; }
  const list = m.equipment, down = list.filter(e => e.status !== "Operational").length;
  return `<div class="row"><button class="btn green big" data-act="eq-edit" data-id="">${icon("plus")}Add Equipment</button>
    <span class="hint">${list.length} piece${list.length === 1 ? "" : "s"} of equipment${down ? ` · <b>${down} need${down === 1 ? "s" : ""} attention</b>` : ""}</span></div>
  <div class="tablewrap">${!list.length ? `<div class="empty"><b>No equipment yet</b>Click “Add Equipment” to list the walk-ins, fryers, mixers and other equipment you maintain, or load the 85 pieces from your maintenance workbook.<div style="margin-top:10px"><button class="btn green" data-act="wb-equip">${icon("down")}Create Equipment from the Workbook</button></div></div>`
    : `<table class="plain"><thead><tr>${["Asset ID", "Equipment", "Area", "Type", "Make / Model", "Status", "Open Issues"].map(h => `<th class="nosort" style="width:auto">${h}</th>`).join("")}<th class="nosort" style="width:450px"></th></tr></thead><tbody>
      ${list.map(e => `<tr><td>${esc(e.assetCode || "")}</td><td><b>${esc(e.name)}</b></td><td>${esc(e.area)}</td><td>${esc(e.type)}</td><td>${esc([e.make, e.model].filter(Boolean).join(" "))}</td><td>${tag(e.status)}${(state.maint.pm || []).some(t => t.equipmentId === e.id && t.state === "Overdue") ? " " + tag("Overdue") : ""}</td><td>${openIssues(e.id) || ""}</td>
        <td style="text-align:right"><div class="acts">${e.status === "Out of Service" ? `<button class="btn small" data-act="tag-print" data-id="${e.id}">${icon("print")}Print Tag</button> <button class="btn small green" data-act="oos-clear-ask" data-id="${e.id}">Clear for Service</button>` : `<button class="btn small wine" data-act="oos-ask" data-id="${e.id}">Take Out of Service</button>`} <button class="btn small wine" data-act="issue-new" data-id="${e.id}">${icon("warn")}Report Issue</button> <button class="btn small" data-act="eq-edit" data-id="${e.id}">${icon("edit")}Edit</button></div></td></tr>`).join("")}
    </tbody></table>`}</div>`;
}

async function loadRepl() { state.repl = await api("/api/replacements"); if (state.view === "repl") render(); }

function vRepl() {
  const d = state.repl; if (!d) { loadRepl(); return `<p class="hint">Loading...</p>`; }
  return `<div class="row"><button class="btn green big" data-act="repl-new" data-eq="">${icon("plus")}New Replacement Request</button><button class="btn" data-act="reg-print" data-id="replace-log">${icon("print")}Print History</button><span class="hint">Use when repeated repairs, downtime, safety risk or running cost suggest replacing instead of repairing again.</span></div>
  <div class="tablewrap">${!d.requests.length ? `<div class="empty"><b>No requests yet</b>Start one here, or from Assets &amp; Costs by opening a piece of equipment.</div>` : `<table class="plain"><thead><tr>${["ID", "Date", "Equipment", "Reason", "Recommendation", "Priority", "Decision", "Est. cost", "Completed"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}<th class="nosort"></th></tr></thead><tbody>
    ${d.requests.map(r => `<tr><td><b>${esc(r.id)}</b></td><td>${esc(r.date)}</td><td>${esc(r.equipment)}</td><td>${esc((r.reasons || "").slice(0, 70))}</td><td>${esc(r.recommendation)}</td><td>${esc(r.priority)}</td><td>${esc(r.decision || "Pending")}</td><td>${r.replaceCost ? money(r.replaceCost) : ""}</td><td>${esc(r.completionDate)}</td>
      <td style="text-align:right"><div class="acts"><button class="btn small" data-act="repl-open" data-id="${esc(r.id)}">${icon("edit")}Edit</button> <button class="btn small" data-act="repl-print" data-id="${esc(r.id)}">${icon("print")}Print</button></div></td></tr>`).join("")}</tbody></table>`}</div>`;
}

function replRow(h) {
  h = h || {}; const tb = $("#rphist tbody"), tr = document.createElement("tr"), i = (k, w, t) => `<input class="field" data-rh="${k}" value="${esc(h[k] == null ? "" : h[k])}" ${t ? `type="${t}" step="0.01"` : ""} style="min-width:${w}px;width:100%">`;
  tr.innerHTML = `<td>${i("date", 120, "date")}</td><td>${i("wo", 70)}</td><td>${i("failure", 160)}</td><td>${i("downtime", 60, "number")}</td><td>${i("repairCost", 70, "number")}</td><td>${i("partsCost", 70, "number")}</td><td>${i("vendor", 90)}</td><td>${i("repeat", 50)}</td><td>${i("resolved", 50)}</td><td>${i("notes", 120)}</td><td><button class="btn small wine" data-act="repl-delrow">${icon("x")}</button></td>`; tb.appendChild(tr);
}

async function replForm(id, equipmentId) {
  if (!state.repl) await loadRepl();
  const d = state.repl; let r = id ? d.requests.find(x => x.id === id) : null;
  if (!r) { r = {date: ymd(new Date()), requestedBy: who(), history: [], condition: "Fair", critical: ""}; if (equipmentId) { try { Object.assign(r, await api("/api/replacements/prefill?equipmentId=" + encodeURIComponent(equipmentId))); } catch (e) { toast(e.message, true); } } }
  const t = (k, l, x) => `<label>${l}</label><input class="field" data-rp="${k}" value="${esc(r[k] == null ? "" : r[k])}" ${x || ""}>`, a = (k, l) => `<label>${l}</label><textarea class="field" rows="2" data-rp="${k}">${esc(r[k] || "")}</textarea>`;
  const s = (k, l, o) => `<label>${l}</label><select class="field" data-rp="${k}" style="width:auto">${["", ...o].map(v => `<option ${v === (r[k] || "") ? "selected" : ""}>${esc(v)}</option>`).join("")}</select>`, num = (k, l) => t(k, l, 'type="number" min="0" step="0.01" style="width:160px"');
  showModal(`<h2>${icon("wrench")}${id ? esc(id) + " " : ""}Equipment Replacement Justification</h2><div class="body" id="rpbody" data-id="${esc(id || "")}"><div class="fields">
    <h3 style="margin:0">Asset Information</h3><label>Equipment: <span class="req">*</span></label><input class="field" data-rp="equipment" list="rpeq" value="${esc(r.equipment || "")}"><datalist id="rpeq">${d.equipment.map(e => `<option value="${esc(e.Name)}">`).join("")}</datalist><input type="hidden" data-rp="equipmentId" value="${esc(r.equipmentId || "")}">
    ${t("assetCode", "Asset ID:")}${t("area", "Area:")}${t("makeModel", "Manufacturer / model:")}${t("age", "Age / install year:")}${num("originalCost", "Original cost ($):")}${num("replaceCost", "Estimated replacement cost ($):")}
    ${s("condition", 'Current condition: <span class="req">*</span>', d.conditions)}${s("critical", "Critical to operation?", ["Yes", "No"])}${t("maintainxRef", "MaintainX / PM ref:")}${t("vendor", "Primary vendor:")}${t("requestedBy", 'Requested by: <span class="req">*</span>')}${t("date", 'Request date: <span class="req">*</span>', 'type="date" style="width:190px"')}</div>
    <h3 style="margin:12px 0 4px">Repair / Failure History</h3><div style="overflow:auto"><table class="plain" id="rphist" style="font-size:12px"><thead><tr>${["Date", "WO / ref", "Failure / repair", "Down hrs", "Repair $", "Parts $", "Vendor", "Repeat?", "Fixed?", "Notes"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}<th class="nosort"></th></tr></thead><tbody></tbody></table></div>
    <div class="row"><button class="btn small" data-act="repl-addrow">${icon("plus")}Add Row</button><span class="hint">Filled in from the equipment's repair issues when you start from the equipment.</span></div>
    <div class="fields"><h3 style="margin:8px 0 0">Replacement Case</h3>${a("reasons", 'Reason(s) for replacement: <span class="req">*</span>')}${a("opImpact", "Operational impact:")}${a("safetyRisk", "Food safety / safety risk:")}${a("partsConcern", "Availability / parts concern:")}${num("annualRepair", "Estimated annual repair cost ($):")}${num("annualDowntime", "Estimated annual downtime cost ($):")}
    ${s("energy", "Energy / efficiency concern?", ["Yes", "No"])}${a("spec", "Recommended replacement / specification:")}${s("recommendation", 'Recommendation: <span class="req">*</span>', d.recs)}${s("priority", "Priority:", d.priorities)}
    <h3 style="margin:8px 0 0">Approval / Decision</h3>${s("decision", "Management decision:", d.decisions)}${t("decisionDate", "Decision date:", 'type="date" style="width:190px"')}${t("approvedBy", "Decided by:")}${t("budgetRef", "Budget / capital ref:")}${t("targetPurchase", "Target purchase date:", 'type="date" style="width:190px"')}${t("vendorQuote", "Vendor / quote:")}${a("decisionNotes", "Final decision notes:")}
    ${s("completed", "Replacement completed?", ["Yes", "No"])}${t("newAssetId", "New asset ID:")}${t("completionDate", "Completion date:", 'type="date" style="width:190px"')}${t("oldDisposition", "Old asset disposition:")}</div></div>
    <div class="foot"><button class="btn green big" data-act="repl-save">${icon("save")}Save Request</button>${id ? `<button class="btn big" data-act="repl-print" data-id="${esc(id)}">${icon("print")}Print</button>` : ""}<button class="btn big" data-act="close">Cancel</button></div>`);
  (r.history || []).forEach(replRow);
}

async function replPick(inp) {
  const e = state.repl.equipment.find(x => x.Name.toLowerCase() === inp.value.trim().toLowerCase()); if (!e) { $('[data-rp="equipmentId"]').value = ""; return; }
  try { const p = await api("/api/replacements/prefill?equipmentId=" + encodeURIComponent(e.ID)), set = (k, v) => { const el = $(`[data-rp="${k}"]`); if (el && !el.value && v) el.value = v; };
    $('[data-rp="equipmentId"]').value = e.ID; ["area", "makeModel", "age", "vendor", "maintainxRef"].forEach(k => set(k, p[k])); ["originalCost", "replaceCost", "annualRepair"].forEach(k => set(k, p[k] || "")); const tb = $("#rphist tbody"); if (!tb.children.length) (p.history || []).forEach(replRow); } catch (e2) { /* not fatal */ }
}

async function loadInsp() { state.insp = await api("/api/inspections"); if (state.view === "insp") render(); }

function vInsp() {
  const d = state.insp; if (!d) { loadInsp(); return `<p class="hint">Loading...</p>`; }
  if (!state.maint) { loadMaint().then(render); return `<p class="hint">Loading...</p>`; }
  const list = [...d.inspections].reverse();
  return `<p class="hint" style="margin-top:0">Do the 16-point check on a piece of equipment during its PM. A failed point needs a maintenance issue before the PM job can be marked done. You can also print a blank sheet to fill in by hand.</p>
  <div class="row"><button class="btn green big" data-act="insp-new" data-task="">${icon("plus")}New Inspection</button><button class="btn" data-act="rep-view" data-id="inspection:blank">${icon("print")}Print Blank Sheet</button></div>
  <div class="tablewrap">${!list.length ? `<div class="empty"><b>No inspections yet</b>Use New Inspection, or Inspect on the Equipment PM screen.</div>`
    : `<table class="plain"><thead><tr>${["#", "Date", "Equipment", "PM Job", "Condition", "Failed", "By", "Issue"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}<th class="nosort"></th></tr></thead><tbody>
      ${list.map(q => { const is = state.maint.issues.find(p => p.id === q.issueId); return `<tr><td>${q.number}</td><td>${fmtDate(q.date)}</td><td>${esc(eqName(q.equipmentId))}</td><td>${esc(q.taskName)}</td><td>${tag(q.condition)}</td><td>${q.fails || ""}</td><td>${esc(q.by)}</td><td>${is ? "#" + is.number : ""}</td>
        <td style="text-align:right"><div class="acts"><button class="btn small" data-act="insp-edit" data-id="${q.id}">${icon("edit")}Edit</button> <button class="btn small" data-act="rep-view" data-id="inspection:${q.id}">${icon("print")}View / Print</button></div>${q.edited ? `<div class="hint">Edited ${esc(q.edited)}</div>` : ""}</td></tr>`; }).join("")}
    </tbody></table>`}</div>`;
}

async function inspForm(taskId) {
  if (!state.insp) await loadInsp(); if (!state.maint) await loadMaint();
  const t = (state.maint.pm || []).find(x => x.id === taskId) || {}, d = state.insp, eqs = state.maint.equipment;
  showModal(`<h2>${icon("wrench")}PM Inspection${t.name ? ": " + esc(t.name) : ""}</h2><div class="body" id="inbody" data-task="${esc(taskId || "")}"><div class="fields">
    <label>Equipment: <span class="req">*</span></label><select class="field" data-in="equipmentId"><option value="">Choose...</option>${eqs.map(e => `<option value="${e.id}" ${e.id === t.equipmentId ? "selected" : ""}>${esc(e.name)}</option>`).join("")}</select>
    <label>PM job:</label><select class="field" data-in="taskId"><option value="">None</option>${(state.maint.pm || []).filter(x => x.group === "Equipment PM").map(x => `<option value="${x.id}" ${x.id === taskId ? "selected" : ""}>${esc(x.name)}${x.equipmentId ? " · " + esc(eqName(x.equipmentId)) : ""}</option>`).join("")}</select>
    <label>Date:</label><input type="date" class="field" data-in="date" value="${ymd(new Date())}" style="width:190px">
    <label>Condition: <span class="req">*</span></label><select class="field" data-in="condition" id="incond">${d.conditions.map(c => `<option>${c}</option>`).join("")}</select></div>
    <table class="plain" style="margin:8px 0"><thead><tr><th class="nosort" style="width:auto">Inspection point</th><th class="nosort" style="width:auto">Result</th><th class="nosort" style="width:auto">Notes</th></tr></thead><tbody>
    ${d.items.map(it => `<tr><td>${esc(it.label)}</td><td style="white-space:nowrap">${["Pass", "Fail", "N/A"].map(v => `<label style="font-weight:400;margin-right:8px"><input type="radio" name="in_${it.key}" value="${v}" ${v === "Pass" ? "checked" : ""}> ${v}</label>`).join("")}</td><td><input class="field" data-in-note="${it.key}" placeholder="Optional"></td></tr>`).join("")}
    </tbody></table><div class="fields">
    <label>Readings:</label><input class="field" data-in="readings" placeholder="Temperatures, pressures, amps...">
    <label>Work performed:</label><textarea class="field" rows="2" data-in="work"></textarea>
    <label>Parts used:</label><input class="field" data-in="parts" placeholder="Optional">
    <label>Corrective action needed:</label><textarea class="field" rows="2" data-in="corrective" placeholder="Required if anything failed or the condition is Poor or Unsafe"></textarea>
    <label>Priority:</label><select class="field" data-in="priority">${state.maint.priorities.map(x => `<option ${x === "P3 - Routine" ? "selected" : ""}>${x}</option>`).join("")}</select>
    <label></label><label style="font-weight:400"><input type="checkbox" data-in="createIssue" checked> Create a maintenance issue for the corrective action</label>
    <label></label><label style="font-weight:400"><input type="checkbox" data-in="markDone" ${taskId ? "checked" : ""}> Mark the PM job done</label>
    <label>Your name: <span class="req">*</span></label><input class="field" data-in="by" value="${esc(who())}"></div></div>
    <div class="foot"><button class="btn green big" data-act="insp-save">${icon("save")}Save Inspection</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

function inspEdit(id) {
  const q = state.insp.inspections.find(x => x.id === id); if (!q) return;
  const res = Object.fromEntries((q.items || []).map(i => [i.key, i]));
  showModal(`<h2>${icon("edit")}Edit PM Inspection #${q.number}: ${esc(eqName(q.equipmentId))}</h2><div class="body" id="iebody" data-id="${esc(q.id)}"><div class="fields">
    <label>Date:</label><input type="date" class="field" data-ie="date" value="${esc(q.date)}" style="width:190px">
    <label>Condition:</label><select class="field" data-ie="condition">${state.insp.conditions.map(c => `<option ${c === q.condition ? "selected" : ""}>${c}</option>`).join("")}</select></div>
    <table class="plain" style="margin:8px 0"><thead><tr><th class="nosort" style="width:auto">Inspection point</th><th class="nosort" style="width:auto">Result</th><th class="nosort" style="width:auto">Notes</th></tr></thead><tbody>
    ${state.insp.items.map(it => { const r = res[it.key] || {result: "Pass", note: ""}; return `<tr><td>${esc(it.label)}</td><td style="white-space:nowrap">${["Pass", "Fail", "N/A"].map(v => `<label style="font-weight:400;margin-right:8px"><input type="radio" name="ie_${it.key}" value="${v}" ${v === r.result ? "checked" : ""}> ${v}</label>`).join("")}</td><td><input class="field" data-ie-note="${it.key}" value="${esc(r.note || "")}"></td></tr>`; }).join("")}</tbody></table><div class="fields">
    <label>Readings:</label><input class="field" data-ie="readings" value="${esc(q.readings || "")}">
    <label>Work performed:</label><textarea class="field" rows="2" data-ie="work">${esc(q.work || "")}</textarea>
    <label>Parts used:</label><input class="field" data-ie="parts" value="${esc(q.parts || "")}">
    <label>Corrective action needed:</label><textarea class="field" rows="2" data-ie="corrective">${esc(q.corrective || "")}</textarea>
    <label>Priority:</label><select class="field" data-ie="priority">${state.maint.priorities.map(x => `<option ${x === q.priority ? "selected" : ""}>${x}</option>`).join("")}</select>
    <label>Your name: <span class="req">*</span></label><input class="field" data-ie="editedBy" value="${esc(who())}"></div>
    <p class="hint">Changes here fix the record only. They do not create or change maintenance issues.</p></div>
    <div class="foot"><button class="btn green big" data-act="insp-edit-go">${icon("save")}Save Changes</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

function vProcs() {
  const p = state.procs; if (!p) { api("/api/procedures").then(r => { state.procs = r; render(); }); return `<p class="hint">Loading...</p>`; }
  return `<div class="row noprint"><button class="btn" data-act="rep-view" data-id="procedures">${icon("print")}Print</button><span class="hint">The wording is in Maintenance Data\\procedures.json if you want to change it.</span></div>
  <h3>How we handle maintenance</h3>${p.steps.map(s => `<div class="panel" style="margin:8px 0"><b>${esc(s.title)}</b><div>${esc(s.text)}</div></div>`).join("")}
  <h3>Priorities</h3><div style="border:1px solid var(--line);border-radius:8px"><table class="plain"><thead><tr><th class="nosort" style="width:auto">Priority</th><th class="nosort" style="width:auto">Use</th><th class="nosort" style="width:auto">Response</th></tr></thead><tbody>
    ${p.priorities.map(x => `<tr><td><b>${esc(x.priority)}</b></td><td>${esc(x.use)}</td><td>${esc(x.response)}</td></tr>`).join("")}</tbody></table></div>
  <h3>Review schedule</h3><div style="border:1px solid var(--line);border-radius:8px"><table class="plain"><thead><tr><th class="nosort" style="width:auto">When</th><th class="nosort" style="width:auto">Review</th><th class="nosort" style="width:auto">Owner</th></tr></thead><tbody>
    ${p.cadence.map(x => `<tr><td><b>${esc(x.when)}</b></td><td>${esc(x.task)}</td><td>${esc(x.owner)}${x.backup ? `<div class="hint">${esc(x.backup)}</div>` : ""}</td></tr>`).join("")}</tbody></table></div>`;
}

function vIssues() {
  const m = state.maint; if (!m) { loadMaint().then(render); return `<p class="hint">Loading...</p>`; }
  const all = state.showAll, list = [...m.issues].reverse().filter(p => all || (p.status !== "Verified" && p.status !== "Closed"));
  return `<div class="row"><button class="btn wine big" data-act="issue-new" data-id="">${icon("warn")}Report Issue</button>
    <label style="font-weight:400"><input type="checkbox" data-act="issues-all" ${all ? "checked" : ""}> Show completed issues too</label></div>
  <div class="tablewrap">${!list.length ? `<div class="empty"><b>${all ? "No issues yet" : "No open issues"}</b>Use Report Issue when a piece of equipment is not working right.</div>`
    : `<table><thead><tr>${["#", "Equipment", "Issue", "Priority", "Status", "Reported", "Work Order"].map(h => `<th class="nosort" style="width:auto">${h}</th>`).join("")}<th class="nosort"></th></tr></thead><tbody>
      ${list.map(p => `<tr data-act="issue-open" data-id="${p.id}"><td>${p.number}</td><td>${esc(eqName(p.equipmentId))}</td><td>${esc(p.title)}</td><td>${tag(p.priority)}</td><td>${tag(p.status)}</td><td>${fmtDate(p.reportedAt)}</td><td>${esc(p.workOrder)}</td><td style="text-align:right"><div class="acts"><button class="btn small" data-act="issue-open" data-id="${p.id}">${icon("edit")}Edit</button></div></td></tr>`).join("")}
    </tbody></table>`}</div>`;
}

function eqForm(id) {
  const e = (state.maint.equipment || []).find(x => x.id === id) || {status: "Operational"};
  const f = (k, label, ph = "") => `<label>${label}</label><input class="field" data-eq="${k}" value="${esc(e[k] || "")}" placeholder="${ph}">`;
  showModal(`<h2>${icon("wrench")}${id ? "Edit" : "Add"} Equipment</h2><div class="body" id="eqbody" data-id="${esc(id)}"><div class="fields">
    ${f("name", "Name: <span class='req'>*</span>", "e.g. Walk-in Cooler 1")}
    <label>Area:</label><div><select class="field" data-eq="area" style="width:auto">${["", ...(state.maint.areas || []), ...(e.area && !(state.maint.areas || []).includes(e.area) ? [e.area] : [])].map(a => `<option ${a === (e.area || "") ? "selected" : ""}>${esc(a)}</option>`).join("")}</select> <span class="hint">Add more areas in Settings, Equipment Areas.</span></div>
    <label>Type:</label><div><select class="field" data-eq="type" style="width:auto">${["", ...(state.maint.types || EQ_TYPES), ...(e.type && !(state.maint.types || EQ_TYPES).includes(e.type) ? [e.type] : [])].map(t => `<option ${t === (e.type || "") ? "selected" : ""}>${esc(t)}</option>`).join("")}</select> <span class="hint">Add more types in Settings, Equipment Areas &amp; Types.</span></div>
    ${f("assetCode", "Asset ID:", "e.g. K-001")}${f("make", "Make:")}${f("model", "Model:")}${f("serial", "Serial Number:")}
    <label>Installed:</label><input type="date" class="field" data-eq="installed" value="${esc(e.installed || "")}" style="width:190px">
    ${f("vendor", "Bought From:")}<label>Purchase Date:</label><input type="date" class="field" data-eq="purchaseDate" value="${esc(e.purchaseDate || "")}" style="width:190px">
    <label>Purchase Cost ($):</label><input type="number" min="0" step="0.01" class="field" data-eq="purchaseCost" value="${e.purchaseCost || ""}" style="width:150px">
    <label>Warranty Ends:</label><input type="date" class="field" data-eq="warrantyEnd" value="${esc(e.warrantyEnd || "")}" style="width:190px">
    <label>New Unit Costs ($):</label><input type="number" min="0" step="0.01" class="field" data-eq="replaceCost" value="${e.replaceCost || ""}" style="width:150px" title="What a replacement would cost today. Used to compare repair costs.">
    <label>Normal Life (years):</label><input type="number" min="0" class="field" data-eq="expectedYears" value="${e.expectedYears || ""}" style="width:110px">
    ${f("maintainxId", "MaintainX Asset:", "Optional: its name or number in MaintainX")}
    <label>Status:</label>${e.status === "Out of Service" ? `<div><select class="field" data-eq="status" disabled><option>Out of Service</option></select><div class="hint">Out of service. A manager clears it with Clear for Service on the equipment list.</div></div>` : `<select class="field" data-eq="status">${state.maint.statuses.filter(x => x !== "Out of Service").map(x => `<option ${x === e.status ? "selected" : ""}>${x}</option>`).join("")}</select>`}
    <label>Notes:</label><textarea class="field" rows="2" data-eq="notes">${esc(e.notes || "")}</textarea></div></div>
    <div class="foot"><button class="btn green big" data-act="eq-save">${icon("save")}Save</button>${id ? `<button class="btn wine big" data-act="eq-delete" data-id="${esc(id)}">${icon("trash")}Delete</button>` : ""}<button class="btn big" data-act="close">Cancel</button></div>`);
}

function issueForm(eqId) {
  showModal(`<h2>${icon("warn")}Report an Issue</h2><div class="body" id="isbody"><div class="fields">
    <label>Equipment: <span class="req">*</span></label><select class="field" data-is="equipmentId"><option value="">Choose...</option>${state.maint.equipment.map(e => `<option value="${e.id}" ${e.id === eqId ? "selected" : ""}>${esc(e.name)}</option>`).join("")}</select>
    <label>What is wrong: <span class="req">*</span></label><input class="field" data-is="title" placeholder="Short, e.g. Not holding temperature">
    <label>Details:</label><textarea class="field" rows="3" data-is="description" placeholder="What happened, when it started, anything tried"></textarea>
    <label>Priority:</label><select class="field" data-is="priority">${state.maint.priorities.map(x => `<option ${x === "P3 - Routine" ? "selected" : ""}>${x}</option>`).join("")}</select>
    <label></label><div class="hint" id="priohint">${esc(state.maint.priorityUse["P3 - Routine"])}</div>
    <label>Out of service:</label><label style="font-weight:400"><input type="checkbox" data-is="outOfService"> Do not use this equipment until it is fixed</label>
    <label>Your name: <span class="req">*</span></label><input class="field" data-is="reportedBy" value="${esc(who())}"></div></div>
    <div class="foot"><button class="btn wine big" data-act="issue-save">${icon("save")}Save Issue</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

function issueDetail(id) {
  const p = state.maint.issues.find(x => x.id === id); if (!p) return;
  showModal(`<h2>${icon("wrench")}#${p.number} ${esc(p.title)}</h2><div class="body" id="idbody" data-id="${p.id}">
    <p><b>${esc(eqName(p.equipmentId))}</b> · ${tag(p.priority)} ${tag(p.status)}<br><span class="hint">Reported ${fmtDate(p.reportedAt)} by ${esc(p.reportedBy)}</span></p>
    ${p.description ? `<p>${esc(p.description).replace(/\n/g, "<br>")}</p>` : ""}
    <div class="fields"><label>Status:</label><select class="field" data-id-f="status">${state.maint.issueStatuses.map(x => `<option ${x === p.status ? "selected" : ""}>${x}</option>`).join("")}</select>
      <label>Work order:</label><input class="field" data-id-f="workOrder" value="${esc(p.workOrder)}" placeholder="Number or link from MaintainX">
      <label>Assigned to:</label><input class="field" data-id-f="assignedTo" value="${esc(p.assignedTo || "")}" placeholder="Technician or vendor">
      <label>Repair costs ($):</label><div class="row"><span class="hint">Parts</span><input type="number" min="0" step="0.01" class="field" data-id-f="parts" value="${p.partsCost || ""}" style="width:110px" placeholder="0.00"><span class="hint">Labor / service</span><input type="number" min="0" step="0.01" class="field" data-id-f="labor" value="${p.laborCost || ""}" style="width:110px" placeholder="0.00"><span class="hint">Downtime (hours)</span><input type="number" min="0" step="0.5" class="field" data-id-f="downtime" value="${p.downtime || ""}" style="width:90px" placeholder="0"></div>
      <label>Follow-up:</label><label style="font-weight:400"><input type="checkbox" class="field" data-id-f="followUp" ${p.followUp ? "checked" : ""}> More work is still needed</label>
      <label>Completion notes:</label><textarea class="field" rows="2" data-id-f="completionNotes" placeholder="What was done, parts replaced, anything to watch">${esc(p.completionNotes || "")}</textarea>
      <label>Add a note:</label><input class="field" data-id-f="note" placeholder="Optional">
      <label>Your name: <span class="req">*</span></label><input class="field" data-id-f="by" value="${esc(who())}"></div>
    <p class="hint">A technician finishing the repair is “Repaired”. Choose “Verified” only after a manager has confirmed the equipment is working and ready for food production.</p>
    <div class="hist">${[...p.history].reverse().map(h => `<div><b>${fmtDate(h.at)} ${esc(h.at.slice(11, 16))}</b> ${esc(h.by)}: ${esc(h.text)}</div>`).join("")}</div></div>
    <div class="foot"><button class="btn green big" data-act="issue-update">${icon("save")}Save</button><button class="btn big" data-act="issue-wo" data-id="${p.id}">${icon("ext")}Copy &amp; Submit Work Order</button><button class="btn big" data-act="close">Close</button></div>`);
}

const GROUPS = {
  "Equipment PM": {title: "equipment preventive maintenance", add: "Add Job", empty: "Add recurring jobs such as a fryer boil-out, a filter change or a thermometer check, and they will show here when they are due.", tpl: "PM_TEMPLATES"},
  "Cleaning": {title: "cleaning task", add: "Add Cleaning Task", empty: "Add the daily, weekly, monthly and yearly cleaning your kitchen does, and they will show on the calendar. A manager marks each one done.", tpl: "CLEAN_TEMPLATES"},
  "Grounds / Facility": {title: "grounds and facility job", add: "Add Facility Job", empty: "Add recurring building and grounds jobs such as pest control, gutters, parking lot and roof checks.", tpl: "FAC_TEMPLATES"},
};

const FREQS = ["Daily", "Weekly", "Monthly", "Quarterly / Semi-Annual", "Yearly", "As Needed"];

function vPM(group) {
  const m = state.maint; if (!m) { loadMaint().then(render); return `<p class="hint">Loading...</p>`; }
  const g = GROUPS[group], list = (m.pm || []).filter(t => t.group === group).sort((a, b) => FREQS.indexOf(a.bucket) - FREQS.indexOf(b.bucket) || a.nextDue.localeCompare(b.nextDue));
  const late = list.filter(t => t.state === "Overdue").length, eq = group === "Equipment PM";
  const when = t => t.state === "Overdue" ? `${-t.days} day${t.days === -1 ? "" : "s"} overdue` : t.days === 0 ? "due today" : `in ${t.days} day${t.days === 1 ? "" : "s"}`;
  return `<div class="row"><button class="btn green big" data-act="pm-edit" data-id="" data-group="${esc(group)}">${icon("plus")}${g.add}</button>
    <span class="hint">${list.length} recurring job${list.length === 1 ? "" : "s"}${late ? ` · <b>${late} overdue</b>` : ""}</span></div>
  <div class="tablewrap">${!list.length ? `<div class="empty"><b>Nothing here yet</b>${g.empty}</div>`
    : `<table class="plain"><thead><tr>${["Equipment / Area", "Job", "Repeats", "Last Done", "Next Due", "Status"].map(h => `<th class="nosort" style="width:auto">${h}</th>`).join("")}<th class="nosort" style="width:300px"></th></tr></thead><tbody>
      ${list.map(t => `<tr><td>${esc([t.equipmentId ? eqName(t.equipmentId) : "", t.area].filter(Boolean).join(" · "))}${t.assetCode ? `<div class="hint">${esc(t.assetCode)}</div>` : ""}</td><td><b>${esc(t.name)}</b>${t.performer ? `<div class="hint">${esc(t.performer)}</div>` : ""}${t.notes ? `<div class="hint">${esc(t.notes)}</div>` : ""}</td><td>${esc(t.freqLabel || t.bucket)}<div class="hint">${t.unit === "asneeded" ? "not scheduled" : `every ${t.every} ${t.every === 1 ? t.unit.replace(/s$/, "") : t.unit}`}${t.code ? " · " + esc(t.code) : ""}</div></td><td>${fmtDate(t.lastDone) || '<span class="hint">Never</span>'}</td><td>${t.nextDue ? `${fmtDate(t.nextDue)}<div class="hint">${when(t)}</div>` : '<span class="hint">As needed</span>'}</td><td>${tag(t.state)}</td>
        <td style="text-align:right"><div class="acts"><button class="btn small green" data-act="pm-done-ask" data-id="${t.id}">${icon("save")}Mark Done</button> ${eq ? `<button class="btn small" data-act="insp-new" data-task="${t.id}">Inspect</button> ` : ""}<button class="btn small" data-act="pm-edit" data-id="${t.id}" data-group="${esc(group)}">${icon("edit")}Edit</button></div></td></tr>`).join("")}
    </tbody></table>`}</div>`;
}

function pmForm(id, group) {
  const t = (state.maint.pm || []).find(x => x.id === id) || {every: 1, unit: "weeks", firstDue: ymd(new Date()), group};
  group = t.group; const tpls = window[GROUPS[group].tpl] || [];
  state.tpls = tpls;
  showModal(`<h2>${icon("wrench")}${id ? "Edit" : "Add"} ${GROUPS[group].title[0].toUpperCase() + GROUPS[group].title.slice(1)}</h2><div class="body" id="pmbody" data-id="${esc(id)}"><div class="fields">
    ${id ? "" : `<label>Start from:</label><select class="field" id="pmtpl"><option value="">A common job (optional)...</option>${tpls.map((x, i) => `<option value="${i}">${x[0]}</option>`).join("")}</select>`}
    <label>Group:</label><select class="field" data-pm="group" style="width:230px">${Object.keys(GROUPS).map(g => `<option ${g === group ? "selected" : ""}>${g}</option>`).join("")}</select>
    <label>Job: <span class="req">*</span></label><input class="field" data-pm="name" value="${esc(t.name || "")}" placeholder="What needs doing">
    <label>Equipment:</label><select class="field" data-pm="equipmentId"><option value="">None / not listed</option>${state.maint.equipment.map(e => `<option value="${e.id}" ${e.id === t.equipmentId ? "selected" : ""}>${esc(e.name)}</option>`).join("")}</select>
    <label>Equipment / Area:</label><div><input class="field" data-pm="area" list="pmareas" value="${esc(t.area || "")}" placeholder="What it is on or where it is done"><datalist id="pmareas">${(state.lib.locations || []).map(l => `<option value="${esc(l)}">`).join("")}</datalist></div>
    <label>Repeats every:</label><div class="row"><input class="field" type="number" min="1" data-pm="every" value="${t.every || 1}" style="width:80px"><select class="field" data-pm="unit" style="width:140px">${[["days", "days"], ["weeks", "weeks"], ["months", "months"], ["years", "years"], ["asneeded", "As needed (not scheduled)"]].map(([u, l]) => `<option value="${u}" ${u === t.unit ? "selected" : ""}>${l}</option>`).join("")}</select></div>
    <label>Frequency as written:</label><input class="field" data-pm="freqLabel" value="${esc(t.freqLabel || "")}" placeholder="Optional, e.g. 3x a week, Seasonal">
    <label>${t.lastDone ? "First due (before it was first done):" : "First due:"}</label><input type="date" class="field" data-pm="firstDue" value="${esc(t.firstDue || "")}" style="width:190px">
    <label>Done by:</label><input class="field" data-pm="performer" value="${esc(t.performer || "")}" placeholder="Person, role or vendor">
    <label>Job ID:</label><div class="row"><input class="field" data-pm="code" value="${esc(t.code || "")}" placeholder="e.g. PM-001" style="width:120px"><label style="font-weight:400">Asset ID</label><input class="field" data-pm="assetCode" value="${esc(t.assetCode || "")}" placeholder="e.g. K-001" style="width:120px"></div>
    <label>MaintainX PM ID:</label><input class="field" data-pm="maintainxId" value="${esc(t.maintainxId || "")}" placeholder="Optional">
    <label>Notes:</label><textarea class="field" rows="2" data-pm="notes" placeholder="How it is done, who does it">${esc(t.notes || "")}</textarea></div>
    ${id ? `<div class="hist">${[...(t.history || [])].reverse().map(h => `<div><b>${fmtDate(h.at)}</b> ${esc(h.by)}: ${esc(h.text)}</div>`).join("") || "<div class='hint'>Not done yet.</div>"}</div>` : ""}</div>
    <div class="foot"><button class="btn green big" data-act="pm-save">${icon("save")}Save</button>${id ? `<button class="btn wine big" data-act="pm-delete" data-id="${esc(id)}">${icon("trash")}Delete</button>` : ""}<button class="btn big" data-act="close">Cancel</button></div>`);
}

function pmDone(id) {
  const t = state.maint.pm.find(x => x.id === id);
  showModal(`<h2>${icon("save")}Mark Done</h2><div class="body" id="pdbody" data-id="${id}"><p><b>${esc(t.name)}</b> on ${esc(eqName(t.equipmentId))}</p><div class="fields">
    <label>Date done:</label><input type="date" class="field" data-pd="date" value="${ymd(new Date())}" style="width:190px">
    <label>Cost ($):</label><input type="number" min="0" step="0.01" class="field" data-pd="cost" style="width:150px" placeholder="0.00 (optional)">
    <label>Note:</label><input class="field" data-pd="note" placeholder="Optional">
    <label>Your name: <span class="req">*</span></label><input class="field" data-pd="by" value="${esc(who())}"></div></div>
    <div class="foot"><button class="btn green big" data-act="pm-done-go">${icon("save")}Save</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

function vAssets() {
  const m = state.maint; if (!m) { loadMaint().then(render); return `<p class="hint">Loading...</p>`; }
  const A = m.assets || {}, list = m.equipment, flag = list.filter(e => (A[e.id] || {}).advice === "Consider Replacing").length;
  return `<p class="hint" style="margin-top:0">Add model, serial number and purchase details on each unit, then record every repair and its cost. When upkeep reaches ${m.replacePercent}% of the cost of a new unit, the program suggests considering a replacement. You can change that limit in Settings, MaintainX.</p>
  <div class="row"><button class="btn green big" data-act="eq-edit" data-id="">${icon("plus")}Add Equipment</button>${flag ? `<span class="hint"><b>${flag} unit${flag === 1 ? "" : "s"} to consider replacing</b></span>` : ""}</div>
  <div class="tablewrap">${!list.length ? `<div class="empty"><b>No equipment yet</b>Click “Add Equipment” to start the asset list.</div>`
    : `<table class="plain"><thead><tr>${["Equipment", "Make / Model", "Serial Number", "Age", "Warranty", "Spent on Upkeep", "Last 12 Months", "Suggestion"].map(h => `<th class="nosort" style="width:auto">${h}</th>`).join("")}<th class="nosort" style="width:190px"></th></tr></thead><tbody>
      ${list.map(e => { const a = A[e.id] || {}; return `<tr><td><b>${esc(e.name)}</b><div class="hint">${esc([e.assetCode, e.type].filter(Boolean).join(" · "))}</div></td><td>${esc([e.make, e.model].filter(Boolean).join(" "))}</td><td>${esc(e.serial)}</td><td>${a.ageYears ? a.ageYears + " yr" : ""}</td><td>${esc(a.warranty || "")}${e.warrantyEnd ? `<div class="hint">${fmtDate(e.warrantyEnd)}</div>` : ""}</td>
        <td>${money(a.total)}${a.percent >= 0 ? `<div class="hint">${a.percent}% of a new unit</div>` : ""}</td><td>${money(a.last12)}</td><td>${a.advice ? tag(a.advice) : '<span class="hint">Add purchase cost</span>'}</td>
        <td style="text-align:right"><div class="acts"><button class="btn small" data-act="asset-open" data-id="${e.id}">Details</button> <button class="btn small" data-act="eq-edit" data-id="${e.id}">${icon("edit")}Edit</button></div></td></tr>`; }).join("")}
    </tbody></table>`}</div>`;
}

function assetDetail(id) {
  const m = state.maint, e = m.equipment.find(x => x.id === id); if (!e) return;
  const a = (m.assets || {})[id] || {}, recs = (m.service || []).filter(r => r.equipmentId === id), line = (k, v) => v ? `<div><span class="hint">${k}:</span> ${esc(v)}</div>` : "";
  showModal(`<h2>${icon("wrench")}${esc(e.name)}</h2><div class="body" id="asbody" data-id="${id}">
    <div class="cols"><div>${line("Type", e.type)}${line("Area", e.area)}${line("Make", e.make)}${line("Model", e.model)}${line("Serial number", e.serial)}${line("Installed", fmtDate(e.installed))}${line("Bought from", e.vendor)}${line("MaintainX asset", e.maintainxId)}</div>
      <div>${line("Purchased", fmtDate(e.purchaseDate) + (e.purchaseCost ? " for " + money(e.purchaseCost) : ""))}${line("Warranty ends", fmtDate(e.warrantyEnd))}${line("New unit costs", e.replaceCost ? money(e.replaceCost) : "")}${line("Normal life", e.expectedYears ? e.expectedYears + " years" : "")}${line("Age", a.ageYears ? a.ageYears + " years" : "")}</div></div>
    <p style="margin:12px 0 4px"><b>Spent on upkeep: ${money(a.total)}</b> · last 12 months ${money(a.last12)} · ${a.repairs || 0} repair${a.repairs === 1 ? "" : "s"} ${a.advice ? tag(a.advice) : ""}</p>
    <p class="hint" style="margin-top:0">${esc(a.reason || "Enter the purchase cost or the cost of a new unit to compare repair costs.")} This is a rule of thumb; also consider downtime, energy use and parts availability.</p>
    <h3 style="margin:10px 0 4px">Maintenance and repair history</h3>
    <div class="hist" style="max-height:150px">${recs.map(r => `<div><b>${fmtDate(r.date)}</b> ${esc(r.type)}: ${esc(r.description)} · <b>${money(r.cost)}</b>${r.vendor ? " · " + esc(r.vendor) : ""} <span class="hint">(${esc(r.by)})</span> <a href="#" data-act="svc-edit" data-id="${r.id}" class="hint">edit</a> · <a href="#" data-act="svc-delete" data-id="${r.id}" class="hint">remove</a></div>`).join("") || "<div class='hint'>Nothing recorded yet.</div>"}</div>
    <h3 style="margin:12px 0 4px">Record maintenance</h3><div class="fields">
      <label>Date:</label><input type="date" class="field" data-sv="date" value="${ymd(new Date())}" style="width:190px">
      <label>Type:</label><select class="field" data-sv="type" style="width:190px">${["Repair", "Preventive", "Inspection", "Other"].map(t => `<option>${t}</option>`).join("")}</select>
      <label>What was done:</label><input class="field" data-sv="description" placeholder="e.g. Replaced compressor">
      <label>Cost ($):</label><input type="number" min="0" step="0.01" class="field" data-sv="cost" style="width:150px" placeholder="0.00">
      <label>Done by (company):</label><input class="field" data-sv="vendor" placeholder="Optional">
      <label>Your name: <span class="req">*</span></label><input class="field" data-sv="by" value="${esc(who())}"></div></div>
    <div class="foot"><button class="btn green big" data-act="svc-save">${icon("save")}Save Record</button><button class="btn big" data-act="eq-edit" data-id="${id}">${icon("edit")}Edit Asset Details</button><button class="btn big" data-act="repl-new" data-eq="${id}">Replacement Request</button><button class="btn big" data-act="close">Close</button></div>`);
}

async function loadFinds() { state.finds = await api("/api/maint/findings"); if (state.view === "finds") render(); }

function vFinds() {
  const d = state.finds; if (!d) { loadFinds(); return `<p class="hint">Loading...</p>`; }
  if (!state.maint) { loadMaint().then(render); return `<p class="hint">Loading...</p>`; }
  const all = state.findAll, today = ymd(new Date()), list = [...d.findings].reverse().filter(f => all || f.status !== "Done");
  return `<p class="hint" style="margin-top:0">Write down anything you find on a walk-through that needs doing. Open items show on the To-Do List until they are marked Done. For a repair, enter it in MaintainX with Submit Work Order and write the number on the item.</p>
  <div class="row"><button class="btn green big" data-act="find-new">${icon("plus")}Add Inspection Item</button>
    <label style="font-weight:400"><input type="checkbox" data-act="finds-all" ${all ? "checked" : ""}> Show completed items too</label></div>
  <div class="tablewrap">${!list.length ? `<div class="empty"><b>${all ? "Nothing logged yet" : "Nothing open"}</b>Use Add Inspection Item for anything that needs doing.</div>`
    : `<table><thead><tr>${["#", "What needs doing", "Where", "Category", "Priority", "Assigned To", "Due", "Status"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}</tr></thead><tbody>
      ${list.map(f => `<tr data-act="find-open" data-id="${f.id}"><td>${f.number}</td><td>${esc(f.what)}<div class="hint">Found ${fmtDate(f.date)} by ${esc(f.foundBy)}</div></td><td>${esc([f.area, f.equipmentId ? eqName(f.equipmentId) : ""].filter(Boolean).join(" · "))}</td><td>${esc(f.category)}</td><td>${tag(f.priority)}</td><td>${esc(f.assignedTo)}</td><td>${f.due ? fmtDate(f.due) + (f.due < today && f.status !== "Done" ? ' <span class="status Overdue">Overdue</span>' : "") : ""}</td><td>${tag(f.status)}</td></tr>`).join("")}
    </tbody></table>`}</div>`;
}

function findForm() {
  const m = state.maint, d = state.finds;
  showModal(`<h2>${icon("list")}Add Inspection Item</h2><div class="body" id="fnbody"><div class="fields">
    <label>What needs doing: <span class="req">*</span></label><textarea class="field" rows="3" data-fn="what" placeholder="e.g. Cove base peeling behind the dish machine"></textarea>
    <label>Area:</label><div><input class="field" data-fn="area" list="fnareas" placeholder="Where it is"><datalist id="fnareas">${(state.lib.locations || []).map(l => `<option value="${esc(l)}">`).join("")}</datalist></div>
    <label>Equipment:</label><select class="field" data-fn="equipmentId"><option value="">None / not equipment</option>${m.equipment.map(e => `<option value="${e.id}">${esc(e.name)}</option>`).join("")}</select>
    <label>Category:</label><select class="field" data-fn="category">${d.categories.map(x => `<option>${x}</option>`).join("")}</select>
    <label>Priority:</label><select class="field" data-fn="priority">${m.priorities.map(x => `<option ${x === "P3 - Routine" ? "selected" : ""}>${x}</option>`).join("")}</select>
    <label>Assigned to:</label><input class="field" data-fn="assignedTo" placeholder="Optional">
    <label>Due:</label><input type="date" class="field" data-fn="due" style="width:190px">
    <label>Your name: <span class="req">*</span></label><input class="field" data-fn="foundBy" value="${esc(who())}"></div></div>
    <div class="foot"><button class="btn green big" data-act="find-save">${icon("save")}Save Item</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

async function findDetail(id) {
  if (!state.finds) await loadFinds();
  const f = state.finds.findings.find(x => x.id === id); if (!f) return;
  showModal(`<h2>${icon("list")}#${f.number} ${esc(f.what.length > 60 ? f.what.slice(0, 60) + "..." : f.what)}</h2><div class="body" id="fdbody" data-id="${f.id}">
    <p>${esc(f.what).replace(/\n/g, "<br>")}</p>
    <p class="hint">${esc([f.area, f.equipmentId ? eqName(f.equipmentId) : "", f.category].filter(Boolean).join(" · "))}<br>Found ${fmtDate(f.date)} by ${esc(f.foundBy)}</p>
    <div class="fields"><label>Status:</label><select class="field" data-fd="status">${state.finds.statuses.map(x => `<option ${x === f.status ? "selected" : ""}>${x}</option>`).join("")}</select>
      <label>Priority:</label><select class="field" data-fd="priority">${state.maint.priorities.map(x => `<option ${x === f.priority ? "selected" : ""}>${x}</option>`).join("")}</select>
      <label>Assigned to:</label><input class="field" data-fd="assignedTo" value="${esc(f.assignedTo)}">
      <label>Due:</label><input type="date" class="field" data-fd="due" value="${esc(f.due)}" style="width:190px">
      <label>Work order:</label><input class="field" data-fd="workOrder" value="${esc(f.workOrder)}" placeholder="Number from MaintainX, if one was entered">
      <label>Add a note:</label><input class="field" data-fd="note" placeholder="Optional">
      <label>Your name: <span class="req">*</span></label><input class="field" data-fd="by" value="${esc(who())}"></div>
    <div class="hist">${[...f.history].reverse().map(h => `<div><b>${fmtDate(h.at)} ${esc(h.at.slice(11, 16))}</b> ${esc(h.by)}: ${esc(h.text)}</div>`).join("")}</div></div>
    <div class="foot"><button class="btn green big" data-act="find-update">${icon("save")}Save</button><button class="btn big" data-act="close">Close</button></div>`);
}

async function loadSummary(month) {
  month = month || state.sumMonth || ymd(new Date()).slice(0, 7);
  state.sumMonth = month;
  state.sum = await api("/api/maint/summary?month=" + month);
  if (state.view === "summary") render();
}

function vSummary() {
  if (!state.sum) { loadSummary().then(() => {}); return `<p class="hint">Loading...</p>`; }
  const s = state.sum, stat = (n, l, warn) => `<div class="stat ${warn ? "warn" : ""}"><div class="n">${n}</div><div>${l}</div></div>`;
  const tbl = (heads, rows, none) => rows.length ? `<table class="plain"><thead><tr>${heads.map(h => `<th class="nosort" style="width:auto">${h}</th>`).join("")}</tr></thead><tbody>${rows.map(r => `<tr>${r.map(c => `<td>${c}</td>`).join("")}</tr>`).join("")}</tbody></table>` : `<p class="hint">${none}</p>`;
  const box = (k, l, ph) => `<div><b>${l}</b><textarea class="field" rows="4" data-sn="${k}" placeholder="${ph}">${esc(s.notes[k] || "")}</textarea></div>`;
  return `<div class="row noprint"><label style="font-weight:600">Month:</label><input type="month" class="field" id="sumMonth" value="${s.month}" style="width:190px"><button class="btn" data-act="sum-print">${icon("print")}Print</button></div>
  <h2 style="margin:12px 0 4px">${esc(s.label)}</h2>
  <div class="stats">${stat(s.opened, "work orders opened")}${stat(s.completed, "completed")}${stat(s.openNow, "open now")}${stat(s.p1p2Open, "P1 / P2 open now", s.p1p2Open > 0)}${stat(s.overduePM, "PM jobs overdue now", s.overduePM > 0)}${stat(s.pmDone, "PM jobs completed")}${stat(s.pmDueThisMonth, "PM jobs due this month")}${stat(money(s.repairCost), "repair cost")}${stat(s.downtime + " hrs", "downtime")}</div>
  <p class="hint">Open counts are as of today. “Open” includes repairs waiting for a manager to verify. Repair cost includes parts ${money(s.partsCost)} and labor/service ${money(s.repairCost - s.partsCost)}. Preventive maintenance cost: ${money(s.preventiveCost)}.</p>
  <h3>Completed repairs by priority</h3>${tbl(["Priority", "Completed", "Cost"], s.byPriority.map(r => [esc(r.priority), r.count, money(r.cost)]), "")}
  <h3>Equipment with repeat problems <span class="hint" style="font-weight:400">(2 or more issues in the last 3 months)</span></h3>${tbl(["Equipment", "Issues"], s.repeat.map(r => [esc(r.equipment), r.issues]), "None.")}
  <h3>Largest repairs this month</h3>${tbl(["Date", "Equipment", "Repair", "Cost"], s.major.map(r => [fmtDate(r.date), esc(r.equipment), esc(r.what), money(r.cost)]), "No repair costs recorded.")}
  <h3>Management notes</h3><div class="cols">${box("highlights", "Highlights", "What went well")}${box("concerns", "Concerns", "What needs attention")}${box("actions", "Actions for next month", "Decisions and follow-up")}</div>
  <div class="row noprint" style="margin-top:8px"><label style="font-weight:600">Your name: <span class="req">*</span></label><input class="field" id="sumBy" value="${esc(who())}" style="width:220px"><button class="btn green big" data-act="sum-save">${icon("save")}Save Notes</button>${s.notes.at ? `<span class="hint">Saved ${fmtDate(s.notes.at)} by ${esc(s.notes.by)}</span>` : ""}</div>`;
}

Object.assign(ACT, {
  "asset-open"(el) { assetDetail(el.dataset.id); },
  async "svc-save"() { const v = grab("[data-sv]", "sv"), id = $("#asbody").dataset.id; v.equipmentId = id; v.cost = parseFloat(v.cost) || 0; rememberWho(v.by);
    state.maint = await api("/api/maint/service", {method: "POST", body: JSON.stringify(v)}); render(); assetDetail(id); toast("Recorded."); },
  "svc-edit"(el, e) { e && e.preventDefault(); const r = (state.maint.service || []).find(x => x.id === el.dataset.id); if (!r) return; const eq = r.equipmentId;
    showModal(`<h2>${icon("edit")}Edit Maintenance Record</h2><div class="body" id="svbody" data-id="${esc(r.id)}" data-eq="${esc(eq)}"><div class="fields">
      <label>Date:</label><input type="date" class="field" data-se="date" value="${esc(r.date)}" style="width:190px">
      <label>Type:</label><select class="field" data-se="type" style="width:190px">${["Repair", "Preventive", "Inspection", "Other"].map(t => `<option ${t === r.type ? "selected" : ""}>${t}</option>`).join("")}</select>
      <label>What was done:</label><input class="field" data-se="description" value="${esc(r.description)}">
      <label>Cost ($):</label><input type="number" min="0" step="0.01" class="field" data-se="cost" style="width:150px" value="${r.cost || ""}">
      <label>Done by (company):</label><input class="field" data-se="vendor" value="${esc(r.vendor || "")}">
      <label>Recorded by: <span class="req">*</span></label><input class="field" data-se="by" value="${esc(who() || r.by)}"></div>
      ${r.issueId ? `<p class="hint">This record came from a maintenance issue; changing the cost on the issue will overwrite what is here.</p>` : ""}</div>
    <div class="foot"><button class="btn green big" data-act="svc-edit-go">${icon("save")}Save Changes</button><button class="btn big" data-act="close">Cancel</button></div>`); },
  async "svc-edit-go"() { const v = grab("[data-se]", "se"), b = $("#svbody"); v.cost = parseFloat(v.cost) || 0; rememberWho(v.by);
    state.maint = await api("/api/maint/service/" + b.dataset.id, {method: "PUT", body: JSON.stringify(v)}); closeModal(); render(); assetDetail(b.dataset.eq); toast("Saved."); },
  async "svc-delete"(el, e) { e && e.preventDefault(); if (!confirm("Remove this maintenance record?")) return; const id = $("#asbody").dataset.id; state.maint = await api("/api/maint/service/" + el.dataset.id, {method: "DELETE"}); render(); assetDetail(id); },
  async "repct-save"() { state.maint = await api("/api/maint/settings", {method: "POST", body: JSON.stringify({replacePercent: parseInt($("#repct").value, 10) || 0})}); render(); toast("Saved."); },
  "pm-edit"(el) { pmForm(el.dataset.id, el.dataset.group); },
  async "pm-save"() { const v = grab("[data-pm]", "pm"); v.id = $("#pmbody").dataset.id; v.every = parseInt(v.every, 10) || 0; await maintDone(await api("/api/maint/pm", {method: "POST", body: JSON.stringify(v)}), "Job saved."); },
  async "pm-delete"(el) { await maintDone(await api("/api/maint/pm/" + el.dataset.id, {method: "DELETE"}), "Job deleted."); },
  "pm-done-ask"(el) { pmDone(el.dataset.id); },
  async "pm-done-go"() { const v = grab("[data-pd]", "pd"); v.cost = parseFloat(v.cost) || 0; rememberWho(v.by); await maintDone(await api("/api/maint/pm/" + $("#pdbody").dataset.id + "/done", {method: "POST", body: JSON.stringify(v)}), "Recorded. The next due date has been updated."); },
  "issues-all"(el) { state.showAll = el.checked; render(); },
  "eq-edit"(el) { eqForm(el.dataset.id); },
  async "eq-save"() { const v = grab("[data-eq]", "eq"); v.id = $("#eqbody").dataset.id; for (const k of ["purchaseCost", "replaceCost"]) v[k] = parseFloat(v[k]) || 0; v.expectedYears = parseInt(v.expectedYears, 10) || 0; await maintDone(await api("/api/maint/equipment", {method: "POST", body: JSON.stringify(v)}), "Equipment saved."); },
  async "eq-delete"(el) { await maintDone(await api("/api/maint/equipment/" + el.dataset.id, {method: "DELETE"}), "Equipment deleted."); },
  "issue-new"(el) { issueForm(el.dataset.id); },
  async "issue-save"() { const v = grab("[data-is]", "is"); rememberWho(v.reportedBy); await maintDone(await api("/api/maint/issues", {method: "POST", body: JSON.stringify(v)}), "Issue saved. Use Submit Work Order to enter it in MaintainX."); if (v.outOfService) tagDetails(v.equipmentId); },
  "issue-open"(el) { issueDetail(el.dataset.id); },
  async "issue-update"() { const v = grab("[data-id-f]", "idF"); for (const k of ["parts", "labor", "downtime"]) v[k] = parseFloat(v[k]) || 0; rememberWho(v.by); await maintDone(await api("/api/maint/issues/" + $("#idbody").dataset.id, {method: "PUT", body: JSON.stringify(v)}), "Issue updated."); },
  "repl-new"(el) { closeModal(); replForm("", el.dataset.eq); },
  "repl-open"(el) { replForm(el.dataset.id); },
  "repl-print"(el) { repPrint("replace:" + el.dataset.id); },
  "repl-addrow"() { replRow(); },
  "repl-delrow"(el) { el.closest("tr").remove(); },
  async "repl-save"() { const v = grab("[data-rp]", "rp"); v.id = $("#rpbody").dataset.id; ["originalCost", "replaceCost", "annualRepair", "annualDowntime"].forEach(k => v[k] = v[k] === "" ? 0 : parseFloat(v[k]));
    v.history = [...document.querySelectorAll("#rphist tbody tr")].map(tr => { const o = {}; tr.querySelectorAll("[data-rh]").forEach(el => { o[el.dataset.rh] = el.value; }); ["downtime", "repairCost", "partsCost"].forEach(k => o[k] = o[k] === "" ? 0 : parseFloat(o[k])); return o; });
    rememberWho(v.requestedBy);
    try { const o = await api("/api/replacements", {method: "POST", body: JSON.stringify(v)}); state.repl = o; closeModal(); if (state.view === "todo") await refreshTodo(); render();
      showModal(`<h2>${icon("save")}${esc(o.saved)} Saved</h2><div class="body"><p>The request is in Replacement Requests. Print it for the decision meeting?</p></div><div class="foot"><button class="btn green big" data-act="repl-print" data-id="${esc(o.saved)}">${icon("print")}Print Now</button><button class="btn big" data-act="close">Print Later</button></div>`); } catch (e) { toast(e.message, true); } },
  "insp-new"(el) { inspForm(el.dataset.task); },
  async "insp-save"() { const v = grab("[data-in]", "in"); rememberWho(v.by); v.results = {};
    for (const it of state.insp.items) v.results[it.key] = {result: (document.querySelector(`[name=in_${it.key}]:checked`) || {}).value || "Pass", note: (document.querySelector(`[data-in-note=${it.key}]`) || {}).value || ""};
    const o = await api("/api/inspections", {method: "POST", body: JSON.stringify(v)}); state.maint = o; state.insp = null; state.sum = null; closeModal(); if (state.view === "cal") await loadCal(true); if (state.view === "todo") await refreshTodo(); render();
    toast(`Inspection #${o.inspection.number} saved.${o.inspection.issueId ? " A maintenance issue was created." : ""}${o.inspection.pmDone ? " The PM job is marked done." : ""}`);
    if (o.inspection.condition === "Unsafe") { const e = o.equipment.find(x => x.id === o.inspection.equipmentId); if (e && e.status !== "Out of Service") showModal(`<h2>${icon("warn")}Condition is Unsafe</h2><div class="body"><p>Should <b>${esc(e.name)}</b> be taken out of service now?</p></div><div class="foot"><button class="btn wine big" data-act="oos-ask" data-id="${e.id}">Take Out of Service</button><button class="btn big" data-act="close">Not Now</button></div>`); } },
  async "insp-edit"(el) { if (!state.insp) await loadInsp(); inspEdit(el.dataset.id); },
  async "insp-edit-go"() { const v = grab("[data-ie]", "ie"), id = $("#iebody").dataset.id; rememberWho(v.editedBy); v.results = {};
    for (const it of state.insp.items) v.results[it.key] = {result: (document.querySelector(`[name=ie_${it.key}]:checked`) || {}).value || "Pass", note: (document.querySelector(`[data-ie-note=${it.key}]`) || {}).value || ""};
    await api("/api/inspections/" + id, {method: "PUT", body: JSON.stringify(v)}); state.insp = null; closeModal(); render(); toast("Inspection updated."); },
  "finds-all"(el) { state.findAll = el.checked; render(); },
  "find-new"() { findForm(); },
  "find-open"(el) { findDetail(el.dataset.id); },
  async "find-save"() { const v = grab("[data-fn]", "fn"); rememberWho(v.foundBy); state.finds = await api("/api/maint/findings", {method: "POST", body: JSON.stringify(v)}); closeModal(); render(); toast("Item added. It is on the To-Do List until it is done."); },
  async "find-update"() { const v = grab("[data-fd]", "fd"); rememberWho(v.by); state.finds = await api("/api/maint/findings/" + $("#fdbody").dataset.id, {method: "PUT", body: JSON.stringify(v)}); closeModal(); if (state.view === "todo") await refreshTodo(); render(); toast("Saved."); },
  "sum-print"() { window.print(); },
  async "sum-save"() { const by = $("#sumBy").value.trim(); if (!by) return toast("Enter your name.", true); rememberWho(by);
    const v = grab("[data-sn]", "sn"); state.sum = await api("/api/maint/summary-notes", {method: "POST", body: JSON.stringify({...v, month: state.sumMonth, by})}); render(); toast("Notes saved."); },
  async "issue-wo"(el) { const p = state.maint.issues.find(x => x.id === el.dataset.id), eq = (state.maint.equipment || []).find(x => x.id === p.equipmentId) || {};
    const text = `${eq.name || ""}${eq.area ? " (" + eq.area + ")" : ""}${eq.make || eq.model ? " - " + [eq.make, eq.model].filter(Boolean).join(" ") : ""}\n${p.title}\nPriority: ${p.priority}\n${p.description || ""}\nReported by ${p.reportedBy} on ${fmtDate(p.reportedAt)}`.trim();
    try { await navigator.clipboard.writeText(text); toast("Issue details copied. Paste them into the new work order in MaintainX."); } catch (e) { toast("Opening MaintainX. The details could not be copied.", true); }
    await api("/api/maint/open-portal", {method: "POST"}); },
  async "submit-wo"() { await api("/api/maint/open-portal", {method: "POST"}); toast("Opening MaintainX."); },
});
