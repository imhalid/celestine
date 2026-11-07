// The writers: carets of the app (Ryudit's caret set) placed about the screen, each writing a little note of its own,
// erasing it, and moving on. They react the way they do in the app: Mario hops at a space and a coin springs out, Kawaii
// sparkles as it writes and squeezes its eyes shut with joy, UwU's face is > w < when quick and T w T while erasing, the
// fireflies gather as the letters come and drift off at rest.
//
// Each is drawn with the 2D canvas API onto the canvas of its own card (CaretCard.svelte), in the page's ink.

const FONT_PX = 15;
const LINE = Math.round(FONT_PX * 1.35);
const FONT = `600 ${FONT_PX}px ui-monospace,"SF Mono",SFMono-Regular,Menlo,Consolas,"Liberation Mono",monospace`;

// What they write: short notes, as one would in a notes app. Spaces, "!" and "?" make some of them react.
const NOTES = [
  "- [ ] buy milk", "## Ideas", "meeting notes", "what a good day!", "#ryudit", "> local-first",
  "- [x] backup done", "ask Deniz?", "idea: flying blocks", "**bold** thought", "hello, world!",
  "notes, but alive", "TODO: sleep a bit", "tea or coffee?", "Nov 7 journal", "[[linked page]]",
  "uwu this is nice!", "plan: monday", "collection > list", "sync done!", "made with MyGo",
  "reading list", "- [ ] workout", "why not?", "block by block", "the / menu", "ideas everywhere",
  "a note a day", "sunday: rest", "new page", "## To read", "great!", "what to write?",
];

// The color each kind writes in, where the page gives no ink.
const INK = {
  slime: "#9dffd0", mario: "#ffd2cc", firefly: "#fff3a6", season: "#ffe2b8", kawaii: "#ffd1ec",
  uwu: "#ded3ff", torch: "#ffd5a8", crt: "#a8ffc8",
};

const rnd = Math.random;
const clamp01 = (x) => Math.max(0, Math.min(1, x));
const easeOut = (x) => 1 - Math.pow(1 - clamp01(x), 3);
const lerp = (a, b, t) => a + (b - a) * t;

function rrect(x, y, w, h, r) {
  const p = new Path2D();
  r = Math.min(r, w / 2, h / 2);
  p.moveTo(x + r, y); p.arcTo(x + w, y, x + w, y + h, r); p.arcTo(x + w, y + h, x, y + h, r);
  p.arcTo(x, y + h, x, y, r); p.arcTo(x, y, x + w, y, r); p.closePath();
  return p;
}
// see-through, lit from above, with a rim: the "glass" body most carets of the app have
function glass(c, b, col, a = 1, r = 3) {
  const p = rrect(b.x, b.y, b.w, b.h, r);
  const g = c.createLinearGradient(0, b.y, 0, b.y + b.h);
  g.addColorStop(0, rgba(mixHex(col, "#ffffff", 0.45), 0.62 * a));
  g.addColorStop(0.45, rgba(col, 0.42 * a));
  g.addColorStop(1, rgba(col, 0.55 * a));
  c.fillStyle = g; c.fill(p);
  c.lineWidth = 1.25; c.strokeStyle = rgba(col, 0.95 * a); c.stroke(p);
  // a gloss down its upper left side
  c.strokeStyle = `rgba(255,255,255,${0.6 * a})`; c.lineWidth = 1.4;
  c.beginPath(); const gx = b.x + Math.max(1.6, b.w * 0.22); c.moveTo(gx, b.y + b.h * 0.15); c.lineTo(gx, b.y + b.h * 0.38); c.stroke();
}
function hex(h) { const n = parseInt(h.slice(1), 16); return [(n >> 16) & 255, (n >> 8) & 255, n & 255]; }
function rgba(h, a) { const [r, g, b] = hex(h); return `rgba(${r},${g},${b},${a})`; }
function mixHex(a, b, t) { const x = hex(a), y = hex(b); return "#" + x.map((v, i) => Math.round(v + (y[i] - v) * t).toString(16).padStart(2, "0")).join(""); }
function dot(c, x, y, r, fill) { c.fillStyle = fill; c.beginPath(); c.arc(x, y, Math.max(0.1, r), 0, Math.PI * 2); c.fill(); }

