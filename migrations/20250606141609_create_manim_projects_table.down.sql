-- migrations/2_create_manim_projects_table.down.sql

-- Drop the trigger associated with the manim_projects table
DROP TRIGGER IF EXISTS update_manim_projects_updated_at ON manim_projects;

-- Drop the manim_projects table. IF EXISTS prevents an error if the table doesn't exist.
DROP TABLE IF EXISTS manim_projects;