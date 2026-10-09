package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/lib/pq"
)

var ErrGenerationConflict = errors.New("a generation is already active for this shot and output type")

type GenerationRepository interface {
	Claim(context.Context, *entity.GenerationJob) (bool, error)
	Save(context.Context, *entity.GenerationJob) error
	Get(context.Context, string, string, string) (*entity.GenerationJob, error)
	List(context.Context, string, string) ([]entity.GenerationJob, error)
}
type generationRepository struct{ db *sql.DB }

func NewGenerationRepository(db *sql.DB) GenerationRepository { return &generationRepository{db} }
func (r *generationRepository) Claim(ctx context.Context, j *entity.GenerationJob) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	err := r.db.QueryRowContext(ctx, `INSERT INTO generation_jobs(shot_id,request_key,kind,prompt,brief_version) SELECT $1,$2,$3,$4,b.version FROM storyboard_shots s JOIN projects p ON p.id=s.project_id JOIN briefs b ON b.project_id=p.id WHERE s.id=$1 AND (p.output_type IN ('','mixed') OR p.output_type=$3) ON CONFLICT(request_key) DO NOTHING RETURNING id,created_at,brief_version`, j.ShotID, j.RequestKey, j.Kind, j.Prompt).Scan(&j.ID, &j.CreatedAt, &j.BriefVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	var pg *pq.Error
	if errors.As(err, &pg) && pg.Code == "23505" {
		return false, ErrGenerationConflict
	}
	return err == nil, err
}
func (r *generationRepository) Save(ctx context.Context, j *entity.GenerationJob) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	outputs, err := json.Marshal(j.Outputs)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE generation_jobs SET provider_id=$2,status=$3,outputs=$4 WHERE id=$1 AND status NOT IN ('completed','failed','nsfw','canceled')`, j.ID, j.ProviderID, j.Status, string(outputs))
	return err
}

const generationColumns = `g.id,g.shot_id,g.request_key,g.kind,g.prompt,g.provider_id,g.status,g.outputs,g.created_at,g.brief_version,g.accepted`

func scanJob(row interface{ Scan(...any) error }) (*entity.GenerationJob, error) {
	j := &entity.GenerationJob{}
	var output []byte
	err := row.Scan(&j.ID, &j.ShotID, &j.RequestKey, &j.Kind, &j.Prompt, &j.ProviderID, &j.Status, &output, &j.CreatedAt, &j.BriefVersion, &j.Accepted)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(output, &j.Outputs)
	return j, err
}
func (r *generationRepository) Get(ctx context.Context, userID, projectID, id string) (*entity.GenerationJob, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	j, err := scanJob(r.db.QueryRowContext(ctx, `SELECT `+generationColumns+` FROM generation_jobs g JOIN storyboard_shots s ON s.id=g.shot_id JOIN projects p ON p.id=s.project_id WHERE p.user_id=$1 AND p.id=$2 AND (g.id=$3::uuid OR g.request_key=$3::uuid)`, userID, projectID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, entity.ErrNotFound
	}
	return j, err
}
func (r *generationRepository) List(ctx context.Context, userID, projectID string) ([]entity.GenerationJob, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `SELECT `+generationColumns+` FROM generation_jobs g JOIN storyboard_shots s ON s.id=g.shot_id JOIN projects p ON p.id=s.project_id WHERE p.user_id=$1 AND p.id=$2 ORDER BY g.created_at`, userID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := []entity.GenerationJob{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *j)
	}
	return jobs, rows.Err()
}
