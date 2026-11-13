// Food costing: cost a special, the specials list, the ingredient price list.
const FC = {name: "", portions: "", price: "", ing: "", res: null, target: null};

const fcMoney = n => "$" + (+n || 0).toFixed(2);

function fcResult(r) {
  const pct = r.pct, col = !pct ? "" : pct <= r.target ? "ok" : pct <= r.target + 5 ? "mid" : "bad";
  const rows = r.lines.map(l => `<tr><td>${esc(l.amount)}</td><td>${esc(l.ingredient)}</td><td style="text-align:right">${l.priced ? fcMoney(l.cost) : `<span class="status Overdue">${l.skipped ? "-" : "No price"}</span>`}</td><td class="hint">${esc(l.note)}</td></tr>`).join("");
  return `<div class="tablewrap"><table class="plain"><thead><tr><th class="nosort" style="width:auto">Amount</th><th class="nosort" style="width:auto">Ingredient</th><th class="nosort" style="width:auto;text-align:right">Cost</th><th class="nosort" style="width:auto">How it was priced</th></tr></thead><tbody>${rows}</tbody></table></div>
  <div class="fctot"><div><span class="hint">Total cost</span><b>${fcMoney(r.total)}</b></div><div><span class="hint">Per portion (${+r.portions || 0})</span><b>${r.portions ? fcMoney(r.perPortion) : "-"}</b></div>
    <div><span class="hint">Menu price</span><b>${r.price ? fcMoney(r.price) : "-"}</b></div><div class="fcpct ${col}"><span class="hint">Food cost (target ${r.target}%)</span><b>${pct ? pct.toFixed(1) + "%" : "-"}</b></div>
    <div><span class="hint">Price for ${r.target}%</span><b>${r.suggest ? fcMoney(r.suggest) : "-"}</b></div></div>
  ${r.missing ? `<div class="note warn">${r.missing} ingredient${r.missing === 1 ? " has" : "s have"} no price, so the cost above is too low. Add ${r.missing === 1 ? "it" : "them"} to the Price List, or type a cost on the line as: amount | ingredient | cost.</div>` : ""}`;
}

function vFcalc() {
  return `<div class="panel" style="max-width:1000px"><p class="hint" style="margin-top:0">Type the ingredients one per line as: <b>amount | ingredient</b> (for example <b>2 lb | Bacon</b>, <b>1 1/2 cups | Heavy Cream</b>, <b>12 | Eggs</b>). It finds each price in the Price List. Put a cost at the end to override it: <b>2 lb | Bacon | 9.50</b>.</p>
  <div class="row" style="flex-wrap:wrap;gap:10px;align-items:flex-end"><label>Special name<br><input id="fc-name" class="field" style="width:260px" value="${esc(FC.name)}" placeholder="Pork Chop Special"></label>
    <label>Portions it makes<br><input id="fc-por" class="field" style="width:120px" value="${esc(FC.portions)}" placeholder="12"></label>
    <label>Menu price ($)<br><input id="fc-price" class="field" style="width:120px" value="${esc(FC.price)}" placeholder="18.00"></label></div>
  <textarea id="fc-ing" class="field" rows="9" style="margin-top:8px;font-family:Consolas,monospace" placeholder="2 lb | Pork Chops&#10;1 cup | Heavy Cream&#10;12 | Eggs">${esc(FC.ing)}</textarea>
  <div class="row" style="margin-top:8px"><button class="btn green big" data-act="fc-go">${icon("chart")}Cost It</button><button class="btn big" data-act="fc-save" ${FC.res ? "" : "disabled"}>${icon("check")}Save as a Special</button><button class="btn" data-act="fc-clear">Clear</button></div>
  ${FC.res ? `<h3 style="margin:14px 0 6px">${esc(FC.name || "Cost breakdown")}</h3>${fcResult(FC.res)}` : ""}</div>`;
}

function fcRead() { const g = id => { const e = $("#" + id); return e ? e.value : null; }; FC.name = g("fc-name") ?? FC.name; FC.portions = g("fc-por") ?? FC.portions; FC.price = g("fc-price") ?? FC.price; FC.ing = g("fc-ing") ?? FC.ing; }

