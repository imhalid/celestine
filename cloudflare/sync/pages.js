// The two pages of the server: the waitlist ("/") and the administrator's ("/admin"). Plain HTML and a little
// script; what a person typed (an e-mail, a name) is put on a page only as text, never as markup.
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

// The entry screen ("/"): one screen, nothing to scroll. The name is typed out with a caret at its end; a few
// carets of the app (a block, a slime, a flame) drift about behind it, stretch as they move, blink when they rest,
// keep out of the way of the form, and the flame follows the pointer when it moves. With reduced motion asked
// for, nothing moves. What a person types goes to /api/waitlist; the answer is shown in the form's own place.
const LANDING_CSS = `
:root{--bg:#f7f5ef;--fg:#16233a;--mut:#6a7384;--line:#dcd8cd;--field:#fffdf8;--acc:#16233a;--accfg:#f7f5ef;--bad:#b4232c;--ok:#1d6b3a;
--c1:#16233a;--c2:#3fae72;--c3:#ee8a3a;--glow:rgba(22,35,58,.06)}
@media (prefers-color-scheme:dark){:root{--bg:#0c1220;--fg:#e9edf5;--mut:#8b94a8;--line:#222c42;--field:#121a2c;--acc:#e9edf5;--accfg:#0c1220;--bad:#ff7b84;--ok:#6fd394;
--c1:#cfdcf5;--c2:#58d08d;--c3:#ffa15a;--glow:rgba(120,150,220,.08)}}
*{box-sizing:border-box}
html,body{height:100%;margin:0;overflow:hidden}
body{background:var(--bg);color:var(--fg);font:16px/1.5 ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;
background-image:radial-gradient(60vmax 60vmax at 50% 42%,var(--glow),transparent 70%)}
#stage{position:fixed;inset:0;pointer-events:none;overflow:hidden}
.caret{position:absolute;left:0;top:0;will-change:transform;opacity:0;transition:opacity .6s}
.caret.on{opacity:1}
.caret i{display:block;width:var(--w);height:var(--h);margin:calc(var(--h) / -2) 0 0 calc(var(--w) / -2);background:var(--col);border-radius:var(--r);box-shadow:var(--sh,none)}
.caret.rest i{animation:blink 1.05s steps(1) infinite}
@keyframes blink{50%{opacity:0}}
main{position:relative;height:100%;max-height:100dvh;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;padding:24px 20px 56px;gap:0;overflow:auto}
h1{margin:0;font-size:clamp(3.4rem,13vw,7.5rem);line-height:1;font-weight:680;letter-spacing:-.045em;display:flex;align-items:center}
h1 .tc{display:inline-block;width:.1em;height:.82em;margin-left:.06em;background:var(--c1);border-radius:.03em;animation:blink 1.05s steps(1) infinite}
p.lead{margin:1.1rem 0 0;color:var(--mut);font-size:clamp(1rem,2.4vw,1.2rem);max-width:30rem;text-wrap:balance}
.slot{margin-top:2rem;width:min(30rem,100%);min-height:3.4rem;display:flex;align-items:center;justify-content:center}
form{width:100%;display:flex;align-items:center;gap:6px;background:var(--field);border:1px solid var(--line);border-radius:999px;padding:5px 5px 5px 20px;
box-shadow:0 1px 0 rgba(255,255,255,.4) inset,0 8px 30px -12px rgba(22,35,58,.25);transition:border-color .2s,box-shadow .2s}
form:focus-within{border-color:var(--fg);box-shadow:0 8px 30px -10px rgba(22,35,58,.35)}
input{flex:1;min-width:0;font:inherit;color:inherit;background:transparent;border:0;outline:0;padding:.6rem 0}
input::placeholder{color:var(--mut)}
button{font:inherit;font-weight:600;cursor:pointer;border:0;border-radius:999px;padding:.7rem 1.3rem;background:var(--acc);color:var(--accfg);white-space:nowrap;transition:transform .15s,opacity .2s}
button:active{transform:translateY(1px)}button[disabled]{opacity:.55;cursor:default}
button:focus-visible{outline:2px solid var(--fg);outline-offset:2px}
.msg{height:1.5rem;margin-top:.8rem;font-size:.92rem;color:var(--mut)}
.msg.bad{color:var(--bad)}
.done{display:flex;align-items:center;gap:.6rem;font-size:1.1rem;color:var(--ok);animation:rise .5s cubic-bezier(.2,.8,.2,1)}
.done svg{flex:none}
@keyframes rise{from{opacity:0;transform:translateY(8px)}}
.cf-turnstile{margin-top:.4rem}
footer{position:fixed;left:0;right:0;bottom:0;padding:14px 20px 18px;text-align:center;color:var(--mut);font-size:.78rem}
@media (max-height:520px){h1{font-size:3rem}.slot{margin-top:1rem}p.lead{margin-top:.5rem}}
@media (prefers-reduced-motion:reduce){.caret{display:none}h1 .tc,.caret.rest i{animation:none}.done{animation:none}}
`;

