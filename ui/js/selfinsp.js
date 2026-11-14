"use strict";

/* Weekly scored self-inspection and the monthly performance review */

const SF = {data: null, month: new Date().toISOString().slice(0, 7)};
const SF_PTS = {C: 0, COS: 1, M: 3, CR: 5, NA: 0};

async function loadSelf() { try { SF.data = await api("/api/selfinsp"); } catch (e) { SF.data = {error: e.message}; } render(); }
const sfBand = b => b === "Green" ? "Done" : b === "Red" ? "Overdue" : "Upcoming";

function vSelf() {
  const d = SF.data;
  if (!d) { loadSelf(); return `<div class="empty"><b>Loading...</b></div>`; }
  if (d.error) return `<div class="empty"><b>Could not load</b>${esc(d.error)}</div>`;
  const m = d.month;
  const rows = d.rows.map(r => `<tr><td><b>${esc(r.id)}</b></td><td>${esc(shortDate(r.date))}</td><td>${esc(r.by)}</td><td><b>${esc(r.score)}</b> / 100</td><td><span class="status ${sfBand(r.band)}">${esc(r.band)}</span></td><td>${esc(r.critical)}</td><td>${esc(r.major)}</td><td>${esc(r.cos)}</td>
    <td style="text-align:right;white-space:nowrap"><button class="btn small" data-act="sf-open" data-id="${esc(r.id)}">${icon("edit")}Edit</button> <button class="btn small" data-act="rep-print" data-id="selfinsp:${esc(r.id)}">${icon("print")}Print</button> <button class="btn small wine" data-act="reg-del" data-c="selfinsp" data-id="${esc(r.id)}">${icon("trash")}</button></td></tr>`).join("");
  return `<div class="row"><button class="btn green big" data-act="sf-new">${icon("plus")}New Weekly Self-Inspection</button><button class="btn" data-act="rep-print" data-id="selfinsp-blank">${icon("print")}Blank Form</button><span class="hint">Score every area, fix what you can, and the internal score is worked out for you.</span></div>
  <div class="panel" style="max-width:980px"><div class="row" style="justify-content:space-between"><h3 style="margin:0">Monthly review: ${esc(m.month)}</h3><span><input type="month" id="sfmonth" class="field" style="width:170px" value="${esc(SF.month)}"> <button class="btn small" data-act="sf-month">Show</button> <button class="btn small" data-act="rep-print" data-id="selfinsp-month">${icon("print")}Print</button></span></div>
    <div class="stats"><div class="stat"><div class="n">${m.count ? m.avg : "-"}</div>average score</div><div class="stat"><div class="n">${m.count ? m.low : "-"}</div>lowest weekly score</div><div class="stat ${m.criticals ? "warn" : ""}"><div class="n">${m.criticals}</div>critical findings</div><div class="stat"><div class="n">${m.majors}</div>major findings</div><div class="stat ${m.overdueCA ? "warn" : ""}"><div class="n">${m.overdueCA}</div>overdue corrective actions</div></div>
    <p class="hint">${m.top.length ? "Top repeat findings: " + esc(m.top.join("; ")) : "No repeat findings recorded this month."}</p></div>
  ${d.rows.length ? `<div class="tablewrap"><table class="plain"><thead><tr><th class="nosort" style="width:auto">ID</th><th class="nosort" style="width:auto">Date</th><th class="nosort" style="width:auto">Inspector</th><th class="nosort" style="width:auto">Score</th><th class="nosort" style="width:auto">Band</th><th class="nosort" style="width:auto">Critical</th><th class="nosort" style="width:auto">Major</th><th class="nosort" style="width:auto">Corrected</th><th class="nosort"></th></tr></thead><tbody>${rows}</tbody></table></div>` : `<div class="empty"><b>No self-inspections yet</b>Press New Weekly Self-Inspection to do the first one.</div>`}`;
}

function sfScore() {
  let rated = 0, ded = 0, crit = 0;
  document.querySelectorAll("[data-sin]").forEach(s => { const v = s.value; if (v && v !== "NA") { rated++; ded += SF_PTS[v]; if (v === "CR") crit++; } });
  const sc = rated ? 100 - Math.floor(ded * 100 / (5 * rated)) : 100, el = $("#sfscore");
  if (el) el.innerHTML = `<b style="font-size:20px">${sc} / 100</b> <span class="hint">${rated} areas scored${crit ? ", " + crit + " critical" : ""}</span>`;
}

