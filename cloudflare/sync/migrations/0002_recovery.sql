-- The hash of what the recovery key proves (auth.js, /api/recover): the key itself is never here, and the server cannot make it.
-- Empty for accounts made before: they set it at their next sign-in (/api/recovery).
ALTER TABLE users ADD COLUMN recovery_hash TEXT;
