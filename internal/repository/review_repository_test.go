package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/farrasnazhif/ripple-be/internal/entity"
	"os"
	"testing"
	"time"
)

func TestReviewWorkflowDatabase(t *testing.T) {
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
	name := fmt.Sprintf("review-%d", time.Now().UnixNano())
	if err = db.QueryRowContext(ctx, `INSERT INTO users(email,username,password)VALUES($1,$2,$3)RETURNING id`, name+"@example.invalid", name, []byte("test-only")).Scan(&user); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, user); err != nil {
			t.Error("test cleanup failed", err)
		}
	}()
	projects := NewProjectRepository(db)
	briefs := NewBriefRepository(db)
	reviews := NewReviewRepository(db)
	shots := NewStoryboardRepository(db)
	jobs := NewGenerationRepository(db)
	p, err := projects.Create(ctx, user, "Review test", "Source")
	if err != nil {
		t.Fatal(err)
	}

	var attachment string
	if err = db.QueryRowContext(ctx, `INSERT INTO brief_attachments(brief_id,object_key,file_name,content_type,size_bytes,status)VALUES($1,$2,'reference.png','image/png',1,'completed')RETURNING id`, p.BriefID, "test/"+p.ID).Scan(&attachment); err != nil {
		t.Fatal(err)
	}
	edit := entity.BriefEdit{ExpectedVersion: 1, RawText: "Original source", Summary: "Confirmed summary", Points: []entity.BriefPoint{{Key: "lighting", Value: "Morning", Scope: "local"}}, Note: "Confirm requirements", RemoveAttachments: []string{attachment}}
	if err = reviews.Edit(ctx, user, p.ID, edit, entity.ProjectChange{Diffs: []entity.PointDiff{}, Impacts: []entity.ShotReview{}}); err != nil {
		t.Fatal(err)
	}
	if err = reviews.Edit(ctx, user, p.ID, edit, entity.ProjectChange{}); !errors.Is(err, ErrWorkflowConflict) {
		t.Fatal("stale edit overwritten", err)
	}
	state, err := reviews.Get(ctx, user, p.ID)
	if err != nil || len(state.Versions) != 2 || state.Versions[0].Summary != "Confirmed summary" {
		t.Fatal("snapshots", state, err)
	}

	if len(state.Versions[1].Attachments) != 1 || state.Versions[1].Attachments[0] != attachment || len(state.Versions[0].Attachments) != 0 {
		t.Fatal("archived attachment snapshot missing", state.Versions)
	}
	active, err := briefs.ListAttachments(ctx, user, p.BriefID)
	if err != nil || len(active) != 0 {
		t.Fatal("removed attachment still active", err)
	}
	if _, err = briefs.GetAttachment(ctx, user, p.BriefID, attachment); err != nil {
		t.Fatal("archive inaccessible", err)
	}
	edit.RemoveAttachments = nil
	if _, err = reviews.Get(ctx, "00000000-0000-0000-0000-000000000000", p.ID); !errors.Is(err, entity.ErrNotFound) {
		t.Fatal("cross-owner review", err)
	}
	if err = reviews.SetType(ctx, user, p.ID, "video"); err != nil {
		t.Fatal(err)
	}
	if err = shots.Create(ctx, user, p.ID, "Confirmed summary", []entity.StoryboardShot{{Title: "One", Prompt: "Garden", RequirementKeys: []string{"lighting"}}, {Title: "Two", Prompt: "Flowers"}}); err != nil {
		t.Fatal(err)
	}
	plan, err := shots.List(ctx, user, p.ID)
	if err != nil || len(plan[0].RequirementKeys) != 1 {
		t.Fatal("requirement links", plan, err)
	}
	ids := []string{}
	for _, shot := range plan {
		var key string
		if err = db.QueryRowContext(ctx, `SELECT gen_random_uuid()`).Scan(&key); err != nil {
			t.Fatal(err)
		}
		j := &entity.GenerationJob{ShotID: shot.ID, RequestKey: key, Kind: "video", Prompt: shot.Prompt, Status: "completed", Outputs: []string{"https://example.com/sample.mp4"}}
		claimed, err := jobs.Claim(ctx, j)
		if err != nil || !claimed || j.BriefVersion != 2 {
			t.Fatal("claim version", j, err)
		}
		if err = jobs.Save(ctx, j); err != nil {
			t.Fatal(err)
		}
		if err = reviews.Decide(ctx, user, p.ID, shot.ID, j.ID, "accept", ""); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, j.ID)
	}
	final := &entity.FinalOutput{BriefVersion: 2, Kind: "video", JobIDs: ids, Outputs: []string{}}
	if err = reviews.SaveFinal(ctx, user, p.ID, final); err != nil {
		t.Fatal("final", err)
	}
	edit.ExpectedVersion = 2
	edit.Points[0].Value = "Evening"
	change := entity.ProjectChange{Diffs: []entity.PointDiff{{Key: "lighting", Old: "Morning", New: "Evening"}}, Impacts: []entity.ShotReview{{ShotID: plan[0].ID, Status: "needs_review", Reasons: []string{"lighting changed"}}, {ShotID: plan[1].ID, Status: "unchanged", Reasons: []string{}}}}
	if err = reviews.Edit(ctx, user, p.ID, edit, change); err != nil {
		t.Fatal(err)
	}
	b, err := briefs.Get(ctx, user, p.BriefID)
	if err != nil || b.Version != 3 {
		t.Fatal(b, err)
	}
	if err = reviews.Decide(ctx, user, p.ID, plan[0].ID, ids[0], "accept", ""); !errors.Is(err, ErrWorkflowConflict) {
		t.Fatal("stale affected output accepted", err)
	}
	if err = reviews.Decide(ctx, user, p.ID, plan[1].ID, ids[1], "accept", ""); err != nil {
		t.Fatal("unaffected output rejected", err)
	}
	final.BriefVersion = 3
	if err = reviews.SaveFinal(ctx, user, p.ID, final); !errors.Is(err, ErrWorkflowConflict) {
		t.Fatal("unresolved final accepted", err)
	}
	if err = reviews.Decide(ctx, user, p.ID, plan[0].ID, ids[0], "keep", "Director approved morning light exception"); err != nil {
		t.Fatal(err)
	}
	if err = reviews.SaveFinal(ctx, user, p.ID, final); err != nil {
		t.Fatal("kept output could not finalize", err)
	}
	// An accepted version cannot be silently replaced by a previously assembled final selection.
	j := &entity.GenerationJob{ShotID: plan[0].ID, RequestKey: p.ID, Kind: "video", Prompt: "Evening", Status: "completed", Outputs: []string{"https://example.com/new.mp4"}}
	if _, err = jobs.Claim(ctx, j); err != nil {
		t.Fatal(err)
	}
	if err = jobs.Save(ctx, j); err != nil {
		t.Fatal(err)
	}
	if err = reviews.Decide(ctx, user, p.ID, plan[0].ID, j.ID, "accept", ""); err != nil {
		t.Fatal(err)
	}
	if err = reviews.SaveFinal(ctx, user, p.ID, final); !errors.Is(err, ErrWorkflowConflict) {
		t.Fatal("outdated selection saved", err)
	}
}
