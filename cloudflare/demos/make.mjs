// Writes the demo pages of the entry screen into ./dist (node make.mjs). Each demo is a look: some CSS for the page
// and one function of GLSL for the picture (fxkit.js does the rest).
import { mkdirSync, writeFileSync, copyFileSync } from "node:fs";

const BASE_CSS = `
*{box-sizing:border-box}
html,body{height:100%;margin:0;overflow:hidden}
.fx-ui{display:flex;align-items:center;justify-content:center;text-align:center;padding:24px 20px 52px;overflow:hidden}
#keep{display:flex;flex-direction:column;align-items:center;width:100%}
.slot{margin-top:2rem;width:min(30rem,100%);min-height:3.2rem;display:flex;align-items:center;justify-content:center}
form{width:100%;display:flex;align-items:stretch;gap:8px}
input{flex:1;min-width:0;font:inherit;outline:0}
button{font:inherit;cursor:pointer;white-space:nowrap}
button[disabled]{opacity:.55;cursor:default}
.msg{height:1.5rem;margin-top:.8rem;font-size:.88rem}
.done{display:flex;align-items:center;gap:.6rem}
footer{position:absolute;left:0;right:0;bottom:0;padding:12px 20px 16px;text-align:center;font-size:.72rem}
.demo-nav{position:fixed;top:10px;left:12px;z-index:50;font:12px/1.3 ui-monospace,Menlo,monospace;display:flex;gap:10px;align-items:center}
.demo-nav a{color:inherit;opacity:.7;text-decoration:none;border-bottom:1px dotted currentColor}
.demo-nav a:hover{opacity:1}
@media (max-height:520px){.slot{margin-top:1rem}}
`;

