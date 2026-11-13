"use strict";

/* Generic lists (registry) */

function vReg(name) {
  const d = state.reg[name]; if (!d) { api("/api/reg/" + name).then(o => { state.reg[name] = o; render(); }); return `<p class="hint">Loading...</p>`; }
  const def = d.def, lab = k => (def.fields.find(f => f.key === k) || {label: ({id: "ID", due: "Next due", state: "Status", employeeName: "Employee", typeName: "Training"})[k] || k}).label, ftype = k => (def.fields.find(f => f.key === k) || {}).type;
  const extra = r => name === "incidents" && r.caNeeded === "Yes" && !r.caRef ? `<button class="btn small" data-act="inc-ca" data-id="${esc(r.id)}">Corrective Action</button> ` : name === "newhire" ? `<button class="btn small green" data-act="nh-open" data-id="${esc(r.id)}">Open Checklist</button> ` : name === "lotoprocs" ? `<button class="btn small" data-act="rep-view" data-id="rec:lotoprocs:${esc(r.id)}">Print</button> <button class="btn small" data-act="rep-view" data-id="lototag:${esc(r.id)}">Tag</button> ` : name === "lotolog" ? `<button class="btn small" data-act="rep-view" data-id="lototag:${esc(r.id)}">Tag</button> ${!r.removed ? `<button class="btn small green" data-act="loto-off" data-id="${esc(r.id)}">Lock Removed</button> ` : ""}` : name === "employees" ? `<button class="btn small green" data-act="train-for" data-id="${esc(r.id)}">Record Training</button> ` : name === "cleantasks" ? `<button class="btn small green" data-act="cl-done" data-id="${esc(r.id)}">Done</button> ` : name === "deepclean" ? `<button class="btn small green" data-act="deep-done" data-id="${esc(r.id)}">Done</button> ` : name === "mgrchecks" ? `<button class="btn small green" data-act="mgr-done" data-id="${esc(r.id)}">Done</button> ` : name === "mgrlog" && r.caNeeded === "Yes" && !r.followUp ? `<button class="btn small" data-act="mgr-ca" data-id="${esc(r.id)}">Corrective Action</button> ` : "";
  const clip = (v, n) => (v || "").length > n ? v.slice(0, n) + "..." : v || "";
  return `<div class="row"><button class="btn green big" data-act="reg-new" data-c="${name}">${icon("plus")}${esc(def.newLabel)}</button><button class="btn" data-act="reg-print" data-id="${esc(def.report)}">${icon("print")}Print</button><span class="hint">${d.rows.length} on record</span></div>
  ${def.intro ? `<p class="hint" style="margin-top:0">${esc(def.intro)}</p>` : ""}
  <div class="tablewrap">${!d.rows.length ? `<div class="empty"><b>Nothing here yet</b></div>` : `<table class="plain"><thead><tr>${def.listCols.map(k => `<th class="nosort" style="width:auto">${esc(lab(k))}</th>`).join("")}<th class="nosort"></th></tr></thead><tbody>
    ${d.rows.map(r => `<tr>${def.listCols.map(k => `<td>${k === "id" ? `<b>${esc(r[k])}</b>` : k === "state" ? tag(r[k]) : k === "due" ? esc(shortDate(r[k] || "")) + (r.lastDone ? `<div class="hint">last ${esc(shortDate(r.lastDone))}</div>` : "") : ftype(k) === "date" ? esc(shortDate(r[k] || "")) : esc(clip(r[k], 90))}</td>`).join("")}<td style="text-align:right"><div class="acts">${extra(r)}${name === "cleanlog" && r.verifyStatus === "Pending" ? `<button class="btn small" data-act="cl-verify" data-id="${esc(r.id)}">Verify</button> ` : ""}<button class="btn small" data-act="reg-open" data-c="${name}" data-id="${esc(r.id)}">${icon("edit")}Edit</button>${def.deletable ? ` <button class="btn small wine" data-act="reg-del" data-c="${name}" data-id="${esc(r.id)}">${icon("trash")}</button>` : ""}</div></td></tr>`).join("")}</tbody></table>`}</div>`;
}

