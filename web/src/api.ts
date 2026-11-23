import { CLIENT, cap, h, say } from './core';
import { follow, live, render } from './main';
import { ask } from './org';
import { link, me, route, signIn } from './shell';
import { field } from './ui';
import { enc } from './util';

// ---- the API: Connect (proto/dutyplan/v1/dutyplan.proto) ---------------------------------------------------------------
// Every call is POST /dutyplan.v1.<Service>/<Method> with a JSON body. The page still names what it wants by method and
// path (and keeps what it fetched by path, see get()); ROUTES turns that into the call: each {name} of the path is a
// field of the request (ids are numbers; org and plan text), the body's fields are added, and `unwrap` is the field of
// the answer that holds what the page uses. Kept until the page has a generated client (TypeScript).
export const ROUTES = [
  ['GET', '/health', 'AccountService/Health'], ['GET', '/me', 'AccountService/Me'],
  ['GET', '/orgs', 'AccountService/ListOrgs', 'orgs'], ['POST', '/orgs', 'AccountService/CreateOrg', 'org'],
  ['GET', '/orgs/{org}', 'OrgService/GetOrg'], ['PATCH', '/orgs/{org}', 'OrgService/RenameOrg', 'org'],
  ['DELETE', '/orgs/{org}', 'OrgService/DeleteOrg'],
  ['GET', '/orgs/{org}/members', 'OrgService/ListMembers', 'members'], ['PATCH', '/orgs/{org}/members/{id}', 'OrgService/SetMemberRole'],
  ['DELETE', '/orgs/{org}/members/{id}', 'OrgService/RemoveMember'],
  ['GET', '/orgs/{org}/links', 'OrgService/ListLinks', 'links'], ['POST', '/orgs/{org}/links', 'OrgService/CreateLink', 'link'],
  ['DELETE', '/orgs/{org}/links/{id}', 'OrgService/DeleteLink'],
  ['GET', '/orgs/{org}/tasks', 'SetupService/ListTasks', 'tasks'], ['POST', '/orgs/{org}/tasks', 'SetupService/CreateTask', 'task'],
  ['PATCH', '/orgs/{org}/tasks/{id}', 'SetupService/UpdateTask', 'task'], ['DELETE', '/orgs/{org}/tasks/{id}', 'SetupService/DeleteTask'],
  ['PUT', '/orgs/{org}/tasks/{id}/position', 'SetupService/MoveTask'],
  ['GET', '/orgs/{org}/combinations', 'SetupService/ListCombinations', 'combinations'], ['PUT', '/orgs/{org}/combinations', 'SetupService/SetCombination'],
  ['PUT', '/orgs/{org}/tasks/{task}/combinations', 'SetupService/SetTaskCombinations'],
  ['GET', '/orgs/{org}/people', 'SetupService/ListPeople', 'people'], ['POST', '/orgs/{org}/people', 'SetupService/CreatePerson', 'person'],
  ['PATCH', '/orgs/{org}/people/{id}', 'SetupService/UpdatePerson', 'person'], ['PUT', '/orgs/{org}/people/{id}/position', 'SetupService/MovePerson'],
  ['PUT', '/orgs/{org}/people/{person}/tasks/{task}', 'SetupService/SetPersonTask'], ['DELETE', '/orgs/{org}/people/{person}/tasks/{task}', 'SetupService/RemovePersonTask'],
  ['GET', '/orgs/{org}/sections', 'SetupService/ListSections', 'sections'], ['POST', '/orgs/{org}/sections', 'SetupService/CreateSection', 'section'],
  ['PATCH', '/orgs/{org}/sections/{id}', 'SetupService/RenameSection', 'section'], ['PUT', '/orgs/{org}/sections/{id}/position', 'SetupService/MoveSection'],
  ['DELETE', '/orgs/{org}/sections/{id}', 'SetupService/DeleteSection'],
  ['GET', '/orgs/{org}/task-sections', 'SetupService/ListTaskSections', 'sections'], ['POST', '/orgs/{org}/task-sections', 'SetupService/CreateTaskSection', 'section'],
  ['PATCH', '/orgs/{org}/task-sections/{id}', 'SetupService/RenameTaskSection', 'section'], ['PUT', '/orgs/{org}/task-sections/{id}/position', 'SetupService/MoveTaskSection'],
  ['DELETE', '/orgs/{org}/task-sections/{id}', 'SetupService/DeleteTaskSection'],
  ['GET', '/orgs/{org}/plans', 'PlanService/ListPlans', 'plans'], ['POST', '/orgs/{org}/plans', 'PlanService/CreatePlan', 'plan'],
  ['GET', '/orgs/{org}/plans/{plan}', 'PlanService/GetPlan', 'plan'], ['PATCH', '/orgs/{org}/plans/{plan}', 'PlanService/RenamePlan', 'plan'],
  ['DELETE', '/orgs/{org}/plans/{plan}', 'PlanService/DeletePlan'],
  ['GET', '/orgs/{org}/plans/{plan}/events', 'PlanService/ListEvents', 'events'], ['POST', '/orgs/{org}/plans/{plan}/events', 'PlanService/AddEvents', 'events'],
  ['POST', '/orgs/{org}/plans/{plan}/events/{id}/copy', 'PlanService/CopyEvent', 'events'], ['PATCH', '/orgs/{org}/plans/{plan}/events/{id}', 'PlanService/UpdateEvent', 'event'],
  ['DELETE', '/orgs/{org}/plans/{plan}/events/{id}', 'PlanService/DeleteEvent'],
  ['GET', '/orgs/{org}/plans/{plan}/places', 'PlanService/ListPlaces', 'places'], ['PUT', '/orgs/{org}/plans/{plan}/events/{event}/places/{task}', 'PlanService/SetPlaces'],
  ['DELETE', '/orgs/{org}/plans/{plan}/events/{event}/places/{task}', 'PlanService/ResetPlaces'], ['PUT', '/orgs/{org}/plans/{plan}/places', 'PlanService/SetPlacesMany'],
  ['GET', '/orgs/{org}/plans/{plan}/fields', 'PlanService/ListFields'], ['POST', '/orgs/{org}/plans/{plan}/fields', 'PlanService/AddField', 'field'],
  ['PATCH', '/orgs/{org}/plans/{plan}/fields/{id}', 'PlanService/RenameField', 'field'], ['PUT', '/orgs/{org}/plans/{plan}/fields/{id}/position', 'PlanService/MoveField'],
  ['PUT', '/orgs/{org}/plans/{plan}/fields/{id}/frozen', 'PlanService/FreezeField'], ['DELETE', '/orgs/{org}/plans/{plan}/fields/{id}', 'PlanService/DeleteField'],
  ['PUT', '/orgs/{org}/plans/{plan}/events/{event}/fields/{field}', 'PlanService/SetFieldValue'],
  ['GET', '/orgs/{org}/plans/{plan}/answers', 'PlanService/ListAnswers'], ['PUT', '/orgs/{org}/plans/{plan}/people/{person}/availability', 'PlanService/SetAvailability'],
  ['PUT', '/orgs/{org}/plans/{plan}/events/{event}/notes/{person}', 'PlanService/SetNote'], ['PUT', '/orgs/{org}/plans/{plan}/answers', 'PlanService/SetAnswers'],
  ['GET', '/orgs/{org}/plans/{plan}/cells', 'DraftService/ListCells', 'cells'], ['POST', '/orgs/{org}/plans/{plan}/cells', 'DraftService/AddCell', 'cell'],
  ['DELETE', '/orgs/{org}/plans/{plan}/cells/{id}', 'DraftService/DeleteCell'],
  ['GET', '/orgs/{org}/plans/{plan}/pools', 'DraftService/ListPools', 'pools'], ['PUT', '/orgs/{org}/plans/{plan}/pools', 'DraftService/SetPool'],
  ['PUT', '/orgs/{org}/plans/{plan}/locks', 'DraftService/SetLocks'],
  ['DELETE', '/orgs/{org}/plans/{plan}/draft', 'DraftService/SetDraftAside'], ['PUT', '/orgs/{org}/plans/{plan}/draft', 'DraftService/ClearDraftAside'],
  ['GET', '/orgs/{org}/plans/{plan}/weights', 'DraftService/GetWeights'], ['PUT', '/orgs/{org}/plans/{plan}/weights', 'DraftService/SetWeights'],
  ['POST', '/orgs/{org}/plans/{plan}/runs', 'DraftService/StartRun'], ['GET', '/orgs/{org}/plans/{plan}/runs', 'DraftService/ListRuns', 'runs'],
  ['GET', '/orgs/{org}/plans/{plan}/runs/{id}', 'DraftService/GetRun'],
  ['PUT', '/orgs/{org}/plans/{plan}/runs/{id}/pin', 'DraftService/PinRun'], ['DELETE', '/orgs/{org}/plans/{plan}/runs/{id}/pin', 'DraftService/UnpinRun'],
].map(([method, path, proc, unwrap]) => ({ method, parts: path.split('/').filter(Boolean), proc, unwrap }));

