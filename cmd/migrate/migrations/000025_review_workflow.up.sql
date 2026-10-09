ALTER TABLE projects ADD COLUMN output_type text NOT NULL DEFAULT '' CHECK(output_type IN ('','image','video','mixed'));
ALTER TABLE brief_attachments ADD COLUMN removed boolean NOT NULL DEFAULT false;
CREATE TABLE brief_versions (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), brief_id uuid NOT NULL REFERENCES briefs(id) ON DELETE CASCADE,
 version integer NOT NULL, raw_text text NOT NULL, summary text NOT NULL DEFAULT '', points jsonb NOT NULL DEFAULT '[]',
 attachments jsonb NOT NULL DEFAULT '[]', actor_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(brief_id,version)
);
INSERT INTO brief_versions(brief_id,version,raw_text,summary,attachments,actor_id) SELECT b.id,b.version,b.raw_text,COALESCE(b.summary,''),COALESCE((SELECT jsonb_agg(a.id) FROM brief_attachments a WHERE a.brief_id=b.id AND a.status='completed'),'[]'),b.user_id FROM briefs b;
CREATE TABLE brief_points (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), brief_id uuid NOT NULL REFERENCES briefs(id) ON DELETE CASCADE,
 key text NOT NULL, value text NOT NULL, scope text NOT NULL CHECK(scope IN ('local','global')), UNIQUE(brief_id,key)
);
CREATE TABLE shot_requirements (
 shot_id uuid NOT NULL REFERENCES storyboard_shots(id) ON DELETE CASCADE,
 point_key text NOT NULL, PRIMARY KEY(shot_id,point_key)
);
CREATE TABLE project_changes (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 brief_version integer NOT NULL, note text NOT NULL, diffs jsonb NOT NULL, impacts jsonb NOT NULL,
 actor_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE shot_decisions (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), shot_id uuid NOT NULL REFERENCES storyboard_shots(id) ON DELETE CASCADE,
 brief_version integer NOT NULL, action text NOT NULL CHECK(action IN ('refine','keep','accept')), reason text NOT NULL,
 job_id uuid REFERENCES generation_jobs(id) ON DELETE CASCADE, actor_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE generation_jobs ADD COLUMN brief_version integer NOT NULL DEFAULT 1;
ALTER TABLE generation_jobs ADD COLUMN accepted boolean NOT NULL DEFAULT false;
CREATE UNIQUE INDEX accepted_output_per_shot ON generation_jobs(shot_id,kind) WHERE accepted;
CREATE TABLE project_final_outputs (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 brief_version integer NOT NULL, kind text NOT NULL CHECK(kind IN ('image','video')), job_ids jsonb NOT NULL,
 outputs jsonb NOT NULL DEFAULT '[]', object_key text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now()
);
UPDATE projects p SET output_type=COALESCE((SELECT CASE WHEN count(DISTINCT g.kind)>1 THEN 'mixed' ELSE min(g.kind) END FROM generation_jobs g JOIN storyboard_shots s ON s.id=g.shot_id WHERE s.project_id=p.id),'') WHERE EXISTS(SELECT 1 FROM storyboard_shots WHERE project_id=p.id);
