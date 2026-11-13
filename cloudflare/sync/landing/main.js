import "./app.css";
import { mount } from "svelte";
import App from "./App.svelte";

// the page is dark unless light was chosen with the toggle (landing.js's head script has set it already; this is the same
// again, for when the page is run without it)
try { document.documentElement.dataset.scheme = localStorage.getItem("celestine-scheme") === "light" ? "light" : "dark"; } catch { document.documentElement.dataset.scheme = "dark"; }

// the pictures of both schemes and of the footer are fetched in the background when the page is idle, so each is there
// when the scheme is switched (the view transition would fade to a blank) or the footer is scrolled to
const fetchPictures = () => {
  const css = getComputedStyle(document.documentElement);
  for (const v of ["--hero", "--hero-dark", "--foot", "--foot-dark"]) {
    const m = /url\(["']?([^"')]+)/.exec(css.getPropertyValue(v));
    if (m) new Image().src = m[1];
  }
};
const whenIdle = () => (window.requestIdleCallback || ((f) => setTimeout(f, 600)))(fetchPictures);
if (document.readyState === "complete") whenIdle(); else addEventListener("load", whenIdle);

const target = document.getElementById("app");
mount(App, { target, props: { contact: target.dataset.contact || "", download: target.dataset.download || "" } });
