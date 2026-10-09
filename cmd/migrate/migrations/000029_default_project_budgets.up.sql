ALTER TABLE project_budgets ALTER COLUMN limit_cents SET DEFAULT 5000;

INSERT INTO project_budgets(project_id)
SELECT id FROM projects
ON CONFLICT(project_id) DO NOTHING;

CREATE FUNCTION create_default_project_budget() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO project_budgets(project_id) VALUES(NEW.id);
 RETURN NEW;
END;
$$;

CREATE TRIGGER projects_default_budget
AFTER INSERT ON projects
FOR EACH ROW EXECUTE FUNCTION create_default_project_budget();