// call: the route of a method and path, and the request it makes (a query's confirm and lang are fields too).
export function call(method, path, body?) {
  const [p, query] = path.split('?');
  const segs = p.split('/').filter(Boolean).map(decodeURIComponent);
  for (const r of ROUTES) {
    if (r.method !== method || r.parts.length !== segs.length) continue;
    const req: Record<string, any> = {};
    if (!r.parts.every((part, i) => {
      if (part[0] !== '{') return part === segs[i];
      const name = part.slice(1, -1);
      req[name] = name === 'org' || name === 'plan' ? segs[i] : segs[i] === 'latest' ? 0 : Number(segs[i]);
      return true;
    })) continue;
    for (const [k, v] of new URLSearchParams(query || '')) req[k] = v;
    return { route: r, req: { ...body, ...req }, org: req.org || '' };
  }
  throw new Error('no route for ' + method + ' ' + path);
}

export async function api(method, path, body?) {
  const { route: r, req, org } = call(method, path, body);
  if (r.proc === 'DraftService/GetRun') req.lang = currentLang;  // the server writes explanations and warnings in the page's language
  const headers = { 'Content-Type': 'application/json' };
  if (method !== 'GET') headers['X-Dutyplan-Client'] = CLIENT;  // the stream tells this tab's own changes apart
  let res;
  try {
    // A sign-in in front of the app (forward auth) answers an expired session with a redirect to its own site, which a fetch
    // cannot follow (another origin): «manual» turns that into an answer the page can recognise.
    res = await fetch('/dutyplan.v1.' + r.proc, { method: 'POST', headers, redirect: 'manual', body: JSON.stringify(req) });
  } catch (e) {
    connection('offline');
    throw Object.assign(new Error(t('net.offline')), { connection: 'offline' });
  }
  // Signed out: a redirect, a refusal from the sign-in, or a page that is not ours (the API always answers JSON).
  const json = (res.headers.get('Content-Type') || '').startsWith('application/json');
  if (res.type === 'opaqueredirect' || res.status === 401 || (res.ok && !json)) {
    connection('signed_out');
    throw Object.assign(new Error(t('net.signed_out')), { connection: 'signed_out' });
  }
  connection('');
  const data = await res.json().catch(() => ({}));
  if (method !== 'GET') forget(changedBy(procOf(r.proc)), org);  // also when refused: what is shown may be out of date
  if (!res.ok) {
    // A Connect error: its first detail is the Problem (kind, code, params) the texts are made from.
    const p = (data.details && data.details[0] && data.details[0].debug) || {};
    const kind = p.error || (data.code === 'invalid_argument' ? 'invalid' : data.error || data.code);
    throw Object.assign(new Error(data.message || res.statusText), { kind, apiCode: p.code || (data.error ? data.code : undefined), params: p.params || data.params, status: res.status });
  }
  return r.unwrap ? data[r.unwrap] : data;
}
export const procOf = (proc) => proc.slice(proc.indexOf('/') + 1);

