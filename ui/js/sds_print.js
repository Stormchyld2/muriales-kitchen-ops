"use strict";

/* SDS Manager: printing and the printed book */

let printOpt = {scope: "all", cover: true, index: true, dividers: true, numbers: false, eco: false, only: null};

function printItems() {
  const src = printOpt.scope === "selected" ? state.lib.products.filter(p => state.sel.has(p.id)) : state.lib.products;
  const order = c => { const i = state.lib.categories.indexOf(c); return i < 0 ? 999 : i; };
  return [...src].sort((a, b) => order(a.category) - order(b.category) || a.name.localeCompare(b.name));
}

// Index page geometry. The print dialog's estimate and the PDF builder both use
// indexPageCount, so the two can never disagree about how long the index is.
const PAGE_W = 612, PAGE_H = 792, IDX = {firstTop: PAGE_H - 122, nextTop: PAGE_H - 50, row: 18, bottom: 50};

function indexPageCount(rows) {
  let pages = 1, y = IDX.firstTop - IDX.row;
  for (let i = 0; i < rows; i++) { if (y < IDX.bottom) { pages++; y = IDX.nextTop - IDX.row; } y -= IDX.row; }
  return pages;
}

function estimate() {
  const items = printItems(), o = printOpt;
  if (o.scope === "index") return {n: items.length, pages: indexPageCount(items.length)};
  let pages = items.reduce((s, p) => s + (p.pages || 0), 0);
  if (o.cover) pages++;
  if (o.index) pages += indexPageCount(items.length);
  if (o.dividers) pages += new Set(items.map(p => p.category)).size;
  return {n: items.length, pages, unknown: items.some(p => !p.pages)};
}

function printDialog(scope) {
  printOpt = {...printOpt, scope, only: null};
  if (scope === "selected" && !state.sel.size) printOpt.scope = "all";
  drawPrintDialog();
}

function drawPrintDialog() {
  const o = printOpt, e = estimate(), idxOnly = o.scope === "index";
  const radio = (v, label, off) => `<label class="${off ? "off" : ""}"><input type="radio" name="scope" value="${v}" ${o.scope === v ? "checked" : ""} ${off ? "disabled" : ""}>${label}</label>`;
  const box = (k, label, off) => `<label class="${off ? "off" : ""}"><input type="checkbox" data-o="${k}" ${o[k] || off === "on" ? "checked" : ""} ${off ? "disabled" : ""}>${label}</label>`;
  showModal(`<h2>${icon("print")}Print SDS Book</h2><div class="body">
    <div class="cols"><div><h4>What Would You Like to Print?</h4>
      ${radio("all", `Entire Book (all ${state.lib.products.length} SDS sheets)`)}
      ${radio("selected", `Selected Sheets Only (${state.sel.size} product${state.sel.size === 1 ? "" : "s"})`, !state.sel.size)}
      ${radio("index", "Index Only")}</div>
    <div><h4>${idxOnly ? "The Book the Index Is For" : "Include in Printout"}</h4>
      ${box("cover", "Cover Page (Muriale's Logo)")}${box("index", "Index of Chemicals", idxOnly ? "on" : "")}${box("dividers", "Category Divider Pages")}${box("numbers", "Add Binder Page Numbers (shrinks each page 3%)")}${box("eco", "Printer-friendly (saves ink)")}</div></div>
    <div class="box"><b>Print Summary</b><br>Products: ${e.n} &nbsp;&nbsp;&nbsp; Estimated Pages: ${e.unknown ? "at least " : ""}${e.pages}
      <br><span style="font-size:13px">${idxOnly ? "Index pages only. Its page references match a book printed with the options ticked above." : o.numbers ? "Each page gets a binder page number in a strip at the bottom. Manufacturer pages are shrunk about 3% to make room, so the number never covers anything on the sheet." : "The manufacturer's pages are copied exactly as they are, with nothing added to them. The index lists the page each SDS starts on, counting every page of the book."}
      The exact page count is confirmed on the preview before anything prints.</span></div>
    <p id="busy" style="margin:0;min-height:20px;font-weight:600"></p></div>
    <div class="foot"><button class="btn green big" data-act="print-go" ${e.n ? "" : "disabled"}>${icon("print")}Preview &amp; Print</button><button class="btn big" data-act="close">${icon("x")}Cancel</button></div>`);
}

