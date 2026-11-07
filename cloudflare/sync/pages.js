// The two pages of the server: the waitlist ("/") and the administrator's ("/admin"). Plain HTML and a little
// script; what a person typed (an e-mail, a name) is put on a page only as text, never as markup.
import { landingHTML } from "./landing.js";
const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);

function nonce() {
  return btoa(String.fromCharCode(...crypto.getRandomValues(new Uint8Array(16))));
}
function page(html, n, extraScript = "", noStore = false) {
  const csp = [
    "default-src 'none'",
    `script-src 'nonce-${n}' ${extraScript}`.trim(),
    "style-src 'unsafe-inline'",
    "connect-src 'self'",
    extraScript ? "frame-src https://challenges.cloudflare.com" : "frame-src 'none'",
    "base-uri 'none'",
    "form-action 'none'",
    "frame-ancestors 'none'",
  ].join("; ");
  return new Response(html, {
    headers: {
      "content-type": "text/html; charset=utf-8",
      "content-security-policy": csp,
      "x-frame-options": "DENY",
      "referrer-policy": "no-referrer",
      "x-content-type-options": "nosniff",
      ...(noStore ? { "cache-control": "no-store" } : {}),
    },
  });
}

const BASE_CSS = `
:root{--bg:#fbfaf7;--fg:#1d1d1f;--mut:#66666b;--line:#e4e1da;--card:#fff;--acc:#1f3a5f;--accfg:#fff;--bad:#c20013;--ok:#1f7534}
@media (prefers-color-scheme:dark){:root{--bg:#0f1115;--fg:#ecebe8;--mut:#9a9aa1;--line:#262932;--card:#171a20;--acc:#dbe4f2;--accfg:#0f1115;--bad:#ff6b6b;--ok:#6fcf8a}}
*{box-sizing:border-box}html{color-scheme:light dark}
body{margin:0;background:var(--bg);color:var(--fg);font:16px/1.5 ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
main{max-width:30rem;margin:0 auto;padding:12vh 20px 48px}
h1{font-size:2rem;letter-spacing:-.02em;margin:0 0 .4rem}
p{margin:.4rem 0 1.2rem;color:var(--mut)}
label{display:block;font-size:.85rem;color:var(--mut);margin:.9rem 0 .25rem}
input{width:100%;font:inherit;color:inherit;background:var(--card);border:1px solid var(--line);border-radius:10px;padding:.65rem .8rem}
input:focus-visible,button:focus-visible{outline:2px solid var(--acc);outline-offset:2px}
button{font:inherit;cursor:pointer;border:0;border-radius:10px;padding:.7rem 1.1rem;background:var(--acc);color:var(--accfg);margin-top:1.1rem}
button[disabled]{opacity:.55;cursor:default}
.msg{margin-top:1rem;min-height:1.5em}.bad{color:var(--bad)}.ok{color:var(--ok)}
small{display:block;margin-top:2rem;color:var(--mut);font-size:.8rem}
`;

// The entry screen ("/"): see landing.js.
export function waitlistPage(env) {
  const n = nonce();
  const site = env.TURNSTILE_SITEKEY || "";
  const contact = env.CONTACT_EMAIL ? ` To have it deleted, write to ${esc(env.CONTACT_EMAIL)}.` : "";
  return page(landingHTML(n, site, contact, esc), n, site ? "https://challenges.cloudflare.com" : "");
}

