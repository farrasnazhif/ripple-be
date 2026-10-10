package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/lib/pq"
)

type FinalRefinementRepository interface {
	Get(context.Context, string, string, string) (*entity.FinalOutput, error)
	Claim(context.Context, string, string, *entity.FinalOutput) (bool, error)
	Save(context.Context, *entity.FinalOutput) error
	Accept(context.Context, string, string, string) (*entity.FinalOutput, error)
}
type finalRefinementRepository struct {
	db    *sql.DB
	rates entity.GenerationRates
}

func NewFinalRefinementRepository(db *sql.DB, rates ...entity.GenerationRates) FinalRefinementRepository {
	var configured entity.GenerationRates
	if len(rates) > 0 {
		configured = rates[0]
	}
	return &finalRefinementRepository{db, configured}
}

const finalColumns = `f.id,f.parent_id,f.request_key,f.image_index,f.prompt,f.reason,f.provider_id,f.status,f.accepted,f.created_at,f.brief_version,f.kind,f.job_ids,f.outputs,f.object_key`

func scanFinal(row interface{ Scan(...any) error }) (*entity.FinalOutput, error) {
	f := &entity.FinalOutput{}
	var ids, outputs []byte
	err := row.Scan(&f.ID, &f.ParentID, &f.RequestKey, &f.ImageIndex, &f.Prompt, &f.Reason, &f.ProviderID, &f.Status, &f.Accepted, &f.CreatedAt, &f.BriefVersion, &f.Kind, &ids, &outputs, &f.ObjectKey)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, entity.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(ids, &f.JobIDs); err != nil {
		return nil, err
	}
	err = json.Unmarshal(outputs, &f.Outputs)
	return f, err
}
func (r *finalRefinementRepository) Get(ctx context.Context, user, project, id string) (*entity.FinalOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	return scanFinal(r.db.QueryRowContext(ctx, `SELECT `+finalColumns+` FROM project_final_outputs f JOIN projects p ON p.id=f.project_id WHERE p.id=$1 AND p.user_id=$2 AND (f.id=$3::uuid OR f.request_key=$3::uuid)`, project, user, id))
}
func (r *finalRefinementRepository) Claim(ctx context.Context, user, project string, f *entity.FinalOutput) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	_, version, err := ownedBrief(ctx, tx, user, project)
	if err != nil {
		return false, err
	}
	if f.BriefVersion != version {
		return false, ErrWorkflowConflict
	}
	if f.ParentID == nil && f.Kind == "image" && len(f.JobIDs) == 0 {
		var summary, outputType string
		var points []byte
		err = tx.QueryRowContext(ctx, `SELECT COALESCE(b.summary,''),p.output_type,COALESCE((SELECT jsonb_agg(jsonb_build_object('key',key,'value',value) ORDER BY key) FROM brief_points WHERE brief_id=b.id),'[]') FROM briefs b JOIN projects p ON p.id=b.project_id WHERE p.id=$1`, project).Scan(&summary, &outputType, &points)
		if err != nil {
			return false, err
		}
		if strings.TrimSpace(summary) == "" || (outputType != "" && outputType != "image") {
			return false, ErrWorkflowConflict
		}
		prompt := summary
		if string(points) != "[]" {
			prompt += "\nConfirmed requirements:\n" + string(points)
		}
		if f.Prompt != "" && f.Prompt != prompt {
			return false, ErrWorkflowConflict
		}
		f.Prompt = prompt
		if len([]rune(f.Prompt)) > 10000 {
			return false, ErrWorkflowConflict
		}
		if _, err = tx.ExecContext(ctx, `UPDATE projects SET output_type='image' WHERE id=$1`, project); err != nil {
			return false, err
		}
	}
	if err = validateFinalInputs(ctx, tx, project, f); err != nil {
		return false, err
	}
	ids, _ := json.Marshal(f.JobIDs)
	err = tx.QueryRowContext(ctx, `INSERT INTO project_final_outputs(project_id,parent_id,request_key,image_index,prompt,reason,status,accepted,brief_version,kind,job_ids)VALUES($1,$2,$3,$4,$5,$6,'submitting',false,$7,$8,$9) ON CONFLICT(request_key) DO NOTHING RETURNING id,created_at`, project, f.ParentID, f.RequestKey, f.ImageIndex, f.Prompt, f.Reason, version, f.Kind, string(ids)).Scan(&f.ID, &f.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	var pg *pq.Error
	if errors.As(err, &pg) && pg.Code == "23505" {
		return false, ErrGenerationConflict
	}
	if err != nil {
		return false, err
	}
	operation := f.Kind
	if f.ParentID != nil {
		operation += "_refinement"
	}
	if err = reserveBudget(ctx, tx, r.rates, project, f.ID, operation, f.Reason); err != nil {
		return false, err
	}
	return true, tx.Commit()
}
func (r *finalRefinementRepository) Save(ctx context.Context, f *entity.FinalOutput) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	outputs, err := json.Marshal(f.Outputs)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	err = tx.QueryRowContext(ctx, `UPDATE project_final_outputs SET provider_id=$2,status=$3,outputs=$4 WHERE id=$1 AND status IN ('submitting','unknown','queued','in_progress') RETURNING status`, f.ID, f.ProviderID, f.Status, string(outputs)).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = settleBudget(ctx, tx, f.ID, status, f.SubmissionRejected); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *finalRefinementRepository) Accept(ctx context.Context, user, project, id string) (*entity.FinalOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	_, version, err := ownedBrief(ctx, tx, user, project)
	if err != nil {
		return nil, err
	}
	f, err := scanFinal(tx.QueryRowContext(ctx, `SELECT `+finalColumns+` FROM project_final_outputs f WHERE f.project_id=$1 AND f.id=$2 FOR UPDATE`, project, id))
	if err != nil {
		return nil, err
	}
	if f.BriefVersion != version || f.Status != "completed" || (f.ObjectKey == "" && len(f.Outputs) == 0) {
		return nil, ErrWorkflowConflict
	}
	if err = validateFinalInputs(ctx, tx, project, f); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE project_final_outputs SET accepted=false WHERE project_id=$1 AND kind=$2 AND accepted`, project, f.Kind); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE project_final_outputs SET accepted=true WHERE id=$1`, f.ID); err != nil {
		return nil, err
	}
	f.Accepted = true
	return f, tx.Commit()
}
