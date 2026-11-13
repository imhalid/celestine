// The dashboard of the admin panel (its GÖSTERGE tab): Cloudflare's figures for the Workers, the D1 database and
// the R2 buckets of Celestine, where its requests come from, and the app's own figures beside them, over a day, a
// week or a month. The figures come from /api/admin/cloudflare and /api/admin/figures (analytics.js); this draws
// them, in one scrolling column of cards.
import { button, card, scrollView, segmented, useTheme } from '@synth-ui/widgets';

/** The ranges offered, as analytics.js names them. */
export const DASH_RANGES = [
  { value: '24h', label: '24 SAAT' },
  { value: '7d', label: '7 GÜN' },
  { value: '30d', label: '30 GÜN' },
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
/** What Cloudflare's free plan holds, in its own (decimal) units: 5 GB of D1, 10 GB of R2. */
const FREE = { d1: 5e9, r2: 10e9 };

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
const pct = (x) => {
  const p = (Number(x) || 0) * 100;
  return (p > 0 && p < 10 ? p.toFixed(1) : Math.round(p)) + '%';
};
/** stepLabel names a step of a series: an hour with its day, or a day. */
const stepLabel = (s, step) => {
  const d = new Date(step === 'date' ? s + 'T00:00:00Z' : s);
  return step === 'date'
    ? d.toLocaleDateString('tr-TR', { day: '2-digit', month: '2-digit' })
    : d.toLocaleString('tr-TR', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' }).replace(',', '');
};
const sum = (a) => a.reduce((s, x) => s + x, 0);

// ---- the pieces ----

/** tone is the colour of an HTTP status: green, blue for a redirect, flame for 4xx, red for 5xx; 0 (no answer) dim. */
const tone = (ctx, status) => {
  const pal = ctx.palette.index;
  return !status ? pal.dim : status >= 500 ? pal.bad : status >= 400 ? pal.flame : status >= 300 ? pal.blue : pal.ok;
};

/** tile is one large figure: a name above, the value, a line below. */
function tile(ctx, r, label, value, sub, color) {
  const { colors: c } = useTheme(ctx);
  ctx.fillRect(r, c.panel);
  ctx.strokeRect(r, c.line);
  ctx.text(label, r.x + 5, r.y + 4, { color: c.muted, font: 'small' });
  const scale = ctx.measureText(value) * 2 <= r.w - 10 ? 2 : 1;
  ctx.text(value, r.x + 5, r.y + 13, { color: color ?? c.paper, scale });
  if (sub) ctx.text(sub, r.x + 5, r.y + r.h - 10, { color: c.dim, font: 'small' });
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

/**
 * columns draws a series as stacked bars, a layer a colour, under its largest value; the step under the pointer is
 * lit and described by `describe(i)`, which the panel shows as a tooltip (returned).
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
  let tip;
  for (let i = 0; i < n; i++) {
    const x = plot.x + Math.floor(i * slot);
    const col = { x, y: plot.y, w: Math.max(1, Math.ceil(slot)), h: plot.h };
    const hot = ctx.containsPointer(col);
    if (hot) {
      ctx.fillRect({ x, y: plot.y, w: bw, h: plot.h }, c.raised);
      tip = stepLabel(steps[i], step) + ' · ' + describe(i);
    }
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
  return tip;
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

/** ranking draws rows of a name, a bar to scale and the figures at the right: a share of the whole. */
function ranking(ctx, r, rows, color, empty) {
  const { colors: c } = useTheme(ctx);
  if (!rows.length) {
    ctx.text(empty, r.x, r.y + 2, { color: c.dim, font: 'small' });
    return;
  }
  const max = Math.max(1, ...rows.map((x) => x.value));
  const total = sum(rows.map((x) => x.value));
  const lw = 30, rw = 92;
  rows.forEach((row, i) => {
    const y = r.y + i * 10;
    if (y + 8 > r.y + r.h) return;
    ctx.text(row.label, r.x, y, { color: c.text, font: 'small' });
    const bw = r.w - lw - rw;
    ctx.fillRect({ x: r.x + lw, y: y + 1, w: bw, h: 5 }, c.raised);
    ctx.fillRect({ x: r.x + lw, y: y + 1, w: Math.max(1, Math.round((row.value / max) * bw)), h: 5 }, color);
    const right = num(row.value) + ' · ' + pct(row.value / total) + (row.extra ? ' · ' + row.extra : '');
    ctx.text(right, r.x + r.w - ctx.measureText(right, { font: 'small' }), y, { color: c.muted, font: 'small' });
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
  // the controls: the range, the time of the figures, and a way to read them again
  const bar = ctx.cutTop(13, 8);
  const r = segmented(ctx, { options: DASH_RANGES, value: d.range, key: 'dash-range', h: 13 }, { x: bar.x, y: bar.y, w: 150, h: 13 });
  if (r !== null && r !== d.range) on.range(r);
  if (button(ctx, { label: d.loading ? 'OKUNUYOR…' : 'YENİLE', h: 13 }, { x: bar.x + bar.w - 70, y: bar.y, w: 70, h: 13 }).clicked && !d.loading) on.refresh();
  const cf = d.cf, fig = d.fig;
  const when = cf?.at ? 'CLOUDFLARE ' + new Date(cf.at * 1000).toLocaleTimeString('tr-TR', { hour: '2-digit', minute: '2-digit' }) + ' · ÖRNEKLEMLİ, YAKLAŞIK' : '';
  ctx.text(when, bar.x + 160, bar.y + 4, { color: c.dim, font: 'small' });

  let tip;
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

    // the large figures
    const tiles = [];
    if (linked) {
      tiles.push(
        ['İSTEK', num(req), 'HATA ' + num(err) + ' · ' + pct(req ? err / req : 0), err ? pal.bad : undefined],
        ['D1 SORGU', num(cf.d1.reads + cf.d1.writes), num(cf.d1.reads) + ' OKUMA · ' + num(cf.d1.writes) + ' YAZMA'],
        ['D1 BOYUT', bytes(cf.d1.size), '5 GB SINIRIN ' + pct(cf.d1.size / FREE.d1)],
        ['R2 DEPO', bytes(r2bytes), '10 GB SINIRIN ' + pct(r2bytes / FREE.r2)],
      );
    }
    if (fig) {
      tiles.push(
        ['HESAP', num(fig.users.total), '+' + num(fig.users.new) + ' YENİ · ' + num(fig.devices.activeUsers) + ' AKTİF'],
        ['BEKLEYEN', num(fig.waitlist.pending.n), '+' + num(sum(['pending', 'approved', 'rejected'].map((k) => fig.waitlist[k].new))) + ' YENİ BAŞVURU', fig.waitlist.pending.n ? c.flame : undefined],
      );
    }
    const tw = Math.floor((W - (tiles.length - 1) * 6) / Math.max(1, tiles.length));
    tiles.forEach(([label, value, sub, color], i) => tile(s, { x: i * (tw + 6), y, w: tw, h: 42 }, label, value, sub, color));
    y += 42 + gap;

    if (linked) {
      // the Workers' requests over time, a colour each, the errors on top
      const layers = SCRIPTS.map(([k, , col]) => ({ values: cf.workers[k].series, color: pal[col] }));
      layers.push({ values: cf.steps.map((_, i) => sum(SCRIPTS.map(([k]) => cf.workers[k].errorSeries[i]))), color: pal.bad });
      const box = card(s, { title: "WORKER İSTEKLERİ", right: num(req) + ' İSTEK · ' + num(sum(ws.map((w) => w.subrequests))) + ' ALT İSTEK' }, { x: 0, y, w: W, h: 104 });
      legend(box, 0, 0, [...SCRIPTS.map(([k, name, col]) => [name, pal[col], num(cf.workers[k].requests)]), ['HATA', pal.bad, num(err)]]);
      tip =
        columns(box, { x: 0, y: 10, w: box.bounds.w, h: box.bounds.h - 10 }, layers, cf.steps, cf.step, (i) =>
          SCRIPTS.map(([k, name]) => name + ' ' + num(cf.workers[k].series[i])).join(' · ') + ' · HATA ' + num(layers[3].values[i]),
        ) ?? tip;
      y += 104 + gap;

      // each Worker: its figures and its responses by status
      const rowH = 20;
      const wb = card(s, { title: "WORKER'LAR", right: 'CPU VE SÜRE: P50 / P99' }, { x: 0, y, w: W, h: 16 + SCRIPTS.length * rowH });
      const cols = [0, 74, 128, 176, 230, 318, 406];
      ['', 'İSTEK', 'HATA', 'ALT İSTEK', 'CPU', 'SÜRE', 'DURUM KODLARI'].forEach((t, i) => wb.text(t, cols[i], 0, { color: c.dim, font: 'small' }));
      SCRIPTS.forEach(([k, name, col], i) => {
        const w = cf.workers[k];
        const ry = 10 + i * rowH;
        wb.fillRect({ x: 0, y: ry + 2, w: 4, h: 4 }, pal[col]);
        wb.text(name, 7, ry + 1, { color: c.paper, font: 'small' });
        wb.text(num(w.requests), cols[1], ry, { color: c.text });
        wb.text(num(w.errors), cols[2], ry, { color: w.errors ? pal.bad : c.text });
        wb.text(num(w.subrequests), cols[3], ry, { color: c.text });
        wb.text(ms(w.cpuP50) + ' / ' + ms(w.cpuP99), cols[4], ry, { color: c.text, font: 'small' });
        wb.text(ms(w.wallP50) + ' / ' + ms(w.wallP99), cols[5], ry, { color: c.text, font: 'small' });
        let x = cols[6];
        const codes = [...w.status].sort((a, b) => b.requests - a.requests);
        if (!codes.length) wb.text(w.requests ? '' : 'İSTEK YOK', x, ry, { color: c.dim, font: 'small' });
        for (const st of codes) {
          const t = st.status + ' ' + num(st.requests);
          if (x + ctx.measureText(t, { font: 'small' }) > wb.bounds.w) break;
          x = wb.text(t, x, ry, { color: tone(ctx, st.status), font: 'small' }) + 8;
        }
        if (i < SCRIPTS.length - 1) wb.hline(0, ry + rowH - 6, wb.bounds.w, c.line);
      });
      y += 16 + SCRIPTS.length * rowH + gap;

      // D1 beside R2
      const h2 = 128;
      const db = card(s, { title: 'D1 · CELESTINE', right: bytes(cf.d1.size) }, { x: 0, y, w: half, h: h2 });
      const gh = grid(db, 0, 0, db.bounds.w, 3, [
        ['OKUMA SORGUSU', num(cf.d1.reads)],
        ['YAZMA SORGUSU', num(cf.d1.writes)],
        ['SORGU P50 / P99', cf.d1.timeP50.toFixed(2) + ' / ' + cf.d1.timeP99.toFixed(1) + ' MS'],
        ['OKUNAN SATIR', num(cf.d1.rowsRead)],
        ['YAZILAN SATIR', num(cf.d1.rowsWritten)],
        ['DÖNEN VERİ', bytes(cf.d1.responseBytes)],
      ]);
      tip =
        columns(db, { x: 0, y: gh + 2, w: db.bounds.w, h: db.bounds.h - gh - 2 }, [
          { values: cf.d1.series.reads, color: pal.blue },
          { values: cf.d1.series.writes, color: c.accent },
        ], cf.steps, cf.step, (i) => 'OKUMA ' + num(cf.d1.series.reads[i]) + ' · YAZMA ' + num(cf.d1.series.writes[i]) + ' · SATIR ' + num(cf.d1.series.rowsRead[i]) + ' / ' + num(cf.d1.series.rowsWritten[i])) ?? tip;

      const rb = card(s, { title: 'R2', right: bytes(r2bytes) + ' · A VE B SINIFI İŞLEM' }, { x: half + 8, y, w: W - half - 8, h: h2 });
      BUCKETS.forEach(([k, name], i) => {
        const b = cf.buckets[k];
        const by = i * 56;
        rb.text(name, 0, by, { color: c.paper, font: 'small' });
        rb.text(k, rb.measureText(name, { font: 'small' }) + 6, by, { color: c.dim, font: 'small' });
        grid(rb, 0, by + 9, rb.bounds.w, 3, [
          ['DEPO', bytes(b.bytes)],
          ['NESNE', num(b.objects)],
          ['DIŞA AKTARIM', bytes(b.egress)],
        ]);
        const ops = 'A ' + num(b.classA) + ' · B ' + num(b.classB) + (b.free ? ' · ÜCRETSİZ ' + num(b.free) : '');
        const top = b.ops.slice(0, 3).map((o) => o.action.toUpperCase() + ' ' + num(o.requests)).join(' · ');
        rb.text(ops, 0, by + 33, { color: c.text, font: 'small' });
        rb.text(top || 'İŞLEM YOK', 0, by + 42, { color: c.dim, font: 'small' });
      });
      y += h2 + gap;

      // where the requests come from: countries (by the host names) beside data centres (by the Workers)
      const rows = Math.max(cf.countries.length, cf.colos.length, 1);
      const h3 = 26 + Math.min(rows, 12) * 10;
      const cb = card(s, { title: 'ÜLKELER', right: 'ALAN ADLARINA GELEN' }, { x: 0, y, w: half, h: h3 });
      ranking(cb, { x: 0, y: 0, w: cb.bounds.w, h: cb.bounds.h }, cf.countries.slice(0, 12).map((x) => ({ label: x.country, value: x.requests, extra: bytes(x.bytes) })), c.accent, 'İSTEK YOK');
      const ob = card(s, { title: 'VERİ MERKEZLERİ', right: "WORKER'LARIN" }, { x: half + 8, y, w: W - half - 8, h: h3 });
      ranking(ob, { x: 0, y: 0, w: ob.bounds.w, h: ob.bounds.h }, cf.colos.slice(0, 12).map((x) => ({ label: x.colo, value: x.requests })), pal.blue, 'İSTEK YOK');
      y += h3 + gap;

      // the host names, as the edge saw them: what the Workers answered and what Cloudflare answered before them
      const hh = 14 + Math.max(1, cf.hosts.length) * 20;
      const hb = card(s, { title: 'ALAN ADLARI', right: 'KENARDA GÖRÜLEN' }, { x: 0, y, w: W, h: hh });
      if (!cf.hosts.length) hb.text('İSTEK YOK', 0, 2, { color: c.dim, font: 'small' });
      cf.hosts.forEach((h, i) => {
        const hy = i * 20;
        hb.text(h.host.toUpperCase(), 0, hy, { color: c.paper, font: 'small' });
        hb.text(num(h.requests) + ' İSTEK · ' + num(h.visits) + ' ZİYARET · ' + bytes(h.bytes) + ' · 4XX ' + pct(h.r4xx) + ' · 5XX ' + pct(h.r5xx), 0, hy + 9, { color: c.text, font: 'small' });
        let x = Math.floor(hb.bounds.w * 0.55);
        for (const st of cf.edgeStatus.filter((e) => e.host === h.host)) {
          const t = st.status + ' ' + num(st.requests);
          if (x + hb.measureText(t, { font: 'small' }) > hb.bounds.w) break;
          x = hb.text(t, x, hy + 9, { color: tone(ctx, st.status), font: 'small' }) + 8;
        }
      });
      y += hh + gap;
    }

    // the app's own figures, from its database
    if (fig) {
      const ab = card(s, { title: 'UYGULAMA', right: 'KENDİ VERİTABANINDAN' }, { x: 0, y, w: W, h: 64 });
      const u = fig.users;
      grid(ab, 0, 0, ab.bounds.w, 6, [
        ['HESAP', num(u.total) + ' · ' + num(u.active) + ' AÇIK'],
        ['YENİ HESAP', '+' + num(u.new)],
        ['GÖRÜLEN CİHAZ', num(fig.devices.seen) + ' / ' + num(fig.devices.total)],
        ['HESAP BAŞI VERİ', bytes(u.avgBytes)],
        ['EN BÜYÜK HESAP', bytes(u.maxBytes)],
        ['TOPLAM / KOTA', bytes(u.bytes) + ' / ' + bytes(u.quota)],
        ['BEKLEYEN', num(fig.waitlist.pending.n) + ' (+' + num(fig.waitlist.pending.new) + ')', fig.waitlist.pending.n ? c.flame : undefined],
        ['ONAYLANAN', num(fig.waitlist.approved.n)],
        ['REDDEDİLEN', num(fig.waitlist.rejected.n)],
        ['AÇIK DAVET KODU', num(fig.invites.open)],
        ['KULLANILAN KOD', '+' + num(fig.invites.used)],
        ['DEĞİŞİKLİK', num(u.changes)],
      ]);
      y += 64 + gap;
    }
    s.place({ w: W, h: y }); // the content's height, for the scroll
  });
  return tip;
}