// ---- the connection: signed out, or the server out of reach ----------------------------------------------------------
// A bar at the top that says so and offers the way back: sign in again (a reload goes through the sign-in and comes back to
// this address) or try again. The page stays as it was under it.
export let connState = '';
export function connection(state) {
  if (state === connState) return;
  connState = state;
  let bar = document.getElementById('conn');
  if (!state) { if (bar) bar.remove(); return; }
  if (!bar) {
    bar = h('div', { id: 'conn', role: 'alert' });
    document.body.prepend(bar);
  }
  bar.className = state;
  bar.replaceChildren(connectionLine(state));
}
// What is wrong, and the button that puts it right.
export function connectionLine(state) {
  const retry = () => { connection(''); fetched.clear(); if (!live) follow(route().org); render(); };
  return h('span', { class: 'conn-line' }, h('span', {}, t(state === 'signed_out' ? 'net.signed_out' : 'net.offline')), ' ',
    state === 'signed_out'
      ? h('button', { type: 'button', onclick: signIn }, t('net.sign_in'))
      : h('button', { type: 'button', onclick: (e) => { const d = e.currentTarget.closest('dialog'); if (d) d.close(); retry(); } }, t('net.retry')));
}
// Is the server still there, and are we still signed in? Asked when the stream breaks.
export const probe = () => api('GET', '/health').then(() => true, () => false);