export function adminPage() {
  const n = nonce();
  const html = `<!doctype html><html lang="tr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex"><title>Ryudit — yönetim</title><style>${BASE_CSS}
main{max-width:64rem;padding-top:6vh}
nav{display:flex;gap:.4rem;flex-wrap:wrap;margin:1rem 0}
nav button{margin:0;background:var(--card);color:var(--fg);border:1px solid var(--line)}
nav button[aria-pressed=true]{background:var(--acc);color:var(--accfg);border-color:var(--acc)}
table{width:100%;border-collapse:collapse;font-size:.92rem}
th,td{text-align:left;padding:.5rem .6rem;border-bottom:1px solid var(--line);vertical-align:middle}
th{color:var(--mut);font-weight:500}
td button{margin:0 .25rem 0 0;padding:.35rem .7rem;font-size:.85rem}
td button.alt{background:var(--card);color:var(--fg);border:1px solid var(--line)}
.code{font:600 1.4rem ui-monospace,SFMono-Regular,Menlo,monospace;letter-spacing:.06em}
.banner{border:1px solid var(--line);background:var(--card);border-radius:12px;padding:1rem;margin:1rem 0}
.row{display:flex;gap:.6rem}.row input{flex:1}.row button{margin-top:0}
</style></head><body><main>
<h1>Yönetim</h1>
<div id="login"><label for="tok">Yönetici anahtarı</label>
<div class="row"><input id="tok" type="password" autocomplete="off"><button id="enter">Gir</button></div></div>
<div id="app" hidden>
<nav id="tabs"></nav><div id="banner"></div><div id="view"></div>
<p><button id="out" class="alt" style="background:var(--card);color:var(--fg);border:1px solid var(--line)">Çıkış</button></p>
</div>
<div id="msg" class="msg bad" role="alert"></div>
</main>
<script nonce="${n}">
const $=(i)=>document.getElementById(i);
let tok=sessionStorage.getItem("adm")||"",tab="pending";
const TABS=[["pending","Bekleyenler"],["approved","Onaylananlar"],["rejected","Reddedilenler"],["users","Hesaplar"],["audit","Kayıt"]];
async function api(path,method="GET",body){
  const r=await fetch("/api/admin/"+path,{method,headers:{authorization:"Bearer "+tok,"content-type":"application/json"},body:body?JSON.stringify(body):undefined});
  const j=await r.json().catch(()=>({}));
  if(r.status===401){sessionStorage.removeItem("adm");tok="";show();throw new Error("anahtar yanlış");}
  if(!r.ok)throw new Error(j.error||("hata "+r.status));
  return j;
}
const el=(t,props={},...kids)=>{const e=document.createElement(t);Object.assign(e,props);for(const k of kids)e.append(k);return e;};
const date=(s)=>s?new Date(s*1000).toLocaleString("tr-TR",{dateStyle:"short",timeStyle:"short"}):"";
const mb=(n)=>(n/1048576).toFixed(1)+" MB";
function btn(label,fn,alt){const b=el("button",{textContent:label,className:alt?"alt":""});b.onclick=async()=>{b.disabled=true;try{await fn();}catch(e){$("msg").textContent=e.message;}await load();};return b;}
function table(head,rows){
  const t=el("table"),h=el("tr");for(const c of head)h.append(el("th",{textContent:c}));t.append(h);
  for(const r of rows){const tr=el("tr");for(const c of r)tr.append(el("td",{},c));t.append(tr);}return t;
}
function showCode(email,j){
  const b=$("banner");b.replaceChildren();
  const box=el("div",{className:"banner"});
  box.append(el("div",{textContent:email}),el("div",{className:"code",textContent:j.code}),
    el("div",{className:j.emailed?"ok":"bad",textContent:j.emailed?"E-posta gönderildi.":"E-posta gitmedi ("+(j.mailError||"?")+"): kodu elle ilet. Bu kod bir daha gösterilmez."}));
  box.append(btn("Kodu kopyala",async()=>{await navigator.clipboard.writeText(j.code);},true));
  b.append(box);
}
async function load(){
  $("msg").textContent="";const v=$("view");v.replaceChildren();
  const tabs=$("tabs");tabs.replaceChildren();
  for(const [k,l] of TABS){const b=el("button",{textContent:l});b.setAttribute("aria-pressed",String(k===tab));b.onclick=()=>{tab=k;$("banner").replaceChildren();load();};tabs.append(b);}
  try{
    if(tab==="users"){
      const us=await api("users");
      v.append(table(["Kullanıcı","Durum","Alan","Cihaz","Kayıt",""],us.map((u)=>[u.username,u.status,mb(u.bytes)+" / "+mb(u.quota),String(u.devices),date(u.created_at),
        el("span",{},u.status==="active"?btn("Askıya al",()=>api("users/"+u.id+"/disable","POST"),true):btn("Aç",()=>api("users/"+u.id+"/enable","POST"),true),
          btn("Sil",async()=>{if(confirm(u.username+" ve tüm verisi silinsin mi? Geri alınamaz."))await api("users/"+u.id+"/delete","POST");},true))])));
    }else if(tab==="audit"){
      const a=await api("audit");v.append(table(["Zaman","İşlem"],a.map((x)=>[date(x.at),x.what])));
    }else{
      const w=await api("waitlist?status="+tab);
      if(!w.length)v.append(el("p",{textContent:"Kimse yok."}));
      else v.append(table(["E-posta","Ad","Yazıldı","Durum",""],w.map((x)=>[x.email,x.name,date(x.created_at),
        x.status==="approved"?(x.used_at?"hesap açtı":x.invite_until?"kod "+date(x.invite_until)+" tarihine kadar":"kodun süresi doldu"):x.status==="pending"?"bekliyor":"reddedildi",
        el("span",{},
          x.status==="pending"?btn("Onayla",async()=>showCode(x.email,await api("waitlist/"+x.id+"/approve","POST"))):
          x.status==="approved"&&!x.used_at?btn("Yeni kod gönder",async()=>showCode(x.email,await api("waitlist/"+x.id+"/resend","POST")),true):
          x.status==="rejected"?btn("Onayla",async()=>showCode(x.email,await api("waitlist/"+x.id+"/approve","POST")),true):"",
          x.status!=="rejected"&&!x.used_at?btn("Reddet",()=>api("waitlist/"+x.id+"/reject","POST"),true):"")])));
    }
  }catch(e){$("msg").textContent=e.message;}
}
function show(){$("login").hidden=!!tok;$("app").hidden=!tok;if(tok)load();}
$("enter").onclick=()=>{tok=$("tok").value.trim();if(tok){sessionStorage.setItem("adm",tok);$("tok").value="";show();}};
$("tok").addEventListener("keydown",(e)=>{if(e.key==="Enter")$("enter").click();});
$("out").onclick=()=>{sessionStorage.removeItem("adm");tok="";$("banner").replaceChildren();show();};
show();
</script></body></html>`;
  return page(html, n, "", true);
}
