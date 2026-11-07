<script>
  // The waitlist form: the field, a button, and a line for what happened. The page places it and dresses it (App.svelte);
  // a button outside the form (form="…") can stand in for its own (button={false}).
  import EmailField from "./EmailField.svelte";

  let {
    id = "signup", label = "Join the waitlist", prompt = "❯", placeholder = "you@example.com", kbd = true, button = true,
    done = "You're on the list. Once you're approved, an invite code will reach your inbox.",
    email = $bindable(""), phase = $bindable("idle"),
  } = $props();

  const reduce = matchMedia("(prefers-reduced-motion: reduce)").matches;
  let error = $state("");
  let frame = $state(), input = $state();

  // what was wrong is forgotten as soon as the address is changed
  $effect(() => { void email; error = ""; });

  function fail(text) {
    error = text;
    // a short shake of the field: the answer is "no", not a crash
    if (frame && !reduce) frame.animate([{ transform: "translateX(0)" }, { transform: "translateX(-6px)" }, { transform: "translateX(5px)" }, { transform: "translateX(-3px)" }, { transform: "translateX(0)" }], { duration: 320, easing: "ease-out" });
  }

  async function submit(e) {
    e.preventDefault();
    if (phase === "sending") return;
    error = "";
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) { fail("Enter a valid email address."); input?.focus(); return; }
    phase = "sending";
    try {
      const t = document.querySelector("[name=cf-turnstile-response]");
      const r = await fetch("/api/waitlist", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ email: email.trim(), turnstile: t ? t.value : "" }) });
      const j = await r.json().catch(() => ({}));
      if (r.ok) { phase = "done"; return; }
      if (window.turnstile) window.turnstile.reset();
      phase = "idle";
      fail(j.error || "That didn't work. Try again later.");
    } catch {
      phase = "idle";
      fail("Couldn't connect. Try again.");
    }
  }
</script>

<div class="signup">
  {#if phase === "done"}
    <div class="done" role="status">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="square" aria-hidden="true"><path d="M4 12.5l5 5L20 6.5" /></svg>
      {done}
    </div>
  {:else}
    <form {id} class="row" novalidate autocomplete="off" data-bwignore="true" data-1p-ignore="true" data-lpignore="true" onsubmit={submit}>
      <EmailField bind:value={email} bind:frame bind:input {placeholder} {prompt} invalid={!!error} disabled={phase === "sending"} />
      {#if button}
        <button type="submit" class="go" disabled={phase === "sending"}>{#if kbd}<kbd aria-hidden="true">↵</kbd>{/if}{phase === "sending" ? "Sending…" : label}</button>
      {/if}
    </form>
  {/if}
  <div class="msg" class:bad={!!error} role="status" aria-live="polite">{error}</div>
</div>
