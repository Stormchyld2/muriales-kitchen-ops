// Par sheets: daily par check, station par sheets, order guide pars.
const PR = {station: "", chk: null, ostation: "", vendor: "", q: "", ploc: ""};

function prLoad(name) {
  if (state.reg[name]) return state.reg[name];
  api("/api/reg/" + name).then(o => { state.reg[name] = o; render(); });
  return null;
}
function prChips(list, cur, act, all) {
  return `<div class="row" style="flex-wrap:wrap;gap:6px;margin-bottom:8px">${all ? `<button class="btn ${cur === "" ? "green" : ""}" data-act="${act}" data-v="">All</button>` : ""}${list.map(v => `<button class="btn ${cur === v ? "green" : ""}" data-act="${act}" data-v="${esc(v)}">${esc(v)}</button>`).join("")}</div>`;
}
const prBtns = (c, id) => `<button class="btn small" data-act="reg-open" data-c="${c}" data-id="${esc(id)}">${icon("edit")}Edit</button> <button class="btn small wine" data-act="reg-del" data-c="${c}" data-id="${esc(id)}">${icon("trash")}</button>`;
const prIn = (c, r, k, w) => `<input class="field ipe" data-c="${c}" data-id="${esc(r.id)}" data-k="${k}" value="${esc(r[k] || "")}" style="width:${w}px">`;

function vParCheck() {
  const sd = prLoad("parsheets"); if (!sd) return `<p class="hint">Loading...</p>`;
  const stations = [...new Set(sd.rows.map(r => r.station))];
  if (!PR.station && stations.length) PR.station = stations[0];
  if (PR.station && (!PR.chk || PR.chk.station !== PR.station)) { api("/api/parcheck?station=" + encodeURIComponent(PR.station)).then(o => { PR.chk = o; render(); }); return `<p class="hint">Loading...</p>`; }
  const c = PR.chk; if (!c) return `<div class="empty"><b>No par sheets yet</b>Add items under Station Par Sheets.</div>`;
  const ck = c.check || {}, on = ck.on || {}, mk = ck.make || {}, ini = ck.init || {};
  return `${prChips(stations, PR.station, "pr-station", false)}
  <p class="hint" style="margin-top:0">Count each item against its par before service. Type what is on hand, and in Make what has to be made or pulled. Any item below par is prepped before service begins.</p>
  <div class="row" style="margin-bottom:8px;flex-wrap:wrap;align-items:flex-end"><label>Your first name<br><input id="pr-by" class="field" style="width:160px" value="${esc(ck.by || who())}"></label><button class="btn green big" data-act="pr-save">${icon("check")}Save Par Check</button>
    <button class="btn" data-act="rep-view" data-id="parsheet:${esc(c.station)}">${icon("print")}Print Blank Sheet</button><button class="btn" data-act="rep-view" data-id="parcheck:${esc(c.station)}">${icon("print")}Print This Check</button>${ck.by ? `<span class="hint">Saved today by ${esc(ck.by)} at ${esc(ck.time)}</span>` : ""}</div>
  <div class="tablewrap"><table class="plain"><thead><tr><th class="nosort" style="width:auto">Product</th><th class="nosort" style="width:auto">PAR quantity</th><th class="nosort" style="width:auto">On hand</th><th class="nosort" style="width:auto">Make</th><th class="nosort" style="width:auto">Initial</th></tr></thead><tbody>${c.items.map(r => `<tr><td><b>${esc(r.item)}</b></td><td>${esc(r.par)}</td><td><input class="field" data-pc="on" data-id="${esc(r.id)}" value="${esc(on[r.id] || "")}" style="width:150px"></td><td><input class="field" data-pc="make" data-id="${esc(r.id)}" value="${esc(mk[r.id] || "")}" style="width:150px"></td><td><input class="field" data-pc="init" data-id="${esc(r.id)}" value="${esc(ini[r.id] || "")}" style="width:70px"></td></tr>`).join("")}</tbody></table></div>
  ${c.notes.length ? `<div class="panel" style="max-width:900px;margin-top:10px"><b>Notes</b><ul>${c.notes.map(n => `<li>${esc(n)}</li>`).join("")}</ul></div>` : ""}`;
}

