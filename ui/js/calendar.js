"use strict";

/* Calendar */

const GSLUG = {"Cleaning": "c", "Equipment PM": "p", "Grounds / Facility": "f", "Issue": "i", "Inspection": "i", "Temperature": "i", "Corrective Action": "i", "Manager Check": "i", "Other": "o"};

const GFILTERS = [["all", "All"], ["Cleaning", "Cleaning"], ["Equipment PM", "Equipment PM"], ["Grounds / Facility", "Grounds / Facility"], ["Other", "Other Events"], ["Issue", "Issues"]];

const FFILTERS = [["all", "Any Frequency"], ["Daily", "Daily"], ["Weekly", "Weekly"], ["Monthly", "Monthly"], ["Quarterly", "Quarterly"], ["Semi-Annually", "Semi-Annually"], ["Yearly", "Yearly"], ["As Needed", "As Needed"]];
state.cal = {mode: "month", ref: new Date(), group: "all", freq: "all", events: null, key: ""};

function calRange() {
  const c = state.cal, y = c.ref.getFullYear(), m = c.ref.getMonth(), ref = new Date(y, m, c.mode === "month" || c.mode === "list" ? 1 : c.ref.getDate());
  if (c.mode === "day") { const d = new Date(y, m, c.ref.getDate()); return [d, d]; }
  if (c.mode === "week") { const s0 = addDays(ref, -ref.getDay()); return [s0, addDays(s0, 6)]; }
  if (c.mode === "list") return [ref, new Date(y, m + 1, 0)];
  if (c.mode === "quarter") { const q = m - m % 3; return [new Date(y, q, 1), new Date(y, q + 3, 0)]; }
  if (c.mode === "semi") { const q = m - m % 6; return [new Date(y, q, 1), new Date(y, q + 6, 0)]; }
  if (c.mode === "year") return [new Date(y, 0, 1), new Date(y, 11, 31)];
  const s0 = addDays(ref, -ref.getDay()); return [s0, addDays(s0, 41)];
}

async function loadCal(force) {
  const [a, b] = calRange(), key = ymd(a) + ymd(b); if (!force && state.cal.key === key && state.cal.events) return;
  if (!state.maint) state.maint = await api("/api/maint");
  state.cal.key = key; const r0 = await api(`/api/maint/calendar?from=${ymd(a)}&to=${ymd(b)}`); state.cal.events = r0.events; if (state.view === "cal") render();
}

const calShown = () => (state.cal.events || []).filter(e => (state.cal.group === "all" || e.group === state.cal.group) && (state.cal.freq === "all" || e.bucket === state.cal.freq || (e.bucket === "Quarterly / Semi-Annual" && (state.cal.freq === "Quarterly" || state.cal.freq === "Semi-Annually"))));

const evChip = (e, full) => `<div class="ev ${GSLUG[e.group]} ${e.state === "Overdue" ? "od" : ""} ${e.kind === "done" ? "dn" : ""}" title="${esc(e.group)}: ${esc(e.title)}${e.unit ? " (" + esc(e.unit) + ")" : ""}${e.bucket ? " · " + e.bucket : ""}">${e.kind === "done" ? "✓ " : e.kind === "issue" ? "⚠ " : ""}${esc(e.title)}${full && (e.area || e.unit) ? `<small> ${esc(e.area || e.unit)}</small>` : ""}</div>`;

