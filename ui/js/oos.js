"use strict";

/* Out-of-service tags */

function oosBanner() {
  const m = state.maint; if (!m) return "";
  const out = m.equipment.filter(e => e.status === "Out of Service"); if (!out.length) return "";
  const rec = id => [...(m.oosLog || [])].reverse().find(r => r.equipmentId === id && !r.clearedAt) || {};
  return `<div class="note bad oos noprint" style="margin-bottom:10px"><span class="grow"><b>OUT OF SERVICE. DO NOT USE:</b> ${out.map(e => { const r = rec(e.id); return `<b>${esc(e.name)}</b>${r.reason ? " (" + esc(r.reason) + (r.removedAt ? ", since " + fmtDate(r.removedAt) : "") + ")" : ""}`; }).join(" · ")}</span>
    <button class="btn small" data-act="nav" data-v="equip">View Equipment</button></div>`;
}

const TAGPOS = {  // where each field sits on the tag picture (1536 x 1024)
  box: {equipment: [332, 372, 780, 417], assetId: [1094, 372, 1514, 417], location: [332, 427, 780, 469], when: [1094, 427, 1514, 469], reason: [369, 482, 1514, 527],
    workOrder: [428, 668, 778, 711], vendor: [1090, 668, 1514, 711], by: [211, 723, 778, 764]},
  check: {safety: {Yes: [432, 569], No: [580, 569]}, foodSafety: {Yes: [1193, 569], No: [1347, 569]}, powerOff: {Yes: [432, 626], No: [580, 626]}, lockout: {Yes: [1193, 626], No: [1347, 626]},
    tagPriority: {High: [1006, 744], Medium: [1176, 744], Low: [1390, 744]}},
};

const activeHold = id => [...(state.maint.oosLog || [])].reverse().find(r => r.equipmentId === id && !r.clearedAt);

const when12 = at => { if (!at) return ""; const t = new Date(at.replace(" ", "T")); return isNaN(t) ? fmtDate(at) : t.toLocaleString("en-US", {month: "numeric", day: "numeric", year: "numeric", hour: "numeric", minute: "2-digit"}); };

async function drawTag(id) {
  const e = state.maint.equipment.find(x => x.id === id), r = activeHold(id);
  const img = new Image(); img.src = "/oos-tag.png"; await img.decode();
  const c = document.createElement("canvas"); c.width = img.naturalWidth; c.height = img.naturalHeight;
  const g = c.getContext("2d"); g.drawImage(img, 0, 0); g.fillStyle = "#111"; g.textBaseline = "middle";
  const text = {equipment: e.name, assetId: r.assetId, location: r.location || e.area, when: when12(r.removedAt), reason: r.reason, workOrder: r.workOrder, vendor: r.vendor, by: r.removedBy};
  for (const [k, [x0, y0, x1, y1]] of Object.entries(TAGPOS.box)) {
    let s = String(text[k] || "").replace(/\s+/g, " ").trim(); if (!s) continue;
    let size = 30; const maxW = x1 - x0 - 24;
    for (; size >= 17; size -= 1) { g.font = `600 ${size}px "Segoe UI", Arial, sans-serif`; if (g.measureText(s).width <= maxW) break; }
    while (g.measureText(s).width > maxW && s.length > 4) s = s.slice(0, -2).trimEnd() + "…";
    g.fillText(s, x0 + 12, (y0 + y1) / 2 + 1);
  }
  g.strokeStyle = "#111"; g.lineWidth = 6; g.lineCap = "round";
  for (const [k, opts] of Object.entries(TAGPOS.check)) { const pos = opts[r[k]]; if (!pos) continue;
    const [x, y] = pos; g.beginPath(); g.moveTo(x - 9, y - 9); g.lineTo(x + 9, y + 9); g.moveTo(x + 9, y - 9); g.lineTo(x - 9, y + 9); g.stroke(); }
  return new Promise(res => c.toBlob(res, "image/png"));
}

async function makeTag(id) {
  const blob = await drawTag(id);
  const resp = await fetch("/api/maint/equipment/" + id + "/tag", {method: "POST", headers: {"Content-Type": "image/png"}, body: blob});
  const o = await resp.json(); if (!resp.ok) throw new Error(o.error || "The tag could not be saved.");
  state.maint = o.payload; return o;
}

const tagPrint = file => window.open("/api/oos-tags/" + encodeURIComponent(file) + "?print=1", "_blank");

