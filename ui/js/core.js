"use strict";

/* Core: state, API, navigation, dialogs, shared helpers */



const ACT = {};

const $ = (s, r = document) => r.querySelector(s);

const esc = s => String(s ?? "").replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));

const ICON = {
  home:'<path d="M3 11l9-8 9 8v9a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1z"/>',
  add:'<path d="M6 2h9l5 5v15H6z"/><path d="M12 11v7M8.5 14.5h7"/>',
  cats:'<path d="M3 6h7l2 2h9v11H3z"/>',
  print:'<path d="M7 8V3h10v5"/><rect x="3" y="8" width="18" height="9" rx="1"/><path d="M7 14h10v7H7z"/>',
  gear:'<circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M19.1 4.9L17 7M7 17l-2.1 2.1"/>',
  info:'<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7.5v.5"/>',
  plus:'<path d="M12 4v16M4 12h16"/>',
  trash:'<path d="M4 7h16M9 7V4h6v3M6 7l1 14h10l1-14M10 11v6M14 11v6"/>',
  edit:'<path d="M4 20l1-5L16 4l4 4L9 19z"/>',
  swap:'<path d="M20 11a8 8 0 0 0-14.5-4M4 4v4h4M4 13a8 8 0 0 0 14.5 4M20 20v-4h-4"/>',
  back:'<path d="M20 12H5M11 5l-7 7 7 7"/>',
  search:'<circle cx="10.5" cy="10.5" r="6.5"/><path d="M15.5 15.5L21 21"/>',
  x:'<path d="M5 5l14 14M19 5L5 19"/>',
  save:'<path d="M4 4h13l3 3v13H4z"/><path d="M8 4v5h7V4M8 20v-6h8v6"/>',
  up:'<path d="M12 16V5M7 10l5-5 5 5M5 20h14"/>',
  list:'<path d="M6 3h12v18H6zM9 8h6M9 12h6M9 16h4"/>',
  warn:'<path d="M12 3l10 18H2z"/><path d="M12 10v5M12 17.5v.5"/>',
  folder:'<path d="M3 6h7l2 2h9v11H3z"/>',
  down:'<path d="M12 4v12M7 11l5 5 5-5M5 20h14"/>',
  wrench:'<path d="M14.5 5.5a4 4 0 0 0 4.9 5.2L20 11l-9 9a2.1 2.1 0 0 1-3-3l9-9z"/>',
  ext:'<path d="M14 4h6v6M20 4l-9 9M18 14v6H4V6h6"/>',
};

const icon = n => `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round">${ICON[n]}</svg>`;

const diamond = inner => `<svg viewBox="0 0 48 48"><path d="M24 2 46 24 24 46 2 24z" fill="#fff" stroke="#d81e2c" stroke-width="3.5" stroke-linejoin="round"/>${inner}</svg>`;

const state = { tab: "storage", backups: null, lock: null, rec: null, diag: null, integ: null, lib: {categories: [], locations: [], catchAll: "Other / Miscellaneous", products: [], removed: [], binder: null, dataFolder: ""}, sel: new Set(), q: "", cat: "", sort: {key: "name", dir: 1}, view: "library", id: null, form: null };

async function api(path, opts) {
  const r = await fetch(path, opts);
  const j = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(j.error || "Something went wrong. Please try again.");
  return j;
}

async function reload() {
  state.lib = await api("/api/library");
  const ids = new Set(state.lib.products.map(p => p.id));
  for (const id of [...state.sel]) if (!ids.has(id)) state.sel.delete(id);
}

let toastT;

function toast(msg, err) {
  const t = $("#toast"); t.textContent = msg; t.className = err ? "err" : ""; t.style.display = "block";
  clearTimeout(toastT); toastT = setTimeout(() => t.style.display = "none", err ? 7000 : 3500);
}

const fmtDate = d => { const m = /^(\d{4})-(\d\d)-(\d\d)/.exec(d || ""); return m ? `${m[2]}/${m[3]}/${m[1]}` : ""; };

