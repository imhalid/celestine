// Touch for the web version and the Android app. MyGo's canvas takes a finger as a mouse: a finger dragged
// would select text or drag a row, and the browser's own scrolling is off (touch-action: none). This script
// takes the touches first, in the capture phase on the window, and gives the canvas what a hand means:
//
//   a finger moved        scrolling, as a touchpad's wheel would, both ways, and gliding on after it lets go
//   a tap                 a click where it was (two taps close together are a double click: MyGo counts them)
//   a long press          a right click: the menu of a row, of the text
//   a second finger       nothing: a pinch would zoom the page
//
// What it sends is an ordinary mouse's events, made here (not trusted), which it lets through.
(function () {
  "use strict";
  const SLOP = 9; // px a finger moves before it is scrolling, not tapping
  const LONG = 480; // ms of a still finger that is a long press
  const FRICTION = 0.94; // of the glide, each 16 ms

  // the events made here have a pointer of their own, which the canvas cannot capture: it would throw
  for (const name of ["setPointerCapture", "releasePointerCapture"]) {
    const orig = Element.prototype[name];
    Element.prototype[name] = function (id) {
      try {
        return orig.call(this, id);
      } catch (e) {}
    };
  }

  const canvas = () => document.querySelector(".mygo-window canvas") || document.querySelector("canvas");
  const send = (type, x, y, button, buttons) => {
    const c = canvas();
    if (!c) return;
    c.dispatchEvent(new PointerEvent(type, { clientX: x, clientY: y, button, buttons, pointerId: 77, pointerType: "mouse", isPrimary: true, bubbles: true, cancelable: true }));
  };
  const wheel = (dx, dy, x, y) => {
    const c = canvas();
    if (!c || (dx === 0 && dy === 0)) return;
    c.dispatchEvent(new WheelEvent("wheel", { deltaX: dx, deltaY: dy, deltaMode: 0, clientX: x, clientY: y, bubbles: true, cancelable: true }));
  };

  let t = null; // the finger down: { id, x0, y0, x, y, mode: "wait" | "pan" | "held", timer, vx, vy, at }
  let glide = 0;
  const stopGlide = () => {
    if (glide) cancelAnimationFrame(glide);
    glide = 0;
  };
  const touch = (e) => e.isTrusted && e.pointerType === "touch";
  const take = (e) => {
    e.stopImmediatePropagation();
    if (e.cancelable) e.preventDefault();
  };

  window.addEventListener("pointerdown", (e) => {
    if (!touch(e)) return;
    take(e);
    if (t) return; // a second finger
    stopGlide();
    t = { id: e.pointerId, x0: e.clientX, y0: e.clientY, x: e.clientX, y: e.clientY, mode: "wait", vx: 0, vy: 0, at: performance.now() };
    const mine = t;
    t.timer = setTimeout(() => {
      if (t !== mine || t.mode !== "wait") return;
      t.mode = "held"; // a long press: a right click where it is
      send("pointermove", t.x0, t.y0, 0, 0);
      send("pointerdown", t.x0, t.y0, 2, 2);
      send("pointerup", t.x0, t.y0, 2, 0);
      if (navigator.vibrate) navigator.vibrate(12);
    }, LONG);
  }, true);

  window.addEventListener("pointermove", (e) => {
    if (!touch(e)) return;
    take(e);
    if (!t || e.pointerId !== t.id) return;
    const x = e.clientX, y = e.clientY;
    if (t.mode === "wait" && Math.hypot(x - t.x0, y - t.y0) > SLOP) {
      t.mode = "pan";
      clearTimeout(t.timer);
    }
    if (t.mode === "pan") {
      const now = performance.now(), dt = Math.max(1, now - t.at);
      const dx = t.x - x, dy = t.y - y; // the content follows the finger
      t.vx = 0.7 * (dx / dt) + 0.3 * t.vx;
      t.vy = 0.7 * (dy / dt) + 0.3 * t.vy;
      t.at = now;
      wheel(dx, dy, x, y);
    }
    t.x = x;
    t.y = y;
  }, true);

  const end = (e, cancel) => {
    if (!touch(e)) return;
    take(e);
    if (!t || e.pointerId !== t.id) return;
    clearTimeout(t.timer);
    const was = t;
    t = null;
    if (was.mode === "wait" && !cancel) { // a tap
      send("pointermove", was.x0, was.y0, 0, 0);
      send("pointerdown", was.x0, was.y0, 0, 1);
      send("pointerup", was.x0, was.y0, 0, 0);
      // the text has the keyboard already (the textarea asks for text): the focus is given again now, in the
      // tap itself, or a phone (Samsung's WebView) does not show the keyboard for a focus it already had
      const ta = textarea();
      if (ta && askText(ta)) refocus(ta);
      keyboard();
      return;
    }
    if (was.mode === "pan" && performance.now() - was.at < 80) { // let go while moving: it glides on
      let vx = was.vx * 16, vy = was.vy * 16;
      const x = was.x, y = was.y;
      const step = () => {
        vx *= FRICTION;
        vy *= FRICTION;
        if (Math.abs(vx) < 0.4 && Math.abs(vy) < 0.4) {
          glide = 0;
          return;
        }
        wheel(vx, vy, x, y);
        glide = requestAnimationFrame(step);
      };
      glide = requestAnimationFrame(step);
    }
  };
  // The keyboard. MyGo's textarea, out of sight, has the keyboard; it says "inputmode=none" until a text
  // input of the content has the focus, which the content knows a frame after the tap. A phone shows its
  // keyboard for a focus given while a tap still counts as the user's (a few seconds): once the textarea
  // asks for text after a tap, it is given the focus again, then.
  const textarea = () => document.querySelector(".mygo-window textarea") || document.querySelector("textarea");
  const askText = (ta) => (ta.getAttribute("inputmode") || "text") !== "none";
  const refocus = (ta) => {
    ta.blur();
    ta.focus({ preventScroll: true });
    // Chrome's own way, where there is one: it shows the keyboard while a tap still counts as the user's
    if (navigator.virtualKeyboard && navigator.virtualKeyboard.show) {
      try {
        navigator.virtualKeyboard.show();
      } catch (e) {}
    }
  };
  const keyboard = () => {
    const at = performance.now();
    const before = textarea() && askText(textarea());
    const look = () => {
      const ta = textarea();
      if (ta && document.activeElement === ta && askText(ta)) {
        if (!before) refocus(ta); // it asks for text only now: the text was tapped into from elsewhere
        return;
      }
      if (performance.now() - at < 700) setTimeout(look, 40);
    };
    setTimeout(look, 40);
  };
  window.addEventListener("pointerup", (e) => end(e, false), true);
  window.addEventListener("pointercancel", (e) => end(e, true), true);
  // The touch events themselves are left alone: the canvas's touch-action: none keeps the page from
  // scrolling and zooming, and a touchstart canceled would also cancel the tap the browser makes of it,
  // which is what shows a phone's keyboard for the text that has the focus.
})();
