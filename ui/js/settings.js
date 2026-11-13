"use strict";

/* Settings, storage, backups and recovery */

const TABS = [["maintainx","MaintainX"],["eqareas","Areas, Types & Departments"],["temppoints","Temperature Points"],["tempsheet","Temperature Form"],["workbook","Load Workbook Data"],["storage","Library Folder"],["areas","Storage Areas"],["backup","Backup &amp; Restore"],["history","Edit History"],["transfer","Import &amp; Export"],["cleanup","Clean Up"],["recovery","Recovery"],["diagnostics","Diagnostics"]];

function loadTab() { if (state.view !== "settings") return; const f = {eqareas: loadMaint, tempsheet: loadTemps, maintainx: loadMaint, backup: loadBackups, recovery: loadRecovery, diagnostics: loadDiag, history: () => loadHistory("")}[state.tab]; if (f) f(); }

async function loadBackups() { state.backups = await api("/api/backups"); if (state.view === "settings") render(); }

async function loadRecovery() { state.rec = await api("/api/recovery"); if (state.view === "settings" && state.tab === "recovery") render(); }

async function loadDiag() { state.diag = await api("/api/diagnostics"); if (state.view === "settings" && state.tab === "diagnostics") render(); }

function vSettings() {
  const tabs = `<div class="tabs">${TABS.map(([k, l]) => `<button class="${state.tab === k ? "on" : ""}" data-act="tab" data-t="${k}">${l}</button>`).join("")}</div>`;
  const body = ({storage: tStorage, areas: tAreas, backup: tBackup, transfer: tTransfer, cleanup: tCleanup, workbook: tWorkbook, eqareas: tEqAreas, temppoints: vTPoints, tempsheet: tTempSheet, maintainx: tMaintainx, recovery: tRecovery, diagnostics: tDiag, history: tHistory}[state.tab] || tStorage)();
  return tabs + body;
}

function tAreas() {
  const list = state.lib.locations || [], count = n => state.lib.products.filter(p => (p.location || "").trim().toLowerCase() === n.toLowerCase()).length;
  const known = new Set(list.map(x => x.toLowerCase()));
  const extra = [...new Set(state.lib.products.map(p => (p.location || "").trim()).filter(l => l && !known.has(l.toLowerCase())))];
  return `
  <div class="panel" style="max-width:860px">
    <h3 style="margin-top:0">Storage areas (the room or area that holds the shelf)</h3>
    <p class="hint">The rooms and areas that hold the shelves chemicals sit on, such as Drystock or the Mop Room. They appear in the Storage Location list when adding or editing a sheet, and in the printed index. Renaming an area updates every product stored there. An area with products in it can't be removed until they are moved.</p>
    ${list.length ? list.map((l, i) => { const n = count(l); return `<div class="catrow"><div class="grow"><b>${esc(l)}</b><div class="hint">${n ? n + " product" + (n === 1 ? "" : "s") + " stored here" : "No products"}</div></div>
      <button class="btn small" data-act="area-rename" data-i="${i}">${icon("edit")}Rename</button>
      <button class="btn small wine" data-act="area-remove" data-i="${i}" ${n ? "disabled title=\"Move the products out of this area first\"" : ""}>${icon("trash")}Remove</button></div>`; }).join("") : `<p class="hint">No storage areas yet.</p>`}
    <div class="row" style="margin-top:10px"><input id="newarea" class="field" style="max-width:300px" maxlength="60" placeholder="New room or area, e.g. Walk-in Cooler"><button class="btn green" data-act="area-add">${icon("plus")}Add Area</button></div>
  </div>
  ${extra.length ? `<div class="panel" style="max-width:860px"><h3 style="margin-top:0">Areas typed in on sheets</h3><p class="hint">These were typed into products but are not in the list above.</p>
    ${extra.map(l => `<div class="catrow"><div class="grow"><b>${esc(l)}</b><div class="hint">${count(l)} product${count(l) === 1 ? "" : "s"}</div></div><button class="btn small green" data-act="area-adopt" data-name="${esc(l)}">Add to List</button></div>`).join("")}</div>` : ""}`;
}

