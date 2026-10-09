DROP TRIGGER projects_default_budget ON projects;
DROP FUNCTION create_default_project_budget();
ALTER TABLE project_budgets ALTER COLUMN limit_cents DROP DEFAULT;
