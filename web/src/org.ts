import { api, fetched, get, write } from './api';
import { h, icon, locale, say, store } from './core';
import { render } from './main';
import { ME, deadLink, go, initials, link } from './shell';
import { closeBtn, field, foot, infoBtn, modal, setRefreshKey } from './ui';
import { enc } from './util';

// ---- the start page and the organisation ----------------------------------------------------------------------------

export async function pageHome() {
  const orgs = await get('/orgs');
  const last = store.get('org');
  if (last && !location.search.includes('switch') && !location.search.includes('link=') && orgs.some((o) => o.slug === last)) return go('/o/' + enc(last));
  const name = h('input', { type: 'text', placeholder: t('home.name_placeholder'), maxlength: '80', required: true, 'aria-label': t('home.name_placeholder') });
  return h('div', {},
    deadLink(),
    h('h1', {}, t('home.title')),
    orgs.length ? h('div', { class: 'org-cards' }, orgs.map((o) => {
      const a = link('/o/' + enc(o.slug), '', { class: 'org-card' });
      a.append(h('span', { class: 'org-tile', 'aria-hidden': 'true' }, initials(o.name)), h('span', { class: 'nm' }, o.name), icon('chevron'));
      return a;
    })) : h('p', { class: 'muted' }, t('home.none')),
    ME && ME.login && !ME.user ? '' : h('h2', {}, t('home.new')),
    ME && ME.login && !ME.user ? '' : h('form', { class: 'row new-org', onsubmit: async (e) => {
      e.preventDefault();
      let o;
      if (await write(async () => { o = await api('POST', '/orgs', { name: name.value }); })) { store.set('org', o.slug); go('/o/' + enc(o.slug)); }
    } }, name, h('button', { type: 'submit' }, t('common.create'))),
    ME && ME.login ? '' : h('p', { class: 'muted small' }, t('home.warning')));
}

// The organisation: its plans, and rename or delete. Tasks and people are added from a plan's table; they belong to the
// organisation, so every plan has them.
export async function pageOrg(r, org) {
  const base = `/orgs/${enc(r.org)}`;
  if (new URLSearchParams(location.search).has('joined')) {  // just joined through an invitation (/s/<token>)
    history.replaceState({}, '', location.pathname);
    setTimeout(() => say(t('invite.joined', { org: org.name })), 0);
  }
  if (org.plan) return go(`/o/${enc(r.org)}/p/${enc(org.plan)}`);  // a link to one plan: that plan is all there is
  const plans = await get(base + '/plans');

  function newPlanForm() {
    const name = h('input', { type: 'text', placeholder: t('org.new_plan_placeholder'), maxlength: '80', required: true, 'aria-label': t('org.new_plan_placeholder') });
    return h('form', { class: 'row', onsubmit: async (e) => {
      e.preventDefault();
      let p;
      if (await write(async () => { p = await api('POST', base + '/plans', { name: name.value }); })) go(`/o/${enc(r.org)}/p/${enc(p.slug)}`);
    } }, name, h('button', { type: 'submit' }, t('common.create')));
  }
  const root = h('div', { class: 'org-page' },
    h('h1', {}, org.name),
    h('h2', {}, t('org.plans')),
    h('div', { class: 'plans' },
      plans.length ? plans.map((p) => h('div', { class: 'plan-item' }, h('span', { class: 'nm' }, p.name), link(`/o/${enc(r.org)}/p/${enc(p.slug)}`, t('org.open'), { class: 'button' })))
        : h('p', { class: 'muted' }, t('org.no_plans')),
      org.level === 'view' ? '' : newPlanForm()),
    viewOnly(org));
  return root;
}

