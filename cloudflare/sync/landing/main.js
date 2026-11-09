import "./app.css";
import { mount } from "svelte";
import App from "./App.svelte";

// a scheme chosen with the toggle is set before anything is drawn, so the page does not flash the system's first
try { const s = localStorage.getItem("ryudit-scheme"); if (s === "light" || s === "dark") document.documentElement.dataset.scheme = s; } catch {}

const target = document.getElementById("app");
mount(App, { target, props: { contact: target.dataset.contact || "", download: target.dataset.download || "" } });
