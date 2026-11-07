// fxkit: a page drawn through a shader. What a demo gives is one function of GLSL, `vec3 shade(vec2 uv)`; this
// does the rest.
//
// The page (an element, `ui`) is real HTML. Where the browser has HTML-in-canvas it is drawn *into* the shader as
// a texture, so the shader changes the text, the field and the button themselves, and they still work (focus,
// typing, a screen reader). That API has had two spellings, and both are tried:
//   - the first:  <canvas layoutsubtree>, texElementSubImage2D, updateElementGeometry
//   - the draft:  <canvas content="drawable"> and a `drawable` attribute on the element
// Whichever the browser has, it is checked at run time (a picture of the page must really arrive), and if it does
// not, the page is put back over the canvas as plain HTML and the shader paints only what is behind it.
// ?debug on the address shows what the browser has and what was used.
//
// In the shader, in `shade`:  ui(uv) is the page at uv (y up, 0..1), with alpha;  uRes the canvas in pixels;
// uT seconds;  uScale device pixels per CSS pixel;  uMouse (x, y, speed, seconds since it moved);  uFocus 1 when
// a field of the page has focus;  uOn 0..1 over the first second;  uHasUI 1 when the page is in the shader;
// uAtlas / uAtlasN a strip of characters, when `atlas` is given.

const VERT = `#version 300 es
out vec2 vUv;
void main(){vec2 p=vec2(float((gl_VertexID<<1)&2),float(gl_VertexID&2));vUv=p;gl_Position=vec4(p*2.-1.,0.,1.);}`;

const PRELUDE = `#version 300 es
precision highp float;
in vec2 vUv;out vec4 o;
uniform sampler2D uUI,uAtlas;
uniform vec2 uRes;
uniform float uT,uScale,uHasUI,uFocus,uOn,uAtlasN;
uniform vec4 uMouse;
vec4 ui(vec2 uv){return uHasUI>.5?texture(uUI,vec2(uv.x,1.-uv.y)):vec4(0.);}
float hash11(float p){p=fract(p*.1031);p*=p+33.33;p*=p+p;return fract(p);}
float hash21(vec2 p){p=fract(p*vec2(233.34,851.73));p+=dot(p,p+23.45);return fract(p.x*p.y);}
float vnoise(vec2 p){vec2 i=floor(p),f=fract(p);f=f*f*(3.-2.*f);
return mix(mix(hash21(i),hash21(i+vec2(1,0)),f.x),mix(hash21(i+vec2(0,1)),hash21(i+vec2(1,1)),f.x),f.y);}
`;
const TAIL = `
void main(){o=vec4(shade(vUv),1.);}`;

const CSS = `
.fx-canvas{position:fixed;left:0;top:0;width:100vw;height:100vh;display:block;z-index:0}
.fx-ui{position:fixed;inset:0;z-index:1}
.fx-ui.in-canvas{position:absolute;inset:auto;left:0;top:0;width:var(--w,100vw);height:var(--h,100vh)}
.fx-debug{position:fixed;left:8px;bottom:8px;z-index:99999;max-width:min(520px,92vw);margin:0;padding:8px 10px;border-radius:6px;background:rgba(0,0,0,.78);color:#9fffc4;font:11px/1.45 ui-monospace,Menlo,monospace;white-space:pre-wrap;pointer-events:none}
`;

