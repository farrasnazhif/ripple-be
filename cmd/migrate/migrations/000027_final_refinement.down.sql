DELETE FROM project_final_outputs WHERE parent_id IS NOT NULL;
DROP INDEX active_final_refinement;
DROP INDEX accepted_final_per_project;
ALTER TABLE project_final_outputs DROP COLUMN parent_id,DROP COLUMN request_key,DROP COLUMN image_index,DROP COLUMN prompt,DROP COLUMN reason,DROP COLUMN provider_id,DROP COLUMN status,DROP COLUMN accepted;
