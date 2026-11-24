-- How the account's password is stretched on the device (auth.js, /api/prelogin; #154): "argon2id$time$memory$threads$salt".
-- Empty for accounts made before, which the app stretches with PBKDF2 until their next new password.
ALTER TABLE users ADD COLUMN kdf TEXT;