const saveAreas = (locations, renames = {}) => api("/api/locations", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({locations, renames})}).then(reload).then(render);

function daysLeft(p) { const t = new Date(p.removedAt).getTime(); if (!t) return ""; const d = Math.max(0, 30 - Math.floor((Date.now() - t) / 86400000)); return d === 0 ? "Deleted at next start" : d + " day" + (d === 1 ? "" : "s") + " left"; }

function tCleanup() {
    const live = [...state.lib.products].sort((a, b) => a.name.localeCompare(b.name)), gone = state.lib.removed || [];
    const head = last => `<thead><tr>${["Product Name", "Manufacturer", "Category", last].map(h => `<th class="nosort" style="width:auto">${h}</th>`).join("")}<th class="nosort" style="width:400px"></th></tr></thead>`;
    return `
  <div class="panel">
    <h3 style="margin-top:0">Products in the library</h3>
    <p class="hint">Replace swaps in a newer PDF from the manufacturer and keeps the old one. Remove takes a discontinued product out of the library and the printed book.</p>
    ${live.length ? `<table class="plain">${head("SDS Date")}<tbody>
      ${live.map(p => `<tr><td>${esc(p.name)}${p.missing ? '<span class="tag red">PDF missing</span>' : ""}</td><td>${esc(p.manufacturer)}</td><td>${esc(p.category)}</td><td>${sdsDate(p)}</td>
        <td style="text-align:right"><div class="acts"><button class="btn small" data-act="replace" data-id="${p.id}">${icon("swap")}Replace</button> <button class="btn small wine" data-act="remove" data-id="${p.id}">${icon("trash")}Remove</button></div></td></tr>`).join("")}
    </tbody></table>` : `<p class="hint">The library is empty.</p>`}
  </div>
  <div class="panel">
    <h3 style="margin-top:0">Removed products</h3>
    ${gone.length ? `<table class="plain">${head("Removed")}<tbody>
      ${gone.map(p => `<tr><td>${esc(p.name)}</td><td>${esc(p.manufacturer)}</td><td>${esc(p.category)}</td><td>${fmtDate(p.removedAt)}<div class="hint">${daysLeft(p)}</div></td>
        <td style="text-align:right"><div class="acts">${p.archivedFile ? `<button class="btn small" data-act="view-removed" data-id="${p.id}">View PDF</button> ` : ""}<button class="btn small green" data-act="restore-product" data-id="${p.id}">Restore</button> <button class="btn small wine" data-act="purge-ask" data-id="${p.id}">${icon("trash")}Delete Permanently</button></div></td></tr>`).join("")}
    </tbody></table>` : `<p class="hint">Nothing has been removed. Removed products are listed here so they can be restored.</p>`}
    <p class="hint">Removed products are kept for 30 days, then deleted permanently along with their PDF.</p>
  </div>`;
}

function tStorage() {
  return `
  <div class="panel" style="max-width:860px">
    <h3 style="margin-top:0">Where the sheets are stored</h3>
    <p>Every PDF and the product list are kept in this folder:</p>
    <p><code>${esc(state.lib.dataFolder)}</code></p>
    <p>The library is made of ordinary files, with no database. The PDFs sit in the “sheets” folder under readable names, and the product details are in “library.json”. A PDF copied straight into the “sheets” folder shows up in the library marked “needs review”.</p>
    <div class="row"><button class="btn" data-act="open-folder">${icon("folder")}Open Data Folder</button><button class="btn" data-act="folder-ask">${icon("swap")}Change Location...</button></div>
    <p class="hint" style="margin-bottom:0">The library can live in a OneDrive, SharePoint or network folder. Only open the program on one computer at a time when it does, so two people never save over each other.</p>
  </div>`;
}