const demos = [
  {
    id: "1-sifre", name: "Şifre çözme", blurb: "Sayfa şifreli karakterlerden oluşuyor; imleç yaklaşınca, odaklanınca ve belli aralıklarla süpürülen bir dalgayla çözülüyor.",
    nav: "#8d93a8", bg: "#07080c",
    css: `
body{background:#07080c;color:#eef0f7;font:16px/1.55 ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
h1{margin:0;font-size:clamp(3.6rem,14vw,9rem);line-height:.95;font-weight:800;letter-spacing:-.065em;background:linear-gradient(180deg,#fff,#aab0ff);-webkit-background-clip:text;background-clip:text;color:transparent}
p.lead{margin:1.3rem 0 0;color:#9aa0b8;font-size:clamp(.95rem,2.2vw,1.1rem);max-width:32rem;text-wrap:balance}
input{color:#fff;background:rgba(255,255,255,.06);border:1px solid #343a55;border-radius:10px;padding:.8rem 1rem;caret-color:#9d92ff}
input::placeholder{color:#6a7090}
input:focus{border-color:#9d92ff;box-shadow:0 0 0 1px #9d92ff}
button{font-weight:700;border:0;border-radius:10px;padding:.8rem 1.3rem;background:#fff;color:#07080c}
button:hover{background:#d9d6ff}
.msg{color:#9aa0b8}.msg.bad{color:#ff8a8a}.done{color:#cfc9ff}
footer{color:#5b617c}
.nofx{background:radial-gradient(ellipse at 50% 40%,#12142a,#07080c 70%)}
`,
    atlas: `{chars:"01234567890ABCDEF<>/\\\\|=+*#%$&@?!:;",cw:20,ch:36}`,
    frag: `
vec3 shade(vec2 uv){
  vec2 pd=uv*uRes;
  vec2 cell=vec2(9.,17.)*uScale;
  vec2 ci=floor(pd/cell);
  vec2 cu=(ci+.5)*cell/uRes;
  vec2 o1=vec2(.28,.28)*cell/uRes,o2=vec2(-.28,.28)*cell/uRes;
  vec4 a0=ui(cu),a1=ui(cu+o1),a2=ui(cu+o2),a3=ui(cu-o1),a4=ui(cu-o2);
  float sa=a0.a+a1.a+a2.a+a3.a+a4.a;
  float cov=sa/5.;
  vec3 ucol=(a0.rgb*a0.a+a1.rgb*a1.a+a2.rgb*a2.a+a3.rgb*a3.a+a4.rgb*a4.a)/max(sa,.001);
  float tick=floor(uT*(cov>.04?9.:2.));
  float gi=floor(hash21(ci+tick*.37)*uAtlasN);
  vec2 loc=fract(pd/cell);
  float g=texture(uAtlas,vec2((gi+loc.x)/uAtlasN,1.-loc.y)).r;
  // where it is resolved: near the pointer (or a drifting one when it is still), while a field has focus, and in a wave
  vec2 auto=vec2(.5+.34*sin(uT*.37),.5+.2*sin(uT*.53+1.));
  vec2 mp=mix(auto,uMouse.xy,smoothstep(4.,2.,uMouse.w));
  float d=length((uv-mp)*vec2(uRes.x/uRes.y,1.))/(250.*uScale/uRes.y);
  float rm=1.-smoothstep(.55,1.,d);
  float cyc=mod(uT,9.);
  float dec=smoothstep(0.,.12,cyc/1.6*1.3-uv.x);
  float enc=1.-smoothstep(0.,.12,(cyc-4.5)/1.6*1.3-uv.x);
  float wave=cyc<4.5?dec:enc;
  float r=clamp(max(max(rm,wave),uFocus),0.,1.);
  vec3 bg=vec3(.027,.03,.045);
  vec3 vio=vec3(.6,.55,1.);
  float rain=step(.9,hash21(ci*1.7+floor(uT*.6+hash21(ci)*5.)))*(.25+.5*hash21(ci+3.));
  vec3 cipher=bg+vio*g*rain*.6*(1.-cov)+mix(vio,ucol,.4)*g*cov*2.6+ucol*cov*.07;
  vec4 u=ui(uv);
  vec2 px=vec2(5.*uScale)/uRes;
  float glow=(ui(uv+px).a+ui(uv-px).a+ui(uv+vec2(px.x,-px.y)).a+ui(uv+vec2(-px.x,px.y)).a)*.25;
  vec3 real=bg+u.rgb*u.a+vio*glow*.18;
  vec3 col=mix(cipher,real,r);
  col+=vio*exp(-pow((d-.78)*9.,2.))*.3*smoothstep(.6,.0,uMouse.w);
  vec2 cc=uv*2.-1.;
  return col*(1.-.3*dot(cc,cc));
}`,
  },
  {
    id: "2-sivi-cam", name: "Sıvı cam", blurb: "Süt beyazı zemin üzerinde yavaş akan pastel lekeler; sayfa camın içinden görünüyor, imleç suya dokunuyormuş gibi dalgalandırıyor.",
    nav: "#16233a", bg: "#f6f3ec",
    css: `
body{background:#f6f3ec;color:#16233a;font:16px/1.55 ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
h1{margin:0;font-size:clamp(3.6rem,14vw,9rem);line-height:.95;font-weight:700;letter-spacing:-.06em;color:#16233a}
p.lead{margin:1.3rem 0 0;color:#46506b;font-size:clamp(.95rem,2.2vw,1.1rem);max-width:32rem;text-wrap:balance}
input{color:#16233a;background:rgba(255,255,255,.5);border:1px solid rgba(22,35,58,.28);border-radius:999px;padding:.85rem 1.2rem;caret-color:#16233a}
input::placeholder{color:#6b7590}
input:focus{border-color:#16233a;box-shadow:0 0 0 1px #16233a}
button{font-weight:650;border:0;border-radius:999px;padding:.85rem 1.5rem;background:#16233a;color:#f6f3ec}
button:hover{background:#26375a}
.msg{color:#46506b}.msg.bad{color:#b4232c}.done{color:#16233a}
footer{color:#6b7590}
.nofx{background:radial-gradient(circle at 25% 30%,#d9deff,transparent 45%),radial-gradient(circle at 75% 65%,#ffd9c9,transparent 45%),#f6f3ec}
`,
    frag: `
vec3 bgc(vec2 uv,float t){
  vec2 p=(uv-.5)*vec2(uRes.x/uRes.y,1.);
  vec3 c=vec3(.965,.953,.925);
  vec3 cols[4]=vec3[](vec3(.48,.55,1.),vec3(1.,.72,.6),vec3(.61,.9,.81),vec3(.84,.7,1.));
  for(int i=0;i<4;i++){
    float fi=float(i);
    vec2 cp=vec2(sin(t*.17+fi*1.9)*.62,cos(t*.13+fi*2.4)*.32);
    float d=length(p-cp);
    c=mix(c,cols[i],exp(-d*d*5.)*.66);
  }
  return c;
}
vec2 disp(vec2 uv){
  vec2 d=(uv-uMouse.xy)*vec2(uRes.x/uRes.y,1.);
  float r=length(d);
  float w=sin(r*46.-uT*5.)*exp(-r*5.)*(.0025+.016*uMouse.z)*smoothstep(3.,.5,uMouse.w);
  return d/max(r,1e-3)*w*vec2(uRes.y/uRes.x,1.);
}
vec3 shade(vec2 uv){
  vec2 dp=disp(uv);
  vec2 q=uv+dp;
  vec2 px=3.*uScale/uRes;
  float aL=ui(q-vec2(px.x,0.)).a,aR=ui(q+vec2(px.x,0.)).a,aD=ui(q-vec2(0.,px.y)).a,aU=ui(q+vec2(0.,px.y)).a;
  vec2 n=vec2(aR-aL,aU-aD);
  vec3 base=bgc(q+n*.012,uT);
  float sh=(ui(q+vec2(0.,-.006)).a+ui(q+vec2(.004,-.01)).a)*.5;
  base*=1.-.15*sh;
  vec2 dv=dp*6.+n*.0012;
  vec4 ur=ui(q+dv*.5),ug=ui(q),ub=ui(q-dv*.5);
  vec3 col=vec3(mix(base.r,ur.r,ur.a),mix(base.g,ug.g,ug.a),mix(base.b,ub.b,ub.a));
  col+=pow(clamp(length(n)*2.,0.,1.),1.5)*.22;
  col+=(hash21(uv*uRes+uT)-.5)*.03;
  return col;
}`,
  },
  {
    id: "3-vhs", name: "VHS", blurb: "Eski bir kaset: dalgalanan çizgiler, alttaki kafa değişim gürültüsü, yukarı kayan takip bandı, renk taşması, tane. Ekran yazıları da kasetten geçiyor.",
    nav: "#9a8fb8", bg: "#0a0912",
    css: `
body{background:#0a0912;color:#f4f1ff;font:16px/1.55 ui-monospace,"SF Mono",Menlo,Consolas,monospace}
h1{margin:0;font-size:clamp(3.4rem,13vw,8.5rem);line-height:1;font-weight:900;font-style:italic;letter-spacing:-.05em;text-transform:uppercase;color:#fff}
p.lead{margin:1.2rem 0 0;color:#b5aed0;font-size:clamp(.85rem,2vw,1rem);max-width:34rem;text-wrap:balance}
input{color:#fff;background:rgba(255,255,255,.07);border:2px solid #6b5fa0;border-radius:2px;padding:.75rem .9rem;caret-color:#ff2d95}
input::placeholder{color:#7d73a6}
input:focus{border-color:#2de2ff}
button{font-weight:800;border:2px solid #ff2d95;border-radius:2px;padding:.75rem 1.2rem;background:#ff2d95;color:#fff;text-transform:uppercase;letter-spacing:.06em}
button:hover{background:#ff5cae}
.msg{color:#b5aed0}.msg.bad{color:#ff8a8a}.done{color:#2de2ff}
footer{color:#6f6794}
.osd{position:absolute;font-size:clamp(.9rem,2vw,1.35rem);font-weight:700;letter-spacing:.08em;color:#fff;text-shadow:0 0 6px rgba(255,255,255,.5)}
.osd.tl{top:5vh;left:4vw}.osd.tr{top:5vh;right:4vw}.osd.bl{bottom:9vh;left:4vw}.osd.br{bottom:9vh;right:4vw}
.rec{color:#ff3b3b;animation:blink 1.2s steps(1) infinite}
@keyframes blink{50%{opacity:0}}
.nofx::after{content:"";position:fixed;inset:0;pointer-events:none;background:repeating-linear-gradient(to bottom,rgba(0,0,0,.22) 0 1px,transparent 1px 3px)}
`,
    extra: `<div class="osd tl">▶ PLAY</div><div class="osd tr"><span class="rec">●</span> SP</div><div class="osd bl" id="clock">0:00:00</div><div class="osd br">NOV 07 2026</div>`,
    script: `const clk=document.getElementById("clock");const t0=Date.now()-12*60000;setInterval(()=>{const s=Math.floor((Date.now()-t0)/1000);clk.textContent=Math.floor(s/3600)+":"+String(Math.floor(s/60)%60).padStart(2,"0")+":"+String(s%60).padStart(2,"0");},500);`,
    frag: `
vec3 shade(vec2 uv){
  float t=uT;
  float line=floor(uv.y*uRes.y/(2.*uScale));
  float wob=(vnoise(vec2(uv.y*9.,t*.8))-.5)*.006;
  wob+=(hash11(line+floor(t*24.))-.5)*.0025*step(.8,hash11(floor(t*2.)+7.));
  float hs=smoothstep(.09,.0,uv.y);
  wob+=hs*(vnoise(vec2(uv.y*60.,t*18.))-.3)*.05;
  float band=1.-fract(t*.07);
  float tb=smoothstep(.04,.0,abs(uv.y-band));
  wob+=tb*(vnoise(vec2(uv.y*300.,t*25.))-.5)*.035;
  vec2 q=vec2(uv.x+wob,uv.y);
  float bl=.0035;
  vec3 bg=vec3(.04,.034,.08)+vec3(.03,.0,.04)*uv.y;
  vec4 r0=ui(q-vec2(bl*.5,0.)),r1=ui(q-vec2(bl*1.5,0.)),r2=ui(q-vec2(bl*2.5,0.));
  vec4 g0=ui(q);
  vec4 b0=ui(q+vec2(bl*.5,0.)),b1=ui(q+vec2(bl*1.5,0.)),b2=ui(q+vec2(bl*2.5,0.));
  float ar=r0.a*.5+r1.a*.3+r2.a*.2,ab=b0.a*.5+b1.a*.3+b2.a*.2;
  vec3 col=bg;
  col.r=mix(bg.r,(r0.r*r0.a*.5+r1.r*r1.a*.3+r2.r*r2.a*.2)/max(ar,.001),ar);
  col.g=mix(bg.g,g0.g,g0.a);
  col.b=mix(bg.b,(b0.b*b0.a*.5+b1.b*b1.a*.3+b2.b*b2.a*.2)/max(ab,.001),ab);
  col+=vec3(1.,.1,.55)*ui(q-vec2(.007,0.)).a*.14+vec3(.1,.9,1.)*ui(q+vec2(.007,0.)).a*.14;
  col*=1.-.14*step(.5,fract(line*.5));
  float n=hash21(vec2(uv.x*uRes.x*.5,line)+t*13.);
  col+=(n-.5)*.085;
  col+=step(.9993,hash21(vec2(floor(uv.x*uRes.x/(6.*uScale)),line)+floor(t*30.)))*.6;
  col+=hs*(hash21(vec2(floor(uv.x*uRes.x/(3.*uScale)),line)+t)-.45)*.45;
  col+=tb*.08;
  vec2 cc=uv*2.-1.;
  col*=1.-.4*dot(cc,cc);
  col*=mix(vec3(1.),vec3(1.04,.97,1.06),.6);
  return col*smoothstep(0.,.8,uOn);
}`,
  },
  {
    id: "4-dither", name: "Dither", blurb: "İki renk: süt beyazı ve lacivert. Her şey sıralı dither ile çiziliyor; imleç bir yuvarlak içinde renkleri ters çeviriyor ve bir blok caret taşıyor.",
    nav: "#14213d", bg: "#f4f1e8",
    css: `
body{background:#f4f1e8;color:#14213d;font:16px/1.55 ui-monospace,"SF Mono",Menlo,Consolas,monospace}
h1{margin:0;font-size:clamp(3.6rem,14vw,9.5rem);line-height:.92;font-weight:900;letter-spacing:-.07em;font-family:Georgia,"Times New Roman",serif;color:#14213d}
p.lead{margin:1.3rem 0 0;color:#14213d;font-weight:700;font-size:clamp(.9rem,2vw,1.05rem);max-width:32rem;text-wrap:balance}
input{color:#14213d;background:transparent;border:2px solid #14213d;border-radius:0;padding:.75rem .9rem;caret-color:#14213d}
input{font-weight:700}
input::placeholder{color:#14213d;opacity:.75}
input:focus{box-shadow:4px 4px 0 #14213d}
button{font-weight:800;border:2px solid #14213d;border-radius:0;padding:.75rem 1.2rem;background:#14213d;color:#f4f1e8}
button:hover{background:#f4f1e8;color:#14213d}
.msg{color:#14213d}.msg.bad{color:#b4232c}.done{color:#14213d;font-weight:700}
footer{color:#4b5875}
.nofx{background-image:radial-gradient(#14213d 1px,transparent 1.2px);background-size:7px 7px;background-color:#f4f1e8}
.nofx .fx-ui{background:radial-gradient(ellipse 36rem 18rem at 50% 50%,#f4f1e8 60%,transparent)}
`,
    frag: `
float bayer2(vec2 a){a=floor(a);return fract(a.x/2.+a.y*a.y*.75);}
float bayer4(vec2 a){return bayer2(.5*a)*.25+bayer2(a);}
float bayer8(vec2 a){return bayer4(.5*a)*.25+bayer2(a);}
float field(vec2 uv){
  vec2 p=(uv-.5)*vec2(uRes.x/uRes.y,1.);
  float t=uT*.25;
  float v=.5+.5*sin(length(p-vec2(sin(t)*.45,cos(t*.8)*.22))*7.-t*2.);
  v+=.5+.5*sin(p.x*3.+p.y*2.+t*1.5+sin(p.y*5.+t));
  v*=.5;
  v=pow(v,1.7)*.5;
  v+=pow(clamp(length(p)*.9,0.,1.5),2.)*.22;
  return v;
}
vec3 shade(vec2 uv){
  float ps=2.*uScale;
  vec2 pd=uv*uRes;
  vec2 pp=floor(pd/ps);
  vec2 puv=(pp+.5)*ps/uRes;
  float thr=bayer8(pp);
  vec3 milk=vec3(.957,.945,.91),navy=vec3(.078,.129,.239),coral=vec3(.91,.31,.227);
  vec4 u=ui(puv);
  float ink=u.a*(1.-dot(u.rgb,vec3(.3,.59,.11)));
  ink=smoothstep(.08,.5,ink);
  float f=field(puv);
  float v=max(f*(1.-u.a*.0),ink);
  float pix=step(thr,v*1.0+ (ink>.5?.0:0.));
  pix=max(pix,step(.5,ink));
  // the pointer: the colours turn over inside a circle, and a block caret follows
  vec2 auto=vec2(.5+.3*sin(uT*.31),.5+.2*sin(uT*.47+2.));
  vec2 mp=mix(auto,uMouse.xy,smoothstep(4.,2.,uMouse.w));
  vec2 mpx=floor(mp*uRes/ps)*ps;
  float inv=step(length(floor(pd/ps)*ps-mpx),(110.+30.*uMouse.z)*uScale);
  float p2=mix(pix,1.-pix,inv);
  vec3 col=mix(milk,navy,p2);
  vec2 bp=pd-(mpx+vec2(14.,-6.)*uScale);
  float blk=step(abs(bp.x),5.*uScale)*step(abs(bp.y),11.*uScale)*step(.5,fract(uT*1.3));
  col=mix(col,coral,blk);
  return col*smoothstep(0.,.5,uOn)+milk*(1.-smoothstep(0.,.5,uOn));
}`,
  },
];

