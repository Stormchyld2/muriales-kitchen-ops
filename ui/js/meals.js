// Employee Meals: dashboard, four-week rotation, recipe viewer.
const ML = {dash: null, cov: null, wk: 0};

async function loadMDash() { try { ML.dash = await api("/api/meals/dash"); } catch (e) { ML.dash = {error: e.message}; } render(); }
async function loadMCov() { try { ML.cov = await api("/api/meals/coverage"); if (!ML.wk) ML.wk = ML.cov.current; } catch (e) { ML.cov = {error: e.message}; } render(); }

function mGauge(pct, text, color) {
  const p = Math.max(0, Math.min(100, pct)), arc = "M20 100 A80 80 0 0 1 180 100";
  return `<svg viewBox="0 0 200 112" class="gsvg" aria-hidden="true"><path d="${arc}" fill="none" stroke="#24262b" stroke-width="18" stroke-linecap="round"/>`
    + (p > 0 ? `<path d="${arc}" fill="none" stroke="${color}" stroke-width="18" stroke-linecap="round" pathLength="100" stroke-dasharray="${p} 100"/>` : "")
    + `<text x="100" y="92" text-anchor="middle" class="gnum" fill="${color}">${esc(text)}</text></svg>`;
}
const M_GREEN = "#58b368", M_AMBER = "#e7b416", M_RED = "#e0525e", M_BLUE = "#5aa6f0";
function costGauge(label, cell, target, sub) {
  if (!cell || !cell.perNum) return dBlock("g", `<div class="dlabel">${esc(label)}</div>${mGauge(0, cell ? "--" : "None", "#7d828b")}<div class="dsub">${esc(cell ? "Type in ingredient costs" : "No recipe for today")}</div>`, "mrec");
  const r = cell.perNum / target, col = r <= 1 ? M_GREEN : r <= 1.15 ? M_AMBER : M_RED;
  return dBlock("g", `<div class="dlabel">${esc(label)}</div>${mGauge(Math.round(r * 100), cell.perPerson, col)}<div class="dsub">${esc(sub)}</div>`, "mrec");
}

function mCell(c, label) {
  if (!c) return `<div class="mcell none"><span class="mlab">${esc(label)}</span><span class="hint">No recipe yet</span></div>`;
  return `<div class="mcell" data-act="meal-open" data-id="${esc(c.id)}" role="button" tabindex="0"><span class="mlab">${esc(label)}</span><b>${esc(c.name)}</b><span class="hint">${esc(c.serves ? "feeds " + c.serves : "")}${c.time ? " · " + esc(c.time) : ""}${c.perPerson ? " · " + esc(c.perPerson) + " each" : ""}</span></div>`;
}

function mWeekGrid(days, mark) {
  return `<div class="mgrid">${days.map(d => `<div class="mday${d.today && mark ? " today" : ""}"><div class="mdn">${esc(d.day)}${d.date ? `<span class="hint">${esc(shortDate(d.date))}</span>` : ""}${d.today && mark ? `<span class="status Soon">Today</span>` : ""}<span class="hint">${d.people} people</span></div>${mCell(d.bl, "Breakfast & Lunch")}${mCell(d.dinner, "Dinner")}</div>`).join("")}</div>`;
}

function mCoverGrid(weeks, cur) {
  return `<table class="mcov"><thead><tr><th>Week</th>${["Mon","Tue","Wed","Thu","Fri","Sat","Sun"].map(x => `<th>${x}</th>`).join("")}<th>Recipes</th></tr></thead><tbody>${weeks.map(w => {
    const n = w.days.reduce((a, d) => a + (d.bl ? 1 : 0) + (d.dinner ? 1 : 0), 0);
    return `<tr class="${w.week === cur ? "cur" : ""}"><td><b>Week ${w.week}</b>${w.week === cur ? ` <span class="status Soon">Now</span>` : ""}</td>${w.days.map(d => `<td><span class="dot ${d.bl ? "ok" : "no"}" title="${esc(d.day)} breakfast/lunch${d.bl ? ": " + esc(d.bl.name) : ": missing"}">B/L</span><span class="dot ${d.dinner ? "ok" : "no"}" title="${esc(d.day)} dinner${d.dinner ? ": " + esc(d.dinner.name) : ": missing"}">D</span></td>`).join("")}<td><b>${n} of 14</b></td></tr>`;
  }).join("")}</tbody></table>`;
}