// ---- what has been fetched ------------------------------------------------------------------------------------------
// The page keeps what it has fetched (GET path -> the answer), and a redraw takes it from here. A write (this tab's, or one
// the stream announces) forgets only what it can have changed, so a click fetches one or two things again, not everything.
export const fetched = new Map();
export function get(path) {
  if (!fetched.has(path)) {
    const p = api('GET', path);
    fetched.set(path, p);
    p.catch(() => { if (fetched.get(path) === p) fetched.delete(path); });
  }
  return fetched.get(path);
}
// What a fetched path holds: '/orgs' → orgs, '/orgs/x' → org, '/orgs/x/people' → people, '/orgs/x/plans/y' → plan,
// '/orgs/x/plans/y/runs/7' → runs, '/health' → health.
export function kindOf(path) {
  const p = path.split('/').filter(Boolean);
  if (p[0] !== 'orgs') return p[0];
  if (p.length === 1) return 'orgs';
  if (p.length === 2) return 'org';
  if (p[2] === 'plans' && p.length >= 4) return p.length === 4 ? 'plan' : p[4];
  return p[2];
}
// What a write can have changed, as kinds of fetched paths, by the procedure's name («SetPlaces»; the stream also says
// «RunFinished» and «Joined»). null = anything (when unsure, everything is fetched again). Every change to the data can
// make a draft out of date, so the runs are always fetched again (they say «stale»).
export const RUNS_TOO = (...kinds) => ['runs', ...kinds];
export const CHANGES = {
  CreateOrg: ['orgs'], CreatePlan: ['plans'], RenamePlan: ['plan', 'plans'],  // a new plan may get the address again
  SetNote: ['answers'],  // a note on an answer: never in the planner's input
  SetPlaces: RUNS_TOO('places'), ResetPlaces: RUNS_TOO('places'), SetPlacesMany: RUNS_TOO('places'),
  SetFieldValue: ['fields'], AddField: ['fields'], RenameField: ['fields'], MoveField: ['fields'], FreezeField: ['fields'], DeleteField: ['fields'],
  AddEvents: RUNS_TOO('events', 'places', 'fields', 'cells', 'pools', 'answers'), CopyEvent: RUNS_TOO('events', 'places', 'fields', 'cells', 'pools', 'answers'),
  UpdateEvent: RUNS_TOO('events', 'places', 'fields', 'cells', 'pools', 'answers'), DeleteEvent: RUNS_TOO('events', 'places', 'fields', 'cells', 'pools', 'answers'),
  SetAvailability: RUNS_TOO('answers'), SetAnswers: RUNS_TOO('answers'),
  SetLocks: RUNS_TOO('cells'),  // names of the draft locked or unlocked
  SetDraftAside: ['plan', 'runs'], ClearDraftAside: ['plan', 'runs'],  // «Remove unlocked»: no draft on show
  StartRun: ['runs', 'plan'], PinRun: ['runs', 'plan'], UnpinRun: ['runs', 'plan'], RunFinished: ['runs', 'plan'],  // a new draft or a pin ends «Remove unlocked»
  AddCell: RUNS_TOO('cells'), DeleteCell: RUNS_TOO('cells'), SetPool: RUNS_TOO('pools'), SetWeights: RUNS_TOO('weights'),
  SetPersonTask: RUNS_TOO('answers'), RemovePersonTask: RUNS_TOO('answers'),
  CreatePerson: RUNS_TOO('people'), UpdatePerson: RUNS_TOO('people'), MovePerson: RUNS_TOO('people'),
  SetCombination: RUNS_TOO('combinations'), SetTaskCombinations: RUNS_TOO('combinations'),
  CreateSection: ['sections', 'people'], RenameSection: ['sections', 'people'], MoveSection: ['sections', 'people'], DeleteSection: ['sections', 'people'],
  CreateTaskSection: ['task-sections', 'tasks'], RenameTaskSection: ['task-sections', 'tasks'], MoveTaskSection: ['task-sections', 'tasks'],
  DeleteTaskSection: ['task-sections', 'tasks'],
};  // tasks, the organisation, members, links, deleting a plan and anything else: everything
export const changedBy = (proc) => CHANGES[proc] || null;
// forget drops what a write in an organisation can have changed (kinds; null = everything of the organisation).
export function forget(kinds, orgSlug) {
  const org = orgSlug ? '/orgs/' + enc(orgSlug) : '';
  for (const key of [...fetched.keys()]) {
    if (!kinds) { if (!org || key.startsWith(org + '/') || key === org || key === '/orgs') fetched.delete(key); continue; }
    if (kinds.includes(kindOf(key)) && (key === '/orgs' || !org || key === org || key.startsWith(org + '/'))) fetched.delete(key);
  }
}