function tagDone(o) {
  showModal(`<h2>${icon("print")}Out-of-Service Tag Ready</h2><div class="body" id="tgbody" data-file="${esc(o.file)}"><p>The tag was saved as <b>${esc(o.file)}</b> in the folder:<br><span class="hint">${esc(o.folder)}</span></p>
    <p style="text-align:center"><img src="/api/oos-tags/${encodeURIComponent(o.file)}" style="max-width:100%;max-height:340px;border:1px solid var(--line)"></p>
    <p class="hint">Print it now and attach it to the equipment. If you print later, use Print Tag on the equipment list.</p>
    <p class="hint">A line was also added to the LOTO Log${state.lotoTag ? " (" + esc(state.lotoTag) + ")" : ""}.</p></div>
    <div class="foot"><button class="btn green big" data-act="tag-print-now">${icon("print")}Print Now</button>${state.lotoTag ? `<button class="btn big" data-act="rep-view" data-id="lototag:${esc(state.lotoTag)}">${icon("print")}Print LOTO Tag</button>` : ""}<button class="btn big" data-act="close">Print Later</button></div>`);
}

function tagFields(id, r, issue) {
  const m = state.maint, asset = r.assetId || ((m.pm || []).find(t => t.equipmentId === id && t.assetCode) || {}).assetCode || (m.equipment.find(x => x.id === id) || {}).maintainxId || "";
  const pr = r.tagPriority || (issue ? (/^P[12]/.test(issue.priority) ? "High" : /^P3/.test(issue.priority) ? "Medium" : "Low") : "High");
  return `${yesNoRadio("safety", "Safety hazard?", r.safety)}${yesNoRadio("foodSafety", "Food safety risk?", r.foodSafety)}${yesNoRadio("powerOff", "Power / gas disconnected?", r.powerOff)}${yesNoRadio("lockout", "Lockout / tagout used?", r.lockout)}
    <label>Priority:</label><div class="row">${["High", "Medium", "Low"].map(v => `<label style="font-weight:400"><input type="radio" name="oos_tagPriority" value="${v}" ${pr === v ? "checked" : ""}> ${v}</label>`).join("")}</div>
    <label>Location / area:</label><input class="field" data-oos="location" value="${esc(r.location || (m.equipment.find(x => x.id === id) || {}).area || "")}" placeholder="Where the equipment is">
    <label>Asset ID:</label><input class="field" data-oos="assetId" value="${esc(asset)}" placeholder="Optional, e.g. K-007">
    <label>Work order / MaintainX ref:</label><input class="field" data-oos="workOrder" value="${esc(r.workOrder || (issue && issue.workOrder) || "")}" placeholder="Optional">
    <label>Vendor / technician:</label><input class="field" data-oos="vendor" value="${esc(r.vendor || (issue && issue.assignedTo) || "")}" placeholder="Optional">`;
}

const collectTag = () => { const v = grab("[data-oos]", "oos"); for (const k of ["safety", "foodSafety", "powerOff", "lockout", "tagPriority"]) v[k] = (document.querySelector(`[name=oos_${k}]:checked`) || {}).value || ""; return v; };

