-- migrations/2_create_manim_projects_table.up.sql

-- Create the manim_projects table to store details about each Manim animation project
CREATE TABLE manim_projects (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), -- Unique identifier for the project, auto-generated UUID
    user_id UUID NOT NULL,                          -- Foreign key linking to the users table (owner of the project)
    name VARCHAR(255) NOT NULL,                     -- Name of the Manim project, max 255 characters, cannot be null
    description TEXT,                               -- Optional longer description of the project
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP, -- Timestamp when the project record was created
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,  -- Timestamp when the project record was last updated

    -- Foreign key constraint to link projects to users.
    -- ON DELETE CASCADE means if a user is deleted, all their associated projects are also deleted.
    CONSTRAINT fk_user
        FOREIGN KEY (user_id)
        REFERENCES users (id)
        ON DELETE CASCADE
);

-- Add indexes for frequently queried columns to improve performance
CREATE INDEX idx_manim_projects_user_id ON manim_projects (user_id); -- For fast lookup of all projects by a user
CREATE UNIQUE INDEX idx_manim_projects_user_id_name ON manim_projects (user_id, name); -- Ensures a user cannot have two projects with the exact same name

-- Add a check constraint to ensure the project name is not an empty string
ALTER TABLE manim_projects ADD CONSTRAINT name_not_empty CHECK (name <> '');

-- Create a trigger to automatically update the 'updated_at' timestamp for manim_projects table
CREATE TRIGGER update_manim_projects_updated_at
BEFORE UPDATE ON manim_projects
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column(); -- Reusing the function created in the users migration