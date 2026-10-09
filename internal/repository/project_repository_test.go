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

func TestProjectEditingAndDeletionDatabase(t *testing.T) {
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
	name := fmt.Sprintf("project-crud-%d", time.Now().UnixNano())
	if err = db.QueryRowContext(ctx, `INSERT INTO users(email,username,password)VALUES($1,$2,$3)RETURNING id`, name+"@example.invalid", name, []byte("test-only")).Scan(&user); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, user); err != nil {
			t.Error(err)
		}
	}()
	projects := NewProjectRepository(db)
	briefs := NewBriefRepository(db)
	reviews := NewReviewRepository(db)
	shots := NewStoryboardRepository(db)
	jobs := NewGenerationRepository(db, testBudgetRates)
	p, err := projects.Create(ctx, user, "Original title", "Original description")
	if err != nil || p.CreatedBy != name {
		t.Fatal(p, err)
	}
	fundTestProject(t, db, user, p.ID)
	oldTime := p.CreatedAt
	if err = briefs.SetText(ctx, user, p.BriefID, "Original brief"); err != nil {
		t.Fatal(err)
	}
	if err = briefs.ConfirmSummary(ctx, user, p.BriefID, "Confirmed summary"); err != nil {
		t.Fatal(err)
	}
	if err = reviews.Edit(ctx, user, p.ID, entity.BriefEdit{ExpectedVersion: 1, RawText: "Original brief", Summary: "Confirmed summary", Points: []entity.BriefPoint{}}, entity.ProjectChange{Diffs: []entity.PointDiff{}, Impacts: []entity.ShotReview{}}); err != nil {
		t.Fatal(err)
	}
	if err = shots.Create(ctx, user, p.ID, "Confirmed summary", []entity.StoryboardShot{{Title: "One", Prompt: "Garden"}}); err != nil {
		t.Fatal(err)
	}
	plan, err := shots.List(ctx, user, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	var key string
	if err = db.QueryRowContext(ctx, `SELECT gen_random_uuid()`).Scan(&key); err != nil {
		t.Fatal(err)
	}
	j := &entity.GenerationJob{ShotID: plan[0].ID, RequestKey: key, Kind: "video", Prompt: "Garden", Status: "completed", Outputs: []string{"https://example.com/clip.mp4"}}
	if _, err = jobs.Claim(ctx, j); err != nil {
		t.Fatal(err)
	}
	if err = jobs.Save(ctx, j); err != nil {
		t.Fatal(err)
	}
	if err = reviews.Decide(ctx, user, p.ID, plan[0].ID, j.ID, "accept", ""); err != nil {
		t.Fatal(err)
	}
	originalKey := "test/" + p.ID + "/original.png"
	removedKey := "test/" + p.ID + "/removed.png"
	finalKey := "test/" + p.ID + "/final.mp4"
	defer db.ExecContext(ctx, `DELETE FROM storage_cleanup WHERE object_key=$1 OR object_key=$2 OR object_key=$3`, originalKey, removedKey, finalKey)
	for _, key := range []string{originalKey, removedKey} {
		if _, err = db.ExecContext(ctx, `INSERT INTO brief_attachments(brief_id,object_key,file_name,content_type,size_bytes,status,removed)VALUES($1,$2,'reference.png','image/png',1,'completed',$3)`, p.BriefID, key, key == removedKey); err != nil {
			t.Fatal(err)
		}
	}

	if _, err = db.ExecContext(ctx, `INSERT INTO storage_cleanup(object_key,not_before)VALUES($1,now()-interval '1 minute')`, originalKey); err != nil {
		t.Fatal(err)
	}
	pending, err := NewStorageCleanupRepository(db).Pending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range pending {
		if key == originalKey {
			t.Fatal("live reference must not be cleaned up")
		}
	}
	final := &entity.FinalOutput{BriefVersion: 2, Kind: "video", JobIDs: []string{j.ID}, Outputs: []string{}, ObjectKey: finalKey}
	if err = reviews.SaveFinal(ctx, user, p.ID, final); err != nil {
		t.Fatal(err)
	}
	outsider := "00000000-0000-0000-0000-000000000000"
	if _, err = projects.Update(ctx, outsider, p.ID, "Attack", "Attack"); !errors.Is(err, entity.ErrNotFound) {
		t.Fatal("cross-owner update", err)
	}
	if err = projects.Delete(ctx, outsider, p.ID); !errors.Is(err, entity.ErrNotFound) {
		t.Fatal("cross-owner delete", err)
	}
	updated, err := projects.Update(ctx, user, p.ID, "Updated title", "Updated description")
	if err != nil || updated.Name != "Updated title" || updated.Description != "Updated description" || updated.BriefID != p.BriefID || !updated.CreatedAt.Equal(oldTime) || updated.CreatedBy != name {
		t.Fatal("project metadata edit damaged identity", updated, err)
	}
	b, err := briefs.Get(ctx, user, p.BriefID)
	if err != nil || b.RawText != "Original brief" {
		t.Fatal("metadata edit changed brief", b, err)
	}
	if err = projects.Delete(ctx, user, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = projects.Get(ctx, user, p.ID); !errors.Is(err, entity.ErrNotFound) {
		t.Fatal("project retained", err)
	}
	if _, err = briefs.Get(ctx, user, p.BriefID); !errors.Is(err, entity.ErrNotFound) {
		t.Fatal("brief retained", err)
	}
	if _, err = shots.GetShot(ctx, user, p.ID, plan[0].ID); !errors.Is(err, entity.ErrNotFound) {
		t.Fatal("shot retained", err)
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM storage_cleanup WHERE object_key=$1 OR object_key=$2 OR object_key=$3`, originalKey, removedKey, finalKey).Scan(&count); err != nil || count != 3 {
		t.Fatal("storage cleanup lost originals/archive/final", count, err)
	}
	cleanup := NewStorageCleanupRepository(db)
	due, err := cleanup.Pending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range due {
		if key == originalKey || key == removedKey || key == finalKey {
			t.Fatal("cleanup ran before signed URLs expired")
		}
	}
	if err = projects.Delete(ctx, user, p.ID); !errors.Is(err, entity.ErrNotFound) {
		t.Fatal("delete nonexistent project", err)
	}
}
