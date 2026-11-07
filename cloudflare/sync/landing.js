// The entry screen ("/"): one screen, nothing to scroll, drawn as a CRT.
//
// Behind it an aurora is painted in a shader, and read out as ASCII (each cell of the screen picks the
// character whose weight is that of the light there). Three carets of the app drift across it: the CRT's phosphor
// block, the flame and the slime. All of it is bloomed, and put through a CRT: scanlines, a mask, a little barrel,
// colour fringes, vignette, grain, a power-on.
//
// The page itself (the name, the form) is real HTML. Where the browser has HTML-in-canvas (Chrome's
// layoutsubtree and texElementSubImage2D) it is drawn *into* the same shader, so the text glows and is scanlined
// with the rest, and stays a live form (typing, focus, a screen reader). Where it has not, the HTML is laid over
// the canvas with a CSS glow and scanlines, and only what is behind it is the shader's. With reduced motion asked
// for, a single frame is drawn and nothing moves.

const CSS = `
:root{color-scheme:dark;--bg:#03070c;--fg:#e8fff1;--mut:#7fa592;--line:#2c6a4c;--ph:#4dffa3;--bad:#ff7b7b}
*{box-sizing:border-box}
html,body{height:100%;margin:0;overflow:hidden;background:var(--bg)}
body{color:var(--fg);font:15px/1.55 ui-monospace,"SF Mono",SFMono-Regular,Menlo,Consolas,"Liberation Mono",monospace}
#fx{position:fixed;left:0;top:0;width:100vw;height:100vh;display:block;z-index:0}
#ui{position:fixed;inset:0;z-index:1;display:flex;align-items:center;justify-content:center;text-align:center;padding:24px 20px 52px;overflow:hidden}
#ui.in-canvas{position:absolute;inset:auto;left:0;top:0;width:var(--w,100vw);height:var(--h,100vh)}
#keep{display:flex;flex-direction:column;align-items:center;width:100%}
h1{margin:0;font-size:clamp(3.4rem,13vw,8rem);line-height:1;font-weight:800;letter-spacing:-.06em;display:flex;align-items:center;color:#f2fff7}
h1 .tc{display:inline-block;width:.56em;height:.78em;margin-left:.08em;background:var(--ph);animation:blink 1.05s steps(1) infinite}
@keyframes blink{50%{opacity:0}}
p.lead{margin:1.2rem 0 0;color:var(--mut);font-size:clamp(.85rem,2vw,1rem);max-width:34rem;text-wrap:balance}
.slot{margin-top:2rem;width:min(30rem,100%);min-height:3.2rem;display:flex;align-items:center;justify-content:center}
form{width:100%;display:flex;align-items:stretch;gap:8px}
input{flex:1;min-width:0;font:inherit;color:var(--fg);background:rgba(2,18,10,.78);border:1px solid var(--line);border-radius:6px;padding:.7rem .9rem;outline:0;caret-color:var(--ph)}
input::placeholder{color:#5d8a74}
input:focus{border-color:var(--ph);box-shadow:0 0 0 1px var(--ph)}
button{font:inherit;font-weight:800;cursor:pointer;border:1px solid #38e096;border-radius:6px;padding:.7rem 1.1rem;background:#2cc98a;color:#000;white-space:nowrap}
button:hover{background:#4be8a0}button:active{transform:translateY(1px)}button[disabled]{opacity:.55;cursor:default}
button:focus-visible{outline:2px solid #fff;outline-offset:2px}
.msg{height:1.5rem;margin-top:.8rem;font-size:.88rem;color:var(--mut)}
.msg.bad{color:var(--bad)}
.done{display:flex;align-items:center;gap:.6rem;font-size:1rem;color:var(--ph)}
.done svg{flex:none}
footer{position:absolute;left:0;right:0;bottom:0;padding:12px 20px 16px;text-align:center;color:#5d8a74;font-size:.72rem}
.cf-turnstile{position:fixed;left:50%;bottom:44px;transform:translateX(-50%);z-index:5}
/* without HTML-in-canvas: the page is laid over the canvas, and gets its glow and scanlines from CSS */
.nofx #ui{text-shadow:0 0 6px rgba(77,255,163,.55),0 0 22px rgba(77,255,163,.28)}
.nofx h1{text-shadow:0 0 10px rgba(120,255,190,.8),0 0 40px rgba(77,255,163,.45)}
.nofx::after{content:"";position:fixed;inset:0;z-index:3;pointer-events:none;
background:repeating-linear-gradient(to bottom,rgba(0,0,0,.18) 0 1px,transparent 1px 3px),radial-gradient(ellipse at center,transparent 55%,rgba(0,0,0,.55) 100%)}
.nogl #ui{text-shadow:0 0 8px rgba(77,255,163,.4)}
@media (max-height:520px){h1{font-size:3rem}.slot{margin-top:1rem}p.lead{margin-top:.5rem}}
@media (prefers-reduced-motion:reduce){h1 .tc{animation:none}}
`;