/* ---------- building the printed book ---------- */
const {PDFDocument, StandardFonts, rgb, degrees} = PDFLib;

const WINE = rgb(0.48, 0.07, 0.13), GOLD = rgb(0.72, 0.56, 0.22), INK = rgb(0.1, 0.1, 0.1), GREY = rgb(0.35, 0.35, 0.35), RED = rgb(0.78, 0.05, 0.1);

// The built-in PDF fonts only hold basic Latin characters, so anything else is swapped out.
const BAND = 22;

const rotOf = pg => ((pg.getRotation().angle % 360) + 360) % 360;

const clean = s => String(s ?? "").replace(/[‘’]/g, "'").replace(/[“”]/g, '"').replace(/[–—]/g, "-").replace(/®|™/g, "").replace(/\s+/g, " ").replace(/[^\x20-\x7E\xA1-\xFF]/g, "?").trim();

function fit(text, font, size, max) {
  let t = clean(text);
  if (font.widthOfTextAtSize(t, size) <= max) return t;
  while (t.length > 1 && font.widthOfTextAtSize(t + "...", size) > max) t = t.slice(0, -1);
  return t.trimEnd() + "...";
}

const centre = (page, text, font, size, y, color = INK) => page.drawText(text, {x: (page.getWidth() - font.widthOfTextAtSize(text, size)) / 2, y, size, font, color});

// loadSheet fetches one product's PDF and says plainly why it cannot be used, if it cannot.
async function loadSheet(p) {
  let bytes;
  try { const r = await fetch("/pdf/" + p.id); if (!r.ok) throw 0; bytes = await r.arrayBuffer(); }
  catch (e) { return {p, why: "its PDF file is missing from the library folder"}; }
  if (p.sha256 && await sha256(bytes) !== p.sha256) return {p, why: "its PDF is not the file that was added (it was changed outside the program). Check it in Settings, Diagnostics, then accept it or replace it"};
  let doc;
  try { doc = await PDFDocument.load(bytes, {ignoreEncryption: true}); }
  catch (e) { return {p, why: "its PDF is damaged and cannot be read"}; }
  if (doc.isEncrypted) return {p, why: "its PDF is password-protected, so it cannot be combined into a book. Print it on its own from its product page"};
  let n = 0; try { n = doc.getPageCount(); } catch (e) {}
  if (!n) return {p, why: "its PDF has no readable pages"};
  return {p, doc, n};
}

// stampFooter writes a small binder page number in the bottom right corner of
// a page as it will be seen, whatever the page's size or rotation. Nothing is
// painted behind it, so it can never cover the manufacturer's text.
function stampFooter(pg, text, font) {
  const size = 7.5, tw = font.widthOfTextAtSize(text, size), box = pg.getCropBox();
  const rot = ((pg.getRotation().angle % 360) + 360) % 360;
  const seenWidth = rot % 180 ? box.height : box.width, vx = seenWidth - tw - 20, vy = 9;
  const at = rot === 90 ? [box.x + box.width - vy, box.y + vx] : rot === 180 ? [box.x + box.width - vx, box.y + box.height - vy]
    : rot === 270 ? [box.x + vy, box.y + box.height - vx] : [box.x + vx, box.y + vy];
  pg.drawText(text, {x: at[0], y: at[1], size, font, color: GREY, rotate: degrees(rot)});
}

