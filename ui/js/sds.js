"use strict";

/* SDS Manager: library, detail, add/edit, categories */

const PICTO = {
  flame: diamond('<path d="M24 12c1 5 7 7 7 14a7 7 0 0 1-14 0c0-3 1.500-5 3-6 0 2 1 3 2 3 1-3 0-7 2-11z" fill="#111"/><path d="M15 36h18" stroke="#111" stroke-width="2.500"/>'),
  oxidizer: diamond('<path d="M24 9c.7 3.500 5 5 5 10a5 5 0 0 1-10 0c0-2 1-3.500 2-4.200 0 1.400.7 2 1.400 2 .7-2 0-5 1.600-7.800z" fill="#111"/><circle cx="24" cy="31" r="6" fill="none" stroke="#111" stroke-width="2.600"/><path d="M14 39h20" stroke="#111" stroke-width="2.500"/>'),
  explosive: diamond('<path d="M24 12l2.500 7 6-4.500-2.500 7 7.500.500-6 4.500 5 5.500-7.500-1.500L28 38l-4-6-4 6-1-7.500-7.500 1.500 5-5.500-6-4.500 7.500-.500-2.500-7 6 4.500z" fill="#111"/>'),
  corrosive: diamond('<path d="M14 13l9 3-2 6-9-3zM34 13l-9 3 2 6 9-3z" fill="#111"/><path d="M20 24c1.200 2 2 3 2 4.200a2 2 0 0 1-4 0c0-1.200.8-2.200 2-4.200zM29 24c1.200 2 2 3 2 4.200a2 2 0 0 1-4 0c0-1.200.8-2.200 2-4.200z" fill="#111"/><path d="M11 35h9l2-2.500 2 2.500h13" stroke="#111" stroke-width="3" fill="none"/>'),
  gas: diamond('<g transform="rotate(-35 24 24)"><rect x="12" y="19" width="22" height="10" rx="5" fill="#111"/><rect x="34" y="22" width="5" height="4" fill="#111"/></g>'),
  skull: diamond('<path d="M24 11a9 9 0 0 0-9 9c0 3 1.400 5 3.500 6.500V30h11v-3.500C31.600 25 33 23 33 20a9 9 0 0 0-9-9z" fill="#111"/><circle cx="20.500" cy="20" r="2.300" fill="#fff"/><circle cx="27.500" cy="20" r="2.300" fill="#fff"/><path d="M15 32l18 6M33 32l-18 6" stroke="#111" stroke-width="2.600" stroke-linecap="round"/>'),
  bang: diamond('<path d="M24 11c2 0 3 1.500 3 3l-1.700 13h-2.600L21 14c0-1.500 1-3 3-3z" fill="#111"/><circle cx="24" cy="33" r="3" fill="#111"/>'),
  health: diamond('<circle cx="24" cy="15" r="4.500" fill="#111"/><path d="M13 38c0-9 4-16 11-16s11 7 11 16z" fill="#111"/><path d="M24 24.500l1.300 3 3.200-.600-2 2.600 2 2.600-3.200-.600-1.300 3-1.300-3-3.200.600 2-2.600-2-2.600 3.200.600z" fill="#fff"/>'),
  environment: diamond('<path d="M17 33V20M17 25l-4-4M17 23l4-4M17 20l-2-4M17 20l2-4" stroke="#111" stroke-width="2" fill="none" stroke-linecap="round"/><path d="M22 33c3-4 9-4 12 0-3 3.500-9 3.500-12 0zM34 33l3-2.500v5z" fill="#111"/><path d="M11 37h26" stroke="#111" stroke-width="2"/>'),
  cross: '<svg viewBox="0 0 48 48"><rect x="3" y="3" width="42" height="42" rx="5" fill="#c8202f"/><path d="M24 11v26M11 24h26" stroke="#fff" stroke-width="8"/></svg>',
  ppe: '<svg viewBox="0 0 48 48" fill="none" stroke="#e0bc6e" stroke-width="3"><rect x="4" y="16" width="17" height="14" rx="6"/><rect x="27" y="16" width="17" height="14" rx="6"/><path d="M21 22h6"/></svg>',
};

// The nine GHS hazard pictograms, in the order they are offered on the form.
const GHS = [["flame","Flammable"],["oxidizer","Oxidizer"],["explosive","Explosive"],["corrosive","Corrosive"],["gas","Gas under pressure"],["skull","Acute toxicity"],["bang","Irritant / harmful"],["health","Health hazard"],["environment","Environmental hazard"]];

const pictosOf = p => GHS.filter(([k]) => (p.pictograms || []).includes(k));

const CAT_HELP = {
  "Dish Machine": "Machine detergents, rinse aids, delimers, machine descalers",
  "Sanitizers & Disinfectants": "Quaternary ammonium sanitizers, chlorine sanitizers, disinfectants",
  "Degreasers": "Heavy-duty degreasers, grill cleaners, oven cleaners, fryer cleaners",
  "General Cleaning": "All-purpose cleaners, surface cleaners, glass cleaners",
  "Floor Care": "Floor cleaners, floor degreasers, strippers, floor finishes",
  "Restroom Cleaning": "Toilet bowl cleaners, urinal cleaners, bathroom disinfectants",
  "Laundry": "Laundry detergents, bleach, fabric softeners, stain removers",
  "Drain Maintenance": "Drain openers, drain cleaners, chemical drain treatments",
  "Descalers & Delimers": "Lime removers, mineral deposit removers, equipment descalers",
  "Pest Control": "Insecticides, pesticides, rodent-control chemicals",
  "Hand Hygiene": "Hand soaps, hand sanitizers, specialty hand cleaners",
  "Maintenance Chemicals": "Lubricants, adhesives, solvents, equipment maintenance chemicals",
  "Air Care & Deodorizers": "Air fresheners, odor neutralizers, deodorizing chemicals",
  "Other / Miscellaneous": "Chemicals that don't fit an established category",
};