const LANDING_JS = `
(()=>{
const reduce=matchMedia("(prefers-reduced-motion: reduce)").matches;
const f=document.getElementById("f"),msg=document.getElementById("msg"),go=document.getElementById("go"),slot=document.getElementById("slot");
f.addEventListener("submit",async(e)=>{
  e.preventDefault();msg.className="msg";msg.textContent="";go.disabled=true;
  try{
    const t=document.querySelector("[name=cf-turnstile-response]");
    const r=await fetch("/api/waitlist",{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({email:f.email.value,turnstile:t?t.value:""})});
    const j=await r.json().catch(()=>({}));
    if(r.ok){
      slot.replaceChildren();const d=document.createElement("div");d.className="done";d.setAttribute("role","status");
      d.innerHTML='<svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M5 12.5l4.5 4.5L19 7.5"/></svg>';
      d.append("Listedesin. Onaylanınca e-postana bir davet kodu gelecek.");slot.append(d);
    }else{msg.className="msg bad";msg.textContent=j.error||"Olmadı, sonra tekrar dene.";if(window.turnstile)turnstile.reset();go.disabled=false;}
  }catch{msg.className="msg bad";msg.textContent="Bağlantı kurulamadı.";go.disabled=false;}
});
// the name is typed, and its caret stays at the end
const word=document.getElementById("w"),full=word.dataset.t;
if(reduce){word.textContent=full;}else{let i=0;word.textContent="";const tick=()=>{word.textContent=full.slice(0,++i);if(i<full.length)setTimeout(tick,95+Math.random()*70);};setTimeout(tick,350);}
if(reduce)return;
// carets that drift
const stage=document.getElementById("stage"),card=document.getElementById("keep");
const defs=[
  {w:14,h:30,r:"3px",col:"var(--c1)"},
  {w:13,h:28,r:"7px",col:"var(--c2)",sh:"0 0 18px -2px var(--c2)"},
  {w:11,h:30,r:"5px 5px 2px 2px",col:"var(--c3)",sh:"0 0 22px 2px var(--c3)",follow:true},
];
const W=()=>innerWidth,H=()=>innerHeight;
let px=-1,py=-1,pmove=0;
addEventListener("pointermove",(e)=>{px=e.clientX;py=e.clientY;pmove=performance.now();},{passive:true});
const keepOut=()=>{const r=card.getBoundingClientRect();const m=28;return {l:r.left-m,t:r.top-m,r:r.right+m,b:r.bottom+m};};
function pick(){
  const k=keepOut();
  for(let n=0;n<30;n++){
    const x=W()*(.04+Math.random()*.92),y=H()*(.06+Math.random()*.88);
    if(!(x>k.l&&x<k.r&&y>k.t&&y<k.b))return [x,y];
  }
  return [W()*.1,H()*.1];
}
const cs=defs.map((d,n)=>{
  const el=document.createElement("div");el.className="caret";el.setAttribute("aria-hidden","true");
  const i=document.createElement("i");el.append(i);
  el.style.cssText="--w:"+d.w+"px;--h:"+d.h+"px;--r:"+d.r+";--col:"+d.col+(d.sh?";--sh:"+d.sh:"");
  stage.append(el);
  const [x,y]=pick();
  const c={el,d,x,y,vx:0,vy:0,tx:x,ty:y,until:0,rest:false};
  setTimeout(()=>el.classList.add("on"),500+n*350);
  return c;
});
let last=performance.now();
function frame(now){
  const dt=Math.min(.033,(now-last)/1000);last=now;
  for(const c of cs){
    const following=c.d.follow&&px>=0&&now-pmove<2200;
    if(following){c.tx=Math.min(W()-8,Math.max(8,px+16));c.ty=Math.min(H()-8,Math.max(8,py-6));}
    else if(now>c.until){[c.tx,c.ty]=pick();c.until=now+1500+Math.random()*2600;}
    const k=following?26:15,damp=following?8.5:6.2;
    c.vx+=((c.tx-c.x)*k-c.vx*damp)*dt;c.vy+=((c.ty-c.y)*k-c.vy*damp)*dt;
    c.x+=c.vx*dt;c.y+=c.vy*dt;
    const sp=Math.hypot(c.vx,c.vy);
    const s=Math.min(.7,sp/1100),a=Math.atan2(c.vy,c.vx)*180/Math.PI;
    c.el.style.transform="translate("+c.x.toFixed(1)+"px,"+c.y.toFixed(1)+"px)"+(s>.02?" rotate("+a.toFixed(1)+"deg) scale("+(1+s).toFixed(3)+","+(1/Math.sqrt(1+s)).toFixed(3)+") rotate("+(-a).toFixed(1)+"deg)":"");
    const rest=sp<14;
    if(rest!==c.rest){c.rest=rest;c.el.classList.toggle("rest",rest);}
  }
  requestAnimationFrame(frame);
}
requestAnimationFrame(frame);
})();
`;