const longDate = () => new Date().toLocaleDateString("en-US", {year: "numeric", month: "long", day: "numeric"});

const MAINT_VIEWS = ["insp", "procs", "finds", "equip", "assets", "issues", "pm", "cal", "clean", "fac", "summary"];

const MODS = [
  {k: "maint", label: "Maintenance", icon: "wrench", subs: [["equip","Equipment"],["assets","Assets & Costs"],["issues","Maintenance Issues"],["pm","Equipment PM"],["insp","PM Inspections"],["finds","Inspection Items"],["repl","Replacement Requests"],["summary","Monthly Summary"],["procs","Procedures"]]},
  {k: "clean", label: "Cleaning & Sanitation", icon: "list", subs: [["cltoday","Today's Checklist"],["cltasks","Cleaning Schedule"],["clweek","Weekly Schedules"],["cldeep","Weekly Deep Clean"],["clarea","Area Matrix"],["clstd","Standards"],["clref","Chemical & Tool Reference"],["clmiss","Missed & Overdue"],["cllog","Completion Log"],["mgrchk","Manager Inspections"],["mgrlog","Manager Inspection Log"]]},
  {k: "sched", label: "Schedule", icon: "list", subs: [["cal","Calendar"],["clean","Cleaning Tasks"],["fac","Grounds / Facility"]]},
  {k: "food", label: "Food Safety", icon: "list", subs: [["temps","Temperature Log"],["tpoints","Temperature Points"],["ready","Inspection Readiness"]]},
  {k: "chem", label: "Chemical Safety", icon: "warn", subs: [["chemresp","Spill / Exposure Response"],["chemlog","Incident Log"]]},
  {k: "ca", label: "Corrective Actions", icon: "warn", subs: [["calog","Corrective Action Log"]]},
  {k: "train", label: "Employee Training", icon: "list", subs: [["trmatrix","Training Matrix"],["tremp","Employees"],["trrec","Training Records"],["trtype","Training Types"]]},
  {k: "sds", label: "SDS Manager", icon: "home", subs: [["library","Library"],["add","Add SDS"],["categories","Categories"],["print","Print / Export"],["notes","Handoff Notes"]]},
];

const NAV_TOP = [["todo","list","To-Do List"],["reports","print","Reports"],["settings","gear","Settings"],["about","info","About"]];

const modOf = v => (MODS.find(m => m.subs.some(([k]) => k === v || (v === "detail" && k === "library"))) || {}).k;

const SUB = {trmatrix:"Employee Training Matrix", tremp:"Employees", trrec:"Training Records", trtype:"Training Types", repl:"Replacement Requests", cltoday:"Today's Cleaning Checklist", cltasks:"Cleaning Schedule", clweek:"Weekly Cleaning Schedules", cldeep:"Weekly Deep Clean", clarea:"Area Cleaning Matrix", clstd:"Cleaning Standards", clref:"Chemical & Tool Reference", clmiss:"Missed & Overdue Cleaning", cllog:"Cleaning Completion Log", mgrchk:"Manager Inspection Schedule", mgrlog:"Manager Inspection Log", ready:"Inspection Readiness", chemresp:"Spill / Exposure Response", chemlog:"Chemical Incident Log", calog:"Corrective Action Log", temps:"Temperature Log", tpoints:"Temperature Points", insp:"PM Inspection Sheets", procs:"Maintenance Procedures", reports:"Reports", finds:"Inspection Items", summary:"Monthly Maintenance Summary", todo:"To-Do List", cal:"Calendar", clean:"Cleaning Tasks", fac:"Grounds / Facility Maintenance", assets:"Assets & Maintenance Costs", pm:"Equipment Preventive Maintenance", equip:"Equipment", issues:"Maintenance Issues", library:"Safety Data Sheet (SDS) Management", detail:"Product Detail / Emergency Quick View", add:"Add / Edit Safety Data Sheet", categories:"Categories", notes:"Manager Handoff Notes", settings:"Settings", about:"About"};

