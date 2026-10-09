package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/farrasnazhif/ripple-be/internal/entity"
	"os"
	"testing"
	"time"
)

func TestFinalRefinementDatabase(t *testing.T) {
	dsn := os.Getenv("RIPPLE_TEST_DB")
	if dsn == "" {
		t.Skip("set RIPPLE_TEST_DB")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	var user string
	name := fmt.Sprintf("final-refine-%d", time.Now().UnixNano())
	if err = db.QueryRowContext(ctx, `INSERT INTO users(email,username,password)VALUES($1,$2,$3)RETURNING id`, name+"@example.invalid", name, []byte("test-only")).Scan(&user); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, user); err != nil {
			t.Error(err)
		}
	}()
	p, err := NewProjectRepository(db).Create(ctx, user, "Final test", "Source")
	if err != nil {
		t.Fatal(err)
	}
	var shot, job string
	if err = db.QueryRowContext(ctx, `INSERT INTO storyboard_shots(project_id,position,title,prompt)VALUES($1,1,'Shot','Prompt')RETURNING id`, p.ID).Scan(&shot); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO generation_jobs(shot_id,request_key,kind,prompt,brief_version,status,outputs,accepted)VALUES($1,gen_random_uuid(),'image','Prompt',1,'completed','["https://example.com/a.png"]',true)RETURNING id`, shot).Scan(&job); err != nil {
		t.Fatal(err)
	}
	base := &entity.FinalOutput{BriefVersion: 1, Kind: "image", JobIDs: []string{job}, Outputs: []string{"https://example.com/a.png"}}
	reviews := NewReviewRepository(db)
	if err = reviews.SaveFinal(ctx, user, p.ID, base); err != nil {
		t.Fatal(err)
	}
	repo := NewFinalRefinementRepository(db)
	key := p.ID
	candidate := &entity.FinalOutput{ParentID: &base.ID, RequestKey: &key, Prompt: "Warm", Reason: "Director feedback", Kind: "image", BriefVersion: 1, JobIDs: base.JobIDs, Status: "submitting", Outputs: []string{}}
	if _, err = repo.Get(ctx, "00000000-0000-0000-0000-000000000000", p.ID, base.ID); !errors.Is(err, entity.ErrNotFound) {
		t.Fatal("cross-owner read", err)
	}
	if _, err = repo.Claim(ctx, "00000000-0000-0000-0000-000000000000", p.ID, candidate); !errors.Is(err, entity.ErrNotFound) {
		t.Fatal("cross-owner claim", err)
	}
	if claimed, err := repo.Claim(ctx, user, p.ID, candidate); err != nil || !claimed {
		t.Fatal("claim", claimed, err)
	}
	if claimed, err := repo.Claim(ctx, user, p.ID, candidate); err != nil || claimed {
		t.Fatal("duplicate claim", claimed, err)
	}
	other := *candidate
	otherKey := shot
	other.RequestKey = &otherKey
	if _, err = repo.Claim(ctx, user, p.ID, &other); !errors.Is(err, ErrGenerationConflict) {
		t.Fatal("parallel paid edit", err)
	}
	if _, err = repo.Accept(ctx, user, p.ID, candidate.ID); !errors.Is(err, ErrWorkflowConflict) {
		t.Fatal("unfinished final accepted", err)
	}
	candidate.Status = "completed"
	candidate.Outputs = []string{"https://example.com/refined.png"}
	if err = repo.Save(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Accept(ctx, "00000000-0000-0000-0000-000000000000", p.ID, candidate.ID); !errors.Is(err, entity.ErrNotFound) {
		t.Fatal("cross-owner acceptance", err)
	}
	if _, err = repo.Accept(ctx, user, p.ID, candidate.ID); err != nil {
		t.Fatal("accept", err)
	}
	state, err := reviews.Get(ctx, user, p.ID)
	if err != nil || len(state.Finals) != 2 {
		t.Fatal("history", state, err)
	}
	if !state.Finals[0].Accepted || state.Finals[1].Accepted || state.Finals[1].Outputs[0] != base.Outputs[0] {
		t.Fatal("original overwritten", state.Finals)
	}
	// A result cannot be accepted after its brief or source selection changes.
	if _, err = db.ExecContext(ctx, `UPDATE briefs SET version=2 WHERE id=$1`, p.BriefID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Accept(ctx, user, p.ID, candidate.ID); !errors.Is(err, ErrWorkflowConflict) {
		t.Fatal("stale brief accepted", err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE briefs SET version=1 WHERE id=$1`, p.BriefID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE generation_jobs SET accepted=false WHERE id=$1`, job); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Accept(ctx, user, p.ID, candidate.ID); !errors.Is(err, ErrWorkflowConflict) {
		t.Fatal("changed source accepted", err)
	}
	var ids []byte
	if err = db.QueryRowContext(ctx, `SELECT job_ids FROM project_final_outputs WHERE id=$1`, candidate.ID).Scan(&ids); err != nil {
		t.Fatal(err)
	}
	var saved []string
	if err = json.Unmarshal(ids, &saved); err != nil || len(saved) != 1 || saved[0] != job {
		t.Fatal("provenance", saved, err)
	}
}
