CREATE TABLE storyboard_shots (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 position integer NOT NULL CHECK(position > 0),
 title text NOT NULL CHECK(length(title) BETWEEN 1 AND 200),
 prompt text NOT NULL CHECK(length(prompt) BETWEEN 1 AND 10000),
 UNIQUE(project_id, position)
);
CREATE TABLE generation_jobs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 shot_id uuid NOT NULL REFERENCES storyboard_shots(id) ON DELETE CASCADE,
 request_key uuid NOT NULL UNIQUE,
 kind text NOT NULL CHECK(kind IN ('image','video')),
 prompt text NOT NULL,
 provider_id text NOT NULL DEFAULT '',
 status text NOT NULL DEFAULT 'submitting',
 outputs jsonb NOT NULL DEFAULT '[]',
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX generation_jobs_active ON generation_jobs(shot_id,kind) WHERE status IN ('submitting','unknown','queued','in_progress');
