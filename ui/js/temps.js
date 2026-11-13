"use strict";

/* Temperature log */

const trOpen = (f, print) => window.open("/api/temp-reports/" + encodeURIComponent(f) + (print ? "?print=1" : ""), "_blank");

async function loadTemps(days) { try { state.treports = (await api("/api/temp-reports")).reports; } catch (e) { state.treports = []; } if (days !== undefined && days !== null) state.tdays = days; if (state.tdays === undefined) state.tdays = 7; let q = ""; if (state.tdays === 0) { if (!state.tfrom) state.tfrom = ymd(new Date(Date.now() - 29 * 864e5)); if (!state.tto) state.tto = ymd(new Date()); q = "?from=" + state.tfrom + "&to=" + state.tto; } else q = "?from=" + ymd(new Date(Date.now() - (state.tdays - 1) * 864e5)); state.temps = await api("/api/temps" + q); if (state.view === "temps" || state.view === "tpoints" || (state.view === "settings" && (state.tab === "tempsheet" || state.tab === "temppoints"))) render(); }

function tTrend(rows) {
  const pts = rows.filter(r => r.temp !== undefined && r.temp !== null && r.temp !== "" && !isNaN(+r.temp)).slice().sort((a, b) => (a.at < b.at ? -1 : 1)); if (!pts.length) return "";
  const v = pts.map(r => +r.temp), min = Math.min(...v), max = Math.max(...v), avg = v.reduce((x, y) => x + y, 0) / v.length, out = pts.filter(r => r.status === "Too Warm" || r.status === "Too Cold").length;
  const W = 760, H = 150, P = 28, lo = Math.min(min, ...[]) - 2, hi = max + 2, rng = (hi - lo) || 1;
  const X = i => P + (pts.length === 1 ? (W - 2 * P) / 2 : i * (W - 2 * P) / (pts.length - 1)), Y = t => H - P / 2 - (t - lo) / rng * (H - P);
  const line = pts.map((r, i) => (i ? "L" : "M") + X(i).toFixed(1) + " " + Y(+r.temp).toFixed(1)).join(" ");
  const dots = pts.map((r, i) => `<circle cx="${X(i).toFixed(1)}" cy="${Y(+r.temp).toFixed(1)}" r="3.5" fill="${r.status === "OK" ? "#2f7d4f" : "#a3243b"}"><title>${esc(when12(r.at))}: ${r.temp}°F</title></circle>`).join("");
  return `<div class="card" style="padding:10px 14px;margin:10px 0"><div class="row" style="gap:18px"><span><b>${pts.length}</b> readings</span><span>Low <b>${min}°F</b></span><span>High <b>${max}°F</b></span><span>Average <b>${avg.toFixed(1)}°F</b></span><span>Out of range <b style="color:${out ? "#a3243b" : "inherit"}">${out}</b></span><span class="hint">Safe range: ${esc(pts[pts.length - 1].limit)}</span></div>
  <svg viewBox="0 0 ${W} ${H}" style="width:100%;height:auto;max-height:180px" role="img" aria-label="Temperature trend"><text x="2" y="${Y(max).toFixed(1)}" font-size="10" fill="currentColor">${max}°</text><text x="2" y="${Y(min).toFixed(1)}" font-size="10" fill="currentColor">${min}°</text><path d="${line}" fill="none" stroke="currentColor" stroke-opacity=".55" stroke-width="1.5"/>${dots}</svg>
  <div class="hint" style="display:flex;justify-content:space-between"><span>${esc(dayLong(pts[0].at.slice(0, 10)))}</span><span>${esc(dayLong(pts[pts.length - 1].at.slice(0, 10)))}</span></div></div>`;
}

const tStatus = s => s === "OK" ? "OK" : s;