function page(d) {
  return `<!doctype html><html lang="tr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>Ryudit — ${d.name}</title><meta name="robots" content="noindex"><style>${BASE_CSS}${d.css}</style></head><body>
<canvas id="fx"></canvas>
<main id="ui"><div id="keep">
<h1>Ryudit</h1>
<p class="lead">Bloklar, koleksiyonlar ve yerel öncelikli senkron. Erken erişimde: listeye yazıl, sıran gelince davet kodunu gönderelim.</p>
<div id="slot" class="slot"><form id="f" novalidate><input id="email" name="email" type="email" inputmode="email" autocomplete="email" placeholder="e-posta adresin" aria-label="E-posta" required maxlength="254"><button id="go" type="submit">Listeye yazıl</button></form></div>
<div id="msg" class="msg" role="status" aria-live="polite"></div>
</div>
${d.extra || ""}
<footer>E-postan yalnızca davet kodunu göndermek için kullanılır, başka yerle paylaşılmaz.</footer>
</main>
<nav class="demo-nav" style="color:${d.nav}"><a href="./">← demolar</a><span>${d.name}</span><a href="?debug">debug</a></nav>
<script type="module">
import {mount,bindForm} from "./fxkit.js";
const $=(i)=>document.getElementById(i);
mount({canvas:$("fx"),ui:$("ui"),frag:${JSON.stringify(d.frag)},probe:"input"${d.atlas ? ",atlas:" + d.atlas : ""}});
bindForm({form:$("f"),slot:$("slot"),msg:$("msg"),go:$("go"),input:$("email")});
${d.script || ""}
</script></body></html>`;
}

