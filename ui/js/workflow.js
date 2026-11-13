"use strict";

/* Daily workflow checklist, and the 90-day goals tracker (under Maintenance) */

const WF = {day: null, g90: null};

async function loadWf(date) { try { WF.day = await api("/api/dw/today" + (date ? "?date=" + date : "")); } catch (e) { WF.day = {error: e.message}; } render(); }
async function loadG90() { try { WF.g90 = await api("/api/g90"); } catch (e) { WF.g90 = {error: e.message}; } render(); }

const wfMin = t => { const m = /^(\d{1,2}):(\d{2})$/.exec(t || ""); return m ? +m[1] * 60 + +m[2] : null; };
function wfCurrent(b, isToday) {
  if (!isToday) return false;
  const a = wfMin(b.start), z = wfMin(b.end), n = new Date().getHours() * 60 + new Date().getMinutes();
  return a !== null && z !== null && n >= a && n < z;
}

function vDwToday() {
  const d = WF.day;
  if (!d) { loadWf(); return `<div class="empty"><b>Loading...</b></div>`; }
  if (d.error) return `<div class="empty"><b>Could not load</b>${esc(d.error)}</div>`;
  const isToday = d.date === new Date().toLocaleDateString("sv"), pct = d.total ? Math.round(d.done / d.total * 100) : 0;
  const blocks = d.blocks.map(b => {
    const left = b.items.filter(i => !i.done).length, cur = wfCurrent(b, isToday);
    return `<div class="panel wfblock ${cur ? "wfnow" : ""}" style="max-width:980px">
      <div class="row" style="justify-content:space-between"><h3 style="margin:0">${esc(b.block)}${cur ? ` <span class="status Upcoming">Now</span>` : ""}</h3><span class="hint">${left ? left + " left" : "All done"}</span></div>
      ${b.items.map(i => `<label class="wfitem ${i.done ? "done" : ""}"><input type="checkbox" data-act="dw-tick" data-id="${esc(i.id)}" ${i.done ? "checked" : ""}><span class="grow">${esc(i.text)}${i.freq !== "Daily" ? ` <span class="hint">(${esc(i.freq.toLowerCase())})</span>` : ""}</span>${i.done ? `<span class="hint">${esc(i.by)} ${esc(i.time)}</span>` : ""}</label>`).join("")}</div>`;
  }).join("");
  return `<div class="panel" style="max-width:980px"><div class="row" style="justify-content:space-between">
      <div><h3 style="margin:0">${esc(fmtDate(d.date))}</h3><div class="hint">${d.done} of ${d.total} checks done${d.openHandoff ? ` · ${d.openHandoff} open handoff item${d.openHandoff === 1 ? "" : "s"}` : ""}</div></div>
      <div class="row"><label style="margin:0">Your name:</label><input id="dwby" class="field" style="width:160px" value="${esc(remembered("by"))}">
      <input type="date" id="dwdate" class="field" style="width:160px" value="${esc(d.date)}"><button class="btn small" data-act="dw-date">Go</button>
      <button class="btn small" data-act="rep-view" data-id="dw-today">${icon("print")}Print</button></div></div>
    <div class="wfbar"><div style="width:${pct}%"></div></div></div>${blocks}`;
}

