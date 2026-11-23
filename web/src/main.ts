import { api, changedBy, errorText, fetched, forget, get, probe } from './api';
import { CLIENT, h, say, store, type Page } from './core';
import { pageHome, pageMembers, pageOrg, pageSettings } from './org';
import { pagePlan, sideways } from './plan';
import { brand, headerSignedOut, link, me, pageLanding, renderHeader, route } from './shell';
import { focusKey, refocus } from './ui';
import { enc } from './util';

export let renderSeq = 0;
export let lastPath = '';
export async function render() {
  const seq = ++renderSeq;
  const main = document.getElementById('main')!;
  const r = route();
  try {
    const who = await me();
    if (seq !== renderSeq) return;
    if (who.login && !who.user && (!(who.links || []).length || new URLSearchParams(location.search).has('invite'))) {
      headerSignedOut();
      main.replaceChildren(pageLanding());
      document.title = t('brand');
      if (new URLSearchParams(location.search).get('signin') === 'failed') say(t('landing.failed'), 'err');
      return;
    }
    let people = [];
    let org: any = null;
    if (r.org) {
      [org, people] = await Promise.all([get('/orgs/' + enc(r.org)), get(`/orgs/${enc(r.org)}/people`)]);
      store.set('org', r.org);
    }
    if (r.page === 'none') throw Object.assign(new Error(t('page.not_found')), { status: 404, keepOrg: !!org });
    const [, body] = await Promise.all([renderHeader(r), r.page === 'home' ? pageHome() : r.page === 'org' ? pageOrg(r, org) : r.page === 'members' ? pageMembers(r, org)
      : r.page === 'settings' ? pageSettings(r, org) : pagePlan(r, org, people)]);
    if (seq !== renderSeq || !body) return;
    // The same page again (after a change): stay where we are, sideways in the tables too, with the focus where it was.
    const same = location.pathname === lastPath;
    const y = same ? window.scrollY : 0;
    const sideways = same ? [...main.querySelectorAll<HTMLElement>('.grid-scroll')].map((g) => [g.scrollLeft, g.scrollTop]) : [];
    const focus = same ? focusKey(main) : null;
    main.replaceChildren(body);
    lastPath = location.pathname;
    main.querySelectorAll<HTMLElement>('.grid-scroll').forEach((g, i) => { if (sideways[i]) [g.scrollLeft, g.scrollTop] = sideways[i]; });
    window.scrollTo(0, y);
    refocus(main, focus);
    if ((body as Page).shown) (body as Page).shown!();
    else document.querySelectorAll<HTMLDialogElement>('dialog.modal[open][data-key]').forEach((d) => d.close());
    follow(r.org);
    document.title = t('brand') + (org ? ' · ' + org.name : '');
  } catch (e) {
    if (seq !== renderSeq) return;
    if (e.connection && main.firstChild) return;  // keep what is shown; the bar at the top says what is wrong
    if (e.status === 404 && r.org && !e.keepOrg) store.set('org', '');
    main.replaceChildren(h('div', {}, h('h1', {}, e.status === 404 ? t('page.not_found') : t('page.error')), h('p', {}, link('/?switch', t('page.choose_org'))), h('p', { class: 'muted small' }, errorText(e))));
  }
}

// ---- live: the organisation's stream -----------------------------------------------------------------------------------
// The server says when something changed in the organisation (another tab, another person, a planning run that ended).
// The page then fetches again and redraws in place. While a field is being typed in, the redraw waits until it is left.
export let live: { org: string; es: EventSource } | null = null;
export let liveTimer = 0;
export function follow(org) {
  if (live && live.org === org) return;
  if (live) { live.es.close(); fetched.clear(); }  // nothing told this tab about the other organisation's changes
  live = null;
  if (!org || typeof EventSource === 'undefined') return;
  const es = new EventSource(`/api/orgs/${enc(org)}/stream`);
  let broken = false;
  let fails = 0;  // errors in a row without getting through (a sign-in in front that has expired answers with a redirect)
  es.addEventListener('message', (e) => {
    let c: { origin?: string; proc?: string; plan?: string } = {};
    try { c = JSON.parse(e.data); } catch (err) { /* a change all the same */ }
    if (c.origin === CLIENT) return;  // this tab's own write: it has forgotten and redrawn already
    forget(c.proc ? changedBy(c.proc) : null, org);
    liveRefresh();
  });
  es.addEventListener('error', () => {
    broken = true;
    if (fails === 0) probe();  // shows the bar at once if the sign-in has expired or the server is gone
    // Give up after a few tries instead of knocking every 3 s; try again when the tab is looked at, or in a minute.
    if (++fails < 5 || !live || live.es !== es) return;
    es.close();
    live = null;
    const retry = () => { clearTimeout(timer); document.removeEventListener('visibilitychange', onShow); if (!live) { fetched.clear(); follow(org); liveRefresh(); } };
    const onShow = () => { if (document.visibilityState === 'visible') retry(); };
    const timer = setTimeout(retry, 60000);
    document.addEventListener('visibilitychange', onShow);
  });
  es.addEventListener('open', () => { if (broken) { fetched.clear(); liveRefresh(); } broken = false; fails = 0; });  // back after a break: changes may be missed
  live = { org, es };
}
export const typing = () => {
  const a = document.activeElement as HTMLInputElement | null;
  return !!a && (a.tagName === 'TEXTAREA' || (a.tagName === 'INPUT' && !['checkbox', 'radio', 'button', 'submit'].includes(a.type)));
};
export function liveRefresh() {
  clearTimeout(liveTimer);
  liveTimer = setTimeout(() => {
    if (typing()) { document.activeElement!.addEventListener('blur', liveRefresh, { once: true }); return; }
    render();
  }, 150);
}

window.addEventListener('popstate', render);
render();
