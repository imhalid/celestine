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

// The entry screen ("/"): see landing.js.
export function waitlistPage(env) {
  const n = nonce();
  const site = env.TURNSTILE_SITEKEY || "";
  const contact = env.CONTACT_EMAIL ? ` To have it deleted, write to ${esc(env.CONTACT_EMAIL)}.` : "";
  return page(landingHTML(n, site, contact, esc), n, site ? "https://challenges.cloudflare.com" : "");
}

// The administrator's page looks like an airport's departure board: black, amber letters, square everything; the
// rows' letters flap into place as they come, and the invite code is shown on flap tiles.
const BOARD_CSS = `
:root{--bg:#08090a;--panel:#0f1011;--row:#111214;--row2:#0c0d0e;--line:#1d1f22;--line2:#2a2d31;--amb:#ffb81c;--amb-hi:#ffd36a;--amb-lo:#a8740a;
  --fg:#ece7da;--mut:#878378;--bad:#ff5a47;--ok:#36d67a;--tile:#17181a;--tile-hi:#202225}
*{box-sizing:border-box;border-radius:0}html{color-scheme:dark}
body{margin:0;background:var(--bg);color:var(--fg);font:14px/1.45 "SF Mono",ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;letter-spacing:.04em;
  background-image:radial-gradient(ellipse at 50% -10%,rgba(255,184,28,.07),transparent 55%),repeating-linear-gradient(0deg,rgba(255,255,255,.016) 0 1px,transparent 1px 3px)}
::selection{background:var(--amb);color:#000}
main{max-width:72rem;margin:0 auto;padding:24px 16px 48px}
.top{display:flex;align-items:stretch;border:1px solid var(--amb-lo);margin-bottom:20px;background:#0b0b0c}
.brand{background:linear-gradient(var(--amb-hi),var(--amb));color:#120d00;font-weight:800;padding:10px 16px;display:flex;align-items:center;gap:10px;letter-spacing:.2em;
  box-shadow:inset 0 -2px 0 var(--amb-lo)}
.brand i{width:10px;height:10px;background:#120d00;display:inline-block}
.title{flex:1;display:flex;align-items:center;padding:0 16px;color:var(--amb);letter-spacing:.22em;font-weight:600;text-transform:uppercase;min-width:0;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.clock{display:flex;gap:3px;align-items:center;padding:6px 10px;border-left:1px solid var(--amb-lo)}
.tile{display:inline-block;position:relative;min-width:1.05em;text-align:center;background:linear-gradient(var(--tile-hi) 0 50%,var(--tile) 50%);color:var(--fg);padding:.12em .14em;font-weight:700;
  box-shadow:inset 0 1px 0 rgba(255,255,255,.07),0 1px 0 #000}
.tile::after{content:"";position:absolute;left:0;right:0;top:50%;height:1px;background:#000}
.clock .tile{font-size:1.05rem}.clock .sep{color:var(--amb);font-weight:700;animation:blink 2s steps(1) infinite}
nav{display:flex;flex-wrap:wrap;border:1px solid var(--line);margin-bottom:16px;background:var(--panel)}
nav button{flex:1 1 auto;margin:0;background:transparent;color:var(--mut);border:0;border-right:1px solid var(--line);box-shadow:none;padding:13px 14px;font-size:.76rem;letter-spacing:.16em;
  display:flex;align-items:center;justify-content:center;gap:10px;transition:color .15s,background .15s}
nav button:last-child{border-right:0}
nav button .no{font-size:.66rem;color:#4c4f55;font-weight:700;transition:color .15s}
nav button:hover{color:var(--fg);background:rgba(255,255,255,.025);transform:none}
nav button:hover .no{color:var(--amb-lo)}
nav button[aria-pressed=true]{color:var(--amb);background:rgba(255,184,28,.07);box-shadow:inset 0 -2px 0 var(--amb)}
nav button[aria-pressed=true] .no{color:var(--amb)}
button{font:inherit;cursor:pointer;display:inline-flex;align-items:center;justify-content:center;gap:8px;height:34px;padding:0 14px;text-transform:uppercase;letter-spacing:.14em;font-size:.72rem;font-weight:700;white-space:nowrap;
  border:1px solid var(--amb-lo);background:linear-gradient(var(--amb-hi),var(--amb));color:#120d00;
  box-shadow:inset 0 1px 0 rgba(255,255,255,.45),0 2px 0 #000;transition:filter .12s,background .12s,color .12s,border-color .12s,transform .06s}
button:hover{filter:brightness(1.08)}
button:active{transform:translateY(1px);box-shadow:inset 0 1px 0 rgba(255,255,255,.3),0 1px 0 #000}
button.alt{background:linear-gradient(var(--tile-hi),var(--tile));color:var(--amb);border-color:var(--line2);box-shadow:inset 0 1px 0 rgba(255,255,255,.05),0 2px 0 #000}
button.alt:hover{border-color:var(--amb);color:var(--amb-hi);filter:none}
button.bad{background:linear-gradient(var(--tile-hi),var(--tile));color:var(--bad);border-color:#3a1f1c;box-shadow:inset 0 1px 0 rgba(255,255,255,.05),0 2px 0 #000}
button.bad:hover{background:var(--bad);border-color:var(--bad);color:#160200;filter:none}
button[disabled]{opacity:.4;cursor:default;transform:none}
button:focus-visible,input:focus-visible{outline:2px solid var(--amb);outline-offset:2px}
input{font:inherit;color:var(--fg);background:#050506;border:1px solid var(--line2);height:34px;padding:0 12px;min-width:0;letter-spacing:.04em;box-shadow:inset 0 2px 0 rgba(0,0,0,.6);transition:border-color .15s}
input::placeholder{color:#55524b;text-transform:uppercase;letter-spacing:.12em}
input:hover{border-color:#3a3d42}input:focus{border-color:var(--amb)}
.panel{border:1px solid var(--line);background:var(--panel);margin:0 0 16px}
.ph{display:flex;justify-content:space-between;gap:12px;padding:9px 12px;border-bottom:1px solid var(--line);background:#0b0c0d;color:var(--amb);font-size:.7rem;letter-spacing:.2em;text-transform:uppercase}
.ph span:last-child{color:var(--mut)}
.pb{padding:14px 12px}
.row{display:flex;gap:8px;flex-wrap:wrap}.row input{flex:1 1 12rem}
label{display:block;color:var(--mut);font-size:.7rem;letter-spacing:.2em;text-transform:uppercase;margin-bottom:8px}
.wrap{overflow-x:auto;border:1px solid var(--line)}
table{width:100%;border-collapse:collapse;min-width:44rem}
th{background:#060607;color:var(--amb);text-align:left;font-weight:600;font-size:.68rem;letter-spacing:.2em;text-transform:uppercase;padding:10px 12px;border-bottom:1px solid var(--amb-lo);white-space:nowrap}
td{padding:9px 12px;border-bottom:1px solid var(--line);vertical-align:middle;white-space:nowrap}
tr:nth-child(odd) td{background:var(--row)}tr:nth-child(even) td{background:var(--row2)}
tr:hover td{background:#16140e}
td:last-child{text-align:right}td button{margin-left:6px;height:28px;padding:0 10px;font-size:.66rem}
.st{display:inline-flex;align-items:center;gap:8px;height:24px;padding:0 9px 0 7px;font-weight:700;letter-spacing:.14em;text-transform:uppercase;font-size:.7rem;
  background:color-mix(in srgb,currentColor 10%,transparent);box-shadow:inset 2px 0 0 currentColor}
.st::before{content:"";width:6px;height:6px;background:currentColor;box-shadow:0 0 6px currentColor}
.st.wait{color:var(--amb)}.st.wait::before{animation:blink 1.1s steps(1) infinite}
.st.ok{color:var(--ok)}.st.bad{color:var(--bad)}.st.off{color:var(--mut)}.st.off::before{box-shadow:none}
@keyframes blink{50%{opacity:.15}}
.code{display:flex;gap:4px;flex-wrap:wrap;margin:10px 0 12px}.code .tile{font-size:1.9rem;padding:.1em .18em;color:var(--amb);text-shadow:0 0 12px rgba(255,184,28,.35)}
.note{margin:0 0 12px}.note.ok{color:var(--ok)}.note.bad{color:var(--bad)}
.empty{padding:32px 12px;text-align:center;color:var(--mut);letter-spacing:.2em;text-transform:uppercase;border:1px dashed var(--line2);background:var(--row2)}
.bar{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap;margin-top:16px;padding-top:12px;border-top:1px solid var(--line);color:var(--mut);font-size:.7rem;letter-spacing:.16em;text-transform:uppercase}
.msg{min-height:1.5em;margin-top:12px;color:var(--bad);letter-spacing:.08em}.msg:not(:empty)::before{content:"! "}
.gate{max-width:30rem;margin:12vh auto 0}
.cursor{display:inline-block;width:.6em;height:1em;background:var(--amb);vertical-align:-2px;animation:blink 1s steps(1) infinite}
@media (prefers-reduced-motion:reduce){*{animation:none!important;transition:none!important}}
@media (max-width:640px){.title{display:none}.clock{margin-left:auto}nav button{flex:1 1 45%;border-bottom:1px solid var(--line)}}
`;

