"use strict";

/* Chemical safety */

async function loadChem() { state.chem = await api("/api/chem"); if (state.view === "chemresp" || state.view === "chemlog") render(); }

function vChemResp() {
  const d = state.chem; if (!d) { loadChem(); return `<p class="hint">Loading...</p>`; }
  const q = state.reg.chemquick; if (!q) { api("/api/reg/chemquick").then(o => { state.reg.chemquick = o; render(); }); return `<p class="hint">Loading...</p>`; }
  const f = state.chemCat || "all", cats = [...new Set(q.rows.map(r => r.category))], rows = q.rows.filter(r => r.active !== "No" && (f === "all" || r.category === f));
  const chips = ["all", ...cats].map(c => `<button class="chip ${f === c ? "on" : ""}" data-act="chem-cat" data-v="${esc(c)}">${c === "all" ? "All" : esc(c)}</button>`).join("");
  const cell = (k, r) => esc(r[k] || "");
  return `<div class="row"><button class="btn wine big" data-act="chem-new">${icon("plus")}Report an Incident</button><button class="btn" data-act="rep-print" data-id="chem-response">${icon("print")}Print Response Sheet</button><button class="btn" data-act="rep-print" data-id="chem-blank">${icon("print")}Print Blank Incident Form</button></div>
  <p class="note warn">${esc(d.note)}</p>
  <h3>Immediate Response</h3>${d.steps.map(s => `<div class="panel" style="margin:6px 0"><b>${esc(s.N)}. ${esc(s.Title)}</b> &nbsp;${esc(s.Text)}</div>`).join("")}
  <div class="row" style="margin-top:14px"><h3 style="margin:0">Exposure Quick Reference</h3><button class="btn small green" data-act="reg-new" data-c="chemquick">${icon("plus")}Add Row</button><span class="hint">${rows.length} situations. Open any row to edit it.</span></div>
  <div class="chips" style="margin:8px 0">${chips}</div>
  <div class="tablewrap" style="min-height:0;flex:none;overflow:visible"><table class="plain"><thead><tr>${["Situation", "Immediate action", "Do not", "PPE", "Call emergency help when", "Manager action"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}<th class="nosort"></th></tr></thead><tbody>
    ${rows.map(r => `<tr><td><b>${cell("situation", r)}</b><div class="hint">${cell("category", r)}</div></td><td>${cell("immediate", r)}</td><td>${cell("doNot", r)}</td><td>${cell("ppe", r)}</td><td>${cell("escalate", r)}</td><td>${cell("manager", r)}</td>
      <td style="text-align:right"><div class="acts"><button class="btn small" data-act="reg-open" data-c="chemquick" data-id="${esc(r.id)}">${icon("edit")}Edit</button></div></td></tr>`).join("")}</tbody></table></div>
  <p class="hint">Each product's own first aid steps are in the SDS Manager. Choosing the product when you report an incident shows them.</p>`;
}

