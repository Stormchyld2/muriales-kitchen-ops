"use strict";

/* About */

function vAbout() {
  return `
  <div class="panel" style="max-width:760px">
    <h3 style="margin-top:0">Muriale's Kitchen Operations <span class="hint">version ${esc(state.lib.version || "")}</span></h3>
    <p>Keeps the Safety Data Sheet (SDS, formerly MSDS) library for Muriale's Italian Kitchen and prints the paper SDS book.</p>
    <p>${state.lib.products.length} product${state.lib.products.length === 1 ? "" : "s"} in the library.</p>
    <p class="hint">Several people can use the same library folder. Only one can make changes at a time; everyone else can search, view and print.</p>
    <p><b>Emergency:</b> 911 &nbsp;·&nbsp; <b>Poison Control:</b> 1-800-222-1222</p>
    <p class="hint">The emergency summaries are typed in by a manager as a quick reference. The manufacturer's sheet is always the authority.</p>
  </div>`;
}
