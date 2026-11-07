// Accounts: the waitlist, invite codes, registration, login, devices, and the administrator's side.
// See docs/22-hesap-ve-bekleme-listesi.md. The password never reaches this server: the app derives from it
// an "auth" value (what proves who one is) and a key that seals the data key ("wrapped"); the server keeps a
// hash of the first and the sealed bytes of the second, and cannot open either.
const enc = new TextEncoder();
const DAY = 86400;
export const DEFAULT_QUOTA = 200 * 1024 * 1024;
const CODE_ALPHABET = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"; // no 0 O 1 I L
const CODE_DAYS = 14;

const now = () => Math.floor(Date.now() / 1000);
const json = (o, status = 200) => Response.json(o, { status });
const fail = (status, error) => json({ error }, status);

export async function sha256hex(s) {
  const d = await crypto.subtle.digest("SHA-256", typeof s === "string" ? enc.encode(s) : s);
  return [...new Uint8Array(d)].map((b) => b.toString(16).padStart(2, "0")).join("");
}
export function same(a, b) {
  if (typeof a !== "string" || typeof b !== "string" || a.length !== b.length) return false;
  let d = 0;
  for (let i = 0; i < a.length; i++) d |= a.charCodeAt(i) ^ b.charCodeAt(i);
  return d === 0;
}
function randomHex(n) {
  return [...crypto.getRandomValues(new Uint8Array(n))].map((b) => b.toString(16).padStart(2, "0")).join("");
}
// RYU-XXXXXX, every letter equally likely (bytes that would make some letters likelier are thrown away)
export function newCode() {
  let s = "";
  while (s.length < 6) {
    for (const b of crypto.getRandomValues(new Uint8Array(12))) {
      if (b < 248 && s.length < 6) s += CODE_ALPHABET[b % 31];
    }
  }
  return "RYU-" + s;
}
// what a person may type: lower case, spaces, the dash or none at all
export function normalizeCode(c) {
  const s = String(c || "").toUpperCase().replace(/[^A-Z0-9]/g, "");
  return s.startsWith("RYU") ? s.slice(3) : s;
}
const codeHash = (c) => sha256hex("ryudit-invite:" + normalizeCode(c));
const authHash = (salt, auth) => sha256hex(salt + ":" + auth);
const validAuth = (s) => typeof s === "string" && /^[0-9a-f]{64}$/.test(s);
const validWrapped = (s) => typeof s === "string" && /^[A-Za-z0-9+/=_-]{20,400}$/.test(s);
const validEmail = (s) => typeof s === "string" && s.length <= 254 && /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s);
const validUser = (s) => typeof s === "string" && /^[a-z0-9][a-z0-9_.-]{2,31}$/.test(s);

const ip = (req) => req.headers.get("cf-connecting-ip") || "unknown";

// A counter in D1: true while the number of tries in the window is within the limit.
export async function allow(env, key, limit, windowSec) {
  const win = Math.floor(now() / windowSec);
  const k = `${key}:${windowSec}`;
  await env.DB.prepare("INSERT INTO hits(key,win,n) VALUES(?,?,1) ON CONFLICT(key,win) DO UPDATE SET n=n+1").bind(k, win).run();
  const r = await env.DB.prepare("SELECT n FROM hits WHERE key=? AND win=?").bind(k, win).first();
  if (Math.random() < 0.02) await env.DB.prepare("DELETE FROM hits WHERE win < ?").bind(Math.floor(now() / windowSec) - 2).run();
  return (r?.n || 0) <= limit;
}
const audit = (env, what) => env.DB.prepare("INSERT INTO audit(at,what) VALUES(?,?)").bind(now(), what).run();

async function readJSON(request, max = 8192) {
  const text = await request.text();
  if (text.length > max) throw new Error("big");
  return JSON.parse(text || "{}");
}

// ---- tokens of devices ----

