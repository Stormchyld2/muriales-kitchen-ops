"use strict";

/* Master to-do list */

const TODO_PERIODS = [["today", "Today"], ["tomorrow", "Tomorrow"], ["week", "This Week"], ["nextweek", "Next Week"], ["next7", "Next 7 Days"], ["month", "This Month"], ["nextmonth", "Next Month"], ["next30", "Next 30 Days"], ["overdue", "Overdue Only"], ["custom", "Pick Dates..."]];

const TODO_SOURCES = [["Cleaning", "Cleaning"], ["Equipment PM", "Equipment PM"], ["Grounds / Facility", "Grounds / Facility"], ["Issue", "Open maintenance issues"], ["Inspection", "Inspection items"], ["Temperature", "Temperatures needing action"], ["Corrective Action", "Corrective action forms"], ["Manager Check", "Manager inspections"], ["Equipment", "Repair or replace decisions"], ["SDS", "SDS follow-ups"], ["Other", "My own items"]];

const TODO_FREQS = ["Daily", "Weekly", "Monthly", "Quarterly", "Semi-Annually", "Yearly"];
state.todo = null; state.todoPick = {kind: "all", dept: "all", freqs: [], period: "today", sources: TODO_SOURCES.map(x => x[0]), from: "", to: ""};

function todoRange(p, a, b) {
  const t = new Date(), day = new Date(t.getFullYear(), t.getMonth(), t.getDate());
  const eow = addDays(day, 6 - day.getDay()), nws = addDays(eow, 1);
  const R = {today: [day, day], tomorrow: [addDays(day, 1), addDays(day, 1)], week: [day, eow], nextweek: [nws, addDays(nws, 6)], next7: [day, addDays(day, 6)],
    month: [day, new Date(day.getFullYear(), day.getMonth() + 1, 0)], nextmonth: [new Date(day.getFullYear(), day.getMonth() + 1, 1), new Date(day.getFullYear(), day.getMonth() + 2, 0)],
    next30: [day, addDays(day, 29)], overdue: [day, day]};
  if (p === "custom") return [new Date(a + "T12:00"), new Date(b + "T12:00")];
  return R[p];
}

