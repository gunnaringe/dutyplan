import type { DescMessage, DescMethodUnary, DescService, MessageInitShape, MessageShape } from '@bufbuild/protobuf';
import { Code, ConnectError, createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { AccountService, DraftService, OrgService, PlanService, ProblemSchema, SetupService } from './gen/dutyplan/v1/dutyplan_pb';
import { CLIENT, cap, h, say } from './core';
import { follow, live, render } from './main';
import { ask } from './org';
import { route, signIn } from './shell';

// ---- the API: Connect (proto/dutyplan/v1/dutyplan.proto), with the client generated into web/src/gen ------------------
// call(Plan.setPlaces, { org, plan, event, task, slots }) makes one call; get(Plan.listPlaces, { org, plan }, 'places') reads
// through the page's cache (see «what has been fetched»). The last argument picks the field of the answer the page uses.
// On the wire the JSON is the API's own (snake_case, as in web/llms.txt); in the page the fields are camelCase.
export const Account = AccountService.method;
export const Org = OrgService.method;
export const Setup = SetupService.method;
export const Plan = PlanService.method;
export const Draft = DraftService.method;

export type Unary = DescMethodUnary<DescMessage, DescMessage>;
export type In<M extends Unary> = MessageInitShape<M['input']>;
// An answer as the server sends it: every field is on the wire (rpc.go), so the page takes a message inside one as there
// (an unset one comes as null, as before Connect); only the `optional` fields of the proto may be missing.
type Msg = { $typeName: string };
type MsgKeys<T> = { [K in keyof T]-?: [NonNullable<T[K]>] extends [Msg] ? K : never }[keyof T];
export type Sent<T> = T extends Msg ? { [K in MsgKeys<T>]-?: Sent<NonNullable<T[K]>> } & { [K in Exclude<keyof T, MsgKeys<T>>]: Sent<T[K]> }
  : T extends readonly (infer E)[] ? Sent<E>[] : T;
export type Out<M extends Unary> = Sent<MessageShape<M['output']>>;

// The names of the procedures («SetPlaces»; the stream says these, and «RunFinished» and «Joined»), and which only read.
type Methods<S> = S extends { method: infer M } ? Capitalize<keyof M & string> : never;
export type Procedure = Methods<typeof AccountService> | Methods<typeof OrgService> | Methods<typeof SetupService>
  | Methods<typeof PlanService> | Methods<typeof DraftService>;
export type Read = Extract<Procedure, `List${string}` | `Get${string}` | 'Health' | 'Me'>;
export type Write = Exclude<Procedure, Read>;
const reads = (m: Unary) => /^(List|Get)/.test(m.name) || m.name === 'Health' || m.name === 'Me';

// What went wrong, for errorText() and the pages: a refusal of the server (kind and code from its Problem detail), or the
// connection (signed out, out of reach: the bar at the top says it).
export type Failure = Error & {
  kind?: string; apiCode?: string; params?: Record<string, unknown>; status?: number;
  connection?: 'offline' | 'signed_out'; cancelled?: boolean; keepOrg?: boolean;
};
const lost = (state: 'offline' | 'signed_out'): Failure => Object.assign(new Error(t(state === 'signed_out' ? 'net.signed_out' : 'net.offline')), { connection: state });
// Connect's codes as the HTTP statuses the server answers with (rpc.go); the pages ask for 404.
const STATUS: Partial<Record<Code, number>> = { [Code.InvalidArgument]: 400, [Code.Unauthenticated]: 401, [Code.PermissionDenied]: 403,
  [Code.NotFound]: 404, [Code.Aborted]: 409, [Code.ResourceExhausted]: 429, [Code.Unavailable]: 503 };
function failure(e: unknown): Failure {
  const ce = ConnectError.from(e);
  const lostBy = [e, ce.cause].find((x) => x && (x as Failure).connection);  // pageFetch's, which the transport wraps
  if (lostBy) return lostBy as Failure;
  const p = ce.findDetails(ProblemSchema)[0];
  return Object.assign(new Error(ce.rawMessage), { kind: p ? p.error : ce.code === Code.InvalidArgument ? 'invalid' : Code[ce.code],
    apiCode: p && p.code ? p.code : undefined, params: p && p.params ? p.params : undefined, status: STATUS[ce.code] || 500 });
}

// A sign-in in front of the app (forward auth) answers an expired session with a redirect to its own site, which a fetch
// cannot follow (another origin): «manual» turns that into an answer the page can recognise. Signed out: a redirect, a
// refusal from the sign-in, or a page that is not ours (the API answers binary, and JSON when it refuses).
const pageFetch: typeof fetch = async (input, init) => {
  let res: Response;
  try {
    res = await fetch(input, { ...init, redirect: 'manual' });
  } catch (e) {
    connection('offline');
    throw lost('offline');
  }
  const ct = res.headers.get('Content-Type') || '';
  const ours = ct === 'application/proto' || ct.startsWith('application/json');  // an answer, or a refusal (always JSON)
  if (res.type === 'opaqueredirect' || res.status === 401 || (res.ok && !ours)) {
    connection('signed_out');
    throw lost('signed_out');
  }
  connection('');
  return res;
};
// Binary protobuf: smaller, and about 3× faster to read than the client's JSON (20 → 6 ms for 12 000 answers); scripts and
// the agent guide use JSON. Unknown fields are kept aside, never refused, so a tab opened before a deploy still reads
// answers that gained a field.
const transport = createConnectTransport({ baseUrl: '/', fetch: pageFetch, useBinaryFormat: true });
const clients = new Map<DescService, any>();
const clientOf = (m: Unary) => {
  if (!clients.has(m.parent)) clients.set(m.parent, createClient(m.parent, transport));
  return clients.get(m.parent);
};

// call makes one call. A write tells the stream which tab made it (X-Dutyplan-Client), and forgets what it can have changed,
// also when refused (what is shown may be out of date).
export async function call<M extends Unary>(m: M, req: In<M>): Promise<Out<M>>;
export async function call<M extends Unary, K extends keyof Out<M>>(m: M, req: In<M>, key: K): Promise<Out<M>[K]>;
export async function call<M extends Unary>(m: M, req: In<M>, key?: keyof Out<M>) {
  const write = !reads(m);
  const org = (req as { org?: string }).org || '';
  if (m.name === 'GetRun') req = { ...req, lang: currentLang } as In<M>;  // the server writes explanations and warnings in the page's language
  if (write) writesOut++;
  try {
    const res = await clientOf(m)[m.localName](req, write ? { headers: { 'X-Dutyplan-Client': CLIENT } } : {});
    if (write) forget(changedBy(m.name), org);
    return key ? res[key] : res;
  } catch (e) {
    const f = failure(e);
    if (write && !f.connection) forget(changedBy(m.name), org);
    throw f;
  } finally {
    if (write) writesOut--;
  }
}
// the writes this tab has sent that have not been answered yet (a redraw that can wait does: redrawSoon)
export let writesOut = 0;

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
export const probe = () => call(Account.health, {}).then(() => true, () => false);

// ---- what has been fetched ------------------------------------------------------------------------------------------
// The page keeps what it has fetched (the procedure and its request -> the answer), and a redraw takes it from here. A write
// (this tab's, or one the stream announces) forgets only what it can have changed, so a click fetches one or two things
// again, not everything.
type Fetched = { name: string; org: string; answer: Promise<unknown> };
export const fetched = new Map<string, Fetched>();
export function get<M extends Unary>(m: M, req: In<M>): Promise<Out<M>>;
export function get<M extends Unary, K extends keyof Out<M>>(m: M, req: In<M>, key: K): Promise<Out<M>[K]>;
export function get<M extends Unary>(m: M, req: In<M>, key?: keyof Out<M>) {
  const k = m.name + ' ' + JSON.stringify(req);
  let f = fetched.get(k);
  if (!f) {
    const answer = call(m, req);
    const entry: Fetched = { name: m.name, org: (req as { org?: string }).org || '', answer };
    fetched.set(k, entry);
    answer.catch(() => { if (fetched.get(k) === entry) fetched.delete(k); });
    f = entry;
  }
  return key ? f.answer.then((x) => (x as Out<M>)[key]) : f.answer;
}
// What a write can have changed, as the reads to fetch again, by the procedure's name («SetPlaces»; the stream also says
// «RunFinished» and «Joined»). Not listed = anything (when unsure, everything is fetched again). Every change to the data
// can make a draft out of date, so the runs are always fetched again (they say «stale»).
const RUNS: Read[] = ['ListRuns', 'GetRun'];
const RUNS_TOO = (...reads: Read[]): Read[] => [...RUNS, ...reads];
const EVENTS: Read[] = RUNS_TOO('ListEvents', 'ListPlaces', 'ListFields', 'ListCells', 'ListPools', 'ListAnswers');
const FIELDS: Read[] = ['ListFields'];
const DRAFT_ON_SHOW: Read[] = ['GetPlan', ...RUNS];  // a new draft or a pin ends «Remove unlocked»
const SECTIONS: Read[] = ['ListSections', 'ListPeople'];
const TASK_SECTIONS: Read[] = ['ListTaskSections', 'ListTasks'];
export const CHANGES: { [P in Write | 'RunFinished']?: Read[] } = {
  CreateOrg: ['ListOrgs'], CreatePlan: ['ListPlans'], RenamePlan: ['GetPlan', 'ListPlans'],  // a new plan may get the address again
  SetNote: ['ListAnswers'],  // a note on an answer: never in the planner's input
  SetPlaces: RUNS_TOO('ListPlaces'), ResetPlaces: RUNS_TOO('ListPlaces'), SetPlacesMany: RUNS_TOO('ListPlaces'),
  SetFieldValue: FIELDS, AddField: FIELDS, RenameField: FIELDS, MoveField: FIELDS, FreezeField: FIELDS, DeleteField: FIELDS,
  AddEvents: EVENTS, CopyEvent: EVENTS, UpdateEvent: EVENTS, DeleteEvent: EVENTS,
  SetAvailability: RUNS_TOO('ListAnswers'), SetAnswers: RUNS_TOO('ListAnswers'),
  SetLocks: RUNS_TOO('ListCells'),  // names of the draft locked or unlocked
  SetDraftAside: DRAFT_ON_SHOW, ClearDraftAside: DRAFT_ON_SHOW,  // «Remove unlocked»: no draft on show
  StartRun: DRAFT_ON_SHOW, PinRun: DRAFT_ON_SHOW, UnpinRun: DRAFT_ON_SHOW, RunFinished: DRAFT_ON_SHOW,
  AddCell: RUNS_TOO('ListCells'), DeleteCell: RUNS_TOO('ListCells'), SetPool: RUNS_TOO('ListPools'), SetWeights: RUNS_TOO('GetWeights'),
  SetPersonTask: RUNS_TOO('ListAnswers'), RemovePersonTask: RUNS_TOO('ListAnswers'),
  CreatePerson: RUNS_TOO('ListPeople'), UpdatePerson: RUNS_TOO('ListPeople'), MovePerson: RUNS_TOO('ListPeople'),
  SetCombination: RUNS_TOO('ListCombinations'), SetTaskCombinations: RUNS_TOO('ListCombinations'),
  CreateSection: SECTIONS, RenameSection: SECTIONS, MoveSection: SECTIONS, DeleteSection: SECTIONS,
  CreateTaskSection: TASK_SECTIONS, RenameTaskSection: TASK_SECTIONS, MoveTaskSection: TASK_SECTIONS, DeleteTaskSection: TASK_SECTIONS,
};  // tasks, the organisation, members, links, deleting a plan and anything else: everything
export const changedBy = (name: string): Read[] | null => CHANGES[name as Write] || null;
// forget drops what a write in an organisation can have changed (reads; null = everything of the organisation).
export function forget(changed: Read[] | null, org: string) {
  for (const [k, f] of [...fetched]) {
    const ours = f.name === 'ListOrgs' || !org || f.org === org;
    if (ours && (!changed || changed.includes(f.name as Read))) fetched.delete(k);
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
