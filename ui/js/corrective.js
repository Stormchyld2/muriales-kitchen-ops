"use strict";

/* Corrective actions */

async function loadCA() { state.ca = await api("/api/corrective"); if (state.view === "calog") render(); }

function vCA() {
  const d = state.ca; if (!d) { loadCA(); return `<p class="hint">Loading...</p>`; }
  const f = state.caFilter || "open", over = x => x.status !== "Closed" && x.target && x.target < ymd(new Date());
  const rows = d.forms.filter(x => f === "all" || (f === "open" ? x.status !== "Closed" : x.status === "Closed"));
  return `<div class="row"><button class="btn green big" data-act="ca-new">${icon("plus")}New Corrective Action</button>
    <select class="field" data-act="ca-filter" style="width:190px">${[["open", "Open and in progress"], ["closed", "Closed"], ["all", "All forms"]].map(([v, l]) => `<option value="${v}" ${v === f ? "selected" : ""}>${l}</option>`).join("")}</select>
    <span class="hint">${d.forms.filter(x => x.status !== "Closed").length} open, ${d.forms.filter(over).length} overdue, ${d.forms.length} on file</span></div>
  <div class="tablewrap">${!rows.length ? `<div class="empty"><b>No forms here</b>Use New Corrective Action for a significant food-safety, sanitation, safety, maintenance or training issue.</div>`
    : `<table class="plain"><thead><tr>${["ID", "Date", "Area", "Priority", "Issue", "Owner", "Target", "Status"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}<th class="nosort"></th></tr></thead><tbody>
    ${rows.map(x => `<tr><td><b>${esc(x.id)}</b></td><td>${esc(x.date)}</td><td>${esc(x.area)}</td><td>${esc(x.priority)}</td><td>${esc(x.issue.length > 90 ? x.issue.slice(0, 90) + "..." : x.issue)}</td><td>${esc(x.owner)}</td><td>${esc(x.target)}</td><td>${tag(over(x) ? "Overdue" : x.status)}</td>
      <td style="text-align:right"><div class="acts"><button class="btn small" data-act="ca-open" data-id="${esc(x.id)}">${icon("edit")}Edit</button> <button class="btn small" data-act="ca-print" data-id="${esc(x.id)}">${icon("print")}Print</button></div></td></tr>`).join("")}</tbody></table>`}</div>`;
}

async function caForm(id, pre) {
  if (!state.ca) await loadCA();
  const d = state.ca, f = Object.assign({date: ymd(new Date()), status: "Open", identifiedBy: who(), priority: "P3 - Routine"}, id ? d.forms.find(x => x.id === id) || {} : (pre || {}));
  const t = (k, label, extra) => `<label>${label}</label><input class="field" data-ca="${k}" value="${esc(f[k] || "")}" ${extra || ""}>`, a = (k, label, rows) => `<label>${label}</label><textarea class="field" rows="${rows || 2}" data-ca="${k}">${esc(f[k] || "")}</textarea>`;
  const sel = (k, label, opts) => `<label>${label}</label><select class="field" data-ca="${k}">${opts.map(o => `<option ${o === f[k] ? "selected" : ""}>${o}</option>`).join("")}</select>`;
  const yn = (k, label) => sel(k, label, ["", "Yes", "No"]);
  showModal(`<h2>${icon("warn")}${id ? esc(id) + " Corrective Action" : "New Corrective Action"}</h2><div class="body" id="cabody" data-id="${esc(id || "")}"><div class="fields">
    <h3 style="margin:0">Issue Information</h3>${t("date", "Date identified:", 'type="date"')}${t("identifiedBy", 'Identified by: <span class="req">*</span>')}${sel("area", "Area / system:", d.areas)}${sel("priority", "Priority:", d.priorities)}
    ${a("issue", 'Issue / finding: <span class="req">*</span>', 3)}${a("risk", "Immediate risk / impact:")}${a("immediate", "Immediate correction taken:")}
    <h3 style="margin:8px 0 0">Root Cause / Required Action</h3>${a("rootCause", "Root cause / contributing factors:")}${a("required", "Required corrective action:", 3)}${t("owner", "Assigned owner:")}${t("target", "Target date:", 'type="date"')}
    ${yn("training", "Training required?")}${yn("workOrder", "Work order / MaintainX needed?")}${yn("vendor", "Vendor needed?")}${yn("sop", "SOP / procedure change needed?")}${yn("followUpNeeded", "Follow-up inspection needed?")}${t("evidence", "Evidence / photo ref:")}
    ${t("doneDate", "Action completed on:", 'type="date"')}${t("doneBy", "Completed by:")}${t("verifyDate", "Verification date:", 'type="date"')}${t("verifiedBy", "Verified by:")}${a("verifyResult", "Verification result:")}
    <h3 style="margin:8px 0 0">Closure</h3>${sel("status", "Status:", d.statuses)}${t("closedBy", "Closed by (needed to close):")}${yn("repeat", "Repeat issue?")}${t("followUpDate", "Follow-up date:", 'type="date"')}${a("followUpResult", "Follow-up result:")}${a("notes", "Management notes:")}${a("related", "Related records:")}
    </div></div><div class="foot"><button class="btn green big" data-act="ca-save">${icon("save")}Save Form</button>${id ? `<button class="btn big" data-act="ca-print" data-id="${esc(id)}">${icon("print")}Print</button>` : ""}<button class="btn big" data-act="close">Cancel</button></div>`);
}

Object.assign(ACT, {
  "ca-new"() { caForm(""); },
  "ca-open"(el) { caForm(el.dataset.id); },
  "ca-print"(el) { window.open("/api/corrective/" + encodeURIComponent(el.dataset.id) + "/print?print=1", "_blank"); },
  "ca-filter"(el) { state.caFilter = el.value; render(); },
  async "ca-save"() { const v = grab("[data-ca]", "ca"); v.id = $("#cabody").dataset.id;
    if (!v.identifiedBy.trim()) return toast("Enter who identified the issue.", true); rememberWho(v.identifiedBy);
    try { const o = await api("/api/corrective", {method: "POST", body: JSON.stringify(v)}); state.ca = o; closeModal(); if (state.view === "todo") await refreshTodo(); render();
      showModal(`<h2>${icon("save")}${esc(o.saved.id)} Saved</h2><div class="body"><p>The form is in the Corrective Action Log. Do you want to print it?</p><p class="hint">A copy is also kept in the Corrective Action Forms folder beside the program.</p></div>
        <div class="foot"><button class="btn green big" data-act="ca-print" data-id="${esc(o.saved.id)}">${icon("print")}Print Now</button><button class="btn big" data-act="close">Print Later</button></div>`); } catch (e) { toast(e.message, true); } },
});