function todoDialog() {
  const k = state.todoPick;
  const depts = (state.maint && state.maint.departments) || [];
  showModal(`<h2>${icon("list")}Create To-Do List</h2><div class="body"><p style="margin-top:0"><b>What kind of list?</b></p>
    <div class="chips" style="margin-bottom:10px">${[["all", "Everything (master list)"], ["clean", "Cleaning"], ["maint", "Maintenance"]].map(([v, l]) => `<button class="chip ${k.kind === v ? "on" : ""}" data-act="todo-kind" data-v="${v}">${l}</button>`).join("")}</div>
    <p><b>Which department?</b> <span class="hint">pick one department, or a separate sheet for each</span></p>
    <div class="chips" style="margin-bottom:10px">${[["all", "All, one list"], ["each", "Each department separately"], ...depts.map(d => [d, d])].map(([v, l]) => `<button class="chip ${k.dept === v ? "on" : ""}" data-act="todo-dept" data-v="${esc(v)}">${esc(l)}</button>`).join("")}</div>
    <p><b>Pull everything that repeats:</b> <span class="hint">pick one or more to print a handout for the crew or maintenance</span></p>
    <div class="chips" style="margin-bottom:10px">${TODO_FREQS.map(f => `<button class="chip ${k.freqs.includes(f) ? "on" : ""}" data-act="todo-freq" data-v="${f}">${f}</button>`).join("")}</div>
    <p><b>Or just what is due in a period:</b></p>
    <div class="chips" style="margin-bottom:10px">${TODO_PERIODS.map(([v, l]) => `<button class="chip ${!k.freqs.length && k.period === v ? "on" : ""}" data-act="todo-period" data-v="${v}">${l}</button>`).join("")}</div>
    ${!k.freqs.length && k.period === "custom" ? `<div class="row" style="margin-bottom:10px"><label>From</label><input type="date" class="field" id="todofrom" value="${esc(k.from)}" style="width:170px"><label>to</label><input type="date" class="field" id="todoto" value="${esc(k.to)}" style="width:170px"></div>` : ""}
    <p><b>What to include?</b></p>${TODO_SOURCES.map(([v, l]) => `<label style="display:block;font-weight:400;margin:3px 0"><input type="checkbox" data-src="${esc(v)}" ${k.sources.includes(v) ? "checked" : ""}> ${l}</label>`).join("")}
    <p class="hint">Overdue jobs and open issues always appear, whatever the period.</p></div>
    <div class="foot"><button class="btn green big" data-act="todo-go">${icon("list")}Create List</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

async function refreshTodo() {
  const t = state.todo; if (!t) return;
  const r0 = await api(`/api/todo?from=${t.from}&to=${t.to}&sources=${encodeURIComponent(t.sources.join(","))}${t.freqs ? "&freq=" + encodeURIComponent(t.freqs.join(",")) : ""}${t.dept && t.dept !== "all" && t.dept !== "each" ? "&dept=" + encodeURIComponent(t.dept) : ""}`);
  t.items = r0.items; t.generated = r0.generated; if (state.view === "todo") render();
}

function vTodo() {
  const t = state.todo;
  if (!t) return `<div class="empty"><b>No to-do list yet</b>Choose a period and the program gathers everything that is due into one list.<br><br><button class="btn green big" data-act="todo-new">${icon("list")}Create To-Do List</button></div>`;
  const items = t.items.filter(i => (!t.overdueOnly || i.state === "Overdue") && !(i.group === "Other" && ((t.kindLabel === "clean" && i.type !== "Cleaning") || (t.kindLabel === "maint" && i.type !== "Maintenance"))));
  const depts = (state.maint && state.maint.departments) || [], byDept = t.dept === "each";
  const groups = byDept ? [...depts, "General"].filter((d, i, a) => a.indexOf(d) === i).map(d => [d, d, items.filter(i => (i.dept || "General") === d)]).filter(x => x[2].length) : t.freqs ? [...TODO_FREQS, ""].map(f => [f, f || "Open items", items.filter(i => (i.bucket || "") === f)]).filter(x => x[2].length) : TODO_SOURCES.map(([g, l]) => [g, l, items.filter(i => i.group === g)]).filter(x => x[2].length);
  const late = items.filter(i => i.state === "Overdue").length;
  const act = i => i.kind === "todo" ? `<button class="btn small green noprint" data-act="todo-done" data-id="${i.id}">Done</button>` : i.kind === "task" ? `<button class="btn small green noprint" data-act="todo-act" data-kind="task" data-id="${i.id}">Mark Done</button>` : `<button class="btn small noprint" data-act="todo-act" data-kind="${i.kind}" data-id="${i.id}">Open</button>`;
  return `<div class="row noprint"><button class="btn green" data-act="todo-new">${icon("list")}Change Period</button><button class="btn" data-act="todo-refresh">${icon("swap")}Refresh</button><button class="btn" data-act="todo-add">${icon("plus")}Add My Own Item</button><button class="btn" data-act="todo-print">${icon("print")}Print</button></div>
  <div class="panel todolist"><h3 style="margin-top:0">To-Do List: ${esc(t.label)}</h3><div class="hint">Made ${new Date(t.generated).toLocaleString("en-US", {dateStyle: "medium", timeStyle: "short"})} · ${items.length} item${items.length === 1 ? "" : "s"}${late ? ` · <b>${late} overdue</b>` : ""}</div>
    ${groups.length ? groups.map(([g, l, list]) => `<h4 ${byDept ? 'class="deptbreak"' : ""}>${l}</h4>${list.map(i => `<div class="trow"><span class="box"></span><div class="grow"><b>${esc(i.title)}</b><div class="hint">${esc(t.freqs ? [i.group, i.detail].filter(Boolean).join(" · ") : i.detail)}</div></div><div class="when">${i.due ? fmtDate(i.due) : ""}<div><span class="status ${({"Due Today": "Needs Attention", "Upcoming": "OK", "Open": "Open", "Overdue": "Overdue"})[i.state]}">${esc(i.state)}</span></div></div>${act(i)}</div>`).join("")}`).join("")
      : `<p>Nothing to do for this period.</p>`}</div>`;
}

Object.assign(ACT, {
  async "todo-new"() { if (!state.maint) state.maint = await api("/api/maint"); todoDialog(); },
  "todo-kind"(el) { const k = state.todoPick; k.kind = el.dataset.v; k.sources = k.kind === "clean" ? ["Cleaning", "Manager Check", "Corrective Action", "Other"] : k.kind === "maint" ? ["Equipment PM", "Grounds / Facility", "Issue", "Inspection", "Equipment", "Other"] : TODO_SOURCES.map(x => x[0]); todoDialog(); },
  "todo-dept"(el) { const k = state.todoPick; k.sources = [...document.querySelectorAll("[data-src]")].filter(x => x.checked).map(x => x.dataset.src); k.dept = el.dataset.v; todoDialog(); },
  "todo-freq"(el) { const k = state.todoPick; k.sources = [...document.querySelectorAll("[data-src]")].filter(x => x.checked).map(x => x.dataset.src); const v = el.dataset.v; k.freqs = k.freqs.includes(v) ? k.freqs.filter(x => x !== v) : TODO_FREQS.filter(x => x === v || k.freqs.includes(x)); todoDialog(); },
  "todo-period"(el) { state.todoPick.freqs = []; state.todoPick.period = el.dataset.v; state.todoPick.sources = [...document.querySelectorAll("[data-src]")].filter(x => x.checked).map(x => x.dataset.src); todoDialog(); },
  async "todo-go"() { const k = state.todoPick; k.sources = [...document.querySelectorAll("[data-src]")].filter(x => x.checked).map(x => x.dataset.src);
    if (!k.sources.length) return toast("Choose at least one thing to include.", true);
    if (k.freqs.length) { const a = new Date(), b = addDays(a, 364); state.todo = {dept: k.dept, kindLabel: k.kind, label: (k.kind === "clean" ? "Cleaning: " : k.kind === "maint" ? "Maintenance: " : "") + k.freqs.join(", ") + " jobs and everything open now" + (k.dept !== "all" && k.dept !== "each" ? ", " + k.dept : ""), period: "freq", freqs: [...k.freqs], from: ymd(a), to: ymd(b), sources: [...k.sources], overdueOnly: false, items: []}; await refreshTodo(); closeModal(); state.view = "todo"; render(); return; }
    if (k.period === "custom") { k.from = $("#todofrom").value; k.to = $("#todoto").value; if (!k.from || !k.to || k.to < k.from) return toast("Choose the dates, with the end on or after the start.", true); }
    const [a, b] = todoRange(k.period, k.from, k.to), label = k.period === "custom" ? `${fmtDate(k.from)} to ${fmtDate(k.to)}` : TODO_PERIODS.find(x => x[0] === k.period)[1];
    const full = label + (k.period === "custom" ? "" : a.getTime() === b.getTime() ? " (" + a.toLocaleDateString("en-US", {weekday: "long", month: "long", day: "numeric"}) + ")" : " (" + a.toLocaleDateString("en-US", {month: "short", day: "numeric"}) + " – " + b.toLocaleDateString("en-US", {month: "short", day: "numeric"}) + ")");
    state.todo = {dept: k.dept, kindLabel: k.kind, label: (k.kind === "clean" ? "Cleaning: " : k.kind === "maint" ? "Maintenance: " : "") + full + (k.dept !== "all" && k.dept !== "each" ? ", " + k.dept : ""), period: k.period, from: ymd(a), to: ymd(b), sources: [...k.sources], overdueOnly: k.period === "overdue", items: []};
    await refreshTodo(); closeModal(); state.view = "todo"; render(); },
  async "todo-refresh"() { await refreshTodo(); toast("Refreshed."); },
  "todo-print"() { window.print(); },
  "todo-add"() { showModal(`<h2>${icon("plus")}Add My Own Item</h2><div class="body"><div class="fields">
    <label>To do: <span class="req">*</span></label><input class="field" data-td="title" placeholder="e.g. Call the hood cleaner">
    <label>Type:</label><select class="field" data-td="type" style="width:auto"><option>Cleaning</option><option>Maintenance</option><option>General</option></select>
    <label>Department:</label><select class="field" data-td="dept" style="width:auto">${["", ...((state.maint && state.maint.departments) || [])].map(d => `<option>${esc(d)}</option>`).join("")}</select>
    <label>Due date:</label><input type="date" class="field" data-td="due" style="width:190px"><label>Notes:</label><input class="field" data-td="notes" placeholder="Optional">
    <label>Your name: <span class="req">*</span></label><input class="field" data-td="createdBy" value="${esc(who())}"></div></div>
    <div class="foot"><button class="btn green big" data-act="todo-add-go">${icon("save")}Add</button><button class="btn big" data-act="close">Cancel</button></div>`); },
  async "todo-add-go"() { const v = grab("[data-td]", "td"); rememberWho(v.createdBy); await api("/api/maint/todos", {method: "POST", body: JSON.stringify(v)}); closeModal(); if (state.todo && !state.todo.sources.includes("Other")) state.todo.sources.push("Other"); await refreshTodo(); toast("Added."); },
  "todo-done"(el) {
    showModal(`<h2>${icon("save")}Done</h2><div class="body"><div class="fields"><label>Your name: <span class="req">*</span></label><input class="field" id="todoby" value="${esc(who())}"></div></div>
    <div class="foot"><button class="btn green big" data-act="todo-done-go" data-id="${el.dataset.id}">${icon("save")}Save</button><button class="btn big" data-act="close">Cancel</button></div>`); },
  async "todo-done-go"(el) { const by = $("#todoby").value; rememberWho(by); await api("/api/maint/todos/" + el.dataset.id + "/done", {method: "POST", body: JSON.stringify({by})}); closeModal(); await refreshTodo(); toast("Done."); },
  async "todo-act"(el) { const k = el.dataset.kind, id = el.dataset.id; if (!state.maint) state.maint = await api("/api/maint");
    if (k === "missed") { state.reg.missed = await api("/api/reg/missed"); return regForm("missed", id); } if (k === "deep") { state.today = null; await loadToday(); return deepDone(id); } if (k === "clean") return cleanDone(id); if (k === "cleanday") return ACT.nav({dataset: {v: "cltoday"}}); if (k === "mgr") return mgrDone(id); if (k === "repl") return replForm(id); if (k === "ca") return caForm(id); if (k === "temp") return tempAction(id); if (k === "finding") return findDetail(id); if (k === "task") return pmDone(id); if (k === "issue") return issueDetail(id); if (k === "equipment") return assetDetail(id);
    if (k === "product") { state.id = id; state.view = "detail"; render(); } },
});
