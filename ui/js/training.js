"use strict";

/* Employee training */

function vTrMatrix() {
  const m = state.trm; if (!m) { api("/api/training/matrix").then(o => { state.trm = o; render(); }); return `<p class="hint">Loading...</p>`; }
  const c = m.count || m.Count || {}, short = x => shortDate(x || "");
  return `<div class="row"><button class="btn green big" data-act="train-new">${icon("plus")}Record Training</button><button class="btn" data-act="rep-print" data-id="training-matrix">${icon("print")}Print Matrix</button><button class="btn" data-act="rep-print" data-id="training-due">${icon("print")}Print What's Due</button><button class="btn" data-act="rep-print" data-id="training-signin">${icon("print")}Sign-In Sheet</button><button class="btn" data-act="rep-email-id" data-id="training-due">${icon("ext")}Email What's Due</button></div>
  <p class="hint" style="margin:6px 0">${tag("Current")} ${c["Current"] || 0} &nbsp; ${tag("Due Soon")} ${c["Due Soon"] || 0} &nbsp; ${tag("Expired")} ${c["Expired"] || 0} &nbsp; ${tag("Missing")} ${c["Missing"] || 0} &nbsp; Click any box to record that training for that person.</p>
  <div class="tablewrap">${!m.rows.length ? `<div class="empty"><b>No employees yet</b>Add your team under Employees, then record their training here.<div style="margin-top:10px"><button class="btn green" data-act="nav" data-v="tremp">${icon("plus")}Go to Employees</button></div></div>`
    : `<table class="plain"><thead><tr><th class="nosort" style="width:auto">Employee</th>${m.types.map(t => `<th class="nosort" style="width:auto">${esc(t.name)}</th>`).join("")}</tr></thead><tbody>
    ${m.rows.map(r => `<tr><td><b>${esc(r.name)}</b><div class="hint">${esc([r.position, r.department].filter(Boolean).join(" · "))}</div></td>${m.types.map(t => { const x = r.cells[t.id]; return !x ? `<td class="hint">n/a</td>` : `<td><a href="#" data-act="train-cell" data-e="${esc(r.id)}" data-t="${esc(t.id)}" style="text-decoration:none">${tag(x.state)}</a><div class="hint">${x.state === "Missing" ? "never done" : x.expires ? "expires " + esc(short(x.expires)) : esc(short(x.completed))}</div></td>`; }).join("")}</tr>`).join("")}</tbody></table>`}</div>`;
}

Object.assign(ACT, {
  async "train-for"(el) { if (!state.reg.training) state.reg.training = await api("/api/reg/training"); regForm("training", "", {employeeId: el.dataset.id}); },
  async "train-cell"(el) { if (!state.reg.training) state.reg.training = await api("/api/reg/training"); regForm("training", "", {employeeId: el.dataset.e, typeId: el.dataset.t}); },
  async "train-new"() { if (!state.reg.training) state.reg.training = await api("/api/reg/training"); regForm("training", ""); },
});

// Required training by role: positions down the side, trainings across the top.
function vTrRole() {
  const m = state.trr; if (!m) { api("/api/training/rolematrix").then(o => { state.trr = o; render(); }); return `<p class="hint">Loading...</p>`; }
  const cell = (ro, t) => {
    const c = ro.cells[t.id];
    if (!c.required) return `<td class="rmc" data-act="role-cell" data-r="${esc(ro.role)}" data-t="${esc(t.id)}" data-q="${esc(c.reqId || "")}" title="Click to require this"><span class="hint">-</span></td>`;
    const tone = !c.people ? "none" : c.current === c.people ? "ok" : c.current === 0 ? "bad" : "mid";
    return `<td class="rmc ${tone}" data-act="role-cell" data-r="${esc(ro.role)}" data-t="${esc(t.id)}" data-q="${esc(c.reqId)}"><b>${c.people ? c.current + "/" + c.people : "Req"}</b>${c.signoff ? `<span class="hint">sign-off</span>` : ""}${c.refresh ? `<span class="hint">${esc(c.refresh)} mo</span>` : ""}</td>`;
  };
  return `<div class="row"><button class="btn green big" data-act="role-add">${icon("plus")}Add Requirement</button><button class="btn" data-act="nav" data-v="trreq">Requirements List</button><button class="btn" data-act="rep-print" data-id="training-roles">${icon("print")}Print</button></div>
  <p class="hint" style="margin:6px 0">Each box shows how many of the people in that position are current on the training. A dash means it is not required for that position. Click a box to change it. All Staff trainings apply to everyone.</p>
  <div class="tablewrap">${!m.roles.length ? `<div class="empty"><b>No requirements yet</b>Add one to start the matrix.</div>`
    : `<table class="plain mxr"><thead><tr><th class="nosort" style="width:auto">Position</th>${m.types.map(t => `<th class="nosort" style="width:auto">${esc(t.name)}</th>`).join("")}</tr></thead><tbody>${m.roles.map(ro => `<tr><td><b>${esc(ro.role)}</b><div class="hint">${ro.people} ${ro.people === 1 ? "person" : "people"}</div></td>${m.types.map(t => cell(ro, t)).join("")}</tr>`).join("")}</tbody></table>`}</div>`;
}

Object.assign(ACT, {
  async "role-add"() { if (!state.reg.trainreq) state.reg.trainreq = await api("/api/reg/trainreq"); regForm("trainreq", ""); },
  async "role-cell"(el) {
    if (!state.reg.trainreq) state.reg.trainreq = await api("/api/reg/trainreq");
    if (el.dataset.q) regForm("trainreq", el.dataset.q); else regForm("trainreq", "", {role: el.dataset.r, typeId: el.dataset.t, required: "Yes"});
  },
});