function tBackup() {
  const b = state.backups;
  return `
  <div class="panel" style="max-width:860px">
    <h3 style="margin-top:0">Backup</h3>
    <p><b>Backup Both Folders</b> puts the SDS library and all the kitchen operations files (equipment, PM jobs, temperatures, cleaning, to-do, settings) into one zip in the Kitchen Backups folder beside the program. Copy it to a USB drive or cloud folder so everything survives if this computer fails.</p>
    <div class="row"><button class="btn green" data-act="backup-full">${icon("down")}Backup Both Folders</button><button class="btn" data-act="backup">${icon("down")}SDS Library Only</button><button class="btn" data-act="open-backups">${icon("folder")}Open Backups Folder</button></div>
  </div>
  <div class="panel" style="max-width:860px">
    <h3 style="margin-top:0">Restore from a backup</h3>
    <p class="hint">To restore a backup kept on a USB drive or another computer, copy its zip file into the backups folder first, then come back to this screen.</p>
    ${!b ? `<p class="hint">Looking for backups...</p>` : !b.backups.length ? `<p class="hint">No backups found in ${esc(b.folder)}.</p>`
      : b.backups.map(x => `<div class="catrow"><div class="grow"><b>${esc(x.file)}</b><div class="hint">${x.kind === "full" ? "Both folders" : x.kind === "maint" ? "Kitchen operations files only" : "SDS library only"} · Created ${fmtDate(x.created)} at ${esc(x.created.slice(11, 16))} · ${(x.size / 1048576).toFixed(1)} MB</div></div><button class="btn small" data-act="${x.kind === "library" ? "restore-ask" : "restore-other"}" data-file="${esc(x.file)}">Restore...</button></div>`).join("")}
  </div>`;
}

function tTransfer() {
  return `
  <div class="panel" style="max-width:860px">
    <h3 style="margin-top:0">Export</h3>
    <p>Save the product list as a spreadsheet that opens in Excel, or save the whole library (every PDF and the product list) as one zip file to keep or move to another computer.</p>
    <div class="row"><a class="btn" href="/api/export/products.csv" download>${icon("down")}Product List (Excel / CSV)</a><a class="btn" href="/api/export/library.zip" download>${icon("down")}Whole Library (zip)</a></div>
  </div>
  <div class="panel" style="max-width:860px">
    <h3 style="margin-top:0">Import many PDFs at once</h3>
    <p>Choose SDS PDFs from Sysco, a supplier or a folder. Each one is added to the library marked “needs review”, with the manufacturer, SDS date and emergency summary drafted from the sheet where possible, so a manager can check it. PDFs already in the library, and files that are not PDFs, are skipped.</p>
    <div class="row"><button class="btn green" data-act="import-pdfs">${icon("up")}Choose PDFs...</button></div>
    <input type="file" id="importpdfs" accept="application/pdf,.pdf" multiple hidden>
  </div>
  <div class="panel" style="max-width:860px">
    <h3 style="margin-top:0">Import a library zip</h3>
    <p>Choose a library zip made by this program, from a backup or another computer. It is checked and placed in the backups list. Nothing changes until you restore it from Backup &amp; Restore.</p>
    <div class="row"><button class="btn" data-act="import-zip">${icon("up")}Choose Library Zip...</button></div>
    <input type="file" id="importzip" accept=".zip,application/zip" hidden>
  </div>`;
}