function index() {
  const cards = demos.map((d) => `<a class="card" href="${d.id}.html" style="--bg:${d.bg};--fg:${d.nav}"><b>${d.name}</b><span>${d.blurb}</span></a>`).join("");
  return `<!doctype html><html lang="tr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="robots" content="noindex"><title>Ryudit — giriş ekranı demoları</title>
<style>
*{box-sizing:border-box}body{margin:0;background:#101216;color:#e8eaf0;font:16px/1.5 ui-sans-serif,system-ui,sans-serif;padding:6vh 20px}
main{max-width:56rem;margin:0 auto}h1{font-size:1.6rem;margin:0 0 .3rem}p{color:#9aa0b0;margin:.2rem 0 1.4rem}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(15rem,1fr));gap:14px}
.card{display:flex;flex-direction:column;gap:.5rem;padding:1.1rem;border-radius:14px;background:var(--bg);color:var(--fg);text-decoration:none;border:1px solid rgba(255,255,255,.12);min-height:10rem}
.card b{font-size:1.15rem}.card span{font-size:.88rem;opacity:.85}.card:hover{outline:2px solid #8d93a8}
code{background:rgba(255,255,255,.08);padding:.1rem .35rem;border-radius:4px}
</style></head><body><main>
<h1>Giriş ekranı demoları</h1>
<p>Dört farklı bakış. Hepsinde sayfa (yazılar, alan, düğme) gerçek HTML ve shader'ın içinden geçiyor; tarayıcı HTML-in-canvas'ı desteklemiyorsa HTML canvas'ın üstüne biniyor.</p>
<div class="grid">${cards}<a class="card" href="https://ryudit.halid.dev" style="--bg:#03070c;--fg:#4dffa3"><b>CRT + aurora (şu an canlı)</b><span>Şu an yayında olan: aurora, ASCII, bloom, CRT ve gezen caret'ler.</span></a></div>
<p style="margin-top:1.6rem">Her demonun adresine <code>?debug</code> eklersen sol altta tarayıcının neyi desteklediği ve hangi yolun kullanıldığı görünür.</p>
</main></body></html>`;
}

mkdirSync("dist", { recursive: true });
copyFileSync("fxkit.js", "dist/fxkit.js");
for (const d of demos) writeFileSync(`dist/${d.id}.html`, page(d));
writeFileSync("dist/index.html", index());
console.log("wrote", demos.length, "demos");