export function mount({ canvas, ui, frag, atlas, probe, pixelCap = 3.4e6, minQuality = .5 }) {
  const root = document.documentElement;
  const params = new URLSearchParams(location.search);
  const reduce = matchMedia("(prefers-reduced-motion: reduce)").matches;
  const st = document.createElement("style"); st.textContent = CSS; document.head.append(st);
  canvas.classList.add("fx-canvas"); ui.classList.add("fx-ui");

  const gl = canvas.getContext("webgl2", { alpha: false, antialias: false, powerPreference: "high-performance" });
  const caps = {
    webgl2: !!gl, onpaint: "onpaint" in canvas, layoutSubtree: "layoutSubtree" in canvas, content: "content" in canvas,
    texElementSubImage2D: !!gl && typeof gl.texElementSubImage2D === "function", updateElementGeometry: typeof canvas.updateElementGeometry === "function",
    chrome: (navigator.userAgent.match(/Chrome\/([\d.]+)/) || [])[1] || "", api: "", status: "", uploads: 0, error: "",
  };
  const fx = { caps, hasUI: false, gl };
  if (!gl) { root.classList.add("nogl"); canvas.remove(); fx.hasUI = false; return fx; }

  // ---- the page into the canvas, if the browser can
  let hasUI = !params.has("nohtml") && caps.onpaint && caps.texElementSubImage2D;
  const home = ui.parentNode, after = ui.nextSibling;
  function enter() {
    canvas.setAttribute("layoutsubtree", ""); canvas.setAttribute("content", "drawable"); ui.setAttribute("drawable", "");
    canvas.append(ui); ui.classList.add("in-canvas"); caps.status = "trying";
  }
  function revert(why) {
    if (!hasUI) return;
    hasUI = false; fx.hasUI = false; caps.status = "fallback: " + why;
    canvas.removeAttribute("layoutsubtree"); canvas.removeAttribute("content"); ui.removeAttribute("drawable");
    ui.classList.remove("in-canvas"); home.insertBefore(ui, after && after.parentNode === home ? after : null);
    root.classList.add("nofx"); canvas.onpaint = null;
  }
  if (hasUI) { enter(); } else { root.classList.add("nofx"); caps.status = "fallback: " + (params.has("nohtml") ? "asked" : "not supported"); }
  fx.hasUI = hasUI;

  // ---- the program
  const sh = (type, src) => { const s = gl.createShader(type); gl.shaderSource(s, src); gl.compileShader(s); if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) throw new Error(gl.getShaderInfoLog(s)); return s; };
  let prog;
  try {
    prog = gl.createProgram(); gl.attachShader(prog, sh(gl.VERTEX_SHADER, VERT)); gl.attachShader(prog, sh(gl.FRAGMENT_SHADER, PRELUDE + frag + TAIL)); gl.linkProgram(prog);
    if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) throw new Error(gl.getProgramInfoLog(prog));
  } catch (e) { console.error(e); caps.error = String(e.message || e).slice(0, 300); revert("shader"); root.classList.add("nogl"); canvas.remove(); return fx; }
  const loc = {}; const U = (n) => loc[n] ?? (loc[n] = gl.getUniformLocation(prog, n));
  gl.useProgram(prog); gl.bindVertexArray(gl.createVertexArray());

  const mkTex = (w, h) => {
    const t = gl.createTexture(); gl.bindTexture(gl.TEXTURE_2D, t);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, w, h, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
    for (const [k, v] of [[gl.TEXTURE_MIN_FILTER, gl.LINEAR], [gl.TEXTURE_MAG_FILTER, gl.LINEAR], [gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE], [gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE]]) gl.texParameteri(gl.TEXTURE_2D, k, v);
    return t;
  };
  let atlasTex = null, atlasN = 1;
  if (atlas) {
    const { chars, font = "700 30px ui-monospace,Menlo,Consolas,monospace", cw = 20, ch = 36 } = atlas;
    const c = document.createElement("canvas"); c.width = cw * chars.length; c.height = ch; const x = c.getContext("2d");
    x.fillStyle = "#000"; x.fillRect(0, 0, c.width, ch); x.fillStyle = "#fff"; x.font = font; x.textAlign = "center"; x.textBaseline = "middle";
    for (let i = 0; i < chars.length; i++) x.fillText(chars[i], cw * i + cw / 2, ch / 2 + 1);
    atlasTex = gl.createTexture(); gl.bindTexture(gl.TEXTURE_2D, atlasTex); gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, c);
    for (const [k, v] of [[gl.TEXTURE_MIN_FILTER, gl.LINEAR], [gl.TEXTURE_MAG_FILTER, gl.LINEAR], [gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE], [gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE]]) gl.texParameteri(gl.TEXTURE_2D, k, v);
    atlasN = chars.length;
  }

  // ---- size
  let quality = 1, W = 0, H = 0, scale = 1, cssW = 0, cssH = 0, uiTex = null;
  function size() {
    cssW = Math.max(1, innerWidth); cssH = Math.max(1, innerHeight);
    scale = Math.min(devicePixelRatio || 1, 2) * quality;
    while (cssW * cssH * scale * scale > pixelCap && scale > .5) scale -= .1;
    const nw = Math.round(cssW * scale), nh = Math.round(cssH * scale);
    if (nw === W && nh === H) return;
    W = canvas.width = nw; H = canvas.height = nh;
    ui.style.setProperty("--w", cssW + "px"); ui.style.setProperty("--h", cssH + "px");
    if (hasUI) { if (uiTex) gl.deleteTexture(uiTex); uiTex = mkTex(W, H); canvas.requestPaint(); }
  }
  // where the page is drawn is told to the browser, or it takes no clicks: the canvas is in device pixels, the page in CSS ones
  function fixGeometry() {
    if (!hasUI || typeof canvas.updateElementGeometry !== "function") return;
    const target = () => (probe && ui.querySelector(probe)) || ui.querySelector("input,button,a") || ui;
    const hit = () => { const r = target().getBoundingClientRect(); const e = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2); return !!e && ui.contains(e); };
    for (const m of [new DOMMatrix([scale, 0, 0, scale, 0, 0]), new DOMMatrix()]) {
      try { canvas.updateElementGeometry(ui, { canvasTransform: m }); } catch (e) { caps.error = "geometry: " + e.message; continue; }
      if (hit()) { caps.api = (caps.api || "") ; return; }
    }
  }
  function upload() {
    if (!hasUI || !uiTex) return;
    gl.activeTexture(gl.TEXTURE1); gl.bindTexture(gl.TEXTURE_2D, uiTex);
    try { gl.texElementSubImage2D(gl.TEXTURE_2D, 0, 0, 0, ui, { width: W, height: H }); }
    catch (e1) {
      try { gl.texElementSubImage2D(gl.TEXTURE_2D, 0, 0, 0, ui); } catch (e2) { caps.error = "upload: " + (e2.message || e2); caps.uploadFailures = (caps.uploadFailures || 0) + 1; if (caps.uploadFailures > 3 && !caps.uploads) revert("upload failed"); return; }
    }
    caps.uploads++; if (caps.status === "trying") caps.status = "html-in-canvas";
  }
  if (hasUI) {
    caps.api = caps.layoutSubtree ? "layoutsubtree" : caps.content ? "content=drawable" : "unknown";
    canvas.onpaint = () => { upload(); fixGeometry(); if (reduce) draw(performance.now()); };
    setTimeout(() => { if (hasUI && !caps.uploads) revert("no picture of the page arrived"); }, 2500);
  }
  size();
  new ResizeObserver(() => { size(); if (reduce) draw(performance.now()); }).observe(root);
  addEventListener("resize", () => { size(); if (reduce) draw(performance.now()); });
  canvas.addEventListener("webglcontextlost", (e) => e.preventDefault());
  canvas.addEventListener("webglcontextrestored", () => location.reload());

  // ---- the pointer, smoothed
  const m = { x: .5, y: .5, tx: .5, ty: .5, sp: 0, moved: 99, last: performance.now() - 1e7 };
  addEventListener("pointermove", (e) => { m.tx = e.clientX / cssW; m.ty = 1 - e.clientY / cssH; m.last = performance.now(); }, { passive: true });
  let focus = 0;

  // ---- the graphics card's time (a timer query): the picture is made smaller only if that is long
  const tq = gl.getExtension("EXT_disjoint_timer_query_webgl2");
  let qNow = null, qDone = null, gpuMs = 0, evalAt = performance.now() + 3000;
  const gpuBegin = () => { if (!tq || qNow || qDone) return; qNow = gl.createQuery(); gl.beginQuery(tq.TIME_ELAPSED_EXT, qNow); };
  const gpuEnd = () => { if (qNow) { gl.endQuery(tq.TIME_ELAPSED_EXT); qDone = qNow; qNow = null; } };
  function adapt(now) {
    if (qDone && gl.getQueryParameter(qDone, gl.QUERY_RESULT_AVAILABLE)) {
      if (!gl.getParameter(tq.GPU_DISJOINT_EXT)) { const ms = gl.getQueryParameter(qDone, gl.QUERY_RESULT) / 1e6; gpuMs = gpuMs ? gpuMs * .9 + ms * .1 : ms; }
      gl.deleteQuery(qDone); qDone = null;
    }
    if (!tq || now < evalAt) return;
    evalAt = now + 1500;
    if (gpuMs > 14 && quality > minQuality) { quality = Math.max(minQuality, quality * .8); size(); gpuMs = 0; }
  }

  // ---- a frame
  const t0 = performance.now(); let lastT = t0;
  function draw(now) {
    const dt = Math.min(.05, (now - lastT) / 1000); lastT = now;
    const T = reduce ? 9 : (now - t0) / 1000;
    m.x += (m.tx - m.x) * Math.min(1, dt * 14); m.y += (m.ty - m.y) * Math.min(1, dt * 14);
    const mv = Math.hypot(m.tx - m.x, m.ty - m.y); m.sp += (Math.min(1, mv * 12) - m.sp) * Math.min(1, dt * 8);
    const since = (now - m.last) / 1000;
    focus += (ui.contains(document.activeElement) && /^(INPUT|TEXTAREA|BUTTON)$/.test(document.activeElement.tagName) ? 1 - focus : -focus) * Math.min(1, dt * 6);
    gl.useProgram(prog); gl.bindFramebuffer(gl.FRAMEBUFFER, null); gl.viewport(0, 0, W, H);
    gl.activeTexture(gl.TEXTURE1); if (hasUI && uiTex) gl.bindTexture(gl.TEXTURE_2D, uiTex); else if (atlasTex) gl.bindTexture(gl.TEXTURE_2D, atlasTex);
    gl.activeTexture(gl.TEXTURE2); if (atlasTex) gl.bindTexture(gl.TEXTURE_2D, atlasTex);
    gl.uniform1i(U("uUI"), 1); gl.uniform1i(U("uAtlas"), 2);
    gl.uniform2f(U("uRes"), W, H); gl.uniform1f(U("uT"), T); gl.uniform1f(U("uScale"), scale); gl.uniform1f(U("uHasUI"), hasUI && caps.uploads ? 1 : 0);
    gl.uniform1f(U("uFocus"), focus); gl.uniform1f(U("uOn"), reduce ? 1 : Math.min(1, T / 1)); gl.uniform1f(U("uAtlasN"), atlasN);
    gl.uniform4f(U("uMouse"), m.x, m.y, m.sp, since);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
  }
  let lastPaint = 0, fps = 0, fpsAt = performance.now(), frames = 0;
  function loop(now) {
    adapt(now);
    // the native caret of a field blinks without a change the page would report: a new picture now and then
    if (hasUI && ui.contains(document.activeElement) && now - lastPaint > 200) { lastPaint = now; canvas.requestPaint(); }
    gpuBegin(); draw(now); gpuEnd();
    frames++; if (now - fpsAt > 1000) { fps = frames * 1000 / (now - fpsAt); frames = 0; fpsAt = now; }
    requestAnimationFrame(loop);
  }
  if (hasUI) canvas.requestPaint();
  if (reduce) draw(performance.now()); else requestAnimationFrame(loop);

  // ---- ?debug
  if (params.has("debug")) {
    const d = document.createElement("pre"); d.className = "fx-debug"; document.body.append(d);
    setInterval(() => { d.textContent = [
      "chrome " + caps.chrome + " · webgl2 " + caps.webgl2,
      "onpaint " + caps.onpaint + " · layoutSubtree " + caps.layoutSubtree + " · content " + caps.content,
      "texElementSubImage2D " + caps.texElementSubImage2D + " · updateElementGeometry " + caps.updateElementGeometry,
      "used: " + (caps.api || "-") + " · " + caps.status + " · uploads " + caps.uploads,
      "canvas " + W + "x" + H + " (x" + scale.toFixed(2) + ") · " + fps.toFixed(0) + " fps · gpu " + gpuMs.toFixed(1) + " ms",
      caps.error ? "error: " + caps.error : "",
    ].filter(Boolean).join("\n"); }, 500);
  }
  fx.size = size; fx.draw = draw;
  return fx;
}