// The text for a refused request, in the page's language: from the error code and its parameters when the server sent
// them, else from the kind of error, else the server's own (English) text.
export function errorText(e) {
  if (e.apiCode && hasMsg('err.' + e.apiCode)) {
    const p = { ...e.params };
    if (typeof p.kind === 'string') p.kind = cap(hasMsg('kind.' + p.kind.replace(' ', '_')) ? t('kind.' + p.kind.replace(' ', '_')) : p.kind);
    if (typeof p.what === 'string' && hasMsg('what.' + p.what)) p.what = t('what.' + p.what);
    if (typeof p.key === 'string' && hasMsg('weight.' + p.key + '.label')) p.key = t('weight.' + p.key + '.label');
    return t('err.' + e.apiCode, p);
  }
  if (e.kind && hasMsg('err.kind.' + e.kind)) return t('err.kind.' + e.kind);
  return e.message || t('err.generic');
}

export const cancelled = () => Object.assign(new Error('cancelled'), { cancelled: true });
// Ask before something destructive, in the page's own dialog (ask); inside a write() action, a «no» ends the action quietly.
// (2026-11-22, the user: "La alle dialoger og bekreftelser være pent rendret." — no confirm() or prompt() of the browser.)
export async function confirmed(title, text, yes) { if (!(await ask(title, text, yes))) throw cancelled(); }

// A write: shows the error instead of throwing, and says whether it went through.
export async function write(action) {
  try {
    await action();
    say('');
    return true;
  } catch (e) {
    if (e.cancelled) return false;
    if (e.connection) {  // the bar at the top says it, and how to get back; a dialog covers it, so the dialog says it too
      const m = document.querySelector('dialog[open] [data-msg]');
      if (m) { m.className = 'err'; m.replaceChildren(connectionLine(e.connection)); } else say('');
      return false;
    }
    say(errorText(e), 'err');
    return false;
  }
}
