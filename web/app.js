// Generated from web/src (TypeScript) by esbuild: mise run web:build. Do not edit.
"use strict";
(() => {
  // web/src/core.ts
  var store = {
    get: (k) => {
      try {
        return localStorage.getItem("tp." + k) || "";
      } catch (e) {
        return "";
      }
    },
    set: (k, v) => {
      try {
        localStorage.setItem("tp." + k, v);
      } catch (e) {
      }
    }
  };
  var CLIENT = Math.random().toString(36).slice(2) + Date.now().toString(36);
  function h(tag, props, ...kids) {
    const el = document.createElement(tag);
    for (const [k, v] of Object.entries(props || {})) {
      if (v === false || v == null) continue;
      if (k === "class") el.className = v;
      else if (k === "checked" || k === "value" || k === "disabled") el[k] = v;
      else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
      else el.setAttribute(k, v === true ? "" : v);
    }
    for (const kid of kids.flat(2)) el.append(kid instanceof Node ? kid : document.createTextNode(String(kid ?? "")));
    return el;
  }
  var ICONS = {
    up: '<path d="M14 9V5a3 3 0 0 0-3-3l-4 9v11h11.28a2 2 0 0 0 2-1.7l1.38-9a2 2 0 0 0-2-2.3zM7 22H4a2 2 0 0 1-2-2v-7a2 2 0 0 1 2-2h3"/>',
    down: '<path d="M10 15v4a3 3 0 0 0 3 3l4-9V2H5.72a2 2 0 0 0-2 1.7l-1.38 9a2 2 0 0 0 2 2.3zm7-13h2.67A2.31 2.31 0 0 1 22 4v7a2.31 2.31 0 0 1-2.33 2H17"/>',
    maybe: '<path d="M8.5 8.5a3.5 3.5 0 1 1 5.2 3.05c-1.1.62-1.7 1.3-1.7 2.45v.5M12 19h.01" stroke-width="3"/>',
    auto: '<path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/>',
    info: '<circle cx="12" cy="12" r="10"/><path d="M12 16v-4M12 8h.01"/>',
    share: '<circle cx="18" cy="5" r="3"/><circle cx="6" cy="12" r="3"/><circle cx="18" cy="19" r="3"/><path d="M8.59 13.51l6.83 3.98M15.41 6.51l-6.82 3.98"/>',
    left: '<path d="M15 18l-6-6 6-6"/>',
    right: '<path d="M9 18l6-6-6-6"/>',
    lock: '<rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/>',
    unlock: '<rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 7.75-1.4"/>',
    more: '<circle cx="5" cy="12" r="1.4" fill="currentColor"/><circle cx="12" cy="12" r="1.4" fill="currentColor"/><circle cx="19" cy="12" r="1.4" fill="currentColor"/>',
    filter: '<path d="M22 3H2l8 9.46V19l4 2v-8.54z"/>',
    hand: '<path d="M12 20h9"/><path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4z"/>',
    users: '<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/>',
    settings: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.6 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.6a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/>',
    logout: '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><path d="M16 17l5-5-5-5M21 12H9"/>',
    check: '<path d="M20 6L9 17l-5-5"/>',
    caret: '<path d="M6 9l6 6 6-6"/>',
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
    chevron: '<path d="M9 18l6-6-6-6"/>'
  };
  var mark = (kind) => h("span", { class: "mark " + kind, title: t(kind === "lock" ? "draft.key_locked" : "draft.key_hand"), "aria-label": t(kind === "lock" ? "draft.key_locked" : "draft.key_hand"), role: "img" }, icon(kind));
  function icon(name) {
    const s = h("span", { class: "ic ic-" + name, "aria-hidden": "true" });
    s.innerHTML = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${ICONS[name]}</svg>`;
    return s;
  }
  var LEVEL_ICON = { never: "down", "": "down", cannot: "down", poor: "maybe", standard: "maybe", good: "up", practical: "up" };
  var sayTimer = 0;
  function say(text, kind) {
    const m = document.querySelector("dialog[open] [data-msg]") || document.getElementById("msg");
    m.textContent = text || "";
    m.className = text ? kind || "ok" : "";
    if (m.id !== "msg") return;
    clearTimeout(sayTimer);
    m.onclick = () => say("");
    if (text) sayTimer = setTimeout(() => {
      m.textContent = "";
      m.className = "";
    }, kind === "err" ? 8e3 : 4e3);
  }
  var cap = (s) => s.charAt(0).toUpperCase() + s.slice(1);
  var locale = () => currentLang === "nb" ? "nb-NO" : "en-GB";

  // web/src/util.ts
  var enc = encodeURIComponent;
  var formats = /* @__PURE__ */ new Map();
  var formatted = (shape, opts, value, date) => {
    const k = locale() + " " + shape;
    let f = formats.get(k);
    if (!f) {
      f = { fmt: new Intl.DateTimeFormat(locale(), opts), memo: /* @__PURE__ */ new Map() };
      formats.set(k, f);
    }
    let out = f.memo.get(value);
    if (out === void 0) {
      out = f.fmt.format(date(value));
      f.memo.set(value, out);
    }
    return out;
  };
  var utcDay = (iso) => /* @__PURE__ */ new Date(iso + "T00:00:00Z");
  var fmtTime = (iso) => formatted("time", { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" }, iso, (x) => new Date(x));
  var fmtDate = (iso) => formatted("date", { weekday: "short", day: "numeric", month: "short", year: "numeric", timeZone: "UTC" }, iso, utcDay);
  var fmtShortDate = (iso) => formatted("short", { weekday: "short", day: "numeric", month: "short", timeZone: "UTC" }, iso, utcDay);
  var addDays = (iso, n) => {
    const d = /* @__PURE__ */ new Date(iso + "T00:00:00Z");
    d.setUTCDate(d.getUTCDate() + n);
    return d.toISOString().slice(0, 10);
  };
  var MAX_EVERY = 8;
  function repeatDates(dates, until, every) {
    const out = /* @__PURE__ */ new Set();
    for (const d of dates.filter(Boolean)) {
      if (!until || !every) {
        out.add(d);
        continue;
      }
      for (let x = d; x <= until && out.size <= 400; x = addDays(x, 7 * every)) out.add(x);
    }
    return [...out];
  }
  var clock = (ev) => ev.start_time ? ev.end_time ? `${ev.start_time}–${ev.end_time}` : ev.start_time : "";

  // web/src/shell.ts
  function go(path) {
    history.pushState({}, "", path);
    render();
  }
  function link(path, text, props) {
    return h("a", { href: path, ...props, onclick: (e) => {
      if (e.metaKey || e.ctrlKey || e.shiftKey) return;
      e.preventDefault();
      go(path);
    } }, text);
  }
  var linkBtn = (text, onclick, aria) => h("button", { type: "button", class: "link-button", "aria-label": aria, onclick }, text);
  var ME = null;
  async function me() {
    if (!ME) ME = await api("GET", "/me");
    return ME;
  }
  function signIn() {
    if (ME && !ME.login) return location.reload();
    location.href = "/auth/login?next=" + encodeURIComponent(location.pathname + location.search);
  }
  async function signOut() {
    let to = "/";
    try {
      const res = await fetch("/auth/logout", { method: "POST" });
      to = (await res.json()).to || "/";
    } catch (e) {
    }
    store.set("org", "");
    location.href = to;
  }
  function deadLink() {
    return new URLSearchParams(location.search).get("link") === "invalid" ? h("p", { class: "notice", role: "status" }, t("link.invalid")) : "";
  }
  function invitedBox() {
    const q = new URLSearchParams(location.search);
    const token = q.get("invite");
    if (!token) return "";
    return h(
      "div",
      { class: "invited", role: "status" },
      icon("join"),
      h(
        "div",
        {},
        h("h2", {}, t("invite.title", { org: q.get("org") || "" })),
        h("p", {}, t(ME.signup ? "invite.text_signup" : "invite.text")),
        h(
          "p",
          { class: "row-inline" },
          h("a", { class: "button solid", href: "/auth/login?next=" + encodeURIComponent("/s/" + token) }, t("invite.sign_in")),
          ME.signup ? h("a", { class: "button", href: ME.signup }, t("landing.sign_up")) : ""
        )
      )
    );
  }
  function pageLanding() {
    const point = ([k, ic]) => h("li", {}, h("span", { class: "point-ic" }, icon(ic)), h("h3", {}, t("landing." + k + "_title")), h("p", {}, t("landing." + k)));
    const step = ([title, text], i) => h("li", {}, h("span", { class: "step-n" }, String(i + 1)), h("h3", {}, t(title)), h("p", {}, t(text)));
    const still = typeof window.matchMedia === "function" && matchMedia("(prefers-reduced-motion: reduce)").matches;
    const film = h("video", {
      class: "demo",
      src: "/demo-" + currentLang + ".mp4",
      poster: "/landing.png",
      width: "1024",
      height: "576",
      controls: true,
      loop: true,
      playsinline: true,
      preload: still ? "none" : "auto",
      autoplay: !still,
      "aria-label": t("landing.video")
    });
    film.muted = true;
    return h(
      "div",
      { class: "landing" },
      deadLink(),
      invitedBox(),
      h(
        "section",
        { class: "hero" },
        h("img", { class: "logo", src: "/logo.svg", alt: "", width: "56", height: "56" }),
        h("h1", {}, t("landing.title")),
        h("p", { class: "lead" }, t("landing.lead")),
        h(
          "p",
          { class: "row-inline cta" },
          h("button", { type: "button", onclick: signIn }, t("landing.sign_in")),
          ME.signup ? h("a", { class: "button ghost", href: ME.signup }, t("landing.sign_up")) : ""
        ),
        h("p", { class: "muted small" }, t(ME.signup ? "landing.account_signup" : "landing.account")),
        h("p", { class: "free" }, t("landing.free"))
      ),
      h("figure", { class: "demo" }, film, h("figcaption", { class: "muted small" }, t("landing.video_caption"))),
      h("section", { class: "how" }, h("h2", {}, t("landing.how")), h("ol", { class: "steps" }, [
        ["landing.step_dates_title", "landing.step_dates"],
        ["landing.step_answers_title", "landing.step_answers"],
        ["landing.step_draft_title", "landing.step_draft"],
        ["landing.step_share_title", "landing.step_share"]
      ].map(step))),
      h("ul", { class: "points" }, [["tap", "up"], ["fair", "check"], ["live", "users"], ["share", "share"]].map(point)),
      about()
    );
  }
  var CONTACT = "contact@sermon-studio.app";
  var REPO = "https://github.com/gunnaringe/dutyplan";
  function withLinks(key, nodes) {
    return t(key).split(/\{(\w+)\}/).map((part, i) => i % 2 ? nodes[part] : part);
  }
  function about() {
    const ext = (href, ic, key, title) => h("li", {}, h("a", { href, title: title ? t(title) : null }, icon(ic), h("span", {}, t(key))));
    return h(
      "footer",
      { class: "about", id: "about" },
      h("h2", {}, t("about.title")),
      h("p", {}, t("about.why")),
      h("p", {}, t("about.free")),
      h("p", {}, withLinks("about.contact", { email: h("a", { href: "mailto:" + CONTACT }, CONTACT), github: h("a", { href: REPO + "/issues" }, "GitHub") })),
      h("p", {}, h("a", { class: "signature", href: "https://gunnaringe.sort.land", title: "gunnaringe.sort.land" }, "Gunnar Inge Gjøvik Sortland")),
      h(
        "ul",
        { class: "about-links" },
        ext(REPO, "code", "about.source", "about.source_title"),
        ext(REPO + "/issues", "message", "about.issues", "about.issues_title"),
        ext(REPO + "/blob/main/LICENSE", "file", "about.licence")
      )
    );
  }
  function brand(to) {
    const a = link(to, "", { class: "brand" });
    a.append(h("img", { src: "/logo.svg", alt: "", width: "28", height: "28" }), h("span", {}, t("brand")));
    return a;
  }
  function headerSignedOut() {
    document.getElementById("top").replaceChildren(
      brand("/"),
      h("span", { class: "spacer" }),
      langMenu(),
      h("button", { type: "button", class: "sm", onclick: signIn }, t("landing.sign_in"))
    );
  }
  function route() {
    const parts = location.pathname.split("/").filter(Boolean);
    if (parts[0] === "o" && parts[1]) {
      if (parts.length === 2) return { page: "org", org: parts[1] };
      if (parts.length === 3 && (parts[2] === "members" || parts[2] === "settings")) return { page: parts[2], org: parts[1] };
      if (parts.length === 4 && parts[2] === "p") return { page: "plan", org: parts[1], plan: parts[3] };
      return { page: "none", org: parts[1] };
    }
    return { page: parts.length ? "none" : "home" };
  }
  async function renderHeader(r) {
    const orgs = await get("/orgs");
    const select = h(
      "select",
      { "aria-label": t("header.org"), onchange: (e) => go(e.target.value === "" ? "/?switch" : "/o/" + enc(e.target.value)) },
      h("option", { value: "" }, r.org ? t("header.switch_org") : t("header.choose_org")),
      orgs.map((o) => h("option", { value: o.slug, selected: o.slug === r.org }, o.name))
    );
    const who = ME && ME.user;
    const org = r.org ? await get("/orgs/" + enc(r.org)).catch(() => null) : null;
    const role = org ? org.role || "" : "";
    const nav = (page, ic, label) => {
      const a = link(`/o/${enc(r.org)}/${page}`, "", { class: "icon-btn" + (r.page === page ? " on" : ""), title: label, "aria-label": label });
      a.append(icon(ic));
      return a;
    };
    const tools = [];
    if (r.org && role && ME && ME.login) tools.push(nav("members", "users", t("nav.members")));
    if (r.org && role === "admin") tools.push(nav("settings", "settings", t("nav.settings")));
    document.getElementById("top").replaceChildren(
      brand("/?switch"),
      orgs.length && r.page !== "home" ? h("label", { class: "org-pick" }, h("span", { class: "lbl" }, t("header.org")), select) : "",
      h("span", { class: "spacer" }),
      ...tools,
      langMenu(),
      who ? userMenu(who) : ME && ME.login ? h("button", { type: "button", class: "sm", onclick: signIn }, t("landing.sign_in")) : ""
    );
  }
  function dropdown(cls, button, label, ...panel) {
    const d = h("details", { class: "dropdown " + cls });
    const close = (e) => {
      if (!d.isConnected) return off();
      if (e.type === "keydown" ? e.key === "Escape" : !d.contains(e.target)) {
        d.open = false;
        if (e.type === "keydown") d.querySelector("summary").focus();
      }
    };
    const off = () => {
      document.removeEventListener("pointerdown", close);
      document.removeEventListener("keydown", close);
    };
    d.addEventListener("toggle", () => {
      off();
      if (d.open) {
        document.addEventListener("pointerdown", close);
        document.addEventListener("keydown", close);
      }
    });
    d.append(h("summary", { title: label, "aria-label": label }, button), h("div", { class: "drop-panel" }, ...panel));
    return d;
  }
  function langMenu() {
    return dropdown(
      "lang",
      [h("span", { class: "code" }, currentLang.toUpperCase()), icon("caret")],
      t("lang.label") + ": " + t("lang." + currentLang),
      LANGS.map((l) => h("button", {
        type: "button",
        class: "drop-item" + (l === currentLang ? " on" : ""),
        lang: l,
        onclick: () => {
          setLang(l);
          fetched.clear();
          render();
        }
      }, h("span", {}, t("lang." + l)), l === currentLang ? icon("check") : ""))
    );
  }
  function initials(name) {
    const w = String(name || "?").trim().split(/\s+/).filter(Boolean);
    return ((w[0] || "?")[0] + (w.length > 1 ? w[w.length - 1][0] : "")).toUpperCase();
  }
  function userMenu(who) {
    return dropdown(
      "user",
      h("span", { class: "avatar" }, initials(who.name)),
      who.name,
      h("div", { class: "who" }, h("strong", {}, who.name), who.email ? h("span", { class: "muted small" }, who.email) : ""),
      h("button", { type: "button", class: "ghost signout", onclick: signOut }, icon("logout"), h("span", {}, t("header.sign_out")))
    );
  }

  // web/src/ui.ts
  var keyed = (dlg, kind, id) => {
    dlg.dataset.key = kind + ":" + id;
    return dlg;
  };
  var refreshKey = null;
  var setRefreshKey = (key) => {
    refreshKey = key;
  };
  function reopened(handlers) {
    for (const dlg of document.querySelectorAll("dialog.modal[open][data-key]")) {
      const [kind, id] = dlg.dataset.key.split(/:(.*)/s);
      refreshKey = dlg.dataset.key;
      if (handlers[kind]) handlers[kind](/^\d+$/.test(id) ? Number(id) : id);
      if (refreshKey) dlg.close();
      refreshKey = null;
    }
  }
  var infoBtn = (title, texts) => h("button", {
    type: "button",
    class: "info",
    title: t("common.info"),
    "aria-label": t("common.info_aria", { name: title }),
    onclick: () => modal(title, h("div", { class: "info-text" }, texts.filter(Boolean).map((x) => h("p", {}, x))), foot("", closeBtn()))
  }, icon("info"));
  function modal(title, ...kids) {
    const info = kids[0] && kids[0].info ? kids.shift().info : null;
    const key = refreshKey;
    refreshKey = null;
    const old = key && [...document.querySelectorAll("dialog.modal[open]")].find((d) => d.dataset.key === key);
    const dlg = old || h("dialog", { class: "modal", "aria-label": title });
    const msg = old ? old.querySelector("[data-msg]") : h("p", { "data-msg": "", role: "status" });
    const content = [h(
      "div",
      { class: "modal-head" },
      h("h2", {}, title),
      info ? infoBtn(title, info) : "",
      h("button", { type: "button", class: "ghost x", "aria-label": t("common.close"), onclick: () => dlg.close() }, "×")
    ), msg, ...kids.flat(3)].filter((x) => x !== "" && x != null && x !== false);
    if (old) {
      const top = old.scrollTop;
      const focus = focusKey(old);
      old.replaceChildren(...content);
      old.setAttribute("aria-label", title);
      old.scrollTop = top;
      refocus(old, focus);
      return old;
    }
    dlg.append(...content);
    dlg.addEventListener("close", () => dlg.remove());
    document.body.append(dlg);
    dlg.showModal();
    return dlg;
  }
  function focusKey(root) {
    const a = document.activeElement;
    if (!a || a === document.body || !root.contains(a)) return null;
    const label = a.getAttribute("aria-label") || a.textContent;
    return { tag: a.tagName, label, index: [...root.querySelectorAll(a.tagName)].filter((x) => (x.getAttribute("aria-label") || x.textContent) === label).indexOf(a) };
  }
  function refocus(root, k) {
    if (!k) return;
    const same = [...root.querySelectorAll(k.tag)].filter((x) => (x.getAttribute("aria-label") || x.textContent) === k.label);
    const el = same[k.index] || same[0];
    if (el) el.focus({ preventScroll: true });
  }
  var foot = (left, ...right) => h("div", { class: "modal-foot" }, h("div", {}, left || ""), h("div", { class: "row-inline" }, ...right));
  var closeBtn = () => h("button", { type: "button", class: "ghost", onclick: (e) => e.currentTarget.closest("dialog").close() }, t("common.close"));
  var field = (text, input) => h("label", { class: "field" }, h("span", {}, text), input);
  function addRow(placeholder, onsubmit) {
    const input = h("input", { type: "text", placeholder, maxlength: "80", required: true, "aria-label": placeholder });
    return h("form", { class: "addrow", onsubmit: async (e) => {
      e.preventDefault();
      if (await write(() => onsubmit(input.value))) render();
    } }, h("span", { class: "plus", "aria-hidden": "true" }, "+"), input, h("button", { type: "submit", "aria-label": placeholder }, t("common.add")));
  }
  function enterMovesOn(input) {
    input.addEventListener("keydown", (e) => {
      if (e.key !== "Enter") return;
      e.preventDefault();
      const td = input.closest("td");
      const next = td && td.nextElementSibling && td.nextElementSibling.querySelector("input");
      input.blur();
      if (next) next.focus();
    });
    return input;
  }

  // web/src/org.ts
  async function pageHome() {
    const orgs = await get("/orgs");
    const last = store.get("org");
    if (last && !location.search.includes("switch") && !location.search.includes("link=") && orgs.some((o) => o.slug === last)) return go("/o/" + enc(last));
    const name = h("input", { type: "text", placeholder: t("home.name_placeholder"), maxlength: "80", required: true, "aria-label": t("home.name_placeholder") });
    return h(
      "div",
      {},
      deadLink(),
      h("h1", {}, t("home.title")),
      orgs.length ? h("div", { class: "org-cards" }, orgs.map((o) => {
        const a = link("/o/" + enc(o.slug), "", { class: "org-card" });
        a.append(h("span", { class: "org-tile", "aria-hidden": "true" }, initials(o.name)), h("span", { class: "nm" }, o.name), icon("chevron"));
        return a;
      })) : h("p", { class: "muted" }, t("home.none")),
      ME && ME.login && !ME.user ? "" : h("h2", {}, t("home.new")),
      ME && ME.login && !ME.user ? "" : h("form", { class: "row new-org", onsubmit: async (e) => {
        e.preventDefault();
        let o;
        if (await write(async () => {
          o = await api("POST", "/orgs", { name: name.value });
        })) {
          store.set("org", o.slug);
          go("/o/" + enc(o.slug));
        }
      } }, name, h("button", { type: "submit" }, t("common.create"))),
      ME && ME.login ? "" : h("p", { class: "muted small" }, t("home.warning"))
    );
  }
  async function pageOrg(r, org) {
    const base = `/orgs/${enc(r.org)}`;
    if (new URLSearchParams(location.search).has("joined")) {
      history.replaceState({}, "", location.pathname);
      setTimeout(() => say(t("invite.joined", { org: org.name })), 0);
    }
    if (org.plan) return go(`/o/${enc(r.org)}/p/${enc(org.plan)}`);
    const plans = await get(base + "/plans");
    function newPlanForm() {
      const name = h("input", { type: "text", placeholder: t("org.new_plan_placeholder"), maxlength: "80", required: true, "aria-label": t("org.new_plan_placeholder") });
      return h("form", { class: "row", onsubmit: async (e) => {
        e.preventDefault();
        let p;
        if (await write(async () => {
          p = await api("POST", base + "/plans", { name: name.value });
        })) go(`/o/${enc(r.org)}/p/${enc(p.slug)}`);
      } }, name, h("button", { type: "submit" }, t("common.create")));
    }
    const root = h(
      "div",
      { class: "org-page" },
      h("h1", {}, org.name),
      h("h2", {}, t("org.plans")),
      h(
        "div",
        { class: "plans" },
        plans.length ? plans.map((p) => h("div", { class: "plan-item" }, h("span", { class: "nm" }, p.name), link(`/o/${enc(r.org)}/p/${enc(p.slug)}`, t("org.open"), { class: "button" }))) : h("p", { class: "muted" }, t("org.no_plans")),
        org.level === "view" ? "" : newPlanForm()
      ),
      viewOnly(org)
    );
    return root;
  }
  async function pageSettings(r, org) {
    const base = `/orgs/${enc(r.org)}`;
    if (org.role !== "admin") return go("/o/" + enc(r.org));
    const plans = await get(base + "/plans");
    {
      const name = h("input", { type: "text", value: org.name, maxlength: "80", "aria-label": t("org.rename_label") });
      let del;
      if (plans.length) del = h("p", { class: "muted small" }, t("org.delete_has_plans"));
      else {
        const typed = h("input", { type: "text", placeholder: t("org.delete_type"), "aria-label": t("org.delete_type"), autocomplete: "off", maxlength: "80" });
        const button = h("button", { type: "submit", class: "danger", disabled: true }, t("org.delete_button"));
        typed.addEventListener("input", () => {
          button.disabled = typed.value.trim() !== org.name;
        });
        del = h("form", { class: "stack", onsubmit: async (e) => {
          e.preventDefault();
          if (typed.value.trim() !== org.name) return;
          if (await write(() => api("DELETE", `${base}?confirm=${enc(r.org)}`))) {
            store.set("org", "");
            go("/?switch");
            say(t("org.deleted", { name: org.name }));
          }
        } }, h("p", { class: "muted small" }, t("org.delete_hint")), h("div", { class: "row-inline" }, typed, button));
      }
      return h(
        "div",
        { class: "org-page settings-page" },
        h("h1", {}, t("settings.title")),
        h("h2", {}, t("org.rename_label")),
        h("form", { class: "row", onsubmit: async (e) => {
          e.preventDefault();
          if (await write(() => api("PATCH", base, { name: name.value }))) {
            await render();
            say(t("org.renamed"));
          }
        } }, name, h("button", { type: "submit" }, t("common.rename"))),
        h("h2", {}, t("org.delete_title")),
        del
      );
    }
  }
  var ROLE = { admin: "members.role_admin", member: "members.role_member" };
  var viewOnly = (org) => org.level === "view" ? h("p", { class: "view-only" }, icon("info"), " ", t("share.view_only")) : "";
  var linkURL = (l) => location.origin + "/s/" + l.token;
  async function redraw(dlg, build) {
    const key = "again:" + Math.random();
    dlg.dataset.key = key;
    try {
      await build(key);
    } finally {
      delete dlg.dataset.key;
    }
  }
  var iconBtn = (ic, label, onclick, cls = "") => h("button", { type: "button", class: "iconbtn " + cls, title: label, "aria-label": label, onclick }, icon(ic));
  function ask(title, text, yes) {
    return new Promise((resolve) => {
      let ok = false;
      const dlg = modal(
        title,
        h("p", {}, text),
        foot(
          "",
          h("button", { type: "button", class: "ghost", onclick: () => dlg.close() }, t("common.cancel")),
          h("button", { type: "button", class: "danger solid yes", onclick: () => {
            ok = true;
            dlg.close();
          } }, yes)
        )
      );
      dlg.classList.add("ask");
      dlg.addEventListener("close", () => resolve(ok));
    });
  }
  async function copyLink(l, button) {
    const join = l.level === "join";
    try {
      await navigator.clipboard.writeText(linkURL(l));
      button.classList.add("done");
      button.replaceChildren(icon("check"));
      setTimeout(() => {
        button.classList.remove("done");
        button.replaceChildren(icon("copy"));
      }, 1500);
      say(t(join ? "members.invite_copied" : "share.copied"));
    } catch (e) {
      const input = h("input", { type: "text", class: "wide", readonly: true, value: linkURL(l), "aria-label": t("share.copy_prompt"), onfocus: (ev) => ev.target.select() });
      modal(t("share.copy_prompt"), input, foot("", closeBtn())).classList.add("copy-by-hand");
      input.focus();
    }
  }
  var LINK_ICON = { view: "eye", answer: "check", edit: "hand", join: "join" };
  var LINK_ACCESS = { view: "share.access_view", answer: "share.access_answer", edit: "share.access_edit", join: "share.access_join" };
  function linkRow(l, base, again, withPlan) {
    const gone = l.expires_at && new Date(l.expires_at) < /* @__PURE__ */ new Date();
    const join = l.level === "join";
    const name = join ? l.label ? t("members.invite_for", { name: l.label }) : t("members.invite_link") : l.label || t("share.no_label");
    const share = !gone && navigator.share ? iconBtn("share", t("share.share"), () => navigator.share({ title: name, url: linkURL(l) }).catch(() => {
    })) : "";
    return h(
      "li",
      { class: "link-row" + (gone ? " gone" : "") },
      h("span", { class: "link-ic " + l.level, "aria-hidden": "true" }, icon(LINK_ICON[l.level])),
      h(
        "span",
        { class: "what" },
        h(
          "span",
          { class: "title" },
          withPlan && l.plan_name ? h("strong", {}, l.plan_name) : "",
          withPlan && l.plan_name ? " · " : "",
          h("span", { class: l.label || l.level === "join" ? "" : "muted" }, name)
        ),
        h(
          "span",
          { class: "meta muted small" },
          h("span", { class: "lvl " + l.level }, t(LINK_ACCESS[l.level])),
          " · ",
          join ? [h("span", { class: "uses" }, t(l.single_use ? "members.invite_once_meta" : "members.invite_many_meta")), " · "] : "",
          gone ? t("share.ran_out") : l.expires_at ? t("share.until", { date: new Date(l.expires_at).toLocaleDateString(locale()) }) : t("share.forever"),
          l.created_by ? " · " + t("share.by", { name: l.created_by }) : ""
        )
      ),
      h(
        "span",
        { class: "acts" },
        share,
        gone ? "" : iconBtn("copy", t("share.copy"), (e) => copyLink(l, e.currentTarget)),
        iconBtn("trash", t(join ? "members.invite_delete_aria" : "share.delete_aria", { name }), async () => {
          if (!await ask(t(join ? "members.invite_delete_title" : "share.delete_title"), t(join ? "share.delete_join" : "share.delete_text", { name }), t("common.remove"))) return;
          if (await write(() => api("DELETE", `${base}/links/${l.id}`))) again();
        }, "danger")
      )
    );
  }
  var expirySelect = (dflt) => h(
    "select",
    { "aria-label": t("share.expires") },
    [[0, "share.never"], [7, "share.days_7"], [30, "share.days_30"], [90, "share.days_90"]].map(([d, k]) => h("option", { value: String(d), selected: d === dflt }, t(k)))
  );
  async function shareDialog(orgSlug, org, plan) {
    const base = `/orgs/${enc(orgSlug)}`;
    let dlg;
    const build = async (key) => {
      const all = await api("GET", base + "/links");
      const mine = all.filter((l) => l.level !== "join" && l.plan === plan.slug);
      const level = h("select", { "aria-label": t("share.level") }, ["view", "answer", "edit"].map((x) => h("option", { value: x }, t(LINK_ACCESS[x]))));
      const label = h("input", { type: "text", maxlength: "80", placeholder: t("share.label_placeholder"), "aria-label": t("share.label") });
      const expires = expirySelect(0);
      const title = t("share.title_plan", { name: plan.name });
      if (key) setRefreshKey(key);
      dlg = modal(
        title,
        { info: [t("share.info"), t("share.info_levels")] },
        mine.length ? h("ul", { class: "plain links" }, mine.map((l) => linkRow(l, base, () => redraw(dlg, build)))) : h("p", { class: "muted" }, t("share.none")),
        h("h3", {}, t("share.new")),
        h("form", { class: "new-link", onsubmit: async (e) => {
          e.preventDefault();
          let made;
          if (await write(async () => {
            made = await api("POST", base + "/links", { plan: plan.slug, level: level.value, label: label.value, expires_days: Number(expires.value) });
          })) {
            redraw(dlg, build);
            try {
              await navigator.clipboard.writeText(linkURL(made));
              say(t("share.made_copied"));
            } catch (err) {
              say(t("share.made"));
            }
          }
        } }, field(t("share.label"), label), field(t("share.level"), level), field(t("share.expires"), expires), h("button", { type: "submit" }, t("share.create"))),
        foot("", closeBtn())
      );
    };
    await build();
  }
  async function pageMembers(r, org) {
    const base = `/orgs/${enc(r.org)}`;
    if (!org.role) return go("/o/" + enc(r.org));
    const admin = org.role === "admin";
    const [members, links] = await Promise.all([api("GET", base + "/members"), api("GET", base + "/links")]);
    const again = () => render();
    const invites = links.filter((l) => l.level === "join");
    const shared = links.filter((l) => l.level !== "join");
    const leave = async (m) => {
      const yes = m.you ? await ask(t("members.leave_title"), t("members.leave_confirm", { name: org.name }), t("members.leave")) : await ask(t("members.remove_title"), t("members.remove_confirm", { name: m.name }), t("members.remove"));
      if (!yes) return;
      if (await write(() => api("DELETE", `${base}/members/${m.id}`))) {
        if (m.you) {
          store.set("org", "");
          fetched.clear();
          go("/?switch");
        } else again();
      }
    };
    const row = (m) => h(
      "li",
      { class: "member" },
      h("span", { class: "avatar sm", "aria-hidden": "true" }, initials(m.name)),
      h(
        "span",
        { class: "what" },
        h("span", { class: "title" }, m.name, m.you ? h("span", { class: "muted small" }, " " + t("members.you")) : ""),
        m.email ? h("span", { class: "meta muted small" }, m.email) : ""
      ),
      h(
        "span",
        { class: "acts" },
        admin ? h("select", { "aria-label": t("members.role_aria", { name: m.name }), onchange: async (e) => {
          const to = e.target.value;
          if (m.you && !await ask(t("members.own_role_title"), t("members.own_role_confirm", { role: t(ROLE[to]).toLowerCase() }), t("members.own_role_yes"))) {
            e.target.value = m.role;
            return;
          }
          if (await write(() => api("PATCH", `${base}/members/${m.id}`, { role: to }))) again();
          else e.target.value = m.role;
        } }, ["admin", "member"].map((x) => h("option", { value: x, selected: x === m.role }, t(ROLE[x])))) : h("span", { class: "muted small" }, t(ROLE[m.role])),
        admin || m.you ? iconBtn(m.you ? "logout" : "trash", m.you ? t("members.leave") : t("members.remove_aria", { name: m.name }), () => leave(m), "danger") : ""
      )
    );
    const expires = expirySelect(7);
    const who = h("input", { type: "text", maxlength: "80", placeholder: t("members.invite_who_placeholder") });
    const uses = h(
      "select",
      { "aria-label": t("members.invite_uses") },
      h("option", { value: "once", selected: true }, t("members.invite_once")),
      h("option", { value: "many" }, t("members.invite_many"))
    );
    return h(
      "div",
      { class: "org-page members-page" },
      h("h1", {}, t("nav.members"), infoBtn(t("nav.members"), [t("members.info")])),
      h("ul", { class: "plain members" }, members.map(row)),
      admin ? [
        h("h2", {}, t("members.invite")),
        h("p", { class: "muted small lead" }, t("members.invite_hint")),
        invites.length ? h("ul", { class: "plain links" }, invites.map((l) => linkRow(l, base, again))) : "",
        h("form", { class: "new-link", onsubmit: async (e) => {
          e.preventDefault();
          let made;
          if (await write(async () => {
            made = await api("POST", base + "/links", { level: "join", label: who.value.trim(), single_use: uses.value === "once", expires_days: Number(expires.value) });
          })) {
            again();
            try {
              await navigator.clipboard.writeText(linkURL(made));
              say(t("members.invite_made_copied"));
            } catch (err) {
              say(t("members.invite_made"));
            }
          }
        } }, field(t("members.invite_who"), who), field(t("members.invite_uses"), uses), field(t("share.expires"), expires), h("button", { type: "submit" }, t("members.invite_create")))
      ] : "",
      h("h2", {}, t("members.plan_links")),
      h("p", { class: "muted small lead" }, t("members.plan_links_hint")),
      shared.length ? h("ul", { class: "plain links" }, shared.map((l) => linkRow(l, base, again, true))) : h("p", { class: "muted" }, t("share.none"))
    );
  }

  // web/src/api.ts
  var ROUTES = [
    ["GET", "/health", "AccountService/Health"],
    ["GET", "/me", "AccountService/Me"],
    ["GET", "/orgs", "AccountService/ListOrgs", "orgs"],
    ["POST", "/orgs", "AccountService/CreateOrg", "org"],
    ["GET", "/orgs/{org}", "OrgService/GetOrg"],
    ["PATCH", "/orgs/{org}", "OrgService/RenameOrg", "org"],
    ["DELETE", "/orgs/{org}", "OrgService/DeleteOrg"],
    ["GET", "/orgs/{org}/members", "OrgService/ListMembers", "members"],
    ["PATCH", "/orgs/{org}/members/{id}", "OrgService/SetMemberRole"],
    ["DELETE", "/orgs/{org}/members/{id}", "OrgService/RemoveMember"],
    ["GET", "/orgs/{org}/links", "OrgService/ListLinks", "links"],
    ["POST", "/orgs/{org}/links", "OrgService/CreateLink", "link"],
    ["DELETE", "/orgs/{org}/links/{id}", "OrgService/DeleteLink"],
    ["GET", "/orgs/{org}/tasks", "SetupService/ListTasks", "tasks"],
    ["POST", "/orgs/{org}/tasks", "SetupService/CreateTask", "task"],
    ["PATCH", "/orgs/{org}/tasks/{id}", "SetupService/UpdateTask", "task"],
    ["DELETE", "/orgs/{org}/tasks/{id}", "SetupService/DeleteTask"],
    ["PUT", "/orgs/{org}/tasks/{id}/position", "SetupService/MoveTask"],
    ["GET", "/orgs/{org}/combinations", "SetupService/ListCombinations", "combinations"],
    ["PUT", "/orgs/{org}/combinations", "SetupService/SetCombination"],
    ["PUT", "/orgs/{org}/tasks/{task}/combinations", "SetupService/SetTaskCombinations"],
    ["GET", "/orgs/{org}/people", "SetupService/ListPeople", "people"],
    ["POST", "/orgs/{org}/people", "SetupService/CreatePerson", "person"],
    ["PATCH", "/orgs/{org}/people/{id}", "SetupService/UpdatePerson", "person"],
    ["PUT", "/orgs/{org}/people/{id}/position", "SetupService/MovePerson"],
    ["PUT", "/orgs/{org}/people/{person}/tasks/{task}", "SetupService/SetPersonTask"],
    ["DELETE", "/orgs/{org}/people/{person}/tasks/{task}", "SetupService/RemovePersonTask"],
    ["GET", "/orgs/{org}/sections", "SetupService/ListSections", "sections"],
    ["POST", "/orgs/{org}/sections", "SetupService/CreateSection", "section"],
    ["PATCH", "/orgs/{org}/sections/{id}", "SetupService/RenameSection", "section"],
    ["PUT", "/orgs/{org}/sections/{id}/position", "SetupService/MoveSection"],
    ["DELETE", "/orgs/{org}/sections/{id}", "SetupService/DeleteSection"],
    ["GET", "/orgs/{org}/task-sections", "SetupService/ListTaskSections", "sections"],
    ["POST", "/orgs/{org}/task-sections", "SetupService/CreateTaskSection", "section"],
    ["PATCH", "/orgs/{org}/task-sections/{id}", "SetupService/RenameTaskSection", "section"],
    ["PUT", "/orgs/{org}/task-sections/{id}/position", "SetupService/MoveTaskSection"],
    ["DELETE", "/orgs/{org}/task-sections/{id}", "SetupService/DeleteTaskSection"],
    ["GET", "/orgs/{org}/plans", "PlanService/ListPlans", "plans"],
    ["POST", "/orgs/{org}/plans", "PlanService/CreatePlan", "plan"],
    ["GET", "/orgs/{org}/plans/{plan}", "PlanService/GetPlan", "plan"],
    ["PATCH", "/orgs/{org}/plans/{plan}", "PlanService/RenamePlan", "plan"],
    ["DELETE", "/orgs/{org}/plans/{plan}", "PlanService/DeletePlan"],
    ["GET", "/orgs/{org}/plans/{plan}/events", "PlanService/ListEvents", "events"],
    ["POST", "/orgs/{org}/plans/{plan}/events", "PlanService/AddEvents", "events"],
    ["POST", "/orgs/{org}/plans/{plan}/events/{id}/copy", "PlanService/CopyEvent", "events"],
    ["PATCH", "/orgs/{org}/plans/{plan}/events/{id}", "PlanService/UpdateEvent", "event"],
    ["DELETE", "/orgs/{org}/plans/{plan}/events/{id}", "PlanService/DeleteEvent"],
    ["GET", "/orgs/{org}/plans/{plan}/places", "PlanService/ListPlaces", "places"],
    ["PUT", "/orgs/{org}/plans/{plan}/events/{event}/places/{task}", "PlanService/SetPlaces"],
    ["DELETE", "/orgs/{org}/plans/{plan}/events/{event}/places/{task}", "PlanService/ResetPlaces"],
    ["PUT", "/orgs/{org}/plans/{plan}/places", "PlanService/SetPlacesMany"],
    ["GET", "/orgs/{org}/plans/{plan}/fields", "PlanService/ListFields"],
    ["POST", "/orgs/{org}/plans/{plan}/fields", "PlanService/AddField", "field"],
    ["PATCH", "/orgs/{org}/plans/{plan}/fields/{id}", "PlanService/RenameField", "field"],
    ["PUT", "/orgs/{org}/plans/{plan}/fields/{id}/position", "PlanService/MoveField"],
    ["PUT", "/orgs/{org}/plans/{plan}/fields/{id}/frozen", "PlanService/FreezeField"],
    ["DELETE", "/orgs/{org}/plans/{plan}/fields/{id}", "PlanService/DeleteField"],
    ["PUT", "/orgs/{org}/plans/{plan}/events/{event}/fields/{field}", "PlanService/SetFieldValue"],
    ["GET", "/orgs/{org}/plans/{plan}/answers", "PlanService/ListAnswers"],
    ["PUT", "/orgs/{org}/plans/{plan}/people/{person}/availability", "PlanService/SetAvailability"],
    ["PUT", "/orgs/{org}/plans/{plan}/events/{event}/notes/{person}", "PlanService/SetNote"],
    ["PUT", "/orgs/{org}/plans/{plan}/answers", "PlanService/SetAnswers"],
    ["GET", "/orgs/{org}/plans/{plan}/cells", "DraftService/ListCells", "cells"],
    ["POST", "/orgs/{org}/plans/{plan}/cells", "DraftService/AddCell", "cell"],
    ["DELETE", "/orgs/{org}/plans/{plan}/cells/{id}", "DraftService/DeleteCell"],
    ["GET", "/orgs/{org}/plans/{plan}/pools", "DraftService/ListPools", "pools"],
    ["PUT", "/orgs/{org}/plans/{plan}/pools", "DraftService/SetPool"],
    ["PUT", "/orgs/{org}/plans/{plan}/locks", "DraftService/SetLocks"],
    ["DELETE", "/orgs/{org}/plans/{plan}/draft", "DraftService/SetDraftAside"],
    ["PUT", "/orgs/{org}/plans/{plan}/draft", "DraftService/ClearDraftAside"],
    ["GET", "/orgs/{org}/plans/{plan}/weights", "DraftService/GetWeights"],
    ["PUT", "/orgs/{org}/plans/{plan}/weights", "DraftService/SetWeights"],
    ["POST", "/orgs/{org}/plans/{plan}/runs", "DraftService/StartRun"],
    ["GET", "/orgs/{org}/plans/{plan}/runs", "DraftService/ListRuns", "runs"],
    ["GET", "/orgs/{org}/plans/{plan}/runs/{id}", "DraftService/GetRun"],
    ["PUT", "/orgs/{org}/plans/{plan}/runs/{id}/pin", "DraftService/PinRun"],
    ["DELETE", "/orgs/{org}/plans/{plan}/runs/{id}/pin", "DraftService/UnpinRun"]
  ].map(([method, path, proc, unwrap]) => ({ method, parts: path.split("/").filter(Boolean), proc, unwrap }));
  function call(method, path, body) {
    const [p, query] = path.split("?");
    const segs = p.split("/").filter(Boolean).map(decodeURIComponent);
    for (const r of ROUTES) {
      if (r.method !== method || r.parts.length !== segs.length) continue;
      const req = {};
      if (!r.parts.every((part, i) => {
        if (part[0] !== "{") return part === segs[i];
        const name = part.slice(1, -1);
        req[name] = name === "org" || name === "plan" ? segs[i] : segs[i] === "latest" ? 0 : Number(segs[i]);
        return true;
      })) continue;
      for (const [k, v] of new URLSearchParams(query || "")) req[k] = v;
      return { route: r, req: { ...body, ...req }, org: req.org || "" };
    }
    throw new Error("no route for " + method + " " + path);
  }
  async function api(method, path, body) {
    const { route: r, req, org } = call(method, path, body);
    if (r.proc === "DraftService/GetRun") req.lang = currentLang;
    const headers = { "Content-Type": "application/json" };
    if (method !== "GET") headers["X-Dutyplan-Client"] = CLIENT;
    let res;
    try {
      res = await fetch("/dutyplan.v1." + r.proc, { method: "POST", headers, redirect: "manual", body: JSON.stringify(req) });
    } catch (e) {
      connection("offline");
      throw Object.assign(new Error(t("net.offline")), { connection: "offline" });
    }
    const json = (res.headers.get("Content-Type") || "").startsWith("application/json");
    if (res.type === "opaqueredirect" || res.status === 401 || res.ok && !json) {
      connection("signed_out");
      throw Object.assign(new Error(t("net.signed_out")), { connection: "signed_out" });
    }
    connection("");
    const data = await res.json().catch(() => ({}));
    if (method !== "GET") forget(changedBy(procOf(r.proc)), org);
    if (!res.ok) {
      const p = data.details && data.details[0] && data.details[0].debug || {};
      const kind = p.error || (data.code === "invalid_argument" ? "invalid" : data.error || data.code);
      throw Object.assign(new Error(data.message || res.statusText), { kind, apiCode: p.code || (data.error ? data.code : void 0), params: p.params || data.params, status: res.status });
    }
    return r.unwrap ? data[r.unwrap] : data;
  }
  var procOf = (proc) => proc.slice(proc.indexOf("/") + 1);
  var connState = "";
  function connection(state) {
    if (state === connState) return;
    connState = state;
    let bar = document.getElementById("conn");
    if (!state) {
      if (bar) bar.remove();
      return;
    }
    if (!bar) {
      bar = h("div", { id: "conn", role: "alert" });
      document.body.prepend(bar);
    }
    bar.className = state;
    bar.replaceChildren(connectionLine(state));
  }
  function connectionLine(state) {
    const retry = () => {
      connection("");
      fetched.clear();
      if (!live2) follow(route().org);
      render();
    };
    return h(
      "span",
      { class: "conn-line" },
      h("span", {}, t(state === "signed_out" ? "net.signed_out" : "net.offline")),
      " ",
      state === "signed_out" ? h("button", { type: "button", onclick: signIn }, t("net.sign_in")) : h("button", { type: "button", onclick: (e) => {
        const d = e.currentTarget.closest("dialog");
        if (d) d.close();
        retry();
      } }, t("net.retry"))
    );
  }
  var probe = () => api("GET", "/health").then(() => true, () => false);
  var fetched = /* @__PURE__ */ new Map();
  function get(path) {
    if (!fetched.has(path)) {
      const p = api("GET", path);
      fetched.set(path, p);
      p.catch(() => {
        if (fetched.get(path) === p) fetched.delete(path);
      });
    }
    return fetched.get(path);
  }
  function kindOf(path) {
    const p = path.split("/").filter(Boolean);
    if (p[0] !== "orgs") return p[0];
    if (p.length === 1) return "orgs";
    if (p.length === 2) return "org";
    if (p[2] === "plans" && p.length >= 4) return p.length === 4 ? "plan" : p[4];
    return p[2];
  }
  var RUNS_TOO = (...kinds) => ["runs", ...kinds];
  var CHANGES = {
    CreateOrg: ["orgs"],
    CreatePlan: ["plans"],
    RenamePlan: ["plan", "plans"],
    // a new plan may get the address again
    SetNote: ["answers"],
    // a note on an answer: never in the planner's input
    SetPlaces: RUNS_TOO("places"),
    ResetPlaces: RUNS_TOO("places"),
    SetPlacesMany: RUNS_TOO("places"),
    SetFieldValue: ["fields"],
    AddField: ["fields"],
    RenameField: ["fields"],
    MoveField: ["fields"],
    FreezeField: ["fields"],
    DeleteField: ["fields"],
    AddEvents: RUNS_TOO("events", "places", "fields", "cells", "pools", "answers"),
    CopyEvent: RUNS_TOO("events", "places", "fields", "cells", "pools", "answers"),
    UpdateEvent: RUNS_TOO("events", "places", "fields", "cells", "pools", "answers"),
    DeleteEvent: RUNS_TOO("events", "places", "fields", "cells", "pools", "answers"),
    SetAvailability: RUNS_TOO("answers"),
    SetAnswers: RUNS_TOO("answers"),
    SetLocks: RUNS_TOO("cells"),
    // names of the draft locked or unlocked
    SetDraftAside: ["plan", "runs"],
    ClearDraftAside: ["plan", "runs"],
    // «Remove unlocked»: no draft on show
    StartRun: ["runs", "plan"],
    PinRun: ["runs", "plan"],
    UnpinRun: ["runs", "plan"],
    RunFinished: ["runs", "plan"],
    // a new draft or a pin ends «Remove unlocked»
    AddCell: RUNS_TOO("cells"),
    DeleteCell: RUNS_TOO("cells"),
    SetPool: RUNS_TOO("pools"),
    SetWeights: RUNS_TOO("weights"),
    SetPersonTask: RUNS_TOO("answers"),
    RemovePersonTask: RUNS_TOO("answers"),
    CreatePerson: RUNS_TOO("people"),
    UpdatePerson: RUNS_TOO("people"),
    MovePerson: RUNS_TOO("people"),
    SetCombination: RUNS_TOO("combinations"),
    SetTaskCombinations: RUNS_TOO("combinations"),
    CreateSection: ["sections", "people"],
    RenameSection: ["sections", "people"],
    MoveSection: ["sections", "people"],
    DeleteSection: ["sections", "people"],
    CreateTaskSection: ["task-sections", "tasks"],
    RenameTaskSection: ["task-sections", "tasks"],
    MoveTaskSection: ["task-sections", "tasks"],
    DeleteTaskSection: ["task-sections", "tasks"]
  };
  var changedBy = (proc) => CHANGES[proc] || null;
  function forget(kinds, orgSlug) {
    const org = orgSlug ? "/orgs/" + enc(orgSlug) : "";
    for (const key of [...fetched.keys()]) {
      if (!kinds) {
        if (!org || key.startsWith(org + "/") || key === org || key === "/orgs") fetched.delete(key);
        continue;
      }
      if (kinds.includes(kindOf(key)) && (key === "/orgs" || !org || key === org || key.startsWith(org + "/"))) fetched.delete(key);
    }
  }
  function errorText(e) {
    if (e.apiCode && hasMsg("err." + e.apiCode)) {
      const p = { ...e.params };
      if (typeof p.kind === "string") p.kind = cap(hasMsg("kind." + p.kind.replace(" ", "_")) ? t("kind." + p.kind.replace(" ", "_")) : p.kind);
      if (typeof p.what === "string" && hasMsg("what." + p.what)) p.what = t("what." + p.what);
      if (typeof p.key === "string" && hasMsg("weight." + p.key + ".label")) p.key = t("weight." + p.key + ".label");
      return t("err." + e.apiCode, p);
    }
    if (e.kind && hasMsg("err.kind." + e.kind)) return t("err.kind." + e.kind);
    return e.message || t("err.generic");
  }
  var cancelled = () => Object.assign(new Error("cancelled"), { cancelled: true });
  async function confirmed(title, text, yes) {
    if (!await ask(title, text, yes)) throw cancelled();
  }
  async function write(action) {
    try {
      await action();
      say("");
      return true;
    } catch (e) {
      if (e.cancelled) return false;
      if (e.connection) {
        const m = document.querySelector("dialog[open] [data-msg]");
        if (m) {
          m.className = "err";
          m.replaceChildren(connectionLine(e.connection));
        } else say("");
        return false;
      }
      say(errorText(e), "err");
      return false;
    }
  }

  // web/src/plan.ts
  var ANSWER_NEXT = { "": "good", good: "poor", poor: "never", never: "" };
  var answerMark = (l) => l ? icon(LEVEL_ICON[l]) : "";
  var TASK_NEXT = { "": "poor", poor: "good", good: "" };
  var COMBO_NEXT = { cannot: "standard", standard: "practical", practical: "cannot" };
  function cycle(kind, level, aria, onNext) {
    const text = (l) => t(kind === "task" ? { "": "pick.not", poor: "pick.poor", good: "pick.good" }[l] : { cannot: "combo.cannot", standard: "combo.bad", practical: "combo.good" }[l]);
    const b = h("button", { type: "button", class: "cyc" });
    const paint = (l) => {
      b.dataset.level = l || "none";
      b.title = text(l);
      b.setAttribute("aria-label", `${aria}: ${text(l)}`);
      b.replaceChildren(icon(LEVEL_ICON[l]));
    };
    let shown = level, busy = false;
    b.addEventListener("click", async () => {
      shown = kind === "task" ? TASK_NEXT[shown] : COMBO_NEXT[shown];
      paint(shown);
      if (busy) return;
      busy = true;
      while (shown !== level) {
        const want = shown;
        if (await onNext(want)) level = want;
        else {
          shown = level;
          paint(level);
        }
      }
      busy = false;
    });
    paint(level);
    return b;
  }
  var WEIGHT_GROUP = {
    w_fair: "fair",
    w_fair_range: "fair",
    w_task_fair: "fair",
    w_task_range: "fair",
    "w_task.good": "wishes",
    "w_task.poor": "wishes",
    "w_date.good": "wishes",
    "w_date.poor": "wishes",
    "w_combo.practical": "combos",
    "w_combo.impractical": "combos"
  };
  var weightLabel = (key) => t("weight." + key + ".label");
  var weightGroup = (key) => t("weight.group." + (WEIGHT_GROUP[key] || "fair"));
  function weightsForm(path, wr, saved) {
    const inputs = wr.specs.map((spec) => {
      const cur = spec.key in wr.values ? wr.values[spec.key] : spec.default;
      const input = h("input", { type: "number", min: String(spec.min), max: String(spec.max), step: "any", value: String(cur), "aria-label": weightLabel(spec.key) });
      return { spec, input, changed: spec.key in wr.values };
    });
    const groups = [...new Set(wr.specs.map((x) => weightGroup(x.key)))];
    const blocks = groups.map((g) => h(
      "div",
      { class: "weight-group" },
      h("h4", {}, g),
      inputs.filter(({ spec }) => weightGroup(spec.key) === g).map(({ spec, input, changed }) => {
        const def = spec.default;
        return h(
          "div",
          { class: "weight-row" + (changed ? " changed" : "") },
          h("label", {}, h("span", { class: "wlabel" }, weightLabel(spec.key), changed ? h("span", { class: "tag" }, t("weights.changed")) : ""), input),
          h("div", { class: "small muted" }, t("weights.default", { value: def }))
        );
      })
    ));
    const save = (values, text) => async () => {
      if (await write(() => api("PUT", path, { values }))) saved(text);
    };
    const nChanged = Object.keys(wr.values || {}).length;
    return h(
      "details",
      {},
      h("summary", {}, nChanged ? t("weights.show_changed", { n: nChanged }) : t("weights.show")),
      h("div", { class: "row-inline" }, infoBtn(t("weights.title"), [t("weights.hint"), ...wr.specs.map((x) => `${weightLabel(x.key)}: ${t("weight." + x.key + ".hint")}`)])),
      h(
        "div",
        { class: "stack" },
        blocks,
        h(
          "div",
          { class: "row-inline" },
          h("button", { type: "button", onclick: () => {
            const values = {};
            for (const { spec, input } of inputs) {
              if (input.value.trim() !== "") values[spec.key] = Number(input.value);
            }
            return save(values, t("weights.saved"))();
          } }, t("weights.save")),
          h("button", { type: "button", class: "ghost", onclick: save({}, t("weights.reset_done")) }, t("weights.reset"))
        )
      )
    );
  }
  var PRIORITIES = [
    { key: "fair", levels: { less: { w_fair: 4, w_fair_range: 4 }, normal: { w_fair: 10, w_fair_range: 10 }, more: { w_fair: 25, w_fair_range: 25 } } },
    { key: "wishes", levels: {
      less: { "w_task.good": 1, "w_task.poor": -2, "w_date.good": 1, "w_date.poor": -3 },
      normal: { "w_task.good": 2, "w_task.poor": -4, "w_date.good": 3, "w_date.poor": -6 },
      more: { "w_task.good": 5, "w_task.poor": -10, "w_date.good": 8, "w_date.poor": -15 }
    } },
    { key: "combos", levels: {
      less: { "w_combo.practical": 1, "w_combo.impractical": -2 },
      normal: { "w_combo.practical": 2, "w_combo.impractical": -5 },
      more: { "w_combo.practical": 5, "w_combo.impractical": -12 }
    } }
  ];
  var weightNow = (wr, k) => k in (wr.values || {}) ? wr.values[k] : (wr.specs.find((x) => x.key === k) || {}).default;
  var priorityLevel = (wr, p) => Object.keys(p.levels).find((l) => Object.entries(p.levels[l]).every(([k, v]) => weightNow(wr, k) === v)) || "";
  var prioritySummary = (wr) => PRIORITIES.map((p) => t("prio." + p.key + ".short") + ": " + t("prio.level." + (priorityLevel(wr, p) || "custom"))).join(" · ");
  function prioritiesForm(path, wr, saved) {
    const rows = PRIORITIES.map((p) => {
      const now = priorityLevel(wr, p);
      return h(
        "div",
        { class: "prio" },
        h("div", {}, h("div", { class: "plabel" }, t("prio." + p.key)), h("div", { class: "small muted" }, t("prio." + p.key + ".hint"))),
        h("div", { class: "seg", role: "group", "aria-label": t("prio." + p.key) }, ["less", "normal", "more"].map((l) => h("button", {
          type: "button",
          "aria-pressed": String(now === l),
          onclick: async () => {
            if (now === l) return;
            if (await write(() => api("PUT", path, { values: { ...wr.values || {}, ...p.levels[l] } }))) saved(t("prio.saved", { what: t("prio." + p.key), level: t("prio.level." + l) }));
          }
        }, t("prio.level." + l)))),
        now ? "" : h("div", { class: "small muted" }, t("prio.custom_hint"))
      );
    });
    return h("div", { class: "stack" }, rows);
  }
  var planState = { key: "", runId: null, aside: false, undo: [], adds: /* @__PURE__ */ new Set() };
  document.addEventListener("keydown", (e) => {
    if (!(e.ctrlKey || e.metaKey) || e.shiftKey || e.altKey || e.key.toLowerCase() !== "z" || !planState.undoLast) return;
    if (!document.querySelector(".draft-bar")) return;
    const target = e.target;
    if (target.closest && target.closest("input, textarea, select, [contenteditable]")) return;
    e.preventDefault();
    planState.undoLast();
  });
  function sideways(grid, events) {
    if (!grid) return "";
    const firstCol = () => (grid.querySelector("tbody th[scope=row]") || { offsetWidth: 0 }).offsetWidth;
    const by = (dir) => grid.scrollBy({ left: dir * Math.max(120, grid.clientWidth - firstCol() - 40), behavior: "smooth" });
    const toNext = () => {
      const today = (/* @__PURE__ */ new Date()).toISOString().slice(0, 10);
      const i = events.findIndex((ev) => ev.date >= today);
      const th = grid.querySelectorAll("thead th[scope=col]")[i < 0 ? events.length - 1 : i];
      if (th) grid.scrollTo({ left: th.offsetLeft - firstCol(), behavior: "smooth" });
    };
    grid.addEventListener("wheel", (e) => {
      if (!e.target.closest("thead") || Math.abs(e.deltaX) > Math.abs(e.deltaY)) return;
      e.preventDefault();
      grid.scrollLeft += e.deltaY;
    });
    grid.addEventListener("keydown", (e) => {
      if (e.target !== grid) return;
      if (e.key === "ArrowRight" || e.key === "ArrowLeft") {
        e.preventDefault();
        by(e.key === "ArrowRight" ? 1 : -1);
      }
    });
    grid.tabIndex = 0;
    grid.setAttribute("aria-label", t("table.scroll_aria"));
    return h(
      "span",
      { class: "sideways" },
      h("button", { type: "button", class: "ghost icon", title: t("table.earlier"), "aria-label": t("table.earlier"), onclick: () => by(-1) }, icon("left")),
      h("button", { type: "button", class: "ghost icon", title: t("table.next_aria"), "aria-label": t("table.next_aria"), onclick: toNext }, icon("today")),
      h("button", { type: "button", class: "ghost icon", title: t("table.later"), "aria-label": t("table.later"), onclick: () => by(1) }, icon("right"))
    );
  }
  function dragScroll(grid) {
    let start = null;
    let moved = false;
    grid.addEventListener("pointerdown", (e) => {
      if (e.pointerType !== "mouse" || e.button !== 0 || e.target.closest("button, input, select, textarea, a, label, .grip, td[data-cell], th[scope=row]:has(.grip)")) return;
      start = { x: e.clientX, left: grid.scrollLeft };
      moved = false;
    });
    grid.addEventListener("pointermove", (e) => {
      if (!start) return;
      const dx = e.clientX - start.x;
      if (!moved && Math.abs(dx) < 5) return;
      if (!moved) grid.setPointerCapture(e.pointerId);
      moved = true;
      grid.classList.add("dragging");
      grid.scrollLeft = start.left - dx;
    });
    const end = () => {
      start = null;
      grid.classList.remove("dragging");
    };
    grid.addEventListener("pointerup", end);
    grid.addEventListener("pointercancel", end);
    grid.addEventListener("click", (e) => {
      if (moved) {
        e.stopPropagation();
        e.preventDefault();
        moved = false;
      }
    }, true);
  }
  async function pagePlan(r, org, people) {
    const base = `/orgs/${enc(r.org)}`;
    const pbase = `${base}/plans/${enc(r.plan)}`;
    const epath = pbase + "/events";
    const key = r.org + "/" + r.plan;
    if (planState.key !== key) Object.assign(planState, { key, runId: null, aside: false, sel: null, adds: /* @__PURE__ */ new Set(), undo: [] });
    const [plan, events, tasks, combos, fields, answers, places, cells, pools, weights, health, runs, sections, taskSections] = await Promise.all([
      get(pbase),
      get(epath),
      get(base + "/tasks"),
      get(base + "/combinations"),
      get(pbase + "/fields"),
      get(pbase + "/answers"),
      get(pbase + "/places"),
      get(pbase + "/cells"),
      get(pbase + "/pools"),
      get(pbase + "/weights"),
      get("/health"),
      get(pbase + "/runs"),
      get(base + "/sections"),
      get(base + "/task-sections")
    ]);
    if (plan.draft_aside && !planState.aside) planState.runId = null;
    planState.aside = plan.draft_aside;
    const pinned = runs.find((x) => x.pinned);
    const shownId = runs.some((x) => x.id === planState.runId) ? planState.runId : plan.draft_aside ? null : pinned ? pinned.id : runs.length ? runs[0].id : null;
    const modeKey = "mode:" + key;
    const modes = org.level === "answer" ? ["me", "show"] : ["edit", "show"];
    const stored = store.get(modeKey);
    const mode = modes.includes(stored) ? stored : org.level === "answer" ? "me" : org.level === "view" ? "show" : "edit";
    const viewId = mode === "edit" ? shownId : plan.draft_aside ? null : pinned ? pinned.id : (runs.find((x) => x.status === "done") || {}).id;
    const view = viewId ? await get(`${pbase}/runs/${viewId}`) : null;
    const done = !!view && view.run.status === "done";
    const label = (ev) => fmtShortDate(ev.date) + (ev.start_time ? " " + ev.start_time : "");
    const nameOf = (list, id) => (list.find((x) => x.id === id) || { name: "?" }).name;
    const at = (ev, tk) => ev.id + "/" + tk.id;
    const ans = {};
    answers.events.forEach((a) => {
      (ans[a.person_id] ||= {})[a.event_id] = a.level;
    });
    const notes = {};
    (answers.notes || []).forEach((n) => {
      (notes[n.person_id] ||= {})[n.event_id] = n.note;
    });
    const chosen = {};
    answers.tasks.forEach((a) => {
      (chosen[a.person_id] ||= []).push(a);
    });
    const placesAt = {};
    places.forEach((p) => {
      placesAt[p.event_id + "/" + p.task_id] = p.slots;
    });
    const fieldValue = {};
    fields.values.forEach((v) => {
      fieldValue[v.event_id + "/" + v.field_id] = v.value;
    });
    const cellsAt = {};
    cells.forEach((c) => {
      (cellsAt[c.event_id + "/" + c.task_id] ||= []).push(c);
    });
    const poolAt = {};
    pools.forEach((p) => {
      poolAt[p.event_id + "/" + p.task_id] = p;
    });
    const planCell = {};
    if (done) for (const ev of view.events) for (const c of ev.cells) planCell[ev.id + "/" + c.task_id] = c;
    const changesKey = "changes:" + key;
    const shownAt = done ? runs.findIndex((x) => x.id === view.run.id) : -1;
    const olderId = shownAt < 0 ? null : (runs.slice(shownAt + 1).find((x) => x.status === "done") || {}).id;
    const comparing = mode === "edit" && !!olderId && store.get(changesKey) === "1";
    const before = {};
    if (comparing) for (const ev of (await get(`${pbase}/runs/${olderId}`)).events) for (const c of ev.cells) before[ev.id + "/" + c.task_id] = c.people;
    const slotsAt = (ev, tk) => {
      const k = at(ev, tk);
      const n = placesAt[k] || 0;
      const items = (cellsAt[k] || []).map((c) => ({
        cell: c,
        kind: c.person_id ? "person" : "text",
        id: c.person_id || 0,
        name: c.person_id ? nameOf(people, c.person_id) : c.label,
        fills: c.fills || 1,
        hand: !c.locked,
        locked: c.locked
      }));
      const rc = planCell[k];
      if (rc) {
        const has = (kind, id) => items.some((x) => x.kind === kind && x.id === id);
        rc.people.filter((p) => (!p.locked || p.kept) && !has("person", p.id)).forEach((p) => items.push({ kind: "person", id: p.id, name: p.name, fills: 1, weak: p.weak }));
      }
      const filled = items.reduce((s, x) => s + x.fills, 0);
      return { n, items, open: Math.max(0, n - filled), why: rc && rc.missing > 0 ? rc.explanation || "" : "" };
    };
    const itemText = (x) => x.name + (x.fills > 1 ? " ×" + x.fills : "");
    const slot = (x) => h(
      "span",
      { class: "slot" + (x.hand ? " hand" : "") + (x.locked ? " locked" : "") },
      x.locked ? mark("lock") : x.hand ? mark("hand") : "",
      itemText(x),
      x.weak ? h("span", { class: "chip poor" }, t("draft.maybe")) : ""
    );
    const lockItems = (pairs, on) => pairs.flatMap(([ev, tk]) => slotsAt(ev, tk).items.filter((x) => x.kind === "person" && (on ? !x.cell : x.locked)).map((x) => ({ event: ev.id, task: tk.id, person: x.id })));
    const setLocks = async (list, on) => {
      if (!list.length) {
        say(t(on ? "sel.nothing_to_lock" : "sel.nothing_to_unlock"));
        return;
      }
      if (!await write(() => api("PUT", pbase + "/locks", { locks: list, locked: on }))) return;
      undoable(tn(on ? "undo.lock" : "undo.unlock", list.length), () => api("PUT", pbase + "/locks", { locks: list, locked: !on }));
      if (sel && sel.grid === "draft") planState.sel = { keys: [...sel.keys], anchor: sel.anchor };
      render();
    };
    const everyPlace = () => events.flatMap((ev) => tasks.map((tk) => [ev, tk]));
    const undoable = (label2, undo) => {
      planState.undo.push({ label: label2, undo });
      if (planState.undo.length > 30) planState.undo.shift();
    };
    const undoLast = planState.undoLast = async () => {
      const u = planState.undo.pop();
      if (!u) return;
      if (await write(u.undo)) {
        await render();
        say(t("undo.done", { what: u.label }));
      } else render();
    };
    const cellBack = (c) => ({ event: c.event_id, task: c.task_id, ...c.person_id ? { person: c.person_id } : { label: c.label }, fills: c.fills, locked: c.locked });
    const groupLock = (pairs, lockAria, unlockAria) => {
      const toLock = lockItems(pairs, true), toUnlock = lockItems(pairs, false);
      if (!toLock.length && !toUnlock.length) return "";
      const on = !toLock.length;
      const name = on ? unlockAria : lockAria;
      return h("button", {
        type: "button",
        class: "lockbtn group" + (on ? " on" : ""),
        "aria-pressed": String(on),
        title: name,
        "aria-label": name,
        onclick: () => setLocks(on ? toUnlock : toLock, !on)
      }, icon(on ? "lock" : "unlock"));
    };
    const modeSwitch = h(
      "div",
      { class: "mode", role: "group", "aria-label": t("mode.aria") },
      modes.map((m) => h("button", {
        type: "button",
        "aria-pressed": String(mode === m),
        "data-mode": m,
        onclick: () => {
          if (m !== mode) {
            store.set(modeKey, m);
            render();
          }
        }
      }, t(m === "edit" && org.level === "view" ? "mode.table" : "mode." + m)))
    );
    const shareBtn = () => ME && ME.login && org.role ? h("button", { type: "button", class: "ghost share", onclick: () => shareDialog(r.org, org, plan) }, icon("share"), h("span", { class: "lbl" }, " " + t("share.open"))) : "";
    const back = () => org.plan ? "" : link(`/o/${enc(r.org)}`, "‹ " + org.name, { class: "back small", "aria-label": t("common.back", { name: org.name }) });
    const showTasksOf = (ev) => tasks.filter((tk) => placesAt[at(ev, tk)] || cellsAt[at(ev, tk)]);
    if (mode === "show") return showPage();
    if (mode === "me") return mePage();
    function mePage() {
      const meKey = "me:" + key;
      const me4 = people.find((p) => String(p.id) === store.get(meKey));
      const title = h("div", { class: "title-bar" }, h("h1", {}, plan.name), infoBtn(plan.name, [t("me.info")]), modeSwitch);
      if (!me4) {
        const picks = [...people].sort((a, b) => a.name.localeCompare(b.name, locale())).map((p) => h("button", {
          type: "button",
          class: "ghost me-pick",
          "data-name": p.name.toLowerCase(),
          onclick: () => {
            store.set(meKey, String(p.id));
            render();
          }
        }, p.name));
        const find = people.length > 8 ? h("input", { type: "search", class: "wide", placeholder: t("table.filter"), "aria-label": t("table.filter"), oninput: (e) => {
          const q = e.target.value.trim().toLowerCase();
          picks.forEach((b) => {
            b.hidden = !!q && !b.dataset.name.includes(q);
          });
        } }) : "";
        return h(
          "div",
          { class: "plan-page me-page" },
          title,
          h("h2", {}, t("me.who")),
          h("p", { class: "muted small" }, t("me.who_hint")),
          people.length ? [find, h("div", { class: "me-picks" }, picks)] : h("p", { class: "muted" }, t("me.nobody"))
        );
      }
      const mine = events.flatMap((ev) => showTasksOf(ev).filter((tk) => slotsAt(ev, tk).items.some((x) => x.kind === "person" && x.id === me4.id)).map((tk) => [ev, tk]));
      const level = (ev) => (ans[me4.id] || {})[ev.id] || "";
      let saving = Promise.resolve();
      const answerRow = (ev) => {
        const btns = ["good", "poor", "never"].map((l) => h("button", {
          type: "button",
          class: "me-ans " + l,
          "data-level": l,
          "aria-pressed": String(level(ev) === l),
          "aria-label": t("me.answer_aria", { label: label(ev), answer: t("me." + l) })
        }, icon(LEVEL_ICON[l]), h("span", {}, t("me." + l))));
        const paint = () => btns.forEach((b) => b.setAttribute("aria-pressed", String(level(ev) === b.dataset.level)));
        btns.forEach((b) => b.addEventListener("click", async () => {
          const was = level(ev);
          const next = was === b.dataset.level ? "" : b.dataset.level;
          (ans[me4.id] ||= {})[ev.id] = next;
          paint();
          saving = saving.then(async () => {
            if (!await write(() => api("PUT", `${pbase}/people/${me4.id}/availability`, { levels: { [ev.id]: next } }))) {
              ans[me4.id][ev.id] = was;
              paint();
            }
          });
        }));
        let saved = (notes[me4.id] || {})[ev.id] || "";
        const note = h("input", { type: "text", class: "me-note", maxlength: "300", value: saved, placeholder: t("note.placeholder"), "aria-label": t("note.aria", { name: me4.name, label: label(ev) }) });
        note.addEventListener("change", async () => {
          const value = note.value.trim();
          if (value === saved) return;
          saving = saving.then(async () => {
            if (await write(() => api("PUT", `${pbase}/events/${ev.id}/notes/${me4.id}`, { note: value }))) {
              saved = value;
              (notes[me4.id] ||= {})[ev.id] = value;
              say(t("me.note_saved"));
            }
          });
        });
        const tasksHere = mine.filter(([e]) => e.id === ev.id).map(([, tk]) => tk.name);
        const details = fields.fields.map((f) => fieldValue[ev.id + "/" + f.id]).filter(Boolean).join(" · ");
        return h(
          "li",
          { class: "me-ev" + (tasksHere.length ? " busy" : ""), "data-event": String(ev.id) },
          h(
            "div",
            { class: "me-when" },
            h("span", { class: "d" }, fmtDate(ev.date)),
            clock(ev) ? h("span", { class: "tm" }, clock(ev)) : "",
            tasksHere.length ? h("span", { class: "me-task" }, t("me.you_have", { tasks: tasksHere.join(", ") })) : "",
            details ? h("span", { class: "small muted" }, details) : ""
          ),
          h("div", { class: "me-btns", role: "group", "aria-label": label(ev) }, btns),
          note
        );
      };
      return h(
        "div",
        { class: "plan-page me-page" },
        title,
        h("div", { class: "me-head" }, h("h2", {}, me4.name), linkBtn(t("me.switch"), () => {
          store.set(meKey, "");
          render();
        })),
        h(
          "section",
          { class: "me-mine" },
          h("h3", {}, t("me.mine")),
          mine.length ? h("ul", { class: "plain" }, mine.map(([ev, tk]) => h("li", {}, h("b", {}, fmtShortDate(ev.date)), clock(ev) ? " " + clock(ev) : "", " · ", tk.name))) : h("p", { class: "muted small" }, done ? t("me.no_tasks") : t("me.no_plan"))
        ),
        h("h3", {}, t("me.when")),
        events.length ? h("ul", { class: "plain me-list" }, events.map(answerRow)) : h("p", { class: "muted small" }, t("show.no_events"))
      );
    }
    function showPage() {
      const showTasks = tasks.filter((tk) => events.some((ev) => placesAt[at(ev, tk)] || cellsAt[at(ev, tk)]));
      const showFields = fields.fields.filter((f) => events.some((ev) => fieldValue[ev.id + "/" + f.id]));
      const has = (ev, tk) => !!(placesAt[at(ev, tk)] || cellsAt[at(ev, tk)]);
      const value = (ev, f) => fieldValue[ev.id + "/" + f.id] || "";
      const names = (ev, tk) => {
        const s = slotsAt(ev, tk);
        return h(
          "span",
          { class: "names" },
          s.items.map((x) => h("span", { class: "nm", "data-person": x.kind === "person" ? String(x.id) : null }, itemText(x))),
          s.open ? h("span", { class: "open" }, tn("show.open", s.open)) : ""
        );
      };
      const hasTask = (ev, id) => showTasks.some((tk) => has(ev, tk) && slotsAt(ev, tk).items.some((x) => x.kind === "person" && x.id === id));
      const when = (ev) => [h("span", { class: "d" }, fmtShortDate(ev.date)), clock(ev) ? h("span", { class: "tm" }, clock(ev)) : ""];
      const findKey = "find:" + key, acrossKey = "across:" + key, onlyKey = "only:" + key;
      let across = store.get(acrossKey) === "1";
      let only = store.get(onlyKey) === "1";
      let found = 0;
      const down = (evs) => h(
        "table",
        { class: "show-table" },
        h("thead", {}, h(
          "tr",
          {},
          h("th", { scope: "col" }, t("show.date")),
          showFields.map((f) => h("th", { scope: "col" }, f.name)),
          showTasks.map((tk) => h("th", { scope: "col" }, tk.name))
        )),
        h("tbody", {}, evs.map((ev) => h(
          "tr",
          { "data-ev": String(ev.id) },
          h("th", { scope: "row" }, when(ev)),
          showFields.map((f) => h("td", { class: "detail" }, value(ev, f))),
          showTasks.map((tk) => h("td", {}, has(ev, tk) ? names(ev, tk) : ""))
        )))
      );
      const acrossTable = (evs) => h(
        "table",
        { class: "show-table across" },
        h("thead", {}, h("tr", {}, h("th", { scope: "col" }, ""), evs.map((ev) => h("th", { scope: "col", "data-ev": String(ev.id) }, when(ev))))),
        h(
          "tbody",
          {},
          showFields.map((f) => h("tr", {}, h("th", { scope: "row" }, f.name), evs.map((ev) => h("td", { class: "detail", "data-ev": String(ev.id) }, value(ev, f))))),
          showTasks.map((tk) => h("tr", {}, h("th", { scope: "row" }, tk.name), evs.map((ev) => h("td", { "data-ev": String(ev.id) }, has(ev, tk) ? names(ev, tk) : ""))))
        )
      );
      const cards = (evs) => h("div", { class: "show-cards" }, evs.map((ev) => h(
        "section",
        { class: "show-card", "data-ev": String(ev.id) },
        h("h2", {}, fmtDate(ev.date), clock(ev) ? h("span", { class: "tm" }, clock(ev)) : ""),
        showFields.filter((f) => value(ev, f)).map((f) => h("p", { class: "detail" }, h("span", { class: "lbl" }, f.name), " ", value(ev, f))),
        h("dl", {}, showTasks.filter((tk) => has(ev, tk)).map((tk) => h("div", {}, h("dt", {}, tk.name), h("dd", {}, names(ev, tk)))))
      )));
      const body = h("div", { class: "show-body" });
      const count = h("span", { class: "small found", "aria-live": "polite" });
      const onlyBox = h("input", { type: "checkbox", checked: only, onchange: (e) => {
        only = e.target.checked;
        store.set(onlyKey, only ? "1" : "");
        draw();
      } });
      const onlyLabel = h("span", {});
      const onlyField = h("label", { class: "check only" }, onlyBox, onlyLabel);
      const acrossBtn = h(
        "button",
        {
          type: "button",
          class: "ghost across-btn",
          "aria-pressed": String(across),
          title: t("show.across_hint"),
          onclick: () => {
            across = !across;
            store.set(acrossKey, across ? "1" : "");
            acrossBtn.setAttribute("aria-pressed", String(across));
            draw();
          }
        },
        icon("check"),
        " " + t("show.across")
      );
      function draw() {
        const evs = found && only ? events.filter((ev) => hasTask(ev, found)) : events;
        body.replaceChildren(h("div", { class: "show-scroll" }, (across ? acrossTable : down)(evs)), cards(evs));
        body.querySelectorAll(".nm[data-person]").forEach((x) => x.classList.toggle("hit", !!found && x.dataset.person === String(found)));
        const hits = new Set(events.filter((ev) => found && hasTask(ev, found)).map((ev) => String(ev.id)));
        body.querySelectorAll("[data-ev]").forEach((x) => x.classList.toggle("miss", !!found && !hits.has(x.dataset.ev)));
        count.textContent = found ? tn("show.found", hits.size) : "";
        const p2 = people.find((x) => x.id === found);
        onlyField.hidden = !p2;
        onlyLabel.textContent = p2 ? " " + t("show.only", { name: p2.name }) : "";
      }
      const sorted = [...people].sort((a, b) => a.name.localeCompare(b.name, locale()));
      const input = h("input", {
        type: "search",
        class: "find",
        placeholder: t("show.find"),
        "aria-label": t("show.find"),
        autocomplete: "off",
        role: "combobox",
        "aria-autocomplete": "list",
        "aria-expanded": "false",
        "aria-controls": "find-list"
      });
      const list = h("ul", { class: "find-list", id: "find-list", role: "listbox", hidden: true });
      let active = -1;
      const options = () => [...list.querySelectorAll("li[role=option]")];
      const close = () => {
        list.hidden = true;
        input.setAttribute("aria-expanded", "false");
        active = -1;
      };
      const pick = (p2) => {
        found = p2 ? p2.id : 0;
        input.value = p2 ? p2.name : "";
        store.set(findKey, found ? String(found) : "");
        close();
        draw();
      };
      const activate = (i) => {
        active = i;
        options().forEach((o, j) => o.setAttribute("aria-selected", String(j === i)));
        if (options()[i]) {
          options()[i].scrollIntoView({ block: "nearest" });
          input.setAttribute("aria-activedescendant", options()[i].id);
        }
      };
      const open = () => {
        const q = input.value.trim().toLowerCase();
        const hits = sorted.filter((p2) => p2.name.toLowerCase().includes(q)).sort((a, b) => Number(!a.name.toLowerCase().startsWith(q)) - Number(!b.name.toLowerCase().startsWith(q)));
        list.replaceChildren(...hits.length ? hits.map((p2, i) => h("li", {
          role: "option",
          id: "find-" + i,
          "aria-selected": "false",
          onmousedown: (e) => {
            e.preventDefault();
            pick(p2);
          }
        }, p2.name)) : [h("li", { class: "none" }, t("show.find_none"))]);
        list.hidden = false;
        input.setAttribute("aria-expanded", "true");
        if (q && hits.length) activate(0);
        else active = -1;
      };
      input.addEventListener("focus", () => {
        input.select();
        open();
      });
      input.addEventListener("input", () => {
        if (!input.value.trim()) {
          pick(null);
          input.focus();
        }
        open();
      });
      input.addEventListener("blur", () => {
        close();
        const p2 = people.find((x) => x.id === found);
        input.value = p2 ? p2.name : "";
      });
      input.addEventListener("keydown", (e) => {
        const n = options().length;
        if (e.key === "ArrowDown") {
          e.preventDefault();
          if (list.hidden) open();
          activate(Math.min(n - 1, active + 1));
        } else if (e.key === "ArrowUp") {
          e.preventDefault();
          activate(Math.max(0, active - 1));
        } else if (e.key === "Enter") {
          e.preventDefault();
          const o = options()[active];
          if (o) pick(sorted.find((p2) => p2.name === o.textContent));
        } else if (e.key === "Escape") {
          if (!list.hidden) {
            e.preventDefault();
            close();
          }
        }
      });
      const source = done ? t("show.source", { id: view.run.id, time: fmtTime(view.run.finished_at || view.run.started_at) }) + (view.run.pinned ? " · " + t("run.current") : "") : t("show.no_draft");
      const root2 = h(
        "div",
        { class: "plan-page show" },
        h("div", { class: "title-bar" }, back(), h("h1", {}, plan.name), infoBtn(plan.name, [t("show.info")]), modeSwitch, shareBtn()),
        events.length ? h(
          "div",
          { class: "show-tools" },
          h("div", { class: "findbox" }, input, list),
          count,
          onlyField,
          acrossBtn,
          h("span", { class: "small muted source" }, source),
          h("button", { type: "button", class: "ghost print", onclick: () => window.print() }, t("show.print"))
        ) : h("p", { class: "muted small" }, t("show.no_events")),
        events.length ? body : ""
      );
      const was = Number(store.get(findKey) || (org.level === "answer" ? store.get("me:" + key) : ""));
      const p = people.find((x) => x.id === was);
      if (p) {
        found = p.id;
        input.value = p.name;
      }
      draw();
      return root2;
    }
    const runsPath = pbase + "/runs";
    const newest = runs[0];
    const running = !!newest && newest.status === "running";
    const pin = async (id, on) => {
      const was = (runs.find((x) => x.pinned) || {}).id;
      if (!await write(() => api(on ? "PUT" : "DELETE", `${runsPath}/${id}/pin`))) return;
      undoable(t(on ? "undo.pin" : "undo.unpin", { id }), () => was ? api("PUT", `${runsPath}/${was}/pin`) : api("DELETE", `${runsPath}/${id}/pin`));
      planState.runId = id;
      await render();
      say(on ? t("run.pin_done", { id }) : t("run.unpin_done"));
    };
    const foldKey = "fold:" + r.org + "/" + r.plan;
    const folded = new Set((() => {
      try {
        return JSON.parse(store.get(foldKey) || "[]");
      } catch (e) {
        return [];
      }
    })());
    const folds = (key2, rows) => [rows].flat(Infinity).filter((x) => x instanceof Element).map((tr) => {
      tr.dataset.fold = ((tr.dataset.fold || "") + " " + key2).trim();
      return tr;
    });
    const applyFolds = (table2) => {
      table2.querySelectorAll("tr[data-fold]").forEach((tr) => {
        tr.hidden = tr.dataset.filtered === "1" || tr.dataset.adds && !planState.adds.has(tr.dataset.adds) || tr.dataset.person && !shown(Number(tr.dataset.person)) || tr.dataset.fold.split(" ").some((k) => folded.has(k));
      });
      table2.querySelectorAll("button.fold").forEach((b) => b.setAttribute("aria-expanded", String(!folded.has(b.dataset.key))));
    };
    const foldBtn = (key2, title) => h("button", {
      type: "button",
      class: "fold",
      "data-key": key2,
      "aria-expanded": String(!folded.has(key2)),
      title: t("table.fold"),
      "aria-label": t("table.fold_aria", { name: title }),
      onclick: (e) => {
        if (folded.has(key2)) folded.delete(key2);
        else folded.add(key2);
        store.set(foldKey, JSON.stringify([...folded]));
        applyFolds(e.currentTarget.closest("table"));
      }
    }, icon("caret"));
    const hideKey = "hide:" + r.org + "/" + r.plan;
    const hidden = new Set((() => {
      try {
        return JSON.parse(store.get(hideKey) || "[]");
      } catch {
        return [];
      }
    })().filter((id) => people.some((p) => p.id === id)));
    const taskKey = "whotasks:" + r.org + "/" + r.plan;
    const byTask = new Set((() => {
      try {
        return JSON.parse(store.get(taskKey) || "[]");
      } catch {
        return [];
      }
    })().filter((id) => tasks.some((tk) => tk.id === id && tk.planned)));
    const takes = (p) => (chosen[p.id] || []).some((x) => byTask.has(x.task_id) && (x.level === "good" || x.level === "poor"));
    const shown = (id) => {
      const p = people.find((x) => x.id === id);
      return !p || !hidden.has(id) && (!byTask.size || takes(p));
    };
    const filtering = () => hidden.size > 0 || byTask.size > 0;
    const whoLabel = () => {
      const n = people.filter((p) => shown(p.id)).length;
      const of = t("who.shown", { shown: n, all: people.length });
      if (!byTask.size) return hidden.size ? of : t("who.all");
      return tasks.filter((tk) => byTask.has(tk.id)).map((tk) => tk.name).join(", ") + " · " + of;
    };
    const whoBtn = h(
      "button",
      { type: "button", class: "who" + (filtering() ? " on" : ""), title: t("who.label"), onclick: () => whoDialog() },
      icon("filter"),
      h("span", { class: "lbl" }, whoLabel())
    );
    const whoSave = () => {
      store.set(hideKey, JSON.stringify([...hidden]));
      store.set(taskKey, JSON.stringify([...byTask]));
      whoBtn.querySelector(".lbl").textContent = whoLabel();
      whoBtn.classList.toggle("on", filtering());
      applyFolds(table.querySelector("table"));
    };
    const whoDialog = () => {
      const boxes = people.map((p) => h("input", { type: "checkbox", checked: !hidden.has(p.id), onchange: (e) => {
        if (e.target.checked) hidden.delete(p.id);
        else hidden.add(p.id);
        whoSave();
      } }));
      const items = people.map((p, i) => h("label", { class: "who-item", "data-name": p.name.toLowerCase() }, boxes[i], h("span", {}, p.name)));
      const find = h("input", { type: "search", class: "wide", placeholder: t("table.filter"), "aria-label": t("table.filter"), oninput: (e) => {
        const q = e.target.value.trim().toLowerCase();
        items.forEach((it) => {
          it.hidden = !!q && !it.dataset.name.includes(q);
        });
      } });
      const all = (show) => () => {
        people.forEach((p, i) => {
          if (!items[i].hidden) {
            if (show) hidden.delete(p.id);
            else hidden.add(p.id);
            boxes[i].checked = show;
          }
        });
        whoSave();
      };
      const planned = tasks.filter((tk) => tk.planned);
      const taskChips = planned.map((tk) => h("label", { class: "chip who-task" }, h("input", { type: "checkbox", checked: byTask.has(tk.id), onchange: (e) => {
        if (e.target.checked) byTask.add(tk.id);
        else byTask.delete(tk.id);
        whoSave();
      } }), " " + tk.name));
      modal(
        t("who.label"),
        { info: [t("who.info")] },
        planned.length ? [h("h3", {}, t("who.tasks")), h("p", { class: "muted small" }, t("who.tasks_hint")), h("div", { class: "who-tasks" }, taskChips), h("h3", {}, t("who.people"))] : "",
        h(
          "div",
          { class: "who-head" },
          find,
          h("button", { type: "button", class: "ghost", onclick: all(true) }, t("who.select_all")),
          h("button", { type: "button", class: "ghost", onclick: all(false) }, t("who.select_none"))
        ),
        h("div", { class: "who-list" }, items),
        foot("", closeBtn())
      );
    };
    const tagsBtn = h("button", {
      type: "button",
      class: "tags-btn",
      "aria-pressed": String(store.get("tags") === "1"),
      title: t("tags.hint"),
      onclick: (e) => {
        const on = store.get("tags") !== "1";
        store.set("tags", on ? "1" : "");
        e.currentTarget.setAttribute("aria-pressed", String(on));
        e.currentTarget.closest("table").classList.toggle("show-tags", on);
      }
    }, icon("check"), h("span", { class: "lbl" }, t("tags.show")));
    let filterText = "";
    const filterBox = () => h("input", {
      type: "search",
      class: "filter",
      value: filterText,
      placeholder: t("table.filter"),
      "aria-label": t("table.filter"),
      oninput: (e) => {
        filterText = e.target.value.trim().toLowerCase();
        const table2 = e.target.closest("table");
        table2.querySelectorAll("tr.person-row").forEach((tr) => {
          tr.dataset.filtered = filterText && !tr.dataset.name.includes(filterText) ? "1" : "";
        });
        applyFolds(table2);
      }
    });
    const bar = h("div", { class: "draft-bar" });
    const poll = async () => {
      if (!bar.isConnected) return;
      try {
        const latest = await api("GET", runsPath + "/latest");
        if (latest.run.status === "running") setTimeout(poll, live2 && live2.es.readyState === 1 ? 5e3 : 1e3);
        else {
          planState.runId = latest.run.id;
          forget(changedBy("RunFinished"), call("GET", runsPath).org);
          render();
        }
      } catch (e) {
        say(errorText(e), "err");
      }
    };
    {
      const limits = [[10, "10 s"], [20, "20 s"], [60, "60 s"]];
      const limitSel = h("select", { "aria-label": t("run.limit_aria") }, limits.map(([v, text]) => h("option", { value: String(v), selected: String(v) === (store.get("limit") || "20") }, text)));
      limitSel.addEventListener("change", () => store.set("limit", limitSel.value));
      const start = async () => {
        let id;
        if (await write(async () => {
          id = (await api("POST", runsPath, { time_limit: parseInt(limitSel.value, 10) })).id;
        })) {
          planState.runId = id;
          render();
        }
      };
      const nothing = !events.length || !people.length || !Object.values(placesAt).some(Boolean);
      const line = [h(
        "button",
        { type: "button", disabled: !health.planner || running || nothing, title: nothing && !running ? t("run.nothing") : null, onclick: start },
        running ? t("run.planning") : view ? t("run.retry") : t("run.plan")
      ), limitSel];
      const notes2 = [];
      if (!health.planner) line.push(h("span", { class: "small" }, t("run.no_planner")));
      if (running) {
        line.push(h("span", { role: "status" }, t("run.running", { n: newest.time_limit })));
        setTimeout(poll, live2 && live2.es.readyState === 1 ? 5e3 : 1e3);
      }
      if (!view) {
        if (!running) line.push(h("span", { class: "small" }, plan.draft_aside && runs.length ? t("run.aside") : t("run.none_yet")));
      } else if (view.run.status === "failed") notes2.push(h("p", { class: "err", role: "alert" }, t("run.failed", { error: view.run.error || t("run.unknown_error") })));
      else if (done) {
        const run = view.run;
        const about3 = t("run.label", { id: run.id, time: fmtTime(run.finished_at || run.started_at) }) + " · " + (run.solver_status === "OPTIMAL" ? t("run.optimal") : t("run.feasible"));
        const step = (i, aria, ic) => h("button", {
          type: "button",
          class: "ghost icon step",
          disabled: !runs[i],
          "aria-label": aria,
          title: aria,
          onclick: () => {
            planState.runId = runs[i].id;
            render();
          }
        }, icon(ic));
        line.push(h(
          "span",
          { class: "steps" },
          step(shownAt + 1, t("run.older"), "left"),
          h("span", { class: "small run-id", title: about3 }, "#" + run.id, run.solver_status === "OPTIMAL" ? "" : h("span", { class: "warn" }, " ⚠")),
          step(shownAt - 1, t("run.newer"), "right")
        ));
        if (olderId) line.push(h("button", {
          type: "button",
          class: "ghost sm changes" + (comparing ? " on" : ""),
          "aria-pressed": String(comparing),
          title: t("run.changes_hint", { id: olderId }),
          onclick: () => {
            store.set(changesKey, comparing ? "" : "1");
            render();
          }
        }, t("run.changes")));
        line.push(h("button", {
          type: "button",
          class: "ghost icon pin" + (run.pinned ? " on" : ""),
          "aria-pressed": String(!!run.pinned),
          title: run.pinned ? t("run.current") + " · " + t("run.unpin") : t("run.pin"),
          "aria-label": run.pinned ? t("run.unpin") : t("run.pin"),
          onclick: () => pin(run.id, !run.pinned)
        }, icon("pin")));
        if (run.id !== newest.id) line.push(linkBtn(t("run.show_newest"), () => {
          planState.runId = newest.id;
          render();
        }));
        if (org.level !== "view") {
          const toLock = lockItems(everyPlace(), true);
          if (run.pinned && !toLock.length) {
            line.push(h("span", { class: "done-mark", role: "status" }, icon("check"), " " + t("draft.is_done")));
            line.push(h("button", { type: "button", class: "ghost sm", title: t("draft.reopen_hint"), onclick: async () => {
              if (await write(() => api("DELETE", `${runsPath}/${run.id}/pin`))) {
                undoable(t("undo.reopen"), () => api("PUT", `${runsPath}/${run.id}/pin`));
                planState.runId = run.id;
                await render();
                say(t("draft.reopened"));
              }
            } }, t("draft.reopen")));
          } else {
            line.push(h("button", { type: "button", class: "done", disabled: running, title: t("draft.done_hint"), onclick: async () => {
              const was = (runs.find((x) => x.pinned) || {}).id;
              if (toLock.length && !await write(() => api("PUT", pbase + "/locks", { locks: toLock, locked: true }))) return;
              if (!run.pinned && !await write(() => api("PUT", `${runsPath}/${run.id}/pin`))) return;
              undoable(t("undo.done_mark"), async () => {
                if (toLock.length) await api("PUT", pbase + "/locks", { locks: toLock, locked: false });
                if (!run.pinned) await (was ? api("PUT", `${runsPath}/${was}/pin`) : api("DELETE", `${runsPath}/${run.id}/pin`));
              });
              planState.runId = run.id;
              store.set(modeKey, "show");
              await render();
              say(t("draft.done_said"));
            } }, icon("check"), " " + t("draft.done")));
          }
          line.push(h("button", { type: "button", class: "ghost sm", disabled: running, title: t("draft.aside_hint"), onclick: async () => {
            if (!await ask(t("draft.aside"), t("draft.aside_confirm"), t("draft.aside_yes"))) return;
            const was = (runs.find((x) => x.pinned) || {}).id;
            if (await write(() => api("DELETE", pbase + "/draft"))) {
              undoable(t("undo.aside"), () => was ? api("PUT", `${runsPath}/${was}/pin`) : api("PUT", pbase + "/draft"));
              planState.runId = null;
              await render();
              say(t("draft.aside_done"));
            }
          } }, t("draft.aside")));
        }
        if (run.stale) notes2.push(h("p", { class: "warn small", role: "status" }, t("run.stale")));
        if (run.checked && run.violations.length) notes2.push(h("div", { class: "err", role: "alert" }, h("p", {}, t("run.violations")), h("ul", {}, run.violations.map((v) => h("li", {}, v)))));
        if (view.warnings.length) notes2.push(h("ul", { class: "warn small" }, view.warnings.map((w) => h("li", {}, w))));
      }
      if (org.level !== "view" && planState.undo.length) {
        const last = planState.undo[planState.undo.length - 1];
        line.push(h(
          "button",
          { type: "button", class: "ghost sm undo", title: t("undo.title", { what: last.label }) + " (Ctrl+Z)", "aria-label": t("undo.title", { what: last.label }), onclick: undoLast },
          icon("undo"),
          " " + t("undo.btn")
        ));
      }
      const key2 = (m, text) => h("span", { class: "key" }, m, h("span", {}, text));
      bar.replaceChildren(h("div", { class: "row-inline" }, line), ...notes2);
      bar.prepend(h(
        "span",
        { class: "bar-title" },
        foldBtn("draft", t("draft.title")),
        t("draft.title"),
        infoBtn(t("draft.title"), [t("draft.info"), key2(mark("hand"), t("draft.key_hand_info")), key2(mark("lock"), t("draft.key_locked_info"))])
      ));
    }
    function eventDialog(ev) {
      let dlg;
      const start = h("input", { type: "time", value: ev.start_time, "aria-label": t("event.starts") });
      const end = h("input", { type: "time", value: ev.end_time, "aria-label": t("event.ends") });
      const values = fields.fields.map((f) => ({ f, input: h("input", { type: "text", maxlength: "200", value: fieldValue[ev.id + "/" + f.id] || "", "aria-label": f.name }) }));
      dlg = modal(
        fmtDate(ev.date),
        { info: [t("event.planner_ignores")] },
        h(
          "form",
          { class: "stack", onsubmit: async (e) => {
            e.preventDefault();
            const body = {};
            const endWanted = start.value ? end.value : "";
            if (start.value !== ev.start_time) body.start_time = start.value;
            if (endWanted !== ev.end_time) body.end_time = endWanted;
            const todo = values.filter(({ f, input }) => input.value.trim() !== (fieldValue[ev.id + "/" + f.id] || ""));
            if (!Object.keys(body).length && !todo.length) {
              dlg.close();
              return;
            }
            if (await write(async () => {
              if (Object.keys(body).length) await api("PATCH", `${epath}/${ev.id}`, body);
              for (const { f, input } of todo) await api("PUT", `${epath}/${ev.id}/fields/${f.id}`, { value: input.value });
            })) {
              dlg.close();
              render();
            }
          } },
          h("div", { class: "row-inline" }, field(t("event.starts"), start), field(t("event.ends"), end)),
          values.map(({ f, input }) => field(f.name, input)),
          foot(
            h("button", { type: "button", class: "danger", onclick: async () => {
              if (await write(async () => {
                await confirmed(t("event.delete"), t("event.delete_confirm", { date: fmtDate(ev.date) }), t("common.delete"));
                await api("DELETE", `${epath}/${ev.id}`);
              })) {
                dlg.close();
                render();
              }
            } }, t("event.delete")),
            h("button", { type: "button", class: "ghost", onclick: () => {
              dlg.close();
              copyDialog(ev);
            } }, icon("copy"), " " + t("event.copy")),
            h("button", { type: "submit" }, t("common.save"))
          )
        )
      );
    }
    function repeat(dates, onEvery, preview) {
      const everyText = (n) => n === 1 ? t("copy.every_1") : n === 2 ? t("copy.every_2") : t("copy.every_n", { n });
      const sel2 = h(
        "select",
        { "aria-label": t("copy.repeat") },
        h("option", { value: "0" }, t("copy.once")),
        Array.from({ length: MAX_EVERY }, (_, i) => h("option", { value: String(i + 1) }, everyText(i + 1)))
      );
      const until = h("input", { type: "date", disabled: true, "aria-label": t("copy.until") });
      const count = h("span", { class: "small muted", "aria-live": "polite" });
      const every = () => Number(sel2.value);
      let untilField;
      const update = () => {
        until.disabled = !every();
        untilField.hidden = !every();
        until.required = !!every();
        const all = repeatDates(dates(), until.value, every()).sort();
        const n = all.length;
        count.textContent = n && !preview ? "→ " + tn("copy.count", n) : "";
        if (preview) preview(all);
      };
      sel2.addEventListener("change", () => {
        if (onEvery) onEvery(every());
        update();
      });
      until.addEventListener("input", update);
      return {
        el: h("div", { class: "row-inline repeat" }, field(t("copy.repeat"), sel2), untilField = field(t("copy.until"), until), count),
        until: () => every() ? until.value : "",
        every,
        update
      };
    }
    function copyDialog(ev) {
      let dlg;
      const inputs = [];
      const list = h("div", { class: "dates" });
      const dates = () => inputs.map((i) => i.value).filter(Boolean);
      const taken = new Set(events.map((x) => x.date));
      const free = (d, step) => {
        for (let i = 0; i < 60 && taken.has(d); i++) d = addDays(d, step);
        return d;
      };
      let suggested = true;
      const shown2 = h("ul", { class: "copy-dates", "aria-label": t("copy.made") });
      const submit = h("button", { type: "submit" }, t("copy.button"));
      const preview = (all) => {
        const max = 14;
        shown2.replaceChildren(
          ...all.slice(0, max).map((d) => h(
            "li",
            { class: taken.has(d) ? "taken" : "", title: taken.has(d) ? t("copy.taken") : "" },
            fmtShortDate(d),
            taken.has(d) ? h("span", { class: "small" }, " · " + t("copy.taken_short")) : ""
          )),
          all.length > max ? h("li", { class: "more" }, t("copy.more", { n: all.length - max })) : ""
        );
        shown2.hidden = !all.length;
        submit.textContent = all.length ? tn("copy.make", all.length) : t("copy.button");
        submit.disabled = !all.length;
      };
      const week = repeat(dates, (n) => {
        if (suggested) inputs[0].value = free(addDays(ev.date, 7 * Math.max(n, 1)), 7 * Math.max(n, 1));
      }, preview);
      const addDate = (v) => {
        const i = h("input", { type: "date", value: v || "", "aria-label": t("copy.date") });
        i.addEventListener("input", week.update);
        inputs.push(i);
        list.append(i);
        return i;
      };
      addDate(free(addDays(ev.date, 7), 7)).addEventListener("input", () => {
        suggested = false;
      });
      const check = (on, text, detail) => {
        const box = h("input", { type: "checkbox", checked: on, "aria-label": text });
        return { box, el: h("label", { class: "check" }, box, " " + text, detail ? h("span", { class: "muted small" }, " " + detail) : "") };
      };
      const details = fields.fields.filter((f) => fieldValue[ev.id + "/" + f.id]).map((f) => f.name);
      const placeText = tasks.filter((tk) => placesAt[at(ev, tk)]).map((tk) => `${tk.name} ${placesAt[at(ev, tk)]}`).join(", ");
      const times = check(true, t("copy.times"), clock(ev) || t("copy.none"));
      const det = check(true, t("copy.details"), details.join(", ") || t("copy.none"));
      const pl = check(true, t("copy.places"), placeText || t("copy.none"));
      const hand = check(false, t("copy.hand"));
      dlg = modal(
        t("copy.title_of", { label: fmtShortDate(ev.date) }),
        { info: [t("copy.never")] },
        h(
          "form",
          { class: "stack", onsubmit: async (e) => {
            e.preventDefault();
            let made;
            if (await write(async () => {
              made = await api("POST", `${epath}/${ev.id}/copy`, { dates: dates(), until: week.until(), every: week.every(), times: times.box.checked, details: det.box.checked, places: pl.box.checked, hand: hand.box.checked });
            })) {
              dlg.close();
              await render();
              say(tn("copy.done", made.length));
            }
          } },
          field(t("copy.date"), list),
          h("div", {}, linkBtn("+ " + t("copy.another_date"), () => {
            addDate("").focus();
            week.update();
          })),
          week.el,
          shown2,
          h("h3", {}, t("copy.comes_along")),
          times.el,
          det.el,
          pl.el,
          hand.el,
          foot("", submit)
        )
      );
      week.update();
    }
    function newEventDialog() {
      let dlg;
      const last = events[events.length - 1];
      const date = h("input", { type: "date", required: true, value: last ? addDays(last.date, 7) : "", "aria-label": t("event.date") });
      const from = h(
        "select",
        { "aria-label": t("event.start_from") },
        h("option", { value: "" }, t("event.empty")),
        [...events].reverse().map((ev) => h("option", { value: String(ev.id), selected: !!last && ev.id === last.id }, t("event.copy_of", { label: label(ev) })))
      );
      const start = h("input", { type: "time", "aria-label": t("event.starts") });
      const end = h("input", { type: "time", "aria-label": t("event.ends") });
      let suggested = !!last;
      const week = repeat(() => [date.value], (n) => {
        if (suggested) date.value = addDays(last.date, 7 * Math.max(n, 1));
      });
      date.addEventListener("input", () => {
        suggested = false;
        week.update();
      });
      const timesHint = h("p", { class: "muted small" });
      const hint = () => {
        timesHint.textContent = from.value ? t("event.times_from_copy") : "";
        timesHint.hidden = !from.value;
      };
      from.addEventListener("change", hint);
      dlg = modal(
        t("event.new"),
        h(
          "form",
          { class: "stack", onsubmit: async (e) => {
            e.preventDefault();
            let made;
            const times = { start_time: start.value, end_time: start.value ? end.value : "" };
            if (await write(async () => {
              made = from.value ? await api("POST", `${epath}/${from.value}/copy`, { dates: [date.value], until: week.until(), every: week.every(), times: !start.value, ...times, details: true, places: true, hand: false }) : await api("POST", epath, { date: date.value, until: week.until(), every: week.every(), ...times });
            })) {
              dlg.close();
              await render();
              say(tn("event.added", made.length));
            }
          } },
          h("div", { class: "row-inline" }, field(t("event.date"), date), events.length ? field(t("event.start_from"), from) : ""),
          h("div", { class: "row-inline" }, field(t("event.starts"), start), field(t("event.ends"), end)),
          timesHint,
          week.el,
          foot("", h("button", { type: "submit" }, t("common.add")))
        )
      );
      hint();
      week.update();
      date.focus();
    }
    function taskDialog(tk) {
      let dlg;
      const changed = () => render();
      const name = h("input", { type: "text", value: tk.name, maxlength: "80", "aria-label": t("common.name") });
      const others = tasks.filter((x) => x.id !== tk.id);
      const levelWith = (o) => (combos.find((c) => c.a === Math.min(tk.id, o.id) && c.b === Math.max(tk.id, o.id)) || { level: "standard" }).level;
      const alone = others.length > 0 && others.every((o) => levelWith(o) === "cannot");
      const aloneBox = h("input", { type: "checkbox", checked: alone, disabled: !others.length, "aria-label": t("task.alone") });
      aloneBox.addEventListener("change", async () => {
        const on = aloneBox.checked;
        if (await write(async () => {
          if (!on) await confirmed(t("task.alone"), t("task.alone_off_confirm", { name: tk.name }), t("task.alone_off_yes"));
          await api("PUT", `${base}/tasks/${tk.id}/combinations`, { level: on ? "cannot" : "standard" });
        })) changed();
        else aloneBox.checked = !on;
      });
      const pairRows = others.map((o) => h("tr", {}, h("td", {}, o.name), h("td", {}, cycle("combo", levelWith(o), `${tk.name} + ${o.name}`, async (next) => {
        const ok = await write(() => api("PUT", base + "/combinations", { a: tk.id, b: o.id, level: next }));
        if (ok) changed();
        return ok;
      }))));
      const mates = tasksIn(tk.section).filter((x) => x.id !== tk.id);
      const now = tasksIn(tk.section).findIndex((x) => x.id === tk.id);
      const where = h(
        "select",
        { "aria-label": t("task.where") },
        h("option", { value: "0", selected: now === 0 }, t("field.at_top")),
        mates.map((x, i) => h("option", { value: String(i + 1), selected: now === i + 1 }, t("field.after", { name: x.name })))
      );
      where.addEventListener("change", async () => {
        if (await write(() => api("PUT", `${base}/tasks/${tk.id}/position`, { position: parseInt(where.value, 10) }))) changed();
        else where.value = String(now);
      });
      const sectionSel = h(
        "select",
        { "aria-label": t("task.section") },
        h("option", { value: "0", selected: !tk.section }, t("task.no_section")),
        taskSections.map((x) => h("option", { value: String(x.id), selected: tk.section === x.id }, x.name))
      );
      sectionSel.addEventListener("change", async () => {
        if (await write(() => api("PUT", `${base}/tasks/${tk.id}/position`, { section: parseInt(sectionSel.value, 10), position: tasks.length }))) changed();
        else sectionSel.value = String(tk.section);
      });
      dlg = keyed(modal(
        tk.name,
        { info: [t("task.org_wide", { org: org.name }), t("task.auto_hint"), t("task.combine_hint")] },
        h("form", { class: "row", onsubmit: async (e) => {
          e.preventDefault();
          if (name.value.trim() !== tk.name && await write(() => api("PATCH", `${base}/tasks/${tk.id}`, { name: name.value }))) changed();
        } }, name, h("button", { type: "submit" }, t("common.rename"))),
        mates.length ? field(t("task.where"), where) : "",
        taskSections.length ? field(t("task.section"), sectionSel) : "",
        h("h3", {}, t("task.combine")),
        h("label", { class: "check" }, aloneBox, " ", h("strong", {}, t("task.alone"))),
        pairRows.length ? h("table", { class: "mini" }, h("tbody", {}, pairRows)) : "",
        foot(h("button", { type: "button", class: "danger", onclick: async () => {
          if (await write(async () => {
            await confirmed(t("task.delete"), t("task.delete_confirm", { name: tk.name }), t("common.delete"));
            await api("DELETE", `${base}/tasks/${tk.id}`);
          })) {
            dlg.close();
            render();
          }
        } }, t("task.delete")), closeBtn())
      ), "task", tk.id);
    }
    function personDialog(p) {
      let dlg;
      const changed = () => render();
      const ppath = `${base}/people/${p.id}`;
      const name = h("input", { type: "text", value: p.name, maxlength: "80", "aria-label": t("common.name") });
      const sectionSel = h(
        "select",
        { "aria-label": t("person.section") },
        h("option", { value: "0", selected: !p.section }, t("person.no_section")),
        sections.map((x) => h("option", { value: String(x.id), selected: p.section === x.id }, x.name))
      );
      sectionSel.addEventListener("change", async () => {
        if (await write(() => api("PUT", `${ppath}/position`, { section: parseInt(sectionSel.value, 10), position: people.length }))) changed();
        else sectionSel.value = String(p.section);
      });
      const levelOf = (tk) => ((chosen[p.id] || []).find((x) => x.task_id === tk.id) || { level: "" }).level;
      const taskRows = tasks.filter((tk) => tk.planned).map((tk) => {
        const cur = levelOf(tk);
        return h("tr", {}, h("td", { class: cur ? "" : "muted" }, tk.name), h("td", {}, cycle("task", cur, t("person.task_aria", { name: p.name, task: tk.name }), async (next) => {
          const ok = await write(() => next ? api("PUT", `${ppath}/tasks/${tk.id}`, { level: next }) : api("DELETE", `${ppath}/tasks/${tk.id}`));
          if (ok) changed();
          return ok;
        })));
      });
      dlg = keyed(modal(
        p.name,
        { info: [t("person.tasks_hint"), t("person.auto_hint"), t("person.org_wide", { org: org.name })] },
        h("form", { class: "row", onsubmit: async (e) => {
          e.preventDefault();
          if (name.value.trim() !== p.name && await write(() => api("PATCH", ppath, { name: name.value }))) changed();
        } }, name, h("button", { type: "submit" }, t("common.rename"))),
        h("h3", {}, t("person.tasks")),
        taskRows.length ? h("table", { class: "mini" }, h("tbody", {}, taskRows)) : h("p", { class: "muted" }, t("person.no_tasks")),
        sections.length ? field(t("person.section"), sectionSel) : "",
        foot("", closeBtn())
      ), "person", p.id);
    }
    function placeDialog(ev, tk) {
      let dlg;
      const k = at(ev, tk);
      const changed = () => render();
      const act = (fn) => async () => {
        if (await write(fn)) changed();
      };
      const s = slotsAt(ev, tk);
      const rows = s.items.map((x) => h(
        "tr",
        {},
        h("td", {}, slot(x)),
        h("td", { class: "small muted" }, x.hand ? t("place.by_hand") : t("place.from_planner")),
        h("td", { class: "right" }, x.cell ? linkBtn(x.locked ? t("place.unlock") : t("common.remove"), act(() => api("DELETE", `${pbase}/cells/${x.cell.id}`)), t(x.locked ? "place.unlock_aria" : "place.remove_aria", { name: x.name })) : h("button", {
          type: "button",
          class: "ghost sm",
          "aria-label": t("place.lock_aria", { name: x.name }),
          onclick: act(() => api("POST", pbase + "/cells", { event: ev.id, task: tk.id, person: x.id, locked: true }))
        }, t("place.lock")))
      ));
      const inCell = new Set(s.items.filter((x) => x.cell && x.kind === "person").map((x) => x.id));
      const pick = people.filter((p) => !inCell.has(p.id)).map((p) => ({ p, box: h("input", { type: "checkbox", "aria-label": t("place.person_aria", { name: p.name }) }) }));
      const peopleForm = pick.length ? h(
        "form",
        { class: "stack", onsubmit: async (e) => {
          e.preventDefault();
          const ticked = pick.filter((x) => x.box.checked);
          if (!ticked.length) return;
          if (await write(async () => {
            for (const { p } of ticked) await api("POST", pbase + "/cells", { event: ev.id, task: tk.id, person: p.id });
          })) changed();
        } },
        h("div", { class: "chips" }, pick.map(({ p, box }) => h("label", { class: "chip" }, box, " " + p.name))),
        h("div", { class: "row-inline" }, h("button", { type: "submit" }, t("place.add_people")))
      ) : "";
      const textIn = h("input", { type: "text", maxlength: "80", placeholder: t("place.text_placeholder"), "aria-label": t("common.text") });
      const fills = h("input", { type: "number", min: "1", max: "50", value: "1", "aria-label": t("place.fills") });
      const setForm = h("form", { class: "row-inline", onsubmit: async (e) => {
        e.preventDefault();
        const body = { event: ev.id, task: tk.id, label: textIn.value, fills: parseInt(fills.value, 10) || 1 };
        if (await write(() => api("POST", pbase + "/cells", body))) changed();
      } }, textIn, fills, h("button", { type: "submit", "aria-label": t("place.add") }, "+"));
      let poolPart = "";
      if (tk.planned) {
        const pool = poolAt[k] || { people: [] };
        const takes2 = (p) => (chosen[p.id] || []).some((x) => x.task_id === tk.id && x.level !== "never");
        const pBoxes = people.filter((p) => takes2(p) || pool.people.includes(p.id)).map((p) => ({ p, box: h("input", { type: "checkbox", checked: pool.people.includes(p.id), "aria-label": t("place.pool_person_aria", { name: p.name }) }) }));
        const who = pool.people.length ? pool.people.map((id) => nameOf(people, id)).join(", ") : t("place.pool_all", { task: tk.name });
        const ticks = h(
          "div",
          { class: "stack pool-ticks", hidden: true },
          pool.people.length ? h("p", { class: "muted small" }, t("place.pool_set")) : "",
          pBoxes.length ? [
            h("div", { class: "chips" }, pBoxes.map(({ p, box }) => h("label", { class: "chip" }, box, " " + p.name))),
            h("div", { class: "row-inline" }, h("button", { type: "button", class: "ghost sm", onclick: act(() => api("PUT", pbase + "/pools", {
              event: ev.id,
              task: tk.id,
              people: pBoxes.filter((x) => x.box.checked).map((x) => x.p.id)
            })) }, t("place.pool_save")))
          ] : h("p", { class: "muted small" }, t("place.pool_nobody", { task: tk.name }))
        );
        poolPart = [h(
          "p",
          { class: "pool-line small" + (pool.people.length ? " set" : "") },
          pool.people.length ? h("span", { class: "mark pool", "aria-hidden": "true" }, icon("filter")) : "",
          t("place.pool_line", { who }),
          " · ",
          linkBtn(t("place.pool_change"), () => {
            ticks.hidden = !ticks.hidden;
          }, t("place.pool_change_aria"))
        ), ticks];
      }
      const hands = s.items.filter((x) => x.hand);
      dlg = keyed(modal(
        `${tk.name} · ${fmtShortDate(ev.date)}`,
        { info: [t("place.add_people_hint"), tk.planned ? t("place.pool_anyone", { task: tk.name }) : t("place.not_planned"), t("place.lock_hint")] },
        h("p", { class: "muted small" }, tn("place.places", s.n)),
        done && s.open && s.why ? h("p", { class: "warn small" }, t("place.why", { why: s.why })) : "",
        rows.length ? h("table", { class: "mini" }, h("tbody", {}, rows)) : h("p", { class: "muted small" }, t("place.nothing")),
        h("h3", {}, t("place.set_by_hand")),
        peopleForm,
        setForm,
        poolPart,
        foot(hands.length ? h("button", { type: "button", class: "danger", onclick: act(async () => {
          await confirmed(t("place.clear"), t("place.clear_confirm", { task: tk.name, date: fmtShortDate(ev.date) }), t("place.clear_yes"));
          for (const x of hands) await api("DELETE", `${pbase}/cells/${x.cell.id}`);
        }) }, t("place.clear")) : "", closeBtn())
      ), "place", k);
    }
    function prioritiesDialog() {
      keyed(modal(
        t("weights.title"),
        { info: [t("weights.hint")] },
        prioritiesForm(pbase + "/weights", weights, (text) => {
          render().then(() => say(text));
        }),
        h("h3", {}, t("prio.always")),
        h("ul", { class: "small rules" }, ["takes", "no", "combo", "pool", "kept"].map((k) => h("li", {}, t("prio.rule." + k)))),
        weightsForm(pbase + "/weights", weights, (text) => {
          render().then(() => say(text));
        }),
        foot("", closeBtn())
      ), "priorities", 0);
    }
    function settingsDialog() {
      let dlg;
      const name = h("input", { type: "text", value: plan.name, maxlength: "80", "aria-label": t("settings.name") });
      const runRows = runs.map((x) => {
        const onShow = !!view && x.id === view.run.id;
        return h(
          "tr",
          { class: onShow ? "shown" : false },
          h("td", {}, t("run.label", { id: x.id, time: fmtTime(x.finished_at || x.started_at) })),
          h("td", {}, x.status !== "done" ? h("span", { class: "tag" }, t("run.status." + x.status)) : "", x.pinned ? h("span", { class: "chip pin" }, t("run.current")) : ""),
          h(
            "td",
            { class: "right" },
            onShow ? h("span", { class: "small muted" }, t("run.on_show")) : linkBtn(t("run.show"), () => {
              planState.runId = x.id;
              dlg.close();
              render();
            }, t("run.show_aria", { id: x.id })),
            x.status === "done" && !x.pinned ? [" · ", linkBtn(t("run.pin_short"), () => {
              dlg.close();
              pin(x.id, true);
            }, t("run.pin_aria", { id: x.id }))] : ""
          )
        );
      });
      dlg = keyed(modal(
        plan.name,
        h("form", { class: "row", onsubmit: async (e) => {
          e.preventDefault();
          if (name.value.trim() !== plan.name && await write(() => api("PATCH", pbase, { name: name.value }))) {
            dlg.close();
            await render();
            say(t("settings.renamed"));
          }
        } }, name, h("button", { type: "submit" }, t("common.rename"))),
        h("h3", {}, t("weights.title")),
        h("p", { class: "small" }, prioritySummary(weights), " · ", linkBtn(t("prio.change"), () => {
          dlg.close();
          prioritiesDialog();
        })),
        h("h3", {}, t("run.history")),
        runs.length ? h("table", { class: "mini" }, h("tbody", {}, runRows)) : h("p", { class: "muted small" }, t("run.no_history")),
        foot(h("button", { type: "button", class: "danger", onclick: async () => {
          if (await write(async () => {
            await confirmed(t("settings.delete"), t("settings.delete_confirm", { name: plan.name }), t("common.delete"));
            await api("DELETE", pbase);
          })) {
            dlg.close();
            go(`/o/${enc(r.org)}`);
            say(t("settings.deleted", { name: plan.name }));
          }
        } }, t("settings.delete")), closeBtn())
      ), "settings", 0);
    }
    const listDialog = (title, items) => modal(title, h("ul", { class: "small" }, items.map((x) => h("li", {}, x))), foot("", closeBtn()));
    const cols = events.length;
    const filler = () => h("td", { class: "addcol" });
    const rowPick = (grid, id, name) => h("td", { class: "addcol" }, h("button", {
      type: "button",
      class: "rowpick",
      title: t("sel.row"),
      "aria-label": t("sel.row_aria", { name }),
      onclick: (e) => selectLine(grid, e, (c) => c.row === id)
    }, icon("rowsel")));
    const sect = (key2, title, info) => h(
      "tr",
      { class: "sect" },
      h("th", { scope: "row" }, h("div", { class: "rl" }, foldBtn(key2, title), title, info ? infoBtn(title, info) : "")),
      h("td", { colspan: String(cols + 1) })
    );
    const addLine = (form) => h("tr", { class: "adds" }, h("th", { scope: "row" }, form), h("td", { colspan: String(cols + 1) }));
    const adds = (key2, rows, host, lines) => {
      const open = planState.adds.has(key2);
      const trs = lines.map((form) => {
        const tr = addLine(form);
        tr.hidden = !open;
        tr.classList.add("folded-add");
        tr.dataset.adds = key2;
        const input = form.querySelector("input");
        input.addEventListener("keydown", (e) => {
          if (e.key === "Escape") {
            e.preventDefault();
            e.stopPropagation();
            toggle();
          }
        });
        return tr;
      });
      const what = lines[0].querySelector("input").placeholder;
      const plus = h("button", {
        type: "button",
        class: "addplus",
        "data-adds": key2,
        "aria-expanded": String(open),
        title: open ? t("adds.close") : what,
        "aria-label": open ? t("adds.close") : what
      }, open ? "−" : "+");
      const toggle = () => {
        if (planState.adds.has(key2)) planState.adds.delete(key2);
        else planState.adds.add(key2);
        const now = planState.adds.has(key2);
        applyFolds(plus.closest("table"));
        plus.textContent = now ? "−" : "+";
        plus.title = now ? t("adds.close") : what;
        plus.setAttribute("aria-label", plus.title);
        plus.setAttribute("aria-expanded", String(now));
        if (now) trs[0].querySelector("input").focus();
        else plus.focus();
      };
      plus.addEventListener("click", toggle);
      const last = rows.length ? rows[rows.length - 1] : host;
      const th = last.querySelector("th");
      th.classList.add("has-addplus");
      th.append(plus);
      if (open) setTimeout(() => {
        if (document.activeElement === document.body) trs[0].querySelector("input").focus({ preventScroll: true });
      }, 0);
      return [...rows, ...trs];
    };
    const head = h(
      "tr",
      {},
      h("th", { scope: "row", class: "corner" }, t("table.date")),
      events.map((ev) => h(
        "th",
        { scope: "col" },
        h(
          "div",
          { class: "rl" },
          h("button", { type: "button", class: "cellbtn", "aria-label": t("table.event_aria", { label: label(ev) }), onclick: () => eventDialog(ev) }, fmtShortDate(ev.date)),
          h("button", { type: "button", class: "edit copy", title: t("copy.title"), "aria-label": t("copy.aria", { label: label(ev) }), onclick: () => copyDialog(ev) }, icon("copy"))
        ),
        clock(ev) ? h("span", { class: "sub" }, clock(ev)) : ""
      )),
      h("th", { scope: "col", class: "addcol" }, h("button", { type: "button", class: "ghost", "aria-label": t("table.add_event"), title: t("table.add_event"), onclick: newEventDialog }, events.length ? "+" : "+ " + t("table.add_event")))
    );
    const textCell = (cur, placeholder, aria, save, values) => {
      let value = cur;
      const input = h("input", { type: "text", value, maxlength: "200", placeholder, "aria-label": aria, autocomplete: "off" });
      const td = h("td", { class: "text" }, input);
      if (values) suggest(input, td, values);
      enterMovesOn(input);
      input.addEventListener("change", async () => {
        let v = input.value.trim();
        const same = values && values().find((x) => x.toLowerCase() === v.toLowerCase());
        if (same) v = same;
        if (v === value) {
          input.value = value;
          return;
        }
        if (await write(() => save(v))) value = v;
        input.value = value;
      });
      return td;
    };
    let suggestN = 0;
    const suggest = (input, td, values) => {
      const id = "sugg-" + ++suggestN;
      const list = h("ul", { class: "elist sugg", id, role: "listbox", hidden: true });
      input.setAttribute("role", "combobox");
      input.setAttribute("aria-autocomplete", "list");
      input.setAttribute("aria-controls", id);
      input.setAttribute("aria-expanded", "false");
      td.append(list);
      let hits = [], active = -1;
      const show = (on) => {
        list.hidden = !on;
        input.setAttribute("aria-expanded", String(on));
        td.classList.toggle("suggesting", on);
      };
      const activate = (i) => {
        active = i;
        [...list.children].forEach((li, j) => li.setAttribute("aria-selected", String(j === i)));
        if (list.children[i]) {
          list.children[i].scrollIntoView({ block: "nearest" });
          input.setAttribute("aria-activedescendant", list.children[i].id);
        } else input.removeAttribute("aria-activedescendant");
      };
      const take = (v) => {
        input.value = v;
        show(false);
      };
      const fill = (all) => {
        const q = input.value.trim().toLowerCase();
        const known = values();
        hits = known.filter((v) => v.toLowerCase() !== q && (all || q) && v.toLowerCase().includes(q)).sort((a, b) => Number(!a.toLowerCase().startsWith(q)) - Number(!b.toLowerCase().startsWith(q))).slice(0, 8);
        list.replaceChildren(...hits.map((v, i) => h("li", { role: "option", id: id + "-" + i, onmousedown: (e) => {
          e.preventDefault();
          take(v);
          input.dispatchEvent(new Event("change"));
        } }, v)));
        show(hits.length > 0);
        activate(q && hits.length && hits[0].toLowerCase().startsWith(q) && !known.some((v) => v.toLowerCase() === q) ? 0 : -1);
      };
      input.addEventListener("input", () => fill(false));
      input.addEventListener("keydown", (e) => {
        if (e.key === "ArrowDown") {
          e.preventDefault();
          if (list.hidden) fill(true);
          else activate(Math.min(hits.length - 1, active + 1));
        } else if (e.key === "ArrowUp" && !list.hidden) {
          e.preventDefault();
          activate(Math.max(0, active - 1));
        } else if (e.key === "Enter" && !list.hidden && hits[active] !== void 0) take(hits[active]);
        else if (e.key === "Escape" && !list.hidden) {
          e.preventDefault();
          e.stopPropagation();
          show(false);
        }
      }, true);
      input.addEventListener("blur", () => show(false));
    };
    const rowValues = (f) => () => {
      const n = /* @__PURE__ */ new Map();
      for (const ev of events) {
        const v = fieldValue[ev.id + "/" + f.id];
        if (v) n.set(v, (n.get(v) || 0) + 1);
      }
      return [...n.keys()].sort((a, b) => n.get(b) - n.get(a) || a.localeCompare(b, locale()));
    };
    const grip = (title) => h("span", { class: "grip", title, "aria-hidden": "true" }, "⋮⋮");
    const handleFor = (handle, kind, item) => {
      handle._drag = { kind, item };
    };
    const dropOn = (row, accept, act, into = false) => {
      row._drop = { accept, act, into };
      return row;
    };
    const dragRow = (row, handle, kind, item, list, move) => {
      handleFor(handle, kind, item);
      return dropOn(row, (k, d) => k === kind && d.id !== item.id, (d, after) => {
        const others = list.filter((x) => x.id !== d.id);
        return move(d, others.findIndex((x) => x.id === item.id) + (after ? 1 : 0));
      });
    };
    let rowDrag = null;
    const dropMarks = () => document.querySelectorAll("tr.drop").forEach((x) => x.classList.remove("drop"));
    const rowDragAt = (x, y) => {
      const d = rowDrag;
      d.ghost.style.left = x + 12 + "px";
      d.ghost.style.top = y + 8 + "px";
      dropMarks();
      d.line.hidden = true;
      d.target = null;
      const tr = document.elementFromPoint(x, y)?.closest("tr");
      if (!tr || tr === d.row || !tr._drop || !tr._drop.accept(d.kind, d.item)) return;
      const r2 = tr.getBoundingClientRect();
      const after = tr._drop.into || y > r2.top + r2.height / 2;
      d.target = { tr, after };
      if (tr._drop.into) {
        tr.classList.add("drop");
        return;
      }
      const box = tr.closest(".grid-scroll").getBoundingClientRect();
      Object.assign(d.line.style, { left: box.left + "px", width: Math.min(box.width, r2.width) + "px", top: (after ? r2.bottom : r2.top) - 1.5 + "px" });
      d.line.hidden = false;
    };
    const rowDragScroll = () => {
      const d = rowDrag;
      if (!d || !d.started) return;
      const box = d.row.closest(".grid-scroll");
      const r2 = box.getBoundingClientRect();
      const head2 = box.querySelector("thead");
      const top = Math.max(r2.top, 0) + (head2 ? head2.offsetHeight : 0);
      const dy = d.y < top + 30 ? -16 : d.y > Math.min(r2.bottom, innerHeight) - 30 ? 16 : 0;
      if (dy) {
        box.scrollTop += dy;
        rowDragAt(d.x, d.y);
      }
    };
    const rowDragMove = (e) => {
      const d = rowDrag;
      if (!d) return;
      d.x = e.clientX;
      d.y = e.clientY;
      if (!d.started) {
        if (Math.hypot(d.x - d.x0, d.y - d.y0) < 5) return;
        d.started = true;
        document.body.classList.add("row-dragging");
        d.row.classList.add("dragged");
        if (document.activeElement && document.activeElement.blur) document.activeElement.blur();
        d.ghost = document.body.appendChild(h("div", { class: "row-ghost" }, d.item.name));
        d.line = document.body.appendChild(h("div", { class: "drop-line", hidden: true }));
        d.timer = setInterval(rowDragScroll, 40);
      }
      e.preventDefault();
      rowDragAt(d.x, d.y);
    };
    const rowDragEnd = async (e) => {
      const d = rowDrag;
      rowDrag = null;
      removeEventListener("pointermove", rowDragMove);
      removeEventListener("pointerup", rowDragEnd);
      removeEventListener("pointercancel", rowDragEnd);
      removeEventListener("keydown", rowDragKey, true);
      if (!d || !d.started) return;
      clearInterval(d.timer);
      d.ghost.remove();
      d.line.remove();
      dropMarks();
      d.row.classList.remove("dragged");
      document.body.classList.remove("row-dragging");
      const swallow = (c) => {
        c.stopPropagation();
        c.preventDefault();
      };
      document.addEventListener("click", swallow, { capture: true, once: true });
      setTimeout(() => document.removeEventListener("click", swallow, { capture: true }), 0);
      if (e.type === "pointerup" && d.target && await write(() => d.target.tr._drop.act(d.item, d.target.after, d.kind))) render();
    };
    const rowDragKey = (e) => {
      if (e.key === "Escape" && rowDrag) {
        e.preventDefault();
        e.stopPropagation();
        rowDrag.target = null;
        rowDragEnd({ type: "pointercancel" });
      }
    };
    const onRowDown = (e) => {
      if (e.button !== 0 || rowDrag) return;
      const th = e.target.closest("th[scope=row]");
      const g = th && th.querySelector(".grip");
      if (!g || !g._drag) return;
      const onGrip = !!e.target.closest(".grip");
      if (e.pointerType !== "mouse" && !onGrip) return;
      if (!onGrip && e.target.closest("input, select, textarea, button.fold, button.auto, button.freeze, button.edit, button.addplus, .tags")) return;
      if (onGrip) e.preventDefault();
      rowDrag = { ...g._drag, row: th.closest("tr"), x0: e.clientX, y0: e.clientY, x: e.clientX, y: e.clientY, started: false };
      addEventListener("pointermove", rowDragMove, { passive: false });
      addEventListener("pointerup", rowDragEnd);
      addEventListener("pointercancel", rowDragEnd);
      addEventListener("keydown", rowDragKey, true);
    };
    const nameField = (cur, aria, hint, rename, remove) => {
      const name = h("input", { type: "text", class: "fname", value: cur, maxlength: "80", "aria-label": aria, title: hint });
      name.addEventListener("keydown", (e) => {
        if (e.key === "Enter") {
          e.preventDefault();
          name.blur();
        } else if (e.key === "Escape") {
          name.value = cur;
          name.blur();
        }
      });
      name.addEventListener("change", async () => {
        const v = name.value.trim();
        if (v === cur) {
          name.value = cur;
          return;
        }
        if (await write(() => v ? rename(v) : remove())) render();
        else name.value = cur;
      });
      return name;
    };
    const fieldRow = (f, i) => {
      const g = grip(t("field.drag"));
      const name = nameField(
        f.name,
        t("field.name_aria", { name: f.name }),
        t("field.name_hint"),
        (v) => api("PATCH", `${pbase}/fields/${f.id}`, { name: v }),
        async () => {
          await confirmed(t("field.delete"), t("field.delete_confirm", { name: f.name }), t("common.delete"));
          return api("DELETE", `${pbase}/fields/${f.id}`);
        }
      );
      const pinName = t(f.frozen ? "field.unfreeze" : "field.freeze", { name: f.name });
      const pin2 = h("button", {
        type: "button",
        class: "freeze" + (f.frozen ? " on" : ""),
        "aria-pressed": String(!!f.frozen),
        title: pinName,
        "aria-label": pinName,
        onclick: async () => {
          if (await write(() => api("PUT", `${pbase}/fields/${f.id}/frozen`, { frozen: !f.frozen }))) render();
        }
      }, icon("pin"));
      const row = h(
        "tr",
        { class: "field-row" + (f.frozen ? " frozen" : "") },
        h("th", { scope: "row" }, h("div", { class: "rl" }, g, name, pin2)),
        events.map((ev) => textCell(
          fieldValue[ev.id + "/" + f.id] || "",
          t("table.add_value", { name: f.name.toLowerCase() }),
          t("table.value_aria", { name: f.name, label: label(ev) }),
          async (v) => {
            await api("PUT", `${epath}/${ev.id}/fields/${f.id}`, { value: v });
            fieldValue[ev.id + "/" + f.id] = v.trim();
          },
          rowValues(f)
        )),
        filler()
      );
      return dragRow(row, g, "field", f, fields.fields, (d, pos) => api("PUT", `${pbase}/fields/${d.id}/position`, { position: pos }));
    };
    const levelRank = { good: 0, poor: 1, "": 2, never: 3 };
    const taskRank = { good: 0, poor: 1, "": 2, never: 3 };
    const editCell = (ev, tk, td, typed = "") => {
      const k = at(ev, tk);
      planState.edit = k;
      const s = slotsAt(ev, tk);
      const again = async (action) => {
        input.value = "";
        if (await write(action)) {
          planState.edit = k;
          await render();
        }
      };
      const chips = s.items.map((x) => h(
        "span",
        { class: "echip" + (x.hand ? " hand" : "") + (x.locked ? " locked" : "") },
        x.locked ? mark("lock") : "",
        h("span", {}, itemText(x)),
        x.hand ? h("button", {
          type: "button",
          class: "x",
          "aria-label": t("place.remove_aria", { name: x.name }),
          title: t("common.remove"),
          onclick: () => again(async () => {
            await api("DELETE", `${pbase}/cells/${x.cell.id}`);
            undoable(t("undo.remove", { name: x.name }), () => api("POST", pbase + "/cells", cellBack(x.cell)));
          })
        }, "×") : ""
      ));
      const taken = new Set(s.items.filter((x) => x.cell && x.kind === "person").map((x) => x.id));
      const fromPlanner = new Set(s.items.filter((x) => !x.cell && x.kind === "person").map((x) => x.id));
      const taskLevel = (p) => ((chosen[p.id] || []).find((x) => x.task_id === tk.id) || {}).level || "";
      const level = (p) => (ans[p.id] || {})[ev.id] || "";
      const candidates = people.filter((p) => !taken.has(p.id)).sort((a, b) => levelRank[level(a)] - levelRank[level(b)] || taskRank[taskLevel(a)] - taskRank[taskLevel(b)] || a.name.localeCompare(b.name, locale()));
      const input = h("input", {
        type: "text",
        class: "ename",
        maxlength: "80",
        placeholder: t("cell.placeholder"),
        autocomplete: "off",
        role: "combobox",
        "aria-label": t("cell.add_aria", { task: tk.name, label: label(ev) }),
        "aria-autocomplete": "list",
        "aria-expanded": "true",
        "aria-controls": "cell-list"
      });
      const list = h("ul", { class: "elist", id: "cell-list", role: "listbox" });
      let options = [];
      let active = -1;
      const activate = (i) => {
        active = i;
        options.forEach((o, j) => o.el.setAttribute("aria-selected", String(j === i)));
        if (options[i]) {
          options[i].el.scrollIntoView({ block: "nearest" });
          input.setAttribute("aria-activedescendant", options[i].el.id);
        }
      };
      const added = (name, body) => again(async () => {
        const c = await api("POST", pbase + "/cells", body);
        undoable(t("undo.add", { name }), () => api("DELETE", `${pbase}/cells/${c.id}`));
      });
      const addPerson2 = (p) => added(p.name, { event: ev.id, task: tk.id, person: p.id, ...fromPlanner.has(p.id) ? { locked: true } : {} });
      const addText = (text) => added(text, { event: ev.id, task: tk.id, label: text, fills: 1 });
      const fill = () => {
        const q = input.value.trim().toLowerCase();
        const hits = candidates.filter((p) => p.name.toLowerCase().includes(q)).sort((a, b) => q ? Number(!a.name.toLowerCase().startsWith(q)) - Number(!b.name.toLowerCase().startsWith(q)) : 0).slice(0, 12);
        options = hits.map((p, i) => {
          const l = level(p), tl = taskLevel(p);
          const el = h(
            "li",
            { role: "option", id: "cell-opt-" + i, class: l === "never" ? "no" : "", onmousedown: (e) => {
              e.preventDefault();
              addPerson2(p);
            } },
            h("span", { class: "ans " + (l || "none") }, l ? icon(LEVEL_ICON[l]) : ""),
            h("span", { class: "nm" }, p.name),
            h("span", { class: "why small" }, fromPlanner.has(p.id) ? t("cell.in_draft") : l === "never" ? t("cell.said_no") : tl && tl !== "never" ? tk.name + ": " + t("pick." + tl) : "")
          );
          return { el, add: () => addPerson2(p) };
        });
        const exact = people.some((p) => p.name.toLowerCase() === q);
        if (q && !exact) {
          const text = input.value.trim();
          const el = h("li", { role: "option", id: "cell-opt-custom", class: "custom", onmousedown: (e) => {
            e.preventDefault();
            addText(text);
          } }, "+ " + t("cell.custom", { text }));
          options.push({ el, add: () => addText(text) });
        }
        list.replaceChildren(...options.map((o) => o.el));
        list.hidden = !options.length;
        activate(q && options.length ? 0 : -1);
      };
      input.addEventListener("input", fill);
      input.addEventListener("keydown", (e) => {
        if (e.key === "ArrowDown") {
          e.preventDefault();
          activate(Math.min(options.length - 1, active + 1));
        } else if (e.key === "ArrowUp") {
          e.preventDefault();
          activate(Math.max(0, active - 1));
        } else if (e.key === "Enter") {
          e.preventDefault();
          if (options[active]) options[active].add();
        } else if (e.key === "Escape") {
          e.preventDefault();
          e.stopPropagation();
          close();
        }
      });
      const addBtn = h("button", {
        type: "button",
        class: "eadd",
        title: t("cell.add"),
        "aria-label": t("cell.add"),
        onmousedown: (e) => e.preventDefault(),
        onclick: () => {
          if (options[active]) options[active].add();
          else input.focus();
        }
      }, "+");
      const box = h("div", { class: "cell-edit" }, h("div", { class: "echips" }, chips), h("div", { class: "erow" }, input, addBtn), list);
      const close = () => {
        if (planState.edit === k) planState.edit = null;
        if (box.isConnected) putDraftCell(ev, tk);
      };
      box.addEventListener("focusout", (e) => {
        if (!box.contains(e.relatedTarget)) setTimeout(() => {
          if (box.isConnected && !box.contains(document.activeElement)) close();
        }, 0);
      });
      td.classList.add("editing");
      td.replaceChildren(box);
      planState.editInput = input;
      input.value = typed;
      fill();
      input.focus();
    };
    const draftTd = {};
    const draftCell = (ev, tk) => {
      const s = slotsAt(ev, tk);
      const edit = org.level !== "view";
      const plus = () => h("button", {
        type: "button",
        class: "addname",
        title: t("cell.add"),
        "aria-label": t("cell.add_aria", { task: tk.name, label: label(ev) }),
        onclick: (e) => editCell(ev, tk, e.currentTarget.closest("td"))
      }, "+");
      const clickable = (td2) => {
        if (edit) td2.addEventListener("click", (e) => {
          if (!e.ctrlKey && !e.metaKey && !e.shiftKey && !e.target.closest("button, input, .cell-edit")) editCell(ev, tk, td2);
        });
        return td2;
      };
      if (!s.n && !s.items.length) return clickable(h("td", { class: "draft none" }, h(
        "div",
        { class: "dcell" },
        h("div", { class: "dlines" }, h("span", { class: "muted small", "aria-hidden": "true" }, "–")),
        edit ? h("div", { class: "dside" }, plus()) : ""
      )));
      const open = () => placeDialog(ev, tk);
      const was = comparing ? before[at(ev, tk)] || [] : null;
      const isNew = (x) => was && x.kind === "person" && !x.hand && !was.some((p) => p.id === x.id);
      const line = (x) => {
        const el = line0(x);
        if (isNew(x)) el.classList.add("new");
        return el;
      };
      const line0 = (x) => {
        if (!edit || x.hand || x.kind !== "person") return slot(x);
        const name = t(x.locked ? "place.unlock_aria" : "place.lock_aria", { name: x.name });
        return h(
          "span",
          { class: "slot" + (x.locked ? " locked" : "") },
          h("button", {
            type: "button",
            class: "lockbtn" + (x.locked ? " on" : ""),
            "aria-pressed": String(!!x.locked),
            title: name,
            "aria-label": name,
            onclick: () => setLocks([{ event: ev.id, task: tk.id, person: x.id }], !x.locked)
          }, icon(x.locked ? "lock" : "unlock")),
          h("span", { class: "nm" }, itemText(x)),
          x.weak ? h("span", { class: "chip poor" }, t("draft.maybe")) : ""
        );
      };
      const lines = s.items.map(line);
      if (s.open && done) for (let i = 0; i < s.open; i++) lines.push(h("span", { class: "slot empty" }, t("draft.not_filled"), s.why ? [" · ", h("button", { type: "button", class: "why link-button", onclick: open }, t("draft.why"))] : ""));
      else if (s.open) lines.push(h("span", { class: "slot open" }, tn("draft.open", s.open)));
      const pool = poolAt[at(ev, tk)];
      const td = h("td", { class: "draft" }, h(
        "div",
        { class: "dcell" },
        h("div", { class: "dlines" }, lines),
        h(
          "div",
          { class: "dside" },
          h("button", { type: "button", class: "dmenu", title: t("draft.open_place"), "aria-label": t("draft.place_aria", { task: tk.name, label: label(ev) }), onclick: open }, icon("more")),
          edit ? plus() : "",
          pool && pool.people.length ? h("span", { class: "mark pool", role: "img", title: t("draft.pool_mark"), "aria-label": t("draft.pool_mark") }, icon("filter")) : ""
        )
      ));
      if (was && (s.items.some(isNew) || was.some((p) => !s.items.some((x) => x.kind === "person" && x.id === p.id)))) {
        td.classList.add("changed");
        td.title = t("draft.before", { id: olderId, names: was.map((p) => p.name).join(", ") || "–" });
      }
      if (done) reg("draft", tk.id, ev, td, () => putDraftCell(ev, tk));
      return clickable(td);
    };
    const putDraftCell = (ev, tk) => {
      const td = draftCell(ev, tk);
      if (draftTd[at(ev, tk)]) draftTd[at(ev, tk)].replaceWith(td);
      draftTd[at(ev, tk)] = td;
      return td;
    };
    const grids = { places: { rows: [], cells: /* @__PURE__ */ new Map() }, answers: { rows: [], cells: /* @__PURE__ */ new Map() }, draft: { rows: [], cells: /* @__PURE__ */ new Map() } };
    const evIndex = new Map(events.map((ev, i) => [ev.id, i]));
    let sel = null;
    const selbar = h("div", { class: "selbar", hidden: true, role: "region", "aria-label": t("sel.bar") });
    const cellKey = (row, ev) => row + "/" + ev;
    const reg = (grid, row, ev, td, paint) => {
      if (org.level === "view") return;
      const g = grids[grid];
      if (!g.rows.includes(row)) g.rows.push(row);
      g.cells.set(cellKey(row, ev.id), { td, row, ev, paint });
      td.dataset.cell = grid + "|" + row + "|" + ev.id;
    };
    const rowShown = (grid, row) => {
      const c = grids[grid].cells.get(cellKey(row, events[0] && events[0].id));
      return !c || !c.td.closest("tr").hidden;
    };
    const paintSel = () => {
      document.querySelectorAll("td.sel").forEach((td) => td.classList.remove("sel"));
      if (sel) sel.keys.forEach((k) => {
        const c = grids[sel.grid].cells.get(k);
        if (c) c.td.classList.add("sel");
      });
      drawBar();
    };
    const unselect = () => {
      sel = null;
      paintSel();
    };
    const rect = (grid, a, b) => {
      const rows = grids[grid].rows;
      const [r0, r1] = [rows.indexOf(a.row), rows.indexOf(b.row)].sort((x, y) => x - y);
      const [c0, c1] = [evIndex.get(a.ev), evIndex.get(b.ev)].sort((x, y) => x - y);
      const keys = /* @__PURE__ */ new Set();
      for (let i = r0; i <= r1; i++) if (rowShown(grid, rows[i])) for (let j = c0; j <= c1; j++) keys.add(cellKey(rows[i], events[j].id));
      return keys;
    };
    const selectRect = (grid, anchor, to, add) => {
      const keys = rect(grid, anchor, to);
      if (add && sel && sel.grid === grid) sel.base.forEach((k) => keys.add(k));
      sel = { grid, keys, anchor, base: add && sel && sel.grid === grid ? sel.base : /* @__PURE__ */ new Set() };
      paintSel();
    };
    const toggleCell = (grid, row, ev) => {
      if (!sel || sel.grid !== grid) sel = { grid, keys: /* @__PURE__ */ new Set(), anchor: { row, ev }, base: /* @__PURE__ */ new Set() };
      const k = cellKey(row, ev);
      if (sel.keys.has(k)) sel.keys.delete(k);
      else sel.keys.add(k);
      sel.anchor = { row, ev };
      sel.base = new Set(sel.keys);
      if (!sel.keys.size) sel = null;
      paintSel();
    };
    const selectLine = (grid, e, pick) => {
      const keys = new Set([...grids[grid].cells.keys()].filter((k) => pick(grids[grid].cells.get(k)) && rowShown(grid, grids[grid].cells.get(k).row)));
      if (!keys.size) return;
      const first = grids[grid].cells.get([...keys][0]);
      if ((e.ctrlKey || e.metaKey) && sel && sel.grid === grid) {
        const all = [...keys].every((k) => sel.keys.has(k));
        keys.forEach((k) => all ? sel.keys.delete(k) : sel.keys.add(k));
        sel.base = new Set(sel.keys);
        if (!sel.keys.size) sel = null;
      } else if (e.shiftKey && sel && sel.grid === grid) {
        const last = grids[grid].cells.get([...keys].pop());
        sel.keys = rect(grid, sel.anchor, { row: last.row, ev: events[events.length - 1].id });
        rect(grid, sel.anchor, { row: first.row, ev: events[0].id }).forEach((k) => sel.keys.add(k));
        sel.base = new Set(sel.keys);
      } else {
        sel = { grid, keys, anchor: { row: first.row, ev: first.ev.id }, base: new Set(keys) };
      }
      paintSel();
    };
    let drag = null;
    const cellAt = (x, y) => {
      const td = document.elementFromPoint(x, y)?.closest("td[data-cell]");
      if (!td) return null;
      const [grid, row, ev] = td.dataset.cell.split("|");
      return { grid, row: Number(row), ev: Number(ev) };
    };
    const dragFollow = () => {
      if (!drag) return;
      const c = cellAt(drag.x, drag.y);
      if (!c || c.grid !== drag.grid) return;
      if (!drag.moved && c.row === drag.from.row && c.ev === drag.from.ev) return;
      if (!drag.moved) {
        drag.moved = true;
        document.body.classList.add("selecting");
        if (document.activeElement && document.activeElement.blur) document.activeElement.blur();
      }
      selectRect(drag.grid, drag.from, c, drag.add);
    };
    let scroller = null;
    const autoScroll = () => {
      if (!drag || !drag.moved) return;
      const box = document.querySelector(".grid-scroll.main");
      if (!box) return;
      const r2 = box.getBoundingClientRect();
      const edge = 40, step = 24;
      const dx = drag.x > r2.right - edge ? step : drag.x < r2.left + 160 + edge ? -step : 0;
      const dy = drag.y > Math.min(r2.bottom, innerHeight) - edge ? step : drag.y < Math.max(r2.top, 0) + 80 ? -step : 0;
      if (dx) box.scrollLeft += dx;
      if (dy) box.scrollTop += dy;
      if (dx || dy) dragFollow();
    };
    const onMove = (e) => {
      if (drag) {
        drag.x = e.clientX;
        drag.y = e.clientY;
        dragFollow();
      }
    };
    const onUp = () => {
      if (!drag) return;
      const moved = drag.moved;
      drag = null;
      clearInterval(scroller);
      document.body.classList.remove("selecting");
      if (moved) document.addEventListener("click", (e) => {
        e.stopPropagation();
        e.preventDefault();
      }, { capture: true, once: true });
    };
    const onCellDown = (e) => {
      if (e.pointerType !== "mouse" || e.button !== 0) return;
      const td = e.target.closest("td[data-cell]");
      if (!td || e.target.closest(".note-tab")) return;
      const [grid, row, ev] = td.dataset.cell.split("|").map((x, i) => i ? Number(x) : x);
      if (e.ctrlKey || e.metaKey) {
        e.preventDefault();
        toggleCell(grid, row, ev);
        swallowClick();
        return;
      }
      if (e.shiftKey && sel && sel.grid === grid) {
        e.preventDefault();
        selectRect(grid, sel.anchor, { row, ev }, false);
        swallowClick();
        return;
      }
      if (sel) unselect();
      drag = { grid, from: { row, ev }, moved: false, x: e.clientX, y: e.clientY, add: false };
      clearInterval(scroller);
      scroller = setInterval(autoScroll, 40);
    };
    const swallowClick = () => document.addEventListener("click", (e) => {
      e.stopPropagation();
      e.preventDefault();
    }, { capture: true, once: true });
    const onKey = (e) => {
      if (!sel || !selbar.isConnected) return;
      if (document.querySelector("dialog[open]")) return;
      const typing3 = e.target.closest && e.target.closest("input, textarea, select");
      if (e.key === "Escape") {
        unselect();
        return;
      }
      if (typing3 && !e.target.closest(".selbar")) return;
      if (sel.grid === "draft") return;
      if (e.key === "Delete" || e.key === "Backspace") {
        e.preventDefault();
        applySel(sel.grid === "answers" ? "" : 0);
        return;
      }
      if (sel.grid === "answers" && ["1", "2", "3"].includes(e.key) && !typing3) {
        e.preventDefault();
        applySel({ 1: "good", 2: "poor", 3: "never" }[e.key]);
      }
      if (sel.grid === "places" && /^[0-9]$/.test(e.key) && !typing3) {
        e.preventDefault();
        const n = selbar.querySelector("input");
        if (n) {
          n.focus();
          n.value = e.key;
        }
      }
    };
    document.addEventListener("pointermove", onMove);
    document.addEventListener("pointerup", onUp);
    document.addEventListener("keydown", onKey);
    const offSel = () => {
      document.removeEventListener("pointermove", onMove);
      document.removeEventListener("pointerup", onUp);
      document.removeEventListener("keydown", onKey);
    };
    const watchGone = setInterval(() => {
      if (!selbar.isConnected && document.querySelector("#main > *")) {
        offSel();
        clearInterval(watchGone);
      }
    }, 2e3);
    const applySel = async (value) => {
      if (!sel) return;
      const g = grids[sel.grid];
      const cells2 = [...sel.keys].map((k) => g.cells.get(k)).filter(Boolean);
      if (sel.grid === "draft") {
        const on = value === "lock";
        await setLocks(lockItems(cells2.map((c) => [c.ev, tasks.find((tk) => tk.id === c.row)]).filter((x) => x[1]), on), on);
        return;
      }
      if (sel.grid === "answers") {
        const was = cells2.map((c) => (ans[c.row] || {})[c.ev.id] || "");
        const items = cells2.filter((c, i) => was[i] !== value).map((c) => ({ person: c.row, event: c.ev.id, level: value }));
        if (!items.length) return;
        const paint = () => {
          cells2.forEach((c) => c.paint());
          new Set(cells2.map((c) => c.ev.id)).forEach(recount);
        };
        cells2.forEach((c) => {
          (ans[c.row] ||= {})[c.ev.id] = value;
        });
        paint();
        if (!await write(() => api("PUT", pbase + "/answers", { answers: items }))) {
          cells2.forEach((c, i) => {
            ans[c.row][c.ev.id] = was[i];
          });
          paint();
        }
      } else {
        const n = Number(value) || 0;
        const was = cells2.map((c) => placesAt[at(c.ev, { id: c.row })] || 0);
        const items = cells2.filter((c, i) => was[i] !== n).map((c) => ({ event: c.ev.id, task: c.row, slots: n }));
        if (!items.length) return;
        const set = (c, v) => {
          const k = at(c.ev, { id: c.row });
          if (v) placesAt[k] = v;
          else delete placesAt[k];
        };
        const paint = () => cells2.forEach((c) => {
          c.paint();
          putDraftCell(c.ev, tasks.find((tk) => tk.id === c.row));
        });
        cells2.forEach((c) => set(c, n));
        paint();
        if (!await write(() => api("PUT", pbase + "/places", { places: items }))) {
          cells2.forEach((c, i) => set(c, was[i]));
          paint();
        }
      }
    };
    const drawBar = () => {
      selbar.hidden = !sel;
      if (!sel) {
        selbar.replaceChildren();
        return;
      }
      const n = sel.keys.size;
      const close = h("button", { type: "button", class: "ghost icon", title: t("sel.unselect"), "aria-label": t("sel.unselect"), onclick: unselect }, icon("x"));
      const count = h("span", { class: "n" }, tn("sel.count", n));
      if (sel.grid === "answers") {
        const lv = (level, key2) => h("button", {
          type: "button",
          class: "ans",
          "data-level": level,
          title: t("answer." + level) + " (" + key2 + ")",
          "aria-label": t("sel.set_answer", { answer: t("answer." + level) }),
          onclick: () => applySel(level)
        }, answerMark(level));
        selbar.replaceChildren(
          count,
          lv("good", "1"),
          lv("poor", "2"),
          lv("never", "3"),
          h("button", { type: "button", class: "ghost sm", title: t("sel.clear") + " (Delete)", onclick: () => applySel("") }, t("sel.clear")),
          close
        );
      } else if (sel.grid === "draft") {
        selbar.replaceChildren(
          count,
          h("button", { type: "button", class: "sm", onclick: () => applySel("lock") }, icon("lock"), " " + t("sel.lock")),
          h("button", { type: "button", class: "ghost sm", onclick: () => applySel("unlock") }, icon("unlock"), " " + t("sel.unlock")),
          close
        );
      } else {
        const num = h("input", { type: "number", min: "0", max: "50", inputmode: "numeric", "aria-label": t("sel.places_aria"), placeholder: "#" });
        const form = h(
          "form",
          { class: "row-inline", onsubmit: (e) => {
            e.preventDefault();
            applySel(parseInt(num.value || "0", 10));
          } },
          num,
          h("button", { type: "submit", class: "sm" }, t("sel.set"))
        );
        selbar.replaceChildren(count, form, h("button", { type: "button", class: "ghost sm", title: t("sel.clear") + " (Delete)", onclick: () => applySel(0) }, t("sel.clear")), close);
      }
    };
    const placesCell = (tk, ev) => {
      const k = at(ev, tk);
      const input = enterMovesOn(h("input", { type: "number", min: "0", max: "50", inputmode: "numeric", "aria-label": t("table.places_aria", { task: tk.name, label: label(ev) }) }));
      const paint = () => {
        const n = placesAt[k] || 0;
        input.value = n ? String(n) : "";
        input.classList.toggle("set", n > 0);
      };
      input.addEventListener("change", async () => {
        const raw = input.value.trim();
        const n = raw === "" ? 0 : parseInt(raw, 10);
        const cur = placesAt[k] || 0;
        if (Number.isNaN(n) || n === cur) {
          paint();
          return;
        }
        const path = `${epath}/${ev.id}/places/${tk.id}`;
        if (await write(() => n === 0 ? api("DELETE", path) : api("PUT", path, { slots: n }))) {
          if (n) placesAt[k] = n;
          else delete placesAt[k];
          putDraftCell(ev, tk);
        }
        paint();
      });
      paint();
      const td = h("td", { class: "places" }, input);
      reg("places", tk.id, ev, td, paint);
      return td;
    };
    const taskRow = (tk, i) => {
      const g = grip(t("task.drag"));
      const zap = zapBtn(
        tk,
        "planned",
        (on) => t(on ? "task.auto_on" : "task.auto_off", { name: tk.name }),
        t("task.planned_aria", { name: tk.name }),
        `${base}/tasks/${tk.id}`,
        () => render()
      );
      const row = h(
        "tr",
        { class: "task-row" },
        h("th", { scope: "row" }, h(
          "div",
          { class: "rl" },
          g,
          h("button", { type: "button", class: "cellbtn", "aria-label": t("table.task_aria", { name: tk.name }), onclick: () => taskDialog(tk) }, tk.name),
          zap
        )),
        events.map((ev) => placesCell(tk, ev)),
        rowPick("places", tk.id, tk.name)
      );
      handleFor(g, "task", tk);
      return dropOn(row, (k, d) => k === "task" && d.id !== tk.id, (d, after) => {
        const others = tasksIn(tk.section).filter((x) => x.id !== d.id);
        return moveTask(d, tk.section, others.findIndex((x) => x.id === tk.id) + (after ? 1 : 0));
      });
    };
    const countCells = {};
    const recount = (evId) => {
      const c = { good: 0, poor: 0, never: 0 };
      people.forEach((p) => {
        const l = (ans[p.id] || {})[evId];
        if (l) c[l]++;
      });
      countCells[evId].replaceChildren(h("b", { class: "good" }, icon("up"), c.good), h("b", { class: "poor" }, icon("maybe"), c.poor), h("b", { class: "never" }, icon("down"), c.never));
      countCells[evId].setAttribute("aria-label", t("table.count", c));
    };
    const notePop = h("div", { class: "note-pop", hidden: true, role: "dialog" });
    let noteOpen = null;
    const placePop = (td) => {
      const r2 = td.getBoundingClientRect();
      notePop.hidden = false;
      const w = notePop.offsetWidth, hgt = notePop.offsetHeight;
      const left = Math.max(8, Math.min(r2.right - 8, innerWidth - w - 8));
      const top = r2.top - hgt - 6 > 8 ? r2.top - hgt - 6 : Math.min(r2.bottom + 6, innerHeight - hgt - 8);
      notePop.style.left = left + "px";
      notePop.style.top = top + "px";
    };
    const closeNote = () => {
      notePop.hidden = true;
      notePop.replaceChildren();
      noteOpen = null;
    };
    const peekNote = (td, p, ev) => {
      if (noteOpen && noteOpen.edit) return;
      const text = (notes[p.id] || {})[ev.id];
      if (!text) return;
      notePop.replaceChildren(h("div", { class: "note-who small muted" }, p.name + " · " + label(ev)), h("div", { class: "note-text" }, text));
      notePop.classList.remove("editing");
      noteOpen = { td, edit: false };
      placePop(td);
    };
    const openNote = (td, p, ev, repaint) => {
      const text = (notes[p.id] || {})[ev.id] || "";
      const head2 = h("div", { class: "note-who small muted" }, p.name + " · " + label(ev));
      if (org.level === "view") {
        if (text) peekNote(td, p, ev);
        return;
      }
      const area = h("textarea", { rows: "3", maxlength: "300", placeholder: t("note.placeholder"), "aria-label": t("note.aria", { name: p.name, label: label(ev) }) });
      area.value = text;
      const save = async (value) => {
        const was = text;
        value = value.trim();
        closeNote();
        if (value === was) return;
        if (value) (notes[p.id] ||= {})[ev.id] = value;
        else if (notes[p.id]) delete notes[p.id][ev.id];
        repaint();
        if (!await write(() => api("PUT", `${pbase}/events/${ev.id}/notes/${p.id}`, { note: value }))) {
          if (was) (notes[p.id] ||= {})[ev.id] = was;
          else if (notes[p.id]) delete notes[p.id][ev.id];
          repaint();
        }
      };
      area.addEventListener("keydown", (e) => {
        if (e.key === "Escape") {
          e.preventDefault();
          e.stopPropagation();
          closeNote();
        }
        if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
          e.preventDefault();
          save(area.value);
        }
      });
      notePop.replaceChildren(head2, area, h(
        "div",
        { class: "row-inline" },
        h("button", { type: "button", class: "sm", onclick: () => save(area.value) }, t("common.save")),
        text ? h("button", { type: "button", class: "ghost sm", onclick: () => save("") }, t("note.delete")) : "",
        h("button", { type: "button", class: "ghost sm", onclick: closeNote }, t("note.cancel"))
      ));
      notePop.classList.add("editing");
      noteOpen = { td, edit: true };
      placePop(td);
      area.focus();
    };
    const onNoteAway = (e) => {
      if (!noteOpen || !noteOpen.edit) return;
      if (!notePop.isConnected) {
        document.removeEventListener("pointerdown", onNoteAway, true);
        return;
      }
      if (!notePop.contains(e.target) && !e.target.closest(".note-tab")) closeNote();
    };
    document.addEventListener("pointerdown", onNoteAway, true);
    const noteTab = (td, p, ev, repaint) => {
      const tab = h("button", { type: "button", class: "note-tab", onclick: (e) => {
        e.stopPropagation();
        if (noteOpen && noteOpen.td === td && noteOpen.edit) closeNote();
        else openNote(td, p, ev, repaint);
      } });
      td.addEventListener("mouseenter", () => peekNote(td, p, ev));
      td.addEventListener("mouseleave", () => {
        if (noteOpen && !noteOpen.edit && noteOpen.td === td) closeNote();
      });
      return tab;
    };
    const answerCell = (p, ev) => {
      const b = h("button", { type: "button", class: "ans" });
      let saving = Promise.resolve();
      const paint = () => {
        const l = (ans[p.id] || {})[ev.id] || "";
        b.replaceChildren(answerMark(l));
        b.dataset.level = l || "none";
        b.setAttribute("aria-label", t("table.answer_aria", { name: p.name, label: label(ev), answer: t("answer." + (l || "none")) }));
      };
      b.addEventListener("click", () => {
        const was = (ans[p.id] || {})[ev.id] || "";
        const next = ANSWER_NEXT[was];
        (ans[p.id] ||= {})[ev.id] = next;
        paint();
        recount(ev.id);
        saving = saving.then(async () => {
          if (!await write(() => api("PUT", `${pbase}/people/${p.id}/availability`, { levels: { [ev.id]: next } }))) {
            ans[p.id][ev.id] = was;
            paint();
            recount(ev.id);
          }
        });
      });
      paint();
      const td = h("td", { class: "answer" }, b);
      const paintNote = () => {
        const text = (notes[p.id] || {})[ev.id] || "";
        td.classList.toggle("has-note", !!text);
        tab.title = text || t("note.add");
        tab.setAttribute("aria-label", text ? t("note.open_aria", { name: p.name, label: label(ev) }) : t("note.add_aria", { name: p.name, label: label(ev) }));
      };
      const tab = noteTab(td, p, ev, paintNote);
      if (org.level !== "view" || (notes[p.id] || {})[ev.id]) td.append(tab);
      paintNote();
      reg("answers", p.id, ev, td, paint);
      return td;
    };
    const inSection = (id) => people.filter((p) => p.section === id);
    const tasksIn = (id) => tasks.filter((x) => x.section === id);
    const taskSectRow = (x) => {
      const g = grip(t("section.drag"));
      const n = tasksIn(x.id).length;
      const row = h(
        "tr",
        { class: "task-sect" },
        h("th", { scope: "row" }, h(
          "div",
          { class: "rl" },
          g,
          foldBtn("ts" + x.id, x.name),
          nameField(
            x.name,
            t("section.name_aria", { name: x.name }),
            t("tsect.name_hint"),
            (v) => api("PATCH", `${base}/task-sections/${x.id}`, { name: v }),
            async () => {
              await confirmed(t("section.delete"), t("tsect.delete_confirm", { name: x.name }), t("common.delete"));
              return api("DELETE", `${base}/task-sections/${x.id}`);
            }
          )
        )),
        h("td", { colspan: String(cols + 1) }, h("span", { class: "small" }, n ? tn("tsect.tasks", n) : t("tsect.empty")))
      );
      dragRow(row, g, "tsect", x, taskSections, (d, pos) => api("PUT", `${base}/task-sections/${d.id}/position`, { position: pos }));
      const asSection = row._drop;
      row._drop = {
        accept: (k, d) => k === "task" || asSection.accept(k, d),
        act: (d, after, kind) => kind === "task" ? moveTask(d, x.id, tasksIn(x.id).filter((o) => o.id !== d.id).length) : asSection.act(d, after),
        get into() {
          return rowDrag && rowDrag.kind === "task";
        }
      };
      return row;
    };
    const movePerson = (d, section, position) => api("PUT", `${base}/people/${d.id}/position`, { section, position });
    const moveTask = (d, section, position) => api("PUT", `${base}/tasks/${d.id}/position`, { section, position });
    const zapBtn = (obj, key2, title, aria, path, after) => {
      const b = h("button", { type: "button", class: "auto", "aria-label": aria });
      const paint = () => {
        b.setAttribute("aria-pressed", String(!!obj[key2]));
        b.title = title(!!obj[key2]);
      };
      b.append(icon("auto"));
      let busy = false;
      b.addEventListener("click", async () => {
        if (busy) return;
        busy = true;
        obj[key2] = !obj[key2];
        paint();
        if (!await write(() => api("PATCH", path, { [key2]: obj[key2] }))) {
          obj[key2] = !obj[key2];
          paint();
        } else if (after) after();
        busy = false;
      });
      paint();
      return b;
    };
    const autoBtn = (p) => zapBtn(
      p,
      "proposable",
      (on) => t(on ? "person.auto_on" : "person.auto_off", { name: p.name }),
      t("person.proposable_aria", { name: p.name }),
      `${base}/people/${p.id}`
    );
    const tagsOf = (p) => tasks.filter((tk) => tk.planned).map((tk) => [tk, ((chosen[p.id] || []).find((x) => x.task_id === tk.id) || {}).level]).filter(([, l]) => l === "good" || l === "poor").map(([tk, l]) => h("span", { class: "tag " + l, title: tk.name + ": " + t("pick." + l) }, tk.name));
    const personRow = (p) => {
      const mine = (chosen[p.id] || []).filter((x) => x.level !== "never" && tasks.some((tk) => tk.id === x.task_id && tk.planned)).map((x) => nameOf(tasks, x.task_id));
      const tags = tagsOf(p);
      const g = grip(t("person.drag"));
      handleFor(g, "person", p);
      const row = h(
        "tr",
        { class: "person-row", "data-name": p.name.toLowerCase(), "data-person": String(p.id) },
        h("th", { scope: "row" }, h("div", { class: "rl" }, g, h(
          "button",
          {
            type: "button",
            class: "cellbtn" + (mine.length ? "" : " no-tasks"),
            title: mine.length ? mine.join(", ") : t("table.choose_tasks"),
            "aria-label": t("table.person_aria", { name: p.name }),
            onclick: () => personDialog(p)
          },
          p.name
        ), autoBtn(p)), tags.length ? h("div", { class: "tags" }, tags) : ""),
        events.map((ev) => answerCell(p, ev)),
        rowPick("answers", p.id, p.name)
      );
      return dropOn(row, (k, d) => k === "person" && d.id !== p.id, (d, after) => {
        const others = inSection(p.section).filter((x) => x.id !== d.id);
        return movePerson(d, p.section, others.findIndex((x) => x.id === p.id) + (after ? 1 : 0));
      });
    };
    const sectionRow = (x, i) => {
      const g = grip(t("section.drag"));
      const n = inSection(x.id).length;
      const row = h(
        "tr",
        { class: "person-sect" },
        h("th", { scope: "row" }, h(
          "div",
          { class: "rl" },
          g,
          foldBtn("s" + x.id, x.name),
          nameField(
            x.name,
            t("section.name_aria", { name: x.name }),
            t("section.name_hint"),
            (v) => api("PATCH", `${base}/sections/${x.id}`, { name: v }),
            async () => {
              await confirmed(t("section.delete"), t("section.delete_confirm", { name: x.name }), t("common.delete"));
              return api("DELETE", `${base}/sections/${x.id}`);
            }
          )
        )),
        h("td", { colspan: String(cols + 1) }, h("span", { class: "small" }, n ? tn("section.people", n) : t("section.empty")))
      );
      dragRow(row, g, "section", x, sections, (d, pos) => api("PUT", `${base}/sections/${d.id}/position`, { position: pos }));
      const asSection = row._drop;
      row._drop = {
        accept: (k, d) => k === "person" || asSection.accept(k, d),
        act: (d, after, kind) => kind === "person" ? movePerson(d, x.id, inSection(x.id).filter((o) => o.id !== d.id).length) : asSection.act(d, after),
        get into() {
          return rowDrag && rowDrag.kind === "person";
        }
      };
      return row;
    };
    if (done) bar.querySelector(".row-inline").append(
      view.weak.length ? linkBtn(tn("draft.compromises", view.weak.length), () => listDialog(tn("draft.compromises", view.weak.length), view.weak)) : "",
      view.load.length ? linkBtn(t("draft.distribution"), () => listDialog(t("draft.distribution"), view.load.map((l) => t("run.load_line", { name: l.name, count: l.count, target: l.target.toLocaleString(locale()) })))) : "",
      runs.length > 1 ? linkBtn(t("draft.earlier"), settingsDialog) : ""
    );
    if (org.level !== "view") bar.querySelector(".row-inline").append(h("button", { type: "button", class: "link-button prio-link", title: prioritySummary(weights), onclick: prioritiesDialog }, t("weights.title")));
    const addPerson = (x) => addRow(
      x ? t("table.new_person_in", { section: x.name }) : t("table.new_person"),
      (name) => api("POST", base + "/people", x ? { name, section: x.id } : { name })
    );
    const addSection = () => addRow(t("table.new_section"), (name) => api("POST", base + "/sections", { name }));
    const detailsHead = sect("details", t("table.details"), [t("table.details_hint")]);
    const tasksHead = dropOn(
      sect("tasks", t("table.tasks"), [t("table.tasks_hint")]),
      (k) => k === "task",
      (d) => moveTask(d, 0, tasksIn(0).filter((o) => o.id !== d.id).length),
      true
    );
    const addTask = (x) => addRow(
      x ? t("table.new_task_in", { section: x.name }) : t("table.new_task"),
      (name) => api("POST", base + "/tasks", x ? { name, section: x.id } : { name })
    );
    const addTaskSection = () => addRow(t("tsect.new"), (name) => api("POST", base + "/task-sections", { name }));
    let availHead;
    const table = h("div", { class: "grid-scroll main" }, h(
      "table",
      { class: "dp-table" + (store.get("tags") === "1" ? " show-tags" : "") },
      h("thead", {}, head),
      h(
        "tbody",
        {},
        detailsHead,
        folds("details", adds("field", fields.fields.map(fieldRow), detailsHead, [addRow(t("table.new_field"), (name) => api("POST", pbase + "/fields", { name }))])),
        tasksHead,
        folds("tasks", [
          adds("task:0", tasksIn(0).map(taskRow), tasksHead, [addTask(), ...taskSections.length ? [] : [addTaskSection()]]),
          taskSections.flatMap((x, i) => {
            const head2 = taskSectRow(x);
            return [head2, ...folds("ts" + x.id, adds("task:" + x.id, tasksIn(x.id).map(taskRow), head2, [addTask(x), ...i === taskSections.length - 1 ? [addTaskSection()] : []]))];
          })
        ]),
        h("tr", { class: "sect draft" }, h("td", { colspan: String(cols + 2) }, bar)),
        folds("draft", [
          tasks.map((tk, i) => [
            // the task sections' headings in the draft too, so it reads as the tasks above
            tk.section && (i === 0 || tasks[i - 1].section !== tk.section) ? h("tr", { class: "draft-sect" }, h("th", { scope: "row" }, (taskSections.find((x) => x.id === tk.section) || {}).name || ""), h("td", { colspan: String(cols + 1) })) : "",
            h(
              "tr",
              { class: "draft-row" },
              h("th", { scope: "row" }, org.level !== "view" ? h(
                "div",
                { class: "rl" },
                h("span", { class: "nm" }, tk.name),
                groupLock(events.map((ev) => [ev, tk]), t("lock.row", { name: tk.name }), t("unlock.row", { name: tk.name }))
              ) : tk.name),
              events.map((ev) => putDraftCell(ev, tk)),
              rowPick("draft", tk.id, tk.name)
            )
          ]),
          // a lock per date, under the draft
          org.level !== "view" && done ? h(
            "tr",
            { class: "draft-locks" },
            h("th", { scope: "row" }, h("span", { class: "small muted" }, t("lock.col_head"))),
            events.map((ev) => h("td", {}, groupLock(tasks.map((tk) => [ev, tk]), t("lock.col", { date: label(ev) }), t("unlock.col", { date: label(ev) })))),
            h("td", {})
          ) : ""
        ]),
        [
          availHead = dropOn(
            h(
              "tr",
              { class: "sect" },
              h(
                "th",
                { scope: "row" },
                h("div", { class: "rl" }, foldBtn("avail", t("table.availability")), t("table.availability"), infoBtn(t("table.availability"), [t("table.availability_info"), t("sel.info")])),
                people.length > 1 ? whoBtn : "",
                people.length ? tagsBtn : "",
                people.length > 8 ? filterBox() : ""
              ),
              events.map((ev) => countCells[ev.id] = h("td", { class: "count small", title: t("sel.column_hint"), onclick: (e) => selectLine("answers", e, (c) => c.ev.id === ev.id) })),
              filler()
            ),
            (k) => k === "person",
            (d) => movePerson(d, 0, inSection(0).filter((o) => o.id !== d.id).length),
            true
          ),
          folds("avail", [
            adds("person:0", inSection(0).map(personRow), availHead, [addPerson(), ...sections.length ? [] : [addSection()]]),
            sections.flatMap((x, i) => {
              const head2 = sectionRow(x, i);
              return [head2, ...folds("s" + x.id, adds("person:" + x.id, inSection(x.id).map(personRow), head2, [addPerson(x), ...i === sections.length - 1 ? [addSection()] : []]))];
            })
          ])
        ]
      )
    ));
    applyFolds(table);
    table.addEventListener("pointerdown", onCellDown);
    const stick = () => {
      const tb = table.querySelector("table");
      let top = tb.tHead.offsetHeight;
      for (const tr of tb.querySelectorAll("tr.field-row.frozen")) {
        tr.style.setProperty("--stick", top + "px");
        top += tr.offsetHeight;
      }
    };
    new ResizeObserver(stick).observe(table.querySelector("table"));
    if (planState.sel) {
      const keys = new Set(planState.sel.keys.filter((k) => grids.draft.cells.has(k)));
      if (keys.size) {
        sel = { grid: "draft", keys, anchor: planState.sel.anchor, base: new Set(keys) };
        paintSel();
      }
      planState.sel = null;
    }
    events.forEach((ev) => recount(ev.id));
    const root = h(
      "div",
      { class: "plan-page" },
      h(
        "div",
        { class: "title-bar" },
        back(),
        h("h1", {}, plan.name),
        infoBtn(plan.name, [t("table.hint")]),
        modeSwitch,
        events.length ? sideways(table, events) : "",
        shareBtn(),
        h("button", { type: "button", class: "ghost settings", "aria-label": t("settings.open"), onclick: settingsDialog }, "⚙", h("span", { class: "lbl" }, " " + t("settings.open")))
      ),
      viewOnly(org),
      events.length ? "" : h("p", { class: "muted small" }, t("table.no_events")),
      table,
      selbar,
      notePop
    );
    dragScroll(table);
    table.addEventListener("pointerdown", onRowDown);
    const editing = planState.edit ? events.flatMap((ev) => tasks.map((tk) => [ev, tk])).find(([ev, tk]) => at(ev, tk) === planState.edit) : void 0;
    planState.edit = null;
    root.shown = () => {
      if (editing && draftTd[at(...editing)]) editCell(...editing, draftTd[at(...editing)], planState.editInput ? planState.editInput.value : "");
      return reopenedPage();
    };
    const reopenedPage = () => reopened({
      task: (id) => {
        const x = tasks.find((tk) => tk.id === id);
        if (x) taskDialog(x);
      },
      person: (id) => {
        const x = people.find((p) => p.id === id);
        if (x) personDialog(x);
      },
      place: (k) => {
        const [e, tk] = k.split("/").map(Number);
        const ev = events.find((x) => x.id === e), task = tasks.find((x) => x.id === tk);
        if (ev && task) placeDialog(ev, task);
      },
      settings: () => settingsDialog(),
      priorities: () => prioritiesDialog()
    });
    return root;
  }

  // web/src/main.ts
  var renderSeq = 0;
  var lastPath = "";
  async function render() {
    const seq = ++renderSeq;
    const main = document.getElementById("main");
    const r = route();
    try {
      const who = await me();
      if (seq !== renderSeq) return;
      if (who.login && !who.user && (!(who.links || []).length || new URLSearchParams(location.search).has("invite"))) {
        headerSignedOut();
        main.replaceChildren(pageLanding());
        document.title = t("brand");
        if (new URLSearchParams(location.search).get("signin") === "failed") say(t("landing.failed"), "err");
        return;
      }
      let people = [];
      let org = null;
      if (r.org) {
        [org, people] = await Promise.all([get("/orgs/" + enc(r.org)), get(`/orgs/${enc(r.org)}/people`)]);
        store.set("org", r.org);
      }
      if (r.page === "none") throw Object.assign(new Error(t("page.not_found")), { status: 404, keepOrg: !!org });
      const [, body] = await Promise.all([renderHeader(r), r.page === "home" ? pageHome() : r.page === "org" ? pageOrg(r, org) : r.page === "members" ? pageMembers(r, org) : r.page === "settings" ? pageSettings(r, org) : pagePlan(r, org, people)]);
      if (seq !== renderSeq || !body) return;
      const same = location.pathname === lastPath;
      const y = same ? window.scrollY : 0;
      const sideways3 = same ? [...main.querySelectorAll(".grid-scroll")].map((g) => [g.scrollLeft, g.scrollTop]) : [];
      const focus = same ? focusKey(main) : null;
      main.replaceChildren(body);
      lastPath = location.pathname;
      main.querySelectorAll(".grid-scroll").forEach((g, i) => {
        if (sideways3[i]) [g.scrollLeft, g.scrollTop] = sideways3[i];
      });
      window.scrollTo(0, y);
      refocus(main, focus);
      if (body.shown) body.shown();
      else document.querySelectorAll("dialog.modal[open][data-key]").forEach((d) => d.close());
      follow(r.org);
      document.title = t("brand") + (org ? " · " + org.name : "");
    } catch (e) {
      if (seq !== renderSeq) return;
      if (e.connection && main.firstChild) return;
      if (e.status === 404 && r.org && !e.keepOrg) store.set("org", "");
      main.replaceChildren(h("div", {}, h("h1", {}, e.status === 404 ? t("page.not_found") : t("page.error")), h("p", {}, link("/?switch", t("page.choose_org"))), h("p", { class: "muted small" }, errorText(e))));
    }
  }
  var live2 = null;
  var liveTimer = 0;
  function follow(org) {
    if (live2 && live2.org === org) return;
    if (live2) {
      live2.es.close();
      fetched.clear();
    }
    live2 = null;
    if (!org || typeof EventSource === "undefined") return;
    const es = new EventSource(`/api/orgs/${enc(org)}/stream`);
    let broken = false;
    let fails = 0;
    es.addEventListener("message", (e) => {
      let c = {};
      try {
        c = JSON.parse(e.data);
      } catch (err) {
      }
      if (c.origin === CLIENT) return;
      forget(c.proc ? changedBy(c.proc) : null, org);
      liveRefresh();
    });
    es.addEventListener("error", () => {
      broken = true;
      if (fails === 0) probe();
      if (++fails < 5 || !live2 || live2.es !== es) return;
      es.close();
      live2 = null;
      const retry = () => {
        clearTimeout(timer);
        document.removeEventListener("visibilitychange", onShow);
        if (!live2) {
          fetched.clear();
          follow(org);
          liveRefresh();
        }
      };
      const onShow = () => {
        if (document.visibilityState === "visible") retry();
      };
      const timer = setTimeout(retry, 6e4);
      document.addEventListener("visibilitychange", onShow);
    });
    es.addEventListener("open", () => {
      if (broken) {
        fetched.clear();
        liveRefresh();
      }
      broken = false;
      fails = 0;
    });
    live2 = { org, es };
  }
  var typing2 = () => {
    const a = document.activeElement;
    return !!a && (a.tagName === "TEXTAREA" || a.tagName === "INPUT" && !["checkbox", "radio", "button", "submit"].includes(a.type));
  };
  function liveRefresh() {
    clearTimeout(liveTimer);
    liveTimer = setTimeout(() => {
      if (typing2()) {
        document.activeElement.addEventListener("blur", liveRefresh, { once: true });
        return;
      }
      render();
    }, 150);
  }
  window.addEventListener("popstate", render);
  render();
})();