// buildBook assembles the PDF. It first opens every sheet, so that it knows the
// true page counts and any sheet that cannot be included before a single page
// is drawn. A book with a sheet left out is stamped INCOMPLETE on its cover and index.
async function buildBook(opt, progress = () => {}) {
  const items = opt.items, W = PAGE_W, H = PAGE_H, today = longDate();
  const loaded = [];
  for (const [i, p] of items.entries()) { progress(`Checking sheet ${i + 1} of ${items.length}...`); loaded.push(await loadSheet(p)); }
  const good = loaded.filter(x => x.doc), problems = loaded.filter(x => !x.doc).map(x => ({name: x.p.name, why: x.why}));
  const complete = problems.length === 0;

  // Work out where everything lands before drawing, so the index can cite real page numbers.
  let page = 0; const starts = {}, dividerBefore = new Set();
  if (opt.cover) page++;
  if (opt.index) page += indexPageCount(items.length);
  let last = null;
  for (const x of good) {
    if (opt.dividers && x.p.category !== last) { last = x.p.category; dividerBefore.add(x.p.id); page++; }
    starts[x.p.id] = page + 1; page += x.n;
  }
  const total = page;

  const out = await PDFDocument.create();
  const F = {r: await out.embedFont(StandardFonts.Helvetica), b: await out.embedFont(StandardFonts.HelveticaBold), t: await out.embedFont(StandardFonts.TimesRomanBold)};
  const ink = opt.eco ? WINE : rgb(1, 1, 1);
  const bar = (pg, x, y, w, h) => pg.drawRectangle(opt.eco ? {x, y, width: w, height: h, borderColor: WINE, borderWidth: 1.5} : {x, y, width: w, height: h, color: WINE});
  const logo = await out.embedPng(await (await fetch("logo.png")).arrayBuffer());
  out.setTitle((complete ? "" : "INCOMPLETE - ") + "Muriale's SDS Book - " + today); out.setAuthor("Muriale's Italian Kitchen");
  // Binder page numbers go in a strip of their own at the foot of the page.
  const footer = (pg, n, band) => { const t = `Binder page ${n} of ${total}`, size = 7.5;
    pg.drawText(t, {x: (pg.getWidth() - F.r.widthOfTextAtSize(t, size)) / 2, y: Math.max(2, band / 2 - 2.5), size, font: F.r, color: GREY}); };
  const numberGenerated = pg => { if (opt.numbers && !opt.indexOnly) footer(pg, out.getPageCount(), 30); };
  const warn = pg => { if (!complete) { pg.drawRectangle({x: 36, y: H - 30, width: W - 72, height: 20, color: RED});
    centre(pg, `INCOMPLETE - ${problems.length} SDS sheet${problems.length === 1 ? " is" : "s are"} missing from this book`, F.b, 11, H - 24, rgb(1, 1, 1)); } };

  if (opt.cover && !opt.indexOnly) {
    const pg = out.addPage([W, H]);
    pg.drawRectangle({x: 30, y: 30, width: W - 60, height: H - 60, borderColor: GOLD, borderWidth: 2.5});
    pg.drawRectangle({x: 38, y: 38, width: W - 76, height: H - 76, borderColor: GOLD, borderWidth: 0.8});
    const lw = 400, lh = lw * logo.height / logo.width;
    pg.drawImage(logo, {x: (W - lw) / 2, y: 470, width: lw, height: lh});
    bar(pg, 38, 358, W - 76, 70);
    centre(pg, "SAFETY DATA SHEETS", F.t, 38, 380, ink);
    centre(pg, "Printed: " + today, F.b, 15, 318);
    centre(pg, good.length + " product" + (good.length === 1 ? "" : "s") + " in this book", F.r, 12, 296, GREY);
    centre(pg, "Emergency: 911     Poison Control: 1-800-222-1222", F.b, 14, 96, WINE);
    if (!complete) { pg.drawRectangle({x: 38, y: 200, width: W - 76, height: 56, color: RED});
      centre(pg, "INCOMPLETE BOOK", F.b, 22, 232, rgb(1, 1, 1)); centre(pg, `${problems.length} SDS sheet${problems.length === 1 ? " is" : "s are"} missing. See the index.`, F.b, 12, 210, rgb(1, 1, 1)); }
  }

  if (opt.index) {
    const paged = true; // the index always says which page each SDS starts on
    const fixed = [["Manufacturer", 96], ["Category", 84], ["Storage Location", 84], ["SDS Date", 54]];
    const cols = [["Product Name", 168], ...fixed, ["Page", 54]];
    const X = 36, RH = IDX.row;
    let pg, y, n = 0;
    const head = first => {
      pg = out.addPage([W, H]); warn(pg); numberGenerated(pg);
      if (first) {
        const lw = 128, lh = lw * logo.height / logo.width;
        pg.drawImage(logo, {x: X, y: H - 40 - lh, width: lw, height: lh});
        pg.drawText("Muriale's SDS Book Index", {x: 184, y: H - 74, size: 22, font: F.t, color: INK});
        pg.drawText("Page numbers count every page of the book, starting at 1.", {x: 184, y: H - 92, size: 8, font: F.r, color: GREY});
        for (const [i, t] of ["Safety Data Sheets Index", "Printed: " + today].entries())
          pg.drawText(t, {x: W - X - F.r.widthOfTextAtSize(t, 9), y: H - 56 - i * 12, size: 9, font: F.r, color: INK});
        y = IDX.firstTop;
      } else y = IDX.nextTop;
      bar(pg, X, y - 5, W - 2 * X, RH);
      let x = X;
      for (const [label, w] of cols) { pg.drawText(label, {x: x + 4, y, size: 9, font: F.b, color: ink}); x += w; }
      y -= RH;
    };
    head(true);
    for (const p of items) {
      if (y < IDX.bottom) head(false);
      n++;
      const gone = !starts[p.id];
      if (n % 2 === 0 && !opt.eco) pg.drawRectangle({x: X, y: y - 5, width: W - 2 * X, height: RH, color: rgb(0.95, 0.94, 0.91)});
      pg.drawLine({start: {x: X, y: y - 5}, end: {x: W - X, y: y - 5}, thickness: 0.4, color: rgb(0.7, 0.7, 0.7)});
      const cells = [p.name, p.manufacturer, p.category, p.location, sdsDate(p)];
      if (paged) cells.push(gone ? "MISSING" : String(starts[p.id])); else cells.unshift(String(n));
      let x = X;
      for (const [i, v] of cells.entries()) {
        const lastCol = paged && i === cells.length - 1;
        pg.drawText(fit(v, lastCol ? F.b : F.r, lastCol && gone ? 7 : 9, cols[i][1] - 7), {x: x + 4, y, size: lastCol && gone ? 7 : 9, font: lastCol ? F.b : F.r, color: gone ? RED : INK}); x += cols[i][1];
      }
      y -= RH;
    }
  }

  if (!opt.indexOnly) {
    let done = 0;
    for (const x of good) {
      progress(`Adding sheet ${++done} of ${good.length}...`);
      if (dividerBefore.has(x.p.id)) {
        const pg = out.addPage([W, H]), count = good.filter(q => q.p.category === x.p.category).length;
        numberGenerated(pg);
        bar(pg, 0, 500, W, 84);
        centre(pg, fit(x.p.category || state.lib.catchAll, F.t, 38, W - 80), F.t, 38, 530, ink);
        centre(pg, count + " product" + (count === 1 ? "" : "s"), F.b, 13, 468, GREY);
        centre(pg, clean(CAT_HELP[x.p.category] || ""), F.r, 11, 446, GREY);
      }
      const idxs = x.doc.getPageIndices();
      if (!opt.numbers) { (await out.copyPages(x.doc, idxs)).forEach(c => out.addPage(c)); continue; }
      // Upright pages are shrunk about 3% into the upper part of a page of the same size, so the
      // number can never cover anything printed on the manufacturer's page. Pages that are
      // rotated keep their own geometry and get a small number in the corner instead.
      const upright = idxs.filter(i => rotOf(x.doc.getPage(i)) === 0), turned = idxs.filter(i => !upright.includes(i));
      const boxes = upright.map(i => { const b = x.doc.getPage(i).getCropBox(); return {left: b.x, bottom: b.y, right: b.x + b.width, top: b.y + b.height}; });
      const emb = upright.length ? await out.embedPages(upright.map(i => x.doc.getPage(i)), boxes) : [];
      const copies = turned.length ? await out.copyPages(x.doc, turned) : [];
      for (const i of idxs) {
        const u = upright.indexOf(i);
        if (u < 0) { const pg = out.addPage(copies[turned.indexOf(i)]); try { stampFooter(pg, `Binder page ${out.getPageCount()} of ${total}`, F.r); } catch (e) {} continue; }
        const b = boxes[u], w = b.right - b.left, h = b.top - b.bottom, band = Math.min(BAND, h * 0.08), s = (h - band) / h;
        const np = out.addPage([w, h]);
        np.drawPage(emb[u], {x: (w - w * s) / 2, y: band, width: w * s, height: h * s});
        footer(np, out.getPageCount(), band);
      }
    }
    if (out.getPageCount() !== total) throw new Error("The book did not come out at the expected length. Nothing was printed; please try again.");
  }
  return {bytes: await out.save(), pages: out.getPageCount(), bookPages: total, problems, complete, starts, included: good.length};
}