function oosAsk(id) {
  const e = state.maint.equipment.find(x => x.id === id);
  const open = state.maint.issues.filter(p => p.equipmentId === id && p.status !== "Verified" && p.status !== "Closed");
  showModal(`<h2>${icon("warn")}Take Out of Service</h2><div class="body" id="oosbody" data-id="${id}"><p><b>${esc(e.name)}</b> will show as OUT OF SERVICE at the top of every screen until a manager clears it. An out-of-service tag is made for you to print.</p><div class="fields">
    <label>Reason: <span class="req">*</span></label><textarea class="field" rows="3" data-oos="reason" placeholder="Why it must not be used, e.g. Holding 48°F, oil leak at the drain valve"></textarea>
    <label>Related issue:</label><select class="field" data-oos="issueId" id="oosissue"><option value="">None</option>${open.map(p => `<option value="${p.id}">#${p.number} ${esc(p.title)}</option>`).join("")}</select>
    ${tagFields(id, {}, open[0])}
    <label>Tagged by: <span class="req">*</span></label><input class="field" data-oos="by" value="${esc(who())}"></div></div>
    <div class="foot"><button class="btn wine big" data-act="oos-go">${icon("warn")}Take Out of Service</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

function tagDetails(id) {  // for equipment already out of service that has no tag yet
  const e = state.maint.equipment.find(x => x.id === id), r = activeHold(id) || {}, issue = state.maint.issues.find(p => p.id === r.issueId);
  showModal(`<h2>${icon("print")}Out-of-Service Tag</h2><div class="body" id="oosbody" data-id="${id}"><p><b>${esc(e.name)}</b>${r.reason ? "<br><span class='hint'>Reason: " + esc(r.reason) + "</span>" : ""}</p><div class="fields">${tagFields(id, r, issue)}</div></div>
    <div class="foot"><button class="btn green big" data-act="tag-make">${icon("print")}Make the Tag</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

function oosClearAsk(id) {
  const m = state.maint, e = m.equipment.find(x => x.id === id), r = [...(m.oosLog || [])].reverse().find(x => x.equipmentId === id && !x.clearedAt) || {};
  const open = m.issues.filter(p => p.equipmentId === id && p.status !== "Verified" && p.status !== "Closed");
  showModal(`<h2>${icon("save")}Clear for Service</h2><div class="body" id="oosbody" data-id="${id}"><p><b>${esc(e.name)}</b> has been out of service${r.removedAt ? " since " + fmtDate(r.removedAt) : ""}.<br><span class="hint">${r.reason ? "Reason: " + esc(r.reason) + (r.removedBy ? " (" + esc(r.removedBy) + ")" : "") : "The reason was not recorded."}</span></p>
    ${open.length ? `<p class="note bad">${open.length} work order${open.length === 1 ? " is" : "s are"} still open on this equipment.</p>` : ""}
    <div class="fields"><label>What was checked or repaired: <span class="req">*</span></label><textarea class="field" rows="3" data-oc="note" placeholder="e.g. Seal replaced, ran 2 hours, holding 38°F"></textarea>
    <label></label><label style="font-weight:400"><input type="checkbox" data-oc="safe"> I have checked this equipment and it is safe for food production</label>
    <label>Manager name: <span class="req">*</span></label><input class="field" data-oc="by" value="${esc(who())}" placeholder="${(m.managers || []).length ? "Must be on the authorized list" : ""}"></div>
    ${(m.managers || []).length ? `<p class="hint">Authorized managers: ${esc(m.managers.join(", "))}</p>` : `<p class="hint">Anyone can clear this until you list authorized managers in Settings.</p>`}</div>
    <div class="foot"><button class="btn green big" data-act="oos-clear-go">${icon("save")}Clear for Service</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

Object.assign(ACT, {
  "oos-ask"(el) { oosAsk(el.dataset.id); },
  "oos-clear-ask"(el) { oosClearAsk(el.dataset.id); },
  async "oos-go"() { const v = collectTag(); rememberWho(v.by); if (!v.reason.trim() || !v.by.trim()) return toast("Enter the reason and your name.", true);
    const id = $("#oosbody").dataset.id; state.maint = await api("/api/maint/equipment/" + id + "/out-of-service", {method: "POST", body: JSON.stringify(v)}); state.sum = null; delete state.reg.lotolog; render();
    try { const lg = (await api("/api/reg/lotolog")).rows.find(x => x.equipment === (state.maint.equipment.find(q => q.id === id) || {}).name && !x.removed); state.lotoTag = lg ? lg.id : ""; } catch (e) { state.lotoTag = ""; }
    try { tagDone(await makeTag(id)); } catch (err) { closeModal(); toast("Out of service, but the tag could not be made: " + err.message + " Use Print Tag on the equipment list.", true); } },
  async "tag-make"() { const id = $("#oosbody").dataset.id, v = collectTag(); delete v.reason;
    state.maint = await api("/api/maint/equipment/" + id + "/tag-details", {method: "POST", body: JSON.stringify(v)});
    try { tagDone(await makeTag(id)); } catch (err) { toast(err.message, true); } render(); },
  "tag-print-now"() { tagPrint($("#tgbody").dataset.file); },
  "tag-print"(el) { const r = activeHold(el.dataset.id); if (r && r.tag) return tagPrint(r.tag); tagDetails(el.dataset.id); },
  async "oos-clear-go"() { const v = grab("[data-oc]", "oc"); rememberWho(v.by); await maintDone(await api("/api/maint/equipment/" + $("#oosbody").dataset.id + "/clear", {method: "POST", body: JSON.stringify(v)}), "Cleared for service."); },
});
