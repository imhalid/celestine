// The dashboard of the admin panel (its GÖSTERGE tab): Cloudflare's figures for the Workers, the D1 database and
// the R2 buckets of Celestine, where its requests come from, and the app's own figures beside them, over a day, a
// week or a month. The figures come from /api/admin/cloudflare and /api/admin/figures (analytics.js); this draws
// them, in one scrolling column of cards. Every figure that can be measured against something (a limit of the free
// plan, the whole it is part of) has a bar and its share; every status code says in words what it means.
import { button, card, scrollView, segmented, useTheme } from '@synth-ui/widgets';
import { SMALL_FACE } from './fonts.js';

/** The ranges offered, as analytics.js names them, with their length in days. */
export const DASH_RANGES = [
  { value: '24h', label: '24 SAAT', days: 1 },
  { value: '7d', label: '7 GÜN', days: 7 },
  { value: '30d', label: '30 GÜN', days: 30 },
];

/** The Workers, each with its name here and its colour (a palette name). */
const SCRIPTS = [
  ['celestine-sync', 'SENKRON', 'accent'],
  ['celestine-updates', 'GÜNCELLEME', 'blue'],
  ['celestine-site', 'SİTE', 'ok'],
];
const BUCKETS = [
  ['celestine-sync', 'SENKRON'],
  ['celestine-releases', 'SÜRÜMLER'],
];

/**
 * FREE is what Cloudflare's free plan allows, in its own (decimal) units: Workers requests and D1 rows by the day,
 * R2's operations by the month, storage at any time. The bars measure the range against these.
 */
const FREE = { requests: 100e3, rowsRead: 5e6, rowsWritten: 100e3, d1: 5e9, r2: 10e9, classA: 1e6, classB: 10e6 };

/** STATUS says what an HTTP status code means, in a few words. */
const STATUS = {
  0: 'YANITSIZ: BAĞLANTI KOPTU',
  200: 'TAMAM',
  201: 'OLUŞTURULDU',
  204: 'BOŞ YANIT',
  206: 'PARÇA',
  301: 'KALICI YÖNLENDİRME',
  302: 'GEÇİCİ YÖNLENDİRME',
  304: 'DEĞİŞMEDİ',
  400: 'BOZUK İSTEK',
  401: 'ANAHTAR YOK YA DA YANLIŞ',
  403: 'YASAK',
  404: 'BULUNAMADI',
  405: 'BU YÖNTEM YOK',
  409: 'ÇAKIŞMA',
  413: 'ÇOK BÜYÜK',
  429: 'ÇOK FAZLA İSTEK',
  500: 'SUNUCU HATASI',
  502: 'ARKADAKİ SUNUCU BOZUK YANIT VERDİ',
  503: 'HİZMET YOK',
  504: 'ZAMAN AŞIMI',
};

/** CLASSES are the kinds of status code, in the order the bars put them, each with its words and colour. */
const CLASSES = [
  ['ok', 'BAŞARILI', 'ok', '2XX: İSTEK KARŞILANDI'],
  ['redirect', 'YÖNLENDİRME', 'blue', '3XX: BAŞKA ADRESE GÖNDERİLDİ'],
  ['client', 'İSTEMCİ HATASI', 'flame', '4XX: OLMAYAN YA DA İZİNSİZ BİR ADRES İSTENDİ (ÇOĞU, ADRES YOKLAYAN BOTLAR)'],
  ['server', 'SUNUCU HATASI', 'bad', '5XX: SUNUCU YANIT VEREMEDİ'],
  ['none', 'YANITSIZ', 'dim', 'YANIT GİTMEDEN BAĞLANTI KOPTU'],
];
const classOf = (status) => (!status ? 'none' : status >= 500 ? 'server' : status >= 400 ? 'client' : status >= 300 ? 'redirect' : 'ok');

// ---- numbers as the panel shows them ----

