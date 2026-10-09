ALTER TABLE briefs ADD COLUMN project_id uuid UNIQUE REFERENCES projects(id) ON DELETE CASCADE;

INSERT INTO briefs (user_id, project_id)
SELECT user_id, id FROM projects;