// ---- the shaders ----
const VERT = `#version 300 es
out vec2 vUv;
void main(){vec2 p=vec2(float((gl_VertexID<<1)&2),float(gl_VertexID&2));vUv=p;gl_Position=vec4(p*2.-1.,0.,1.);}`;

const FRAG_AURORA = `#version 300 es
precision highp float;
in vec2 vUv;out vec4 o;
uniform vec2 uRes;uniform float uT;
float hash(vec2 p){p=fract(p*vec2(123.34,456.21));p+=dot(p,p+45.32);return fract(p.x*p.y);}
float noise(vec2 p){vec2 i=floor(p),f=fract(p);f=f*f*(3.-2.*f);
return mix(mix(hash(i),hash(i+vec2(1,0)),f.x),mix(hash(i+vec2(0,1)),hash(i+vec2(1,1)),f.x),f.y);}
float fbm(vec2 p){float a=.5,s=0.;for(int i=0;i<4;i++){s+=a*noise(p);p=p*2.03+vec2(17.1,9.2);a*=.5;}return s;}
void main(){
  vec2 uv=vUv;float asp=uRes.x/uRes.y;float t=uT;vec3 col=vec3(0.);
  for(int i=0;i<3;i++){
    float fi=float(i);
    float x=uv.x*asp*(.8+fi*.32)+fi*7.3;
    float flow=fbm(vec2(x*.8,t*.045+fi*3.1));
    float base=.50+.115*fi+.24*(flow-.5)+.05*sin(x*1.6+t*.17+fi*2.);
    float d=uv.y-base;
    float env=d>0.?exp(-d*(3.+fi*.9)):exp(d*26.);
    float rays=.4+.6*noise(vec2(x*13.+fi*5.,t*.33+d*1.8));
    rays*=.55+.45*noise(vec2(x*38.,t*.8));
    float br=.6+.4*sin(t*.21+fi*1.9+flow*4.);
    vec3 c1=vec3(.08,1.,.5),c2=vec3(.12,.72,1.),c3=vec3(.68,.32,1.);
    vec3 c=mix(c1,mix(c2,c3,smoothstep(.1,.75,d)),smoothstep(-.02,.28,d));
    col+=c*env*rays*br*(.95-fi*.2);
  }
  o=vec4(col,1.);
}`;