function vCal() {
  const c = state.cal; if (!c.events || state.cal.key !== ymd(calRange()[0]) + ymd(calRange()[1])) { loadCal().catch(e => toast(e.message, true)); if (!c.events) return `<p class="hint">Loading...</p>`; }
  const [a, b] = calRange(), today = ymd(new Date()), shown = calShown(), by = {};
  for (const e of shown) (by[e.date] = by[e.date] || []).push(e);
  const label = c.mode === "day" ? c.ref.toLocaleDateString("en-US", {weekday: "long", month: "long", day: "numeric", year: "numeric"}) : c.mode === "quarter" ? `Q${Math.floor(a.getMonth() / 3) + 1} ${a.getFullYear()}` : c.mode === "semi" ? `${a.getMonth() < 6 ? "First" : "Second"} half of ${a.getFullYear()}` : c.mode === "year" ? String(a.getFullYear()) : c.mode === "week" ? `${a.toLocaleDateString("en-US", {month: "short", day: "numeric"})} – ${b.toLocaleDateString("en-US", {month: "short", day: "numeric", year: "numeric"})}` : c.ref.toLocaleDateString("en-US", {month: "long", year: "numeric"});
  const chips = (list, cur, act) => list.map(([k, l]) => `<button class="chip ${cur === k ? "on" : ""}" data-act="${act}" data-v="${esc(k)}">${l}</button>`).join("");
  let grid;
  if (c.mode === "day") {
    const ev = by[ymd(a)] || [];
    grid = `<div class="agenda">${ev.length ? ev.map(evRow).join("") : `<div class="empty"><b>Nothing scheduled</b>Nothing matches these filters today. Use Add Event to put something on this day.</div>`}</div>`;
  } else if (c.mode === "quarter" || c.mode === "semi" || c.mode === "year") {
    const months = c.mode === "quarter" ? 3 : c.mode === "semi" ? 6 : 12;
    grid = `<div class="mmwrap">${Array.from({length: months}, (_, i) => { const first = new Date(a.getFullYear(), a.getMonth() + i, 1), last = new Date(first.getFullYear(), first.getMonth() + 1, 0), cells = [];
      for (let k = 0; k < first.getDay(); k++) cells.push(`<div class="day" style="cursor:default;background:#1f2125"></div>`);
      for (let d = 1; d <= last.getDate(); d++) { const k = ymd(new Date(first.getFullYear(), first.getMonth(), d)), ev = by[k] || [], od = ev.filter(e => e.state === "Overdue").length;
        cells.push(`<div class="day ${k === today ? "today" : ""}" data-act="cal-day" data-d="${k}"><div class="dn">${d}</div>${ev.length ? `<span class="badge ${od ? "od" : ""}" title="${ev.length} scheduled${od ? ", " + od + " overdue" : ""}">${ev.length}</span>` : ""}</div>`); }
      return `<div class="mm"><h4>${first.toLocaleDateString("en-US", {month: "long", year: "numeric"})}</h4><div class="calgrid">${["S", "M", "T", "W", "T", "F", "S"].map(x => `<div class="dow">${x}</div>`).join("")}${cells.join("")}</div></div>`; }).join("")}</div>`;
  } else if (c.mode === "list") {
    const days = Object.keys(by).sort();
    grid = days.length ? days.map(d => `<div class="agenda"><b>${new Date(d + "T12:00").toLocaleDateString("en-US", {weekday: "long", month: "long", day: "numeric"})}</b>${by[d].map(e => evChip(e, true)).join("")}</div>`).join("") : `<div class="empty"><b>Nothing scheduled</b>Nothing matches these filters this month.</div>`;
  } else {
    const n = c.mode === "week" ? 7 : 42, cells = [];
    for (let i = 0; i < n; i++) { const d = addDays(a, i), k = ymd(d), ev = by[k] || [], max = c.mode === "week" ? 99 : 3;
      cells.push(`<div class="day ${k === today ? "today" : ""} ${c.mode === "month" && d.getMonth() !== c.ref.getMonth() ? "other" : ""} ${c.mode === "week" ? "tall" : ""}" data-act="cal-day" data-d="${k}"><div class="dn">${d.getDate()}</div>${ev.slice(0, max).map(e => evChip(e, c.mode === "week")).join("")}${ev.length > max ? `<div class="more">+${ev.length - max} more</div>` : ""}</div>`); }
    grid = `<div class="calgrid">${["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"].map(x => `<div class="dow">${x}</div>`).join("")}${cells.join("")}</div>`;
  }
  return `<div class="row calbar"><button class="btn small" data-act="cal-nav" data-d="-1">${icon("back")}</button><b style="min-width:190px;text-align:center;font-size:17px">${label}</b><button class="btn small" data-act="cal-nav" data-d="1" style="transform:scaleX(-1)">${icon("back")}</button><button class="btn small" data-act="cal-nav" data-d="0">Today</button><button class="btn small green" data-act="cal-event-new" data-d="">${icon("plus")}Add Event</button>
    <span class="grow"></span><div class="chips">${chips([["day", "Day"], ["week", "Week"], ["month", "Month"], ["quarter", "Quarter"], ["semi", "Semi-Annual"], ["year", "Year"], ["list", "List"]], c.mode, "cal-mode")}</div></div>
  <div class="row calbar"><span class="hint">Show:</span><div class="chips">${chips(GFILTERS, c.group, "cal-group")}</div></div>
  <div class="row calbar"><span class="hint">How often:</span><div class="chips">${chips(FFILTERS, c.freq, "cal-freq")}</div>
    <span class="grow"></span><span class="legend"><i class="c"></i>Cleaning <i class="p"></i>Equipment PM <i class="f"></i>Facility <i class="i"></i>Issue <i class="od"></i>Overdue</span></div>
  <div class="tablewrap" style="padding:8px">${grid}</div>`;
}

