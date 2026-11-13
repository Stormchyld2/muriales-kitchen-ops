// Health inspection packet, and the daily summary / backup copy settings.
const PK = {parts: null, from: "", to: "", off: new Set()};

async function loadPacket() { PK.parts = (await api("/api/packet/parts")).parts; if (!PK.from) { const d = new Date(); PK.to = ymd(d); d.setDate(d.getDate() - 29); PK.from = ymd(d); } render(); }

function pkUrl(print) {
  const keys = PK.parts.filter(p => !PK.off.has(p.key)).map(p => p.key).join(",");
  return "/api/reports/packet:" + PK.from + ":" + PK.to + ":" + keys + (print ? "?print=1" : "");
}

function vPacket() {
  if (!PK.parts) { loadPacket(); return `<div class="empty"><b>Loading...</b></div>`; }
  const groups = [...new Set(PK.parts.map(p => p.group))];
  const range = (days, label) => `<button class="btn small" data-act="pk-range" data-d="${days}">${label}</button>`;
  return `<div class="panel" style="max-width:980px">
    <p class="hint" style="margin-top:0">One report with everything an inspector asks for, set out in numbered parts with a contents page and a sign-off line. Pick the dates, untick anything you do not want, then view or print it (print to PDF to keep a copy).</p>
    <div class="row" style="align-items:flex-end;flex-wrap:wrap;gap:10px"><label>From<br><input id="pk-from" type="date" value="${esc(PK.from)}"></label><label>To<br><input id="pk-to" type="date" value="${esc(PK.to)}"></label>
      ${range(30, "Last 30 days")}${range(90, "Last 90 days")}${range(365, "Last year")}
      <span style="margin-left:auto"></span><button class="btn green big" data-act="pk-open">${icon("print")}View / Print Packet</button><button class="btn big" data-act="pk-email">${icon("ext")}Email Packet</button></div>
    <p class="hint">Temperatures, cleaning, training given, incidents, meetings, lockout/tagout and corrective actions follow the dates. The matrix, SDS library, equipment and binder show how things stand today.</p>
    ${groups.map((g, i) => `<h3 style="margin:14px 0 4px">Part ${i + 1}: ${esc(g)}</h3><div class="row" style="flex-wrap:wrap;gap:6px 18px">${PK.parts.filter(p => p.group === g).map(p => `<label style="display:flex;gap:6px;align-items:center"><input type="checkbox" data-pk="${esc(p.key)}" ${PK.off.has(p.key) ? "" : "checked"}> ${esc(p.title)}</label>`).join("")}</div>`).join("")}
  </div>`;
}

function pkRead() { PK.from = $("#pk-from").value || PK.from; PK.to = $("#pk-to").value || PK.to; document.querySelectorAll("[data-pk]").forEach(c => c.checked ? PK.off.delete(c.dataset.pk) : PK.off.add(c.dataset.pk)); }

// ---- daily summary and backup copy (a Settings tab) ----
async function loadDsum() { state.dsum = await api("/api/summary/settings"); render(); }
function tDsum() {
  const d = state.dsum || {recipients: "", copyFolder: ""};
  return `<div class="panel" style="max-width:860px"><h3 style="margin-top:0">Daily summary</h3>
    <p class="hint">A short text of how the day stands: what is overdue, the gauges, alerts and urgent items. The program cannot send mail on its own. The Email Daily Summary button on the dashboard opens a ready-made message to this list and you press Send. To get it as a text, add your phone's email-to-text address (for example 5551234567@vtext.com for Verizon) to the list.</p>
    <label>Send to (email addresses separated by commas)</label><textarea id="ds-to" class="field" rows="3" placeholder="you@example.com, manager@example.com">${esc(d.recipients)}</textarea>
    <p class="hint">Take your own address out of this list when you leave so the summary stops coming to you.</p>
    <label style="margin-top:8px;display:block">Send it by itself every day at (24-hour time, blank for off)</label><input id="ds-auto" class="field" style="max-width:120px" value="${esc(d.autoSend || "")}" placeholder="16:00">
    <p class="hint">This sends through the Outlook that is signed in on this computer, with no window and no password. It needs classic Outlook (the new Outlook cannot be controlled by other programs), the program left running, and it only sends from the computer that has the program open for editing, so two computers never send it twice. If it fails, an alert shows on the dashboard.${d.lastSent ? ` Last sent: ${esc(d.lastSent)}.` : ""}${d.lastError ? ` <b>Last problem: ${esc(d.lastError)}</b>` : ""}</p>
    <div class="row"><button class="btn green" data-act="dsum-save">Save</button><button class="btn" data-act="dsum-send">Send a Test Now</button><button class="btn" data-act="dsum-email">${icon("ext")}Open as a Draft</button><button class="btn" data-act="rep-view" data-id="daily-summary">${icon("print")}Preview</button></div></div>
  <div class="panel" style="max-width:860px"><h3 style="margin-top:0">Second place for backups</h3>
    <p class="hint">Each day the program zips everything into Kitchen Backups beside it. Type a folder here, for example one inside OneDrive, and each daily zip is also copied there (the last 14 are kept). Leave it blank to keep backups in one place.</p>
    <label>Backup copy folder (full path)</label><input id="ds-copy" class="field" value="${esc(d.copyFolder)}" placeholder="C:\\Users\\Name\\OneDrive - Company\\Kitchen Backups">
    <div class="row" style="margin-top:8px"><button class="btn green" data-act="dsum-save">Save Folder</button></div></div>`;
}

Object.assign(ACT, {
  "pk-range"(el) { pkRead(); const d = new Date(); PK.to = ymd(d); d.setDate(d.getDate() - (+el.dataset.d - 1)); PK.from = ymd(d); render(); },
  "pk-open"() { pkRead(); if (PK.to < PK.from) return toast("The end date is before the start date."); window.open(pkUrl(true), "_blank"); },
  async "pk-email"() { pkRead(); if (PK.to < PK.from) return toast("The end date is before the start date."); if (!state.reports) state.reports = (await api("/api/reports")).reports || []; const id = "packet:" + PK.from + ":" + PK.to + ":" + PK.parts.filter(p => !PK.off.has(p.key)).map(p => p.key).join(","); state.reports.push({id, title: "Health Inspection Packet"}); repEmail(id); },
  async "dsum-save"() {
    const to = $("#ds-to"), cp = $("#ds-copy"), cur = state.dsum || {};
    state.dsum = await api("/api/summary/settings", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({recipients: to ? to.value : cur.recipients, copyFolder: cp ? cp.value : cur.copyFolder, autoSend: $("#ds-auto") ? $("#ds-auto").value : cur.autoSend})});
    render(); toast("Saved.");
  },
  async "dsum-send"() { try { await api("/api/summary/send", {method: "POST"}); toast("Sent. Check your inbox or phone."); } catch (e) { toast(e.message); } },
  async "dsum-email"() {
    const r = await api("/api/summary/email", {method: "POST"});
    toast(r.method === "outlook" ? "A draft is open in Outlook. Check it and press Send." : "A draft was opened in your mail program. Check it and press Send.");
  },
});