// ---- one writer ----
class Writer {
  constructor(kind, host) {
    this.kind = kind; this.host = host;
    this.text = ""; this.target = ""; this.sel = false;
    this.x = 0; this.y = 0; this.cx = 0; this.cy = 0; this.vx = 0; this.vy = 0; // the line's start; the caret's drawn place
    this.state = "hidden"; this.until = performance.now() + 300 + rnd() * 2400;
    this.vis = 0; this.motes = []; this.lastAct = -1e9; this.rate = 0;
    this.left = false; this.hop = -1e9; this.joy = -1e9; this.shock = -1e9; this.sad = -1e9;
    this.blinkAt = 0; this.flies = []; this.flyCount = 0; this.seasonT0 = rnd() * 4;
    this.slot = null; this.typo = -1; this.typoDone = false;
  }
  // the width it needs: the note, and room for a wide face
  span(note) { return (note.length + 4) * this.host.cw; }
  begin(now) {
    const note = this.host.note(this);
    const slot = this.host.place(this, this.span(note));
    if (!slot) { this.until = now + 800 + rnd() * 1200; return; }
    this.slot = slot; this.target = note; this.text = ""; this.sel = false;
    this.typo = note.length > 5 && rnd() < 0.22 ? 2 + ((rnd() * (note.length - 3)) | 0) : -1; this.typoDone = false;
    const first = this.state === "hidden" && this.vis === 0;
    this.x = slot.x; this.y = slot.y;
    if (first) { this.cx = this.x; this.cy = this.y; }
    this.state = first ? "arrive" : "glide";
    this.until = now + (first ? 500 : 0);
  }
  step(now, dt) {
    const k = { wrote: 0, erased: 0, typed: "" };
    const cw = this.host.cw;
    this.dt = dt;
    // where the caret should be: after the last letter written
    const tx = this.x + this.text.length * cw, ty = this.y;
    if (this.state === "glide") {
      const ax = (tx - this.cx) * 30 - this.vx * 9, ay = (ty - this.cy) * 30 - this.vy * 9;
      this.vx += ax * dt; this.vy += ay * dt; this.cx += this.vx * dt; this.cy += this.vy * dt;
      if (Math.abs(this.vx) > 20) this.left = this.vx < 0;
      if (Math.hypot(tx - this.cx, ty - this.cy) < 0.6 && Math.hypot(this.vx, this.vy) < 12) { this.cx = tx; this.cy = ty; this.vx = this.vy = 0; this.state = "think"; this.until = now + 300 + rnd() * 700; }
    } else {
      // a little spring to each new letter, so the caret slides rather than jumps
      this.cx = lerp(this.cx, tx, Math.min(1, dt * 28)); this.cy = lerp(this.cy, ty, Math.min(1, dt * 28));
    }
    const show = this.state !== "hidden" && this.state !== "leave";
    this.vis = clamp01(this.vis + (show ? dt * 3 : -dt * 3));

    if (now >= this.until) {
      switch (this.state) {
        case "hidden": this.begin(now); break;
        case "arrive": this.state = "think"; this.until = now + 200 + rnd() * 500; break;
        case "think": this.state = "type"; this.until = now; break;
        case "type": {
          const t = this.text, goal = this.target;
          if (this.typo === t.length && !this.typoDone) { // a slip of the finger: a wrong letter, a pause, taken back
            const wrong = "asdfghjklqwertyuiop"[(rnd() * 19) | 0];
            this.text += wrong; k.wrote = 1; k.typed = wrong; this.typoDone = true; this.state = "oops"; this.until = now + 260 + rnd() * 200;
          } else if (t.length < goal.length) {
            const ch = goal[t.length]; this.text += ch; k.wrote = 1; k.typed = ch;
            this.until = now + (ch === " " ? 120 + rnd() * 260 : 55 + rnd() * 120) + (rnd() < 0.06 ? 400 : 0);
          } else { this.state = "rest"; this.until = now + 1600 + rnd() * 2600; }
          break;
        }
        case "oops": this.text = this.text.slice(0, -1); k.erased = 1; this.state = "type"; this.until = now + 140 + rnd() * 120; break;
        case "rest":
          // erase it: one by one, or (sometimes) all selected and gone at once
          if (rnd() < 0.35) { this.sel = true; this.state = "wipe"; this.until = now + 420; }
          else { this.state = "erase"; this.until = now; }
          break;
        case "wipe": k.erased = this.text.length; this.text = ""; this.sel = false; this.state = "after"; this.until = now + 300 + rnd() * 500; break;
        case "erase":
          if (this.text.length) { this.text = this.text.slice(0, -1); k.erased = 1; this.until = now + 32 + rnd() * 40; }
          else { this.state = "after"; this.until = now + 300 + rnd() * 600; }
          break;
        case "after":
          this.host.free(this);
          if (rnd() < 0.3) { this.state = "leave"; this.until = now + 700 + rnd() * 1800; } // it goes out for a while
          else this.begin(now);
          break;
        case "leave": this.state = "hidden"; this.vis = 0; this.until = now + 200; break;
      }
    }
    if (k.wrote || k.erased) this.lastAct = now;
    this.rate += ((k.wrote ? 1 / Math.max(dt, 1e-3) : 0) - this.rate) * Math.min(1, dt * 4);
    this.react(now, dt, k);
    this.motes = this.motes.filter((m) => now - m.t0 < m.life);
    for (const m of this.motes) { m.vy += (m.grav || 0) * dt; m.x += m.vx * dt; m.y += m.vy * dt; m.ang += (m.spin || 0) * dt; }
  }
  // the app's reactions, each caret its own
  react(now, dt, k) {
    const cw = this.host.cw, x = this.cx + cw / 2, y = this.cy + LINE / 2;
    const add = (m) => { this.motes.push({ ang: 0, t0: now, ...m }); if (this.motes.length > 40) this.motes.shift(); };
    switch (this.kind) {
      case "mario":
        if (k.typed === " ") { this.hop = now; add({ type: "coin", x, y: this.cy, vx: 0, vy: 0, life: 520 }); }
        break;
      case "kawaii":
        if (k.wrote) for (let i = 0; i < 2; i++) { const a = -Math.PI / 2 + (rnd() - 0.5) * 2.4; add({ type: "star", x, y: y - LINE * 0.3, vx: Math.cos(a) * 45, vy: Math.sin(a) * 45 - 10, spin: 4, life: 650, size: LINE * (0.16 + rnd() * 0.12), col: `hsl(${300 + rnd() * 80} 90% 75%)` }); }
        if (k.typed === "!") for (let i = 0; i < 3; i++) add({ type: "heart", x: x + (rnd() - 0.5) * LINE * 0.6, y: y - LINE * 0.4, vx: (rnd() - 0.5) * 30, vy: -55 - rnd() * 25, life: 1100, size: LINE * 0.5 });
        if (k.typed === " ") this.joy = now;
        break;
      case "uwu":
        if (k.wrote && rnd() < 0.35) add({ type: "tilde", x: x + cw * 1.2, y: this.cy + LINE * 0.2, vx: 25 + rnd() * 20, vy: -30 - rnd() * 20, life: 900, size: LINE * 0.55 });
        if (k.typed === "!" || k.typed === "?") this.shock = now;
        if (k.erased) this.sad = now;
        break;
      case "torch":
        if (k.wrote && rnd() < 0.7) add({ type: "ember", x: x + (rnd() - 0.5) * cw, y: this.cy, vx: (rnd() - 0.5) * 30, vy: -40 - rnd() * 50, life: 700 + rnd() * 400, size: 1 + rnd() * 1.6 });
        break;
      case "firefly":
        this.flyCount += k.wrote;
        while (this.flyCount >= 3 && this.flies.length < 12) { this.flyCount -= 3; this.flies.push({ ang: rnd() * 6.28, r: LINE * (0.9 + rnd() * 1.2), speed: (0.6 + rnd()) * (rnd() > 0.5 ? 1 : -1), ph: rnd() * 6, born: now, away: false, leave: 0 }); }
        if (now - this.lastAct > 2200) for (const f of this.flies) if (!f.away) { f.away = true; f.leave = now + rnd() * 600; }
        if (k.wrote) for (const f of this.flies) f.away = false;
        break;
      case "season": {
        const ph = this.season(now);
        if (ph === 0 && rnd() < dt * 1.6) add({ type: "dew", x: this.cx + rnd() * cw, y: this.cy + LINE, vx: 0, vy: 10, grav: 60, life: 900, size: 1.6 + rnd() });
        if (ph === 2 && rnd() < dt * 1.8) add({ type: "leaf", x: x + (rnd() - 0.5) * LINE, y: this.cy - LINE * 0.6, vx: 0, vy: 18, life: 1800, size: LINE * 0.32, spin: 2, col: rnd() < 0.5 ? "#f97316" : "#eab308" });
        break;
      }
    }
  }
  season(now) { return Math.floor(((now / 1000 + this.seasonT0 * 6) % 24) / 6); } // dawn, day, dusk, night: six seconds each

