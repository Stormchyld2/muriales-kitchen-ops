"use strict";

/* The "someone is editing" lock */

function lockBanner() {
  const L = state.lock, ro = L ? L.readOnly : state.lib.readOnly;
  if (!ro) return "";
  const h = L && L.holder;
  return `<div class="note bad" style="margin-bottom:10px"><span class="grow"><b>View only.</b> ${h ? esc(h.who) + " is editing the library" + (h.since ? " (since " + fmtDate(h.since) + " " + esc(h.since.slice(11, 16)) + ")" : "") : "The library is being edited on another computer"}. You can search, view and print, but changes are turned off here so nothing gets overwritten.</span>
    <button class="btn small green" data-act="lock-acquire">Start Editing</button><button class="btn small" data-act="lock-takeover-ask">Take Over...</button></div>`;
}

async function refreshLock() {
  const was = JSON.stringify(state.lock && [state.lock.readOnly, state.lock.holder && state.lock.holder.who]);
  state.lock = await api("/api/lock");
  if (JSON.stringify([state.lock.readOnly, state.lock.holder && state.lock.holder.who]) !== was && !$("#modal").className && !(state.view === "add")) render();
}

Object.assign(ACT, {
  async "lock-acquire"() { await api("/api/lock/acquire", {method: "POST"}); await reload(); await refreshLock(); render(); toast("You can now make changes."); },
  "lock-takeover-ask"() { const h = state.lock && state.lock.holder;
    showModal(`<h2>${icon("warn")}Take Over Editing</h2><div class="body"><p style="margin-top:0">${h ? esc(h.who) + " is" : "Someone else is"} editing${h && h.live ? " right now" : ""}. If you take over, they can no longer make changes and anything they have not saved is lost.</p>
      <p>Only do this if you are sure they have finished, for example if that computer was turned off or crashed.</p></div>
      <div class="foot"><button class="btn wine big" data-act="lock-takeover">Take Over</button><button class="btn big" data-act="close">Cancel</button></div>`); },
  async "lock-takeover"() { await api("/api/lock/takeover", {method: "POST"}); closeModal(); await reload(); await refreshLock(); render(); toast("You are now the editor."); },
});