function regForm(name, id, preset) {
  if (!state.reg[name]) { api("/api/reg/" + name).then(o => { state.reg[name] = o; regForm(name, id, preset); }); return; }
  const d = state.reg[name], def = d.def, r = id ? d.rows.find(x => x.id === id) || {} : {}; preset = preset || {};
  const need = def.fields.filter(f => (f.type === "ref" && !state.reg[f.src]) || (f.type === "station" && !state.reg.stations)).map(f => f.src || "stations");
  if (need.length) { Promise.all(need.map(n => api("/api/reg/" + n).then(o => { state.reg[n] = o; }))).then(() => regForm(name, id, preset)); return; }
  const defaults = {date: ymd(new Date()), active: "Yes", status: def.fields.find(f => f.key === "status")?.opts?.[0], priority: "P3 - Routine", verifyStatus: "Pending", completedBy: who(), by: who(), assignedTo: ""};
  const val = f => id ? (r[f.key] || "") : f.key in preset ? preset[f.key] : (f.key in defaults && ["date", "active", "status", "priority", "completedBy", "by"].includes(f.key) ? defaults[f.key] || "" : "");
  const input = f => { const v = val(f), a = `data-rg="${f.key}"`;
    return f.type === "doc" ? `<textarea class="field" rows="22" style="font-family:Consolas,monospace;font-size:13px" ${a}>${esc(v)}</textarea>` : f.type === "station" ? `<select class="field" ${a}><option value="">Choose...</option>${((state.reg.stations || {}).rows || []).map(o => o.name).filter((n, i, a2) => a2.indexOf(n) === i).map(n => `<option ${n === v ? "selected" : ""}>${esc(n)}</option>`).join("")}</select>` : f.type === "long" ? `<textarea class="field" rows="3" ${a}>${esc(v)}</textarea>` : f.type === "date" ? `<input type="date" class="field" ${a} value="${esc(v)}" style="width:190px">`
      : f.type === "select" ? `<select class="field" ${a}>${(f.req ? f.opts : ["", ...f.opts]).map(o => `<option ${o === v ? "selected" : ""}>${esc(o)}</option>`).join("")}</select>`
      : f.type === "ref" ? `<select class="field" ${a}><option value="">Choose...</option>${((state.reg[f.src] || {}).rows || []).filter(o => (o.status || "Active") !== "Inactive" && o.active !== "No" || o.id === v).map(o => `<option value="${esc(o.id)}" ${o.id === v ? "selected" : ""}>${esc(o.name)}</option>`).join("")}</select>`
      : f.type === "yn" ? `<select class="field" ${a} style="width:120px">${["", "Yes", "No"].map(o => `<option ${o === v ? "selected" : ""}>${o}</option>`).join("")}</select>` : `<input class="field" ${a} value="${esc(v)}">`; };
  showModal(`<h2>${icon("list")}${id ? esc(id) : esc(def.newLabel)}</h2><div class="body" id="rgbody" data-c="${name}" data-id="${esc(id || "")}"><div class="fields">${def.fields.filter(f => f.type !== "hidden").map(f => `<label>${esc(f.label)}:${f.req ? ' <span class="req">*</span>' : ""}</label>${input(f)}`).join("")}</div></div>
    <div class="foot"><button class="btn green big" data-act="reg-save">${icon("save")}Save</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

Object.assign(ACT, {
  "reg-new"(el) { regForm(el.dataset.c, ""); },
  "reg-open"(el) { regForm(el.dataset.c, el.dataset.id); },
  async "reg-del"(el) { if (!confirm("Delete this record?")) return; state.reg[el.dataset.c] = await api("/api/reg/" + el.dataset.c + "/" + encodeURIComponent(el.dataset.id), {method: "DELETE"}); render(); },
  async "reg-save"() { const c = $("#rgbody").dataset.c, v = grab("[data-rg]", "rg"); v.id = $("#rgbody").dataset.id;
    try { state.reg[c] = await api("/api/reg/" + c, {method: "POST", body: JSON.stringify(v)}); state.today = null; state.trm = null; closeModal(); render(); toast("Saved."); } catch (e) { toast(e.message, true); } },
  "reg-print"(el) { repPrint(el.dataset.id); },
});
