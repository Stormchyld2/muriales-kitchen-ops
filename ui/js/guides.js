"use strict";

/* Station Guides and Opening / Closing checklists */

const GD = {station: "", q: "", list: null, lists: null, pick: "", ticked: new Set()};

async function loadGuides() {
  try {
    const [g, st] = await Promise.all([api("/api/guides" + (GD.q ? "?q=" + encodeURIComponent(GD.q) : "")), api("/api/reg/stations")]);
    GD.list = g.guides; state.reg.stations = st;
  } catch (e) { GD.list = []; }
  render();
}

function vGuides() {
  if (!GD.list || !state.reg.stations) { loadGuides(); return `<p class="hint">Loading...</p>`; }
  const sts = state.reg.stations.rows.filter(r => r.active !== "No").map(r => r.name);
  const rows = GD.list.filter(g => (!GD.station || g.station === GD.station));
  return `<div class="row"><input class="field" id="gdq" style="width:320px" placeholder="Search every guide..." value="${esc(GD.q)}"><button class="btn" data-act="gd-search">Search</button>${GD.q ? `<button class="btn" data-act="gd-clear">Clear</button>` : ""}<button class="btn green" data-act="reg-new" data-c="guides">${icon("plus")}Add Guide</button><span class="hint">${rows.length} guides</span></div>
  <div class="row" style="flex-wrap:wrap;gap:6px"><button class="btn small ${GD.station ? "" : "green"}" data-act="gd-station" data-s="">All</button>${sts.map(n => `<button class="btn small ${GD.station === n ? "green" : ""}" data-act="gd-station" data-s="${esc(n)}">${esc(n)}</button>`).join("")}</div>
  ${GD.station ? `<div class="row"><button class="btn green" data-act="gd-book" data-s="${esc(GD.station)}">${icon("print")}Print the ${esc(GD.station)} Station Book</button><span class="hint">Every SOP, guide and checklist for this station, each on its own page.</span></div>` : ""}
  ${!rows.length ? `<div class="empty"><b>No guides found</b></div>` : [...new Set(rows.map(g => g.station))].map(st => `<div class="tablewrap"><h3 style="margin:10px 0 4px">${esc(st)}</h3><table class="plain"><thead><tr><th class="nosort" style="width:150px">Kind</th><th class="nosort" style="width:auto">Guide</th><th class="nosort"></th></tr></thead><tbody>
  ${rows.filter(g => g.station === st).map(g => `<tr><td>${esc(g.kind)}</td><td><b>${esc(g.title)}</b>${g.snip ? `<div class="hint">...${esc(g.snip)}...</div>` : ""}</td><td style="text-align:right;white-space:nowrap"><button class="btn small green" data-act="gd-open" data-id="${esc(g.id)}">Open</button> <button class="btn small" data-act="reg-open" data-c="guides" data-id="${esc(g.id)}">${icon("edit")}Edit</button> <button class="btn small" data-act="gd-print" data-id="${esc(g.id)}">${icon("print")}Print</button></td></tr>`).join("")}</tbody></table></div>`).join("")}`;
}

async function guideView(id) {
  const o = await api("/api/guides/" + id), g = o.guide;
  showModal(`<h2>${icon("list")}${esc(g.title)}</h2><div class="body"><div class="hint">${esc(g.station)} - ${esc(g.kind)}${g.updated ? " - updated " + esc(shortDate(g.updated)) : ""}</div><div class="gmd">${o.html}</div></div>
    <div class="foot"><button class="btn" data-act="gd-print" data-id="${esc(id)}">${icon("print")}Print</button><button class="btn" data-act="reg-open" data-c="guides" data-id="${esc(id)}">Edit</button><button class="btn big" data-act="close">Close</button></div>`);
}

async function loadStChk() { try { GD.lists = (await api("/api/stchecklists")).lists; } catch (e) { GD.lists = []; } render(); }