// The SDS date is the manufacturer's revision date and nothing else. The day a
// manager last edited the product is a different thing and is never shown in its place.
const sdsDate = p => fmtDate(p.revisionDate) || "Unknown";

const hasSummary = p => [p.hazards, p.firstAidEyes, p.firstAidSkin, p.firstAidInhalation, p.firstAidSwallowed, p.ppe, p.signalWord].some(v => (v || "").trim()) || (p.pictograms || []).length > 0;

// Where the emergency summary stands: missing, typed in but never checked,
// checked but the PDF has since been replaced, or verified.
function summaryState(p) {
  if (!hasSummary(p)) return {k: "none", tag: "summary missing", cls: ""};
  if (p.summaryNeedsCheck) return {k: "recheck", tag: "summary needs re-check", cls: "red"};
  if (!p.summaryVerifiedAt) return {k: "unverified", tag: "summary not verified", cls: ""};
  return {k: "ok"};
}

// What has changed in the library since the paper binder was last recorded as up to date.
function binderDiff() {
  const b = state.lib.binder; if (!b) return null;
  const items = b.items || {}, live = state.lib.products, d = {add: [], replace: [], pull: [], index: false};
  for (const p of live) {
    const was = items[p.id];
    if (!was) d.add.push(p);
    else if (was.pdfUpdatedAt !== p.pdfUpdatedAt) d.replace.push(p);
    else if (["name", "manufacturer", "category", "location", "revisionDate"].some(k => (was[k] || "") !== (p[k] || ""))) d.index = true;
  }
  for (const id in items) if (!live.some(p => p.id === id)) d.pull.push(items[id].name);
  d.any = d.add.length + d.replace.length + d.pull.length > 0 || d.index;
  return d;
}

const byId = id => state.lib.products.find(p => p.id === id);

function visible() {
  const q = state.q.trim().toLowerCase();
  const k = state.sort.key;
  const val = p => (k === "date" ? p.revisionDate || "" : p[k] || "").toLowerCase();
  return state.lib.products
    .filter(p => (!state.cat || p.category === state.cat) && (!q || [p.name, p.aliases, p.manufacturer, p.supplier, p.tags].join(" ").toLowerCase().includes(q)))
    .sort((a, b) => val(a).localeCompare(val(b)) * state.sort.dir || a.name.localeCompare(b.name));
}

function vLibrary() {
  const rows = visible(), total = state.lib.products.length;
  const th = (key, label) => `<th data-act="sort" data-k="${key}">${label}<span class="arr">${state.sort.key === key ? (state.sort.dir > 0 ? "▲" : "▼") : ""}</span></th>`;
  const allOn = rows.length > 0 && rows.every(p => state.sel.has(p.id));
  const body = !total
    ? `<div class="empty"><b>No safety data sheets yet</b>Click “Add SDS” to add the first product and its PDF.</div>`
    : !rows.length ? `<div class="empty"><b>No products match</b>Try a different search or category.</div>`
    : `<table><thead><tr><th class="nosort"><input type="checkbox" data-act="selall-box" ${allOn ? "checked" : ""} title="Select all shown"></th>
        ${th("name","Product Name")}${th("manufacturer","Manufacturer")}${th("category","Category")}${th("location","Storage Location")}${th("date","SDS Date")}</tr></thead><tbody>
        ${rows.map(p => `<tr class="${state.sel.has(p.id) ? "sel" : ""}" data-act="open" data-id="${p.id}">
          <td><input type="checkbox" data-act="toggle" data-id="${p.id}" ${state.sel.has(p.id) ? "checked" : ""}></td>
          <td>${esc(p.name)}${p.missing ? '<span class="tag red">PDF missing</span>' : ""}${p.needsReview ? '<span class="tag blue">needs review</span>' : ""}${(state.lib.integrity || {})[p.id] === "changed" ? '<span class="tag red">file changed</span>' : ""}${summaryState(p).tag ? `<span class="tag ${summaryState(p).cls}">${summaryState(p).tag}</span>` : ""}</td>
          <td>${esc(p.manufacturer)}</td><td>${esc(p.category)}</td><td>${esc(p.location)}</td><td>${p.revisionDate ? sdsDate(p) : '<span class="hint">Unknown</span>'}</td></tr>`).join("")}
      </tbody></table>`;
  return notesBanner() + binderNote() + `
  <div class="row">
    <div class="search">${icon("search")}<input id="q" class="field" placeholder="Search by product name, other name, manufacturer, supplier or tag..." value="${esc(state.q)}"></div>
    <label>Category:</label>
    <select id="cat" class="field" style="width:210px"><option value="">All Categories</option>
      ${state.lib.categories.map(c => `<option ${c === state.cat ? "selected" : ""}>${esc(c)}</option>`).join("")}</select>
    <button class="btn" data-act="refresh-folder" title="Pick up PDFs that were dropped into the sheets folder">${icon("swap")}Refresh</button>
  </div>
  <div class="tablewrap">${body}</div>
  <div class="row">
    <button class="btn small" data-act="selall">${icon("plus")}Select All</button>
    <button class="btn small" data-act="clear">Clear Selection</button>
    <span>Selected: ${state.sel.size} of ${total}</span>
  </div>
  <div class="row">
    <button class="btn green big" data-act="nav" data-v="add">${icon("plus")}Add SDS</button>
    <span class="grow"></span>
    <button class="btn big" data-act="print" data-scope="selected" ${state.sel.size ? "" : "disabled"}>${icon("list")}Print Selected</button>
    <button class="btn big" data-act="print" data-scope="all" ${total ? "" : "disabled"}>${icon("print")}Print Entire Book</button>
    <button class="btn big" data-act="print" data-scope="index" ${total ? "" : "disabled"}>${icon("list")}Print Index</button>
  </div>`;
}

