package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

type ReviewRepository interface {
	Get(context.Context, string, string) (*entity.ProjectReview, error)
	Edit(context.Context, string, string, entity.BriefEdit, entity.ProjectChange) error
	SetType(context.Context, string, string, string) error
	Link(context.Context, string, string, string, []string) error
	Decide(context.Context, string, string, string, string, string, string) error
	SaveFinal(context.Context, string, string, *entity.FinalOutput) error
}
type reviewRepository struct{ db *sql.DB }

func NewReviewRepository(db *sql.DB) ReviewRepository { return &reviewRepository{db} }
func (r *reviewRepository) Get(ctx context.Context, user, project string) (*entity.ProjectReview, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	var payload []byte
	err := r.db.QueryRowContext(ctx, `SELECT jsonb_build_object(
 'output_type',p.output_type,
 'points',COALESCE((SELECT jsonb_agg(jsonb_build_object('key',key,'value',value,'scope',scope) ORDER BY key) FROM brief_points WHERE brief_id=b.id),'[]'),
 'versions',COALESCE((SELECT jsonb_agg(jsonb_build_object('version',version,'raw_text',raw_text,'summary',summary,'points',points,'attachments',attachments,'created_at',created_at) ORDER BY version DESC) FROM brief_versions WHERE brief_id=b.id),'[]'),
 'changes',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',id,'brief_version',brief_version,'note',note,'diffs',diffs,'impacts',impacts,'created_at',created_at) ORDER BY created_at DESC) FROM project_changes WHERE project_id=p.id),'[]'),
 'decisions',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',d.id,'shot_id',d.shot_id,'brief_version',d.brief_version,'action',d.action,'reason',d.reason,'job_id',d.job_id,'created_at',d.created_at) ORDER BY d.created_at DESC) FROM shot_decisions d JOIN storyboard_shots s ON s.id=d.shot_id WHERE s.project_id=p.id),'[]'),
 'finals',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',id,'brief_version',brief_version,'kind',kind,'job_ids',job_ids,'outputs',outputs,'object_key',object_key,'parent_id',parent_id,'request_key',request_key,'image_index',image_index,'prompt',prompt,'reason',reason,'status',status,'accepted',accepted,'created_at',created_at) ORDER BY created_at DESC,id DESC) FROM project_final_outputs WHERE project_id=p.id),'[]')
 ) FROM projects p JOIN briefs b ON b.project_id=p.id WHERE p.id=$1 AND p.user_id=$2`, project, user).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, entity.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// object_key remains server-only; JSON decoding uses a private intermediary.
	var state entity.ProjectReview
	if err = json.Unmarshal(payload, &state); err != nil {
		return nil, err
	}
	var keys struct {
		Finals []struct {
			ObjectKey string `json:"object_key"`
		} `json:"finals"`
	}
	if err = json.Unmarshal(payload, &keys); err != nil {
		return nil, err
	}
	for i, key := range keys.Finals {
		state.Finals[i].ObjectKey = key.ObjectKey
	}
	return &state, nil
}
func ownedBrief(ctx context.Context, tx *sql.Tx, user, project string) (string, int, error) {
	var id string
	var version int
	err := tx.QueryRowContext(ctx, `SELECT b.id,b.version FROM briefs b JOIN projects p ON p.id=b.project_id WHERE p.id=$1 AND p.user_id=$2 FOR UPDATE OF b`, project, user).Scan(&id, &version)
	if errors.Is(err, sql.ErrNoRows) {
		err = entity.ErrNotFound
	}
	return id, version, err
}
func snapshotBrief(ctx context.Context, tx *sql.Tx, id, user string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO brief_versions(brief_id,version,raw_text,summary,points,attachments,actor_id)
 SELECT b.id,b.version,b.raw_text,COALESCE(b.summary,''),COALESCE((SELECT jsonb_agg(jsonb_build_object('key',key,'value',value,'scope',scope) ORDER BY key) FROM brief_points WHERE brief_id=b.id),'[]'),COALESCE((SELECT jsonb_agg(id) FROM brief_attachments WHERE brief_id=b.id AND status='completed' AND NOT removed),'[]'),$2 FROM briefs b WHERE b.id=$1 ON CONFLICT(brief_id,version) DO NOTHING`, id, user)
	return err
}
func (r *reviewRepository) Edit(ctx context.Context, user, project string, input entity.BriefEdit, change entity.ProjectChange) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	id, version, err := ownedBrief(ctx, tx, user, project)
	if err != nil {
		return err
	}
	if version != input.ExpectedVersion {
		return ErrWorkflowConflict
	}
	if err = snapshotBrief(ctx, tx, id, user); err != nil {
		return err
	}
	for _, attachment := range input.RemoveAttachments {
		result, err := tx.ExecContext(ctx, `UPDATE brief_attachments SET removed=true WHERE id=$1 AND brief_id=$2 AND status='completed' AND NOT removed`, attachment, id)
		if err != nil {
			return err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return entity.ErrNotFound
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM brief_points WHERE brief_id=$1`, id); err != nil {
		return err
	}
	for _, point := range input.Points {
		if _, err = tx.ExecContext(ctx, `INSERT INTO brief_points(brief_id,key,value,scope)VALUES($1,$2,$3,$4)`, id, point.Key, point.Value, point.Scope); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE briefs SET raw_text=$2,summary=NULLIF($3,''),version=version+1 WHERE id=$1`, id, input.RawText, input.Summary); err != nil {
		return err
	}
	if err = snapshotBrief(ctx, tx, id, user); err != nil {
		return err
	}
	diffs, _ := json.Marshal(change.Diffs)
	impacts, _ := json.Marshal(change.Impacts)
	if _, err = tx.ExecContext(ctx, `INSERT INTO project_changes(project_id,brief_version,note,diffs,impacts,actor_id)VALUES($1,$2,$3,$4,$5,$6)`, project, version+1, input.Note, string(diffs), string(impacts), user); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *reviewRepository) SetType(ctx context.Context, user, project, kind string) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, _, err = ownedBrief(ctx, tx, user, project); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE projects SET output_type=$3 WHERE id=$1 AND user_id=$2 AND (output_type=$3 OR (NOT EXISTS(SELECT 1 FROM storyboard_shots WHERE project_id=$1) AND NOT EXISTS(SELECT 1 FROM project_final_outputs WHERE project_id=$1 AND status NOT IN ('failed','nsfw','canceled'))))`, project, user, kind)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return ErrWorkflowConflict
	}
	return tx.Commit()
}

