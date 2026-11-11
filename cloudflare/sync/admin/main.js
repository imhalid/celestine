// The admin panel of the sync Worker, drawn with synth-ui on a canvas: the waitlist by status, the accounts and the
// log, behind the admin key. It talks to /api/admin/* (auth.js) as the panel before it did. The page loads this as
// a module from the Worker itself (no script of another site on the page that holds the key); the backend's
// WebAssembly is fetched beside it.
import { Palette } from '@synth-ui/core';
import { createViewHost } from '@synth-ui/backend';
import { THEME_COLORS, button, card, sheet, statusLine, tabs, textField, useTheme } from '@synth-ui/widgets';
import { FONTS } from './fonts.js';
import { badge, cellText, codeTiles, secretField, table } from './ui.js';

const palette = new Palette([
  ...THEME_COLORS,
  ['ok', '#6FBF73'],
  ['okbg', '#1D3021'],
  ['bad', '#E5605A'],
  ['badbg', '#3A1C19'],
]);

const TABS = [
  ['pending', 'BEKLEYENLER'],
  ['approved', 'ONAYLANANLAR'],
  ['rejected', 'REDDEDİLENLER'],
  ['users', 'HESAPLAR'],
  ['audit', 'KAYIT'],
];

// What the panel holds between frames: the key, the tab, what was read, what is being typed.
const app = {
  tok: read('adm'),
  tab: 0,
  rows: [],
  loaded: '',
  loading: false,
  error: '',
  message: null,
  at: '',
  code: null, // {email, code, emailed, mailError}: shown once
  form: { email: '', name: '' },
  key: { text: '' },
  confirm: null, // {text, label, run}
};
let host;

function read(k) {
  try {
    return sessionStorage.getItem(k) || '';
  } catch {
    return '';
  }
}
function keep(k, v) {
  try {
    if (v) sessionStorage.setItem(k, v);
    else sessionStorage.removeItem(k);
  } catch {}
}
const redraw = () => host?.invalidate();
const say = (text) => {
  app.message = { text, duration: 4 };
  redraw();
};

