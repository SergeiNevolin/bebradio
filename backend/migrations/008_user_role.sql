-- Add role column to users table.
-- 'user' is the default; 'admin' grants elevated privileges.
ALTER TABLE users ADD COLUMN role VARCHAR(20) NOT NULL DEFAULT 'user';
