"use strict";

/* Dashboard: the home screen. Half-circle gauges and number tiles in fixed-size blocks. */

async function loadDash() {
  try { state.dash = await api("/api/dashboard"); state.dashAt = new Date(); }
  catch (e) { state.dash = state.dash || {error: e.message}; }
  if (state.view === "dash") render();
}

const gaugeColor = p => p < 0 ? "#7d828b" : p >= 90 ? "#58b368" : p >= 70 ? "#e7b416" : "#e0525e";

// A half circle that fills left to right. -1 means the thing is not set up yet.
function gaugeSvg(p) {
  const pct = p < 0 ? 0 : Math.max(0, Math.min(100, p)), c = gaugeColor(p), arc = "M20 100 A80 80 0 0 1 180 100";
  return `<svg viewBox="0 0 200 112" class="gsvg" aria-hidden="true"><path d="${arc}" fill="none" stroke="#24262b" stroke-width="18" stroke-linecap="round"/>`
    + (pct > 0 ? `<path d="${arc}" fill="none" stroke="${c}" stroke-width="18" stroke-linecap="round" pathLength="100" stroke-dasharray="${pct} 100"/>` : "")
    + `<text x="100" y="92" text-anchor="middle" class="gnum" fill="${c}">${p < 0 ? "Set up" : p + "%"}</text></svg>`;
}

const DASH_VIEW = {"Cleaning": "cltoday", "Equipment PM": "pm", "Grounds / Facility": "fac", "Issue": "issues", "Inspection": "insp", "Temperature": "temps", "Corrective Action": "calog", "Manager Check": "mgrchk", "SDS": "library", "Equipment": "equip", "LOTO": "llog", "Binder": "blog", "Incident": "inc", "Other": "todo"};

function dBlock(cls, inner, view) { return `<div class="dblock ${cls}" ${view ? `data-act="nav" data-v="${esc(view)}" role="button" tabindex="0"` : ""}>${inner}</div>`; }