let pv = {url: null, name: "", blob: false};

function showPreview(url, blob, name, title, note) {
  if (pv.url && pv.blob) URL.revokeObjectURL(pv.url);
  pv = {url, name, blob};
  $("#pvTitle").textContent = title; $("#pvNote").textContent = note;
  $("#pvFrame").src = url; $("#preview").className = "show";
}

let pendingBook = null;

function openBook(book) {
  const o = printOpt, idxOnly = o.scope === "index";
  const url = URL.createObjectURL(new Blob([book.bytes], {type: "application/pdf"}));
  const stamp = ymd(new Date()), pages = `${book.pages} page${book.pages === 1 ? "" : "s"}, confirmed`;
  if (idxOnly) return showPreview(url, true, `Muriales SDS Index ${stamp}.pdf`, `SDS Index — ${pages}`, "Check the pages, then click Print.");
  showPreview(url, true, `${book.complete ? "" : "INCOMPLETE "}Muriales SDS Book ${stamp}.pdf`,
    book.complete ? `SDS Book — ${pages}` : `INCOMPLETE SDS BOOK — ${book.problems.length} sheet${book.problems.length === 1 ? "" : "s"} left out — ${pages}`,
    book.complete ? "After the pages are in the binder, click “Binder Is Up to Date” on the Library screen." : "This is not a complete binder. Do not record the binder as up to date from this printout.");
}

