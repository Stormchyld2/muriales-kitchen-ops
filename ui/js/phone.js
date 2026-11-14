// Settings > Phone Access: the switch for the phone page on the kitchen Wi-Fi.
async function loadPhone() { try { state.phone = await api("/api/phone"); } catch (e) { state.phone = {on: false, urls: []}; } if (state.view === "settings" && state.tab === "phone") render(); }

function tPhone() {
  const p = state.phone; if (!p) return `<p class="hint">Loading...</p>`;
  return `<div class="panel" style="max-width:860px"><h3 style="margin-top:0">Phone page</h3>
    <p class="hint">Staff can open a simple page on a phone to log temperatures, do the par check, tick the daily workflow, log breakage and leave a handoff note. It only works on the kitchen Wi-Fi, only while this program is open on this computer, and it can reach nothing else in the program. There are no logins, so keep it off the guest Wi-Fi.</p>
    <label style="font-weight:700"><input type="checkbox" data-act="phone-toggle" ${p.on ? "checked" : ""}> Allow phones on the kitchen Wi-Fi to open the phone page</label>
    ${p.error ? `<div class="note warn">${esc(p.error)}</div>` : ""}
    ${p.on && p.running ? `<h3>Open this on the phone</h3><p>In the phone's browser (Chrome), type this address:</p>${p.urls.length ? p.urls.map(u => `<p style="font-size:22px;font-weight:700;margin:6px 0">${esc(u)}</p>`).join("") : `<p class="hint">No kitchen network address was found. Connect this computer to the Wi-Fi or network.</p>`}
      <ol class="hint"><li>Open the address in Chrome on the phone (the phone must be on the kitchen Wi-Fi).</li><li>Tap the three dots at the top right, then <b>Add to Home screen</b>.</li><li>The first time, Windows asks whether to allow this program on the private network. Choose Allow.</li><li>If the address stops working after the router restarts, the computer was given a new address. Ask whoever manages the router to reserve this computer's address.</li></ol>` : ""}
    ${p.on && !p.running && !p.error ? `<p class="hint">Starting...</p>` : ""}</div>`;
}

document.addEventListener("change", async e => {
  const el = e.target.closest && e.target.closest("[data-act=phone-toggle]"); if (!el) return;
  try { state.phone = await api("/api/phone", {method: "POST", body: JSON.stringify({on: el.checked})}); } catch (err) { toast(err.message || "Could not change that.", true); await loadPhone(); }
  render();
});