/** num is a count, short: 1234, 12.3K, 4.5M. */
export const num = (n) => {
  n = Number(n) || 0;
  const a = Math.abs(n);
  return a >= 1e9 ? (n / 1e9).toFixed(1) + 'G' : a >= 1e6 ? (n / 1e6).toFixed(1) + 'M' : a >= 1e4 ? (n / 1e3).toFixed(1) + 'K' : String(Math.round(n));
};
/** bytes is a size in Cloudflare's decimal units, as its dashboard shows them. */
export const bytes = (n) => {
  n = Number(n) || 0;
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0;
  while (n >= 1000 && i < units.length - 1) {
    n /= 1000;
    i++;
  }
  return (i && n < 10 ? n.toFixed(1) : Math.round(n)) + ' ' + units[i];
};
/** ms is a time given in microseconds (the Workers' CPU and wall time), in milliseconds. */
const ms = (us) => {
  const m = (Number(us) || 0) / 1000;
  return (m < 10 ? m.toFixed(2) : Math.round(m)) + ' MS';
};
/** pct is a share as Turkish writes it: %12, %4.5, and <%0.1 for what is there but too little to show. */
const pct = (x) => {
  const p = (Number(x) || 0) * 100;
  if (p > 0 && p < 0.1) return '<%0.1';
  return '%' + (p > 0 && p < 10 ? p.toFixed(1) : Math.round(p));
};
/** stepLabel names a step of a series: an hour with its day, or a day. */
const stepLabel = (s, step) => {
  const d = new Date(step === 'date' ? s + 'T00:00:00Z' : s);
  return step === 'date'
    ? d.toLocaleDateString('tr-TR', { day: '2-digit', month: '2-digit' })
    : d.toLocaleString('tr-TR', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' }).replace(',', '');
};
const sum = (a) => a.reduce((s, x) => s + x, 0);

/** countryName is a country's Turkish name by its code, in the letters the panel's face has (TR: TÜRKİYE). */
const regions = (() => {
  try {
    return new Intl.DisplayNames(['tr'], { type: 'region' });
  } catch {
    return null;
  }
})();
const countryName = (code) => {
  let name = code;
  try {
    name = regions?.of(code) ?? code;
  } catch {}
  return [...name.toLocaleUpperCase('tr-TR')].map((ch) => (SMALL_FACE.glyphs[ch] ? ch : ch.normalize('NFD')[0])).join('');
};

// ---- the pieces ----

// tip is the tooltip of what is under the pointer this frame, set by the pieces as they are drawn.
let tip;
/** hover offers `text` as the tooltip while the pointer is over `r`; returns whether it is. */
const hover = (ctx, r, text) => {
  const on = ctx.containsPointer(r);
  if (on) tip = text;
  return on;
};

/** level is a bar's colour by how full it is: green, flame past half, red past four fifths. */
const level = (ctx, share) => {
  const pal = ctx.palette.index;
  return share >= 0.8 ? pal.bad : share >= 0.5 ? pal.flame : pal.ok;
};

/** meter is a bar of a share of something: a track, and a fill at least a pixel wide when there is anything. */
function meter(ctx, r, share, color) {
  const { colors: c } = useTheme(ctx);
  ctx.fillRect(r, c.raised);
  const s = Math.min(1, Math.max(0, share || 0));
  if (s > 0) ctx.fillRect({ x: r.x, y: r.y, w: Math.max(1, Math.round(s * r.w)), h: r.h }, color ?? level(ctx, s));
}

/**
 * tile is one large figure: its name (and a note at the right), the value, and a bar of what it is measured
 * against, with the share in words under it. The words show whole as a tooltip when they do not fit.
 */
function tile(ctx, r, t) {
  const { colors: c } = useTheme(ctx);
  ctx.fillRect(r, c.panel);
  ctx.strokeRect(r, c.line);
  ctx.text(t.label, r.x + 5, r.y + 4, { color: c.muted, font: 'small' });
  if (t.right) ctx.text(t.right, r.x + r.w - 5 - ctx.measureText(t.right, { font: 'small' }), r.y + 4, { color: t.rightColor ?? c.dim, font: 'small' });
  const scale = ctx.measureText(t.value) * 2 <= r.w - 10 ? 2 : 1;
  ctx.text(t.value, r.x + 5, r.y + 13, { color: t.color ?? c.paper, scale });
  if (t.share != null) meter(ctx, { x: r.x + 5, y: r.y + 35, w: r.w - 10, h: 4 }, t.share, t.barColor);
  if (t.sub) ctx.text(t.sub, r.x + 5, r.y + 43, { color: c.dim, font: 'small' });
  hover(ctx, r, t.tip ?? t.sub);
}

/** stat is a small figure in a grid: its name, and its value under it. */
function stat(ctx, x, y, label, value, color) {
  const { colors: c } = useTheme(ctx);
  ctx.text(label, x, y, { color: c.muted, font: 'small' });
  ctx.text(value, x, y + 8, { color: color ?? c.paper });
}

/** grid lays stats out in `cols` columns from x, y; returns the height it took. */
function grid(ctx, x, y, w, cols, items) {
  const cw = Math.floor(w / cols);
  items.forEach(([label, value, color], i) => stat(ctx, x + (i % cols) * cw, y + Math.floor(i / cols) * 22, label, value, color));
  return Math.ceil(items.length / cols) * 22;
}

/** figures is a line of named figures, the name dim and the value bright; returns where it ended. */
function figures(ctx, x, y, items) {
  const { colors: c } = useTheme(ctx);
  for (const [label, value, color] of items) {
    x = ctx.text(label, x, y, { color: c.muted, font: 'small' }) + 3;
    x = ctx.text(value, x, y, { color: color ?? c.paper, font: 'small' }) + 10;
  }
  return x;
}

/**
 * statusBar shows what the responses were: one bar split by status code, each code a piece in the colour of its
 * kind (and a tooltip that says what it means), and under it the kinds in words with their shares.
 */
function statusBar(ctx, r, codes) {
  const { colors: c } = useTheme(ctx);
  const pal = ctx.palette.index;
  const total = sum(codes.map((x) => x.requests));
  if (!total) {
    ctx.text('İSTEK YOK', r.x, r.y, { color: c.dim, font: 'small' });
    return;
  }
  const order = Object.fromEntries(CLASSES.map(([k], i) => [k, i]));
  const sorted = [...codes].sort((a, b) => order[classOf(a.status)] - order[classOf(b.status)] || b.requests - a.requests);
  const color = (k) => pal[CLASSES[order[k]][2]];
  // the bar: a piece a code, a pixel of ground between them
  let x = r.x;
  const room = r.w - (sorted.length - 1);
  sorted.forEach((s, i) => {
    const w = i === sorted.length - 1 ? r.x + r.w - x : Math.max(1, Math.round((s.requests / total) * room));
    const piece = { x, y: r.y, w, h: 6 };
    const on = hover(ctx, { ...piece, y: piece.y - 1, h: 8 }, `${s.status || 0} ${STATUS[s.status] ?? CLASSES[order[classOf(s.status)]][1]} · ${num(s.requests)} İSTEK · ${pct(s.requests / total)}`);
    ctx.fillRect(piece, on ? c.paper : color(classOf(s.status)));
    x += w + 1;
  });
  // the kinds, in words
  let lx = r.x;
  for (const [k, name, , meaning] of CLASSES) {
    const n = sum(sorted.filter((s) => classOf(s.status) === k).map((s) => s.requests));
    if (!n) continue;
    const t = name + ' ' + pct(n / total);
    const w = ctx.measureText(t, { font: 'small' }) + 8;
    if (lx + w > r.x + r.w) break;
    ctx.fillRect({ x: lx, y: r.y + 10, w: 4, h: 4 }, color(k));
    ctx.text(t, lx + 6, r.y + 9, { color: c.text, font: 'small' });
    hover(ctx, { x: lx, y: r.y + 8, w, h: 8 }, meaning + ' · ' + num(n) + ' İSTEK');
    lx += w + 8;
  }
}

/**
 * columns draws a series as stacked bars, a layer a colour, under its largest value; the step under the pointer is
 * lit and described by `describe(i)` as the tooltip.
 */
function columns(ctx, r, layers, steps, step, describe) {
  const { colors: c } = useTheme(ctx);
  const n = steps.length;
  const totals = steps.map((_, i) => sum(layers.map((l) => l.values[i] ?? 0)));
  const max = Math.max(1, ...totals);
  const gut = Math.max(ctx.measureText(num(max), { font: 'small' }), 8) + 4;
  const plot = { x: r.x + gut, y: r.y + 3, w: r.w - gut, h: r.h - 13 };
  ctx.text(num(max), r.x, plot.y - 2, { color: c.dim, font: 'small' });
  ctx.text('0', r.x, plot.y + plot.h - 5, { color: c.dim, font: 'small' });
  ctx.hline(plot.x, plot.y, plot.w, c.line);
  ctx.hline(plot.x, plot.y + Math.floor(plot.h / 2), plot.w, c.line);
  const slot = plot.w / n;
  const bw = Math.max(1, Math.floor(slot) - (slot >= 3 ? 1 : 0));
  for (let i = 0; i < n; i++) {
    const x = plot.x + Math.floor(i * slot);
    const hot = hover(ctx, { x, y: plot.y, w: Math.max(1, Math.ceil(slot)), h: plot.h }, stepLabel(steps[i], step) + ' · ' + describe(i));
    if (hot) ctx.fillRect({ x, y: plot.y, w: bw, h: plot.h }, c.raised);
    let y = plot.y + plot.h;
    for (const l of layers) {
      const v = l.values[i] ?? 0;
      if (!v) continue;
      const h = Math.max(1, Math.round((v / max) * plot.h));
      y -= h;
      ctx.fillRect({ x, y, w: bw, h }, hot ? c.paper : l.color);
    }
  }
  ctx.hline(plot.x, plot.y + plot.h, plot.w, c.border);
  // the first, middle and last steps' names under the axis
  const ly = plot.y + plot.h + 3;
  const first = stepLabel(steps[0], step), mid = stepLabel(steps[Math.floor(n / 2)], step), last = stepLabel(steps[n - 1], step);
  ctx.text(first, plot.x, ly, { color: c.dim, font: 'small' });
  ctx.text(mid, plot.x + Math.floor(plot.w / 2 - ctx.measureText(mid, { font: 'small' }) / 2), ly, { color: c.dim, font: 'small' });
  ctx.text(last, plot.x + plot.w - ctx.measureText(last, { font: 'small' }), ly, { color: c.dim, font: 'small' });
}

/** legend is a row of colour keys with their names and figures; returns where it ended. */
function legend(ctx, x, y, items) {
  const { colors: c } = useTheme(ctx);
  for (const [label, color, value] of items) {
    ctx.fillRect({ x, y: y + 1, w: 5, h: 5 }, color);
    x = ctx.text(label, x + 8, y, { color: c.muted, font: 'small' }) + 4;
    x = ctx.text(value, x, y, { color: c.text, font: 'small' }) + 12;
  }
  return x;
}

/** ranking draws rows of a name, a bar to scale and the figures at the right: each row's share of the whole. */
function ranking(ctx, r, rows, color, empty, lw) {
  const { colors: c } = useTheme(ctx);
  if (!rows.length) {
    ctx.text(empty, r.x, r.y + 2, { color: c.dim, font: 'small' });
    return;
  }
  const max = Math.max(1, ...rows.map((x) => x.value));
  const total = rows.total ?? sum(rows.map((x) => x.value));
  const rw = 70;
  rows.forEach((row, i) => {
    const y = r.y + i * 10;
    if (y + 8 > r.y + r.h) return;
    let label = row.label;
    while (label.length > 1 && ctx.measureText(label, { font: 'small' }) > lw - 4) label = label.slice(0, -1);
    ctx.text(label, r.x, y, { color: c.text, font: 'small' });
    const bw = r.w - lw - rw;
    ctx.fillRect({ x: r.x + lw, y: y + 1, w: bw, h: 5 }, c.raised);
    ctx.fillRect({ x: r.x + lw, y: y + 1, w: Math.max(1, Math.round((row.value / max) * bw)), h: 5 }, color);
    const right = num(row.value) + '  ' + pct(row.value / total);
    ctx.text(right, r.x + r.w - ctx.measureText(right, { font: 'small' }), y, { color: c.muted, font: 'small' });
    hover(ctx, { x: r.x, y, w: r.w, h: 9 }, row.tip ?? row.label + ' · ' + num(row.value) + ' İSTEK · ' + pct(row.value / total));
  });
}

/** note is a line of small text, wrapped to the width; returns the height it took. */
function note(ctx, text, x, y, w, color) {
  const lay = ctx.layoutText(text, { font: 'small', color, width: w, wrap: 'word' });
  ctx.drawText(lay, x, y);
  return lay.bounds.h;
}

// ---- the dashboard ----

/**
 * drawDash draws the dashboard into the rest of the region. `d` is what was read ({range, cf, fig, loading});
 * `on.range(value)` and `on.refresh()` are its two controls. Returns the tooltip of what is under the pointer.
 */
export function drawDash(ctx, d, on) {
  const { colors: c } = useTheme(ctx);
  const pal = ctx.palette.index;
  tip = undefined;
  const days = DASH_RANGES.find((r) => r.value === d.range)?.days ?? 1;
  const perDay = days === 1 ? 'GÜNDE' : days + ' GÜNDE';
  // the controls: the range, the time of the figures, and a way to read them again
  const bar = ctx.cutTop(13, 8);
  const r = segmented(ctx, { options: DASH_RANGES, value: d.range, key: 'dash-range', h: 13 }, { x: bar.x, y: bar.y, w: 150, h: 13 });
  if (r !== null && r !== d.range) on.range(r);
  if (button(ctx, { label: d.loading ? 'OKUNUYOR…' : 'YENİLE', h: 13 }, { x: bar.x + bar.w - 70, y: bar.y, w: 70, h: 13 }).clicked && !d.loading) on.refresh();
  const cf = d.cf, fig = d.fig;
  const when = cf?.at ? 'CLOUDFLARE ' + new Date(cf.at * 1000).toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' }) + ' · ÖRNEKLEMLİ, YAKLAŞIK · ÇUBUKLAR ÜCRETSİZ PLANIN SINIRLARINA GÖRE' : '';
  ctx.text(when, bar.x + 160, bar.y + 4, { color: c.dim, font: 'small' });

  scrollView(ctx, { key: 'dash', fade: 6 }, (s) => {
    const W = s.bounds.w;
    const half = Math.floor((W - 8) / 2);
    let y = 0;
    const gap = 8;

    if (!fig && !cf) {
      s.text(d.loading ? 'OKUNUYOR…' : 'VERİ YOK', 2, 4, { color: c.dim });
      s.place({ w: W, h: 20 });
      return;
    }
    const linked = cf?.configured === true;
    const ws = linked ? SCRIPTS.map(([k]) => cf.workers[k]) : [];
    const req = sum(ws.map((w) => w.requests));
    const err = sum(ws.map((w) => w.errors));
    const r2bytes = linked ? sum(BUCKETS.map(([k]) => cf.buckets[k].bytes)) : 0;
    const classA = linked ? sum(BUCKETS.map(([k]) => cf.buckets[k].classA)) : 0;
    const classB = linked ? sum(BUCKETS.map(([k]) => cf.buckets[k].classB)) : 0;

    // what is missing or went wrong with Cloudflare, said first
    if (!linked || cf.errors?.length || cf.error) {
      const text = cf?.error
        ? 'CLOUDFLARE OKUNAMADI: ' + cf.error
        : !linked
          ? "CLOUDFLARE'IN RAKAMLARI İÇİN WORKER'A İKİ GİZLİ DEĞER GEREK: CF_ANALYTICS_TOKEN (YALNIZ OKUYAN BİR API ANAHTARI: ACCOUNT ANALYTICS READ, ZONE ANALYTICS READ) VE CF_ACCOUNT_ID. ŞİMDİLİK YALNIZ UYGULAMANIN RAKAMLARI."
          : 'BAZI VERİLER GELMEDİ: ' + cf.errors.join(' · ');
      const box = card(s, { title: 'CLOUDFLARE', right: linked ? 'EKSİK' : 'BAĞLI DEĞİL' }, { x: 0, y, w: W, h: 40 });
      note(box, text, 0, 2, box.bounds.w, c.flame);
      y += 40 + gap;
    }

    // the large figures, each against what it is measured by
    const tiles = [];
    if (linked) {
      const reqLimit = FREE.requests * days;
      const rowsLimit = FREE.rowsRead * days;
      tiles.push(
        {
          label: 'İSTEK', value: num(req), share: req / reqLimit,
          right: 'HATA ' + pct(req ? err / req : 0), rightColor: err ? pal.bad : undefined,
          sub: num(reqLimit) + ' SINIRIN ' + pct(req / reqLimit),
          tip: "ÜÇ WORKER'IN İSTEĞİ. ÜCRETSİZ PLAN GÜNDE 100K İSTEK VERİR (TÜM HESAP İÇİN): " + perDay + ' ' + num(reqLimit) + "'IN " + pct(req / reqLimit),
        },
        {
          label: 'D1 SORGU', value: num(cf.d1.reads + cf.d1.writes), share: cf.d1.rowsRead / rowsLimit,
          right: num(cf.d1.reads) + ' O · ' + num(cf.d1.writes) + ' Y',
          sub: 'OKUNAN SATIR ' + pct(cf.d1.rowsRead / rowsLimit),
          tip: `${num(cf.d1.reads)} OKUMA, ${num(cf.d1.writes)} YAZMA SORGUSU. SINIR SORGUDA DEĞİL SATIRDA: GÜNDE 5M SATIR OKUNUR, 100K YAZILIR. ${num(cf.d1.rowsRead)} SATIR OKUNDU (${pct(cf.d1.rowsRead / rowsLimit)}), ${num(cf.d1.rowsWritten)} YAZILDI (${pct(cf.d1.rowsWritten / (FREE.rowsWritten * days))})`,
        },
        { label: 'D1 BOYUT', value: bytes(cf.d1.size), share: cf.d1.size / FREE.d1, sub: '5 GB SINIRIN ' + pct(cf.d1.size / FREE.d1), tip: 'VERİTABANININ BOYUTU. ÜCRETSİZ PLAN 5 GB VERİR' },
        { label: 'R2 DEPO', value: bytes(r2bytes), share: r2bytes / FREE.r2, sub: '10 GB SINIRIN ' + pct(r2bytes / FREE.r2), tip: 'İKİ KOVANIN TOPLAMI (SENKRON VE SÜRÜMLER). ÜCRETSİZ PLAN 10 GB VERİR' },
      );
    }
    if (fig) {
      const u = fig.users;
      const wlTotal = sum(['pending', 'approved', 'rejected'].map((k) => fig.waitlist[k].n));
      const wlNew = sum(['pending', 'approved', 'rejected'].map((k) => fig.waitlist[k].new));
      tiles.push(
        {
          label: 'HESAP', value: num(u.total), share: u.total ? fig.devices.activeUsers / u.total : 0, barColor: pal.blue,
          right: '+' + num(u.new) + ' YENİ', sub: 'AKTİF ' + pct(u.total ? fig.devices.activeUsers / u.total : 0),
          tip: `${num(fig.devices.activeUsers)} HESAP BU ARALIKTA EN AZ BİR CİHAZDAN GÖRÜNDÜ; ${num(u.new)} HESAP YENİ AÇILDI`,
        },
        {
          label: 'BEKLEYEN', value: num(fig.waitlist.pending.n), color: fig.waitlist.pending.n ? c.flame : undefined,
          share: wlTotal ? fig.waitlist.pending.n / wlTotal : 0, barColor: c.flame,
          right: '+' + num(wlNew) + ' YENİ', sub: 'LİSTENİN ' + pct(wlTotal ? fig.waitlist.pending.n / wlTotal : 0),
          tip: `BEKLEME LİSTESİNDE ${num(wlTotal)} KİŞİ: ${num(fig.waitlist.pending.n)} BEKLİYOR, ${num(fig.waitlist.approved.n)} ONAYLANDI, ${num(fig.waitlist.rejected.n)} REDDEDİLDİ`,
        },
      );
    }
    const th = 52;
    const tw = Math.floor((W - (tiles.length - 1) * 6) / Math.max(1, tiles.length));
    tiles.forEach((t, i) => tile(s, { x: i * (tw + 6), y, w: tw, h: th }, t));
    y += th + gap;

    if (linked) {
      // the Workers' requests over time, a colour each, the errors on top
      const layers = SCRIPTS.map(([k, , col]) => ({ values: cf.workers[k].series, color: pal[col] }));
      layers.push({ values: cf.steps.map((_, i) => sum(SCRIPTS.map(([k]) => cf.workers[k].errorSeries[i]))), color: pal.bad });
      const box = card(s, { title: "WORKER İSTEKLERİ", right: cf.step === 'date' ? 'GÜN GÜN' : 'SAAT SAAT' }, { x: 0, y, w: W, h: 104 });
      legend(box, 0, 0, [...SCRIPTS.map(([k, name, col]) => [name, pal[col], num(cf.workers[k].requests)]), ['HATA', pal.bad, num(err)]]);
      columns(box, { x: 0, y: 10, w: box.bounds.w, h: box.bounds.h - 10 }, layers, cf.steps, cf.step, (i) =>
        SCRIPTS.map(([k, name]) => name + ' ' + num(cf.workers[k].series[i])).join(' · ') + ' · HATA ' + num(layers[3].values[i]),
      );
      y += 104 + gap;

      // each Worker: its figures, and what it answered
      const rowH = 24;
      const wb = card(s, { title: "WORKER'LAR", right: 'YANITLAR: ÇUBUĞUN ÜSTÜNE GEL, HER KODUN ANLAMI ÇIKAR' }, { x: 0, y, w: W, h: 16 + SCRIPTS.length * rowH });
      const cols = [0, 70, 112, 150, 196, 270, 344];
      const heads = [
        ['', ''], ['İSTEK', ''], ['HATA', 'WORKER KODU ÇÖKTÜ YA DA SINIRI AŞTI'], ['ALT İSTEK', "WORKER'IN DIŞARIYA YAPTIĞI İSTEKLER"],
        ['CPU P50/P99', 'İŞLEMCİ SÜRESİ: İSTEKLERİN YARISI, YÜZDE 99U BUNDAN KISA'], ['SÜRE P50/P99', 'İSTEĞİN BAŞTAN SONA SÜRESİ (BEKLEMELER DAHİL)'],
        ['YANITLAR', 'HER İSTEĞİN DURUM KODU, TÜRÜNE GÖRE RENKLİ'],
      ];
      heads.forEach(([t, explain], i) => {
        const x2 = wb.text(t, cols[i], 0, { color: c.dim, font: 'small' });
        if (explain) hover(wb, { x: cols[i], y: 0, w: x2 - cols[i], h: 7 }, explain);
      });
      SCRIPTS.forEach(([k, name, col], i) => {
        const w = cf.workers[k];
        const ry = 10 + i * rowH;
        wb.fillRect({ x: 0, y: ry + 2, w: 4, h: 4 }, pal[col]);
        wb.text(name, 7, ry + 1, { color: c.paper, font: 'small' });
        wb.text(num(w.requests), cols[1], ry, { color: c.text });
        wb.text(num(w.errors), cols[2], ry, { color: w.errors ? pal.bad : c.text });
        wb.text(num(w.subrequests), cols[3], ry, { color: c.text });
        wb.text(ms(w.cpuP50) + ' / ' + ms(w.cpuP99), cols[4], ry + 1, { color: c.text, font: 'small' });
        wb.text(ms(w.wallP50) + ' / ' + ms(w.wallP99), cols[5], ry + 1, { color: c.text, font: 'small' });
        statusBar(wb, { x: cols[6], y: ry + 1, w: wb.bounds.w - cols[6], h: 16 }, w.status);
        if (i < SCRIPTS.length - 1) wb.hline(0, ry + rowH - 5, wb.bounds.w, c.line);
      });
      y += 16 + SCRIPTS.length * rowH + gap;

      // D1 beside R2
      const h2 = 132;
      const db = card(s, { title: 'D1 · CELESTINE', right: bytes(cf.d1.size) }, { x: 0, y, w: half, h: h2 });
      const gh = grid(db, 0, 0, db.bounds.w, 3, [
        ['OKUMA SORGUSU', num(cf.d1.reads)],
        ['YAZMA SORGUSU', num(cf.d1.writes)],
        ['SORGU P50 / P99', cf.d1.timeP50.toFixed(2) + ' / ' + cf.d1.timeP99.toFixed(1) + ' MS'],
        ['OKUNAN SATIR', num(cf.d1.rowsRead)],
        ['YAZILAN SATIR', num(cf.d1.rowsWritten)],
        ['DÖNEN VERİ', bytes(cf.d1.responseBytes)],
      ]);
      legend(db, 0, gh, [['OKUMA', pal.blue, ''], ['YAZMA', c.accent, '']]);
      columns(db, { x: 0, y: gh + 10, w: db.bounds.w, h: db.bounds.h - gh - 10 }, [
        { values: cf.d1.series.reads, color: pal.blue },
        { values: cf.d1.series.writes, color: c.accent },
      ], cf.steps, cf.step, (i) => 'OKUMA ' + num(cf.d1.series.reads[i]) + ' · YAZMA ' + num(cf.d1.series.writes[i]) + ' · SATIR ' + num(cf.d1.series.rowsRead[i]) + ' / ' + num(cf.d1.series.rowsWritten[i]));

      const rb = card(s, { title: 'R2', right: bytes(r2bytes) }, { x: half + 8, y, w: W - half - 8, h: h2 });
      BUCKETS.forEach(([k, name], i) => {
        const b = cf.buckets[k];
        const by = i * 40;
        rb.text(name, 0, by, { color: c.paper, font: 'small' });
        rb.text(k, rb.measureText(name, { font: 'small' }) + 6, by, { color: c.dim, font: 'small' });
        grid(rb, 0, by + 9, rb.bounds.w, 4, [
          ['DEPO', bytes(b.bytes)],
          ['NESNE', num(b.objects)],
          ['İNDİRİLEN', bytes(b.egress)],
          ['A / B İŞLEM', num(b.classA) + ' / ' + num(b.classB)],
        ]);
        const top = b.ops.slice(0, 3).map((o) => o.action.toUpperCase() + ' ' + num(o.requests));
        hover(rb, { x: 0, y: by, w: rb.bounds.w, h: 32 }, top.length ? 'EN ÇOK: ' + top.join(' · ') : 'İŞLEM YOK');
      });
      // the operations against the free plan's month, both buckets together
      const month = days / 30;
      const ops = [
        ['A SINIFI', classA, FREE.classA * month, 'A SINIFI: YAZMA VE LİSTELEME (PUT, LIST, YÜKLEME). ÜCRETSİZ PLAN AYDA 1M VERİR'],
        ['B SINIFI', classB, FREE.classB * month, 'B SINIFI: OKUMA (GET, HEAD). ÜCRETSİZ PLAN AYDA 10M VERİR'],
      ];
      ops.forEach(([label, n, limit, explain], i) => {
        const oy = 84 + i * 11;
        rb.text(label, 0, oy, { color: c.muted, font: 'small' });
        const t = num(n) + ' / ' + num(limit) + '  ' + pct(n / limit);
        meter(rb, { x: 44, y: oy + 1, w: rb.bounds.w - 44 - 90, h: 5 }, n / limit);
        rb.text(t, rb.bounds.w - rb.measureText(t, { font: 'small' }), oy, { color: c.text, font: 'small' });
        hover(rb, { x: 0, y: oy, w: rb.bounds.w, h: 9 }, explain + (days < 30 ? ` (${days} GÜNE DÜŞEN: ${num(limit)})` : ''));
      });
      y += h2 + gap;

      // where the requests come from: countries (by the host names) beside data centres (by the Workers)
      const rows = Math.max(cf.countries.length, cf.colos.length, 1);
      const h3 = 26 + Math.min(rows, 12) * 10;
      const hostReq = sum(cf.hosts.map((h) => h.requests));
      const workerReq = req;
      const countries = cf.countries.slice(0, 12).map((x) => ({ label: countryName(x.country), value: x.requests, tip: `${countryName(x.country)} · ${num(x.requests)} İSTEK · ${bytes(x.bytes)} GİDEN VERİ` }));
      countries.total = hostReq || undefined;
      const cb = card(s, { title: 'ÜLKELER', right: 'İSTEKLER NEREDEN GELDİ' }, { x: 0, y, w: half, h: h3 });
      ranking(cb, { x: 0, y: 0, w: cb.bounds.w, h: cb.bounds.h }, countries, c.accent, 'İSTEK YOK', 74);
      const colos = cf.colos.slice(0, 12).map((x) => ({ label: x.colo, value: x.requests, tip: `${x.colo}: CLOUDFLARE'IN BU ŞEHİRDEKİ VERİ MERKEZİ (HAVALİMANI KODU) · ${num(x.requests)} İSTEK` }));
      colos.total = workerReq || undefined;
      const ob = card(s, { title: 'VERİ MERKEZLERİ', right: "WORKER'LARIN ÇALIŞTIĞI YER" }, { x: half + 8, y, w: W - half - 8, h: h3 });
      ranking(ob, { x: 0, y: 0, w: ob.bounds.w, h: ob.bounds.h }, colos, pal.blue, 'İSTEK YOK', 30);
      y += h3 + gap;

      // the host names, as Cloudflare saw them: every request, before and with the Workers
      const blockH = 30;
      const hh = 16 + Math.max(1, cf.hosts.length) * blockH + 10;
      const hb = card(s, { title: 'ALAN ADLARI', right: "CLOUDFLARE'IN KARŞILADIĞI TÜM İSTEKLER" }, { x: 0, y, w: W, h: hh });
      if (!cf.hosts.length) hb.text('İSTEK YOK', 0, 2, { color: c.dim, font: 'small' });
      const sx = Math.floor(hb.bounds.w * 0.45);
      cf.hosts.forEach((h, i) => {
        const hy = i * blockH;
        hb.text(h.host.toUpperCase(), 0, hy, { color: c.paper, font: 'small' });
        figures(hb, 0, hy + 10, [['İSTEK', num(h.requests)], ['SAYFA ZİYARETİ', num(h.visits)], ['GİDEN VERİ', bytes(h.bytes)]]);
        statusBar(hb, { x: sx, y: hy + 1, w: hb.bounds.w - sx, h: 16 }, cf.edgeStatus.filter((e) => e.host === h.host));
        if (i < cf.hosts.length - 1) hb.hline(0, hy + blockH - 6, hb.bounds.w, c.line);
      });
      note(hb, 'ÇUBUKLAR YANITLARIN TÜRÜ: YEŞİL KARŞILANDI, MAVİ YÖNLENDİRİLDİ, TURUNCU OLMAYAN YA DA İZİNSİZ ADRES (ÇOĞU BOT), KIRMIZI SUNUCU YANIT VEREMEDİ.', 0, cf.hosts.length * blockH, hb.bounds.w, c.dim);
      y += hh + gap;
    }

    // the app's own figures, from its database
    if (fig) {
      const ab = card(s, { title: 'UYGULAMA', right: 'KENDİ VERİTABANINDAN' }, { x: 0, y, w: W, h: 78 });
      const u = fig.users;
      grid(ab, 0, 0, ab.bounds.w, 6, [
        ['HESAP', num(u.total) + ' · ' + num(u.active) + ' AÇIK'],
        ['YENİ HESAP', '+' + num(u.new)],
        ['GÖRÜLEN CİHAZ', num(fig.devices.seen) + ' / ' + num(fig.devices.total)],
        ['HESAP BAŞI VERİ', bytes(u.avgBytes)],
        ['EN BÜYÜK HESAP', bytes(u.maxBytes)],
        ['DEĞİŞİKLİK', num(u.changes)],
        ['BEKLEYEN', num(fig.waitlist.pending.n) + ' (+' + num(fig.waitlist.pending.new) + ')', fig.waitlist.pending.n ? c.flame : undefined],
        ['ONAYLANAN', num(fig.waitlist.approved.n)],
        ['REDDEDİLEN', num(fig.waitlist.rejected.n)],
        ['AÇIK DAVET KODU', num(fig.invites.open)],
        ['KULLANILAN KOD', '+' + num(fig.invites.used)],
      ]);
      // the accounts' data against their quotas, together
      const qy = 48;
      ab.text('KOTA', 0, qy, { color: c.muted, font: 'small' });
      const qt = bytes(u.bytes) + ' / ' + bytes(u.quota) + '  ' + pct(u.quota ? u.bytes / u.quota : 0);
      meter(ab, { x: 30, y: qy + 1, w: ab.bounds.w - 30 - 130, h: 5 }, u.quota ? u.bytes / u.quota : 0);
      ab.text(qt, ab.bounds.w - ab.measureText(qt, { font: 'small' }), qy, { color: c.text, font: 'small' });
      hover(ab, { x: 0, y: qy, w: ab.bounds.w, h: 9 }, 'HESAPLARIN VERİSİ, KOTALARININ TOPLAMINA GÖRE');
      y += 78 + gap;
    }
    s.place({ w: W, h: y }); // the content's height, for the scroll
  });
  return tip;
}