function vStChk() {
  if (!GD.lists) { loadStChk(); return `<p class="hint">Loading...</p>`; }
  const cur = GD.lists.find(c => c.id === GD.pick);
  if (cur) {
    const items = cur.lines.filter(l => l.kind === "item");
    return `<div class="row"><button class="btn" data-act="stc-back">Back</button><b style="font-size:17px">${esc(cur.title)}</b><span class="hint" id="stcount">${GD.ticked.size} of ${items.length} done</span><span style="margin-left:auto"></span><button class="btn" data-act="reg-open" data-c="stchecklists" data-id="${esc(cur.id)}">Edit Checklist</button><button class="btn" data-act="stc-print" data-id="${esc(cur.id)}">${icon("print")}Blank Sheet</button></div>
    ${cur.finished ? `<div class="note warn">Already finished today by <b>${esc(cur.finished.by)}</b> at ${esc(cur.finished.time)} (${esc(cur.finished.done)}). You can still finish it again.</div>` : ""}
    <div class="panel">${cur.lines.map(l => l.kind === "head" ? `<h3 style="margin:14px 0 4px;color:var(--gold)">${esc(l.text)}</h3>` : l.kind === "note" ? `<div class="hint" style="margin:2px 0">${esc(l.text)}</div>`
      : `<label style="display:flex;gap:10px;align-items:center;padding:7px 4px;font-size:15px;cursor:pointer"><input type="checkbox" data-stn="${l.n}" ${GD.ticked.has(l.n) ? "checked" : ""} style="width:22px;height:22px">${esc(l.text)}</label>`).join("")}
    <div class="fields" style="margin-top:14px"><label>Your first name:</label><input class="field" id="stby" value="${esc(who())}" style="width:220px"><label>Notes:</label><textarea class="field" id="stnotes" rows="2"></textarea></div>
    <div class="row" style="margin-top:10px"><button class="btn green big" data-act="stc-finish">${icon("save")}Finish &amp; Sign</button><button class="btn" data-act="stc-all">Tick Everything</button></div></div>`;
  }
  const sts = [...new Set(GD.lists.map(c => c.station))];
  return `<p class="hint" style="margin-top:0">Pick a station and tap Opening or Closing. Tick each task, type your first name, and finish. The result goes in the Checklist Log.</p>
  <div class="dgrid" style="grid-template-columns:repeat(4,240px);grid-auto-rows:auto">${sts.map(n => `<div class="dblock" style="padding:14px"><div class="dlabel" style="font-size:16px;margin-bottom:8px">${esc(n)}</div>${GD.lists.filter(c => c.station === n).map(c => `<button class="btn" style="width:100%;margin:3px 0;justify-content:space-between" data-act="stc-pick" data-id="${esc(c.id)}"><span>${esc(c.kind)}</span>${c.finished ? `<span style="color:#58b368">&#10003; ${esc(c.finished.by)}</span>` : ""}</button>`).join("")}</div>`).join("")}</div>`;
}

Object.assign(ACT, {
  "gd-station"(el) { GD.station = el.dataset.s; render(); },
  "gd-search"() { GD.q = $("#gdq").value.trim(); GD.list = null; render(); },
  "gd-clear"() { GD.q = ""; GD.list = null; render(); },
  "gd-open"(el) { guideView(el.dataset.id); },
  "gd-book"(el) { repPrint("stationbook:" + el.dataset.s); },
  "gd-print"(el) { repPrint("guide:" + el.dataset.id); },
  "stc-pick"(el) { GD.pick = el.dataset.id; GD.ticked = new Set(); render(); },
  "stc-back"() { GD.pick = ""; GD.lists = null; render(); },
  "stc-print"(el) { repPrint("checklist:" + el.dataset.id); },
  "stc-all"() { const c = GD.lists.find(x => x.id === GD.pick); c.lines.forEach(l => { if (l.kind === "item") GD.ticked.add(l.n); }); render(); },
  async "stc-finish"() {
    const by = $("#stby").value.trim(); if (!by) { toast("Type your first name.", true); return; }
    try { const r = await api("/api/stchecklog/finish", {method: "POST", body: JSON.stringify({id: GD.pick, by, notes: $("#stnotes").value, done: [...GD.ticked]})});
      toast(r.missed ? `Saved. ${r.missed} not done.` : "Saved. All done."); GD.pick = ""; GD.lists = null; delete state.reg.stchecklog; state.today = null; render(); } catch (e) { toast(e.message, true); } },
});
document.addEventListener("change", e => { const c = e.target.closest && e.target.closest("[data-stn]"); if (!c) return; const n = +c.dataset.stn; c.checked ? GD.ticked.add(n) : GD.ticked.delete(n); const cur = GD.lists.find(x => x.id === GD.pick), h = document.getElementById("stcount"); if (h && cur) h.textContent = `${GD.ticked.size} of ${cur.lines.filter(l => l.kind === "item").length} done`; });