const FRAG_SCENE = `#version 300 es
precision highp float;
in vec2 vUv;out vec4 o;
uniform sampler2D uAur,uAtlas,uUI;
uniform vec2 uRes,uCell,uKeepC,uKeepH;
uniform float uT,uHasUI,uN,uScale;
uniform vec4 uC[3];   // x, y (pixels, from the top left), width, height
uniform vec4 uB[3];   // angle, stretch, visible
float hash21(vec2 p){p=fract(p*vec2(233.34,851.73));p+=dot(p,p+23.45);return fract(p.x*p.y);}
float sdRound(vec2 p,vec2 b,float r){vec2 q=abs(p)-b+r;return length(max(q,0.))+min(max(q.x,q.y),0.)-r;}
vec3 caret(int i,vec2 pd,float t){
  vec4 c=uC[i],b=uB[i];
  vec2 p=pd-c.xy;
  float ca=cos(b.x),sa=sin(b.x);
  p=vec2(ca*p.x+sa*p.y,-sa*p.x+ca*p.y);
  p.x/=(1.+b.y);p.y*=sqrt(1.+b.y);
  p=vec2(ca*p.x-sa*p.y,sa*p.x+ca*p.y);
  vec2 hb=c.zw*.5;float s=uScale;vec3 col=vec3(0.);
  if(dot(p,p)>3600.*s*s)return col;
  if(i==0){            // the CRT's: a phosphor block, sliced by scanlines
    float d=sdRound(p,hb,2.*s);
    float body=smoothstep(1.,-1.,d);
    float scan=.62+.38*step(.5,fract(p.y/(3.*s)));
    float glow=exp(-max(d,0.)/(11.*s))*.6;
    col=vec3(.35,1.,.55)*(body*scan*1.15+glow)*(.93+.07*sin(t*57.));
  }else if(i==1){      // the flame: narrow at the top, licking
    float h=clamp(p.y/hb.y,-1.,1.);
    vec2 q=p;q.x+=sin(p.y/s*.34-t*15.)*2.2*s*(.5-h*.5);
    float w=1.-.55*(.5-h*.5);
    float d=sdRound(vec2(q.x/w,q.y),hb,4.*s)*w;
    float body=smoothstep(1.,-1.,d);
    float fl=.8+.2*sin(t*23.+hash21(vec2(floor(t*9.),1.))*6.);
    vec3 hot=mix(vec3(1.,.3,.04),vec3(1.,.92,.55),smoothstep(.9,-1.,h));
    col=hot*body*1.1+vec3(1.,.42,.1)*exp(-max(d,0.)/(13.*s))*.75*fl;
  }else{               // the slime: a wet blob with a highlight
    float d=sdRound(p,hb,hb.x*.92+1.1*s*sin(t*3.7));
    float body=smoothstep(1.,-1.,d);
    float hl=smoothstep(hb.x*.6,0.,length(p-vec2(-hb.x*.3,-hb.y*.5)))*.8;
    float rim=smoothstep(-4.*s,0.,d);
    col=vec3(.18,.95,.72)*(body*(.6+.35*rim)+exp(-max(d,0.)/(12.*s))*.5)+vec3(1.)*hl*body;
  }
  return col*b.z;
}
void main(){
  vec2 pd=vec2(vUv.x*uRes.x,(1.-vUv.y)*uRes.y);
  vec3 col=mix(vec3(.008,.02,.035),vec3(.014,.04,.05),vUv.y);
  // the aurora, smooth, and read out as characters
  vec3 a=texture(uAur,vUv).rgb;
  vec2 ci=floor(pd/uCell);
  vec2 cuv=(ci+.5)*uCell/uRes;cuv.y=1.-cuv.y;
  vec3 ac=texture(uAur,cuv).rgb;
  float L=clamp(dot(ac,vec3(.3,.59,.11))*1.5,0.,1.);
  float idx=floor(pow(L,.85)*(uN-1.)+.5);
  if(hash21(ci+floor(uT*6.))>.985)idx=min(idx+1.,uN-1.);
  vec2 loc=fract(pd/uCell);
  float cov=texture(uAtlas,vec2((idx+loc.x)/uN,loc.y)).r;
  vec3 hue=ac/max(max(ac.r,max(ac.g,ac.b)),.001);
  // the page is darkened under itself, so that its text reads: the light and its characters give way there
  vec2 q=abs(pd-uKeepC)-uKeepH;
  float sd=length(max(q,0.))+min(max(q.x,q.y),0.);
  float under=1.-smoothstep(0.,150.*uScale,sd);
  col+=(a*.95+hue*cov*(.22+1.6*L))*(1.-.86*under);
  for(int i=0;i<3;i++)col+=caret(i,pd,uT);
  if(uHasUI>.5){vec4 ui=texture(uUI,vec2(vUv.x,1.-vUv.y));col=mix(col,ui.rgb,ui.a)+ui.rgb*ui.a*.08;}
  o=vec4(col,1.);
}`;

const FRAG_BRIGHT = `#version 300 es
precision highp float;
in vec2 vUv;out vec4 o;uniform sampler2D uS;uniform vec2 uTexel;
void main(){
  vec3 c=(texture(uS,vUv+uTexel*vec2(-1,-1)).rgb+texture(uS,vUv+uTexel*vec2(1,-1)).rgb+texture(uS,vUv+uTexel*vec2(-1,1)).rgb+texture(uS,vUv+uTexel*vec2(1,1)).rgb)*.25;
  float l=max(c.r,max(c.g,c.b));o=vec4(c*smoothstep(.38,.85,l),1.);
}`;