// The organisation's settings (admins): rename (the address stays) or delete. Deleting needs the organisation to have no
// plans, and its name typed in.
export async function pageSettings(r, org) {
  const base = `/orgs/${enc(r.org)}`;
  if (org.role !== 'admin') return go('/o/' + enc(r.org));
  const plans = await get(base + '/plans');
  {
    const name = h('input', { type: 'text', value: org.name, maxlength: '80', 'aria-label': t('org.rename_label') });
    let del;
    if (plans.length) del = h('p', { class: 'muted small' }, t('org.delete_has_plans'));
    else {
      const typed = h('input', { type: 'text', placeholder: t('org.delete_type'), 'aria-label': t('org.delete_type'), autocomplete: 'off', maxlength: '80' });
      const button = h('button', { type: 'submit', class: 'danger', disabled: true }, t('org.delete_button'));
      typed.addEventListener('input', () => { button.disabled = typed.value.trim() !== org.name; });
      del = h('form', { class: 'stack', onsubmit: async (e) => {
        e.preventDefault();
        if (typed.value.trim() !== org.name) return;
        if (await write(() => api('DELETE', `${base}?confirm=${enc(r.org)}`))) { store.set('org', ''); go('/?switch'); say(t('org.deleted', { name: org.name })); }
      } }, h('p', { class: 'muted small' }, t('org.delete_hint')), h('div', { class: 'row-inline' }, typed, button));
    }
    return h('div', { class: 'org-page settings-page' },
      h('h1', {}, t('settings.title')),
      h('h2', {}, t('org.rename_label')),
      h('form', { class: 'row', onsubmit: async (e) => {
        e.preventDefault();
        if (await write(() => api('PATCH', base, { name: name.value }))) { await render(); say(t('org.renamed')); }
      } }, name, h('button', { type: 'submit' }, t('common.rename'))),
      h('h2', {}, t('org.delete_title')), del);
  }
}

// ---- sharing: links and members ------------------------------------------------------------------------------------
// A link to one plan lets whoever opens it in (no account needed): view or edit. An invitation (join) makes whoever opens
// it, signed in, a member. Members share plans; admins change members and invite. The page asks the server what this
// person may do (org.role, org.level, org.plan from GET /orgs/{org}); someone with a link never makes links.

export const ROLE = { admin: 'members.role_admin', member: 'members.role_member' };
export const viewOnly = (org) => (org.level === 'view' ? h('p', { class: 'view-only' }, icon('info'), ' ', t('share.view_only')) : '');
export const linkURL = (l) => location.origin + '/s/' + l.token;
// redraws an open dialog in place (modal() fills the dialog whose key is refreshKey)
export async function redraw(dlg, build) {
  const key = 'again:' + Math.random();
  dlg.dataset.key = key;
  try { await build(key); } finally { delete dlg.dataset.key; }
}

// A round button with an icon; its name is its title and its label for screen readers.
export const iconBtn = (ic, label, onclick, cls = '') => h('button', { type: 'button', class: 'iconbtn ' + cls, title: label, 'aria-label': label, onclick }, icon(ic));

// «Are you sure?» in a dialog of the page's own: resolves true when the person says yes.
export function ask(title, text, yes) {
  return new Promise((resolve) => {
    let ok = false;
    const dlg = modal(title, h('p', {}, text),
      foot('', h('button', { type: 'button', class: 'ghost', onclick: () => dlg.close() }, t('common.cancel')),
        h('button', { type: 'button', class: 'danger solid yes', onclick: () => { ok = true; dlg.close(); } }, yes)));
    dlg.classList.add('ask');
    dlg.addEventListener('close', () => resolve(ok));
  });
}

export async function copyLink(l, button) {
  const join = l.level === 'join';
  try {
    await navigator.clipboard.writeText(linkURL(l));
    button.classList.add('done');
    button.replaceChildren(icon('check'));
    setTimeout(() => { button.classList.remove('done'); button.replaceChildren(icon('copy')); }, 1500);
    say(t(join ? 'members.invite_copied' : 'share.copied'));
  } catch (e) {
    // no clipboard (an old browser, or not https): the link selected in a dialog, to copy by hand
    const input = h('input', { type: 'text', class: 'wide', readonly: true, value: linkURL(l), 'aria-label': t('share.copy_prompt'), onfocus: (ev) => ev.target.select() });
    modal(t('share.copy_prompt'), input, foot('', closeBtn())).classList.add('copy-by-hand');
    input.focus();
  }
}

