"use strict";

/* Reports and email */

const repPrint = id => window.open("/api/reports/" + encodeURIComponent(id) + "?print=1", "_blank");

function vReports() {
  if (!state.reports) { api("/api/reports").then(r => { state.reports = r.reports; render(); }); return `<p class="hint">Loading...</p>`; }
  const mods = [...new Set(state.reports.map(r => r.module))], month = state.repMonth || ymd(new Date()).slice(0, 7);
  return `<p class="hint" style="margin-top:0">Every report the program can produce, by module. <b>View</b> opens it in its own window to print. <b>Email</b> opens a message in Outlook with the report attached. You read it and press Send yourself; the program never sends anything.</p>` +
    mods.map(m => `<h3 style="margin:18px 0 6px">${esc(m)}</h3><div style="border:1px solid var(--line);border-radius:8px"><table class="plain"><tbody>
      ${state.reports.filter(r => r.module === m).map(r => `<tr><td style="width:34%"><b>${esc(r.title)}</b></td><td class="hint">${esc(r.about)}</td>
        <td style="white-space:nowrap;text-align:right">${r.month ? `<input type="month" class="field" data-rep-month value="${month}" style="width:150px;display:inline-block"> ` : ""}<button class="btn small" data-act="rep-view" data-id="${r.id}">${icon("print")}View</button> <button class="btn small green" data-act="rep-email" data-id="${r.id}">${icon("ext")}Email</button></td></tr>`).join("")}
    </tbody></table></div>`).join("");
}

const repQuery = id => { const r = state.reports.find(x => x.id === id); return r && r.month ? "?month=" + (document.querySelector("[data-rep-month]") || {value: state.repMonth || ymd(new Date()).slice(0, 7)}).value : ""; };

function repEmail(id) {
  const r = state.reports.find(x => x.id === id), month = repQuery(id);
  let last = ""; try { last = localStorage.getItem("repTo") || ""; } catch (e) {}
  showModal(`<h2>${icon("ext")}Email: ${esc(r.title)}</h2><div class="body" id="rebody" data-id="${id}" data-month="${esc(month.replace("?month=", ""))}"><div class="fields">
    <label>To: <span class="req">*</span></label><input class="field" data-re="to" value="${esc(last)}" placeholder="manager@example.com; another@example.com">
    <label>Subject:</label><input class="field" data-re="subject" placeholder="${esc(r.title)}">
    <label>Message:</label><textarea class="field" rows="4" data-re="message" placeholder="Optional note to go above the report"></textarea></div>
    <p class="hint">This opens a new email with the report written into it. Check it, then press Send.</p></div>
    <div class="foot"><button class="btn green big" data-act="rep-email-go">${icon("ext")}Open Email</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

Object.assign(ACT, {
  "rep-view"(el) { window.open("/api/reports/" + el.dataset.id + (state.reports ? repQuery(el.dataset.id) : ""), "_blank"); },
  "rep-email"(el) { repEmail(el.dataset.id); },
  async "rep-email-id"(el) { if (!state.reports) state.reports = (await api("/api/reports")).reports || []; repEmail(el.dataset.id); },
  async "rep-email-go"() { const v = grab("[data-re]", "re"), b = $("#rebody"); if (!v.to.trim()) return toast("Enter who to send it to.", true);
    try { localStorage.setItem("repTo", v.to); } catch (e) {}
    const o = await api("/api/reports/email", {method: "POST", body: JSON.stringify({...v, id: b.dataset.id, month: b.dataset.month})}); closeModal(); emailResult(o, "report"); },
  "rep-print"(el) { repPrint(el.dataset.id); },
});