function vChemLog() {
  const d = state.chem; if (!d) { loadChem(); return `<p class="hint">Loading...</p>`; }
  return `<div class="row"><button class="btn wine big" data-act="chem-new">${icon("plus")}Report an Incident</button><button class="btn" data-act="rep-print" data-id="chem-log">${icon("print")}Print Log</button><span class="hint">${d.incidents.length} on record</span></div>
  <div class="tablewrap">${!d.incidents.length ? `<div class="empty"><b>No incidents recorded</b>Use Report an Incident for any chemical spill or exposure.</div>` : `<table class="plain"><thead><tr>${["ID", "When", "Product", "What happened", "Person(s)", "Emergency help?", "CA ref", "Training follow-up?"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}<th class="nosort"></th></tr></thead><tbody>
    ${d.incidents.map(c => `<tr><td><b>${esc(c.id)}</b></td><td>${esc(when12(c.at))}</td><td>${esc(c.product)}</td><td>${esc(c.kind)}</td><td>${esc(c.persons)}</td><td>${esc(c.emergency)}</td><td>${esc(c.caRef)}</td><td>${esc(c.training)}</td>
      <td style="text-align:right"><div class="acts"><button class="btn small" data-act="chem-open" data-id="${esc(c.id)}">${icon("edit")}Edit</button> <button class="btn small" data-act="chem-print" data-id="${esc(c.id)}">${icon("print")}Print</button> <button class="btn small" data-act="chem-ca" data-id="${esc(c.id)}">Corrective Action</button></div></td></tr>`).join("")}</tbody></table>`}</div>`;
}

async function chemForm(id) {
  if (!state.chem) await loadChem();
  const d = state.chem, now = new Date(), pad = n => String(n).padStart(2, "0"), c = id ? d.incidents.find(x => x.id === id) || {} : {at: `${ymd(now)} ${pad(now.getHours())}:${pad(now.getMinutes())}`, by: who(), emergency: "", mgmtNotified: "", training: ""};
  const t = (k, l, x) => `<label>${l}</label><input class="field" data-ch="${k}" value="${esc(c[k] || "")}" ${x || ""}>`, a = (k, l, r) => `<label>${l}</label><textarea class="field" rows="${r || 2}" data-ch="${k}">${esc(c[k] || "")}</textarea>`;
  const sel = (k, l, o) => `<label>${l}</label><select class="field" data-ch="${k}">${o.map(v => `<option ${v === (c[k] || "") ? "selected" : ""}>${v}</option>`).join("")}</select>`;
  showModal(`<h2>${icon("warn")}${id ? esc(id) : "Report"} Chemical Incident</h2><div class="body" id="chbody" data-id="${esc(id || "")}"><p class="hint" style="margin-top:0">For a serious exposure or an uncontrolled spill, deal with the emergency first and fill this in afterwards.</p><div class="fields">
    <label>Date and time: <span class="req">*</span></label><input type="datetime-local" class="field" data-ch="at" value="${esc((c.at || "").replace(" ", "T"))}" style="width:230px">
    ${sel("kind", "What happened: <span class=\"req\">*</span>", d.kinds)}
    <label>Product / chemical: <span class="req">*</span></label><input class="field" data-ch="product" list="chprods" value="${esc(c.product || "")}" placeholder="Start typing to pick from the SDS library"><datalist id="chprods">${d.products.map(p => `<option value="${esc(p.Name)}">`).join("")}</datalist>
    <label></label><div id="chinfo"></div>
    ${t("manufacturer", "Manufacturer:")}${t("sdsLocation", "SDS location:")}${t("amount", "Amount / concentration:")}${t("area", 'Area: <span class="req">*</span>')}${t("persons", "Person(s) involved:")}${t("ppe", "PPE used:")}${a("firstAid", "First aid given:")}
    ${sel("emergency", 'Emergency services called? <span class="req">*</span>', ["", "Yes", "No"])}${t("medical", "Poison Control / medical contact:")}${sel("mgmtNotified", 'Management notified? <span class="req">*</span>', ["", "Yes", "No"])}
    ${t("reopenedBy", "Area reopened by:")}${t("reopenedAt", "Time reopened:")}${t("caRef", "Corrective action ref:")}${sel("training", "Training follow-up?", ["", "Yes", "No"])}
    ${a("summary", 'Incident summary: <span class="req">*</span>', 3)}${a("notes", "Notes:")}${t("by", 'Recorded by: <span class="req">*</span>')}</div></div>
    <div class="foot"><button class="btn green big" data-act="chem-save">${icon("save")}Save Incident</button><button class="btn big" data-act="close">Cancel</button></div>`);
  chemPick(true);
}

function chemPick(quiet) {
  const name = ($('[data-ch="product"]') || {}).value || "", p = state.chem.products.find(x => x.Name.toLowerCase() === name.trim().toLowerCase()), box = $("#chinfo"); if (!box) return;
  if (!p) { box.innerHTML = name ? `<span class="hint">Not in the SDS library. Find the product's SDS before treating anyone.</span>` : ""; return; }
  const set = (k, v) => { const el = $(`[data-ch="${k}"]`); if (el && !el.value && v) el.value = v; };
  if (!quiet) { set("manufacturer", p.Manufacturer); set("sdsLocation", p.Location); }
  const kind = ($('[data-ch="kind"]') || {}).value, fa = {"Eye Contact": p.Eyes, "Skin Contact": p.Skin, "Inhalation": p.Inhalation, "Ingestion": p.Swallowed}[kind];
  box.innerHTML = `<div class="box" style="margin:0">${p.Signal ? `<b>${esc(p.Signal)}</b> · ` : ""}${fa ? `<b>${esc(kind)}:</b> ${esc(fa)}` : kind === "Spill Only" ? "" : `<span class="hint">No ${esc(kind.toLowerCase())} first aid is saved for this product. Use the SDS.</span>`}${p.PPE ? `<div><b>PPE:</b> ${esc(p.PPE)}</div>` : ""}<div class="hint">From the SDS library. Always confirm against the product's own SDS.</div></div>`;
}

Object.assign(ACT, {
  "chem-cat"(el) { state.chemCat = el.dataset.v; render(); },
  "chem-new"() { chemForm(""); },
  "chem-open"(el) { chemForm(el.dataset.id); },
  "chem-print"(el) { repPrint("chem:" + el.dataset.id); },
  "chem-ca"(el) { const c = state.chem.incidents.find(x => x.id === el.dataset.id); if (!c) return;
    caForm("", {area: "Chemical / Janitorial", priority: "P2 - Urgent", issue: `${c.kind}: ${c.product} in ${c.area} (${c.id}).${c.summary ? " " + c.summary : ""}`, immediate: c.firstAid || "", related: "Chemical Incident Log " + c.id}); },
  async "chem-save"() { const v = grab("[data-ch]", "ch"); v.id = $("#chbody").dataset.id; rememberWho(v.by);
    const pr = state.chem.products.find(p => p.Name.toLowerCase() === (v.product || "").trim().toLowerCase()); v.productId = pr ? pr.ID : "";
    try { const o = await api("/api/chem", {method: "POST", body: JSON.stringify(v)}); state.chem = o; closeModal(); render();
      showModal(`<h2>${icon("save")}${esc(o.saved.id)} Saved</h2><div class="body"><p>The incident is in the Chemical Incident Log.</p>${o.saved.training === "Yes" ? `<p class="hint">Training follow-up was marked Yes. Use Corrective Action Form on the log row to track it.</p>` : ""}</div>
        <div class="foot"><button class="btn green big" data-act="chem-print" data-id="${esc(o.saved.id)}">${icon("print")}Print Now</button><button class="btn big" data-act="close">Print Later</button></div>`); } catch (e) { toast(e.message, true); } },
});