export const LINK_ICON = { view: 'eye', answer: 'check', edit: 'hand', join: 'join' };
export const LINK_ACCESS = { view: 'share.access_view', answer: 'share.access_answer', edit: 'share.access_edit', join: 'share.access_join' };
// A link: what it is (the plan and its label, or whom an invitation is for) on top, what it gives and how long under it,
// then share (where the device can), copy and delete.
export function linkRow(l, base, again, withPlan?) {
  const gone = l.expires_at && new Date(l.expires_at) < new Date();
  const join = l.level === 'join';
  const name = join ? (l.label ? t('members.invite_for', { name: l.label }) : t('members.invite_link')) : l.label || t('share.no_label');
  const share = !gone && navigator.share ? iconBtn('share', t('share.share'), () => navigator.share({ title: name, url: linkURL(l) }).catch(() => {})) : '';
  return h('li', { class: 'link-row' + (gone ? ' gone' : '') },
    h('span', { class: 'link-ic ' + l.level, 'aria-hidden': 'true' }, icon(LINK_ICON[l.level])),
    h('span', { class: 'what' },
      h('span', { class: 'title' }, withPlan && l.plan_name ? h('strong', {}, l.plan_name) : '', withPlan && l.plan_name ? ' · ' : '',
        h('span', { class: l.label || l.level === 'join' ? '' : 'muted' }, name)),
      h('span', { class: 'meta muted small' }, h('span', { class: 'lvl ' + l.level }, t(LINK_ACCESS[l.level])), ' · ',
        join ? [h('span', { class: 'uses' }, t(l.single_use ? 'members.invite_once_meta' : 'members.invite_many_meta')), ' · '] : '',
        gone ? t('share.ran_out') : l.expires_at ? t('share.until', { date: new Date(l.expires_at).toLocaleDateString(locale()) }) : t('share.forever'),
        l.created_by ? ' · ' + t('share.by', { name: l.created_by }) : '')),
    h('span', { class: 'acts' }, share,
      gone ? '' : iconBtn('copy', t('share.copy'), (e) => copyLink(l, e.currentTarget)),
      iconBtn('trash', t(join ? 'members.invite_delete_aria' : 'share.delete_aria', { name }), async () => {
        if (!(await ask(t(join ? 'members.invite_delete_title' : 'share.delete_title'), t(join ? 'share.delete_join' : 'share.delete_text', { name }), t('common.remove')))) return;
        if (await write(() => api('DELETE', `${base}/links/${l.id}`))) again();
      }, 'danger')));
}

export const expirySelect = (dflt) => h('select', { 'aria-label': t('share.expires') },
  [[0, 'share.never'], [7, 'share.days_7'], [30, 'share.days_30'], [90, 'share.days_90']].map(([d, k]) => h('option', { value: String(d), selected: d === dflt }, t(k as string))));

// Share one plan: its links, and a new one.
export async function shareDialog(orgSlug, org, plan) {
  const base = `/orgs/${enc(orgSlug)}`;
  let dlg;
  const build = async (key?) => {
    const all = await api('GET', base + '/links');
    const mine = all.filter((l) => l.level !== 'join' && l.plan === plan.slug);
    const level = h('select', { 'aria-label': t('share.level') }, ['view', 'answer', 'edit'].map((x) => h('option', { value: x }, t(LINK_ACCESS[x]))));
    const label = h('input', { type: 'text', maxlength: '80', placeholder: t('share.label_placeholder'), 'aria-label': t('share.label') });
    const expires = expirySelect(0);
    const title = t('share.title_plan', { name: plan.name });
    if (key) setRefreshKey(key);
    dlg = modal(title, { info: [t('share.info'), t('share.info_levels')] },
      mine.length ? h('ul', { class: 'plain links' }, mine.map((l) => linkRow(l, base, () => redraw(dlg, build)))) : h('p', { class: 'muted' }, t('share.none')),
      h('h3', {}, t('share.new')),
      h('form', { class: 'new-link', onsubmit: async (e) => {
        e.preventDefault();
        let made;
        if (await write(async () => { made = await api('POST', base + '/links', { plan: plan.slug, level: level.value, label: label.value, expires_days: Number(expires.value) }); })) {
          redraw(dlg, build);
          try { await navigator.clipboard.writeText(linkURL(made)); say(t('share.made_copied')); } catch (err) { say(t('share.made')); }
        }
      } }, field(t('share.label'), label), field(t('share.level'), level), field(t('share.expires'), expires), h('button', { type: 'submit' }, t('share.create'))),
      foot('', closeBtn()));
  };
  await build();
}

