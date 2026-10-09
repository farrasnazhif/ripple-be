ALTER TABLE project_final_outputs
 ADD COLUMN parent_id uuid REFERENCES project_final_outputs(id) ON DELETE CASCADE,
 ADD COLUMN request_key uuid UNIQUE,
 ADD COLUMN image_index integer NOT NULL DEFAULT 0 CHECK(image_index>=0),
 ADD COLUMN prompt text NOT NULL DEFAULT '',
 ADD COLUMN reason text NOT NULL DEFAULT '',
 ADD COLUMN provider_id text NOT NULL DEFAULT '',
 ADD COLUMN status text NOT NULL DEFAULT 'completed' CHECK(status IN ('submitting','unknown','queued','in_progress','completed','failed','nsfw','canceled')),
 ADD COLUMN accepted boolean NOT NULL DEFAULT true;
WITH ranked AS (
 SELECT id,row_number() OVER(PARTITION BY project_id,kind ORDER BY created_at DESC,id DESC) AS rank FROM project_final_outputs
) UPDATE project_final_outputs f SET accepted=false FROM ranked WHERE f.id=ranked.id AND ranked.rank>1;
CREATE UNIQUE INDEX accepted_final_per_project ON project_final_outputs(project_id,kind) WHERE accepted;
CREATE UNIQUE INDEX active_final_refinement ON project_final_outputs(project_id,kind) WHERE status IN ('submitting','unknown','queued','in_progress');