  draw(c, now) {
    if (this.vis <= 0.001) return;
    const cw = this.host.cw;
    c.save();
    c.globalAlpha = this.vis * this.host.fade(this.cx, this.cy);
    if (c.globalAlpha <= 0.01) { c.restore(); return; }
    // the note
    c.font = FONT; c.textBaseline = "middle";
    if (this.sel && this.text) { c.fillStyle = this.host.sel; c.fillRect(this.x, this.y - 1, this.text.length * cw, LINE + 2); }
    c.fillStyle = this.host.ink || rgba(INK[this.kind], 0.72);
    for (let i = 0; i < this.text.length; i++) c.fillText(this.text[i], this.x + i * cw + cw * 0.02, this.y + LINE / 2 + 1);
    this.drawMotes(c, now);
    const b = { x: this.cx, y: this.cy, w: cw, h: LINE };
    const rest = now - this.lastAct > 500 && this.state !== "glide";
    const blinkOn = !rest || Math.floor(now / 530) % 2 === 0;
    this[`draw_${this.kind}`](c, b, now, blinkOn);
    c.restore();
  }
  drawMotes(c, now) {
    for (const m of this.motes) {
      const age = (now - m.t0) / m.life;
      c.save(); c.globalAlpha *= 1 - age;
      switch (m.type) {
        case "coin": {
          const t = age, h = LINE;
          let rise = 1.5 * h * (1 - (1 - t) * (1 - t)); if (t > 0.7) rise -= 0.35 * h * clamp01((t - 0.7) / 0.3);
          const ch = 0.62 * h, w = ch * 0.72 * Math.max(0.12, Math.abs(Math.cos((now - m.t0) / 1000 * 14)));
          const cy = m.y - rise - ch / 2;
          c.fillStyle = "#ac7c00"; c.fill(rrect(m.x - w / 2, cy - ch / 2, w, ch, w / 2));
          c.fillStyle = "#f8b800"; c.fill(rrect(m.x - w * 0.32, cy - ch * 0.4, w * 0.64, ch * 0.8, w * 0.32));
          if (w > ch * 0.3) { c.fillStyle = "#fcf0b0"; c.fillRect(m.x - w * 0.08, cy - ch * 0.25, w * 0.16, ch * 0.5); }
          break;
        }
        case "star": {
          const s = m.size * (1 - age * 0.6);
          c.translate(m.x, m.y); c.rotate(m.ang); c.fillStyle = m.col; c.beginPath();
          c.moveTo(0, -s); c.quadraticCurveTo(0, 0, s, 0); c.quadraticCurveTo(0, 0, 0, s); c.quadraticCurveTo(0, 0, -s, 0); c.quadraticCurveTo(0, 0, 0, -s); c.fill();
          break;
        }
        case "heart": c.font = `${m.size}px sans-serif`; c.textAlign = "center"; c.fillStyle = "#fb7185"; c.fillText("♥", m.x, m.y); break;
        case "tilde": c.font = `700 ${m.size}px ui-monospace,monospace`; c.textAlign = "center"; c.fillStyle = "#c4b5fd"; c.fillText("~", m.x, m.y); break;
        case "ember": dot(c, m.x, m.y, m.size * (1 - age * 0.5), age < 0.4 ? "#fff1b8" : "#ff8a3d"); break;
        case "dew": dot(c, m.x, m.y, m.size, "rgba(186,230,253,.9)"); break;
        case "leaf": {
          m.vx = 18 * Math.sin((now - m.t0) / 180);
          const s = m.size; c.translate(m.x, m.y); c.rotate(m.ang); c.fillStyle = m.col; c.beginPath();
          c.moveTo(0, 0); c.quadraticCurveTo(s * 0.6, -s * 0.5, s, -s * 0.15); c.quadraticCurveTo(s * 0.5, s * 0.15, 0, 0); c.fill();
          break;
        }
      }
      c.restore();
    }
  }