async function runPrint() {
  const o = printOpt, items = printItems(), idxOnly = o.scope === "index";
  const busy = $("#busy"); const say = t => { if (busy) busy.textContent = t; };
  say("Preparing the pages...");
  const book = await buildBook({items, cover: o.cover, index: idxOnly || o.index, dividers: o.dividers, numbers: o.numbers, eco: o.eco, indexOnly: idxOnly}, say);
  api("/api/binder/generated", {method: "POST", body: JSON.stringify({scope: o.scope, pages: book.pages, complete: book.complete, missing: book.problems.map(x => x.name), ids: items.map(p => p.id)})})
    .then(reload).then(() => { if (state.view === "library") render(); }).catch(() => {});
  if (book.complete) { closeModal(); return openBook(book); }
  // A sheet could not be included. Stop and say so; nothing reaches the printer unless the manager chooses to go on.
  pendingBook = book;
  showModal(`<h2>${icon("warn")}This ${idxOnly ? "Index" : "Book"} Is Incomplete</h2><div class="body">
    <p style="margin-top:0"><b>${book.problems.length} of ${items.length} SDS sheets could not be included.</b> A binder printed now would be missing ${book.problems.length === 1 ? "it" : "them"}.</p>
    <div class="box" style="max-height:210px;overflow:auto">${book.problems.map(x => `<p style="margin:4px 0"><b>${esc(x.name)}:</b> ${esc(x.why)}.</p>`).join("")}</div>
    <p>Fix ${book.problems.length === 1 ? "this sheet" : "these sheets"} in Settings (Diagnostics to check files, Clean Up to replace), then print again. If you go on, every copy is stamped INCOMPLETE.</p></div>
    <div class="foot"><button class="btn green big" data-act="fix-sheets">Fix the Sheets</button><button class="btn big" data-act="open-incomplete">Preview Incomplete Copy</button><button class="btn big" data-act="close">Cancel</button></div>`);
}

