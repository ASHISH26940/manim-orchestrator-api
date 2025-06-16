-- migrations/4_add_parent_project_id_to_manim_projects.down.sql

-- Drop the index first before dropping the column.
DROP INDEX IF EXISTS idx_manim_projects_parent_id;

-- Remove the 'parent_project_id' column from the manim_projects table.
ALTER TABLE manim_projects
DROP COLUMN IF EXISTS parent_project_id;