export function waitlistPage(env) {
  const n = nonce();
  const site = env.TURNSTILE_SITEKEY || "";
  const contact = env.CONTACT_EMAIL ? ` Silinmesini istersen ${esc(env.CONTACT_EMAIL)} adresine yaz.` : "";
  const html = `<!doctype html><html lang="tr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>Ryudit</title><meta name="description" content="Ryudit erken erişimde: bekleme listesine yazıl."><meta name="color-scheme" content="light dark">
<style>${LANDING_CSS}</style></head><body>
<div id="stage"></div>
<main><div id="keep" style="display:flex;flex-direction:column;align-items:center;width:100%">
<h1 aria-label="Ryudit"><span id="w" data-t="Ryudit" aria-hidden="true">Ryudit</span><span class="tc" aria-hidden="true"></span></h1>
<p class="lead">Bloklar, koleksiyonlar ve yerel öncelikli senkron. Erken erişimde: listeye yazıl, sıran gelince davet kodunu gönderelim.</p>
<div id="slot" class="slot">
<form id="f" novalidate><input id="email" name="email" type="email" inputmode="email" autocomplete="email" placeholder="e-posta adresin" aria-label="E-posta" required maxlength="254"><button id="go" type="submit">Listeye yazıl</button></form>
</div>
<div id="msg" class="msg" role="status" aria-live="polite"></div>
${site ? `<div class="cf-turnstile" data-sitekey="${esc(site)}" data-appearance="interaction-only"></div>` : ""}
</div></main>
<footer>E-postan yalnızca davet kodunu göndermek için kullanılır, başka yerle paylaşılmaz.${contact}</footer>
${site ? `<script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>` : ""}
<script nonce="${n}">${LANDING_JS}</script></body></html>`;
  return page(html, n, site ? "https://challenges.cloudflare.com" : "");
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
