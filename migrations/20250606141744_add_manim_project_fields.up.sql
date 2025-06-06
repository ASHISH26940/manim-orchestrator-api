-- migrations/3_add_manim_project_fields.up.sql

-- Add new columns to the manim_projects table based on the discussed architecture
ALTER TABLE manim_projects
ADD COLUMN prompt TEXT,                         -- The text prompt used to generate Manim code
ADD COLUMN render_status VARCHAR(50) DEFAULT 'pending' NOT NULL, -- Current status of the rendering process (e.g., 'pending', 'rendering', 'completed', 'failed')
ADD COLUMN video_url TEXT;                      -- URL of the rendered video (e.g., from Cloudflare R2)

-- Optional: Add an index on render_status if you'll frequently query projects by their status
CREATE INDEX idx_manim_projects_render_status ON manim_projects (render_status);