async function api(path, method = 'GET', body) {
  const r = await fetch('/api/admin/' + path, {
    method,
    headers: { authorization: 'Bearer ' + app.tok, 'content-type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  });
  const j = await r.json().catch(() => ({}));
  if (r.status === 401) {
    signOut();
    throw new Error('ANAHTAR YANLIŞ');
  }
  if (!r.ok) throw new Error(j.error || 'HATA ' + r.status);
  return j;
}

// load reads the tab shown.
async function load() {
  const [tab] = TABS[app.tab];
  app.loading = true;
  app.error = '';
  redraw();
  try {
    const path = tab === 'users' ? 'users' : tab === 'audit' ? 'audit' : 'waitlist?status=' + tab;
    app.rows = await api(path);
    app.loaded = tab;
    app.at = new Date().toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' });
  } catch (e) {
    app.error = e.message;
  }
  app.loading = false;
  redraw();
}

// act runs what a button does, says what went wrong, and reads the tab again.
async function act(fn) {
  try {
    await fn();
  } catch (e) {
    say(e.message);
  }
  await load();
}

// the waitlist as CSV, fetched with the key (a plain link would not carry it) and saved as a file
async function csv() {
  const r = await fetch('/api/admin/waitlist.csv', { headers: { authorization: 'Bearer ' + app.tok } });
  if (!r.ok) throw new Error('HATA ' + r.status);
  const a = document.createElement('a');
  a.href = URL.createObjectURL(await r.blob());
  a.download = 'celestine-waitlist-' + new Date().toISOString().slice(0, 10) + '.csv';
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(a.href), 1000);
}

function signIn() {
  const t = app.key.text.trim();
  app.key.text = '';
  if (!t) return;
  app.tok = t;
  keep('adm', t);
  load();
}
function signOut() {
  app.tok = '';
  app.rows = [];
  app.code = null;
  keep('adm', '');
  redraw();
}

const date = (s) => (s ? new Date(s * 1000).toLocaleString('tr-TR', { dateStyle: 'short', timeStyle: 'short' }) : '');
const mb = (n) => (n / 1048576).toFixed(1) + ' MB';
const approve = (x) => act(async () => (app.code = { email: x.email, ...(await api('waitlist/' + x.id + '/approve', 'POST')) }));

// ---- the frame ----

function paint(ctx) {
  const { colors: c } = useTheme(ctx);
  ctx.fillRect(ctx.bounds, c.bg);
  ctx.inset(14, 10, 14, 8);
  header(ctx);
  if (!app.tok) login(ctx);
  else desk(ctx);
  confirmSheet(ctx);
}

function header(ctx) {
  const { colors: c } = useTheme(ctx);
  const h = ctx.cutTop(12, 10);
  ctx.fillRect({ x: h.x, y: h.y + 2, w: 5, h: 5 }, c.accent);
  const x = ctx.text('CELESTINE', h.x + 9, h.y + 1, { color: c.paper });
  ctx.text('YÖNETİM · KONTROL MASASI', x + 10, h.y + 2, { color: c.muted, font: 'small' });
  const now = new Date().toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  ctx.text(now, h.x + h.w - ctx.measureText(now), h.y + 1, { color: c.flame });
  ctx.hline(h.x, h.y + h.h + 4, h.w, c.line);
  ctx.requestFrame(); // the clock
}

function login(ctx) {
  const { colors: c } = useTheme(ctx);
  const w = Math.min(280, ctx.bounds.w), h = 74;
  const r = { x: Math.floor((ctx.bounds.w - w) / 2), y: Math.floor(ctx.bounds.h / 3 - h / 2), w, h };
  const box = card(ctx, { title: 'ERİŞİM', right: 'YETKİ GEREKLİ' }, r);
  box.column({ gap: 5 }, (col) => {
    col.place({ w: 1, h: 1 });
    const l = col.place({ w: col.bounds.w, h: 6 });
    col.text('YÖNETİCİ ANAHTARI', l.x, l.y, { color: c.muted, font: 'small' });
    const f = secretField(col, { key: 'tok', state: app.key, placeholder: 'ANAHTAR' });
    col.row({ gap: 4, align: 'end' }, (row) => {
      row.cursor.x = row.bounds.w - 46;
      if (button(row, { label: 'GİR ▸', h: 13 }).clicked || f.submitted) signIn();
    });
  });
}

function desk(ctx) {
  const { colors: c } = useTheme(ctx);
  if (!app.loading && !app.error && app.loaded !== TABS[app.tab][0]) load();
  // the status line at the foot
  const foot = ctx.cutBottom(10, 6);
  const [, title] = TABS[app.tab];
  const text = app.error ? 'BAĞLANTI YOK · ' + app.error : app.loading ? 'OKUNUYOR…' : `${title} · ${app.rows.length} SATIR · GÜNCELLENDİ ${app.at}`;
  statusLine(ctx, { text, message: app.message, key: 'status', color: app.error ? c.accent : c.muted }, foot);
  // the tabs, and the way out
  const top = ctx.cutTop(13, 8);
  const sel = tabs(ctx, { tabs: TABS.map(([, l], i) => ({ label: `0${i + 1} ${l}` })), selected: app.tab, key: 'tabs', h: 13 }, { x: top.x, y: top.y, w: top.w - 60, h: 13 });
  if (sel !== null && sel !== app.tab) {
    app.tab = sel;
    app.code = null;
    load();
  }
  if (button(ctx, { label: 'ÇIKIŞ', h: 13 }, { x: top.x + top.w - 52, y: top.y, w: 52, h: 13 }).clicked) signOut();
  if (app.code) codeCard(ctx);
  const tab = TABS[app.tab][0];
  if (tab === 'pending') addCard(ctx);
  if (tab !== 'users' && tab !== 'audit') {
    const bar = ctx.cutTop(12, 6);
    if (button(ctx, { label: 'LİSTEYİ CSV İNDİR', h: 12 }, { x: bar.x + bar.w - 104, y: bar.y, w: 104, h: 12 }).clicked) csv().catch((e) => say(e.message));
  }
  rows(ctx, tab);
}

// codeCard shows the invite code made just now: once, with a way to copy it.
function codeCard(ctx) {
  const { colors: c } = useTheme(ctx);
  const j = app.code;
  const box = card(ctx, { title: 'DAVET KODU', right: j.email }, ctx.cutTop(84, 8));
  box.column({ gap: 6 }, (col) => {
    col.place({ w: 1, h: 1 });
    codeTiles(col, j.code || '');
    const note = j.emailed ? 'E-POSTA GÖNDERİLDİ.' : `E-POSTA GİTMEDİ (${j.mailError || '?'}): KODU ELLE İLET. BU KOD BİR DAHA GÖSTERİLMEZ.`;
    const n = col.place({ w: col.bounds.w, h: 6 });
    col.text(note, n.x, n.y, { color: j.emailed ? c.text : c.flame, font: 'small' });
    col.row({ gap: 4 }, (row) => {
      if (button(row, { label: 'KODU KOPYALA', h: 12 }).clicked) navigator.clipboard.writeText(j.code).then(() => say('KOD KOPYALANDI'), (e) => say(e.message));
      if (button(row, { label: 'KAPAT', h: 12 }).clicked) app.code = null;
    });
  });
}

// addCard puts a person on the list by hand; "Ekle ve onayla" makes the code at once.
function addCard(ctx) {
  const box = card(ctx, { title: 'ELLE EKLE', right: 'LİSTEYE YAZ' }, ctx.cutTop(32, 8));
  box.row({ gap: 6, align: 'center' }, (row) => {
    const take = (res, k) => {
      if (res.committed != null) app.form[k] = res.committed;
      else if (res.editing) app.form[k] = res.text;
    };
    take(textField(row, { key: 'add-email', value: app.form.email, placeholder: 'E-POSTA', w: 180 }), 'email');
    take(textField(row, { key: 'add-name', value: app.form.name, placeholder: 'AD (İSTEĞE BAĞLI)', w: 130 }), 'name');
    const add = async (andApprove) => {
      const email = app.form.email.trim();
      if (!email) throw new Error('E-POSTA YAZ');
      const w = await api('waitlist', 'POST', { email, name: app.form.name.trim() });
      app.form = { email: '', name: '' };
      if (andApprove) app.code = { email, ...(await api('waitlist/' + w.id + '/approve', 'POST')) };
    };
    if (button(row, { label: 'EKLE', h: 12 }).clicked) act(() => add(false));
    if (button(row, { label: 'EKLE VE ONAYLA', h: 12, on: true }).clicked) act(() => add(true));
  });
}

// rows is the table of the tab shown.
function rows(ctx, tab) {
  const { colors: c } = useTheme(ctx);
  const btns = (cc, list) =>
    cc.row({ gap: 3 }, (row) => {
      row.cursor.y = 1;
      for (const [label, fn, on] of list) if (button(row, { label, h: 11, on }).clicked) fn();
    });
  if (tab === 'users') {
    const cols = [{ title: 'KULLANICI' }, { title: 'DURUM', w: 54 }, { title: 'ALAN', w: 120 }, { title: 'CİHAZ', w: 36 }, { title: 'KAYIT', w: 96 }, { title: '', w: 112 }];
    table(ctx, { key: 'users', columns: cols, rows: app.rows, empty: 'HESAP YOK' }, (cc, u, col) => {
      switch (col.title) {
        case 'KULLANICI': return cellText(cc, u.username, c.paper);
        case 'DURUM': return badge(cc, u.status === 'active' ? 'AKTİF' : 'ASKIDA', u.status === 'active' ? 'ok' : 'bad', { x: 0, y: 3 });
        case 'ALAN': return cellText(cc, mb(u.bytes) + ' / ' + mb(u.quota));
        case 'CİHAZ': return cellText(cc, String(u.devices));
        case 'KAYIT': return cellText(cc, date(u.created_at), c.muted);
        default:
          return btns(cc, [
            u.status === 'active' ? ['ASKIYA AL', () => act(() => api('users/' + u.id + '/disable', 'POST'))] : ['AÇ', () => act(() => api('users/' + u.id + '/enable', 'POST'))],
            ['SİL', () => ask({ text: u.username + ' VE TÜM VERİSİ SİLİNSİN Mİ? GERİ ALINAMAZ.', label: 'SİL', run: () => api('users/' + u.id + '/delete', 'POST') })],
          ]);
      }
    });
    return;
  }
  if (tab === 'audit') {
    table(ctx, { key: 'audit', columns: [{ title: 'ZAMAN', w: 100 }, { title: 'İŞLEM' }], rows: app.rows, empty: 'KAYIT YOK' }, (cc, x, col) =>
      col.title === 'ZAMAN' ? cellText(cc, date(x.at), c.muted) : cellText(cc, x.what),
    );
    return;
  }
  const cols = [{ title: 'E-POSTA' }, { title: 'AD', w: 110 }, { title: 'YAZILDI', w: 96 }, { title: 'DURUM', w: 130 }, { title: '', w: 150 }];
  table(ctx, { key: 'waitlist:' + tab, columns: cols, rows: app.rows, empty: 'KİMSE YOK' }, (cc, x, col) => {
    switch (col.title) {
      case 'E-POSTA': return cellText(cc, x.email, c.paper);
      case 'AD': return cellText(cc, x.name || '—');
      case 'YAZILDI': return cellText(cc, date(x.created_at), c.muted);
      case 'DURUM': {
        const [t, tone] =
          x.status === 'approved' ? (x.used_at ? ['HESAP AÇTI', 'ok'] : x.invite_until ? ['KOD ' + date(x.invite_until) + ' KADAR', 'wait'] : ['KODUN SÜRESİ DOLDU', 'off'])
          : x.status === 'pending' ? ['BEKLİYOR', 'wait'] : ['REDDEDİLDİ', 'bad'];
        return badge(cc, t, tone, { x: 0, y: 3 });
      }
      default: {
        const list = [];
        if (x.status === 'pending' || x.status === 'rejected') list.push(['ONAYLA', () => approve(x), x.status === 'pending']);
        if (x.status === 'approved' && !x.used_at) list.push(['YENİ KOD', () => act(async () => (app.code = { email: x.email, ...(await api('waitlist/' + x.id + '/resend', 'POST')) }))]);
        if (x.status !== 'rejected' && !x.used_at) list.push(['REDDET', () => act(() => api('waitlist/' + x.id + '/reject', 'POST'))]);
        if (!x.used_at) list.push(['SİL', () => ask({ text: x.email + ' LİSTEDEN VE KAYITTAN SİLİNSİN Mİ? GERİ ALINAMAZ.', label: 'SİL', run: () => api('waitlist/' + x.id + '/delete', 'POST') })]);
        return btns(cc, list);
      }
    }
  });
}

// ask opens the question after the frame of the click that asked it: in that frame the click's release would
// fall outside the sheet and close it at once.
const ask = (q) =>
  setTimeout(() => {
    app.confirm = q;
    redraw();
  });

// confirmSheet asks before what cannot be undone.
function confirmSheet(ctx) {
  const { colors: c } = useTheme(ctx);
  const q = app.confirm;
  const res = sheet(ctx, { open: !!q, title: 'EMİN MİSİN?', w: 280, h: 66, key: 'confirm' }, (s) => {
    if (!q) return;
    const lay = s.layoutText(q.text, { font: 'small', color: c.text, width: s.bounds.w, wrap: 'word', maxLines: 3 });
    s.drawText(lay, 0, 0);
    s.row({ gap: 4 }, (row) => {
      row.cursor.y = s.bounds.h - 13;
      row.cursor.x = s.bounds.w - 110;
      if (button(row, { label: 'VAZGEÇ', h: 12 }).clicked) app.confirm = null;
      if (button(row, { label: q.label, h: 12, on: true }).clicked) {
        app.confirm = null;
        act(q.run);
      }
    });
  });
  if (res.dismissed) app.confirm = null;
}

host = await createViewHost(document.getElementById('panel'), paint, {
  palette,
  fonts: FONTS,
  pixelScale: 2,
  redraw: 'auto',
  label: 'Celestine yönetim paneli',
});
if (app.tok) load();