function render() {
  const active = state.view === "detail" ? "library" : state.view;
  if (state.lastView !== state.view) { const cur = modOf(state.view); if (cur) state.openMod = cur; state.lastView = state.view; }
  $("#nav").innerHTML = MODS.map(m => `<button class="nav mod ${state.openMod === m.k ? "open" : ""}" data-act="mod" data-k="${m.k}">${icon(m.icon)}<span class="grow">${m.label}</span><span class="chev">${state.openMod === m.k ? "▾" : "▸"}</span></button>`
      + (state.openMod === m.k ? m.subs.map(([v, l]) => `<button class="nav sub ${v === active ? "on" : ""}" data-act="nav" data-v="${v}">${l}</button>`).join("") : "")).join("")
    + `<button class="btn green wo" data-act="todo-new" title="Gather everything due into one list">${icon("list")}Create To-Do List</button><div class="navsep"></div>` + NAV_TOP.map(([v, i, l]) => `<button class="nav ${v === active ? "on" : ""}" data-act="nav" data-v="${v}">${icon(i)}${l}</button>`).join("")
    + (MAINT_VIEWS.includes(state.view) ? `<button class="btn green wo" data-act="submit-wo" title="Opens MaintainX in its own window">${icon("ext")}Submit Work Order</button>` : "");
  $("#subtitle").textContent = state.view === "add" && state.form && state.form.replaceMode ? "Replace SDS PDF" : SUB[state.view];
  const shown = state.view === "detail" && byId(state.id);
  $("#pictos").innerHTML = !shown ? "" : (shown.signalWord ? `<span class="signal ${esc(shown.signalWord)}">${esc(shown.signalWord.toUpperCase())}</span>` : "")
    + pictosOf(shown).map(([k, label]) => `<figure title="${label}">${PICTO[k]}<figcaption>${label}</figcaption></figure>`).join("");
  $("#view").innerHTML = oosBanner() + lockBanner() + ({trmatrix: vTrMatrix, tremp: () => vReg("employees"), trrec: () => vReg("training"), trtype: () => vReg("trainingtypes"), repl: vRepl, cltoday: vClToday, cltasks: () => vReg("cleantasks"), clweek: vClWeek, cldeep: () => vReg("deepclean"), clarea: () => vReg("cleanareas"), clstd: () => vReg("standards"), clref: () => vReg("cleanref"), clmiss: () => vReg("missed"), cllog: () => vReg("cleanlog"), mgrchk: () => vReg("mgrchecks"), mgrlog: () => vReg("mgrlog"), ready: vReady, chemresp: vChemResp, chemlog: vChemLog, calog: vCA, temps: vTemps, tpoints: vTPoints, insp: vInsp, procs: vProcs, reports: vReports, finds: vFinds, summary: vSummary, todo: vTodo, cal: vCal, clean: () => vPM("Cleaning"), fac: () => vPM("Grounds / Facility"), assets: vAssets, pm: () => vPM("Equipment PM"), equip: vEquip, issues: vIssues, library: vLibrary, detail: vDetail, add: vForm, categories: vCats, notes: vNotes, settings: vSettings, about: vAbout}[state.view])();
  if (state.view === "library") { const s = $("#q"); if (s && state.focusQ) { s.focus(); s.setSelectionRange(s.value.length, s.value.length); } }
}

function closeModal() { $("#modal").className = ""; $("#modal").innerHTML = ""; }

function showModal(html) { $("#modal").innerHTML = `<div class="dialog">${html}</div>`; $("#modal").className = "show"; }

