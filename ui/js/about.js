"use strict";

/* About */

// Newest first. Plain words: this is what a manager sees under "What's new".
const WHATS_NEW = [
  ["2.39.0", "Weekly Self-Inspection (Inspections menu): the 20 areas from the Health Inspection Success Program, scored C / COS / M / CR / NA with the internal score out of 100 and Green, Yellow or Red band, repeat findings flagged, and optional corrective action forms for every Major and Critical finding. Monthly Inspection Performance Review worked out from the weekly scores. Added the 15-Minute Inspection Sweep and Inspection-Day Response checklists to Kitchen-Wide station guides. Employee meals: revised Week 1 and Week 4 recipes and five new appendix recipes (Hot Dog Chili, Italian Pasta Salad, Nashville Hot Chicken Sauce, Scratch Pancakes, Spanish Rice)."],
  ["2.38.0", "Station Guides: grouped by station, with Open, Edit and Print on every SOP row. Pick a station and press Print Station Book to print every SOP, guide and checklist for it, each on its own page."],
  ["2.37.3", "The Muriale's logo now stays put at the top of the menu while the menu below it scrolls."],
  ["2.37.2", "90-Day Goals: an Add Goal button at the top, goals renumber automatically, and every edit form now has a Delete button at the bottom right (works on all lists that allow deleting)."],
  ["2.37.1", "Monthly Summary: the highlighted warning tile (such as PM jobs overdue) was a white block with unreadable text; it now matches the dark theme."],
  ["2.37.0", "New Operations section: Vendors & Contracts (contacts, service dates, contract end, order profile), Daily Forecast (expected vs actual covers with variance), Prep Pars by station, Smallware & Service-Ware Pars (count against par, Short flagged), Winter Preparedness (12 starter items) and Shed Inventory. Expired contracts, overdue services, smallware below par and overdue winter-prep items show in Needs Attention."],
  ["2.36.0", "Daily Workflow: your daily duties as timed blocks you tick off (the block you are in is highlighted), with who ticked it and when, plus a Handoff Log with owner, next action and due date. Weekly and monthly checks tick once per week or month. Overdue handoff items show on the dashboard. 90-Day Goals (under Maintenance): the nine signed goals with owner, target date, status and evidence, the leadership scorecard and weekly progress reviews, with day and week of the 90. The Today's Task Progress ring now includes the workflow."],
  ["2.35.0", "Food Costing: a Price List (type a case price and size, or paste a list), Cost a Special (type the ingredients and see cost, cost per portion, food cost % against your target, and the price that would hit it, then save it as a Special), and Specials. Employee meal recipes now cost from the same Price List. The daily summary can be sent by itself at a set time through the signed-in classic Outlook, with a Send a Test Now button and a dashboard alert if it fails."],
  ["2.34.0", "Health Inspection Packet (Inspections menu): pick the dates and parts and get one report with a contents page, numbered parts and sub-parts, and a sign-off line, ready to print or save as PDF. Daily Summary: a short text of the day, with a recipient list in Settings (commas) and an Email Daily Summary button on the dashboard. Backup Copy: each daily zip can also be copied to a second folder such as OneDrive. Training by station: the 13 stations are now roles in the Role Matrix, employees have a Stations field, and each station has a Station Competency Sign-Off requirement."],
  ["2.33.0", "Dashboard extras from the mockup: Today's Task Progress ring (cleaning, temperatures and station checklists), Operational Alerts (equipment down, urgent problems, temperatures out of range, late corrective actions, expired training), Key Metrics for the last 30 days against the 30 before, and larger illustrated icons on the shortcut tiles."],
  ["2.32.0", "Required training by position: a Role Matrix (positions against trainings, showing how many people in each position are current), and a Role Requirements list in the shape of your workbook (required, refresh months, trainer, sign-off, SOP, last reviewed). It starts filled from the training types, and the employee Training Matrix and expiry dates follow it. A position can be excused from a training that applies to everyone."],
  ["2.31.1", "Week 2 employee meal recipes added (14 meals), so all 56 meals in the four-week rotation now have a recipe. Appendix links are smarter, for example Brown Gravy for Schnitzel now shows on the Schnitzel dinner."],
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
