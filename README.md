# Muriale's Kitchen Operations

A Windows program for Muriale's Italian Kitchen. Like the SDS Manager it was built from, it is a single program with no installer and no outside server: it keeps plain files next to the program: an **SDS Data** folder (exactly the folder the SDS Manager uses, so PDFs can be dropped into it) and a separate **Maintenance Data** folder. The SDS Data location can be changed in Settings, for example to a shared 365 or network drive; Maintenance Data always sits beside it.

## Modules

- **Equipment** – the list of walk-ins, fryers, mixers and so on, with area, make, model, serial number and status (Operational, Needs Attention, Out of Service).
- **Maintenance Issues** – managers report a problem on a piece of equipment, track it (Open, Work Order Submitted, In Progress, Repaired, Verified, Closed) and keep a history. *Repaired* and *Verified* are separate: Verified means a manager has confirmed the equipment is ready for food production, and only then does the equipment return to Operational.
- **Assets & Costs** – model, serial number, purchase date and cost, warranty and normal life for each unit, plus a record of every repair or service and what it cost. When upkeep reaches a set share (default 50%) of the cost of a new unit, the program suggests considering a replacement. It is a rule of thumb, adjustable in Settings, MaintainX.
- **Preventive Maintenance** – recurring jobs on any equipment, including HVAC, hoods and fire suppression, with common starting points to pick from. A manager marks each job done (with an optional cost) and the next due date follows.
- **Schedule** – a calendar (month, week or list) that fills itself from every recurring job: Cleaning, Equipment PM and Grounds / Facility, plus done jobs and reported issues. Filter by group (for example Cleaning) and by how often (Daily, Weekly, Monthly, Quarterly / Semi-Annual, Yearly). Click a day to see its jobs and mark them done. Cleaning Tasks and Grounds / Facility have their own lists with common starting points to pick from.
- **To-Do List** – the **Create To-Do List** button in the side bar works from anywhere. Choose Today, Tomorrow, This Week, Next Week, Next 7 Days, This Month, Next Month, Next 30 Days, Overdue Only or your own dates, and what to include. The program gathers overdue and due recurring jobs, open maintenance issues, repair-or-replace decisions, SDS follow-ups and your own one-off items into one list that can be printed. Jobs can be marked done straight from it.
- **Load Workbook Data** (Settings) – loads the 74 PM jobs from the PM Calendar workbook and the 14 daily and weekly grounds, facility and cleaning tasks from the Quick Look workbook, keeping each job's ID, asset ID, performer, frequency wording and notes. Safe to run twice.
- **Submit Work Order** – a button in the side bar on every maintenance screen. It opens the MaintainX web portal in its own app-style window (Edge, kept signed in per computer). On an issue, **Copy & Submit Work Order** also copies the issue details so they can be pasted into MaintainX. The work order number can be written back on the issue. The program does not use the MaintainX API. Set the portal address in Settings, MaintainX.
- **Safety Data Sheets** – the complete SDS Manager (library, auto-drafting from PDFs, emergency quick view, printable binder, handoff notes, backups, edit lock, 30-day removed-products clean-up). See `SDS-MANAGER.md`.

Who may use it is decided by who can open the folder: only people with access to the drive can edit.

## Files

```
SDS Data/                      the SDS library, same as the SDS Manager
  library.json  sheets/  removed/
Maintenance Data/
  equipment.json  issues.json  pm.json (all recurring jobs)  service.json  todos.json  settings.json   (each with a .bak copy)
  Maintenance Log.md           readable copy, rewritten by the program
Kitchen Backups/               SDS zips, plus a Maintenance zip with each backup
```

Settings, Backup & Restore has **Backup Both Folders**: one zip holding the SDS library and all the kitchen operations files. Restore it, or a Maintenance zip, from the same list (what is replaced is saved to Kitchen Backups first). The "someone is editing" lock covers both modules.

## Building

Push to `main`; GitHub Actions builds `MurialesKitchenOps.exe` for Windows and runs the checks. It uses port 47715, so it can run beside the stand-alone SDS Manager (port 47615).

- **Inspection-Ready Checklist and Weekly Summary** – in Food Safety, Inspection Readiness, and in Reports. The checklist reads your records live (temperatures today, out-of-range actions, corrective actions, cleaning, deep clean, manager checks, PM, issues, SDS, last walk-through) and adds the walk-through points with a sign-off line. The summary covers the last 7 days. Both can be emailed.
