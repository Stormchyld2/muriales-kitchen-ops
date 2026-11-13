"use strict";

/* Edit history: who changed what and when (Settings > Edit History). */

async function loadHistory(q) {
  try { state.hist = await api("/api/history?limit=400" + (q ? "&q=" + encodeURIComponent(q) : "")); state.hist.q = q || ""; }
  catch (e) { state.hist = {rows: [], total: 0, error: e.message, q: q || ""}; }
  if (state.view === "settings" && state.tab === "history") render();
}

function tHistory() {
  const h = state.hist;
  if (!h) { loadHistory(""); return `<div class="empty"><b>Loading...</b></div>`; }
  const when = s => { const t = new Date(String(s).replace(" ", "T")); return isNaN(t) ? s : t.toLocaleString([], {month: "short", day: "numeric", year: "numeric", hour: "numeric", minute: "2-digit"}); };
  const brief = r => { if (r.action === "Deleted" && r.detail[0] === "{") { try { return Object.entries(JSON.parse(r.detail)).filter(([k, v]) => v && k !== "id").map(([k, v]) => v).slice(0, 4).join(" · "); } catch (e) {} } return r.detail || ""; };
  return `<div class="panel" style="margin-bottom:10px"><h3 style="margin:0 0 4px">Edit History</h3>
    <p class="hint" style="margin:0 0 8px">Every entry added, changed, deleted or marked done is recorded here with who and when. Names are the ones typed on the forms. Nothing here can be edited. It is kept in <b>Maintenance Data\\Edit History.jsonl</b>.</p>
    <div class="row"><input class="field" id="hq" placeholder="Search by name, record, word..." value="${esc(h.q)}" style="max-width:340px"><button class="btn" data-act="hist-search">Search</button><button class="btn" data-act="hist-clear">Show All</button><span class="hint">Showing ${h.rows.length} of ${h.total}</span></div></div>
    <div class="tablewrap">${!h.rows.length ? `<div class="empty"><b>${h.error ? "Could not load" : "Nothing recorded yet"}</b>${esc(h.error || "Changes appear here as soon as someone saves something.")}</div>` : `<table class="plain"><thead><tr>${["When", "Who", "What", "Record", "Details"].map(x => `<th class="nosort" style="width:auto">${x}</th>`).join("")}</tr></thead><tbody>
      ${h.rows.map((r, i) => `<tr data-act="hist-row" data-i="${i}" style="cursor:pointer"><td style="white-space:nowrap">${esc(when(r.at))}</td><td>${esc(r.who || "-")}</td><td><span class="status ${r.action === "Deleted" ? "Overdue" : r.action === "Changed" ? "Soon" : "OK"}">${esc(r.action)}</span></td><td>${esc(r.kind)}${r.label ? " · " + esc(r.label) : ""}</td><td class="hint" style="max-width:360px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${esc(brief(r))}</td></tr>`).join("")}</tbody></table>`}</div>`;
}

Object.assign(ACT, {
  async "hist-search"() { await loadHistory(($("#hq") || {}).value || ""); },
  async "hist-clear"() { await loadHistory(""); },
  "hist-row"(el) {
    const r = state.hist.rows[+el.dataset.i]; let body = esc(r.detail || "No further detail was kept for this one.");
    if (r.detail && r.detail[0] === "{") { try { body = Object.entries(JSON.parse(r.detail)).filter(([, v]) => v).map(([k, v]) => `<tr><td class="hint" style="padding-right:12px">${esc(k)}</td><td>${esc(v)}</td></tr>`).join(""); body = `<table class="plain"><tbody>${body}</tbody></table>`; } catch (e) {} }
    else if (r.detail && r.detail.includes("; ")) body = r.detail.split("; ").map(x => `<div>${esc(x)}</div>`).join("");
    showModal(`<h2>${icon("list")}${esc(r.action)} · ${esc(r.kind)}</h2><div class="body"><p><b>${esc(r.label || r.id || "")}</b><br><span class="hint">${esc(r.who || "Nobody named")} · ${esc(r.at.replace("T", " "))}${r.id ? " · " + esc(r.id) : ""}</span></p><div>${body}</div></div><div class="foot"><button class="btn" data-act="close">Close</button></div>`);
  },
});