const FRAG_BLUR = `#version 300 es
precision highp float;
in vec2 vUv;out vec4 o;uniform sampler2D uS;uniform vec2 uDir;
void main(){
  float w[5]=float[](.227027,.1945946,.1216216,.054054,.016216);
  vec3 c=texture(uS,vUv).rgb*w[0];
  for(int i=1;i<5;i++){c+=texture(uS,vUv+uDir*float(i)).rgb*w[i];c+=texture(uS,vUv-uDir*float(i)).rgb*w[i];}
  o=vec4(c,1.);
}`;

const FRAG_FINAL = `#version 300 es
precision highp float;
in vec2 vUv;out vec4 o;
uniform sampler2D uScene,uBloomA,uBloomC;
uniform vec2 uRes;uniform float uT,uOn,uScale;
float hash(vec2 p){p=fract(p*vec2(443.897,441.423));p+=dot(p,p+19.19);return fract(p.x*p.y);}
void main(){
  vec2 cc=vUv*2.-1.;
  float r2=dot(cc,cc);
  cc*=1.+.032*r2;                                   // a little barrel
  vec2 uv=cc*.5+.5;
  if(uv.x<0.||uv.x>1.||uv.y<0.||uv.y>1.){o=vec4(0.,0.,0.,1.);return;}
  vec2 off=cc*.0016;                                // colour fringes grow toward the edge
  vec3 sc=vec3(texture(uScene,uv+off).r,texture(uScene,uv).g,texture(uScene,uv-off).b);
  vec3 bl=texture(uBloomA,uv).rgb*.85+texture(uBloomC,uv).rgb*.9;
  vec3 col=sc+bl;
  float y=gl_FragCoord.y;
  float sl=.5+.5*sin(y*6.2831853/max(3.*uScale,2.));
  col*=1.-.17*(1.-sl);                              // scanlines
  float xm=mod(floor(gl_FragCoord.x/uScale),3.);
  col*=1.+.05*vec3(xm==0.?1.:-.5,xm==1.?1.:-.5,xm==2.?1.:-.5); // the mask
  col*=1.+.045*smoothstep(.0,.06,fract(uv.y-uT*.06))*smoothstep(.2,.06,fract(uv.y-uT*.06)); // a slow roll
  col*=1.+.012*sin(uT*113.);
  col+=(hash(gl_FragCoord.xy+uT*61.)-.5)*.045;      // grain
  col*=1.-.5*pow(r2*.5,1.3);                        // vignette
  // power on: a line opens into the picture
  float h=smoothstep(.12,.9,uOn);float band=mix(.006,1.,h*h);
  float inb=1.-smoothstep(band,band+.02,abs(cc.y));
  col=col*inb+vec3(.6,1.,.8)*exp(-uOn*3.5)*.6*inb;
  o=vec4(pow(max(col,0.),vec3(.95)),1.);
}`;