function tRecovery() {
  const r = state.rec;
  if (!r) return `<div class="panel"><p class="hint">Looking...</p></div>`;
  const ps = r.previousSave || {};
  return `
  <div class="panel" style="max-width:860px">
    <h3 style="margin-top:0">Go back one save</h3>
    <p>Every time the library is saved, the version before it is kept. If something was changed by mistake, this puts that earlier version back. The current one is kept too, in case you change your mind.</p>
    ${ps.damaged ? `<p><b>The previous-save copy is damaged and cannot be used.</b></p>` : ps.exists ? `<p class="hint">Previous save from ${fmtDate(ps.modified)} at ${esc(String(ps.modified).slice(11, 16))}: ${ps.products} product${ps.products === 1 ? "" : "s"}${ps.removed ? `, ${ps.removed} removed` : ""}.</p><div class="row"><button class="btn wine" data-act="rec-prev">${icon("back")}Go Back One Save...</button></div>` : `<p class="hint">There is no earlier save yet.</p>`}
  </div>
  <div class="panel" style="max-width:860px">
    <h3 style="margin-top:0">Rebuild from the PDFs in the folder</h3>
    <p>If the product list is lost or damaged but the PDFs are still in the “sheets” folder, this sets the damaged list aside and builds a new one from the PDFs. Every product comes back marked “needs review”, because the details were in the lost list.</p>
    <div class="row"><button class="btn wine" data-act="rec-rebuild">${icon("swap")}Rebuild Library...</button></div>
  </div>
  <div class="panel" style="max-width:860px">
    <h3 style="margin-top:0">Old PDFs not linked to a product</h3>
    <p class="hint">PDFs kept in the “removed” folder that no removed product points to, for example after a rebuild or a restore.</p>
    ${(r.unlinked || []).length ? r.unlinked.map(x => `<div class="catrow"><div class="grow"><b>${esc(x.title)}</b><div class="hint">${esc(x.file)}</div></div><button class="btn small green" data-act="rec-unlinked" data-file="${esc(x.file)}">Add Back to Library</button></div>`).join("") : `<p class="hint">None.</p>`}
  </div>
  <div class="panel" style="max-width:860px">
    <h3 style="margin-top:0">Libraries set aside by a restore</h3>
    <p class="hint">Restoring a backup never deletes the library it replaces. It is moved into the backups folder.</p>
    ${(r.beforeRestore || []).length ? r.beforeRestore.map(n => `<div>${esc(n)}</div>`).join("") : `<p class="hint">None.</p>`}
    <div class="row" style="margin-top:8px"><button class="btn" data-act="open-backups">${icon("folder")}Open Backups Folder</button></div>
  </div>`;
}

function tDiag() {
  const d = state.diag;
  if (!d) return `<div class="panel"><p class="hint">Checking...</p></div>`;
  const cls = {ok: "ok", warn: "warn", bad: "bad"}, mark = {ok: "OK", warn: "Check", bad: "Problem"};
  const rows = state.integ;
  return `
  <div class="panel" style="max-width:860px">
    <h3 style="margin-top:0">Program health <span class="hint">version ${esc(d.version)}</span></h3>
    ${d.checks.map(c => `<div class="note ${cls[c.status] || "warn"}" style="margin-bottom:6px"><b style="min-width:70px">${mark[c.status] || c.status}</b><span class="grow"><b>${esc(c.name)}.</b> ${esc(c.detail)}</span></div>`).join("")}
    <div class="row" style="margin-top:10px"><button class="btn" data-act="diag-run">${icon("swap")}Check Again</button><button class="btn" data-act="diag-save">${icon("down")}Save Report</button><button class="btn" data-act="diag-copy">Copy Report</button></div>
    <p class="hint" style="margin-bottom:0">Save the report and send it to whoever supports this program. It lists the computer, the folders and the checks above, and no safety data.</p>
  </div>
  <div class="panel" style="max-width:860px">
    <h3 style="margin-top:0">Document check</h3>
    <p>Each PDF is fingerprinted when it is added. This reads every PDF again and shows any that no longer match, which means the file was changed or damaged outside the program.</p>
    <div class="row"><button class="btn green" data-act="integ-run">${icon("search")}Check Every PDF</button></div>
    ${!rows ? "" : (() => { const bad = rows.filter(r => r.status !== "ok");
      return !bad.length ? `<p style="margin-bottom:0"><b>All ${rows.length} PDFs match their fingerprints.</b></p>`
        : `<table class="plain" style="margin-top:10px"><tbody>${bad.map(r => `<tr><td>${esc(r.name)}</td><td>${r.status === "missing" ? "PDF missing" : "File changed"}</td><td style="text-align:right"><div class="acts">${r.status === "changed" ? `<button class="btn small" data-act="integ-accept" data-id="${r.id}">Accept This File</button> ` : ""}<button class="btn small" data-act="open-product" data-id="${r.id}">Open</button></div></td></tr>`).join("")}</tbody></table>`; })()}
  </div>`;
}

