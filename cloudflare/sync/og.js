// The Open Graph picture, the card a link to the site shows where it is pasted (chat, social, search): /og.jpg, a 1200 x 630 JPEG
// (og.bin) of the two glowing crystals under a night sky, painted with an image model in the watercolor of the site's other
// pictures and compressed with squoosh. ogTags are the <meta> tags of a page that names it.
import OG from "./og.bin";

// SITE is the address the card's picture is asked for by (an og:image must be absolute); deploy.sh's DOMAIN is the same.
export const SITE = "https://celestine.halid.dev";

// the picture's address suffix changes when the picture does, so no chat or browser keeps the old card for its day
const V = (OG.byteLength || OG.length || 0).toString(36);

// ogImage answers GET /og.jpg.
export function ogImage() {
  return new Response(OG, {
    headers: { "content-type": "image/jpeg", "cache-control": "public, max-age=86400", "x-content-type-options": "nosniff" },
  });
}

// ogTags is the Open Graph and Twitter card tags of a page: its title, its sentence and its path ("/" or "/changelog").
export function ogTags(title, description, path) {
  const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);
  const image = `${SITE}/og.jpg?v=${V}`;
  const alt = "Two glowing blue crystal capsules in a meadow under a night sky of stars";
  return [
    ["property", "og:type", "website"],
    ["property", "og:site_name", "Celestine"],
    ["property", "og:title", title],
    ["property", "og:description", description],
    ["property", "og:url", SITE + path],
    ["property", "og:image", image],
    ["property", "og:image:type", "image/jpeg"],
    ["property", "og:image:width", "1200"],
    ["property", "og:image:height", "630"],
    ["property", "og:image:alt", alt],
    ["name", "twitter:card", "summary_large_image"],
    ["name", "twitter:title", title],
    ["name", "twitter:description", description],
    ["name", "twitter:image", image],
    ["name", "twitter:image:alt", alt],
  ].map(([k, n, v]) => `<meta ${k}="${n}" content="${esc(v)}">`).join("");
}