  // ---- the carets ----
  draw_slime(c, b, now, on) {
    // a wet jelly: it squashes after each letter and wobbles back
    const t = (now - this.lastAct) / 1000, k = Math.exp(-t * 7) * Math.sin(t * 30) * 0.22;
    const w = b.w * (1 + k), h = b.h * (1 - k * 0.8);
    const bb = { x: b.x + (b.w - w) / 2, y: b.y + b.h - h, w, h };
    glass(c, bb, "#2ef2b8", on ? 1 : 0.55, Math.min(w, h) * 0.45);
    dot(c, bb.x + w * 0.32, bb.y + h * 0.22, Math.max(1, w * 0.13), "rgba(255,255,255,.85)");
  }
  draw_crt(c, b, now, on) {
    if (!on) return;
    c.fillStyle = "#4dffa3"; c.fillRect(b.x, b.y, b.w, b.h);
    c.fillStyle = "rgba(0,0,0,.22)"; for (let y = b.y + 2; y < b.y + b.h; y += 3) c.fillRect(b.x, y, b.w, 1);
  }
  draw_torch(c, b, now) {
    // the flame: narrow at the top, licking
    const fl = 0.85 + 0.15 * Math.sin(now / 43) * Math.sin(now / 71);
    const cx = b.x + b.w / 2, base = b.y + b.h, top = b.y - b.h * 0.15 * fl;
    const sway = Math.sin(now / 90) * b.w * 0.25;
    const g = c.createLinearGradient(0, top, 0, base);
    g.addColorStop(0, "rgba(255,240,170,.95)"); g.addColorStop(0.45, "rgba(255,150,40,.9)"); g.addColorStop(1, "rgba(255,70,10,.85)");
    c.fillStyle = g; c.beginPath();
    c.moveTo(cx + sway, top);
    c.bezierCurveTo(cx + b.w * 0.9, b.y + b.h * 0.35, cx + b.w * 0.75, base, cx, base);
    c.bezierCurveTo(cx - b.w * 0.75, base, cx - b.w * 0.9, b.y + b.h * 0.35, cx + sway, top);
    c.fill();
    dot(c, cx, base - b.h * 0.25, b.w * 0.22, "rgba(255,255,230,.9)");
  }
  draw_mario(c, b, now) {
    let lift = 0; const t = (now - this.hop) / 230; if (t < 1) lift = 0.32 * b.h * 4 * t * (1 - t);
    const bb = { ...b, y: b.y - lift };
    const p = rrect(bb.x, bb.y, bb.w, bb.h, 3);
    const g = c.createLinearGradient(0, bb.y, 0, bb.y + bb.h);
    g.addColorStop(0, "rgba(255,160,150,.75)"); g.addColorStop(0.3, "rgba(255,75,62,.6)"); g.addColorStop(0.7, "rgba(91,124,255,.6)"); g.addColorStop(1, "rgba(91,124,255,.7)");
    c.fillStyle = g; c.fill(p);
    const s = c.createLinearGradient(0, bb.y, 0, bb.y + bb.h); s.addColorStop(0.3, "#ff4b3e"); s.addColorStop(0.7, "#5b7cff");
    c.strokeStyle = s; c.lineWidth = 1.25; c.stroke(p);
    const d = this.left ? -1 : 1, r = Math.max(1.5, Math.min(bb.w * 0.15, 2.6));
    const ex = bb.x + bb.w / 2 + d * bb.w * 0.17, ey = bb.y + bb.h * 0.24;
    dot(c, ex, ey, r, "#1a0e08"); dot(c, ex + r * 0.15, ey - r * 0.25, r * 0.35, "rgba(255,255,255,.9)");
  }
  draw_firefly(c, b, now) {
    const cx = b.x + b.w / 2, cy = b.y + b.h / 2, dt = this.dt || 1 / 60;
    this.flies = this.flies.filter((f) => {
      f.ang += f.speed * dt;
      let a = clamp01((now - f.born) / 400);
      if (f.away && now > f.leave) { f.r += LINE * 2 * dt; a *= 1 - clamp01((now - f.leave) / 1200); if (a <= 0) return false; }
      const x = cx + Math.cos(f.ang) * f.r, y = cy + Math.sin(f.ang * 1.3 + f.ph) * f.r * 0.55;
      const pulse = 0.55 + 0.45 * Math.sin(now / 1000 * 4 + f.ph);
      dot(c, x, y, 5, `rgba(253,224,71,${a * pulse * 0.14})`); dot(c, x, y, 3, `rgba(253,224,71,${a * pulse * 0.32})`); dot(c, x, y, 1.6, `rgba(255,250,200,${a * pulse})`);
      return true;
    });
    glass(c, b, "#d9b23a", 1);
  }
  draw_season(c, b, now) {
    const ph = this.season(now), cx = b.x + b.w / 2, cy = b.y + b.h / 2;
    const col = ["#f9a8d4", "#facc15", "#fb923c", "#6366f1"][ph];
    if (ph === 1) { // the sun's rays, turning
      c.save(); c.translate(cx, cy); c.rotate(now / 2400); c.strokeStyle = "rgba(253,224,71,.75)"; c.lineWidth = 1.4;
      for (let i = 0; i < 8; i++) { c.rotate(Math.PI / 4); c.beginPath(); c.moveTo(0, b.h * 0.7); c.lineTo(0, b.h * (0.95 + 0.1 * Math.sin(now / 300 + i))); c.stroke(); }
      c.restore();
    }
    if (ph === 3) { // stars about it, twinkling
      for (let i = 0; i < 5; i++) {
        const a = i * 1.26 + this.seasonT0, r = b.h * (0.9 + (i % 2) * 0.5);
        const tw = 0.4 + 0.6 * Math.abs(Math.sin(now / 500 + i * 1.7));
        dot(c, cx + Math.cos(a) * r, cy + Math.sin(a) * r * 0.7, 1.1 + (i % 2) * 0.5, `rgba(224,231,255,${tw})`);
      }
    }
    glass(c, b, col, 1);
  }
  draw_kawaii(c, b, now) {
    const w = Math.max(b.w * 2.3, b.h * 1.2), bb = { x: b.x + b.w / 2 - w / 2, y: b.y, w, h: b.h };
    if (bb.x < b.x - w * 0.15) bb.x = b.x - w * 0.15;
    glass(c, bb, mixHex("#f9a8d4", "#d8b4fe", 0.25 + 0.25 * Math.sin(now / 1500)), 1, 5);
    const ink = "#3b0764", glad = now - this.joy < 700;
    if (now > this.blinkAt + 130) this.blinkAt = now + 2200 + rnd() * 2600;
    const blinking = now > this.blinkAt && now < this.blinkAt + 130;
    const er = Math.max(1.2, bb.h * 0.11);
    for (const ox of [0.28, 0.72]) {
      const ex = bb.x + bb.w * ox, ey = bb.y + bb.h * 0.42;
      c.fillStyle = ink; c.strokeStyle = ink; c.lineWidth = Math.max(0.9, er * 0.5);
      if (glad) { c.beginPath(); c.moveTo(ex - er, ey + er * 0.3); c.lineTo(ex, ey - er * 0.7); c.lineTo(ex + er, ey + er * 0.3); c.stroke(); }
      else if (blinking) c.fillRect(ex - er, ey, 2 * er, Math.max(0.8, er * 0.35));
      else { c.fill(rrect(ex - er, ey - er * 1.2, 2 * er, 2.4 * er, er)); dot(c, ex - er * 0.2, ey - er * 0.6, er * 0.35, "rgba(255,255,255,.95)"); }
    }
    for (const ox of [0.14, 0.86]) { c.fillStyle = "rgba(251,113,133,.6)"; c.fill(rrect(bb.x + bb.w * ox - er, bb.y + bb.h * 0.62, er * 2, er * 0.9, er * 0.45)); }
    const mx = bb.x + bb.w / 2, my = bb.y + bb.h * 0.68, mw = bb.h * 0.07;
    c.strokeStyle = ink; c.lineWidth = Math.max(0.8, bb.h * 0.05); c.beginPath();
    c.moveTo(mx - 2 * mw, my); c.quadraticCurveTo(mx - mw, my + mw * 1.4, mx, my); c.quadraticCurveTo(mx + mw, my + mw * 1.4, mx + 2 * mw, my); c.stroke();
  }
  draw_uwu(c, b, now) {
    let eyes = "u";
    if (now - this.sad < 900) eyes = "T"; else if (now - this.shock < 900) eyes = "O"; else if (this.rate > 7) eyes = "><";
    const w = Math.max(b.w * 2.6, b.h * 1.5), bb = { x: b.x + b.w / 2 - w / 2, y: b.y, w, h: b.h };
    if (bb.x < b.x - w * 0.15) bb.x = b.x - w * 0.15;
    glass(c, bb, "#a78bfa", 0.95, 5);
    const ink = "#2e1065", size = b.h * 0.62, cy = bb.y + bb.h * 0.5;
    c.fillStyle = ink; c.textAlign = "center"; c.textBaseline = "middle"; c.font = `800 ${size}px ui-monospace,monospace`;
    const [l, r] = eyes === "><" ? [">", "<"] : [eyes, eyes];
    c.fillText(l, bb.x + bb.w * 0.2, cy); c.fillText(r, bb.x + bb.w * 0.8, cy);
    c.font = `800 ${size * 0.85}px ui-monospace,monospace`; c.fillText("w", bb.x + bb.w * 0.5, cy + b.h * 0.06);
    for (const ox of [0.08, 0.92]) { c.fillStyle = "rgba(244,114,182,.7)"; c.fill(rrect(bb.x + bb.w * ox - bb.h * 0.08, bb.y + bb.h * 0.68, bb.h * 0.16, bb.h * 0.08, bb.h * 0.04)); }
  }
}