function vTemps() {
  const d = state.temps; if (!d) { loadTemps(); return `<p class="hint">Loading...</p>`; }
  const bad = state.tbad, rows = d.readings.filter(r => (!bad || r.status === "Too Warm" || r.status === "Too Cold") && (!state.tpoint || r.point === state.tpoint));
  const needs = r => (r.status === "Too Warm" || r.status === "Too Cold") && !(r.action || "").trim();
  const isBad = r => r.status === "Too Warm" || r.status === "Too Cold";
  return `<div class="row"><button class="btn green big" data-act="temp-enter">${icon("plus")}Enter Temperatures</button>${d.url ? `<button class="btn" data-act="temp-sync">${icon("down")}Sync Google Form</button>` : ""}
    <button class="btn" data-act="tr-save">${icon("print")}Save Daily Report</button><input type="date" class="field" id="trdate" value="${esc(state.trdate || ymd(new Date()))}" style="width:150px" title="Day for the daily report">
    <select class="field" id="tdays" style="width:170px">${[[1, "Today"], [7, "Last 7 days"], [30, "Last 30 days"], [90, "Last 90 days"], [365, "Last year"], [36500, "All readings"], [0, "Pick dates..."]].map(([n, l]) => `<option value="${n}" ${n === state.tdays ? "selected" : ""}>${l}</option>`).join("")}</select>
    ${state.tdays === 0 ? `<label style="font-weight:400">From <input type="date" class="field" id="tfrom" value="${esc(state.tfrom || "")}" style="width:150px"></label><label style="font-weight:400">To <input type="date" class="field" id="tto" value="${esc(state.tto || "")}" style="width:150px"></label>` : ""}
    <select class="field" id="tpoint" style="width:200px" title="Show one piece of equipment to see its trend"><option value="">All equipment</option>${[...new Set(d.readings.map(r => r.point))].sort().map(n => `<option ${n === state.tpoint ? "selected" : ""}>${esc(n)}</option>`).join("")}</select>
    <label style="font-weight:400"><input type="checkbox" data-act="temp-bad" ${bad ? "checked" : ""}> Out of range only</label>
    ${d.needAction ? `<span class="hint"><b>${d.needAction} reading${d.needAction === 1 ? "" : "s"} need a corrective action</b></span>` : ""}${d.syncedAt ? `<span class="hint">Form synced ${esc(d.syncedAt)}</span>` : ""}</div>
  ${!d.points.length ? `<p class="note warn">Add the places you check (coolers, freezers, hot wells) under Temperature Points, or import your Google Form responses in Settings, Temperature Form.</p>` : ""}
  ${state.tpoint ? tTrend(rows) : ""}<div class="tablewrap">${!rows.length ? `<div class="empty"><b>No readings</b>Use Enter Temperatures for a round of checks.</div>`
    : `<table class="plain"><thead><tr>${["When", "Point", "Temp", "Safe Range", "Status", "By", "Notes / Corrective Action"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}<th class="nosort"></th></tr></thead><tbody>
      ${rows.map(r => `<tr><td>${esc(when12(r.at))}</td><td><b>${esc(r.point)}</b></td><td>${r.temp}°F</td><td>${esc(r.limit)}</td><td>${tag(tStatus(r.status))}</td><td>${esc(r.by)}</td><td>${esc([r.notes, r.action].filter(Boolean).join(" · "))}${r.edited ? `<div class="hint">Edited by ${esc(r.edited)}</div>` : ""}</td>
        <td style="text-align:right"><div class="acts">${needs(r) ? `<button class="btn small wine" data-act="temp-action" data-id="${r.id}">Add Action</button> ` : ""}${isBad(r) ? `<button class="btn small" data-act="temp-ca" data-id="${r.id}">Corrective Action Form</button> ` : ""}<button class="btn small" data-act="temp-edit" data-id="${r.id}">${icon("edit")}Edit</button></div></td></tr>`).join("")}</tbody></table>`}</div>
  ${(state.treports || []).length ? `<h3>Saved Daily Reports</h3><div class="tablewrap"><table class="plain"><thead><tr><th class="nosort" style="width:auto">Day</th><th class="nosort" style="width:auto">Saved</th><th class="nosort"></th></tr></thead><tbody>${state.treports.map(x => `<tr><td><b>${esc(dayLong(x.Date))}</b></td><td>${esc(x.Saved)}</td><td style="text-align:right"><div class="acts"><button class="btn small" data-act="tr-open" data-f="${esc(x.File)}">Open</button> <button class="btn small" data-act="tr-print" data-f="${esc(x.File)}">${icon("print")}Print</button></div></td></tr>`).join("")}</tbody></table></div><p class="hint">Reports are also kept as files in the Temperature Reports folder beside the program.</p>` : ""}`;
}

function tempEnter() {
  const d = state.temps;
  showModal(`<h2>${icon("list")}Enter Temperatures</h2><div class="body" id="tebody"><p class="hint" style="margin-top:0">Fill in only the ones you checked. If a reading is out of range, a box asks for the corrective action.</p>
    ${!d.points.length ? `<div class="note warn"><b>No locations set up yet.</b> Add the coolers, freezers and hot wells you check, then Enter Temperatures will list each one.<div class="row" style="margin-top:8px"><button class="btn green" data-act="tpt-starter-enter">${icon("down")}Load Muriale's Units (29)</button><button class="btn" data-act="tpt-add-enter">${icon("plus")}Add a Location</button></div></div>` : ""}
    <table class="plain"><thead><tr><th class="nosort" style="width:auto">Point</th><th class="nosort" style="width:auto">Safe range</th><th class="nosort" style="width:auto">Temp °F</th><th class="nosort" style="width:auto">Notes / action</th></tr></thead><tbody>
    ${d.points.map(p => { const lim = tLimit(p); return `<tr data-pid="${p.id}"><td><b>${esc(p.name)}</b>${p.area ? `<div class="hint">${esc(p.area)}</div>` : ""}</td><td class="hint">${esc(lim.text)}</td><td><input type="number" step="0.1" class="field" data-tt="${p.id}" style="width:90px"></td><td><input class="field" data-tn="${p.id}" placeholder="Optional"></td></tr>`; }).join("")}</tbody></table>
    <div class="fields" style="margin-top:8px"><label>Your name or initials: <span class="req">*</span></label><input class="field" id="teby" value="${esc(who())}" style="width:220px"></div></div>
    <div class="foot"><button class="btn green big" data-act="temp-save">${icon("save")}Save Readings</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

function tLimit(p) {
  const k = (state.temps.kinds.find(x => x.kind === p.kind) || {}), min = p.min != null || p.max != null ? p.min : k.min, max = p.min != null || p.max != null ? p.max : k.max;
  return {min, max, text: min != null && max != null ? `${min} to ${max}°F` : max != null ? `${max}°F or below` : min != null ? `${min}°F or above` : "No limit"};
}

const tOut = (p, t) => { const l = tLimit(p); return (l.max != null && t > l.max) || (l.min != null && t < l.min); };

function tempEdit(id) {
  const r = state.temps && state.temps.readings.find(x => x.id === id); if (!r) return toast("That reading is not in the list.", true);
  const [d, t] = r.at.split(" ");
  showModal(`<h2>${icon("edit")}Edit Reading</h2><div class="body" id="tebody2" data-id="${id}"><div class="fields">
    <label>Date:</label><input type="date" class="field" data-te="date" value="${esc(d)}" style="width:190px">
    <label>Time:</label><input type="time" class="field" data-te="time" value="${esc(t || "")}" style="width:150px">
    <label>Point:</label><select class="field" data-te="pointId">${state.temps.points.map(p => `<option value="${p.id}" ${p.id === r.pointId ? "selected" : ""}>${esc(p.name)}</option>`).join("")}</select>
    <label>Temp °F:</label><input type="number" step="0.1" class="field" data-te="temp" value="${r.temp}" style="width:110px">
    <label>Taken by:</label><input class="field" data-te="by" value="${esc(r.by)}">
    <label>Notes:</label><textarea class="field" rows="2" data-te="notes">${esc(r.notes || "")}</textarea>
    <label>Corrective action:</label><textarea class="field" rows="2" data-te="action">${esc(r.action || "")}</textarea>
    <label>Your name: <span class="req">*</span></label><input class="field" data-te="editedBy" value="${esc(who())}"></div>
    <p class="hint">${r.source === "import" ? "This reading came from the form. Your change stays when the form is synced again." : ""} The correction is recorded with your name and the date.</p></div>
    <div class="foot"><button class="btn green big" data-act="temp-edit-go">${icon("save")}Save</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

function tempAction(id) {
  const r = state.temps && state.temps.readings.find(x => x.id === id);
  const go = async () => { if (!state.temps) await loadTemps(); tempAction(id); };
  if (!r) { go(); return; }
  showModal(`<h2>${icon("warn")}Corrective Action</h2><div class="body" id="tabody" data-id="${id}"><p><b>${esc(r.point)}</b> was <b>${r.temp}°F</b> (${esc(r.status.toLowerCase())}; safe range ${esc(r.limit)}) on ${esc(when12(r.at))}.</p><div class="fields">
    <label>What was done: <span class="req">*</span></label><textarea class="field" rows="3" id="taact" placeholder="e.g. Moved food to the reach-in, discarded items above 41°F for 4 hours, called service"></textarea>
    <label>Your name: <span class="req">*</span></label><input class="field" id="taby" value="${esc(who())}"></div></div>
    <div class="foot"><button class="btn green big" data-act="temp-action-go">${icon("save")}Save</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

function vTPoints() {
  const d = state.temps; if (!d) { loadTemps(); return `<p class="hint">Loading...</p>`; }
  return `<p class="hint" style="margin-top:0">The places you check temperatures. The safe range comes from the type (FDA Food Code) unless you set your own. Confirm the limits with your health department.</p>
  <div class="row"><button class="btn green big" data-act="tpt-edit" data-id="">${icon("plus")}Add Point</button><button class="btn" data-act="tpt-starter">${icon("down")}Load Muriale's Units (29)</button></div>
  <div class="tablewrap">${!d.points.length ? `<div class="empty"><b>No points yet</b>Add each cooler, freezer, hot well or other place you check.</div>` : `<table class="plain"><thead><tr>${["Point", "Area", "Type", "Safe range"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}<th class="nosort"></th></tr></thead><tbody>
    ${d.points.map(p => `<tr><td><b>${esc(p.name)}</b></td><td>${esc(p.area || "")}</td><td>${esc(p.kind)}</td><td>${esc(tLimit(p).text)}</td><td style="text-align:right"><div class="acts"><button class="btn small" data-act="tpt-edit" data-id="${p.id}">${icon("edit")}Edit</button> <button class="btn small wine" data-act="tpt-del" data-id="${p.id}">${icon("trash")}Delete</button></div></td></tr>`).join("")}</tbody></table>`}</div>`;
}

function tptForm(id) {
  const p = state.temps.points.find(x => x.id === id) || {kind: "Cold Holding"};
  showModal(`<h2>${icon("list")}${id ? "Edit" : "Add"} Temperature Point</h2><div class="body" id="tpbody" data-id="${esc(id)}"><div class="fields">
    <label>Name: <span class="req">*</span></label><input class="field" data-tp="name" value="${esc(p.name || "")}" placeholder="e.g. Walk-in Cooler 1">
    <label>Area / station:</label><input class="field" data-tp="area" list="tpareas" value="${esc(p.area || "")}" placeholder="e.g. Back Line"><datalist id="tpareas">${((state.maint && state.maint.areas) || []).map(a => `<option value="${esc(a)}">`).join("")}</datalist>
    <label>Type:</label><select class="field" data-tp="kind">${state.temps.kinds.map(k => `<option ${k.kind === p.kind ? "selected" : ""}>${k.kind}</option>`).join("")}</select>
    <label>Lowest safe °F:</label><input type="number" step="0.1" class="field" data-tp="min" value="${p.min != null ? p.min : ""}" style="width:130px" placeholder="Leave empty to use the type's limit">
    <label>Highest safe °F:</label><input type="number" step="0.1" class="field" data-tp="max" value="${p.max != null ? p.max : ""}" style="width:130px" placeholder="Leave empty to use the type's limit"></div>
    <p class="hint">Defaults: Cold Holding 41°F or below, Freezer 0°F or below, Hot Holding 135°F or above, Cooking 165°F or above.</p></div>
    <div class="foot"><button class="btn green big" data-act="tpt-save">${icon("save")}Save</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

function tTempSheet() {
  const d = state.temps; if (!d) { loadTemps(); return `<p class="hint">Loading...</p>`; }
  return `<div class="panel" style="max-width:820px"><h3 style="margin-top:0">Temperature Google Form</h3>
    <p>Your form saves each response in a Google Sheet. To bring responses in here, publish the responses sheet as CSV and paste its link:</p>
    <ol class="hint"><li>Open the form's response sheet in Google Sheets.</li><li>File, Share, <b>Publish to web</b>.</li><li>Choose the responses sheet and <b>Comma-separated values (.csv)</b>, then Publish and copy the link.</li></ol>
    <p class="hint">Anyone with the link can read the sheet, so keep it to temperatures. Nothing is sent back to Google.</p>
    <label>Published sheet link:</label><input class="field" id="tsurl" value="${esc(d.url || "")}" placeholder="https://docs.google.com/spreadsheets/d/e/.../pub?output=csv">
    <div class="row" style="margin-top:10px"><button class="btn green" data-act="tsurl-save">${icon("save")}Save Link</button><button class="btn" data-act="temp-sync">${icon("down")}Sync Now</button><button class="btn" data-act="temp-cols" ${d.url ? "" : "disabled"}>Check Columns</button>${d.syncedAt ? `<span class="hint">Last synced ${esc(d.syncedAt)}</span>` : ""}</div></div>
  <div class="panel" style="max-width:820px"><h3 style="margin-top:0">Import a file (Microsoft Forms or Google)</h3>
    <p>From Microsoft Forms choose Open in Excel and save or download the workbook, or in Google Sheets download as .csv. Then pick the .xlsx or .csv file here. Responses already imported are skipped, so you can import the same sheet again.</p>
    <div class="row"><button class="btn green" data-act="temp-file">${icon("down")}Choose CSV or Excel File</button><span class="hint" id="tempfilemsg"></span></div><input type="file" id="tempcsv" accept=".csv,.xlsx,.txt,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" hidden></div>`;
}

function tempMapDialog(pv, src) {
  const H = pv.headers, m = pv.map, opt = (sel, none) => `<option value="">${none || "(none)"}</option>` + H.map(h => `<option ${h === sel ? "selected" : ""}>${esc(h)}</option>`).join("");
  state.tmpSrc = src;
  showModal(`<h2>${icon("down")}Match the Form Columns</h2><div class="body" id="tmbody"><p class="hint" style="margin-top:0">${pv.count} response${pv.count === 1 ? "" : "s"} found. Check that each item matches the right column.</p><div class="fields">
    <label>Date / time: <span class="req">*</span></label><select class="field" data-tm="timestamp">${opt(m.timestamp, "Choose...")}</select>
    <label>Separate time column:</label><select class="field" data-tm="time">${opt(m.time)}</select>
    <label>Which unit was checked:</label><select class="field" data-tm="point">${opt(m.point, "(each unit has its own column)")}</select>
    <label>Temperature column(s): <span class="req">*</span></label><select class="field" multiple size="${Math.min(6, H.length)}" data-tm="temp">${H.map(h => `<option ${(m.temp || []).includes(h) ? "selected" : ""}>${esc(h)}</option>`).join("")}</select>
    <label></label><span class="hint">Choose one column, or hold Ctrl and choose several when each unit has its own temperature column.</span>
    <label>Initials / name:</label><select class="field" data-tm="by">${opt(m.by)}</select>
    <label>Notes:</label><select class="field" data-tm="notes">${opt(m.notes)}</select>
    <label>Corrective action:</label><select class="field" data-tm="action">${opt(m.action)}</select></div>
    ${pv.sample.length ? `<p class="hint">First response: ${esc(H.map((h, i) => h + " = " + (pv.sample[0][i] || "")).join(" | "))}</p>` : ""}</div>
    <div class="foot"><button class="btn green big" data-act="tm-go">${icon("down")}Import</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

const readTMap = () => { const m = {}; document.querySelectorAll("[data-tm]").forEach(el => { m[el.dataset.tm] = el.multiple ? [...el.selectedOptions].map(o => o.value) : el.value; }); return m; };

const syncMsg = o => { const r = o.result; return `${r.Added} new reading${r.Added === 1 ? "" : "s"} added${r.Skipped ? `, ${r.Skipped} already there` : ""}${r.NewPoints ? `, ${r.NewPoints} new point${r.NewPoints === 1 ? "" : "s"} created` : ""}${r.OutOfRange ? `. ${r.OutOfRange} out of range!` : "."}${r.Unreadable ? ` ${r.Unreadable} could not be read.` : ""}`; };

Object.assign(ACT, {
  "temp-edit"(el) { tempEdit(el.dataset.id); },
  async "temp-edit-go"() { const v = grab("[data-te]", "te"); if (!v.editedBy.trim()) return toast("Enter your name.", true); rememberWho(v.editedBy); v.temp = parseFloat(v.temp);
    if (isNaN(v.temp)) return toast("Enter the temperature.", true);
    try { state.temps = await api("/api/temps/" + $("#tebody2").dataset.id, {method: "PUT", body: JSON.stringify(v)}); closeModal(); render(); toast("Reading corrected."); } catch (e) { toast(e.message, true); } },
  "temp-enter"() { tempEnter(); },
  "temp-bad"(el) { state.tbad = el.checked; render(); },
  "temp-action"(el) { tempAction(el.dataset.id); },
  async "temp-action-go"() { const by = $("#taby").value.trim(), a = $("#taact").value.trim(); if (!a || !by) return toast("Write what was done and your name.", true); rememberWho(by);
    await api("/api/temps/" + $("#tabody").dataset.id + "/action", {method: "PUT", body: JSON.stringify({action: a, by})}); closeModal(); await loadTemps(); if (state.view === "todo") await refreshTodo(); toast("Saved."); },
  async "temp-save"() { const by = $("#teby").value.trim(), readings = [];
    for (const p of state.temps.points) { const v = $(`[data-tt="${p.id}"]`).value; if (v === "") continue; const t = parseFloat(v), note = $(`[data-tn="${p.id}"]`).value.trim();
      readings.push({pointId: p.id, temp: t, notes: tOut(p, t) ? "" : note, action: tOut(p, t) ? note : ""}); const out = tOut(p, t); if (out && !note) return toast(`${p.name} is out of range. Write the corrective action in the notes box.`, true); }
    if (!readings.length) return toast("Enter at least one temperature.", true); if (!by) return toast("Enter your name or initials.", true); rememberWho(by);
    state.temps = await api("/api/temps", {method: "POST", body: JSON.stringify({by, readings})}); closeModal(); render(); toast(`${readings.length} reading${readings.length === 1 ? "" : "s"} saved.`); },
  "temp-file"() { $("#tempcsv").click(); },
  async "temp-sync"() { try { const o = await api("/api/temps/sync", {method: "POST"}); state.temps = o.data; await loadTemps(Math.max(state.tdays || 7, 365)); render(); toast(syncMsg(o) + " Showing the last year in the Temperature Log."); } catch (e) { toast(e.message, true); } },
  async "temp-cols"() { try { tempMapDialog(await api("/api/temps/sheet-preview", {method: "POST"}), {sheet: true}); } catch (e) { toast(e.message, true); } },
  async "tsurl-save"() { state.temps = await api("/api/temps/settings", {method: "POST", body: JSON.stringify({url: $("#tsurl").value})}); render(); toast("Saved."); },
  async "tm-go"() { const m = readTMap(), src = state.tmpSrc;
    try { if (src.sheet) { state.temps = await api("/api/temps/settings", {method: "POST", body: JSON.stringify({map: m})}); const o = await api("/api/temps/sync", {method: "POST"}); state.temps = o.data; closeModal(); await loadTemps(Math.max(state.tdays || 7, 365)); render(); toast(syncMsg(o) + " Showing the last year in the Temperature Log."); }
      else { const o = await api("/api/temps/import", {method: "POST", body: JSON.stringify({csv: src.csv, map: m, save: true})}); state.temps = o.data; closeModal(); await loadTemps(Math.max(state.tdays || 7, 365)); render(); toast(syncMsg(o) + " Showing the last year in the Temperature Log."); } } catch (e) { toast(e.message, true); } },
  async "tr-save"() { const date = $("#trdate").value; if (!date) return toast("Choose a day for the report.", true); state.trdate = date;
    try { const o = await api("/api/temp-reports", {method: "POST", body: JSON.stringify({date})}); await loadTemps();
      showModal(`<h2>${icon("print")}Daily Temperature Report Saved</h2><div class="body"><p>The report for <b>${esc(dayLong(date))}</b> is saved. Save it again later in the day to include newer readings.</p><p class="hint">Saved in the Temperature Reports folder beside the program.</p></div>
        <div class="foot"><button class="btn green big" data-act="tr-print" data-f="${esc(o.file)}">${icon("print")}Print Now</button><button class="btn big" data-act="tr-open" data-f="${esc(o.file)}">Open</button><button class="btn big" data-act="close">Print Later</button></div>`); } catch (e) { toast(e.message, true); } },
  "tr-open"(el) { trOpen(el.dataset.f); },
  "tr-print"(el) { trOpen(el.dataset.f, true); },
  "temp-ca"(el) { const r = state.temps.readings.find(x => x.id === el.dataset.id); if (!r) return;
    caForm("", {area: "Food Safety", priority: "P2 - Urgent", issue: `${r.point} read ${r.temp}°F (${r.status.toLowerCase()}; safe range ${r.limit}) on ${when12(r.at)}.`, immediate: r.action || "", related: `Temperature log, ${r.point}, ${when12(r.at)}`}); },
  "tpt-edit"(el) { tptForm(el.dataset.id); },
  async "tpt-starter-enter"() { state.temps = await api("/api/temps/points/starter", {method: "POST"}); render(); tempEnter(); },
  "tpt-add-enter"() { state.tptReturn = true; tptForm(""); },
  async "tpt-starter"() { state.temps = await api("/api/temps/points/starter", {method: "POST"}); render(); toast("Units loaded. Existing ones were kept."); },
  async "tpt-save"() { const v = grab("[data-tp]", "tp"); const num = x => x === "" ? null : parseFloat(x); v.min = num(v.min); v.max = num(v.max); v.id = $("#tpbody").dataset.id;
    state.temps = await api("/api/temps/points", {method: "POST", body: JSON.stringify(v)}); closeModal(); render(); toast("Saved."); if (state.tptReturn) { state.tptReturn = false; tempEnter(); } },
  async "tpt-del"(el) { if (!confirm("Delete this point? Its past readings are kept.")) return; state.temps = await api("/api/temps/points/" + el.dataset.id, {method: "DELETE"}); render(); },
});