function binderNote() {
  if (!state.lib.products.length && !state.lib.binder) return "";
  const d = binderDiff(), names = a => a.map(p => esc(p.name || p)).join(", ");
  const lb = state.lib.lastBuild, can = lb && lb.complete && !lb.confirmed;
  const confirm = can ? `<button class="btn small" data-act="binder-mark">Confirm Binder Updated...</button>` : "";
  const made = !lb || lb.confirmed ? "" : lb.complete
    ? ` A complete ${lb.scope === "all" ? "book" : lb.scope === "index" ? "index" : "set of sheets"} was assembled ${fmtDate(lb.at)} (${lb.pages} pages). Once it is printed and in the binder, confirm it.`
    : ` <b>The last book assembled (${fmtDate(lb.at)}) was incomplete</b>${(lb.missing || []).length ? ", missing: " + esc(lb.missing.join(", ")) : ""}. Fix those sheets and print again.`;
  if (!d) return `<div class="note warn"><span class="grow"><b>Paper binder:</b> not recorded yet. Print the entire book and put it in the binder.${made}</span>${confirm}</div>`;
  const when = fmtDate(state.lib.binder.updatedAt), pages = state.lib.binder.pages;
  if (!d.any) return `<div class="note ok"><span class="grow"><b>Paper binder is up to date.</b> Confirmed by a manager ${when}${pages ? `, ${pages} pages` : ""}.</span></div>`;
  const parts = [];
  if (d.add.length) parts.push(`<b>Add:</b> ${names(d.add)}`);
  if (d.replace.length) parts.push(`<b>Replace:</b> ${names(d.replace)}`);
  if (d.pull.length) parts.push(`<b>Take out:</b> ${names(d.pull)}`);
  if (d.index || d.add.length || d.pull.length) parts.push("<b>Reprint the index.</b>");
  return `<div class="note warn"><span class="grow"><b>Paper binder is out of date</b> (last confirmed ${when}). ${parts.join(" &nbsp;·&nbsp; ")}${made}</span>
    ${d.add.length + d.replace.length ? `<button class="btn small" data-act="binder-select">Select Sheets to Print</button>` : ""}${d.pull.length ? `<button class="btn small" data-act="binder-pulled">Sheets Taken Out</button>` : ""}${confirm}</div>`;
}