function vParSheets() {
  const d = prLoad("parsheets"); if (!d) return `<p class="hint">Loading...</p>`;
  const stations = [...new Set(d.rows.map(r => r.station))], rows = d.rows.filter(r => !PR.ploc || r.station === PR.ploc);
  return `<div class="row" style="flex-wrap:wrap"><button class="btn green big" data-act="pr-new" data-c="parsheets">${icon("plus")}Add Par Item</button><button class="btn" data-act="rep-view" data-id="${PR.ploc ? "parsheet:" + esc(PR.ploc) : "parsheets-all"}">${icon("print")}Print ${PR.ploc ? esc(PR.ploc) + " Sheet" : "All Sheets"}</button><button class="btn" data-act="nav" data-v="parnotes">Station Notes</button><span class="hint">${rows.length} items</span></div>
  <p class="hint" style="margin-top:0">${esc(regDefs0(d).intro)} Type a new par straight into the list and it saves.</p>${prChips(stations, PR.ploc, "pr-ploc", true)}
  <div class="tablewrap"><table class="plain"><thead><tr><th class="nosort" style="width:auto">Station</th><th class="nosort" style="width:auto">Product</th><th class="nosort" style="width:auto">PAR quantity</th><th class="nosort"></th></tr></thead><tbody>${rows.map(r => `<tr><td>${esc(r.station)}</td><td><b>${esc(r.item)}</b></td><td>${prIn("parsheets", r, "par", 360)}</td><td style="text-align:right"><div class="acts">${prBtns("parsheets", r.id)}</div></td></tr>`).join("")}</tbody></table></div>`;
}
const regDefs0 = d => d.def;

function prToday() {
  const od = state.reg.orderdays; if (!od) return "";
  const day = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"][new Date().getDay()], r = od.rows.find(x => x.day === day);
  return `<div class="note" style="max-width:1100px"><b>Order today (${day}):</b> ${r ? esc(r.vendors) : "no order days entered"}</div>`;
}

function prView(d) {
  const vendors = [...new Set([...(d.def.fields.find(f => f.key === "vendor").opts || []), ...d.rows.map(r => r.vendor)])];
  if (!PR.vendor || !vendors.includes(PR.vendor)) PR.vendor = vendors.includes("Sysco") ? "Sysco" : vendors[0];
  return vendors;
}

function vOrderPars() {
  const d = prLoad("orderpars"); if (!d) return `<p class="hint">Loading...</p>`; if (!prLoad("orderdays")) return `<p class="hint">Loading...</p>`;
  const vendors = prView(d), q = PR.q.toLowerCase(), od = state.reg.orderdays.rows, today = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"][new Date().getDay()];
  const dueToday = v => { const r = od.find(x => x.day === today); return r && r.vendors.toLowerCase().replace(/[^a-z ,]/g, "").includes(v.toLowerCase().replace(/[^a-z ]/g, "")); };
  const rows = d.rows.filter(r => r.vendor === PR.vendor && (!q || (r.item + " " + r.location + " " + r.upc).toLowerCase().includes(q)));
  const locs = [...new Set(rows.map(r => r.location || "No location"))];
  const body = locs.map(l => `<tr><td colspan="6" style="background:rgba(255,255,255,.06);font-weight:700">${esc(l)}</td></tr>` + rows.filter(r => (r.location || "No location") === l).map(r => `<tr><td><b>${esc(r.item)}</b></td><td>${esc(r.upc)}</td><td>${esc(r.pack)}</td><td>${prIn("orderpars", r, "busy", 80)}</td><td>${prIn("orderpars", r, "slow", 80)}</td><td style="text-align:right"><div class="acts">${prBtns("orderpars", r.id)}</div></td></tr>`).join("")).join("");
  return `${prToday()}<div class="row" style="flex-wrap:wrap;align-items:flex-end"><label>View<br><select id="pr-view" class="field" style="width:240px">${vendors.map(v => `<option value="${esc(v)}" ${v === PR.vendor ? "selected" : ""}>${esc(v)}${dueToday(v) ? " (order today)" : ""}</option>`).join("")}</select></label>
    <button class="btn green big" data-act="pr-new" data-c="orderpars">${icon("plus")}Add ${esc(PR.vendor)} Item</button><button class="btn" data-act="rep-view" data-id="orderpars:${esc(PR.vendor)}">${icon("print")}Print ${esc(PR.vendor)} Sheet</button><button class="btn" data-act="rep-view" data-id="orderpars-days">${icon("print")}Print Order Days</button>
    <label style="margin-left:10px">Find<br><input id="pr-q" class="field" style="width:220px" value="${esc(PR.q)}" placeholder="product or location"></label><span class="hint">${rows.length} items in this view</span></div>
  <p class="hint" style="margin-top:0">One vendor at a time. Pick the vendor from View, count against the busy or slow par, and print the sheet. Type a new par straight into the list. This is a par reference; purchasing stays in 365.</p>
  <div class="tablewrap"><table class="plain"><thead><tr><th class="nosort" style="width:auto">Product</th><th class="nosort" style="width:auto">Item #</th><th class="nosort" style="width:auto">Pack</th><th class="nosort" style="width:auto">Busy par</th><th class="nosort" style="width:auto">Slow par</th><th class="nosort"></th></tr></thead><tbody>${!rows.length ? `<tr><td colspan="6" class="hint">Nothing here yet for ${esc(PR.vendor)}. Use Add ${esc(PR.vendor)} Item to enter a line: where it is kept, product, item number, pack, busy and slow par.</td></tr>` : body}</tbody></table></div>`;
}