const JS = `
(()=>{
const PERF=location.search.includes("perf");
const reduce=matchMedia("(prefers-reduced-motion: reduce)").matches;
const root=document.documentElement,cvs=document.getElementById("fx"),ui=document.getElementById("ui"),keep=document.getElementById("keep");
// ---- the form
const f=document.getElementById("f"),msg=document.getElementById("msg"),go=document.getElementById("go"),slot=document.getElementById("slot"),inp=document.getElementById("email");
f.addEventListener("submit",async(e)=>{
  e.preventDefault();msg.className="msg";msg.textContent="";go.disabled=true;
  try{
    const t=document.querySelector("[name=cf-turnstile-response]");
    const r=await fetch("/api/waitlist",{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({email:f.email.value,turnstile:t?t.value:""})});
    const j=await r.json().catch(()=>({}));
    if(r.ok){
      slot.replaceChildren();const d=document.createElement("div");d.className="done";d.setAttribute("role","status");
      d.innerHTML='<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="square" aria-hidden="true"><path d="M4 12.5l5 5L20 6.5"/></svg>';
      d.append("Listedesin. Onaylanınca e-postana bir davet kodu gelecek.");slot.append(d);
    }else{msg.className="msg bad";msg.textContent=j.error||"Olmadı, sonra tekrar dene.";if(window.turnstile)turnstile.reset();go.disabled=false;}
  }catch{msg.className="msg bad";msg.textContent="Bağlantı kurulamadı.";go.disabled=false;}
});
// ---- the name is typed, and its caret stays at the end
const word=document.getElementById("w"),full=word.dataset.t;
if(reduce){word.textContent=full;}else{let i=0;word.textContent="";const tick=()=>{word.textContent=full.slice(0,++i);if(i<full.length)setTimeout(tick,110+Math.random()*80);};setTimeout(tick,900);}

// ---- the picture
const gl=cvs.getContext("webgl2",{alpha:false,antialias:false,powerPreference:"high-performance"});
if(!gl){root.classList.add("nogl");cvs.remove();return;}
const hasUI=!location.search.includes("nohtml")&&("layoutSubtree" in cvs)&&("onpaint" in cvs)&&typeof gl.texElementSubImage2D==="function";
if(hasUI){cvs.setAttribute("layoutsubtree","");cvs.append(ui);ui.classList.add("in-canvas");}else root.classList.add("nofx");

const sh=(type,src)=>{const s=gl.createShader(type);gl.shaderSource(s,src);gl.compileShader(s);if(!gl.getShaderParameter(s,gl.COMPILE_STATUS))throw new Error(gl.getShaderInfoLog(s));return s;};
const prog=(frag)=>{const p=gl.createProgram();gl.attachShader(p,sh(gl.VERTEX_SHADER,VERT));gl.attachShader(p,sh(gl.FRAGMENT_SHADER,frag));gl.linkProgram(p);if(!gl.getProgramParameter(p,gl.LINK_STATUS))throw new Error(gl.getProgramInfoLog(p));
  const loc={};p.u=(n)=>loc[n]??(loc[n]=gl.getUniformLocation(p,n));return p;};
let pAur,pScene,pBright,pBlur,pFinal;
try{pAur=prog(FRAG_AURORA);pScene=prog(FRAG_SCENE);pBright=prog(FRAG_BRIGHT);pBlur=prog(FRAG_BLUR);pFinal=prog(FRAG_FINAL);}
catch(e){console.error(e);root.classList.remove("nofx");root.classList.add("nogl");if(hasUI){document.body.append(ui);ui.classList.remove("in-canvas");}cvs.remove();return;}
gl.bindVertexArray(gl.createVertexArray());

const tex=(w,h,filter=gl.LINEAR)=>{const t=gl.createTexture();gl.bindTexture(gl.TEXTURE_2D,t);gl.texImage2D(gl.TEXTURE_2D,0,gl.RGBA,w,h,0,gl.RGBA,gl.UNSIGNED_BYTE,null);
  gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MIN_FILTER,filter);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MAG_FILTER,filter);
  gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_WRAP_S,gl.CLAMP_TO_EDGE);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_WRAP_T,gl.CLAMP_TO_EDGE);return t;};
const target=(w,h)=>{const t=tex(w,h);const fb=gl.createFramebuffer();gl.bindFramebuffer(gl.FRAMEBUFFER,fb);gl.framebufferTexture2D(gl.FRAMEBUFFER,gl.COLOR_ATTACHMENT0,gl.TEXTURE_2D,t,0);return {t,fb,w,h};};

// the characters, by weight
const RAMP=" .:-~=+*xX#%@";
const atlas=(()=>{const cw=20,ch=36,c=document.createElement("canvas");c.width=cw*RAMP.length;c.height=ch;const x=c.getContext("2d");
  x.fillStyle="#000";x.fillRect(0,0,c.width,ch);x.fillStyle="#fff";x.font="700 30px ui-monospace,Menlo,Consolas,monospace";x.textAlign="center";x.textBaseline="middle";
  for(let i=0;i<RAMP.length;i++)x.fillText(RAMP[i],cw*i+cw/2,ch/2+1);
  const t=gl.createTexture();gl.bindTexture(gl.TEXTURE_2D,t);gl.texImage2D(gl.TEXTURE_2D,0,gl.RGBA,gl.RGBA,gl.UNSIGNED_BYTE,c);
  gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MIN_FILTER,gl.LINEAR);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MAG_FILTER,gl.LINEAR);
  gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_WRAP_S,gl.CLAMP_TO_EDGE);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_WRAP_T,gl.CLAMP_TO_EDGE);return t;})();

let quality=1,W=0,H=0,scale=1,cssW=0,cssH=0,aur,scn,bA,bB,cA,cB,uiTex;
function size(){
  cssW=Math.max(1,innerWidth);cssH=Math.max(1,innerHeight);
  scale=Math.min(devicePixelRatio||1,2)*quality;
  while(cssW*cssH*scale*scale>3.4e6&&scale>.5)scale-=.1;
  const nw=Math.round(cssW*scale),nh=Math.round(cssH*scale);
  if(nw===W&&nh===H)return;
  W=cvs.width=nw;H=cvs.height=nh;
  ui.style.setProperty("--w",cssW+"px");ui.style.setProperty("--h",cssH+"px");
  aur=target(Math.ceil(W/4),Math.ceil(H/4));scn=target(W,H);
  bA=target(Math.ceil(W/4),Math.ceil(H/4));bB=target(bA.w,bA.h);cA=target(Math.ceil(W/8),Math.ceil(H/8));cB=target(cA.w,cA.h);
  if(hasUI){uiTex=tex(W,H);}
  gl.bindFramebuffer(gl.FRAMEBUFFER,null);
  if(hasUI)cvs.requestPaint();
}
// where the page is drawn is told to the browser, or it takes no clicks: the canvas is in device pixels, the page in CSS ones
function fixGeometry(){
  if(!hasUI)return;
  const probe=()=>{const r=inp.isConnected?inp.getBoundingClientRect():keep.getBoundingClientRect();const e=document.elementFromPoint(r.left+r.width/2,r.top+r.height/2);return !!e&&ui.contains(e);};
  for(const m of [new DOMMatrix([scale,0,0,scale,0,0]),new DOMMatrix()]){
    try{cvs.updateElementGeometry(ui,{canvasTransform:m});}catch(e){continue;}
    if(probe())return;
  }
}
function uploadUI(){
  if(!hasUI||!uiTex)return;
  gl.activeTexture(gl.TEXTURE2);gl.bindTexture(gl.TEXTURE_2D,uiTex);
  try{gl.texElementSubImage2D(gl.TEXTURE_2D,0,0,0,ui,{width:W,height:H});}catch(e){}
}
if(hasUI){cvs.onpaint=()=>{uploadUI();fixGeometry();if(reduce)draw(performance.now());};}

// ---- the carets
const kinds=[{w:15,h:30},{w:11,h:32},{w:14,h:28}];
let px=-1,py=-1,pmove=0;
addEventListener("pointermove",(e)=>{px=e.clientX;py=e.clientY;pmove=performance.now();},{passive:true});
function keepRect(){const r=keep.getBoundingClientRect();return {l:r.left-34,t:r.top-34,r:r.right+34,b:r.bottom+34};}
function pick(){const k=keepRect();for(let n=0;n<30;n++){const x=cssW*(.04+Math.random()*.92),y=cssH*(.06+Math.random()*.88);if(!(x>k.l&&x<k.r&&y>k.t&&y<k.b))return [x,y];}return [cssW*.1,cssH*.1];}
const cs=kinds.map((d,n)=>{size();const [x,y]=pick();return {d,x,y,vx:0,vy:0,tx:x,ty:y,until:0,rest:true,s:0,a:0,born:performance.now()+500+n*450,seed:Math.random()*10};});
const uC=new Float32Array(12),uB=new Float32Array(12);
function stepCarets(now,dt){
  for(let i=0;i<cs.length;i++){const c=cs[i];
    const following=i===1&&px>=0&&now-pmove<2200;
    if(following){c.tx=Math.min(cssW-8,Math.max(8,px+18));c.ty=Math.min(cssH-8,Math.max(8,py-6));}
    else if(now>c.until){[c.tx,c.ty]=pick();c.until=now+1500+Math.random()*2600;}
    const k=following?26:15,damp=following?8.5:6.2;
    c.vx+=((c.tx-c.x)*k-c.vx*damp)*dt;c.vy+=((c.ty-c.y)*k-c.vy*damp)*dt;c.x+=c.vx*dt;c.y+=c.vy*dt;
    const sp=Math.hypot(c.vx,c.vy);c.s=Math.min(.7,sp/1100);if(sp>20)c.a=Math.atan2(c.vy,c.vx);c.rest=sp<14;
    const blink=c.rest?(Math.floor((now/525)+c.seed)%2===0?1:0):1;
    const born=Math.min(1,Math.max(0,(now-c.born)/500));
    uC.set([c.x*scale,c.y*scale,c.d.w*scale,c.d.h*scale],i*4);uB.set([c.a,c.s,blink*born,0],i*4);
  }
}

// ---- one frame
const t0=performance.now();let last=t0;
function quad(){gl.drawArrays(gl.TRIANGLES,0,3);}
function pass(p,fbo,w,h){gl.useProgram(p);gl.bindFramebuffer(gl.FRAMEBUFFER,fbo?fbo.fb:null);gl.viewport(0,0,w,h);}
function bind(unit,t){gl.activeTexture(gl.TEXTURE0+unit);gl.bindTexture(gl.TEXTURE_2D,t);}
function draw(now){
  const dt=Math.min(.033,(now-last)/1000);last=now;
  const T=reduce?14:(now-t0)/1000;
  if(!reduce)stepCarets(now,dt);else{for(let i=0;i<3;i++){const c=cs[i];uC.set([c.x*scale,c.y*scale,c.d.w*scale,c.d.h*scale],i*4);uB.set([0,0,1,0],i*4);}}
  const kr=keep.getBoundingClientRect();
  pass(pAur,aur,aur.w,aur.h);gl.uniform2f(pAur.u("uRes"),aur.w,aur.h);gl.uniform1f(pAur.u("uT"),T);quad();
  pass(pScene,scn,W,H);
  bind(0,aur.t);bind(1,atlas);bind(2,hasUI?uiTex:atlas);
  gl.uniform1i(pScene.u("uAur"),0);gl.uniform1i(pScene.u("uAtlas"),1);gl.uniform1i(pScene.u("uUI"),2);
  gl.uniform2f(pScene.u("uRes"),W,H);gl.uniform2f(pScene.u("uCell"),8*scale,15*scale);gl.uniform1f(pScene.u("uN"),RAMP.length);
  gl.uniform1f(pScene.u("uT"),T);gl.uniform1f(pScene.u("uHasUI"),hasUI?1:0);gl.uniform1f(pScene.u("uScale"),scale);
  gl.uniform2f(pScene.u("uKeepC"),(kr.left+kr.width/2)*scale,(kr.top+kr.height/2)*scale);gl.uniform2f(pScene.u("uKeepH"),kr.width/2*scale,kr.height/2*scale);
  gl.uniform4fv(pScene.u("uC"),uC);gl.uniform4fv(pScene.u("uB"),uB);quad();
  // bloom: bright parts, small and smaller, blurred
  pass(pBright,bA,bA.w,bA.h);bind(0,scn.t);gl.uniform1i(pBright.u("uS"),0);gl.uniform2f(pBright.u("uTexel"),1/W,1/H);quad();
  const blur=(src,dst,dx,dy)=>{pass(pBlur,dst,dst.w,dst.h);bind(0,src.t);gl.uniform1i(pBlur.u("uS"),0);gl.uniform2f(pBlur.u("uDir"),dx/src.w,dy/src.h);quad();};
  blur(bA,bB,1.6,0);blur(bB,bA,0,1.6);
  pass(pBright,cA,cA.w,cA.h);bind(0,bA.t);gl.uniform1i(pBright.u("uS"),0);gl.uniform2f(pBright.u("uTexel"),1/bA.w,1/bA.h);quad();
  blur(cA,cB,2.2,0);blur(cB,cA,0,2.2);blur(cA,cB,2.2,0);blur(cB,cA,0,2.2);
  pass(pFinal,null,W,H);bind(0,scn.t);bind(1,bA.t);bind(2,cA.t);
  gl.uniform1i(pFinal.u("uScene"),0);gl.uniform1i(pFinal.u("uBloomA"),1);gl.uniform1i(pFinal.u("uBloomC"),2);
  gl.uniform2f(pFinal.u("uRes"),W,H);gl.uniform1f(pFinal.u("uT"),T);gl.uniform1f(pFinal.u("uScale"),scale);gl.uniform1f(pFinal.u("uOn"),reduce?1:Math.min(1,T/1.1));quad();
}
let lastPaint=0;
// how long the graphics card takes for a frame, when it can say (a timer query): the picture is made smaller only
// if that is long. How often frames come says nothing: a machine may show only 24 a second, whatever is drawn
const tq=gl.getExtension("EXT_disjoint_timer_query_webgl2");
let qNow=null,qDone=null,gpuMs=0,evalAt=performance.now()+3000;
function gpuBegin(){if(!tq||qNow||qDone)return;qNow=gl.createQuery();gl.beginQuery(tq.TIME_ELAPSED_EXT,qNow);}
function gpuEnd(){if(qNow){gl.endQuery(tq.TIME_ELAPSED_EXT);qDone=qNow;qNow=null;}}
function gpuPoll(){
  if(!qDone||!gl.getQueryParameter(qDone,gl.QUERY_RESULT_AVAILABLE))return;
  if(!gl.getParameter(tq.GPU_DISJOINT_EXT)){const ms=gl.getQueryParameter(qDone,gl.QUERY_RESULT)/1e6;gpuMs=gpuMs?gpuMs*.9+ms*.1:ms;}
  gl.deleteQuery(qDone);qDone=null;
}
function adapt(now){
  gpuPoll();
  if(!tq||now<evalAt||(PERF&&!location.search.includes("adapt")))return;
  evalAt=now+1500;
  if(gpuMs>14&&quality>.5){quality=Math.max(.5,quality*.8);size();gpuMs=0;}   // it only ever goes down: no going back and forth
}
function loop(now){
  adapt(now);
  // the native caret of the field blinks without a change the page would report: ask for a new picture now and then
  if(hasUI&&document.activeElement===inp&&now-lastPaint>200){lastPaint=now;cvs.requestPaint();}
  if(PERF){
    // ?perf: frames drawn, the size of the picture, and what a frame costs the graphics card (window.__fx)
    gpuBegin();draw(now);gpuEnd();
    const st=(window.__fx??={frames:0});st.frames++;st.q=quality;st.w=W;st.h=H;st.gpu=gpuMs;
  }else{gpuBegin();draw(now);gpuEnd();}
  requestAnimationFrame(loop);
}
size();
new ResizeObserver(()=>{size();if(reduce)draw(performance.now());}).observe(document.documentElement);
addEventListener("resize",()=>{size();if(reduce)draw(performance.now());});
cvs.addEventListener("webglcontextlost",(e)=>e.preventDefault());
cvs.addEventListener("webglcontextrestored",()=>location.reload());
if(hasUI)cvs.requestPaint();
if(reduce){draw(performance.now());}else requestAnimationFrame(loop);
})();
`;

