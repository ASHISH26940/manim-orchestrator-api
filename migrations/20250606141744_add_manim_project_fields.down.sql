-- migrations/3_add_manim_project_fields.down.sql

-- Drop the index on render_status
DROP INDEX IF EXISTS idx_manim_projects_render_status;

-- Drop the newly added columns from the manim_projects table
ALTER TABLE manim_projects
DROP COLUMN IF EXISTS prompt,
DROP COLUMN IF EXISTS render_status,
DROP COLUMN IF EXISTS video_url;