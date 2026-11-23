import { write } from './api';
import { h, icon } from './core';
import { render } from './main';

// ---- building blocks -------------------------------------------------------------------------------------------------

// After a change in a dialog the page is drawn again with the new data, and the dialog stays open: the page's shown() builds
// it again from the new data, and modal() puts the new contents into the dialog that is open (no closing, no flicker).
// The same happens when the stream says another tab changed something.
// A dialog that does this has a key («person:12»), set by keyed().
export const keyed = (dlg, kind, id) => { dlg.dataset.key = kind + ':' + id; return dlg; };
export let refreshKey: string | null = null;
export const setRefreshKey = (key: string | null) => { refreshKey = key; };  // while set, modal() fills the open dialog with this key instead of opening a new one
export function reopened(handlers) {
  for (const dlg of document.querySelectorAll<HTMLDialogElement>('dialog.modal[open][data-key]')) {
    const [kind, id] = dlg.dataset.key!.split(/:(.*)/s);
    refreshKey = dlg.dataset.key!;
    if (handlers[kind]) handlers[kind](/^\d+$/.test(id) ? Number(id) : id);
    if (refreshKey) dlg.close();  // what it showed is gone (deleted in another tab)
    refreshKey = null;
  }
}

// An ⓘ button: the explanation of something, in a dialog of its own, instead of help text on the page.
export const infoBtn = (title, texts) => h('button', { type: 'button', class: 'info', title: t('common.info'), 'aria-label': t('common.info_aria', { name: title }),
  onclick: () => modal(title, h('div', { class: 'info-text' }, texts.filter(Boolean).map((x) => h('p', {}, x))), foot('', closeBtn())) }, icon('info'));

// A dialog over the page; messages (say) go into it while it is open, and it is removed when closed. A first argument
// { info: [texts] } puts an ⓘ in its head. key names a dialog that redraws in place (see again()).
export function modal(title, ...kids: any[]): HTMLDialogElement {
  const info = kids[0] && kids[0].info ? kids.shift().info : null;
  const key = refreshKey;
  refreshKey = null;
  const old = key && [...document.querySelectorAll<HTMLDialogElement>('dialog.modal[open]')].find((d) => d.dataset.key === key);
  const dlg = old || h('dialog', { class: 'modal', 'aria-label': title });
  const msg = old ? old.querySelector('[data-msg]')! : h('p', { 'data-msg': '', role: 'status' });
  // flat and without the empty '' of conditional parts, as h() does: append() would write a list as «[object …],…»
  const content = [h('div', { class: 'modal-head' }, h('h2', {}, title), info ? infoBtn(title, info) : '',
    h('button', { type: 'button', class: 'ghost x', 'aria-label': t('common.close'), onclick: () => dlg.close() }, '×')), msg, ...kids.flat(3)]
    .filter((x) => x !== '' && x != null && x !== false);
  if (old) {
    const top = old.scrollTop;
    const focus = focusKey(old);
    old.replaceChildren(...content);
    old.setAttribute('aria-label', title);
    old.scrollTop = top;
    refocus(old, focus);
    return old;
  }
  dlg.append(...content);
  dlg.addEventListener('close', () => dlg.remove());
  document.body.append(dlg);
  dlg.showModal();
  return dlg;
}

// Where the focus is, as something the same control in a redrawn copy also has, so the focus can be put back there.
export function focusKey(root) {
  const a = document.activeElement;
  if (!a || a === document.body || !root.contains(a)) return null;
  const label = a.getAttribute('aria-label') || a.textContent;
  return { tag: a.tagName, label, index: [...root.querySelectorAll(a.tagName)].filter((x) => (x.getAttribute('aria-label') || x.textContent) === label).indexOf(a) };
}
export function refocus(root, k) {
  if (!k) return;
  const same = [...root.querySelectorAll(k.tag)].filter((x) => (x.getAttribute('aria-label') || x.textContent) === k.label);
  const el = same[k.index] || same[0];
  if (el) el.focus({ preventScroll: true });
}
// The bottom line of a dialog: something destructive on the left, the buttons on the right.
export const foot = (left, ...right) => h('div', { class: 'modal-foot' }, h('div', {}, left || ''), h('div', { class: 'row-inline' }, ...right));
export const closeBtn = () => h('button', { type: 'button', class: 'ghost', onclick: (e) => e.currentTarget.closest('dialog').close() }, t('common.close'));
export const field = (text, input) => h('label', { class: 'field' }, h('span', {}, text), input);

// A small form: a name and «+».
export function addRow(placeholder, onsubmit) {
  const input = h('input', { type: 'text', placeholder, maxlength: '80', required: true, 'aria-label': placeholder });
  return h('form', { class: 'addrow', onsubmit: async (e) => {
    e.preventDefault();
    if (await write(() => onsubmit(input.value))) render();
  } }, h('span', { class: 'plus', 'aria-hidden': 'true' }, '+'), input, h('button', { type: 'submit', 'aria-label': placeholder }, t('common.add')));
}

// Enter in a field of the table moves on to the same field of the next event (the change is saved when the field is left).
export function enterMovesOn(input) {
  input.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    const td = input.closest('td');
    const next = td && td.nextElementSibling && td.nextElementSibling.querySelector('input');
    input.blur();
    if (next) next.focus();
  });
  return input;
}
