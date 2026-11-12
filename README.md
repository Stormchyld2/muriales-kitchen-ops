# Muriale's Kitchen Operations

A Windows program for Muriale's Italian Kitchen. Like the SDS Manager it was built from, it is a single program with no installer and no outside server: it keeps plain files in a **Kitchen Data** folder next to the program (or in a folder chosen in Settings, such as a shared 365 or network drive).

## Modules

- **Equipment** – the list of walk-ins, fryers, mixers and so on, with area, make, model, serial number and status (Operational, Needs Attention, Out of Service).
- **Maintenance Issues** – managers report a problem on a piece of equipment, track it (Open, Work Order Submitted, In Progress, Repaired, Verified, Closed) and keep a history. *Repaired* and *Verified* are separate: Verified means a manager has confirmed the equipment is ready for food production, and only then does the equipment return to Operational.
- **Submit Work Order** – a button in the side bar on every maintenance screen. It opens the MaintainX web portal in its own app-style window (Edge, kept signed in per computer). On an issue, **Copy & Submit Work Order** also copies the issue details so they can be pasted into MaintainX. The work order number can be written back on the issue. The program does not use the MaintainX API. Set the portal address in Settings, MaintainX.
- **Safety Data Sheets** – the complete SDS Manager (library, auto-drafting from PDFs, emergency quick view, printable binder, handoff notes, backups, edit lock, 30-day removed-products clean-up). See `SDS-MANAGER.md`.

Who may use it is decided by who can open the folder: only people with access to the drive can edit.

## Files

```
Kitchen Data/
  library.json  sheets/  removed/        SDS library
  maintenance/
    equipment.json  issues.json  settings.json   (each with a .bak copy)
    Maintenance Log.md                   readable copy, rewritten by the program
```

Backups (Settings, Backup & Restore) zip the whole folder, maintenance files included. The "someone is editing" lock covers everything.

## Building

Push to `main`; GitHub Actions builds `MurialesKitchenOps.exe` for Windows and runs the checks. It uses port 47715, so it can run beside the stand-alone SDS Manager (port 47615).
