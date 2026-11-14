// Event costing: summary tiles and margin thresholds above the event list.
const EV = {sum: null};

function vEvents() {
  if (!EV.sum) { api("/api/events/summary").then(o => { EV.sum = o; render(); }); return `<p class="hint">Loading...</p>`; }
  const m = EV.sum;
  return `<div class="stats"><div class="stat"><div class="n">${m.open}</div>open (inquiry or quoted)</div><div class="stat"><div class="n">${m.confirmed}</div>confirmed</div><div class="stat"><div class="n">${fcMoney(m.revenue)}</div>projected revenue</div>
    <div class="stat"><div class="n">${fcMoney(m.cost)}</div>projected cost</div><div class="stat ${m.revenue && m.margin < m.watch ? "warn" : ""}"><div class="n">${m.revenue ? m.margin.toFixed(1) + "%" : "-"}</div>projected gross margin</div></div>
    <div class="row" style="margin-bottom:8px;flex-wrap:wrap;align-items:flex-end"><label>GOOD margin at or above (%)<br><input id="ev-good" type="number" class="field" style="width:110px" value="${m.good}"></label><label>WATCH at or above (%)<br><input id="ev-watch" type="number" class="field" style="width:110px" value="${m.watch}"></label>
    <button class="btn" data-act="ev-set">Save Margins</button><span class="hint">Below the WATCH margin an event shows LOW.</span></div>${vReg("events")}`;
}

Object.assign(ACT, {
  async "ev-set"() { try { EV.sum = await api("/api/events/settings", {method: "POST", body: JSON.stringify({good: +$("#ev-good").value, watch: +$("#ev-watch").value})}); delete state.reg.events; toast("Saved."); render(); } catch (e) { toast(e.message || "Could not save."); } },
  "ev-add"(el) { regForm(el.dataset.c, "", {event: el.dataset.id}); },
});
{
  const nav = ACT.nav; ACT.nav = (el, e) => { if (["evt", "evm", "evc", "evr"].includes(el.dataset.v)) { EV.sum = null; delete state.reg.events; delete state.reg.eventmenu; delete state.reg.eventcost; delete state.reg.eventres; } return nav(el, e); };
  const sv = ACT["reg-save"]; ACT["reg-save"] = async (el, e) => { const r = await sv(el, e); EV.sum = null; delete state.reg.events; if (state.view === "evt") render(); return r; };
  const del = ACT["reg-del"]; ACT["reg-del"] = async (el, e) => { const r = await del(el, e); EV.sum = null; delete state.reg.events; if (state.view === "evt") render(); return r; };
}