// "2-4, 7" -> [1,2,3,6] (zero-based), or null if it is not a valid list for a PDF of n pages.
function parsePages(text, n) {
  const out = [];
  for (const part of text.split(",").map(s => s.trim()).filter(Boolean)) {
    const m = /^(\d+)(?:\s*-\s*(\d+))?$/.exec(part); if (!m) return null;
    const a = +m[1], b = m[2] ? +m[2] : a; if (a < 1 || b < a || b > n) return null;
    for (let i = a; i <= b; i++) out.push(i - 1);
  }
  return out.length ? out : null;
}

async function printOne() {
  const p = byId(state.id), all = $('input[name="pp"]:checked').value === "all", stamp = ymd(new Date());
  if (all) { closeModal(); return showPreview(`/pdf/${p.id}?v=${encodeURIComponent(p.pdfUpdatedAt || "")}`, false, `${p.name} SDS.pdf`, `${p.name} — complete SDS`, "This is the manufacturer's sheet exactly as supplied."); }
  const x = await loadSheet(p); if (!x.doc) throw new Error(`Pages cannot be picked from this sheet because ${x.why}.`);
  const want = parsePages($("#pprange").value, x.n);
  if (!want) throw new Error(`Enter pages between 1 and ${x.n}, for example 2-4 or 1, 3.`);
  const out = await PDFDocument.create(); (await out.copyPages(x.doc, want)).forEach(c => out.addPage(c));
  closeModal();
  showPreview(URL.createObjectURL(new Blob([await out.save()], {type: "application/pdf"})), true, `${p.name} SDS pages ${$("#pprange")?.value || ""} ${stamp}.pdf`.replace(/\s+/g, " "),
    `${p.name} — ${want.length} of ${x.n} pages`, "A reference copy of selected pages. The official binder uses the complete sheet.");
}

Object.assign(ACT, {
  "selall-box"(el) { visible().forEach(p => el.checked ? state.sel.add(p.id) : state.sel.delete(p.id)); render(); },
  print(el) { printDialog(el.dataset.scope); },
  "print-go"(el) { el.disabled = true; return runPrint(); },
  "print-one"() { const p = byId(state.id);
    showModal(`<h2>${icon("print")}Print This Sheet</h2><div class="body"><p style="margin-top:0"><b>${esc(p.name)}</b>${p.pages ? ` — ${p.pages} pages` : ""}</p>
      <label><input type="radio" name="pp" value="all" checked>The complete SDS (use this for the binder)</label>
      <label><input type="radio" name="pp" value="some">Only these pages: <input id="pprange" class="field" style="width:150px" placeholder="e.g. 2-4, 7"></label>
      <p class="hint" style="color:#444">Picking pages makes a separate reference copy. The stored SDS is not changed.</p></div>
      <div class="foot"><button class="btn green big" data-act="print-one-go">${icon("print")}Preview &amp; Print</button><button class="btn big" data-act="close">Cancel</button></div>`); },
  "print-one-go": printOne,
});