// The members page: who is in the organisation, how to invite more, and the links that share its plans (made under
// «Share» on each plan; listed here so they can be seen and deleted in one place). An admin changes roles and invites;
// everyone may leave. Leaving, removing someone and changing one's own role are asked about first.
export async function pageMembers(r, org) {
  const base = `/orgs/${enc(r.org)}`;
  if (!org.role) return go('/o/' + enc(r.org));
  const admin = org.role === 'admin';
  const [members, links] = await Promise.all([api('GET', base + '/members'), api('GET', base + '/links')]);
  const again = () => render();
  const invites = links.filter((l) => l.level === 'join');
  const shared = links.filter((l) => l.level !== 'join');
  const leave = async (m) => {
    const yes = m.you ? await ask(t('members.leave_title'), t('members.leave_confirm', { name: org.name }), t('members.leave'))
      : await ask(t('members.remove_title'), t('members.remove_confirm', { name: m.name }), t('members.remove'));
    if (!yes) return;
    if (await write(() => api('DELETE', `${base}/members/${m.id}`))) { if (m.you) { store.set('org', ''); fetched.clear(); go('/?switch'); } else again(); }
  };
  const row = (m) => h('li', { class: 'member' },
    h('span', { class: 'avatar sm', 'aria-hidden': 'true' }, initials(m.name)),
    h('span', { class: 'what' }, h('span', { class: 'title' }, m.name, m.you ? h('span', { class: 'muted small' }, ' ' + t('members.you')) : ''),
      m.email ? h('span', { class: 'meta muted small' }, m.email) : ''),
    h('span', { class: 'acts' },
      admin ? h('select', { 'aria-label': t('members.role_aria', { name: m.name }), onchange: async (e) => {
        const to = e.target.value;
        if (m.you && !(await ask(t('members.own_role_title'), t('members.own_role_confirm', { role: t(ROLE[to]).toLowerCase() }), t('members.own_role_yes')))) { e.target.value = m.role; return; }
        if (await write(() => api('PATCH', `${base}/members/${m.id}`, { role: to }))) again(); else e.target.value = m.role;
      } }, ['admin', 'member'].map((x) => h('option', { value: x, selected: x === m.role }, t(ROLE[x])))) : h('span', { class: 'muted small' }, t(ROLE[m.role])),
      admin || m.you ? iconBtn(m.you ? 'logout' : 'trash', m.you ? t('members.leave') : t('members.remove_aria', { name: m.name }), () => leave(m), 'danger') : ''));
  const expires = expirySelect(7);
  // an invitation is for a person: whom it is for, as its label (a name, or a role), so the list says who has not joined yet
  const who = h('input', { type: 'text', maxlength: '80', placeholder: t('members.invite_who_placeholder') });
  // for one person (ends when someone joins with it), or for anyone who has it until it runs out
  const uses = h('select', { 'aria-label': t('members.invite_uses') },
    h('option', { value: 'once', selected: true }, t('members.invite_once')), h('option', { value: 'many' }, t('members.invite_many')));
  return h('div', { class: 'org-page members-page' },
    h('h1', {}, t('nav.members'), infoBtn(t('nav.members'), [t('members.info')])),
    h('ul', { class: 'plain members' }, members.map(row)),
    admin ? [h('h2', {}, t('members.invite')),
      h('p', { class: 'muted small lead' }, t('members.invite_hint')),
      invites.length ? h('ul', { class: 'plain links' }, invites.map((l) => linkRow(l, base, again))) : '',
      h('form', { class: 'new-link', onsubmit: async (e) => {
        e.preventDefault();
        let made;
        if (await write(async () => { made = await api('POST', base + '/links', { level: 'join', label: who.value.trim(), single_use: uses.value === 'once', expires_days: Number(expires.value) }); })) {
          again();
          try { await navigator.clipboard.writeText(linkURL(made)); say(t('members.invite_made_copied')); } catch (err) { say(t('members.invite_made')); }
        }
      } }, field(t('members.invite_who'), who), field(t('members.invite_uses'), uses), field(t('share.expires'), expires), h('button', { type: 'submit' }, t('members.invite_create')))] : '',
    h('h2', {}, t('members.plan_links')),
    h('p', { class: 'muted small lead' }, t('members.plan_links_hint')),
    shared.length ? h('ul', { class: 'plain links' }, shared.map((l) => linkRow(l, base, again, true))) : h('p', { class: 'muted' }, t('share.none')));
}
