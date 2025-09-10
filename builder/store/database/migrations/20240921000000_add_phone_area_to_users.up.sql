-- add phone_area column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone_area VARCHAR;