function sfForm(id) {
  const r = id ? SF.data.rows.find(x => x.id === id) : null, got = {};
  if (r) (r.results || "").split("\n").forEach(l => { const p = l.split("|"); if (p.length >= 2) got[p[0].trim()] = {r: p[1].trim(), n: p.slice(2).join("|").trim()}; });
  showModal(`<h2>${icon("check")}${r ? esc(r.id) : "New Weekly Self-Inspection"}</h2><div class="body" id="sfbody" data-id="${esc(id || "")}">
    <p class="hint">C = Compliant, COS = Corrected on Spot (1 point off), M = Major (3 off), CR = Critical (5 off), NA = Not applicable. Green is 95 or more with no unresolved critical finding; Yellow is 90 to 94; Red is under 90.</p>
    <div class="fields"><label>Inspector / manager:</label><input class="field" id="sfby" value="${esc(r ? r.by : who())}"><label>Date:</label><input type="date" class="field" id="sfdate" value="${esc(r ? r.date : ymd(new Date()))}"></div>
    <div class="tablewrap"><table class="plain"><thead><tr><th class="nosort" style="width:auto">Area</th><th class="nosort" style="width:100px">Rating</th><th class="nosort" style="width:220px">Notes</th></tr></thead><tbody>
    ${SF.data.areas.map(a => `<tr><td><b>${esc(a.area)}</b><div class="hint">${esc(a.standard)}</div></td><td><select class="field" data-sin="${esc(a.id)}"><option value=""></option>${["C", "COS", "M", "CR", "NA"].map(o => `<option ${(got[a.id] || {}).r === o ? "selected" : ""}>${o}</option>`).join("")}</select></td><td><input class="field" data-sinn="${esc(a.id)}" value="${esc((got[a.id] || {}).n || "")}"></td></tr>`).join("")}</tbody></table></div>
    <div id="sfscore" style="margin:10px 0"></div>
    <div class="fields"><label>Recheck date:</label><input type="date" class="field" id="sfrecheck" value="${esc(r ? r.recheck : "")}"><label>Critical findings corrected and verified?</label><select class="field" id="sfcrit" style="width:120px">${["", "Yes", "No"].map(o => `<option ${r && r.critResolved === o ? "selected" : ""}>${o}</option>`).join("")}</select><label>Notes:</label><textarea class="field" id="sfnotes" rows="2">${esc(r ? r.notes : "")}</textarea></div>
    ${r ? "" : `<label style="display:flex;gap:8px;align-items:center;margin-top:8px"><input type="checkbox" id="sfca" checked style="width:20px;height:20px">Create a corrective action form for each Major and Critical finding</label>`}</div>
    <div class="foot"><button class="btn green big" data-act="sf-save">${icon("save")}Save</button><button class="btn big" data-act="close">Cancel</button></div>`);
  sfScore();
}

document.addEventListener("change", e => { if (e.target && e.target.matches && e.target.matches("[data-sin]")) sfScore(); });

Object.assign(ACT, {
  "sf-new"() { sfForm(""); },
  "sf-open"(el) { sfForm(el.dataset.id); },
  async "sf-month"() { SF.month = $("#sfmonth").value || SF.month; try { SF.data.month = await api("/api/selfinsp/month?month=" + encodeURIComponent(SF.month)); render(); } catch (e) { toast(e.message, true); } },
  async "sf-save"() {
    const res = [...document.querySelectorAll("[data-sin]")].filter(s => s.value).map(s => `${s.dataset.sin}|${s.value}|${(document.querySelector(`[data-sinn="${s.dataset.sin}"]`) || {}).value || ""}`.replace(/\n/g, " ")).join("\n");
    const body = {id: $("#sfbody").dataset.id, date: $("#sfdate").value, by: $("#sfby").value, results: res, recheck: $("#sfrecheck").value, critResolved: $("#sfcrit").value, notes: $("#sfnotes").value, makeCA: !!($("#sfca") && $("#sfca").checked)};
    try { const o = await api("/api/selfinsp", {method: "POST", body: JSON.stringify(body)}); remembered("by", body.by); closeModal(); SF.data = null; state.today = null; render(); toast(o.made ? `Saved. ${o.made} corrective action form${o.made === 1 ? "" : "s"} created.` : "Saved."); } catch (e) { toast(e.message, true); }
  },
});
{
  const nav = ACT.nav; ACT.nav = (el, e) => { if (["selfi"].includes(el.dataset.v)) SF.data = null; return nav(el, e); };
  const del = ACT["reg-del"]; ACT["reg-del"] = async (el, e) => { const r = await del(el, e); SF.data = null; if (state.view === "selfi") render(); return r; };
}