function vG90() {
  const g = WF.g90;
  if (!g) { loadG90(); return `<div class="empty"><b>Loading...</b></div>`; }
  if (g.error) return `<div class="empty"><b>Could not load</b>${esc(g.error)}</div>`;
  const done = g.counts.Done || 0, pct = g.total ? Math.round(done / g.total * 100) : 0;
  const head = g.start
    ? (g.day ? `<b>Day ${Math.min(g.day, 90)} of 90</b> · Week ${g.week}${g.phase >= 0 ? " · " + esc(g.phases[g.phase]) : ""}` : `Starts ${esc(fmtDate(g.start))}`)
    : `Set the first day of the 90 days to track the weeks.`;
  const stc = {"Not Started": "Open", "In Progress": "Upcoming", "Blocked": "Overdue", "Done": "Done"};
  const goals = g.goals.map(x => `<div class="catrow wfgoal"><div class="grow"><b>${esc(x.order)}. ${esc(x.title)}</b><div class="hint">${esc(x.scope)}</div>
      <div class="hint">${x.owner ? "Owner: " + esc(x.owner) : "No owner yet"}${x.target ? " · Target " + esc(shortDate(x.target)) : ""}${x.nextAction ? " · Next: " + esc(x.nextAction) : ""}</div>${x.evidence ? `<div class="hint">Evidence: ${esc(x.evidence)}</div>` : ""}</div>
      <span class="status ${stc[x.status] || "Open"}">${esc(x.status)}</span><button class="btn small" data-act="reg-open" data-c="goals90" data-id="${esc(x.id)}">${icon("edit")}Edit</button></div>`).join("");
  const obj = g.objectives.map(o => `<div class="catrow"><div class="grow">${esc(o.objective)}${o.notes ? `<div class="hint">${esc(o.notes)}</div>` : ""}</div><span class="status ${o.rating === "On Track" ? "Done" : o.rating === "Behind" ? "Overdue" : "Upcoming"}">${esc(o.rating)}</span><button class="btn small" data-act="reg-open" data-c="obj90" data-id="${esc(o.id)}">${icon("edit")}Edit</button></div>`).join("");
  const revs = g.reviews.length ? g.reviews.map(r => `<div class="catrow"><div class="grow"><b>${esc(fmtDate(r.date))}</b><div class="hint">Done: ${esc(r.done) || "-"}</div><div class="hint">Outstanding: ${esc(r.outstanding) || "-"}</div>${r.decision ? `<div class="hint">Decision needed: ${esc(r.decision)}</div>` : ""}</div><button class="btn small" data-act="reg-open" data-c="rev90" data-id="${esc(r.id)}">${icon("edit")}Edit</button></div>`).join("") : `<p class="hint">No weekly reviews yet.</p>`;
  return `<div class="panel" style="max-width:980px"><div class="row" style="justify-content:space-between"><div><div>${head}</div><div class="hint">${done} of ${g.total} goals done${g.counts.Blocked ? " · " + g.counts.Blocked + " blocked" : ""}</div></div>
      <div class="row"><label style="margin:0">First day:</label><input type="date" id="g90start" class="field" style="width:160px" value="${esc(g.start)}"><button class="btn small" data-act="g90-start">Save</button><button class="btn small" data-act="rep-view" data-id="g90">${icon("print")}Print</button></div></div>
    <div class="wfbar"><div style="width:${pct}%"></div></div>${g.reviewedThisWeek ? "" : `<div class="note warn" style="margin-top:8px">No weekly review has been written this week.</div>`}</div>
  <div class="panel" style="max-width:980px"><h3 style="margin-top:0">Nine signed short-term goals</h3>${goals}</div>
  <div class="panel" style="max-width:980px"><div class="row" style="justify-content:space-between"><h3 style="margin:0">Weekly progress review</h3><button class="btn green" data-act="reg-new" data-c="rev90">${icon("plus")}Add Weekly Review</button></div>${revs}</div>
  <div class="panel" style="max-width:980px"><h3 style="margin-top:0">Leadership scorecard</h3>${obj}</div>`;
}

Object.assign(ACT, {
  async "dw-tick"(el) {
    const by = ($("#dwby") || {}).value || ""; remembered("by", by);
    try { await api("/api/dw/tick", {method: "POST", body: JSON.stringify({date: WF.day.date, itemId: el.dataset.id, by, on: el.checked})}); await loadWf(WF.day.date); state.today = null; } catch (e) { toast(e.message, true); loadWf(WF.day.date); }
  },
  "dw-date"() { const v = $("#dwdate").value; if (v) loadWf(v); },
  async "g90-start"() { try { WF.g90 = await api("/api/g90/start", {method: "POST", body: JSON.stringify({start: $("#g90start").value})}); render(); toast("Saved."); } catch (e) { toast(e.message, true); } },
});
{
  const nav = ACT.nav; ACT.nav = (el, e) => { if (["dwtoday", "g90"].includes(el.dataset.v)) { WF.day = null; WF.g90 = null; } return nav(el, e); };
  const sv = ACT["reg-save"]; ACT["reg-save"] = async (el, e) => { const r = await sv(el, e); WF.day = null; WF.g90 = null; return r; };
}
