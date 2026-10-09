CREATE TABLE project_budgets (
 project_id uuid PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
 limit_cents bigint NOT NULL CHECK(limit_cents BETWEEN 0 AND 100000000)
);
CREATE TABLE generation_usage (
 operation_id uuid PRIMARY KEY,
 project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 actor_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 operation text NOT NULL CHECK(operation IN ('image','video','image_refinement','video_refinement')),
 description text NOT NULL,
 estimate_cents bigint NOT NULL CHECK(estimate_cents>0),
 state text NOT NULL DEFAULT 'reserved' CHECK(state IN ('reserved','spent','released')),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX generation_usage_project ON generation_usage(project_id,created_at DESC);