function emailResult(o, what) {
  if (o.method === "outlook") return toast("The message is open in Outlook with the " + what + " in it. Press Send there. If you do not see it, check behind this window or on the taskbar.");
  try { navigator.clipboard.writeText(o.text || ""); } catch (e) {}
  showModal(`<h2>${icon("ext")}Your email is open</h2><div class="body"><p style="margin-top:0">A new message opened in your mail program with the ${esc(what)} written in it. Check it, then press Send there.</p>
    ${o.cut ? `<p class="note warn"><b>This ${esc(what)} is long, so the message may be cut short.</b> The full text is copied. In the message, click the body, press Ctrl+A, then Ctrl+V to replace it with the whole thing.</p>` : ""}
    <p class="hint">If no mail window opened, your computer has no mail program set up for links. Open your email, start a new message and paste the text below.</p>
    <textarea class="field" rows="8" readonly onclick="this.select()">${esc(o.text || "")}</textarea></div><div class="foot"><button class="btn big" data-act="close">Close</button></div>`);
}

const ymd = d => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

const addDays = (d, n) => { const x = new Date(d); x.setDate(x.getDate() + n); return x; };

const money = n => "$" + (Number(n) || 0).toLocaleString("en-US", {minimumFractionDigits: 2, maximumFractionDigits: 2});

const who = () => { try { return localStorage.getItem("kitchenName") || ""; } catch (e) { return ""; } };

const rememberWho = n => { try { localStorage.setItem("kitchenName", n); } catch (e) {} };

async function loadMaint() { state.maint = await api("/api/maint"); if (state.view === "settings" && (state.tab === "maintainx" || state.tab === "eqareas")) render(); }

const tag = v => `<span class="status ${esc(v)}">${esc(v)}</span>`;

const openIssues = id => (state.maint.issues || []).filter(p => p.equipmentId === id && p.status !== "Verified" && p.status !== "Closed").length;

const eqName = id => ((state.maint.equipment || []).find(e => e.id === id) || {}).name || "(removed)";

const yesNoRadio = (k, label, val) => `<label>${label}</label><div class="row">${["Yes", "No"].map(v => `<label style="font-weight:400"><input type="radio" name="oos_${k}" value="${v}" ${val === v ? "checked" : ""}> ${v}</label>`).join("")}</div>`;

const dayLong = d => { const t = new Date(d + "T12:00:00"); return isNaN(t) ? d : t.toLocaleDateString(undefined, {weekday: "short", year: "numeric", month: "short", day: "numeric"}); };

const el_val = name => (document.querySelector(`[name=${name}]:checked`) || {}).value;
state.reg = state.reg || {};

const shortDate = d => { const t = new Date(d + "T12:00:00"); return isNaN(t) ? d : t.toLocaleDateString(undefined, {month: "numeric", day: "numeric", year: "numeric"}); };

const grab = (sel, attr) => Object.fromEntries([...document.querySelectorAll(sel)].map(el => [el.dataset[attr], el.type === "checkbox" ? el.checked : el.value]));

const maintDone = async (data, msg) => { state.maint = data; state.sum = null; closeModal(); if (state.view === "cal") await loadCal(true); if (state.view === "todo") await refreshTodo(); render(); if (msg) toast(msg); };

Object.assign(ACT, {
  mod(el) { const k = el.dataset.k, m = MODS.find(x => x.k === k);
    if (state.openMod === k) { state.openMod = null; return render(); }
    state.openMod = k; return ACT.nav({dataset: {v: m.subs[0][0]}}); },
  nav(el) { const v = el.dataset.v; if (v === "insp") state.insp = null; if (v === "finds") state.finds = null; if (v === "summary") state.sum = null; if (v === "print") return state.lib.products.length ? printDialog("all") : toast("Add a product first."); if (v === "add") state.form = blankForm(); state.view = v; if (v === "settings") loadTab(); state.focusQ = false; render(); if (v === "library") return reload().then(render); },
  close: closeModal,
  "pv-print"() { try { $("#pvFrame").contentWindow.focus(); $("#pvFrame").contentWindow.print(); } catch (e) { toast("Use the printer button in the top right corner of the preview.", true); } },
  "pv-save"() { const a = document.createElement("a"); a.href = pv.url; a.download = pv.name; a.click(); },
  "pv-close"() { $("#preview").className = ""; $("#pvFrame").src = "about:blank"; render(); },
});
