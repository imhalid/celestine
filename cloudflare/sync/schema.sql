-- D1 schema of the account system (docs/22-hesap-ve-bekleme-listesi.md). Apply with deploy.sh / `wrangler d1 execute`.
CREATE TABLE IF NOT EXISTS waitlist (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  email      TEXT NOT NULL UNIQUE,
  name       TEXT NOT NULL DEFAULT '',
  status     TEXT NOT NULL DEFAULT 'pending',   -- pending | approved | rejected
  created_at INTEGER NOT NULL,
  decided_at INTEGER
);
CREATE TABLE IF NOT EXISTS invites (
  code_hash   TEXT PRIMARY KEY,                 -- sha256 of the code: the code itself is not kept
  waitlist_id INTEGER NOT NULL REFERENCES waitlist(id),
  created_at  INTEGER NOT NULL,
  expires_at  INTEGER NOT NULL,
  used_at     INTEGER,
  used_by     INTEGER
);
CREATE TABLE IF NOT EXISTS users (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  username    TEXT NOT NULL UNIQUE COLLATE NOCASE,
  salt        TEXT NOT NULL,
  auth_hash   TEXT NOT NULL,                    -- sha256(salt + auth): "auth" is derived on the device, the password never comes
  wrapped_key TEXT NOT NULL,                    -- the data key, sealed on the device: the server cannot open it
  quota       INTEGER NOT NULL,
  bytes       INTEGER NOT NULL DEFAULT 0,
  status      TEXT NOT NULL DEFAULT 'active',   -- active | disabled
  created_at  INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS devices (
  token_hash TEXT PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id),
  name       TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  last_seen  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS devices_user ON devices(user_id);
CREATE TABLE IF NOT EXISTS hits (key TEXT NOT NULL, win INTEGER NOT NULL, n INTEGER NOT NULL, PRIMARY KEY (key, win));
CREATE TABLE IF NOT EXISTS audit (id INTEGER PRIMARY KEY AUTOINCREMENT, at INTEGER NOT NULL, what TEXT NOT NULL);
