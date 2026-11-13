"use strict";

/* Shift handoff notes */

const openNotes = () => (state.lib.notes || []).filter(n => !n.done);

function notesBanner() {
  const n = openNotes().length; if (!n) return "";
  return `<div class="note warn"><span class="grow"><b>${n} handoff note${n === 1 ? "" : "s"} from other managers</b> ${n === 1 ? "is" : "are"} waiting to be read.</span><button class="btn small" data-act="nav" data-v="notes">Read Notes</button></div>`;
}

function vNotes() {
  const all = [...(state.lib.notes || [])].reverse(), open = all.filter(n => !n.done), done = all.filter(n => n.done);
  const when = s => `${fmtDate(s)} ${esc(String(s || "").slice(11, 16))}`;
  return `
  <div class="panel" style="max-width:860px">
    <h3 style="margin-top:0">Leave a note for the next manager</h3>
    <p class="hint">Things the next person should know: a sheet that is on order, a binder page that needs reprinting, a product that was discontinued.</p>
    <textarea id="notetext" class="field" rows="3" maxlength="2000" placeholder="Write the note here..."></textarea>
    <div class="row" style="margin-top:8px"><label style="margin:0">Your name:</label><input id="noteby" class="field" style="width:200px" value="${esc(remembered("by"))}"><button class="btn green" data-act="note-add">${icon("plus")}Add Note</button></div>
  </div>
  <div class="panel" style="max-width:860px">
    <div class="row" style="justify-content:space-between"><h3 style="margin:0">Open notes</h3><span><button class="btn small" data-act="rep-view" data-id="handoff">${icon("print")}View / Print Handoff</button> <button class="btn small green" data-act="handoff-email">${icon("ext")}Email Handoff</button></span></div>
    <div class="scrolly">${open.length ? open.map(n => `<div class="catrow"><div class="grow"><div style="white-space:pre-wrap">${esc(n.text)}</div><div class="hint">${esc(n.by) || "A manager"}, ${when(n.at)}</div></div><button class="btn small green" data-act="note-done" data-id="${n.id}">Done</button></div>`).join("") : `<p class="hint">No open notes.</p>`}</div>
  </div>
  ${done.length ? `<div class="panel" style="max-width:860px"><h3 style="margin-top:0">Finished notes</h3><div class="scrolly">${done.map(n => `<div class="catrow"><div class="grow"><div style="white-space:pre-wrap">${esc(n.text)}</div><div class="hint">${esc(n.by) || "A manager"}, ${when(n.at)} · finished ${when(n.doneAt)}${n.doneBy ? " by " + esc(n.doneBy) : ""}</div></div><button class="btn small" data-act="note-reopen" data-id="${n.id}">Reopen</button></div>`).join("")}</div></div>` : ""}`;
}

Object.assign(ACT, {
  async "handoff-email"() { if (!state.reports) state.reports = (await api("/api/reports")).reports || []; repEmail("handoff"); },
  async "note-add"() { const text = $("#notetext").value.trim(), by = $("#noteby").value.trim(); if (!text) return toast("Write the note first.", true);
    if (by) remembered("by", by); await api("/api/notes", {method: "POST", body: JSON.stringify({by, text})}); await reload(); render(); toast("Note added."); },
  async "note-done"(el) { await api(`/api/notes/${el.dataset.id}/done`, {method: "POST", body: JSON.stringify({by: remembered("by")})}); await reload(); render(); },
  async "note-reopen"(el) { await api(`/api/notes/${el.dataset.id}/reopen`, {method: "POST", body: "{}"}); await reload(); render(); },
});