function vMDash() {
  const d = ML.dash;
  if (!d) { loadMDash(); return `<div class="empty"><b>Loading...</b></div>`; }
  if (d.error) return `<div class="empty"><b>Could not load</b>${esc(d.error)}</div>`;
  const st = d.settings, t = d.today;
  const g1 = dBlock("g", `<div class="dlabel">Rotation Week</div>${mGauge(d.week * 25, "Week " + d.week, M_BLUE)}<div class="dsub">of 4 · ${esc(shortDate(d.monday))} to ${esc(shortDate(d.sunday))}</div>`, "mrot");
  const g2 = dBlock("g", `<div class="dlabel">Recipes Entered</div>${mGauge(Math.round(d.have / d.total * 100), d.have + "/" + d.total, d.have === d.total ? M_GREEN : M_AMBER)}<div class="dsub">${d.missing.length ? esc(d.missing.join(", ")) : "Every meal has a recipe"}</div>`, "mrot");
  const g3 = costGauge("Breakfast & Lunch Cost", t && t.bl, st.blTarget, "per person vs $" + st.blTarget.toFixed(2));
  const g4 = costGauge("Dinner Cost", t && t.dinner, st.dinnerTarget, "per person vs $" + st.dinnerTarget.toFixed(2));
  const g5 = dBlock("g", `<div class="dlabel">Meals Logged</div>${mGauge(Math.round(Math.min(d.logged, 14) / 14 * 100), String(d.logged), d.logged >= 10 ? M_GREEN : M_AMBER)}<div class="dsub">this week of 14</div>`, "mplan");
  const g6 = dBlock("t " + "", `<div class="dlabel">Feeding Today</div><div class="dval">${t ? t.people : "-"}</div><div class="dsub">${t ? esc(t.day) + " head count" : "Set in 4-Week Rotation"}</div>`, "mrot");
  const todayB = dBlock("w3 r3", `<div class="dhead"><span>Today${t ? " · " + esc(t.day) : ""}</span><span class="hint">Week ${d.week}</span></div><div class="mtoday">${t ? mCell(t.bl, "Breakfast & Lunch") + mCell(t.dinner, "Dinner") : `<p class="hint">Today is not in the current week.</p>`}</div>`);
  const qa = dBlock("w3 r3", `<div class="dhead"><span>Quick Actions</span></div><div class="dqa"><button class="btn" data-act="nav" data-v="mrot">${icon("list")}4-Week Rotation</button><button class="btn" data-act="nav" data-v="mrec">${icon("book")}Recipes</button><button class="btn" data-act="meal-log-new">${icon("check")}Log a Meal</button><button class="btn" data-act="rep-view" data-id="mealweek:${d.week}">${icon("print")}Print This Week</button><button class="btn" data-act="rep-view" data-id="mealweek:${d.next}">${icon("print")}Print Week ${d.next}</button><button class="btn" data-act="nav" data-v="mapp">${icon("flask")}Appendix</button></div>`);
  const week = dBlock("wall r4", `<div class="dhead"><span>Week ${d.week} Breakdown · Breakfast & Lunch and Dinner</span><span class="hint">Click a meal for its recipe</span></div>${mWeekGrid(d.days, true)}`);
  const nxt = dBlock("wall r4", `<div class="dhead"><span>Coming Next · Week ${d.next}</span><span class="hint">starts ${esc(shortDate(d.nextDays[0].date))}</span></div>${mWeekGrid(d.nextDays, false)}`);
  return `<div class="dgrid">${g1}${g2}${g3}${g4}${g5}${g6}${todayB}${qa}${week}${nxt}</div>`;
}