function tWorkbook() {
  return `<div class="panel" style="max-width:760px"><h3 style="margin-top:0">Load the PM schedule from your workbooks</h3>
    <p>Loads the 74 recurring jobs from your Maintenance PM Calendar (PM-001 to PM-074) and the 14 daily and weekly grounds, facility and cleaning tasks from the Quick Look (WG-003 to WG-016). Each keeps its job ID, asset ID, who does it, how often, and its notes. Kitchen, dish and refrigeration jobs go to Equipment PM, building and grounds jobs to Grounds / Facility, and cleaning jobs to Cleaning. You can change any job's group later.</p>
    <p>The due dates in the workbook are from early September. Choose what to do with dates that have already gone by:</p>
    <label style="display:block;font-weight:400;margin:4px 0"><input type="radio" name="past" value="today" checked> Move them forward to their next turn on the same schedule (recommended). A monthly job due Sept 1 starts Dec 1 and a weekly Thursday job starts next Thursday, so nothing starts out overdue and the work stays spread out.</label>
    <label style="display:block;font-weight:400;margin:4px 0"><input type="radio" name="past" value="keep"> Keep the dates from the workbook. Jobs past due will show as overdue.</label>
    <p class="hint">Jobs already loaded are skipped, so running this again does not create duplicates.</p>
    <div class="row"><button class="btn green big" data-act="wb-load">${icon("down")}Load the Jobs</button></div></div>
  <div class="panel" style="max-width:760px"><h3 style="margin-top:0">Equipment from the workbook</h3><p>The workbook numbers each piece of equipment (K-001, D-001, R-001 ...). This adds those as Equipment records with their area and type guessed from the name, and links each maintenance job to its equipment. Edit any of them afterwards. Running it again does not create duplicates.</p><div class="row"><button class="btn big" data-act="wb-equip">${icon("down")}Create Equipment Records</button></div></div>`;
}

function tEqAreas() {
  const m = state.maint; if (!m) { loadMaint(); return `<p class="hint">Loading...</p>`; }
  return `<div class="panel" style="max-width:760px"><h3 style="margin-top:0">Equipment Areas</h3>
    <p>The places equipment sits, used in the Area list when you add or edit equipment. Put one area per line, then Save. Removing an area here does not change equipment already assigned to it.</p>
    <textarea class="field" id="eqareaslist" rows="12" placeholder="One area per line">${esc((m.areas || []).join("\n"))}</textarea>
    <div class="row" style="margin-top:10px"><button class="btn green" data-act="eqareas-save">${icon("save")}Save</button></div></div>
  <div class="panel" style="max-width:760px"><h3 style="margin-top:0">Equipment Types</h3>
    <p>The kinds of equipment, used in the Type list when you add or edit equipment. One type per line, then Save.</p>
    <textarea class="field" id="eqtypeslist" rows="12" placeholder="One type per line">${esc((m.types || []).join("\n"))}</textarea>
    <div class="row" style="margin-top:10px"><button class="btn green" data-act="eqtypes-save">${icon("save")}Save</button></div></div>
  <div class="panel" style="max-width:760px"><h3 style="margin-top:0">Departments</h3>
    <p>Used to split the To-Do list into a sheet for each department, such as Dish or Prep. An item goes to the department whose name appears in its area or wording; anything else goes under General. One department per line. To send more things to a department, add a bar and words after its name, for example <b>Salads | salad, garde manger</b>. Then Save.</p>
    <textarea class="field" id="deptlist" rows="9" placeholder="One department per line">${esc((m.departmentLines || m.departments || []).join("\n"))}</textarea>
    <div class="row" style="margin-top:10px"><button class="btn green" data-act="dept-save">${icon("save")}Save</button></div></div>`;
}

