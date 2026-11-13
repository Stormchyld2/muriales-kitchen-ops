$('[data-act="pv-print"]').innerHTML = icon("print") + "Print";
$('[data-act="pv-save"]').innerHTML = icon("down") + "Save as PDF";
$('[data-act="pv-close"]').innerHTML = icon("x") + "Close";

document.addEventListener("click", e => {
  const el = e.target.closest("[data-act]"); if (!el || el.disabled) return;
  const act = e.target.closest("input[data-act]") ? e.target.dataset.act : el.dataset.act;
  const target = e.target.closest("input[data-act]") || el;
  Promise.resolve().then(() => ACT[act](target, e)).catch(err => { toast(err.message || String(err), true); if (act === "print-go") drawPrintDialog(); if (act === "print-one-go") el.disabled = false; });
});
document.addEventListener("input", e => {
  const t = e.target;
  if (t.id === "q") { state.q = t.value; state.focusQ = true; render(); }
  else if (t.dataset.f && state.form) { state.form[t.dataset.f] = t.value; if (t.dataset.f === "name") state.form.nameFromFile = false; if (t.dataset.f === "category") $("#cathelp").textContent = CAT_HELP[t.value] || ""; }
});
document.addEventListener("change", e => {
  const t = e.target;
  if (t.id === "tdays") loadTemps(+t.value);
  else if (t.id === "tfrom") { state.tfrom = t.value; if (state.tfrom && state.tto) loadTemps(0); }
  else if (t.id === "tto") { state.tto = t.value; if (state.tfrom && state.tto) loadTemps(0); }
  else if (t.id === "tpoint") { state.tpoint = t.value; render(); }
  else if (t.id === "trdate") state.trdate = t.value;
  else if (t.dataset.ch === "product" || t.dataset.ch === "kind") chemPick();
  else if (t.dataset.rp === "equipment") replPick(t);
  else if (t.dataset.act === "ca-filter") ACT["ca-filter"](t);
  else if (t.id === "tempcsv" && t.files.length) { const f = t.files[0]; t.value = ""; const say = m => { const el = $("#tempfilemsg"); if (el) el.textContent = m; };
    (async () => { try { say("Reading " + f.name + "..."); toast("Reading " + f.name + "..."); const buf = await f.arrayBuffer(); let csv;
      if (/\.xlsx$/i.test(f.name)) csv = (await api("/api/temps/xlsx", {method: "POST", body: buf})).csv;
      else { const u = new Uint8Array(buf), enc = u[0] === 0xFF && u[1] === 0xFE ? "utf-16le" : u[0] === 0xFE && u[1] === 0xFF ? "utf-16be" : "utf-8"; csv = new TextDecoder(enc).decode(buf).replace(/^\uFEFF/, ""); }
      if (!csv.trim()) throw new Error("That file is empty.");
      const pv = await api("/api/temps/import/preview", {method: "POST", body: JSON.stringify({csv})}); say(""); tempMapDialog(pv, {csv}); }
    catch (e) { say("Could not read " + f.name + ": " + (e.message || e)); toast("Could not read that file: " + (e.message || e), true); } })(); }
  else if (t.dataset.repMonth !== undefined) state.repMonth = t.value;
  else if (t.id === "sumMonth") { if (t.value) loadSummary(t.value); }
  else if (t.id === "cat") { state.cat = t.value; state.focusQ = false; render(); }
  else if (t.id === "file") takeFile(t.files[0]);
  else if (t.dataset.is === "priority") $("#priohint").textContent = state.maint.priorityUse[t.value] || "";
  else if (t.id === "pmtpl" && t.value !== "") { const x = state.tpls[+t.value]; const set = (k, v) => { const el = document.querySelector(`[data-pm=${k}]`); if (el) el.value = v; }; set("name", x[0]); set("every", x[1]); set("unit", x[2]); set("notes", x[3]); }
  else if (t.id === "importpdfs" && t.files.length) { const files = [...t.files]; t.value = "";
    upload("/api/import/pdfs", "pdf", files).then(async r => { await reload(); const sk = r.skipped || [];
      backfillPages();
      showModal(`<h2>${icon("up")}Import Finished</h2><div class="body"><p style="margin-top:0"><b>${r.added.length} PDF${r.added.length === 1 ? "" : "s"} added</b> and marked “needs review”. The manufacturer, SDS date and emergency summary were drafted from each sheet where they could be read. Open each one to check them and add the category and storage location.</p>
        ${sk.length ? `<p><b>${sk.length} skipped:</b></p><div class="box" style="max-height:180px;overflow:auto">${sk.map(x => `<div>${esc(x.name)} ${esc(x.reason)}</div>`).join("")}</div>` : ""}</div>
        <div class="foot"><button class="btn green big" data-act="close">OK</button></div>`); render(); }).catch(e => toast(e.message, true)); }
  else if (t.id === "importzip" && t.files.length) { const f = t.files[0]; t.value = "";
    upload("/api/import/backup", "zip", [f]).then(async () => { state.tab = "backup"; render(); await loadBackups(); toast("Library zip added to the backups list. Restore it below when you are ready."); }).catch(e => toast(e.message, true)); }
  else if (t.dataset.fc && state.form) state.form[t.dataset.fc] = t.checked;
  else if (t.dataset.pic && state.form) { const s = new Set(state.form.pictograms); t.checked ? s.add(t.dataset.pic) : s.delete(t.dataset.pic); state.form.pictograms = [...s]; }
  else if (t.dataset.f === "revisionDate" && state.form) render();
  else if (t.name === "scope") { printOpt.scope = t.value; drawPrintDialog(); }
  else if (t.id === "pprange") { const some = $('input[name="pp"][value="some"]'); if (some) some.checked = true; }
  else if (t.dataset.o) { printOpt[t.dataset.o] = t.checked; drawPrintDialog(); }
});
document.addEventListener("keydown", e => { if (e.key === "Escape" && $("#modal").className) closeModal(); if (e.key === "Enter" && e.target.id === "newcat") ACT["cat-add"](); if (e.key === "Enter" && e.target.id === "rename") ACT["cat-rename-go"](); if (e.key === "Enter" && e.target.id === "newarea") ACT["area-add"](); if (e.key === "Enter" && e.target.id === "renamearea") ACT["area-rename-go"](); });
for (const ev of ["dragover", "dragleave", "drop"]) document.addEventListener(ev, e => {
  if (state.view !== "add") { if (ev !== "dragleave") e.preventDefault(); return; }
  e.preventDefault(); const d = $("#drop"); if (d) d.classList.toggle("over", ev === "dragover");
  if (ev === "drop" && e.dataTransfer.files[0]) takeFile(e.dataTransfer.files[0]);
});

let misses = 0;
setInterval(() => refreshLock().catch(() => {}), 15000);
setInterval(() => fetch("/api/ping").then(r => { if (!r.ok) throw 0; misses = 0; $("#lost").style.display = "none"; }).catch(() => { if (++misses >= 2) $("#lost").style.display = "block"; }), 5000);
fetch("/api/ping");
addEventListener("pagehide", () => { try { fetch("/api/closing", { keepalive: true }); } catch (e) {} });
reload().then(() => { loadMaint().then(render).catch(() => {}); render(); refreshLock().catch(() => {}); backfillPages(); }).catch(e => toast(e.message, true));