async function newDevice(env, userId, name) {
  const token = randomHex(24);
  const t = now();
  await env.DB.prepare("INSERT INTO devices(token_hash,user_id,name,created_at,last_seen) VALUES(?,?,?,?,?)")
    .bind(await sha256hex(token), userId, String(name || "").slice(0, 40) || "cihaz", t, t).run();
  return token;
}

// The user of a device's token ({device, user}), or null.
export async function userOfToken(env, token) {
  if (!token) return null;
  const h = await sha256hex(token);
  const r = await env.DB.prepare(
    "SELECT d.token_hash AS dh, d.last_seen AS seen, u.id, u.username, u.quota, u.bytes, u.status FROM devices d JOIN users u ON u.id=d.user_id WHERE d.token_hash=?",
  ).bind(h).first();
  if (!r || r.status !== "active") return null;
  if (now() - r.seen > 600) await env.DB.prepare("UPDATE devices SET last_seen=? WHERE token_hash=?").bind(now(), h).run();
  return { device: r.dh, id: r.id, username: r.username, quota: r.quota, bytes: r.bytes };
}

// ---- mail (Resend) ----

async function sendMail(env, to, subject, text) {
  if (!env.RESEND_API_KEY) return { sent: false, error: "no mail key" };
  try {
    const res = await fetch("https://api.resend.com/emails", {
      method: "POST",
      headers: { authorization: "Bearer " + env.RESEND_API_KEY, "content-type": "application/json" },
      body: JSON.stringify({ from: env.MAIL_FROM || "Ryudit <onboarding@resend.dev>", to: [to], subject, text }),
    });
    if (!res.ok) return { sent: false, error: `mail ${res.status}` };
    return { sent: true };
  } catch (e) {
    return { sent: false, error: String(e) };
  }
}
function inviteMail(code, base) {
  return (
    `Merhaba,\n\nRyudit bekleme listesinden seni onayladık. Davet kodun:\n\n    ${code}\n\n` +
    `Uygulamayı aç, "Hesap aç" ekranına kullanıcı adı, şifre ve bu kodu yaz. Kod ${CODE_DAYS} gün geçerli ve bir kez kullanılır.\n\n` +
    `Şifren sunucuya gönderilmez; unutursan verini kimse geri getiremez, kurtarma anahtarını sakla.\n\n— Ryudit\n${base}\n`
  );
}

async function turnstileOK(env, token, remoteip) {
  if (!env.TURNSTILE_SECRET) return true; // not set up: the other limits stand
  if (!token) return false;
  const body = new URLSearchParams({ secret: env.TURNSTILE_SECRET, response: String(token), remoteip });
  try {
    const r = await fetch("https://challenges.cloudflare.com/turnstile/v0/siteverify", { method: "POST", body });
    return !!(await r.json()).success;
  } catch {
    return false;
  }
}

// ---- deleting an account's files ----

export async function deleteAccountFiles(env, userId) {
  const prefix = `u${userId}/`;
  let cursor;
  do {
    const page = await env.SYNC.list({ prefix, cursor });
    if (page.objects.length) await env.SYNC.delete(page.objects.map((o) => o.key));
    cursor = page.truncated ? page.cursor : undefined;
  } while (cursor);
}
async function deleteUser(env, userId) {
  await deleteAccountFiles(env, userId);
  await env.DB.batch([
    env.DB.prepare("DELETE FROM devices WHERE user_id=?").bind(userId),
    env.DB.prepare("UPDATE invites SET used_by=NULL WHERE used_by=?").bind(userId),
    env.DB.prepare("DELETE FROM users WHERE id=?").bind(userId),
  ]);
}

// ---- the routes ----