function tMaintainx() {
  const m = state.maint; if (!m) { loadMaint(); return `<p class="hint">Loading...</p>`; }
  return `<div class="panel" style="max-width:760px"><h3 style="margin-top:0">MaintainX</h3>
    <p>The <b>Submit Work Order</b> button opens your MaintainX web address in its own window, with no browser tabs or address bar. Sign in once and it stays signed in on that computer. This program does not connect to MaintainX; work orders are entered there, and the work order number can be written on the issue here.</p>
    <label>MaintainX web address:</label><input class="field" id="portal" value="${esc(m.portalUrl)}">
    <div class="row" style="margin-top:10px"><button class="btn green" data-act="portal-save">${icon("save")}Save</button><button class="btn" data-act="submit-wo">${icon("ext")}Open It Now</button></div></div>
  <div class="panel" style="max-width:760px"><h3 style="margin-top:0">Who can clear out-of-service equipment</h3>
    <p>When equipment is taken out of service it stays that way until one of these managers clears it, and says what was checked. Put one name per line. The program has no logins, so it checks the name typed when clearing; leave the list empty to let anyone clear it.</p>
    <textarea class="field" id="mgrs" rows="4" placeholder="One manager per line">${esc((m.managers || []).join("\n"))}</textarea>
    <div class="row" style="margin-top:10px"><button class="btn green" data-act="mgr-save">${icon("save")}Save</button></div></div>
  <div class="panel" style="max-width:760px"><h3 style="margin-top:0">Repair or replace</h3>
    <p>When the total spent on a unit's upkeep reaches this percentage of the cost of a new unit, the Assets screen suggests considering a replacement. It also flags units past their normal life that have reached half of this.</p>
    <label>Suggest replacing at (% of a new unit):</label><input class="field" type="number" id="repct" min="5" max="200" value="${m.replacePercent}" style="width:110px">
    <div class="row" style="margin-top:10px"><button class="btn green" data-act="repct-save">${icon("save")}Save</button></div></div>`;
}

