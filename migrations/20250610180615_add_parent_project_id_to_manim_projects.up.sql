-- migrations/4_add_parent_project_id_to_manim_projects.up.sql

-- Add the new column 'parent_project_id' to the manim_projects table.
-- This column will store the UUID of a parent project if the current project
-- is a sub-part of a larger, decomposed animation.
ALTER TABLE manim_projects
ADD COLUMN parent_project_id UUID REFERENCES manim_projects(id) ON DELETE SET NULL;

-- Add an index on parent_project_id to improve performance
-- when querying for all sub-projects related to a specific parent.
CREATE INDEX idx_manim_projects_parent_id ON manim_projects (parent_project_id);

-- Optionally, you might want to adjust your unique index on (user_id, name)
-- if you foresee sub-projects of different parents having the same name.
-- However, for now, we'll keep the existing unique index, implying that
-- a user cannot have two projects (even sub-projects) with the exact same name.
-- If this constraint becomes problematic (e.g., "Part 1" for multiple complex videos),
-- you might need to reconsider the unique constraint's scope or name generation.