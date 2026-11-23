import { api, fetched, get } from './api';
import { h, icon, store } from './core';
import { live, render } from './main';
import { enc } from './util';

export function go(path) {
  history.pushState({}, '', path);
  render();
}
export function link(path, text, props?) {
  return h('a', { href: path, ...props, onclick: (e) => { if (e.metaKey || e.ctrlKey || e.shiftKey) return; e.preventDefault(); go(path); } }, text);
}
export const linkBtn = (text, onclick, aria?) => h('button', { type: 'button', class: 'link-button', 'aria-label': aria, onclick }, text);

// ---- signing in ------------------------------------------------------------------------------------------------------
// AccountService/Me: {login: false} when the server has no login (everyone may do everything), else {login: true, user: {name, email} or null}.
export let ME: any = null;
export async function me() {
  if (!ME) ME = await api('GET', '/me');
  return ME;
}
// to the sign-in, and back to this page afterwards (without a login: the sign-in in front of the app, by loading again)
export function signIn() {
  if (ME && !ME.login) return location.reload();
  location.href = '/auth/login?next=' + encodeURIComponent(location.pathname + location.search);
}
export async function signOut() {
  let to = '/';
  try {
    const res = await fetch('/auth/logout', { method: 'POST' });
    to = (await res.json()).to || '/';
  } catch (e) { /* signed out here all the same, or the next page says the server is gone */ }
  store.set('org', '');
  location.href = to;
}

// The start for someone not signed in: what this is, and the way in.
// deadLink: /s/<token> sends a link that no longer works to /?link=invalid; say so where it lands.
export function deadLink() {
  return new URLSearchParams(location.search).get('link') === 'invalid' ? h('p', { class: 'notice', role: 'status' }, t('link.invalid')) : '';
}

// invitedBox: /s/<invitation> opened signed out comes here as /?invite=<token>&org=<name>: who invites, and the way in
// (signing in goes back to the invitation, which then makes the person a member).
export function invitedBox() {
  const q = new URLSearchParams(location.search);
  const token = q.get('invite');
  if (!token) return '';
  return h('div', { class: 'invited', role: 'status' }, icon('join'),
    h('div', {}, h('h2', {}, t('invite.title', { org: q.get('org') || '' })), h('p', {}, t(ME.signup ? 'invite.text_signup' : 'invite.text')),
      h('p', { class: 'row-inline' }, h('a', { class: 'button solid', href: '/auth/login?next=' + encodeURIComponent('/s/' + token) }, t('invite.sign_in')),
        ME.signup ? h('a', { class: 'button', href: ME.signup }, t('landing.sign_up')) : '')));
}

export function pageLanding() {
  const point = ([k, ic]: string[]) => h('li', {}, h('span', { class: 'point-ic' }, icon(ic)), h('h3', {}, t('landing.' + k + '_title')), h('p', {}, t('landing.' + k)));
  const step = ([title, text]: string[], i: number) => h('li', {}, h('span', { class: 'step-n' }, String(i + 1)), h('h3', {}, t(title)), h('p', {}, t(text)));
  // the film of how it works (scripts/video.js), in the page's language; still, with its controls, for those who ask for
  // less motion
  const still = typeof window.matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
  const film = h('video', { class: 'demo', src: '/demo-' + currentLang + '.mp4', poster: '/landing.png', width: '1024', height: '576',
    controls: true, loop: true, playsinline: true, preload: still ? 'none' : 'auto', autoplay: !still, 'aria-label': t('landing.video') });
  film.muted = true; // muted as a property too: browsers only start a video by themselves when it has no sound
  return h('div', { class: 'landing' },
    deadLink(),
    invitedBox(),
    h('section', { class: 'hero' },
      h('img', { class: 'logo', src: '/logo.svg', alt: '', width: '56', height: '56' }),
      h('h1', {}, t('landing.title')),
      h('p', { class: 'lead' }, t('landing.lead')),
      h('p', { class: 'row-inline cta' }, h('button', { type: 'button', onclick: signIn }, t('landing.sign_in')),
        ME.signup ? h('a', { class: 'button ghost', href: ME.signup }, t('landing.sign_up')) : ''),
      h('p', { class: 'muted small' }, t(ME.signup ? 'landing.account_signup' : 'landing.account')),
      h('p', { class: 'free' }, t('landing.free'))),
    h('figure', { class: 'demo' }, film, h('figcaption', { class: 'muted small' }, t('landing.video_caption'))),
    h('section', { class: 'how' }, h('h2', {}, t('landing.how')), h('ol', { class: 'steps' }, [['landing.step_dates_title', 'landing.step_dates'], ['landing.step_answers_title', 'landing.step_answers'],
      ['landing.step_draft_title', 'landing.step_draft'], ['landing.step_share_title', 'landing.step_share']].map(step))),
    h('ul', { class: 'points' }, [['tap', 'up'], ['fair', 'check'], ['live', 'users'], ['share', 'share']].map(point)),
    about());
}

