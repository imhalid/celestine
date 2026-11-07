<script>
  // A card with one of Ryudit's carets in it, writing little notes and erasing them (writers.js). The page's ink is read
  // from the card itself, so it follows light and dark. App.svelte runs one frame loop for all the cards.
  import { onMount } from "svelte";
  import { createCard } from "./writers.js";

  let { kind, name, hint, register } = $props();
  let canvas = $state(), card = $state();

  onMount(() => {
    const reduce = matchMedia("(prefers-reduced-motion: reduce)").matches;
    // the selection's color is carried by text-decoration-color, which the browser hands back resolved (a custom property would not be)
    const colors = () => { const cs = getComputedStyle(card); return [cs.color, cs.textDecorationColor]; };
    const [ink, sel] = colors();
    const w = createCard(canvas, kind, { reduce, ink, sel });
    const ro = new ResizeObserver(() => w.resize());
    ro.observe(canvas);
    // the scheme changed (the toggle, or the system): the ink with it
    const mo = new MutationObserver(() => w.setInk(...colors()));
    mo.observe(document.documentElement, { attributes: true, attributeFilter: ["data-scheme"] });
    const mq = matchMedia("(prefers-color-scheme: dark)");
    const onScheme = () => w.setInk(...colors());
    mq.addEventListener("change", onScheme);
    const off = register(card, w);
    if (reduce) w.frame(performance.now(), 0);
    return () => { ro.disconnect(); mo.disconnect(); mq.removeEventListener("change", onScheme); off(); };
  });
</script>

<article class="card" bind:this={card}>
  <header><span class="name">{name}</span><span class="hint">{hint}</span></header>
  <canvas bind:this={canvas} aria-hidden="true"></canvas>
</article>

<style>
  .card {
    text-decoration-color: color-mix(in oklab, var(--accent) 24%, transparent);
    display: flex;
    flex-direction: column;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 16px;
    box-shadow: var(--shadow-sm);
    color: var(--text);
    overflow: hidden;
    transition: border-color 0.2s, box-shadow 0.2s, translate 0.3s var(--ease);
  }
  .card:hover { border-color: color-mix(in oklab, var(--text) 16%, transparent); box-shadow: var(--shadow); translate: 0 -2px; }
  header { display: flex; flex-direction: column; gap: 2px; padding: 14px 16px 0; }
  .name { font-size: 14px; font-weight: 600; letter-spacing: -0.01em; }
  .hint { font-size: 12.5px; color: var(--muted); }
  canvas { display: block; width: 100%; height: 104px; }
  @media (prefers-reduced-motion: reduce) { .card { transition: none; } .card:hover { translate: none; } }
</style>
