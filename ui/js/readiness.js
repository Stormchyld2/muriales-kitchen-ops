"use strict";

/* Inspection readiness */

async function loadReady() { state.ready = await api("/api/readiness"); if (state.view === "ready") render(); }

function vReady() {
  const d = state.ready; if (!d) { loadReady(); return `<p class="hint">Loading...</p>`; }
  return `<div class="row"><button class="btn green big" data-act="rep-print" data-id="inspection-ready">${icon("print")}Inspection-Ready Checklist</button><button class="btn" data-act="rep-email-id" data-id="inspection-ready">${icon("ext")}Email It</button><button class="btn" data-act="rep-print" data-id="weekly-summary">${icon("print")}Weekly Summary</button><button class="btn" data-act="rep-email-id" data-id="weekly-summary">${icon("ext")}Email Weekly Summary</button></div>
  <p class="hint" style="margin:6px 0 10px"><b>Inspection-Ready Checklist</b> checks your records right now (temperatures, cleaning, maintenance, corrective actions, SDS) and adds the walk-through points to check by eye. <b>Weekly Summary</b> is the last 7 days at a glance.</p>
  <div class="row"><button class="btn green big" data-act="rd-new">${icon("plus")}New Readiness Review</button><button class="btn" data-act="rep-print" data-id="readiness-blank">${icon("print")}Print Blank Checklist</button><button class="btn" data-act="rep-print" data-id="readiness-log">${icon("print")}Print History</button></div>
  <p class="hint" style="margin-top:0">A manager self-inspection to run before a health department or internal food-safety inspection. Items that need attention can become Corrective Action Forms.</p>
  <div class="tablewrap">${!d.reviews.length ? `<div class="empty"><b>No reviews yet</b>Use New Readiness Review to walk the kitchen with the 30-point checklist.</div>` : `<table class="plain"><thead><tr>${["ID", "Date", "Manager", "Type", "Ready?", "Critical?", "Open actions", "Recheck", "Outcome"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}<th class="nosort"></th></tr></thead><tbody>
    ${d.reviews.map(r => `<tr><td><b>${esc(r.id)}</b></td><td>${esc(r.date)}</td><td>${esc(r.manager)}</td><td>${esc(r.type)}</td><td>${esc(r.overall)}</td><td>${esc(r.critical)}</td><td>${r.open}</td><td>${esc(r.recheck)}</td><td>${esc(r.outcome)}</td>
      <td style="text-align:right"><div class="acts"><button class="btn small" data-act="rd-open" data-id="${esc(r.id)}">${icon("edit")}Edit</button> <button class="btn small" data-act="rep-print" data-id="readiness:${esc(r.id)}">${icon("print")}Print</button></div></td></tr>`).join("")}</tbody></table>`}</div>`;
}

async function readyForm(id) {
  if (!state.ready) await loadReady();
  const d = state.ready, r = id ? d.reviews.find(x => x.id === id) : null, now = new Date(), pad = n => String(n).padStart(2, "0");
  const h = r || {date: ymd(now), manager: who(), type: d.types[0], walkTime: `${pad(now.getHours())}:${pad(now.getMinutes())}`, overall: "", critical: "", photos: "", items: []};
  const sel = (k, o, v, attr) => `<select class="field" ${attr} style="width:auto;max-width:100%">${o.map(x => `<option ${x === (v || "") ? "selected" : ""}>${x}</option>`).join("")}</select>`;
  const hf = (k, l, x) => `<label>${l}</label><input class="field" data-rh="${k}" value="${esc(h[k] || "")}" ${x || ""}>`, hs = (k, l, o) => `<label>${l}</label>${sel(k, o, h[k], `data-rh="${k}"`)}`;
  const open = r ? r.items.filter(i => i.status === "Needs Attention" && !i.caRef).length : 0;
  showModal(`<h2>${icon("list")}${id ? esc(id) + " " : ""}Inspection Readiness Review</h2><div class="body" id="rdbody" data-id="${esc(id || "")}"><div class="fields">
    ${hf("date", "Date: <span class=\"req\">*</span>", 'type="date" style="width:170px"')}${hf("manager", 'Manager: <span class="req">*</span>')}${hs("type", "Inspection type:", d.types)}${hf("walkTime", "Walk time:", 'style="width:120px"')}
    ${hs("overall", 'Overall ready? <span class="req">*</span>', ["", "Yes", "No"])}${hs("critical", 'Critical finding? <span class="req">*</span>', ["", "Yes", "No"])}${hs("photos", "Photos taken?", ["", "Yes", "No"])}${hf("recheck", "Recheck date:", 'type="date" style="width:170px"')}${hf("reviewedBy", "Reviewed by:")}${hf("outcome", "Inspection / visit outcome:", 'placeholder="Fill in afterwards, e.g. Passed"')}</div>
    <table class="plain" style="margin:10px 0;font-size:12.5px"><thead><tr>${["#  Area / point", "Status", "Finding", "Fixed now?", "Priority", "Owner", "Target"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}</tr></thead><tbody>
    ${d.points.map((p, i) => { const it = (r && r.items[i]) || {status: "Ready"}; return `<tr><td><b>${p.n}. ${esc(p.area)}</b><div class="hint">${esc(p.point)}</div></td>
      <td>${sel("status", d.statuses, it.status, `data-ri="${i}" data-rf="status"`)}${it.caRef ? `<div class="hint">${esc(it.caRef)}</div>` : ""}</td><td><input class="field" data-ri="${i}" data-rf="finding" value="${esc(it.finding || "")}" placeholder="If it needs attention"></td>
      <td>${sel("corrected", ["", "Yes", "No", "N/A"], it.corrected, `data-ri="${i}" data-rf="corrected"`)}</td><td>${sel("priority", ["", ...d.priorities], it.priority, `data-ri="${i}" data-rf="priority"`)}</td><td>${sel("owner", ["", ...d.owners], it.owner, `data-ri="${i}" data-rf="owner"`)}</td><td><input type="date" class="field" data-ri="${i}" data-rf="target" value="${esc(it.target || "")}"><input type="hidden" data-ri="${i}" data-rf="caRef" value="${esc(it.caRef || "")}"></td></tr>`; }).join("")}</tbody></table>
    <div class="fields"><label>Notes:</label><textarea class="field" rows="2" data-rh="notes">${esc(h.notes || "")}</textarea></div></div>
    <div class="foot"><button class="btn green big" data-act="rd-save">${icon("save")}Save Review</button>${open ? `<button class="btn big" data-act="rd-ca" data-id="${esc(id)}" data-n="0">${icon("warn")}Create Corrective Actions (${open})</button>` : ""}${id ? `<button class="btn big" data-act="rep-print" data-id="readiness:${esc(id)}">${icon("print")}Print</button>` : ""}<button class="btn big" data-act="close">Cancel</button></div>`);
}

Object.assign(ACT, {
  "rd-new"() { readyForm(""); },
  "rd-open"(el) { readyForm(el.dataset.id); },
  async "rd-save"() { const id = $("#rdbody").dataset.id, v = grab("[data-rh]", "rh"); v.id = id; v.items = [];
    for (let i = 0; i < state.ready.points.length; i++) { const it = {}; document.querySelectorAll(`[data-ri="${i}"]`).forEach(el => { it[el.dataset.rf] = el.value; }); v.items.push(it); }
    rememberWho(v.manager);
    try { const o = await api("/api/readiness", {method: "POST", body: JSON.stringify(v)}); state.ready = o; const r = o.reviews.find(x => x.id === o.saved), need = r.items.filter(i => i.status === "Needs Attention" && !i.caRef).length; closeModal(); render();
      showModal(`<h2>${icon("save")}${esc(o.saved)} Saved</h2><div class="body"><p>${r.open ? `<b>${r.open} item${r.open === 1 ? " needs" : "s need"} attention.</b>` : "Nothing needs attention."}</p>${need ? `<p>Make a Corrective Action Form for ${need === 1 ? "it" : "each one"}? They will be filled in from the review and added to the Corrective Action Log and To-Do list.</p>` : ""}</div>
        <div class="foot">${need ? `<button class="btn green big" data-act="rd-ca" data-id="${esc(o.saved)}" data-n="0">${icon("warn")}Create Corrective Actions</button>` : ""}<button class="btn big" data-act="rep-print" data-id="readiness:${esc(o.saved)}">${icon("print")}Print</button><button class="btn big" data-act="close">Done</button></div>`); } catch (e) { toast(e.message, true); } },
  async "rd-ca"(el) { try { const o = await api("/api/readiness/" + el.dataset.id + "/corrective", {method: "POST", body: JSON.stringify({n: +el.dataset.n})}); state.ready = o; state.ca = null; closeModal(); render(); toast(`${o.made} corrective action form${o.made === 1 ? "" : "s"} created. See Corrective Actions.`); } catch (e) { toast(e.message, true); } },
});
