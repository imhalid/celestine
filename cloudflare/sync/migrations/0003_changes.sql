-- The change counter of an account: bumped by every write to its files (worker.js, files()), read by GET /changes, which a
-- device asks every few seconds while its window is in front, and syncs when it moved.
ALTER TABLE users ADD COLUMN changes INTEGER NOT NULL DEFAULT 0;
