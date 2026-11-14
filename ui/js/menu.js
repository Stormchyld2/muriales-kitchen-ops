// Show or hide tabs and subtabs (Settings > Show / Hide Tabs).
const MENU = {hidden: new Set(), loaded: false};
api("/api/menu").then(o => { MENU.hidden = new Set(o.hidden || []); MENU.loaded = true; render(); }).catch(() => {});

function menuMods() {
  return MODS.map(m => ({...m, subs: m.subs.filter(([v]) => !MENU.hidden.has("sub:" + v))})).filter(m => !MENU.hidden.has("mod:" + m.k) && m.subs.length);
}

function tMenu() {
  return `<div class="panel" style="max-width:900px"><div class="row" style="justify-content:space-between"><h3 style="margin:0">Show or hide tabs</h3><button class="btn" data-act="menu-showall">Show Everything</button></div>
  <p class="hint">Untick a tab or subtab to hide it from the menu on every computer. Nothing is deleted: the information stays and you can show it again here. The Dashboard, To-Do List, Shift Handoff, Reports, Settings and About always stay.</p>
  ${MODS.map(m => { const modOn = !MENU.hidden.has("mod:" + m.k); return `<div style="margin:10px 0"><label style="font-weight:700"><input type="checkbox" data-menu="mod:${esc(m.k)}" ${modOn ? "checked" : ""}> ${esc(m.label)}</label>
    <div style="margin-left:26px;display:flex;flex-wrap:wrap;gap:4px 22px">${m.subs.map(([v, l]) => `<label style="font-weight:400;${modOn ? "" : "opacity:.5"}"><input type="checkbox" data-menu="sub:${esc(v)}" ${!MENU.hidden.has("sub:" + v) ? "checked" : ""}> ${esc(l)}</label>`).join("")}</div></div>`; }).join("")}</div>`;
}

async function menuSave() {
  try { const o = await api("/api/menu", {method: "POST", body: JSON.stringify({hidden: [...MENU.hidden]})}); MENU.hidden = new Set(o.hidden); } catch (e) { toast(e.message || "Could not save.", true); }
  render();
}

document.addEventListener("change", e => {
  const el = e.target.closest && e.target.closest("[data-menu]"); if (!el) return;
  if (el.checked) MENU.hidden.delete(el.dataset.menu); else MENU.hidden.add(el.dataset.menu);
  menuSave();
});
Object.assign(ACT, { "menu-showall"() { MENU.hidden = new Set(); menuSave(); } });