function vDash() {
  const d = state.dash;
  if (!d) { loadDash(); return `<div class="empty"><b>Loading the dashboard...</b></div>`; }
  if (d.error) return `<div class="empty"><b>The dashboard could not load</b>${esc(d.error)}<br><br><button class="btn" data-act="dash-refresh">Try again</button></div>`;
  const gauges = d.gauges.map(g => dBlock("g", `<div class="dlabel">${esc(g.label)}</div>${gaugeSvg(g.pct)}<div class="dsub">${esc(g.sub)}</div>`, g.view)).join("");
  const mk = t => dBlock("t " + (t.tone || ""), `<div class="dlabel">${esc(t.label)}</div><div class="dval">${esc(t.value)}</div><div class="dsub">${esc(t.sub)}</div>`, t.view);
  const tilesOnly = d.tiles.filter(t => t.key !== "backup" && t.label !== "Last Backup").map(mk).join(""), backupTile = d.tiles.filter(t => t.key === "backup" || t.label === "Last Backup").map(mk).join("");
  const tiles = d.tiles.map(t => dBlock("t " + (t.tone || ""), `<div class="dlabel">${esc(t.label)}</div><div class="dval">${esc(t.value)}</div><div class="dsub">${esc(t.sub)}</div>`, t.view)).join("");
  const att = d.attention.length ? d.attention.map(a => `<div class="drow ${a.state === "Overdue" ? "od" : "td"}" data-act="nav" data-v="${esc(DASH_VIEW[a.group] || "todo")}"><span class="status ${a.state === "Overdue" ? "Overdue" : "Soon"}">${a.state === "Overdue" ? "Overdue" : "Today"}</span><b title="${esc(a.title)}">${esc(a.title)}</b><span class="hint">${esc(a.group)}${a.detail ? " · " + esc(a.detail) : ""}</span></div>`).join("")
    : `<div class="empty" style="margin:10px"><b>Nothing overdue or due today</b>Everything the program tracks is up to date.</div>`;
  const when = s => { const t = new Date(String(s).replace(" ", "T")); return isNaN(t) ? s : t.toLocaleString([], {month: "short", day: "numeric", hour: "numeric", minute: "2-digit"}); };
  const notes = d.notes.length ? d.notes.map(n => `<div class="drow" data-act="nav" data-v="notes" style="align-items:flex-start"><b>${esc(n.by || "A manager")}</b><span class="hint" style="white-space:normal">${esc(when(n.at))}</span><span style="white-space:normal;flex:1">${esc(n.text)}</span></div>`).join("")
    : `<div class="empty" style="margin:10px"><b>No open handoff notes</b></div>`;
  const recent = d.recent.length ? d.recent.map(h => `<div class="drow" style="cursor:default"><span class="hint" style="flex:none">${esc(when(h.at))}</span><span style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap"><b style="max-width:none">${esc(h.who || "Someone")}</b> ${esc(h.action.toLowerCase())} ${esc(h.kind)}${h.label ? " · " + esc(h.label) : ""}</span></div>`).join("")
    : `<div class="empty" style="margin:10px"><b>No changes recorded yet</b></div>`;
  const act = (a, ic, l, extra) => `<button class="btn" data-act="${a}" ${extra || ""}>${icon(ic)}${l}</button>`;
  const GI = {ready: "shield", temps: "temp", clean: "drop", pm: "wrench", train: "users", stchk: "check"};
  const gauges2 = d.gauges.map(g => dBlock("g", `<div class="dlabel"><span class="dic">${icon(GI[g.key] || "list")}</span>${esc(g.label)}</div>${gaugeSvg(g.pct)}<div class="dsub">${esc(g.sub)}</div>`, g.view)).join("");
  const SC = [["equip", "wrench", "Maintenance", "cb"], ["cltoday", "drop", "Cleaning & Sanitation", "cg"], ["temps", "temp", "Food Safety", "ca"], ["walk", "check", "Readiness Walk", "cg"], ["guides", "book", "Station Guides", "cb"], ["stchk", "list", "Station Checklists", "cg"],
    ["trmatrix", "users", "Training", "cb"], ["library", "flask", "SDS & Chemicals", "ca"], ["llog", "lock", "Lockout / Tagout", "cr"], ["bindex", "siren", "Emergency Binder", "cr"], ["calog", "warn", "Corrective Actions", "ca"], ["reports", "chart", "Reports", "cb"]];
  const short = SC.map(([v, ic, l, c]) => `<div class="dblock s ${c}" data-act="nav" data-v="${v}" role="button" tabindex="0"><span class="sic">${icon(ic)}</span><span>${esc(l)}</span></div>`).join("");
  return `<div class="row noprint" style="justify-content:space-between"><span class="hint">${esc(new Date().toLocaleDateString([], {weekday: "long", month: "long", day: "numeric"}))} · updated ${esc((state.dashAt || new Date()).toLocaleTimeString([], {hour: "numeric", minute: "2-digit"}))}</span><button class="btn small" data-act="dash-refresh">${icon("swap")}Refresh</button></div>
  <div class="dgrid">${gauges2}${tilesOnly}
    <div class="dblock w2 r2"><div class="dhead"><span>Quick Actions</span></div><div class="dqa">
      ${act("problem-new", "warn", "Report a Problem")}${act("temp-enter", "temp", "Enter Temperatures")}${act("nav", "drop", "Today's Cleaning", 'data-v="cltoday"')}${act("todo-new", "list", "Create To-Do List")}${act("rep-view", "print", "Weekly Review", 'data-id="weekly-review"')}${act("nav", "list", "Shift Handoff", 'data-v="notes"')}</div></div>
    <div class="dblock wall r4"><div class="dhead"><span>Needs Attention</span><span class="hint">${d.overdue} overdue · ${d.dueToday} due today</span></div><div class="dlist">${att}</div></div>
    ${short}
    <div class="dblock w2 r3"><div class="dhead"><span>Shift Handoff</span><span class="hint">${d.notes.length} open note${d.notes.length === 1 ? "" : "s"}</span></div><div class="dlist">${notes}</div></div>
    ${backupTile}
    <div class="dblock w3 r3"><div class="dhead"><span>Recent Changes</span><a href="#" class="hint" data-act="hist-open-tab">Full history</a></div><div class="dlist">${recent}</div></div>
  </div>`;
}

Object.assign(ACT, {
  async "dash-refresh"() { await loadDash(); },
  "hist-open-tab"(el, e) { if (e) e.preventDefault(); state.tab = "history"; state.view = "settings"; render(); loadTab(); },
});

function tickClock() { const d = new Date(), el = document.getElementById("clock"); if (el) el.innerHTML = `<b>${d.toLocaleDateString([], {weekday: "short", month: "short", day: "numeric", year: "numeric"})}</b><span>${d.toLocaleTimeString([], {hour: "numeric", minute: "2-digit"})}</span>`; }
tickClock(); setInterval(tickClock, 15000);
