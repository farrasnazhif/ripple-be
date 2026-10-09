ALTER TABLE users ALTER COLUMN is_active SET DEFAULT TRUE;
UPDATE users SET is_active = TRUE WHERE is_active = FALSE;
