// Recipe Book: the recipe list with a cost summary on top.
const RB = {sum: null};

function vRbook() {
  if (!RB.sum) { api("/api/rbook/summary").then(o => { RB.sum = o; render(); }); return `<p class="hint">Loading...</p>`; }
  const m = RB.sum;
  return `<div class="stats"><div class="stat"><div class="n">${m.active}</div>active recipes</div><div class="stat"><div class="n">${m.active ? m.avgPct.toFixed(1) + "%" : "-"}</div>average food cost</div>
    <div class="stat ${m.over ? "warn" : ""}"><div class="n">${m.over}</div>over target (review)</div><div class="stat ${m.needsInfo ? "warn" : ""}"><div class="n">${m.needsInfo}</div>need a price or cost</div><div class="stat"><div class="n">${fcMoney(m.menuValue)}</div>total of selling prices</div></div>
    <div class="row" style="margin-bottom:8px"><button class="btn" data-act="rb-recost">${icon("chart")}Re-cost From Price List</button><button class="btn" data-act="reg-print" data-id="rbook-cost">${icon("print")}Print Cost Summary</button><button class="btn" data-act="reg-print" data-id="rbook-all">${icon("print")}Print Whole Recipe Book</button></div>${vReg("recipes")}`;
}

Object.assign(ACT, {
  async "rb-recost"() { const o = await api("/api/rbook/recost", {method: "POST", body: "{}"}); RB.sum = null; delete state.reg.recipes; delete state.reg.costhist; toast(o.changed ? `${o.changed} recipe${o.changed === 1 ? "" : "s"} changed cost. See Cost History.` : "No recipe cost has changed."); render(); },
});
{
  const nav = ACT.nav; ACT.nav = (el, e) => { if (["rbook", "rhist"].includes(el.dataset.v)) { RB.sum = null; delete state.reg.recipes; delete state.reg.costhist; } return nav(el, e); };
  const sv = ACT["reg-save"]; ACT["reg-save"] = async (el, e) => { const r = await sv(el, e); RB.sum = null; delete state.reg.costhist; if (state.view === "rbook") render(); return r; };
  const del = ACT["reg-del"]; ACT["reg-del"] = async (el, e) => { const r = await del(el, e); RB.sum = null; if (state.view === "rbook") render(); return r; };
}