// the form of the waitlist, the same in every demo; with ?mock (or no server) the answer is made up here
export function bindForm({ form, slot, msg, go, input, endpoint = "/api/waitlist" }) {
  form.addEventListener("submit", async (e) => {
    e.preventDefault(); msg.className = "msg"; msg.textContent = ""; go.disabled = true;
    const ok = () => {
      slot.replaceChildren(); const d = document.createElement("div"); d.className = "done"; d.setAttribute("role", "status");
      d.textContent = "✓ Listedesin. Onaylanınca e-postana bir davet kodu gelecek."; slot.append(d);
    };
    if (new URLSearchParams(location.search).has("mock") || location.hostname.endsWith("workers.dev") || location.protocol === "file:") {
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(input.value)) { msg.className = "msg bad"; msg.textContent = "e-posta geçersiz"; go.disabled = false; return; }
      ok(); return;
    }
    try {
      const t = document.querySelector("[name=cf-turnstile-response]");
      const r = await fetch(endpoint, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ email: input.value, turnstile: t ? t.value : "" }) });
      const j = await r.json().catch(() => ({}));
      if (r.ok) ok(); else { msg.className = "msg bad"; msg.textContent = j.error || "Olmadı, sonra tekrar dene."; go.disabled = false; }
    } catch { msg.className = "msg bad"; msg.textContent = "Bağlantı kurulamadı."; go.disabled = false; }
  });
}
