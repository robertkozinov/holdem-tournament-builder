-- Former status values cannot be restored; rows receive the legacy default.
ALTER TABLE tournaments ADD COLUMN status TEXT NOT NULL DEFAULT 'created';