function vFprice() {
  if (FC.target === null) { api("/api/costing/settings").then(o => { FC.target = o.targetPct; render(); }); }
  return `<div class="panel" style="max-width:1000px"><div class="row" style="flex-wrap:wrap;gap:10px;align-items:flex-end"><label>Food cost target (%)<br><input id="fc-target" type="number" min="1" max="100" class="field" style="width:110px" value="${FC.target ?? 30}"></label><button class="btn" data-act="fc-target">Save Target</button>
    <span style="margin-left:auto"></span><button class="btn green" data-act="fc-paste">${icon("plus")}Paste Prices</button></div>
    <p class="hint">Add each ingredient once: what you paid and how much that bought (for example 60.00 for 15 lb). Costs for every special and employee meal are worked out from this list. Name ingredients the way recipes do.</p></div>${vReg("prices")}`;
}

function fcShow(id) {
  return api("/api/costing/special/" + id).then(o => showModal(`<h2>${icon("chart")}${esc(o.rec.name)}</h2><div class="body">${fcResult(o.cost)}${o.rec.method ? `<h3>How it is made</h3><p style="white-space:pre-wrap">${esc(o.rec.method)}</p>` : ""}</div>
    <div class="foot"><button class="btn" data-act="rep-view" data-id="special:${esc(id)}">${icon("print")}Print</button><button class="btn" data-act="reg-open" data-c="specials" data-id="${esc(id)}">Edit</button><button class="btn big" data-act="close">Close</button></div>`));
}

Object.assign(ACT, {
  async "fc-go"() {
    fcRead(); if (!FC.ing.trim()) return toast("Type the ingredients first.");
    FC.res = await api("/api/costing/calc", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({ingredients: FC.ing, portions: FC.portions, price: FC.price})}); render();
  },
  "fc-clear"() { Object.assign(FC, {name: "", portions: "", price: "", ing: "", res: null}); render(); },
  "fc-save"() { fcRead(); regForm("specials", "", {name: FC.name, portions: FC.portions, menuPrice: FC.price, ingredients: FC.ing, status: "Planned"}); },
  async "fc-target"() { const o = await api("/api/costing/settings", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({targetPct: +$("#fc-target").value})}); FC.target = o.targetPct; toast("Saved."); render(); },
  "fc-paste"() {
    showModal(`<h2>${icon("plus")}Paste Prices</h2><div class="body"><p class="hint">One ingredient per line: <b>Ingredient | price paid | how much it buys | unit | vendor</b>. Vendor is optional. Example: <b>Bacon | 60.00 | 15 | lb | Sysco</b>. Units: ${priceUnitsText()}. An ingredient already on the list is updated.</p>
      <textarea id="fc-paste-text" class="field" rows="10" style="font-family:Consolas,monospace" placeholder="Bacon | 60.00 | 15 | lb | Sysco&#10;Eggs | 4.80 | 12 | each&#10;Heavy Cream | 18.50 | 1 | gal"></textarea></div>
      <div class="foot"><button class="btn green big" data-act="fc-paste-go">Add to Price List</button><button class="btn big" data-act="close">Cancel</button></div>`);
  },
  async "fc-paste-go"() {
    const o = await api("/api/costing/prices/import", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({text: $("#fc-paste-text").value})});
    closeModal && closeModal(); state.reg.prices = null; delete state.reg.prices;
    toast(`${o.added} added, ${o.updated} updated${o.bad.length ? `, ${o.bad.length} line(s) skipped` : ""}.`);
    if (o.bad.length) alert("These lines could not be read (check the price and unit):\n\n" + o.bad.join("\n"));
    render();
  },
  "fc-show"(el) { return fcShow(el.dataset.id); },
});
const priceUnitsText = () => "lb, oz, g, kg, gal, qt, pt, cup, fl oz, tbsp, tsp, ml, l, each, dozen, can, bunch, head, loaf, pan, bag, box";
