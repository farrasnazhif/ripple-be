package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/farrasnazhif/ripple-be/internal/entity"
	"os"
	"sync"
	"testing"
	"time"
)

func TestProjectWorkflowDatabase(t *testing.T) {
	dsn := os.Getenv("RIPPLE_TEST_DB")
	if dsn == "" {
		t.Skip("set RIPPLE_TEST_DB to run PostgreSQL workflow checks")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	var user string
	name := fmt.Sprintf("workflow-%d", time.Now().UnixNano())
	err = db.QueryRowContext(ctx, `INSERT INTO users(email,username,password)VALUES($1,$2,$3)RETURNING id`, name+"@example.invalid", name, []byte("test-only")).Scan(&user)
	if err != nil {
		t.Fatal(err)
	}
	defer db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, user)
	projects := NewProjectRepository(db)
	briefs := NewBriefRepository(db)
	shots := NewStoryboardRepository(db)
	jobs := NewGenerationRepository(db, testBudgetRates)
	p, err := projects.Create(ctx, user, "Workflow test", "description")
	if err != nil {
		t.Fatal(err)
	}
	if err = briefs.SetText(ctx, user, p.BriefID, "Original source"); err != nil {
		t.Fatal(err)
	}
	if err = briefs.SetText(ctx, user, p.BriefID, "Changed source"); !errors.Is(err, ErrWorkflowConflict) {
		t.Fatal("original overwritten", err)
	}
	if err = briefs.ConfirmSummary(ctx, user, p.BriefID, "Confirmed summary"); err != nil {
		t.Fatal(err)
	}
	if err = shots.Create(ctx, user, p.ID, "Confirmed summary", []entity.StoryboardShot{{Title: "Shot 1", Prompt: "A beach"}}); err != nil {
		t.Fatal(err)
	}
	if err = shots.Create(ctx, user, p.ID, "Confirmed summary", []entity.StoryboardShot{{Title: "Duplicate", Prompt: "bad"}}); !errors.Is(err, ErrWorkflowConflict) {
		t.Fatal("duplicate storyboard", err)
	}
	if err = briefs.ConfirmSummary(ctx, user, p.BriefID, "Changed summary"); !errors.Is(err, ErrWorkflowConflict) {
		t.Fatal("storyboard source summary overwritten", err)
	}
	list, err := shots.List(ctx, user, p.ID)
	if err != nil || len(list) != 1 {
		t.Fatal("shots", err)
	}
	if _, err = shots.GetShot(ctx, "00000000-0000-0000-0000-000000000000", p.ID, list[0].ID); !errors.Is(err, entity.ErrNotFound) {
		t.Fatal("owner isolation", err)
	}
	fundTestProject(t, db, user, p.ID)
	key := "11111111-1111-1111-1111-111111111111"
	var wait sync.WaitGroup
	claims := make(chan bool, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			j := &entity.GenerationJob{ShotID: list[0].ID, Kind: "image", RequestKey: key, Prompt: "A beach"}
			ok, err := jobs.Claim(ctx, j)
			claims <- ok
			errs <- err
		}()
	}
	wait.Wait()
	close(claims)
	close(errs)
	count := 0
	for ok := range claims {
		if ok {
			count++
		}
	}
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if count != 1 {
		t.Fatal("duplicate claims", count)
	}
	j, err := jobs.Get(ctx, user, p.ID, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = jobs.Claim(ctx, &entity.GenerationJob{ShotID: list[0].ID, Kind: "image", RequestKey: "22222222-2222-2222-2222-222222222222", Prompt: "A beach"}); !errors.Is(err, ErrGenerationConflict) {
		t.Fatal("concurrent paid request allowed", err)
	}
	j.Status = "completed"
	j.ProviderID = "provider"
	j.Outputs = []string{"https://example.com/image.png"}
	if err = jobs.Save(ctx, j); err != nil {
		t.Fatal(err)
	}
	persisted, err := jobs.Get(ctx, user, p.ID, j.ID)
	if err != nil || persisted.Status != "completed" || len(persisted.Outputs) != 1 {
		t.Fatal("output persistence", err)
	}

	stale := *persisted
	stale.Status = "queued"
	stale.Outputs = []string{}
	if err = jobs.Save(ctx, &stale); err != nil {
		t.Fatal(err)
	}
	final, err := jobs.Get(ctx, user, p.ID, j.ID)
	if err != nil || final.Status != "completed" || len(final.Outputs) != 1 {
		t.Fatal("stale poll erased terminal output", err)
	}
	if _, err = jobs.Get(ctx, "00000000-0000-0000-0000-000000000000", p.ID, j.ID); !errors.Is(err, entity.ErrNotFound) {
		t.Fatal("job ownership", err)
	}
}