export function adminPage() {
  const n = nonce();
  const html = `<!doctype html><html lang="tr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex"><title>Ryudit — yönetim</title><style>${BOARD_CSS}</style></head><body><main>
<header class="top"><div class="brand"><i></i>RYUDIT</div><div class="title">Yönetim · Kontrol masası</div><div class="clock" id="clock" aria-label="saat"></div></header>
<section id="login" class="panel gate"><div class="ph"><span>Erişim</span><span>Yetki gerekli</span></div>
<div class="pb"><label for="tok">Yönetici anahtarı <span class="cursor"></span></label>
<div class="row"><input id="tok" type="password" autocomplete="off"><button id="enter">Gir ▸</button></div></div></section>
<div id="app" hidden>
<nav id="tabs"></nav><div id="banner"></div><div id="view"></div>
<div class="bar"><span id="stat">—</span><span><button id="out" class="alt">Çıkış</button></span></div>
</div>
<div id="msg" class="msg" role="alert"></div>
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
// flap turns the letters of a node over until each settles on its own, one after another, as a split-flap board does.
const FL="ABCDEFGHIJKLMNOPRSTUVYZ0123456789@.-";
const still=matchMedia("(prefers-reduced-motion: reduce)").matches;
function flap(node,text,wait=0){
  node.textContent=text;if(still||!text)return;
  const cs=[...text];let f=-wait;
  const t=setInterval(()=>{f++;if(f<0)return;
    node.textContent=cs.map((c,i)=>c===" "||f>4+i*.7?c:FL[Math.random()*FL.length|0]).join("");
    if(f>4+cs.length*.7){clearInterval(t);node.textContent=text;}},38);
}
function tiles(text){const d=el("div",{className:"code"});[...text].forEach((c,i)=>{const s=el("span",{className:"tile"});d.append(s);flap(s,c,i*2);});return d;}
function btn(label,fn,kind){const b=el("button",{textContent:label,className:kind===true?"alt":kind||""});b.onclick=async()=>{b.disabled=true;try{await fn();}catch(e){$("msg").textContent=e.message;}await load();};return b;}
function status(cls,text){return el("span",{className:"st "+cls,textContent:text});}
function table(head,rows){
  const w=el("div",{className:"wrap"}),t=el("table"),h=el("tr");for(const c of head)h.append(el("th",{textContent:c}));t.append(h);
  rows.forEach((r,ri)=>{const tr=el("tr");for(const c of r){const td=el("td");if(typeof c==="string"){const s=el("span");flap(s,c,ri*2);td.append(s);}else td.append(c);tr.append(td);}t.append(tr);});
  w.append(t);return w;
}
function panel(title,right,...kids){const p=el("div",{className:"panel"}),h=el("div",{className:"ph"},el("span",{textContent:title}),el("span",{textContent:right||""})),b=el("div",{className:"pb"},...kids);p.append(h,b);return p;}
function showCode(email,j){
  const b=$("banner");b.replaceChildren();
  b.append(panel("Davet kodu",email,tiles(j.code),
    el("p",{className:"note "+(j.emailed?"ok":"bad"),textContent:j.emailed?"E-posta gönderildi.":"E-posta gitmedi ("+(j.mailError||"?")+"): kodu elle ilet. Bu kod bir daha gösterilmez."}),
    btn("Kodu kopyala",async()=>{await navigator.clipboard.writeText(j.code);},true)));
}
// A person put on the list by hand: an e-mail and a name; "Ekle ve onayla" makes the code at once (shown once, as with Onayla).
function addForm(){
  const mail=el("input",{type:"email",placeholder:"e-posta",autocomplete:"off"}),name=el("input",{type:"text",placeholder:"ad (isteğe bağlı)",autocomplete:"off"});
  mail.setAttribute("aria-label","e-posta");name.setAttribute("aria-label","ad");
  const r=el("div",{className:"row"});
  r.append(mail,name,btn("Ekle",async()=>{if(!mail.value.trim())throw new Error("e-posta yaz");await api("waitlist","POST",{email:mail.value.trim(),name:name.value.trim()});},true),
    btn("Ekle ve onayla",async()=>{if(!mail.value.trim())throw new Error("e-posta yaz");const e=mail.value.trim();
      const w=await api("waitlist","POST",{email:e,name:name.value.trim()});showCode(e,await api("waitlist/"+w.id+"/approve","POST"));}));
  return panel("Elle ekle","Listeye yaz",r);
}
const hm=()=>new Date().toLocaleTimeString("tr-TR",{hour:"2-digit",minute:"2-digit"});
async function load(){
  $("msg").textContent="";const v=$("view");v.replaceChildren();
  const tabs=$("tabs");tabs.replaceChildren();
  TABS.forEach(([k,l],i)=>{const b=el("button",{},el("span",{className:"no",textContent:"0"+(i+1)}),l);b.setAttribute("aria-pressed",String(k===tab));b.onclick=()=>{tab=k;$("banner").replaceChildren();load();};tabs.append(b);});
  let count=0;
  try{
    if(tab==="users"){
      const us=await api("users");count=us.length;
      if(!us.length)v.append(el("div",{className:"empty",textContent:"Hesap yok"}));
      else v.append(table(["Kullanıcı","Durum","Alan","Cihaz","Kayıt",""],us.map((u)=>[u.username,u.status==="active"?status("ok","Aktif"):status("bad","Askıda"),mb(u.bytes)+" / "+mb(u.quota),String(u.devices),date(u.created_at),
        el("span",{},u.status==="active"?btn("Askıya al",()=>api("users/"+u.id+"/disable","POST"),true):btn("Aç",()=>api("users/"+u.id+"/enable","POST"),true),
          btn("Sil",async()=>{if(confirm(u.username+" ve tüm verisi silinsin mi? Geri alınamaz."))await api("users/"+u.id+"/delete","POST");},"bad"))])));
    }else if(tab==="audit"){
      const a=await api("audit");count=a.length;
      if(!a.length)v.append(el("div",{className:"empty",textContent:"Kayıt yok"}));
      else v.append(table(["Zaman","İşlem"],a.map((x)=>[date(x.at),x.what])));
    }else{
      const w=await api("waitlist?status="+tab);count=w.length;
      if(tab==="pending")v.append(addForm());
      if(!w.length)v.append(el("div",{className:"empty",textContent:"Kimse yok"}));
      else v.append(table(["E-posta","Ad","Yazıldı","Durum",""],w.map((x)=>[x.email,x.name||"—",date(x.created_at),
        x.status==="approved"?(x.used_at?status("ok","Hesap açtı"):x.invite_until?status("wait","Kod "+date(x.invite_until)+" kadar"):status("off","Kodun süresi doldu")):x.status==="pending"?status("wait","Bekliyor"):status("bad","Reddedildi"),
        el("span",{},
          x.status==="pending"?btn("Onayla",async()=>showCode(x.email,await api("waitlist/"+x.id+"/approve","POST"))):
          x.status==="approved"&&!x.used_at?btn("Yeni kod gönder",async()=>showCode(x.email,await api("waitlist/"+x.id+"/resend","POST")),true):
          x.status==="rejected"?btn("Onayla",async()=>showCode(x.email,await api("waitlist/"+x.id+"/approve","POST")),true):"",
          x.status!=="rejected"&&!x.used_at?btn("Reddet",()=>api("waitlist/"+x.id+"/reject","POST"),"bad"):"")])));
    }
    $("stat").textContent=TABS.find((t)=>t[0]===tab)[1]+" · "+count+" satır · güncellendi "+hm();
  }catch(e){$("msg").textContent=e.message;$("stat").textContent="Bağlantı yok";}
}
// the clock, on flap tiles, as the board's corner has it
function clock(){
  const c=$("clock"),t=new Date().toLocaleTimeString("tr-TR",{hour:"2-digit",minute:"2-digit",second:"2-digit"});
  if(c.childElementCount!==t.length)c.replaceChildren(...[...t].map((ch)=>el("span",{className:ch===":"?"sep":"tile",textContent:ch})));
  [...t].forEach((ch,i)=>{const s=c.children[i];if(s.textContent!==ch)s.textContent=ch;});
}
clock();setInterval(clock,1000);
function show(){$("login").hidden=!!tok;$("app").hidden=!tok;if(tok)load();else $("tok").focus();}
$("enter").onclick=()=>{tok=$("tok").value.trim();if(tok){sessionStorage.setItem("adm",tok);$("tok").value="";show();}};
$("tok").addEventListener("keydown",(e)=>{if(e.key==="Enter")$("enter").click();});
$("out").onclick=()=>{sessionStorage.removeItem("adm");tok="";$("banner").replaceChildren();show();};
show();
</script></body></html>`;
  return page(html, n, "", true);
}