// handleApi answers /api/…; it returns null for what is not its.
export async function handleApi(request, env, url) {
  const path = url.pathname.replace(/\/+$/, "");
  const method = request.method;
  const where = ip(request);
  let body = {};
  if (method === "POST" || method === "DELETE") {
    try {
      body = await readJSON(request);
    } catch {
      return fail(400, "bad request");
    }
  }
  const bearer = (request.headers.get("authorization") || "").replace(/^Bearer\s+/i, "");

  if (path.startsWith("/api/admin")) return admin(request, env, path, method, body, bearer, url);

  // the waitlist: always the same answer, so that the list cannot be read out of it
  if (path === "/api/waitlist" && method === "POST") {
    if (!(await allow(env, "wl:" + where, 5, 3600))) return fail(429, "çok fazla deneme, sonra tekrar dene");
    const email = String(body.email || "").trim().toLowerCase();
    if (!validEmail(email)) return fail(400, "e-posta geçersiz");
    if (!(await turnstileOK(env, body.turnstile, where))) return fail(400, "doğrulama başarısız, sayfayı yenile");
    const name = String(body.name || "").trim().slice(0, 80);
    const r = await env.DB.prepare("INSERT OR IGNORE INTO waitlist(email,name,created_at) VALUES(?,?,?)").bind(email, name, now()).run();
    if (r.meta.changes === 1 && env.ADMIN_EMAIL) await sendMail(env, env.ADMIN_EMAIL, "Ryudit bekleme listesi: yeni kayıt", `${email}${name ? " (" + name + ")" : ""}`);
    return json({ ok: true });
  }

  if (path === "/api/register" && method === "POST") {
    if (!(await allow(env, "reg:" + where, 20, 3600))) return fail(429, "çok fazla deneme, sonra tekrar dene");
    const username = String(body.username || "").trim().toLowerCase();
    if (!validUser(username)) return fail(400, "kullanıcı adı 3-32 karakter: harf, rakam, . _ -");
    if (!validAuth(body.auth) || !validWrapped(body.wrapped)) return fail(400, "bad request");
    const code = normalizeCode(body.code);
    // wrong codes are counted apart and fewer: the code is short, what protects it is that it can only be tried a few times
    if (!(await allow(env, "code:" + where, 10, 3600)) || !(await allow(env, "code:all", 200, 3600))) return fail(429, "çok fazla deneme, sonra tekrar dene");
    const ch = await codeHash(code);
    const t = now();
    const claim = await env.DB.prepare("UPDATE invites SET used_at=? WHERE code_hash=? AND used_at IS NULL AND expires_at>?").bind(t, ch, t).run();
    if (claim.meta.changes !== 1) return fail(400, "davet kodu geçersiz ya da süresi dolmuş");
    const salt = randomHex(16);
    try {
      const u = await env.DB.prepare("INSERT INTO users(username,salt,auth_hash,wrapped_key,quota,created_at) VALUES(?,?,?,?,?,?)")
        .bind(username, salt, await authHash(salt, body.auth), body.wrapped, DEFAULT_QUOTA, t).run();
      await env.DB.prepare("UPDATE invites SET used_by=? WHERE code_hash=?").bind(u.meta.last_row_id, ch).run();
      const token = await newDevice(env, u.meta.last_row_id, body.device);
      await audit(env, `register ${username}`);
      return json({ token, username });
    } catch (e) {
      await env.DB.prepare("UPDATE invites SET used_at=NULL WHERE code_hash=?").bind(ch).run(); // the code is not spent on a taken name
      if (String(e).includes("UNIQUE")) return fail(409, "bu kullanıcı adı alınmış");
      throw e;
    }
  }

  if (path === "/api/login" && method === "POST") {
    const username = String(body.username || "").trim().toLowerCase();
    if (!(await allow(env, "login:" + where, 30, 3600)) || !(await allow(env, "loginu:" + username, 10, 900))) return fail(429, "çok fazla deneme, sonra tekrar dene");
    if (!validAuth(body.auth)) return fail(400, "bad request");
    const u = await env.DB.prepare("SELECT id,salt,auth_hash,wrapped_key,status FROM users WHERE username=?").bind(username).first();
    // a name that is not there costs the same work, and gets the same answer
    const ok = same(await authHash(u?.salt || "x", body.auth), u?.auth_hash || "0".repeat(64));
    if (!u || !ok) return fail(401, "kullanıcı adı ya da şifre yanlış");
    if (u.status !== "active") return fail(403, "hesap askıya alınmış");
    const token = await newDevice(env, u.id, body.device);
    return json({ token, username, wrapped: u.wrapped_key });
  }

  // from here on: a device's token
  const me = await userOfToken(env, bearer);
  if (!me) return fail(401, "oturum yok");

  if (path === "/api/me" && method === "GET") {
    const ds = await env.DB.prepare("SELECT token_hash,name,created_at,last_seen FROM devices WHERE user_id=? ORDER BY created_at").bind(me.id).all();
    return json({
      username: me.username, bytes: me.bytes, quota: me.quota,
      devices: ds.results.map((d) => ({ id: d.token_hash.slice(0, 12), name: d.name, created: d.created_at, seen: d.last_seen, current: d.token_hash === me.device })),
    });
  }
  if (path === "/api/logout" && method === "POST") {
    await env.DB.prepare("DELETE FROM devices WHERE token_hash=?").bind(me.device).run();
    return json({ ok: true });
  }
  if (path.startsWith("/api/devices/") && method === "DELETE") {
    const id = path.slice("/api/devices/".length);
    if (!/^[0-9a-f]{12}$/.test(id)) return fail(400, "bad request");
    await env.DB.prepare("DELETE FROM devices WHERE user_id=? AND substr(token_hash,1,12)=?").bind(me.id, id).run();
    return json({ ok: true });
  }

  // changing the password or deleting the account asks for the auth again: a stolen token is not enough
  if (path === "/api/password" && method === "POST") {
    if (!(await allow(env, "pw:" + me.id, 10, 900))) return fail(429, "çok fazla deneme");
    if (!validAuth(body.auth) || !validAuth(body.newAuth) || !validWrapped(body.newWrapped)) return fail(400, "bad request");
    const u = await env.DB.prepare("SELECT salt,auth_hash FROM users WHERE id=?").bind(me.id).first();
    if (!same(await authHash(u.salt, body.auth), u.auth_hash)) return fail(401, "şifre yanlış");
    const salt = randomHex(16);
    await env.DB.batch([
      env.DB.prepare("UPDATE users SET salt=?,auth_hash=?,wrapped_key=? WHERE id=?").bind(salt, await authHash(salt, body.newAuth), body.newWrapped, me.id),
      env.DB.prepare("DELETE FROM devices WHERE user_id=? AND token_hash<>?").bind(me.id, me.device), // the others sign in again
    ]);
    return json({ ok: true });
  }
  if (path === "/api/account" && method === "DELETE") {
    if (!(await allow(env, "del:" + me.id, 5, 900))) return fail(429, "çok fazla deneme");
    if (!validAuth(body.auth)) return fail(400, "bad request");
    const u = await env.DB.prepare("SELECT salt,auth_hash FROM users WHERE id=?").bind(me.id).first();
    if (!same(await authHash(u.salt, body.auth), u.auth_hash)) return fail(401, "şifre yanlış");
    await deleteUser(env, me.id);
    await audit(env, `delete ${me.username}`);
    return json({ ok: true });
  }
  return null;
}