Object.assign(ACT, {
  "pr-station"(el) { PR.station = el.dataset.v; PR.chk = null; render(); },
  "pr-ploc"(el) { PR.ploc = el.dataset.v; render(); },
  
  "pr-new"(el) { regForm(el.dataset.c, "", el.dataset.c === "orderpars" ? {vendor: PR.vendor} : {station: PR.ploc}); },
  async "pr-save"() {
    const by = ($("#pr-by").value || "").trim(); if (!by) return toast("Type your first name first.");
    const rows = PR.chk.items.map(r => { const g = k => { const e = document.querySelector(`[data-pc=${k}][data-id="${r.id}"]`); return e ? e.value : ""; }; return {id: r.id, on: g("on"), make: g("make"), init: g("init")}; });
    try { const o = await api("/api/parcheck", {method: "POST", body: JSON.stringify({station: PR.station, by, rows})}); remembered("by", by); PR.chk = null; toast("Par check saved at " + o.time + "."); render(); } catch (e) { toast(e.message || "Could not save.", true); }
  },
});
document.addEventListener("change", e => { if (e.target.id === "pr-view") { PR.vendor = e.target.value; PR.q = ""; render(); } });
document.addEventListener("input", e => { if (e.target.id === "pr-q") { PR.q = e.target.value; const p = e.target.selectionStart; render(); const n = $("#pr-q"); if (n) { n.focus(); n.setSelectionRange(p, p); } } });
document.addEventListener("change", async e => {
  const el = e.target.closest && e.target.closest(".ipe"); if (!el) return;
  const c = el.dataset.c, d = state.reg[c], row = d && d.rows.find(x => x.id === el.dataset.id); if (!row) return;
  try { state.reg[c] = await api("/api/reg/" + c, {method: "POST", body: JSON.stringify({...row, [el.dataset.k]: el.value.trim()})}); toast("Saved."); } catch (err) { toast(err.message || "Could not save.", true); }
  render();
});
{
  const nav = ACT.nav; ACT.nav = (el, e) => { if (["parcheck", "parsheets", "ordpars", "orddays", "parnotes", "brk", "brkrev"].includes(el.dataset.v)) { PR.chk = null; ["parsheets", "parnotes", "orderpars", "orderdays", "breakage", "breakrev"].forEach(k => delete state.reg[k]); } return nav(el, e); };
  const sv = ACT["reg-save"]; ACT["reg-save"] = async (el, e) => { const r = await sv(el, e); PR.chk = null; return r; };
}