const evRow = e => `<div class="catrow"><div class="grow">${evChip(e, false)}<div class="hint">${esc(e.group)}${e.bucket ? " · " + e.bucket : ""}${e.area ? " · " + esc(e.area) : ""}${e.unit ? " · " + esc(e.unit) : ""}${e.by ? " · " + esc(e.by) : ""}${e.notes ? " · " + esc(e.notes) : ""}${e.kind === "due" ? " · " + esc(e.state) : ""}</div></div>
  ${e.kind === "due" && e.src === "cleantasks" ? `<button class="btn small green" data-act="cl-done" data-id="${esc(e.taskId)}">Done</button>` : e.kind === "due" && e.src === "mgrchecks" ? `<button class="btn small green" data-act="mgr-done" data-id="${esc(e.taskId)}">Done</button>` : e.kind === "due" && e.src === "deepclean" ? `<button class="btn small green" data-act="deep-done" data-id="${esc(e.taskId)}">Done</button>` : e.kind === "due" ? `<button class="btn small green" data-act="pm-done-ask" data-id="${e.taskId}">Mark Done</button>` : e.kind === "issue" ? `<button class="btn small" data-act="issue-open" data-id="${e.issueId}">Open</button>` : e.kind === "event" ? `<button class="btn small wine" data-act="cal-event-del" data-id="${esc(e.eventId)}">${icon("trash")}Remove</button>` : ""}</div>`;

function dayDialog(d) {
  const ev = calShown().filter(e => e.date === d), title = new Date(d + "T12:00").toLocaleDateString("en-US", {weekday: "long", month: "long", day: "numeric", year: "numeric"});
  showModal(`<h2>${icon("list")}${title}</h2><div class="body">${ev.length ? ev.map(evRow).join("") : `<p class="hint">Nothing scheduled this day.</p>`}</div>
    <div class="foot"><button class="btn green big" data-act="cal-event-new" data-d="${d}">${icon("plus")}Add Event</button><button class="btn big" data-act="close">Close</button></div>`);
}

function eventDialog(d) {
  const opts = (list, cur) => list.map(o => `<option ${o === cur ? "selected" : ""}>${esc(o)}</option>`).join("");
  showModal(`<h2>${icon("plus")}Add Calendar Event</h2><div class="body"><div class="fields">
    <label>What is it: <span class="req">*</span></label><input class="field" data-ev="title" placeholder="e.g. Hood cleaning, deep clean walk-in, health inspection">
    <label>Date: <span class="req">*</span></label><input type="date" class="field" data-ev="date" value="${esc(d || ymd(state.cal.ref))}" style="width:190px">
    <label>Type:</label><select class="field" data-ev="group" style="width:auto">${opts(["Cleaning", "Equipment PM", "Grounds / Facility", "Other"], "Other")}</select>
    <label>Area / equipment:</label><input class="field" data-ev="area" placeholder="Optional">
    <label>Repeats:</label><select class="field" data-ev="repeat" style="width:auto">${opts(["None", "Daily", "Weekly", "Monthly", "Quarterly", "Semi-Annually", "Yearly"], "None")}</select>
    <label>Notes:</label><textarea class="field" rows="2" data-ev="notes"></textarea>
    <label>Your name: <span class="req">*</span></label><input class="field" data-ev="by" value="${esc(who())}"></div>
    <p class="hint">For a job you want tracked with a Mark Done and history, add it under Schedule instead. Calendar events are reminders that show on their day.</p></div>
    <div class="foot"><button class="btn green big" data-act="cal-event-save">${icon("save")}Save</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

Object.assign(ACT, {
  "cal-event-new"(el) { eventDialog(el.dataset.d); },
  async "cal-event-save"() { const v = grab("[data-ev]", "ev"); if (!v.title.trim() || !v.by.trim()) return toast("Enter what it is and your name.", true); if (!v.date) return toast("Choose the date.", true); rememberWho(v.by);
    try { await api("/api/maint/calendar/events", {method: "POST", body: JSON.stringify(v)}); closeModal(); await loadCal(true); toast("Added to the calendar."); } catch (e) { toast(e.message, true); } },
  async "cal-event-del"(el) { if (!confirm("Remove this event? A repeating event is removed from every day.")) return; await api("/api/maint/calendar/events/" + el.dataset.id, {method: "DELETE"}); closeModal(); await loadCal(true); },
  "cal-nav"(el) { const c = state.cal, n = +el.dataset.d, y = c.ref.getFullYear(), m = c.ref.getMonth(), step = {month: 1, list: 1, quarter: 3, semi: 6, year: 12}[c.mode]; if (n === 0) c.ref = new Date(); else if (c.mode === "day") c.ref = addDays(c.ref, n); else if (c.mode === "week") c.ref = addDays(c.ref, 7 * n); else c.ref = new Date(y, m + step * n, 1); c.events = null; render(); },
  "cal-mode"(el) { state.cal.mode = el.dataset.v; state.cal.events = null; render(); },
  "cal-group"(el) { state.cal.group = el.dataset.v; render(); },
  "cal-freq"(el) { state.cal.freq = el.dataset.v; render(); },
  "cal-day"(el) { dayDialog(el.dataset.d); },
});
