
// Dutyplan: a small frontend without a framework. The organisation is in the address (/o/<org>/...), so switching
// organisation is just going to another address. There is no login. All texts come from i18n.js (t, tn); the language is
// a per-browser choice.

export const store = {
  get: (k) => { try { return localStorage.getItem('tp.' + k) || ''; } catch (e) { return ''; } },
  set: (k, v) => { try { localStorage.setItem('tp.' + k, v); } catch (e) { /* private mode */ } },
};

// This tab's id: sent with every write, so the change the stream announces back can be skipped (the tab has redrawn already).
export const CLIENT = Math.random().toString(36).slice(2) + Date.now().toString(36);

// A page's content; shown() runs once it is in the document (to open a dialog again, or a field).
// shown runs once the page is in the document; gone when it is replaced (a redraw too): what it set up outside itself goes with it
export type Page = HTMLElement & { shown?: () => unknown; gone?: () => void };

// What h() takes as properties: attributes, class, the value/checked/disabled properties, and on<event> listeners.
export type Props = Record<string, any> | null | undefined;
// A child: a node, text (anything printable), or a list of them (flattened two deep).
export type Kid = Node | string | number | boolean | null | undefined | Kid[];
export function h<K extends keyof HTMLElementTagNameMap>(tag: K, props?: Props, ...kids: Kid[]): HTMLElementTagNameMap[K] {
  const el: any = document.createElement(tag);
  for (const [k, v] of Object.entries(props || {})) {
    if (v === false || v == null) continue;
    if (k === 'class') el.className = v;
    else if (k === 'checked' || k === 'value' || k === 'disabled') el[k] = v;
    else if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? '' : v);
  }
  for (const kid of (kids as any[]).flat(2)) el.append(kid instanceof Node ? kid : document.createTextNode(String(kid ?? '')));
  return el;
}

// Small line icons (after Feather, MIT), drawn in the colour of the text around them.
export const ICONS = {
  up: '<path d="M14 9V5a3 3 0 0 0-3-3l-4 9v11h11.28a2 2 0 0 0 2-1.7l1.38-9a2 2 0 0 0-2-2.3zM7 22H4a2 2 0 0 1-2-2v-7a2 2 0 0 1 2-2h3"/>',
  down: '<path d="M10 15v4a3 3 0 0 0 3 3l4-9V2H5.72a2 2 0 0 0-2 1.7l-1.38 9a2 2 0 0 0 2 2.3zm7-13h2.67A2.31 2.31 0 0 1 22 4v7a2.31 2.31 0 0 1-2.33 2H17"/>',
  maybe: '<path d="M8.5 8.5a3.5 3.5 0 1 1 5.2 3.05c-1.1.62-1.7 1.3-1.7 2.45v.5M12 19h.01" stroke-width="3"/>',
  auto: '<path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/>',
  info: '<circle cx="12" cy="12" r="10"/><path d="M12 16v-4M12 8h.01"/>',
  share: '<circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><path d="M8.59 13.51l6.83 3.98M15.41 6.51l-6.82 3.98"/>',
  left: '<path d="M15 18l-6-6 6-6"/>', right: '<path d="M9 18l6-6-6-6"/>',
  lock: '<rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/>',
  unlock: '<rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 7.75-1.4"/>',
  more: '<circle cx="5" cy="12" r="1.4" fill="currentColor"/><circle cx="12" cy="12" r="1.4" fill="currentColor"/><circle cx="19" cy="12" r="1.4" fill="currentColor"/>',
  filter: '<path d="M22 3H2l8 9.46V19l4 2v-8.54z"/>',
  hand: '<path d="M12 20h9"/><path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4z"/>',
  users: '<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/>',
  settings: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.6 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.6a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>',
  logout: '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><path d="M16 17l5-5-5-5M21 12H9"/>',
  check: '<path d="M20 6L9 17l-5-5"/>', caret: '<path d="M6 9l6 6 6-6"/>',
  today: '<rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/><circle cx="12" cy="16" r="1.6" fill="currentColor"/>',
  pin: '<path d="M12 17v5M9 3h6l-1 6 4 4H6l4-4z"/>',
  undo: '<path d="M9 14L4 9l5-5"/><path d="M4 9h10.5a5.5 5.5 0 0 1 0 11H11"/>',
  x: '<path d="M18 6L6 18M6 6l12 12"/>',
  copy: '<rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>',
  trash: '<path d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/>',
  eye: '<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/>',
  join: '<path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="8.5" cy="7" r="4"/><path d="M20 8v6M23 11h-6"/>',
  rowsel: '<line x1="3" y1="12" x2="15" y2="12"/><polyline points="10 7 15 12 10 17"/><line x1="20" y1="5" x2="20" y2="19"/>',
  code: '<polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/>',
  message: '<path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z"/>',
  file: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/>',
  chevron: '<path d="M9 18l6-6-6-6"/>',
  download: '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><path d="M7 10l5 5 5-5M12 15V3"/>',
  sliders: '<path d="M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6"/>',
};
// mark: the square that says a place is locked (kept by the planner) or set by hand.
export const mark = (kind) => h('span', { class: 'mark ' + kind, title: t(kind === 'lock' ? 'draft.key_locked' : 'draft.key_hand'), 'aria-label': t(kind === 'lock' ? 'draft.key_locked' : 'draft.key_hand'), role: 'img' }, icon(kind));
// Each icon is parsed once and then cloned: the table has thousands, and parsing each was a large part of a redraw.
const iconMade = new Map<string, HTMLElement>();
export function icon(name) {
  let s = iconMade.get(name);
  if (!s) {
    s = h('span', { class: 'ic ic-' + name, 'aria-hidden': 'true' });
    s.innerHTML = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${ICONS[name]}</svg>`;
    iconMade.set(name, s);
  }
  return s.cloneNode(true) as HTMLElement;
}
// The three levels every choice has: can't (red thumb down), if needed (yellow ?), good (green thumb up).
export const LEVEL_ICON = { never: 'down', '': 'down', cannot: 'down', poor: 'maybe', standard: 'maybe', good: 'up', practical: 'up' };

// A message: in the open dialog, else over the page (#msg), which goes away by itself (a click also hides it).
export let sayTimer = 0;
export function say(text, kind?) {
  const m = (document.querySelector<HTMLElement>('dialog[open] [data-msg]') || document.getElementById('msg'))!;
  m.textContent = text || '';
  m.className = text ? kind || 'ok' : '';
  if (m.id !== 'msg') return;
  clearTimeout(sayTimer);
  m.onclick = () => say('');
  if (text) sayTimer = setTimeout(() => { m.textContent = ''; m.className = ''; }, kind === 'err' ? 8000 : 4000);
}

export const cap = (s) => s.charAt(0).toUpperCase() + s.slice(1);
export const locale = () => (currentLang === 'nb' ? 'nb-NO' : 'en-GB');