// ---- one card: a caret, writing its notes one after another at the same place ----
// canvas: the card's canvas (sized here to its box); ink and sel: CSS colors of the letters and of a selection.
export function createCard(canvas, kind, { reduce = false, ink = null, sel = "rgba(0,120,255,.22)" } = {}) {
  const c = canvas.getContext("2d");
  const measure = document.createElement("canvas").getContext("2d");
  measure.font = FONT;
  let W = 1, H = 1, dpr = 1;
  const pad = 18;
  const host = {
    cw: measure.measureText("M").width, ink, sel, used: new Set(),
    note() {
      const fits = NOTES.filter((n) => this.span(n) < W - pad * 2);
      const free = fits.filter((n) => !this.used.has(n));
      const pool = free.length ? free : fits.length ? fits : NOTES;
      const n = pool[(rnd() * pool.length) | 0];
      this.used.add(n); if (this.used.size > fits.length - 3) this.used.clear();
      return n;
    },
    span(n) { return (n.length + 3) * this.cw; },
    place() { return { x: pad, y: Math.round((H - LINE) / 2 + LINE * 0.35) }; }, // a little low: room above for coins and hearts
    free() {},
    fade() { return 1; },
  };
  const w = new Writer(kind, host);
  function resize() {
    dpr = Math.min(devicePixelRatio || 1, 2);
    W = canvas.clientWidth || 1; H = canvas.clientHeight || 1;
    canvas.width = Math.round(W * dpr); canvas.height = Math.round(H * dpr);
    if (reduce) { const now = performance.now(); w.begin(now); w.text = w.target; w.cx = w.x + w.text.length * host.cw; w.cy = w.y; w.state = "rest"; w.vis = 1; w.until = Infinity; }
  }
  resize();
  return {
    resize,
    setInk(i, s2) { host.ink = i || null; if (s2) host.sel = s2; },
    frame(now, dt) {
      if (!reduce) w.step(now, dt);
      c.setTransform(1, 0, 0, 1, 0, 0); c.clearRect(0, 0, canvas.width, canvas.height);
      c.setTransform(dpr, 0, 0, dpr, 0, 0); w.draw(c, now);
    },
  };
}