function vDetail() {
  const p = byId(state.id);
  if (!p) { state.view = "library"; return vLibrary(); }
  const line = (label, v) => (v || "").trim() ? `<b>${label}:</b> ${esc(v.trim())}\n` : "";
  const aid = line("Eyes", p.firstAidEyes) + line("Skin", p.firstAidSkin) + line("Inhalation", p.firstAidInhalation) + line("Swallowed", p.firstAidSwallowed);
  const none = `<p class="none">Not entered yet. Click Edit to add it, or read the full sheet.</p>`;
  const st = summaryState(p);
  const verify = st.k === "ok"
    ? `<div class="note ok"><span class="grow"><b>Summary verified</b> ${fmtDate(p.summaryVerifiedAt)}${p.summaryVerifiedBy ? " by " + esc(p.summaryVerifiedBy) : ""}, against the SDS ${p.summarySourceRevision ? "revised " + fmtDate(p.summarySourceRevision) : "(revision date unknown)"}.</span></div>`
    : st.k === "recheck" ? `<div class="note bad"><span class="grow"><b>The PDF was replaced after this summary was checked.</b> Rely on the full sheet until a manager re-checks the summary (Edit).</span></div>`
    : st.k === "unverified" ? `<div class="note warn"><span class="grow"><b>This summary has not been verified against the SDS.</b> Rely on the full sheet until a manager checks it (Edit).</span></div>`
    : "";
  const pics = pictosOf(p).map(([, l]) => l).join(", ");
  return `
  <div class="bar">${esc(p.name)}</div>
  <div class="detail">
    <div class="left">
      <div class="row" style="align-items:stretch;flex-wrap:nowrap">
        <div class="panel grow"><dl>
          <dt>Manufacturer:</dt><dd>${esc(p.manufacturer) || "—"}</dd>
          <dt>Supplier:</dt><dd>${esc(p.supplier) || "—"}</dd>
          ${(p.aliases || "").trim() ? `<dt>Also known as:</dt><dd>${esc(p.aliases)}</dd>` : ""}
          <dt>Category:</dt><dd>${esc(p.category) || "—"}</dd>
          ${(p.tags || "").trim() ? `<dt>Also used for:</dt><dd>${esc(p.tags)}</dd>` : ""}
          <dt>Storage Location:</dt><dd>${esc(p.location) || "—"}</dd>
          <dt>SDS Revision Date:</dt><dd>${fmtDate(p.revisionDate) || "Unknown"}</dd>
          <dt>Record Last Edited:</dt><dd>${fmtDate(p.updatedAt)}</dd>
        </dl></div>
        <div class="poison"><span>Poison Control</span><strong>1-800-222-1222</strong><span>Emergency: 911</span></div>
      </div>
      ${p.needsReview ? `<div class="note warn"><span class="grow"><b>Needs review.</b> This PDF was added without a manager's check. Any details shown were drafted from the sheet. Click Edit, check them against the PDF, and add the category and storage location.</span></div>` : ""}
      ${p.missing ? `<div class="note bad"><span class="grow"><b>The PDF for this product is missing from the sheets folder.</b> Use Replace to add it again, or Remove if the product is no longer used.</span><button class="btn small" data-act="replace" data-id="${p.id}">Replace PDF</button><button class="btn small wine" data-act="remove" data-id="${p.id}">Remove</button></div>` : ""}
      ${(state.lib.integrity || {})[p.id] === "changed" ? `<div class="note bad"><span class="grow"><b>This PDF is not the file that was added.</b> It was changed outside the program. Open it and check it. If it is the right sheet, accept it; the binder will then show this sheet as needing a reprint.</span><button class="btn small" data-act="integ-accept" data-id="${p.id}">Accept This File</button></div>` : ""}
      ${verify}
      <div class="panel sum">
        <div class="blk">${PICTO.bang}<div><h3>Hazard Warnings${p.signalWord ? " — " + esc(p.signalWord.toUpperCase()) : ""}</h3>${pics ? `<p><b>Pictograms on the SDS:</b> ${pics}</p>` : ""}${(p.hazards || "").trim() ? `<p>${esc(p.hazards)}</p>` : pics || p.signalWord ? "" : none}</div></div>
        <div class="blk">${PICTO.cross}<div><h3>First Aid</h3>${aid ? `<p>${aid}</p>` : none}</div></div>
        <div class="blk">${PICTO.ppe}<div><h3>Protective Equipment (PPE)</h3>${(p.ppe || "").trim() ? `<p>${esc(p.ppe)}</p>` : none}</div></div>
      </div>
    </div>
    <iframe src="/pdf/${p.id}?v=${encodeURIComponent(p.updatedAt)}#view=FitH" title="Safety data sheet"></iframe>
  </div>
  <div class="row">
    <button class="btn" data-act="print-one">${icon("print")}Print This Sheet</button>
    <button class="btn" data-act="emerg">${icon("warn")}Emergency Sheet</button>
    <button class="btn" data-act="edit">${icon("edit")}Edit</button>
    <button class="btn" data-act="replace" data-id="${p.id}">${icon("swap")}Replace PDF</button>
    <button class="btn wine" data-act="remove" data-id="${p.id}">${icon("trash")}Remove</button>
    <span class="grow"></span>
    <button class="btn" data-act="nav" data-v="library">${icon("back")}Back to Library</button>
  </div>`;
}

function remembered(k, v) { try { if (v === undefined) return localStorage.getItem("sds." + k) || ""; localStorage.setItem("sds." + k, v); } catch (e) {} return ""; }

const locations = () => [...new Set([...(state.lib.locations || []), ...state.lib.products.map(p => (p.location || "").trim()).filter(Boolean)])];

function blankForm() { return {id: null, name: "", manufacturer: "", supplier: "", aliases: "", tags: "", category: state.lib.categories[0] || state.lib.catchAll, location: "", revisionDate: "", pictograms: [], signalWord: "", verify: false, verifiedBy: remembered("by"), replaceMode: false, hazards: "", firstAidEyes: "", firstAidSkin: "", firstAidInhalation: "", firstAidSwallowed: "", ppe: "", pages: 0, file: null, fileNote: ""}; }

function vForm() {
  const f = state.form || (state.form = blankForm());
  const inp = (k, ph = "") => `<input class="field" data-f="${k}" value="${esc(f[k])}" placeholder="${ph}">`;
  const ta = (k, ph) => `<textarea class="field" rows="2" data-f="${k}" placeholder="${ph}">${esc(f[k])}</textarea>`;
  const st = f.id ? summaryState(byId(f.id) || f) : {k: "none"};
  const status = st.k === "ok" ? `Currently verified ${fmtDate(f.summaryVerifiedAt)}${f.summaryVerifiedBy ? " by " + esc(f.summaryVerifiedBy) : ""}. Changing the summary removes that until it is checked again.`
    : st.k === "recheck" ? "The PDF was replaced after the last check. Compare the summary with the new sheet, then tick the box."
    : "Not verified yet.";
  const fileText = f.file ? `<span class="file">${esc(f.file.name)}</span><small>${esc(f.fileNote)}</small>`
    : f.replaceMode ? `<small>Choose the new PDF for<br><b>${esc(f.name)}</b>.<br>The current one is kept in the removed folder.</small>`
    : f.id ? `<small>Current PDF is kept.<br>To replace it, use Settings, Clean Up.</small>`
    : `<small>Drag and drop a PDF file here<br>or click Browse...</small>`;
  return `
  <div class="form">
    <div class="drop" id="drop" data-act="browse">${icon("up")}<b>${f.replaceMode ? "New SDS PDF" : f.id ? "SDS PDF" : "Upload SDS PDF"}${f.id && !f.replaceMode ? "" : ' <span class="req">*</span>'}</b>${fileText}
      ${f.id && !f.replaceMode ? "" : '<button class="btn small" type="button">Browse...</button>'}
      <input type="file" id="file" accept="application/pdf,.pdf" hidden></div>
    <div class="fields">
      <label>Product Name: <span class="req">*</span></label>${inp("name")}
      <label>Manufacturer:</label>${inp("manufacturer", "The company named on the SDS, e.g. Ecolab Inc.")}
      <label>Supplier:</label>${inp("supplier", "Who you buy it from, e.g. Sysco")}
      <label>Also Known As:</label>${inp("aliases", "Other names staff or the supplier use, separated by commas")}
      <label>Category:</label><div><select class="field" data-f="category">${state.lib.categories.map(c => `<option ${c === f.category ? "selected" : ""}>${esc(c)}</option>`).join("")}</select>
        <div class="hint" id="cathelp" style="margin-top:3px">${esc(CAT_HELP[f.category] || "")}</div></div>
      <label>Also Used For:</label>${inp("tags", "Optional tags, separated by commas, e.g. Fryer cleaner, Grill cleaner")}
      <label>Storage Location:</label><div><input class="field" data-f="location" list="loclist" value="${esc(f.location)}" placeholder="Which storage area holds the shelf? Choose from the list">
        <datalist id="loclist">${locations().map(l => `<option value="${esc(l)}">`).join("")}</datalist></div>
      <label>SDS Revision Date:</label><input type="date" class="field" data-f="revisionDate" value="${esc(f.revisionDate)}" style="width:190px">
    </div>
  </div>
  <div class="sect"><h2>Emergency Summary (At-a-Glance)</h2><div>
    ${extractBox(f)}
    <p class="hint" style="margin:0 0 10px">Optional. Copy the key points from the sheet: Section 2 (hazards), Section 4 (first aid) and Section 8 (protective equipment). You can save now and fill this in later.</p>
    <div class="fields">
      <label>Signal Word:</label><select class="field" data-f="signalWord" style="width:190px">${["", "Warning", "Danger"].map(v => `<option value="${v}" ${v === f.signalWord ? "selected" : ""}>${v || "None on the SDS"}</option>`).join("")}</select>
      <label>Hazard Pictograms:</label><div><div class="picgrid">${GHS.map(([k, label]) => `<label><input type="checkbox" data-pic="${k}" ${f.pictograms.includes(k) ? "checked" : ""}>${PICTO[k]}${label}</label>`).join("")}</div>
        <div class="hint" style="margin-top:4px">Tick only the red diamonds printed in Section 2 of this product's SDS. Leave all unticked if it shows none.</div></div>
      <label>Hazards:</label>${ta("hazards", "e.g. Causes serious eye irritation. May cause skin irritation.")}
      <label>First Aid — Eyes:</label>${ta("firstAidEyes", "e.g. Rinse with water for at least 15 minutes.")}
      <label>First Aid — Skin:</label>${ta("firstAidSkin", "e.g. Wash with soap and water.")}
      <label>First Aid — Inhalation:</label>${ta("firstAidInhalation", "e.g. Move to fresh air.")}
      <label>First Aid — Swallowed:</label>${ta("firstAidSwallowed", "e.g. Rinse mouth. Do NOT induce vomiting. Call Poison Control.")}
      <label>Protective Equipment:</label>${ta("ppe", "e.g. Chemical-resistant gloves, safety goggles.")}
      <label>Verification:</label><div>
        <label style="display:flex;gap:9px;align-items:center;font-weight:400;cursor:pointer"><input type="checkbox" data-fc="verify" ${f.verify ? "checked" : ""}>I have checked this summary against the manufacturer's SDS${f.revisionDate ? " (revised " + fmtDate(f.revisionDate) + ")" : ""}</label>
        <div class="row" style="margin-top:6px"><span>Checked by:</span><input class="field" data-f="verifiedBy" value="${esc(f.verifiedBy)}" placeholder="Name or initials" style="width:220px" maxlength="40"><span class="hint grow">${status}</span></div></div>
    </div></div></div>
  <div class="row stickybar" style="justify-content:center">
    <button class="btn green big" data-act="save">${icon("save")}Save</button>
    <button class="btn wine big" data-act="cancel">${icon("x")}Cancel</button>
  </div>`;
}

function vCats() {
  const count = c => state.lib.products.filter(p => p.category === c).length;
  const cs = state.lib.categories;
  return `
  <div class="panel" style="max-width:860px">
    <p class="hint" style="margin-top:0">Categories group the library and set the order of sections in the printed book. Each product has one category. “${esc(state.lib.catchAll)}” always stays, and products from a removed category move into it.</p>
    ${cs.map((c, i) => `<div class="catrow"><div class="grow"><b>${esc(c)}</b><div class="hint">${esc(CAT_HELP[c] || "")}</div></div><span class="hint" style="white-space:nowrap">${count(c)} product${count(c) === 1 ? "" : "s"}</span>
      <button class="btn small" data-act="cat-move" data-i="${i}" data-d="-1" ${i === 0 || c === state.lib.catchAll ? "disabled" : ""} title="Move up">▲</button>
      <button class="btn small" data-act="cat-move" data-i="${i}" data-d="1" ${i >= cs.length - 2 || c === state.lib.catchAll ? "disabled" : ""} title="Move down">▼</button>
      <button class="btn small" data-act="cat-rename" data-i="${i}" ${c === state.lib.catchAll ? "disabled" : ""}>Rename</button>
      <button class="btn small wine" data-act="cat-remove" data-i="${i}" ${c === state.lib.catchAll ? "disabled" : ""}>Remove</button></div>`).join("")}
    <div class="row" style="margin-top:14px"><input id="newcat" class="field grow" placeholder="New category name" maxlength="40">
      <button class="btn green" data-act="cat-add">${icon("plus")}Add Category</button></div>
  </div>`;
}

async function sha256(buf) { const d = await crypto.subtle.digest("SHA-256", buf); return [...new Uint8Array(d)].map(b => b.toString(16).padStart(2, "0")).join(""); }

function wrapText(text, font, size, max) {
  const out = [];
  for (const para of clean(text).split(/\s*\n\s*/)) {
    let line = "";
    for (const w of para.split(" ")) { const tryL = line ? line + " " + w : w; if (font.widthOfTextAtSize(tryL, size) > max && line) { out.push(line); line = w; } else line = tryL; }
    out.push(line);
  }
  return out;
}

// One page a manager can hang up or hand to the emergency crew: the summary, with its checked/unchecked status.
async function emergencySheet(p) {
  const out = await PDFDocument.create(), F = {r: await out.embedFont(StandardFonts.Helvetica), b: await out.embedFont(StandardFonts.HelveticaBold), t: await out.embedFont(StandardFonts.TimesRomanBold)};
  const W = PAGE_W, H = PAGE_H, X = 46, MW = W - 2 * X, pg = out.addPage([W, H]); let y = H - 50;
  pg.drawRectangle({x: X - 6, y: y - 36, width: MW + 12, height: 52, borderColor: RED, borderWidth: 2});
  centre(pg, "EMERGENCY SHEET", F.t, 30, y - 24, RED); y -= 66;
  for (const l of wrapText(p.name, F.t, 24, MW)) { pg.drawText(l, {x: X, y, size: 24, font: F.t, color: INK}); y -= 28; }
  const meta = [p.manufacturer && "Manufacturer: " + p.manufacturer, p.location && "Stored: " + p.location, p.revisionDate && "SDS revision: " + fmtDate(p.revisionDate)].filter(Boolean).join("     ");
  if (meta) for (const l of wrapText(meta, F.r, 10.5, MW)) { pg.drawText(l, {x: X, y, size: 10.5, font: F.r, color: GREY}); y -= 14; }
  y -= 8;
  const st = summaryState(p);
  if (st.k !== "ok") { pg.drawRectangle({x: X - 6, y: y - 8, width: MW + 12, height: 28, color: RED}); centre(pg, "SUMMARY NOT VERIFIED - READ THE FULL SDS IN THE BINDER", F.b, 11, y + 2, rgb(1, 1, 1)); y -= 34; }
  const section = (title, body) => {
    pg.drawRectangle({x: X - 6, y: y - 4, width: MW + 12, height: 18, borderColor: WINE, borderWidth: 1});
    pg.drawText(clean(title).toUpperCase(), {x: X, y, size: 10.5, font: F.b, color: WINE}); y -= 20;
    for (const l of wrapText(body || "Not entered. See the full SDS.", F.r, 12, MW)) { pg.drawText(l, {x: X, y, size: 12, font: F.r, color: INK}); y -= 16; }
    y -= 8;
  };
  const pics = pictosOf(p).map(([, l]) => l).join(", ");
  section("Hazards" + (p.signalWord ? " - " + p.signalWord : ""), [pics && "Pictograms: " + pics, (p.hazards || "").trim()].filter(Boolean).join("\n"));
  section("First aid", [["Eyes", p.firstAidEyes], ["Skin", p.firstAidSkin], ["Inhalation", p.firstAidInhalation], ["Swallowed", p.firstAidSwallowed]].filter(([, v]) => (v || "").trim()).map(([k, v]) => k + ": " + v.trim()).join("\n"));
  section("Protective equipment (PPE)", (p.ppe || "").trim());
  pg.drawLine({start: {x: X, y: 92}, end: {x: W - X, y: 92}, thickness: 1, color: GREY});
  centre(pg, "Emergency: 911        Poison Control: 1-800-222-1222", F.b, 15, 66, RED);
  centre(pg, clean(`Quick reference only. The manufacturer's SDS is the authority.  Printed ${longDate()}.`), F.r, 8.5, 44, GREY);
  out.setTitle("Emergency Sheet - " + clean(p.name));
  return out.save();
}

