-- migrations/1_create_users_table.down.sql

-- Drop the trigger associated with the users table
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- Drop the function that updates the 'updated_at' timestamp
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop the users table. IF EXISTS prevents an error if the table doesn't exist.
DROP TABLE IF EXISTS users;

-- Optionally, drop the uuid-ossp extension. Be cautious if other tables also rely on it.
-- If this is the only table using it, you can uncomment the line below.
-- DROP EXTENSION IF EXISTS "uuid-ossp";