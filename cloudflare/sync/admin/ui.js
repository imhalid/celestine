// The controls the admin panel needs and synth-ui has none of, drawn the way its own are: immediate mode, the
// theme's colours by name, whole pixels. A table of columns over synth-ui's list, a field that hides what is typed,
// a status badge, and a code on large tiles.
import { list, useTheme } from '@synth-ui/widgets';

/** The tones a badge can have, as palette names: the theme's and the panel's own (main.js adds them). */
const TONES = {
  ok: ['okbg', 'ok'],
  wait: ['ember', 'flame'],
  bad: ['badbg', 'bad'],
  off: ['control', 'muted'],
};

/**
 * table draws rows under a line of column titles, filling `rect` or the rest of the region. A column is
 * `{ title, w }`, `w` in pixels or, left out, a share of what the others leave. `cell(ctx, row, column)` draws one
 * cell into a region its size; buttons drawn there are on top of the row.
 */
export function table(ctx, { key, columns, rows, rowHeight = 15, empty = '' }, cell, rect) {
  const { colors: c } = useTheme(ctx);
  const b = ctx.bounds; // the rest of the region, as synth-ui's own list takes it (after cuts, bounds starts past 0)
  const r = rect ?? { x: ctx.cursor.x, y: ctx.cursor.y, w: b.x + b.w - ctx.cursor.x, h: b.y + b.h - ctx.cursor.y };
  const fixed = columns.reduce((n, col) => n + (col.w ?? 0), 0);
  const flex = columns.filter((col) => col.w == null).length;
  const gap = 6;
  const rest = Math.max(0, r.w - fixed - gap * (columns.length - 1));
  const widths = columns.map((col) => col.w ?? Math.floor(rest / Math.max(1, flex)));
  const xs = [];
  widths.reduce((x, w) => (xs.push(x), x + w + gap), 0);
  // the titles
  const th = ctx.fontMetrics('small').height + 3;
  columns.forEach((col, i) => ctx.text(col.title, r.x + xs[i] + 2, r.y + 1, { color: c.muted, font: 'small' }));
  ctx.hline(r.x, r.y + th, r.w, c.border);
  const body = { x: r.x, y: r.y + th + 2, w: r.w, h: r.h - th - 2 };
  if (!rows.length) {
    ctx.text(empty, body.x + 2, body.y + 6, { color: c.dim });
    return { clicked: null };
  }
  const res = list(ctx, { key, count: rows.length, rowHeight, separators: true, keyboard: false, rowKey: (i) => rows[i].id ?? i }, (rc, row) => {
    columns.forEach((col, i) => {
      rc.allocate({ x: xs[i], y: 0, w: widths[i], h: rowHeight }, (cc) => cell(cc, rows[row.index], col, row), { clip: true });
    });
  }, body);
  return { clicked: res.clicked };
}

/** A cell's text, middle of the row, cut to the cell with an ellipsis. */
export function cellText(ctx, text, color) {
  const { colors: c } = useTheme(ctx);
  let t = String(text ?? '');
  const room = ctx.bounds.w - 2;
  if (ctx.measureText(t) > room) {
    while (t.length > 1 && ctx.measureText(t + '…') > room) t = t.slice(0, -1);
    t += '…';
  }
  ctx.text(t, 1, Math.floor((ctx.bounds.h - ctx.fontMetrics().height) / 2), { color: color ?? c.text });
}

/** A badge: a few words on a tinted ground, in one of TONES. Placed by the flow, or at x, y. */
export function badge(ctx, text, tone = 'off', at) {
  const pal = ctx.palette.index;
  const [bg, fg] = TONES[tone] ?? TONES.off;
  const w = ctx.measureText(text, { font: 'small' }) + 6;
  const h = ctx.fontMetrics('small').height + 2;
  const r = at ? { x: at.x, y: at.y, w, h } : ctx.place({ w, h });
  ctx.fillRect(r, pal[bg]);
  ctx.text(text, r.x + 3, r.y + 1, { color: pal[fg], font: 'small' });
  return r;
}

/**
 * secretField is a field whose text is never shown: a dot for each letter typed. Enter gives `submitted`; Escape
 * clears it. The text is the app's (`state.text`), so it can be cleared once used.
 */
export function secretField(ctx, { key, state, placeholder = '', w }) {
  const { colors: c } = useTheme(ctx);
  const id = ctx.makeKey(key);
  const r = ctx.place({ w: w ?? ctx.bounds.x + ctx.bounds.w - ctx.cursor.x, h: ctx.fontMetrics().height + 4 });
  const it = ctx.interaction(r, { key: `${id}:it`, focusable: true, text: true, cursor: 'text' });
  let submitted = false;
  const dotsW = state.text.length * 6;
  if (it.focused) {
    const caret = { x: r.x + 3 + dotsW, y: r.y + 3, w: 1, h: 7 };
    for (const ev of ctx.textInput(it, { caret, selection: '' })) {
      if (ev.type === 'text' || ev.type === 'paste') state.text += ev.text.replace(/\s+/g, '');
    }
    for (const ev of ctx.keyEvents()) {
      if (ev.type !== 'keydown' || ev.consumed) continue;
      if (ev.key === 'Backspace') state.text = state.text.slice(0, -1);
      else if (ev.key === 'Enter') submitted = true;
      else if (ev.key === 'Escape') state.text = '';
      else continue;
      ev.consumed = true;
    }
  }
  ctx.fillRect(r, c.void);
  ctx.strokeRect(r, it.focused ? c.accent : it.hovered ? c.dim : c.border);
  if (!state.text && !it.focused) ctx.text(placeholder, r.x + 3, r.y + 2, { color: c.dim });
  for (let i = 0; i < state.text.length && 3 + i * 6 < r.w - 4; i++) ctx.fillRect({ x: r.x + 4 + i * 6, y: r.y + 5, w: 3, h: 3 }, c.paper);
  if (it.focused && Math.floor(ctx.time / 530) % 2 === 0) ctx.vline(r.x + 3 + Math.min(dotsW, r.w - 6), r.y + 3, 7, c.accent);
  if (it.focused) ctx.requestFrame(); // the caret blinks
  return { it, submitted };
}

/** codeTiles draws a code a letter to a tile, large, as a split-flap board shows it; returns the rect it took. */
export function codeTiles(ctx, code, scale = 2) {
  const { colors: c } = useTheme(ctx);
  const tw = 5 * scale + 6, th = ctx.fontMetrics().height * scale + 4;
  const r = ctx.place({ w: code.length * (tw + 2), h: th });
  [...code].forEach((ch, i) => {
    const t = { x: r.x + i * (tw + 2), y: r.y, w: tw, h: th };
    if (ch === '-') {
      ctx.text('-', t.x + 3, t.y + 2, { color: c.dim, scale });
      return;
    }
    ctx.fillRect(t, c.raised);
    ctx.strokeRect(t, c.border);
    ctx.hline(t.x + 1, t.y + Math.floor(th / 2), tw - 2, c.void); // the board's split
    ctx.text(ch, t.x + 3, t.y + 2, { color: c.flame, scale });
  });
  return r;
}