function vMRot() {
  const c = ML.cov;
  if (!c) { loadMCov(); return `<div class="empty"><b>Loading...</b></div>`; }
  if (c.error) return `<div class="empty"><b>Could not load</b>${esc(c.error)}</div>`;
  const st = c.settings, w = c.weeks[(ML.wk || c.current) - 1];
  const miss = c.weeks.reduce((a, x) => a + x.missing.length, 0);
  return `<div class="row">${[1, 2, 3, 4].map(n => `<button class="btn ${n === (ML.wk || c.current) ? "green" : ""}" data-act="mrot-week" data-n="${n}">Week ${n}${n === c.current ? " (now)" : ""}</button>`).join("")}<span style="margin-left:auto"></span><button class="btn" data-act="rep-view" data-id="mealweek:${w.week}">${icon("print")}Print Week ${w.week}</button></div>
  ${mWeekGrid(w.days, w.week === c.current)}
  ${w.missing.length ? `<div class="note warn">Week ${w.week} is missing a recipe for: ${esc(w.missing.join(", "))}. Add them under Recipes with Rotation week ${w.week} and the day and meal.</div>` : ""}
  <h3 style="margin:18px 0 6px">Recipes against the rotation</h3>
  <p class="hint">Every slot in the four weeks, with a green mark where a recipe is entered for it. ${miss ? miss + " slot" + (miss === 1 ? " is" : "s are") + " still empty." : "All 56 meals have a recipe."}</p>
  ${mCoverGrid(c.weeks, c.current)}
  <h3 style="margin:18px 0 6px">Rotation settings</h3>
  <div class="row" style="align-items:flex-end;flex-wrap:wrap;gap:10px">
    <label>Week 1 starts on (a Monday)<br><input id="ms-anchor" type="date" value="${esc(st.anchor)}"></label>
    <label>Weekday people<br><input id="ms-wd" type="number" min="1" value="${st.weekdayPeople}" style="width:90px"></label>
    <label>Weekend people<br><input id="ms-we" type="number" min="1" value="${st.weekendPeople}" style="width:90px"></label>
    <label>Minutes per meal<br><input id="ms-min" type="number" min="1" value="${st.minutes}" style="width:90px"></label>
    <label>Breakfast/lunch $ per person<br><input id="ms-bl" type="number" step="0.01" min="0" value="${st.blTarget}" style="width:90px"></label>
    <label>Dinner $ per person<br><input id="ms-dn" type="number" step="0.01" min="0" value="${st.dinnerTarget}" style="width:90px"></label>
    <button class="btn green" data-act="mset-save">Save Settings</button></div>`;
}

async function mealOpen(id, app) {
  const o = await api("/api/meals/" + (app ? "appendix/" : "recipe/") + id), r = o.rec;
  const links = o.appendix.length ? `<div class="row" style="flex-wrap:wrap"><span class="hint">Uses from the appendix:</span>${o.appendix.map(a => `<button class="btn small" data-act="meal-open" data-app="1" data-id="${esc(a.id)}">${esc(a.name)}</button>`).join("")}</div>` : "";
  showModal(`<h2>${icon(app ? "flask" : "book")}${esc(r.name)}</h2><div class="body">${links}<div class="gmd">${o.html}</div></div>
    <div class="foot"><button class="btn" data-act="rep-view" data-id="${app ? "mealapp:" : "meal:"}${esc(id)}">${icon("print")}Print</button><button class="btn" data-act="reg-open" data-c="${app ? "mealappendix" : "mealrecipes"}" data-id="${esc(id)}">Edit</button><button class="btn big" data-act="close">Close</button></div>`);
}

Object.assign(ACT, {
  "meal-open"(el) { return mealOpen(el.dataset.id, !!el.dataset.app); },
  "mrot-week"(el) { ML.wk = +el.dataset.n; render(); },
  "meal-log-new"() { return regForm("mealplan", ""); },
  async "mset-save"() {
    const v = id => $("#" + id).value;
    await api("/api/meals/settings", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({anchor: v("ms-anchor"), weekdayPeople: +v("ms-wd"), weekendPeople: +v("ms-we"), minutes: +v("ms-min"), blTarget: +v("ms-bl"), dinnerTarget: +v("ms-dn")})});
    ML.dash = null; ML.cov = null; toast("Rotation settings saved."); render();
  },
});

// fresh numbers whenever the person arrives at, or saves from, a meals screen
(() => {
  const nav = ACT.nav; ACT.nav = (el, e) => { if (el.dataset.v === "mdash" || el.dataset.v === "mrot") { ML.dash = null; ML.cov = null; } return nav(el, e); };
  const sv = ACT["reg-save"]; if (sv) ACT["reg-save"] = async (el, e) => { const r = await sv(el, e); ML.dash = null; ML.cov = null; return r; };
})();
