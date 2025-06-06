-- migrations/1_create_users_table.up.sql

-- Enable the uuid-ossp extension if it's not already enabled.
-- This is necessary for the uuid_generate_v4() function, which creates UUIDs.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create the users table to store user authentication details
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), -- Unique identifier for the user, automatically generated as a UUID
    username VARCHAR(50) UNIQUE NOT NULL,           -- Unique username for login, max 50 characters, cannot be null
    email VARCHAR(255) UNIQUE NOT NULL,             -- Unique email for login and contact, max 255 characters, cannot be null
    password_hash VARCHAR(255) NOT NULL,            -- Stores the securely hashed password, cannot be null
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP, -- Timestamp when the user record was created, defaults to current time in UTC
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP  -- Timestamp when the user record was last updated, defaults to current time in UTC
);

-- Create an index on the email column for faster lookups during login, as email will be a common query field
CREATE INDEX idx_users_email ON users (email);

-- Create a function to update the 'updated_at' timestamp automatically
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create a trigger that calls the update_updated_at_column function before every UPDATE on the users table
CREATE TRIGGER update_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();