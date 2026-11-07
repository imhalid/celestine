<script>
  // The e-mail field, drawn by the page instead of the browser: the text, the selection and a phosphor block caret
  // (the CRT caret of the app) are ours. A real <input> lies over it, transparent, and does everything else: the keyboard,
  // input methods, pasting, autofill, selecting with the pointer, a screen reader. What it holds is read back from it,
  // and its own scroll is followed, so what is drawn always matches what it would have shown.
  //
  // It is type="text" (with inputmode="email" for the phone's keyboard): an email input tells nothing of its selection.
  // The page's font is monospaced: a character is 1ch wide, so the caret and the selection are placed by index.
  import { onMount } from "svelte";

  let { value = $bindable(""), frame = $bindable(), input = $bindable(), placeholder = "", disabled = false, invalid = false, prompt = "❯" } = $props();

  let focused = $state(false);
  let a = $state(0); // selection start
  let b = $state(0); // selection end
  let back = $state(false); // the selection was made toward the start: the caret is at its start
  let scroll = $state(0);
  let kick = $state(0); // counts changes: the caret is solid, and jumps a little, after each

  const caretAt = $derived(back ? a : b);
  const under = $derived(value.slice(caretAt, caretAt + 1));

  function sync() {
    if (!input) return;
    a = input.selectionStart ?? input.value.length;
    b = input.selectionEnd ?? a;
    back = input.selectionDirection === "backward";
    scroll = input.scrollLeft;
  }
  function changed() {
    kick++;
    // the input scrolls after the change is laid out
    requestAnimationFrame(sync);
    sync();
  }

  onMount(() => {
    const sel = () => { if (document.activeElement === input) sync(); };
    document.addEventListener("selectionchange", sel);
    return () => document.removeEventListener("selectionchange", sel);
  });
</script>

<div class="field" class:focused class:invalid class:disabled class:bare={!prompt} bind:this={frame}>
  {#if prompt}<span class="prompt" aria-hidden="true">{prompt}</span>{/if}
  <div class="view" aria-hidden="true">
    <div class="line" style:transform={`translateX(${-scroll}px)`}>
      {#if focused && a !== b}
        <span class="sel" style:left={`${a}ch`} style:width={`${b - a}ch`}></span>
      {/if}
      {#if value}
        <span class="text">{value}</span>
      {:else}
        <span class="ph">{placeholder}</span>
      {/if}
      {#if focused && !disabled}
        {#key kick}
          <span class="caret" class:solid={a !== b} style:left={`${caretAt}ch`}>{under}</span>
        {/key}
      {/if}
    </div>
  </div>
  <input
    bind:this={input}
    bind:value
    {disabled}
    id="addr"
    name="ryudit-addr"
    type="text"
    inputmode="email"
    autocapitalize="off"
    autocomplete="off"
    data-bwignore="true"
    data-1p-ignore="true"
    data-lpignore="true"
    data-form-type="other"
    aria-label="Email"
    aria-invalid={invalid ? "true" : undefined}
    required
    maxlength="254"
    spellcheck="false"
    oninput={changed}
    onkeydown={() => requestAnimationFrame(sync)}
    onkeyup={sync}
    onselect={sync}
    onscroll={sync}
    onpointerup={sync}
    onfocus={() => { focused = true; kick++; requestAnimationFrame(sync); }}
    onblur={() => (focused = false)}
  />
</div>

<style>
  /* a terminal prompt: no box, a line under it, a ❯ before it */
  .field {
    position: relative;
    /* monospaced whatever the layout's type: the caret and the selection are placed by character */
    font-family: ui-monospace, "SF Mono", SFMono-Regular, Menlo, Consolas, "Liberation Mono", monospace;
    flex: 1;
    min-width: 0;
    border-bottom: 1px solid var(--line);
    background: var(--field-bg, transparent);
    cursor: text;
    transition: border-color 0.2s;
  }
  .field:hover { border-color: color-mix(in oklab, var(--line) 60%, var(--line-hi)); }
  .field.focused { border-color: var(--line-hi); }
  .field.invalid { border-color: var(--bad); }
  .field.disabled { opacity: 0.55; }
  .prompt {
    position: absolute;
    left: 0.15rem;
    top: 50%;
    translate: 0 -50%;
    color: var(--prompt);
    transition: color 0.2s, translate 0.2s cubic-bezier(.2,.8,.2,1);
    pointer-events: none;
  }
  .field.focused .prompt { color: var(--prompt-hi); translate: 2px -50%; }
  .field.invalid .prompt { color: var(--bad); }

  /* the input and the view share one box: same font, same padding, so a click lands on the character drawn there */
  input,
  .view {
    font: inherit;
    padding: var(--field-py, 0.6rem) var(--field-px, 0.2rem) var(--field-py, 0.6rem) calc(var(--field-px, 0.2rem) + 1.3rem);
    text-align: left;
  }
  .bare input, .bare .view { padding-left: var(--field-px, 0.2rem); }
  input { white-space: pre; }
  input {
    position: relative;
    z-index: 1;
    display: block;
    width: 100%;
    margin: 0;
    border: 0;
    outline: 0;
    background: transparent;
    color: transparent;
    caret-color: transparent;
    cursor: text;
  }
  input::selection { background: transparent; color: transparent; }
  input::placeholder { color: transparent; }
  /* autofill would paint its own background and text over the view */
  input:-webkit-autofill,
  input:-webkit-autofill:hover,
  input:-webkit-autofill:focus {
    -webkit-text-fill-color: transparent;
    transition: background-color 600000s 0s;
  }

  .view {
    position: absolute;
    inset: 0;
    overflow: hidden;
    pointer-events: none;
  }
  /* flex: the template's white space between the spans is not laid out (it would push the text a space to the right) */
  .line { position: relative; display: flex; height: 100%; }
  .text, .ph { white-space: pre; }
  .text { color: var(--fg); }
  .ph { color: var(--dim); }
  .sel {
    position: absolute;
    top: -0.1em;
    bottom: -0.1em;
    background: var(--sel);
  }

  /* the caret: a block over the next character, which shows through it in the block's ink */
  .caret {
    position: absolute;
    top: -0.12em;
    bottom: -0.12em;
    width: 1ch;
    color: var(--caret-ink);
    background: var(--caret);
    text-align: center;
    transform-origin: 50% 100%;
    /* after a change: a short jump, then solid for a moment, then the blink */
    animation: kick 0.14s ease-out, blink 1.05s steps(1) 0.6s infinite;
  }
  .caret.solid { animation: kick 0.14s ease-out; }
  @keyframes kick {
    from { transform: scaleY(1.22); filter: brightness(1.5); }
    to { transform: none; filter: none; }
  }
  @keyframes blink { 50% { opacity: 0; } }
  @media (prefers-reduced-motion: reduce) {
    .caret, .caret.solid { animation: none; }
  }
</style>