func (r *reviewRepository) Link(ctx context.Context, user, project, shot string, keys []string) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	brief, _, err := ownedBrief(ctx, tx, user, project)
	if err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM storyboard_shots WHERE id=$1 AND project_id=$2)`, shot, project).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return entity.ErrNotFound
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM shot_requirements WHERE shot_id=$1`, shot); err != nil {
		return err
	}
	for _, key := range keys {
		result, err := tx.ExecContext(ctx, `INSERT INTO shot_requirements(shot_id,point_key) SELECT $1,key FROM brief_points WHERE brief_id=$2 AND key=$3`, shot, brief, key)
		if err != nil {
			return err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return entity.ErrNotFound
		}
	}
	return tx.Commit()
}
func (r *reviewRepository) Decide(ctx context.Context, user, project, shot, job, action, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, version, err := ownedBrief(ctx, tx, user, project)
	if err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM storyboard_shots WHERE id=$1 AND project_id=$2)`, shot, project).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return entity.ErrNotFound
	}
	var jobID any
	if job != "" {
		var kind, status string
		var based int
		var count int
		err = tx.QueryRowContext(ctx, `SELECT kind,status,brief_version,jsonb_array_length(outputs) FROM generation_jobs WHERE id=$1 AND shot_id=$2 FOR UPDATE`, job, shot).Scan(&kind, &status, &based, &count)
		if errors.Is(err, sql.ErrNoRows) {
			return entity.ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != "completed" || count == 0 {
			return ErrWorkflowConflict
		}
		if action == "accept" && based != version {
			var affected bool
			err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM project_changes c, jsonb_array_elements(c.impacts) i WHERE c.project_id=$1 AND c.brief_version>$2 AND i->>'shot_id'=$3 AND i->>'status'='needs_review')`, project, based, shot).Scan(&affected)
			if err != nil {
				return err
			}
			if affected {
				return ErrWorkflowConflict
			}
		}
		if action == "accept" || action == "keep" {
			if _, err = tx.ExecContext(ctx, `UPDATE generation_jobs SET accepted=false WHERE shot_id=$1 AND kind=$2`, shot, kind); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `UPDATE generation_jobs SET accepted=true WHERE id=$1`, job); err != nil {
				return err
			}
		}
		jobID = job
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO shot_decisions(shot_id,brief_version,action,reason,job_id,actor_id)VALUES($1,$2,$3,$4,$5,$6)`, shot, version, action, reason, jobID, user); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *reviewRepository) SaveFinal(ctx context.Context, user, project string, final *entity.FinalOutput) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, version, err := ownedBrief(ctx, tx, user, project)
	if err != nil {
		return err
	}
	if final.BriefVersion != version {
		return ErrWorkflowConflict
	}
	if err = validateFinalInputs(ctx, tx, project, final); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE project_final_outputs SET accepted=false WHERE project_id=$1 AND kind=$2 AND accepted`, project, final.Kind); err != nil {
		return err
	}
	ids, _ := json.Marshal(final.JobIDs)
	outputs, _ := json.Marshal(final.Outputs)
	if err = tx.QueryRowContext(ctx, `INSERT INTO project_final_outputs(project_id,brief_version,kind,job_ids,outputs,object_key)VALUES($1,$2,$3,$4,$5,$6)RETURNING id,created_at`, project, version, final.Kind, string(ids), string(outputs), final.ObjectKey).Scan(&final.ID, &final.CreatedAt); err != nil {
		return err
	}
	final.Accepted = true
	final.Status = "completed"
	return tx.Commit()
}

