"use strict";

/* About */

// Newest first. Plain words: this is what a manager sees under "What's new".
const WHATS_NEW = [
  ["2.31.0", "Employee Meals rebuilt around the four-week rotation: a dashboard showing which week it is with the breakfast/lunch and dinner for every day, a 4-Week Rotation tab that checks each recipe against its slot, 42 recipes from your menu documents (Weeks 1, 3 and 4), a Meal Log, and an Appendix of the 12 sauces and sides, last in the list. Week 2 still needs its recipes."],
  ["2.30.0", "Employee Meals: meal recipes with cost per serving, a meal plan, and a 30-day cost report. Dashboard now flags a lockout left on, an overdue LOTO inspection, an overdue binder review and incident follow-ups. Safety meetings can start a corrective action."],
  ["2.29.1", "Dashboard re-laid out: Needs Attention full width, icon shortcut tiles to every area, Last Backup beside Recent Changes."],
  ["2.29.0", "Lockout / Tagout: taking equipment out of service now adds a filled-in LOTO Log line and a pre-filled lock tag, and clearing it marks the lock removed; equipment procedures and annual inspections. Daily Readiness Walk (25 checks) with history. New Hire Training checklist, Safety Meetings, Incidents & Near Misses, Training Materials folder. Emergency Binder: index, contacts, critical equipment plan, vendor contacts, review log. Station checklist gauge on the dashboard."],
  ["2.28.0", "Stations group: 74 station guides (overviews, SOPs, par sheets, recipes, training programs) you can search, read, edit and print; tick-off Opening and Closing checklists for each station with a log; charcoal theme with meter colors; date and time at the top; Enter Temperatures button on the dashboard fixed."],
  ["2.27.0", "Dashboard home screen with gauges; Report a Problem button; Inspections grouped in one place; Shift Handoff on its own; Chemical Safety back under SDS; Training and Emergency grouped; a backup every day; Edit History in Settings; a Weekly Review report; names tidied up as they are saved."],
  ["2.26.0", "Employee Training: employees, training types, records, and a matrix showing who is current, due soon, expired or missing."],
  ["2.25.0", "Six departments (Front Line, Back Line, Prep, Salads, Dish, General) with their own keywords; report emails put the text right in the message."],
  ["2.24.0", "Inspection-Ready Checklist and Weekly Summary; Edit buttons on service records, PM inspections and issues; Backup Both Folders and restore."],
  ["2.23.0 and earlier", "Maintenance, cleaning and sanitation, temperature log, chemical safety, corrective actions, calendar and to-do list, SDS Manager."],
];

function vAbout() {
  return `
  <div class="panel" style="max-width:760px">
    <h3 style="margin-top:0">Muriale's Kitchen Operations <span class="hint">version ${esc(state.lib.version || "")}</span></h3>
    <p>Keeps the Safety Data Sheet (SDS, formerly MSDS) library for Muriale's Italian Kitchen and prints the paper SDS book.</p>
    <p>${state.lib.products.length} product${state.lib.products.length === 1 ? "" : "s"} in the library.</p>
    <p class="hint">Several people can use the same library folder. Only one can make changes at a time; everyone else can search, view and print.</p>
    <p><b>Emergency:</b> 911 &nbsp;·&nbsp; <b>Poison Control:</b> 1-800-222-1222</p>
    <p class="hint">The emergency summaries are typed in by a manager as a quick reference. The manufacturer's sheet is always the authority.</p>
  </div>
  <div class="panel" style="max-width:760px;margin-top:12px"><h3 style="margin-top:0">What's new</h3>
    ${WHATS_NEW.map(([v, t]) => `<p style="margin:0 0 8px"><b>${esc(v)}</b><br><span class="hint" style="font-size:13.5px">${esc(t)}</span></p>`).join("")}
  </div>`;
}