// About: why it exists, that it is free, and how to reach the maker (the same address as Sermon Studio's).
export const CONTACT = 'contact@sermon-studio.app';
export const REPO = 'https://github.com/gunnaringe/dutyplan';
// withLinks: a text whose {name} placeholders become the given nodes (links inside a translated sentence).
export function withLinks(key, nodes) {
  return t(key).split(/\{(\w+)\}/).map((part, i) => (i % 2 ? nodes[part] : part));
}
export function about() {
  const ext = (href, ic, key, title?) => h('li', {}, h('a', { href, title: title ? t(title) : null }, icon(ic), h('span', {}, t(key))));
  return h('footer', { class: 'about', id: 'about' },
    h('h2', {}, t('about.title')),
    h('p', {}, t('about.why')),
    h('p', {}, t('about.free')),
    h('p', {}, withLinks('about.contact', { email: h('a', { href: 'mailto:' + CONTACT }, CONTACT), github: h('a', { href: REPO + '/issues' }, 'GitHub') })),
    h('p', {}, h('a', { class: 'signature', href: 'https://gunnaringe.sort.land', title: 'gunnaringe.sort.land' }, 'Gunnar Inge Gjøvik Sortland')),
    h('ul', { class: 'about-links' },
      ext(REPO, 'code', 'about.source', 'about.source_title'),
      ext(REPO + '/issues', 'message', 'about.issues', 'about.issues_title'),
      ext(REPO + '/blob/main/LICENSE', 'file', 'about.licence')));
}

// brand: the logo and the app's name, linking home.
export function brand(to) {
  const a = link(to, '', { class: 'brand' });
  a.append(h('img', { src: '/logo.svg', alt: '', width: '28', height: '28' }), h('span', {}, t('brand')));
  return a;
}

export function headerSignedOut() {
  document.getElementById('top')!.replaceChildren(
    brand('/'),
    h('span', { class: 'spacer' }),
    langMenu(),
    h('button', { type: 'button', class: 'sm', onclick: signIn }, t('landing.sign_in')));
}

export function route() {
  const parts = location.pathname.split('/').filter(Boolean);
  if (parts[0] === 'o' && parts[1]) {
    if (parts.length === 2) return { page: 'org', org: parts[1] };
    if (parts.length === 3 && (parts[2] === 'members' || parts[2] === 'settings')) return { page: parts[2], org: parts[1] };
    if (parts.length === 4 && parts[2] === 'p') return { page: 'plan', org: parts[1], plan: parts[3] };
    return { page: 'none', org: parts[1] };
  }
  return { page: parts.length ? 'none' : 'home' };
}

export async function renderHeader(r) {
  const orgs = await get('/orgs');
  const select = h('select', { 'aria-label': t('header.org'), onchange: (e) => go(e.target.value === '' ? '/?switch' : '/o/' + enc(e.target.value)) },
    h('option', { value: '' }, r.org ? t('header.switch_org') : t('header.choose_org')),
    orgs.map((o) => h('option', { value: o.slug, selected: o.slug === r.org }, o.name)));
  const who = ME && ME.user;
  const org = r.org ? await get('/orgs/' + enc(r.org)).catch(() => null) : null;
  const role = org ? org.role || '' : '';
  const nav = (page, ic, label) => {
    const a = link(`/o/${enc(r.org)}/${page}`, '', { class: 'icon-btn' + (r.page === page ? ' on' : ''), title: label, 'aria-label': label });
    a.append(icon(ic));
    return a;
  };
  const tools: HTMLElement[] = [];
  if (r.org && role && ME && ME.login) tools.push(nav('members', 'users', t('nav.members')));
  if (r.org && role === 'admin') tools.push(nav('settings', 'settings', t('nav.settings')));
  document.getElementById('top')!.replaceChildren(
    brand('/?switch'),
    orgs.length && r.page !== 'home' ? h('label', { class: 'org-pick' }, h('span', { class: 'lbl' }, t('header.org')), select) : '',
    h('span', { class: 'spacer' }),
    ...tools,
    langMenu(),
    who ? userMenu(who) : ME && ME.login ? h('button', { type: 'button', class: 'sm', onclick: signIn }, t('landing.sign_in')) : '');
}

// dropdown: a button that opens a panel under it; closes on a click outside, Escape, or a choice.
export function dropdown(cls, button, label, ...panel) {
  const d = h('details', { class: 'dropdown ' + cls });
  const close = (e) => {
    if (!d.isConnected) return off();
    if (e.type === 'keydown' ? e.key === 'Escape' : !d.contains(e.target)) { d.open = false; if (e.type === 'keydown') d.querySelector('summary')!.focus(); }
  };
  const off = () => { document.removeEventListener('pointerdown', close); document.removeEventListener('keydown', close); };
  d.addEventListener('toggle', () => { off(); if (d.open) { document.addEventListener('pointerdown', close); document.addEventListener('keydown', close); } });
  d.append(h('summary', { title: label, 'aria-label': label }, button), h('div', { class: 'drop-panel' }, ...panel));
  return d;
}

// The language: a button with its code (EN, NB) and a list of the languages.
export function langMenu() {
  return dropdown('lang', [h('span', { class: 'code' }, currentLang.toUpperCase()), icon('caret')], t('lang.label') + ': ' + t('lang.' + currentLang),
    LANGS.map((l) => h('button', { type: 'button', class: 'drop-item' + (l === currentLang ? ' on' : ''), lang: l,
      onclick: () => { setLang(l); fetched.clear(); render(); } }, h('span', {}, t('lang.' + l)), l === currentLang ? icon('check') : '')));
}

// The person signed in: their initials in a circle; the menu has their name and Sign out.
export function initials(name) {
  const w = String(name || '?').trim().split(/\s+/).filter(Boolean);
  return ((w[0] || '?')[0] + (w.length > 1 ? w[w.length - 1][0] : '')).toUpperCase();
}
export function userMenu(who) {
  return dropdown('user', h('span', { class: 'avatar' }, initials(who.name)), who.name,
    h('div', { class: 'who' }, h('strong', {}, who.name), who.email ? h('span', { class: 'muted small' }, who.email) : ''),
    h('button', { type: 'button', class: 'ghost signout', onclick: signOut }, icon('logout'), h('span', {}, t('header.sign_out'))));
}