export function landingHTML(n, site, contact, esc) {
  const consts = `const VERT=${JSON.stringify(VERT)},FRAG_AURORA=${JSON.stringify(FRAG_AURORA)},FRAG_SCENE=${JSON.stringify(FRAG_SCENE)},FRAG_BRIGHT=${JSON.stringify(FRAG_BRIGHT)},FRAG_BLUR=${JSON.stringify(FRAG_BLUR)},FRAG_FINAL=${JSON.stringify(FRAG_FINAL)};`;
  return `<!doctype html><html lang="tr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>Ryudit</title><meta name="description" content="Ryudit erken erişimde: bekleme listesine yazıl."><meta name="color-scheme" content="dark">
<style>${CSS}</style></head><body>
<canvas id="fx"></canvas>
<main id="ui"><div id="keep">
<h1 aria-label="Ryudit"><span id="w" data-t="Ryudit" aria-hidden="true">Ryudit</span><span class="tc" aria-hidden="true"></span></h1>
<p class="lead">Bloklar, koleksiyonlar ve yerel öncelikli senkron. Erken erişimde: listeye yazıl, sıran gelince davet kodunu gönderelim.</p>
<div id="slot" class="slot">
<form id="f" novalidate><input id="email" name="email" type="email" inputmode="email" autocomplete="email" placeholder="e-posta adresin" aria-label="E-posta" required maxlength="254"><button id="go" type="submit">listeye yazıl</button></form>
</div>
<div id="msg" class="msg" role="status" aria-live="polite"></div>
</div>
<footer>E-postan yalnızca davet kodunu göndermek için kullanılır, başka yerle paylaşılmaz.${contact}</footer>
</main>
${site ? `<div class="cf-turnstile" data-sitekey="${esc(site)}" data-appearance="interaction-only"></div><script src="https://challenges.cloudflare.com/turnstile/v0/api.js" async defer></script>` : ""}
<script nonce="${n}">${consts}${JS}</script></body></html>`;
}
