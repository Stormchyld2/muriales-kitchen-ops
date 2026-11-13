"use strict";

/* Report a Problem: one front door that sends the report to the right record. */

function problemDialog() {
  const opt = (kind, title, text) => `<button class="catrow prob" data-act="problem-go" data-kind="${kind}" style="width:100%;text-align:left;cursor:pointer"><div class="grow"><b>${title}</b><div class="hint">${text}</div></div></button>`;
  showModal(`<h2>${icon("warn")}Report a Problem</h2><div class="body"><p class="hint" style="margin-top:0">What kind of problem is it? Each one goes to the right place so nothing gets lost.</p>
    ${opt("issue", "Equipment or building", "Broken, leaking, not cooling, needs repair. Becomes a Maintenance Issue you can send to MaintainX.")}
    ${opt("ca", "Food safety, sanitation, people or process", "A rule was missed or a standard failed. Becomes a Corrective Action form.")}
    ${opt("chem", "Chemical spill or exposure", "Someone was exposed, or a chemical spilled or was mixed. Goes in the Chemical Incident Log.")}
    ${opt("temp", "Temperature out of range", "Enter the reading; if it is out of range the form asks what you did about it.")}
    ${opt("train", "Training gap", "Someone did not know or follow the procedure. Becomes a Corrective Action in the Training area.")}
    ${opt("todo", "Just a reminder or one-off task", "Adds an item to the To-Do List.")}
  </div><div class="foot"><button class="btn" data-act="close">Cancel</button></div>`);
}

Object.assign(ACT, {
  "problem-new"() { problemDialog(); },
  async "problem-go"(el) {
    const k = el.dataset.kind; closeModal();
    try {
      if (k === "issue") { if (!state.maint) state.maint = await api("/api/maint"); issueForm(""); }
      else if (k === "ca") await caForm("");
      else if (k === "train") await caForm("", {area: "Training"});
      else if (k === "chem") await chemForm("");
      else if (k === "temp") { if (!state.temps) await loadTemps(); tempEnter(); }
      else if (k === "todo") ACT["todo-add"]();
    } catch (e) { toast(e.message, true); }
  },
});
