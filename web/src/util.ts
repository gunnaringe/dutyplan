import { locale } from './core';

export const enc = encodeURIComponent;
// Dates are formatted with one Intl.DateTimeFormat per language and shape, and each date once: toLocaleDateString builds
// a new formatter every call, and the table names a date in every cell (it was over half of a redraw with 50 people).
export const formats = new Map();
export const formatted = (shape, opts, value, date) => {
  const k = locale() + ' ' + shape;
  let f = formats.get(k);
  if (!f) { f = { fmt: new Intl.DateTimeFormat(locale(), opts), memo: new Map() }; formats.set(k, f); }
  let out = f.memo.get(value);
  if (out === undefined) { out = f.fmt.format(date(value)); f.memo.set(value, out); }
  return out;
};
export const utcDay = (iso) => new Date(iso + 'T00:00:00Z');
export const fmtTime = (iso) => formatted('time', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }, iso, (x) => new Date(x));
export const fmtDate = (iso) => formatted('date', { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' }, iso, utcDay);
export const fmtShortDate = (iso) => formatted('short', { weekday: 'short', day: 'numeric', month: 'short', timeZone: 'UTC' }, iso, utcDay);
export const addDays = (iso, n) => { const d = new Date(iso + 'T00:00:00Z'); d.setUTCDate(d.getUTCDate() + n); return d.toISOString().slice(0, 10); };
// The dates «every N weeks until» (the server makes the same, store.repeatEvery): from each start date, every 7·every
// days up to and including until, without repeats. every 0 is no repeat.
export const MAX_EVERY = 8; // store.maxEveryWeeks
export function repeatDates(dates, until, every) {
  const out = new Set();
  for (const d of dates.filter(Boolean)) {
    if (!until || !every) { out.add(d); continue; }
    for (let x = d; x <= until && out.size <= 400; x = addDays(x, 7 * every)) out.add(x);
  }
  return [...out];
}

// The time of day of an event: «11:00» or «11:00–12:30», or '' when it has none. A 24-hour clock in both languages.
export const clock = (ev) => (ev.start_time ? (ev.end_time ? `${ev.start_time}–${ev.end_time}` : ev.start_time) : '');