Object.assign(ACT, {
  async "wb-load"() { const past = (document.querySelector("[name=past]:checked") || {}).value || "today"; const o = await api("/api/maint/import-schedule", {method: "POST", body: JSON.stringify({pastDates: past})});
    state.maint = await api("/api/maint"); state.cal.events = null; render(); toast(o.added ? `Loaded ${o.added} jobs${o.skipped ? ` (${o.skipped} were already there)` : ""}. See them under Schedule.` : `Nothing new to load. ${o.skipped} jobs were already there.`); },
  async "wb-equip"() { const o = await api("/api/maint/import-equipment", {method: "POST", body: "{}"}); state.maint = await api("/api/maint"); render(); toast(o.added ? `Added ${o.added} pieces of equipment and linked ${o.linked} jobs.` : `Nothing new. ${o.linked} jobs linked.`); },
  async "eqareas-save"() { state.maint = await api("/api/maint/settings", {method: "POST", body: JSON.stringify({areas: $("#eqareaslist").value.split("\n")})}); render(); toast("Saved."); },
  async "eqtypes-save"() { state.maint = await api("/api/maint/settings", {method: "POST", body: JSON.stringify({types: $("#eqtypeslist").value.split("\n")})}); render(); toast("Saved."); },
  async "dept-save"() { state.maint = await api("/api/maint/settings", {method: "POST", body: JSON.stringify({departments: $("#deptlist").value.split("\n")})}); render(); toast("Saved."); },
  async "mgr-save"() { state.maint = await api("/api/maint/settings", {method: "POST", body: JSON.stringify({managers: $("#mgrs").value.split("\n")})}); render(); toast("Saved."); },
  async "portal-save"() { state.maint = await api("/api/maint/settings", {method: "POST", body: JSON.stringify({portalUrl: $("#portal").value})}); render(); toast("Saved."); },
  async "refresh-folder"() { const had = new Set(state.lib.products.map(p => p.id)), gone = state.lib.products.filter(p => p.missing).length;
    await reload(); render();
    const added = state.lib.products.filter(p => !had.has(p.id)), nowGone = state.lib.products.filter(p => p.missing).length;
    toast(added.length ? `Added ${added.length} new sheet${added.length === 1 ? "" : "s"}: ${added.slice(0, 3).map(p => p.name).join(", ")}${added.length > 3 ? "..." : ""}. Check the details and edit if needed.`
      : nowGone !== gone ? "Refreshed. The PDF-missing flags were updated." : "Refreshed. No new PDFs in the folder."); },
  tab(el) { state.tab = el.dataset.t; render(); loadTab(); },
  async "restore-ask"(el) { const file = el.dataset.file, i = await api("/api/backups/inspect", {method: "POST", body: JSON.stringify({file})});
    showModal(`<h2>${icon("swap")}Restore From Backup</h2><div class="body">
      <p style="margin-top:0">Replace the current library with this backup?</p>
      <div class="box"><dl><dt>Backup:</dt><dd>${esc(file)}</dd><dt>Contains:</dt><dd>${i.products} product${i.products === 1 ? "" : "s"}, ${i.sheets} PDF${i.sheets === 1 ? "" : "s"}${i.removed ? `, ${i.removed} removed` : ""}</dd><dt>Library now:</dt><dd>${state.lib.products.length} product${state.lib.products.length === 1 ? "" : "s"}</dd></dl></div>
      ${i.missingPdfs.length ? `<p><b>Note:</b> the backup has no PDF for: ${esc(i.missingPdfs.join(", "))}.</p>` : ""}
      <p>The current library is not deleted. It is moved into a “Before restore” folder inside SDS Backups. If the backup does not open correctly, the current library is put back.</p></div>
      <div class="foot"><button class="btn wine big" data-act="restore-go" data-file="${esc(file)}">Restore This Backup</button><button class="btn big" data-act="close">Cancel</button></div>`); },
  async "restore-go"(el) { el.disabled = true; const r = await api("/api/restore", {method: "POST", body: JSON.stringify({file: el.dataset.file})}); closeModal(); state.sel.clear(); await reload(); await loadBackups(); toast(`Backup restored: ${r.products} products. The previous library is in ${r.previousKeptIn}`); },
  "open-backups"() { return api("/api/open-folder?which=backups", {method: "POST"}); },
  "folder-ask"() { showModal(`<h2>${icon("folder")}Change Library Location</h2><div class="body">
      <p style="margin-top:0">Paste the full path of the folder to keep the library in. For Microsoft 365, use the OneDrive or SharePoint folder that is synced to this computer, or a mapped network drive.</p>
      <input id="newfolder" class="field" placeholder="e.g. C:\\Users\\Office\\OneDrive - Muriale's\\SDS Data">
      <p class="hint" style="color:#444">Tip: open the folder in File Explorer, click the address bar, copy, and paste it here.</p>
      <p>If the folder is empty, the current library is copied into it. If it already holds an SDS library, that library is opened instead. The current folder is left where it is.</p></div>
      <div class="foot"><button class="btn green big" data-act="folder-go">Use This Folder</button><button class="btn big" data-act="close">Cancel</button></div>`); $("#newfolder").focus(); },
  async "folder-go"(el) { const path = $("#newfolder").value; if (!path.trim()) return toast("Paste the folder path first.", true); el.disabled = true;
    try { const r = await api("/api/data-folder", {method: "POST", body: JSON.stringify({path})}); closeModal(); state.sel.clear(); await reload(); await loadBackups(); toast(r.copied ? `Library copied to ${r.folder}. The old copy is still in ${r.previous}` : `Now using the library already in ${r.folder} (${r.products} products).`); }
    finally { el.disabled = false; } },
  "area-add"() { const v = $("#newarea").value.trim(); if (!v) return toast("Type the name of the area first.", true);
    if ((state.lib.locations || []).some(x => x.toLowerCase() === v.toLowerCase())) return toast("That area is already in the list.", true);
    return saveAreas([...(state.lib.locations || []), v]).then(() => toast(`“${v}” added.`)); },
  "area-adopt"(el) { return saveAreas([...(state.lib.locations || []), el.dataset.name]); },
  "area-remove"(el) { const l = [...state.lib.locations], [gone] = l.splice(+el.dataset.i, 1); return saveAreas(l).then(() => toast(`“${gone}” removed.`)); },
  "area-rename"(el) { const old = state.lib.locations[+el.dataset.i], n = state.lib.products.filter(p => (p.location || "").trim().toLowerCase() === old.toLowerCase()).length;
    showModal(`<h2>${icon("edit")}Rename Storage Area</h2><div class="body"><input id="renamearea" class="field" value="${esc(old)}" maxlength="60" data-old="${esc(old)}">${n ? `<p class="hint" style="color:#444">${n} product${n === 1 ? "" : "s"} stored here will move to the new name.</p>` : ""}</div>
      <div class="foot"><button class="btn green big" data-act="area-rename-go">Save</button><button class="btn big" data-act="close">Cancel</button></div>`); $("#renamearea").select(); },
  "area-rename-go"() { const el = $("#renamearea"), old = el.dataset.old, v = el.value.trim(); if (!v || v === old) return closeModal();
    if (state.lib.locations.some(x => x !== old && x.toLowerCase() === v.toLowerCase())) return toast("That area is already in the list.", true);
    closeModal(); return saveAreas(state.lib.locations.map(x => x === old ? v : x), {[old]: v}).then(() => toast("Renamed.")); },
  async "integ-accept"(el) { await api(`/api/integrity/${el.dataset.id}/accept`, {method: "POST"}); await reload(); if (state.integ) state.integ = (await api("/api/integrity?full=1")).rows; render(); toast("File accepted. The binder will show this sheet as needing a reprint."); },
  async "integ-run"(el) { el.disabled = true; try { state.integ = (await api("/api/integrity?full=1")).rows; } finally { el.disabled = false; } render(); },
  async "diag-run"() { state.diag = null; render(); await loadDiag(); },
  "diag-save"() { const a = document.createElement("a"), b = new Blob([state.diag.report], {type: "text/plain"}); a.href = URL.createObjectURL(b); a.download = `SDS Manager diagnostics ${ymd(new Date())}.txt`; a.click(); setTimeout(() => URL.revokeObjectURL(a.href), 5000); },
  async "diag-copy"() { try { await navigator.clipboard.writeText(state.diag.report); toast("Report copied."); } catch (e) { toast("Could not copy. Use Save Report instead.", true); } },
  "rec-prev"() { showModal(`<h2>${icon("warn")}Go Back One Save</h2><div class="body"><p style="margin-top:0">Replace the product list with the version from before the last save?</p><p>Changes made in the last save will be undone. The current list is kept as a copy beside it.</p></div>
      <div class="foot"><button class="btn wine big" data-act="rec-prev-go">Go Back</button><button class="btn big" data-act="close">Cancel</button></div>`); },
  async "rec-prev-go"(el) { el.disabled = true; const r = await api("/api/recovery/previous-save", {method: "POST"}); closeModal(); await reload(); await loadRecovery(); toast(`Went back one save: ${r.products} products. The list it replaced is kept as ${r.keptAs}.`); },
  "rec-rebuild"() { showModal(`<h2>${icon("warn")}Rebuild Library</h2><div class="body"><p style="margin-top:0">Set the current product list aside and build a new one from the PDFs in the “sheets” folder?</p><p>Details such as manufacturer, category and emergency summaries are not in the PDFs, so each product comes back as “needs review”. Only use this if the list is lost or damaged.</p></div>
      <div class="foot"><button class="btn wine big" data-act="rec-rebuild-go">Rebuild</button><button class="btn big" data-act="close">Cancel</button></div>`); },
  async "rec-rebuild-go"(el) { el.disabled = true; const r = await api("/api/recovery/rebuild", {method: "POST"}); closeModal(); await reload(); await loadRecovery(); toast(`Library rebuilt: ${r.products} products need review. The old list is kept as ${r.keptAs}.`); },
  async "rec-unlinked"(el) { const p = await api("/api/recovery/unlinked", {method: "POST", body: JSON.stringify({file: el.dataset.file})}); await reload(); await loadRecovery(); toast(`“${p.name}” was added back. Check its details.`); },
  "open-folder"() { return api("/api/open-folder", {method: "POST"}); },
  async "backup-full"() { const r = await api("/api/backup/full", {method: "POST"}); await loadBackups(); toast("Backup of both folders created: " + r.file); },
  async "restore-other"(el) { const full = /^Full Backup/.test(el.dataset.file);
    if (!confirm((full ? "Replace the SDS library AND the kitchen operations files with this backup?" : "Replace the kitchen operations files (equipment, PM jobs, temperatures, cleaning, to-do) with this backup?") + "\n\nWhat is there now is saved to the Kitchen Backups folder first.")) return;
    const r = await api("/api/backup/restore-other", {method: "POST", body: JSON.stringify({file: el.dataset.file})}); state.sel.clear(); await reload(); await loadBackups(); toast("Restored. The files that were replaced are kept in " + (r.previousKeptIn || "the Kitchen Backups folder")); },
  async backup() { const r = await api("/api/backup", {method: "POST"}); await loadBackups(); toast("Backup created: " + r.file); },
});