// ---- the administrator ----

async function admin(request, env, path, method, body, bearer, url) {
  if (!env.ADMIN_TOKEN || !same(await sha256hex(bearer), await sha256hex(env.ADMIN_TOKEN))) {
    await allow(env, "adminfail:" + ip(request), 20, 3600);
    return fail(401, "no");
  }
  if (!(await allow(env, "admin:" + ip(request), 600, 3600))) return fail(429, "çok fazla istek");
  const base = `${url.protocol}//${url.host}`;

  if (path === "/api/admin/waitlist" && method === "GET") {
    const st = url.searchParams.get("status");
    const rows = await env.DB.prepare(
      `SELECT w.id,w.email,w.name,w.status,w.created_at,w.decided_at,
              (SELECT MAX(expires_at) FROM invites i WHERE i.waitlist_id=w.id AND i.used_at IS NULL) AS invite_until,
              (SELECT MAX(used_at) FROM invites i WHERE i.waitlist_id=w.id AND i.used_at IS NOT NULL) AS used_at
       FROM waitlist w ${st ? "WHERE w.status=?" : ""} ORDER BY w.created_at DESC LIMIT 500`,
    ).bind(...(st ? [st] : [])).all();
    return json(rows.results);
  }
  let m = path.match(/^\/api\/admin\/waitlist\/(\d+)\/(approve|resend|reject)$/);
  if (m && method === "POST") {
    const id = Number(m[1]);
    const w = await env.DB.prepare("SELECT id,email,status FROM waitlist WHERE id=?").bind(id).first();
    if (!w) return fail(404, "yok");
    if (m[2] === "reject") {
      await env.DB.batch([
        env.DB.prepare("UPDATE waitlist SET status='rejected',decided_at=? WHERE id=?").bind(now(), id),
        env.DB.prepare("DELETE FROM invites WHERE waitlist_id=? AND used_at IS NULL").bind(id),
      ]);
      await audit(env, `reject ${w.email}`);
      return json({ ok: true });
    }
    const code = newCode();
    const t = now();
    await env.DB.batch([
      env.DB.prepare("DELETE FROM invites WHERE waitlist_id=? AND used_at IS NULL").bind(id), // the older code stops working
      env.DB.prepare("INSERT INTO invites(code_hash,waitlist_id,created_at,expires_at) VALUES(?,?,?,?)").bind(await codeHash(code), id, t, t + CODE_DAYS * DAY),
      env.DB.prepare("UPDATE waitlist SET status='approved',decided_at=? WHERE id=?").bind(t, id),
    ]);
    const mail = await sendMail(env, w.email, "Ryudit davet kodun", inviteMail(code, base));
    await audit(env, `${m[2]} ${w.email} mail=${mail.sent}`);
    // the code is shown here once, whether or not the mail went: it is the fallback, and it cannot be read again
    return json({ ok: true, code, emailed: mail.sent, mailError: mail.error || "" });
  }
  if (path === "/api/admin/users" && method === "GET") {
    const rows = await env.DB.prepare(
      "SELECT id,username,status,quota,bytes,created_at,(SELECT COUNT(*) FROM devices d WHERE d.user_id=users.id) AS devices FROM users ORDER BY created_at DESC LIMIT 500",
    ).all();
    return json(rows.results);
  }
  m = path.match(/^\/api\/admin\/users\/(\d+)\/(disable|enable|delete|quota)$/);
  if (m && method === "POST") {
    const id = Number(m[1]);
    const u = await env.DB.prepare("SELECT username FROM users WHERE id=?").bind(id).first();
    if (!u) return fail(404, "yok");
    if (m[2] === "delete") await deleteUser(env, id);
    else if (m[2] === "quota") {
      const q = Math.floor(Number(body.quota));
      if (!(q >= 0 && q <= 100 * 1024 ** 3)) return fail(400, "bad request");
      await env.DB.prepare("UPDATE users SET quota=? WHERE id=?").bind(q, id).run();
    } else {
      await env.DB.prepare("UPDATE users SET status=? WHERE id=?").bind(m[2] === "disable" ? "disabled" : "active", id).run();
      if (m[2] === "disable") await env.DB.prepare("DELETE FROM devices WHERE user_id=?").bind(id).run();
    }
    await audit(env, `${m[2]} ${u.username}`);
    return json({ ok: true });
  }
  if (path === "/api/admin/audit" && method === "GET") {
    const rows = await env.DB.prepare("SELECT at,what FROM audit ORDER BY id DESC LIMIT 100").all();
    return json(rows.results);
  }
  return fail(404, "yok");
}