func validateFinalInputs(ctx context.Context, tx *sql.Tx, project string, final *entity.FinalOutput) error {
	if final.Kind == "image" && len(final.JobIDs) == 0 {
		var confirmed bool
		err := tx.QueryRowContext(ctx, `SELECT p.output_type='image' AND length(trim(COALESCE(b.summary,'')))>0 FROM projects p JOIN briefs b ON b.project_id=p.id WHERE p.id=$1`, project).Scan(&confirmed)
		if err != nil {
			return err
		}
		if !confirmed {
			return ErrWorkflowConflict
		}
		return nil
	}
	ids, _ := json.Marshal(final.JobIDs)
	var current []byte
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(jsonb_agg(g.id ORDER BY s.position),'[]') FROM generation_jobs g JOIN storyboard_shots s ON s.id=g.shot_id WHERE s.project_id=$1 AND g.kind=$2 AND g.accepted`, project, final.Kind).Scan(&current)
	if err != nil {
		return err
	}
	if string(current) != string(ids) {
		var currentIDs []string
		if err = json.Unmarshal(current, &currentIDs); err != nil {
			return err
		}
		if len(currentIDs) != len(final.JobIDs) {
			return ErrWorkflowConflict
		}
		for i, id := range currentIDs {
			if id != final.JobIDs[i] {
				return ErrWorkflowConflict
			}
		}
	}

	var unsafe bool
	err = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM storyboard_shots WHERE project_id=$1)<>(SELECT count(*) FROM generation_jobs g JOIN storyboard_shots s ON s.id=g.shot_id WHERE s.project_id=$1 AND g.kind=$2 AND g.accepted AND g.status='completed' AND jsonb_array_length(g.outputs)>0) OR EXISTS(SELECT 1 FROM generation_jobs g JOIN storyboard_shots s ON s.id=g.shot_id JOIN project_changes c ON c.project_id=s.project_id, jsonb_array_elements(c.impacts) i WHERE s.project_id=$1 AND g.kind=$2 AND g.accepted AND c.brief_version>g.brief_version AND i->>'shot_id'=s.id::text AND i->>'status'='needs_review' AND NOT EXISTS(SELECT 1 FROM shot_decisions d WHERE d.shot_id=s.id AND d.job_id=g.id AND d.brief_version>=c.brief_version AND d.action IN ('keep','accept')))`, project, final.Kind).Scan(&unsafe)
	if err != nil {
		return err
	}
	if unsafe || len(final.JobIDs) == 0 {
		return ErrWorkflowConflict
	}
	return nil
}