async function backfillPages() {
  if (state.lib.readOnly) return;
  const need = state.lib.products.filter(p => !p.pages && !p.missing); if (!need.length) return;
  let did = false;
  for (const p of need) {
    try {
      const r = await fetch("/pdf/" + p.id); if (!r.ok) continue;
      const doc = await PDFDocument.load(await r.arrayBuffer(), {ignoreEncryption: true}), n = doc.getPageCount();
      if (n) { await api("/api/products/" + p.id + "/pages", {method: "POST", body: JSON.stringify({pages: n})}); did = true; }
    } catch (e) {}
  }
  if (did) { await reload(); if (state.view === "library") render(); }
}

async function upload(path, field, files) {
  const fd = new FormData(); for (const f of files) fd.append(field, f);
  return api(path, {method: "POST", body: fd});
}

function removeDialog(p) {
  showModal(`<h2>${icon("warn")}Remove SDS</h2><div class="body">
    <p style="margin-top:0">Are you sure you want to remove this product from the SDS library?</p>
    <div class="box"><dl><dt>Product:</dt><dd>${esc(p.name)}</dd><dt>Manufacturer:</dt><dd>${esc(p.manufacturer) || "—"}</dd><dt>Category:</dt><dd>${esc(p.category) || "—"}</dd></dl></div>
    <p>The product leaves the library and the printed book. Its details and PDF are kept for 30 days and it can be brought back from Settings, Clean Up. After that it is deleted permanently.</p></div>
    <div class="foot"><button class="btn wine big" data-act="remove-yes">${icon("trash")}Remove</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

async function takeFile(file) {
  const f = state.form;
  if (!file || !f || (f.id && !f.replaceMode)) return;
  if (!/\.pdf$/i.test(file.name) && file.type !== "application/pdf") return toast("That file is not a PDF.", true);
  f.file = file; f.pages = 0; f.fileNote = "";
  try { f.pages = (await PDFDocument.load(await file.arrayBuffer(), {ignoreEncryption: true})).getPageCount(); f.fileNote = f.pages + " page" + (f.pages === 1 ? "" : "s"); }
  catch (e) { f.fileNote = "Page count could not be read"; }
  if (!f.id && !f.name.trim()) { f.name = file.name.replace(/\.pdf$/i, "").replace(/[_-]+/g, " ").replace(/\s*\b(sds|msds)\b\s*/ig, " ").trim(); f.nameFromFile = true; }
  f.extract = {kind: "busy"}; render();
  // Read the sheet and draft the details. This happens for both Browse and drag-and-drop.
  try {
    const fd = new FormData(); fd.append("pdf", file);
    const d = await api("/api/extract", {method: "POST", body: fd});
    if (state.form === f && f.file === file) mergeDraft(f, d, false);
  } catch (e) { if (state.form === f) f.extract = {kind: "warn", msg: "The details could not be read from this PDF automatically. Type them from the sheet."}; }
  if (state.form === f) render();
}

// Fills the form from a drafted summary. Only empty fields are filled unless overwrite is set,
// so nothing a manager typed is lost. A draft is never counted as verified.
const DRAFT_KEYS = ["manufacturer", "signalWord", "hazards", "firstAidEyes", "firstAidSkin", "firstAidInhalation", "firstAidSwallowed", "ppe"];

function mergeDraft(f, d, overwrite) {
  if (d.unreadable) { f.extract = {kind: "warn", msg: d.unreadable}; return; }
  const set = (k, v) => { if (v && (overwrite || !String(f[k] || "").trim())) f[k] = v; };
  if (!f.id && d.name && (!f.name.trim() || f.nameFromFile)) { f.name = d.name; f.nameFromFile = false; }
  DRAFT_KEYS.forEach(k => set(k, d[k]));
  if (d.revisionDate && (overwrite || f.replaceMode || !f.revisionDate)) f.revisionDate = d.revisionDate;
  if ((d.pictograms || []).length && (overwrite || !f.pictograms.length)) f.pictograms = [...d.pictograms];
  f.verify = false;
  f.extract = {kind: "ok", found: d.found || [], missing: d.missing || [], notes: d.notes || [], draft: d};
}

function extractBox(f) {
  const x = f.extract, stored = f.id && !f.replaceMode;
  let box = "";
  if (x && x.kind === "busy") box = `<div class="note warn" style="margin-bottom:10px">Reading the PDF to draft the details...</div>`;
  else if (x && x.kind === "warn") box = `<div class="note warn" style="margin-bottom:10px"><span class="grow">${esc(x.msg)}</span></div>`;
  else if (x && x.kind === "ok") box = `<div class="note ok" style="margin-bottom:10px"><span class="grow"><b>Drafted from the PDF.</b> Check every line against the sheet before ticking “verified” below.${x.missing.length ? ` <b>Not found:</b> ${esc(x.missing.join(", "))}.` : ""}${x.notes.map(n => " " + esc(n)).join("")}</span>
    ${f.replaceMode ? `<button class="btn small" data-act="extract-overwrite">Replace Summary With This Sheet's Draft</button>` : ""}</div>`;
  if (stored) box += `<div class="row" style="margin:0 0 10px"><button class="btn small" data-act="extract-stored">${icon("search")}Fill Empty Fields From the PDF</button><span class="hint">Reads the stored sheet. Anything already typed is left alone.</span></div>`;
  return box;
}

async function saveForm() {
  const f = state.form;
  if (!f.name.trim()) return toast("Enter the product name.", true);
  if ((!f.id || f.replaceMode) && !f.file) return toast(f.replaceMode ? "Choose the new PDF first." : "Choose the SDS PDF for this product.", true);
  if (f.verify && !f.verifiedBy.trim()) return toast("Enter who checked the summary.", true);
  if (f.verify) remembered("by", f.verifiedBy.trim());
  const {file, fileNote, id, replaceMode, extract, nameFromFile, ...meta} = f;
  const fd = new FormData(); fd.append("meta", JSON.stringify(meta)); if (file) fd.append("pdf", file);
  const p = await api(id ? "/api/products/" + id : "/api/products", {method: "POST", body: fd});
  await reload(); state.form = null; state.view = "detail"; state.id = p.id; render(); backfillPages();
  toast(replaceMode ? "PDF replaced. The old one is in the removed folder." : id ? "Changes saved." : "SDS added to the library.");
}

const saveCats = (categories, renames = {}) => api("/api/categories", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({categories, renames})}).then(reload).then(render);

Object.assign(ACT, {
  open(el, e) { if (e.target.closest("input")) return; state.view = "detail"; state.id = el.dataset.id; render(); },
  toggle(el) { state.sel.has(el.dataset.id) ? state.sel.delete(el.dataset.id) : state.sel.add(el.dataset.id); render(); },
  selall() { visible().forEach(p => state.sel.add(p.id)); render(); },
  clear() { state.sel.clear(); render(); },
  sort(el) { const k = el.dataset.k; state.sort = {key: k, dir: state.sort.key === k ? -state.sort.dir : 1}; render(); },
  "fix-sheets"() { closeModal(); pendingBook = null; state.view = "settings"; state.tab = "diagnostics"; render(); loadTab(); },
  "open-incomplete"() { const b = pendingBook; pendingBook = null; closeModal(); if (b) openBook(b); },
  edit() { const p = byId(state.id); state.form = {...blankForm(), ...p, pictograms: p.pictograms || []}; state.view = "add"; render(); },
  replace(el) { const p = byId(el.dataset.id); state.form = {...blankForm(), ...p, pictograms: p.pictograms || [], replaceMode: true}; state.view = "add"; render(); },
  remove(el) { state.id = el.dataset.id; removeDialog(byId(state.id)); },
  async "remove-yes"() { const p = byId(state.id); await api("/api/products/" + state.id, {method: "DELETE"}); closeModal(); await reload(); render(); toast(`“${p.name}” was removed. It can be restored from Settings, Clean Up for 30 days.`); },
  "purge-ask"(el) { const p = (state.lib.removed || []).find(x => x.id === el.dataset.id) || {name: "this product"};
    showModal(`<h2>${icon("trash")}Delete Permanently</h2><div class="body"><p>Delete <b>${esc(p.name)}</b> and its PDF for good? This cannot be undone.</p></div>
    <div class="foot"><button class="btn wine big" data-act="purge-go" data-id="${el.dataset.id}">${icon("trash")}Delete Permanently</button><button class="btn big" data-act="close">Cancel</button></div>`); },
  async "purge-go"(el) { el.disabled = true; await api("/api/removed/" + el.dataset.id, {method: "DELETE"}); closeModal(); await reload(); render(); toast("Deleted permanently."); },
  async "restore-product"(el) { const p = await api("/api/removed/" + el.dataset.id + "/restore", {method: "POST"}); await reload(); render(); toast(`“${p.name}” is back in the library.`); },
  "view-removed"(el) { window.open("/pdf-removed/" + el.dataset.id, "_blank"); },
  "binder-mark"() { const lb = state.lib.lastBuild;
    if (!lb || !lb.complete || lb.confirmed) return toast("Print a complete book first. Only a complete book can be confirmed.", true);
    const n = Object.keys(lb.items || {}).length, what = lb.scope === "all" ? "entire book" : lb.scope === "index" ? "index" : "selected sheets";
    showModal(`<h2>${icon("list")}Confirm the Paper Binder</h2><div class="body">
      <p style="margin-top:0">Record that the printed ${what} is now in the paper binder?</p>
      <div class="box">Assembled ${fmtDate(lb.at)} at ${esc(lb.at.slice(11, 16))}: ${n} product${n === 1 ? "" : "s"}, ${lb.pages} pages, complete.
        ${lb.scope === "selected" ? "<br>Only these sheets are recorded as updated." : lb.scope === "index" ? "<br>Only the index is recorded as reprinted." : ""}</div>
      <p>Only confirm once the printed pages are actually in the binder. Assembling or previewing a book does not count.</p></div>
      <div class="foot"><button class="btn green big" data-act="binder-mark-go">Yes, It Is in the Binder</button><button class="btn big" data-act="close">Not Yet</button></div>`); },
  async "binder-mark-go"() { await api("/api/binder/confirm", {method: "POST"}); closeModal(); await reload(); render(); toast("Recorded: the paper binder was updated."); },
  async "binder-pulled"() { await api("/api/binder/pulled", {method: "POST"}); await reload(); render(); toast("Recorded: those sheets were taken out of the binder."); },
  "binder-select"() { const d = binderDiff(); state.sel = new Set([...d.add, ...d.replace].map(p => p.id)); state.q = ""; state.cat = ""; render(); toast("Changed sheets selected. Click Print Selected."); },
  async emerg() { const p = byId(state.id), bytes = await emergencySheet(p), stamp = ymd(new Date());
    showPreview(URL.createObjectURL(new Blob([bytes], {type: "application/pdf"})), true, `${p.name} Emergency Sheet ${stamp}.pdf`, `Emergency Sheet — ${p.name}`, "One page. Check it, then click Print."); },
  async "extract-stored"(el) { const f = state.form; el.disabled = true;
    try { mergeDraft(f, await api(`/api/products/${f.id}/extract`, {method: "POST"}), false); } finally { render(); } },
  "extract-overwrite"() { const f = state.form; if (f.extract && f.extract.draft) { mergeDraft(f, f.extract.draft, true); render(); toast("Summary replaced with the draft from the new sheet. Check it, then verify."); } },
  "open-product"(el) { state.view = "detail"; state.id = el.dataset.id; render(); },
  "import-pdfs"() { $("#importpdfs").click(); },
  "import-zip"() { $("#importzip").click(); },
  browse(el, e) { if (e.target.id !== "file") $("#file").click(); },
  save: saveForm,
  cancel() { const id = state.form && state.form.id; state.form = null; state.view = id ? "detail" : "library"; render(); },
  "cat-add"() { const v = $("#newcat").value.trim(); if (!v) return; const c = state.lib.categories; if (c.some(x => x.toLowerCase() === v.toLowerCase())) return toast("That category already exists.", true); return saveCats([...c.slice(0, -1), v, state.lib.catchAll]); },
  "cat-move"(el) { const c = [...state.lib.categories], i = +el.dataset.i, j = i + +el.dataset.d; [c[i], c[j]] = [c[j], c[i]]; return saveCats(c); },
  "cat-remove"(el) { const c = [...state.lib.categories], [gone] = c.splice(+el.dataset.i, 1); if (state.cat === gone) state.cat = ""; return saveCats(c); },
  "cat-rename"(el) { const old = state.lib.categories[+el.dataset.i];
    showModal(`<h2>${icon("edit")}Rename Category</h2><div class="body"><input id="rename" class="field" value="${esc(old)}" maxlength="40" data-old="${esc(old)}"></div><div class="foot"><button class="btn green big" data-act="cat-rename-go">Save</button><button class="btn big" data-act="close">Cancel</button></div>`); $("#rename").select(); },
  "cat-rename-go"() { const el = $("#rename"), old = el.dataset.old, v = el.value.trim(); if (!v || v === old) return closeModal(); const c = state.lib.categories; if (c.some(x => x !== old && x.toLowerCase() === v.toLowerCase())) return toast("That category already exists.", true); closeModal(); if (state.cat === old) state.cat = v; return saveCats(c.map(x => x === old ? v : x), {[old]: v}); },
});
