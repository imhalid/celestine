<script>
  // The entry screen, in the manner of ui.halid.dev (Mizan): calm and plain. A thin bar on top, the name being typed with
  // its caret and one line on what it is, the download for the visitor's computer and the others under it; below a hatched
  // rule, the app's carets each in a card, writing their little notes; a footer, where the waitlist of sync is, quietly: the
  // app needs no account, and sync is by invite. Light or dark as the system has it, or as the toggle says.
  import { onMount } from "svelte";
  import SignupForm from "./SignupForm.svelte";
  import CaretCard from "./CaretCard.svelte";

  let { contact = "", download = "" } = $props();

  // the builds, by the name the update server knows them by (cloudflare/update: /download/<os-arch>, which gives
  // macOS a zipped celestine.app and Linux a .tar.gz with its .desktop file)
  const BUILDS = [
    { id: "darwin-arm64", os: "macOS", chip: "Apple silicon" },
    { id: "darwin-amd64", os: "macOS", chip: "Intel" },
    { id: "windows-amd64", os: "Windows", chip: "x64" },
    { id: "linux-amd64", os: "Linux", chip: "x64" },
    { id: "linux-arm64", os: "Linux", chip: "ARM" },
  ];
  const href = (b) => download.replace(/\/+$/, "") + "/" + b.id;
  // the visitor's: a Mac is taken for Apple silicon (a browser does not say), its Intel build is the next link
  const mine = (() => {
    const ua = navigator.userAgent;
    if (/iPhone|iPad|Android/i.test(ua)) return null;
    if (/Mac/i.test(ua)) return BUILDS[0];
    if (/Win/i.test(ua)) return BUILDS[2];
    if (/Linux/i.test(ua)) return /aarch64|arm64/i.test(ua) ? BUILDS[4] : BUILDS[3];
    return null;
  })();
  const others = BUILDS.filter((b) => b !== mine);

  const NAME = "Celestine";
  const reduce = matchMedia("(prefers-reduced-motion: reduce)").matches;
  let typed = $state(reduce ? NAME : "");

  // light or dark: the system's, until the toggle is used (then remembered in this browser)
  const stored = (() => { try { return localStorage.getItem("celestine-scheme"); } catch { return null; } })();
  let scheme = $state(stored === "light" || stored === "dark" ? stored : matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light");
  function toggle() {
    scheme = scheme === "dark" ? "light" : "dark";
    document.documentElement.dataset.scheme = scheme;
    try { localStorage.setItem("celestine-scheme", scheme); } catch {}
  }

  const CARETS = [
    { kind: "slime", name: "Slime", hint: "Stretches with every letter" },
    { kind: "mario", name: "Mario", hint: "Hops at a space, out pops a coin" },
    { kind: "kawaii", name: "Kawaii", hint: "Sparkles as it writes, hearts at !" },
    { kind: "uwu", name: "UwU", hint: "> w < when quick, T w T when erasing" },
    { kind: "firefly", name: "Firefly", hint: "Fireflies gather as you write" },
    { kind: "season", name: "Season", hint: "Dawn, day, dusk, night" },
    { kind: "torch", name: "Torch", hint: "A flame, and its sparks" },
    { kind: "crt", name: "CRT", hint: "A block of phosphor" },
  ];

  // one frame loop for every card; a card off the screen is not drawn
  const cards = new Map();
  function register(el, w) {
    const entry = { w, seen: false };
    cards.set(el, entry);
    io?.observe(el);
    return () => { io?.unobserve(el); cards.delete(el); };
  }
  let io = typeof IntersectionObserver === "function" ? new IntersectionObserver((es) => { for (const e of es) { const c = cards.get(e.target); if (c) c.seen = e.isIntersecting; } }) : null;

  onMount(() => {
    let timer, raf, last = performance.now();
    if (!reduce) {
      const step = () => { typed = NAME.slice(0, typed.length + 1); if (typed.length < NAME.length) timer = setTimeout(step, 110 + Math.random() * 80); };
      timer = setTimeout(step, 500);
      const loop = (now) => {
        const dt = Math.min(0.033, (now - last) / 1000); last = now;
        for (const c of cards.values()) if (c.seen || !io) c.w.frame(now, dt);
        raf = requestAnimationFrame(loop);
      };
      raf = requestAnimationFrame(loop);
    }
    return () => { clearTimeout(timer); cancelAnimationFrame(raf); io?.disconnect(); };
  });
</script>

<div class="page">
  <nav class="bar">
    <div class="inner">
      <a class="brand" href="/" aria-label="Celestine">
        <img class="mark" src="/icon.png" alt="" width="24" height="24">Celestine
      </a>
      <div class="right">
        <span class="status">Early access</span>
        <button class="icon" type="button" onclick={toggle} aria-label={scheme === "dark" ? "Switch to light" : "Switch to dark"}>
          {#if scheme === "dark"}
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5Z" /></svg>
          {:else}
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></svg>
          {/if}
        </button>
      </div>
    </div>
  </nav>

  <main class="frame">
    <section class="hero">
      <span class="badge">Free <span aria-hidden="true">·</span> early access</span>
      <h1 aria-label={NAME}><span aria-hidden="true">{typed}</span><span class="tc" aria-hidden="true"></span></h1>
      <p class="lead">A one-of-a-kind notes app, built with MyGo.</p>
      <div class="get">
        {#if !download}
          <span class="dl off">Downloads are on their way</span>
        {:else if mine}
          <a class="dl" href={href(mine)} download>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 4v11M7 10.5l5 5 5-5M5 20h14" /></svg>
            Download for {mine.os}
          </a>
          <span class="chip">{mine.chip}</span>
        {/if}
      </div>
      {#if download}
        <p class="also">
          {mine ? "Also for" : "Download for"}
          {#each others as b, i (b.id)}<a href={href(b)} download>{b.os} ({b.chip})</a>{i < others.length - 1 ? ", " : ""}{/each}
        </p>
      {/if}
      <p class="fine">No account needed: your notes stay on your computer, in one file of their own.</p>
    </section>

    <div class="rule" aria-hidden="true"></div>

    <section class="carets">
      <div class="head">
        <h2>Someone to write with</h2>
        <p>Celestine's carets answer what you type, each in its own way. Here they all are, live.</p>
      </div>
      <div class="grid">
        {#each CARETS as c (c.kind)}
          <CaretCard kind={c.kind} name={c.name} hint={c.hint} {register} />
        {/each}
      </div>
    </section>

    <div class="rule" aria-hidden="true"></div>

    <footer class="foot">
      <div class="wait" id="waitlist">
        <p>Sync across your computers, encrypted end to end, is by invite while it is young. Leave your email and an invite code comes when there is room.</p>
        <SignupForm id="waitlist-form" prompt="" placeholder="you@example.com" label="Join the waitlist" kbd={false} />
        <p class="fine">Your email is only used to send your invite code, and is never shared.{contact}</p>
      </div>
      <div class="meta">
        <span>Celestine</span>
        <span>Made with MyGo</span>
        <a href="/privacy">Privacy</a>
      </div>
    </footer>
  </main>
</div>

<style>
  .page { min-height: 100%; }
  .inner, .frame { width: min(1180px, 100%); margin: 0 auto; }

  /* the bar */
  .bar { position: sticky; top: 0; z-index: 2; border-bottom: 1px solid var(--hairline); background: color-mix(in oklab, var(--bg) 82%, transparent); backdrop-filter: saturate(1.4) blur(12px); }
  .bar .inner { display: flex; align-items: center; justify-content: space-between; height: 56px; padding: 0 24px; }
  .brand { display: inline-flex; align-items: center; gap: 10px; font-weight: 600; font-size: 15px; letter-spacing: -0.01em; color: var(--text); text-decoration: none; }
  .mark { width: 24px; height: 24px; display: block; transform: scale(1.2); }
  .right { display: flex; align-items: center; gap: 14px; }
  .status { font-size: 13px; color: var(--muted); }
  .icon { width: 32px; height: 32px; display: grid; place-items: center; border-radius: 999px; color: var(--text); background: var(--control); border: 1px solid var(--control-border); box-shadow: var(--control-shadow); cursor: pointer; transition: filter 0.2s, transform 0.15s var(--ease); }
  .icon:hover { filter: brightness(var(--control-hover)); }
  .icon:active { transform: translateY(1px); }
  .icon:focus-visible { outline: none; box-shadow: var(--focus-ring); }

  /* the frame: two hairlines down the sides, as on ui.halid.dev */
  .frame { border-left: 1px solid var(--hairline); border-right: 1px solid var(--hairline); }

  /* the hero */
  .hero { position: relative; isolation: isolate; display: flex; flex-direction: column; align-items: center; text-align: center;
    padding: clamp(64px, 12vh, 128px) 24px clamp(200px, 30vw, 340px); }
  /* the picture behind it: two blue crystals in a meadow, at dawn or at night as the scheme is (--hero, set by landing.js),
     fading into the page at the bottom */
  .hero::before { content: ""; position: absolute; inset: 0; z-index: -1; background: var(--hero) center bottom / cover no-repeat;
    -webkit-mask-image: linear-gradient(#000 82%, transparent); mask-image: linear-gradient(#000 82%, transparent); }
  :global(html[data-scheme="dark"]) .hero::before { background-image: var(--hero-dark); }
  @media (prefers-color-scheme: dark) { :global(html:not([data-scheme="light"])) .hero::before { background-image: var(--hero-dark); } }
  :global(html[data-scheme="dark"]) .hero .lead, :global(html[data-scheme="dark"]) .hero .fine { color: oklch(86% .03 260); }
  @media (prefers-color-scheme: dark) { :global(html:not([data-scheme="light"])) .hero .lead, :global(html:not([data-scheme="light"])) .hero .fine { color: oklch(86% .03 260); } }
  .badge { font-size: 13px; color: var(--muted); background: var(--well); border: 1px solid var(--border); border-radius: 999px; padding: 6px 14px; }
  h1 { margin: 22px 0 0; display: flex; align-items: center; font-size: clamp(56px, 10vw, 104px); line-height: 1; font-weight: 700; letter-spacing: -0.05em; color: var(--text); min-height: 1em; }
  .tc { display: inline-block; width: 0.07em; height: 0.78em; margin-left: 0.06em; border-radius: 2px; background: var(--accent); animation: blink 1.05s steps(1) infinite; }
  .lead { margin: 18px 0 0; font-size: clamp(18px, 2.3vw, 24px); line-height: 1.35; letter-spacing: -0.015em; color: var(--muted); max-width: 30ch; }
  .fine { margin: 14px 0 0; font-size: 12.5px; color: var(--muted); max-width: 52ch; }

  /* the download: one button for this computer, the others as a line of links */
  .get { margin-top: 36px; display: flex; align-items: center; gap: 12px; flex-wrap: wrap; justify-content: center; }
  .dl { display: inline-flex; align-items: center; gap: 9px; height: 46px; padding: 0 22px; border-radius: 999px; font-size: 15px; font-weight: 600; letter-spacing: -0.01em;
    text-decoration: none; color: var(--on-accent); background: linear-gradient(180deg, var(--solid-top), var(--solid-bottom));
    border: 1px solid color-mix(in oklab, var(--solid-bottom) 80%, #000); box-shadow: var(--sheen), 0 1px 2px color-mix(in oklab, var(--accent) 45%, transparent);
    transition: filter 0.2s, transform 0.15s var(--ease); }
  .dl:hover { filter: brightness(1.1); }
  .dl:active { transform: translateY(1px); filter: brightness(0.95); }
  .dl:focus-visible { outline: none; box-shadow: var(--focus-ring); }
  .dl.off { color: var(--muted); background: var(--well); border-color: var(--border); box-shadow: none; font-weight: 500; }
  .chip { font-size: 13px; color: var(--muted); }
  .also { margin: 16px 0 0; font-size: 13.5px; color: var(--muted); max-width: 60ch; }
  .also a { color: var(--text); text-decoration: underline; text-decoration-color: var(--border); text-underline-offset: 3px; }
  .also a:hover { text-decoration-color: currentColor; }

  /* the hatched rule between sections, with its crosses at the frame's corners */
  .rule { position: relative; height: 26px; border-top: 1px solid var(--hairline); border-bottom: 1px solid var(--hairline);
    background: repeating-linear-gradient(-45deg, var(--hairline) 0 1px, transparent 1px 7px); margin: 0 -1px; }
  .rule::before, .rule::after { content: ""; position: absolute; top: -6px; width: 11px; height: 11px;
    background: linear-gradient(var(--muted), var(--muted)) center / 1px 100% no-repeat, linear-gradient(var(--muted), var(--muted)) center / 100% 1px no-repeat; opacity: 0.5; }
  .rule::before { left: -6px; } .rule::after { right: -6px; }

  /* the carets */
  .carets { padding: 64px 24px 72px; }
  .head { max-width: 620px; margin-bottom: 28px; }
  h2 { margin: 0; font-size: clamp(24px, 3vw, 32px); line-height: 1.15; font-weight: 700; letter-spacing: -0.03em; }
  .head p { margin: 10px 0 0; font-size: 16px; line-height: 1.5; color: var(--muted); }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 14px; }

  .foot { padding: 28px 24px; font-size: 13px; color: var(--muted); }
  .meta { display: flex; justify-content: space-between; gap: 16px; margin-top: 24px; }
  .meta a { color: inherit; }
  .wait { max-width: 460px; }
  .wait > p:first-child { margin: 0 0 10px; line-height: 1.5; }
  .wait .fine { margin-top: 2px; font-size: 12px; }

  @keyframes blink { 50% { opacity: 0; } }
  @media (prefers-reduced-motion: reduce) { .tc { animation: none; } }

  /* the waitlist, small, in the footer: the field and the button inside one rounded box, as ui.halid.dev's install line */
  .wait :global(.row) { display: flex; align-items: center; gap: 8px; background: var(--well); border: 1px solid var(--border); border-radius: 12px; padding: 4px 4px 4px 12px; transition: border-color 0.2s, box-shadow 0.2s; }
  .wait :global(.row:focus-within) { border-color: color-mix(in oklab, var(--accent) 55%, transparent); box-shadow: 0 0 0 4px color-mix(in oklab, var(--accent) 16%, transparent); }
  .wait :global(.field) { --fg: var(--text); --dim: var(--muted); --line: transparent; --line-hi: transparent; --bad: transparent;
    --caret: var(--accent); --caret-ink: var(--on-accent); --sel: color-mix(in oklab, var(--accent) 25%, transparent); --field-px: 0; --field-py: 0.4rem; font-size: 14px; }
  .wait :global(.go) { height: 34px; padding: 0 14px; border-radius: 999px; font: inherit; font-size: 13px; font-weight: 600; letter-spacing: -0.01em; white-space: nowrap; cursor: pointer;
    color: var(--on-accent); background: linear-gradient(180deg, var(--solid-top), var(--solid-bottom)); border: 1px solid color-mix(in oklab, var(--solid-bottom) 80%, #000);
    box-shadow: var(--sheen), 0 1px 2px color-mix(in oklab, var(--accent) 45%, transparent); transition: filter 0.2s, transform 0.15s var(--ease); }
  .wait :global(.go:hover) { filter: brightness(1.1); }
  .wait :global(.go:active) { transform: translateY(1px); filter: brightness(0.95); }
  .wait :global(.go:disabled) { opacity: 0.55; cursor: default; filter: none; transform: none; }
  .wait :global(.go:focus-visible) { outline: none; box-shadow: var(--focus-ring); }
  .wait :global(.msg) { min-height: 20px; margin: 8px 4px 0; font-size: 13px; color: var(--muted); }
  .wait :global(.msg.bad) { color: var(--danger); }
  .wait :global(.done) { display: flex; align-items: center; gap: 10px; padding: 14px 16px; border-radius: 14px; background: var(--well); border: 1px solid var(--border); font-size: 14px; color: var(--text); }
  .wait :global(.done svg) { color: var(--accent); flex: none; }
</style>
