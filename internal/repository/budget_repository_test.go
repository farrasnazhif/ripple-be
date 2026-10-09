package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

var testBudgetRates = entity.GenerationRates{"image": 20, "video": 100, "image_refinement": 40, "video_refinement": 200}

func fundTestProject(t *testing.T, db *sql.DB, user, project string) {
	t.Helper()
	if err := NewBudgetRepository(db, testBudgetRates).Set(context.Background(), user, project, 100000); err != nil {
		t.Fatal(err)
	}
}

func TestProjectBudgetDatabase(t *testing.T) {
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
	name := fmt.Sprintf("budget-%d", time.Now().UnixNano())
	if err = db.QueryRowContext(ctx, `INSERT INTO users(email,username,password)VALUES($1,$2,$3)RETURNING id`, name+"@example.invalid", name, []byte("test-only")).Scan(&user); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, user); err != nil {
			t.Error(err)
		}
	}()
	p, err := NewProjectRepository(db).Create(ctx, user, "Budget test", "")
	if err != nil {
		t.Fatal(err)
	}
	budget := NewBudgetRepository(db, testBudgetRates)
	outsider := "00000000-0000-0000-0000-000000000000"
	if _, err = budget.Get(ctx, outsider, p.ID); !errors.Is(err, entity.ErrNotFound) {
		t.Fatal("cross-owner read", err)
	}
	if err = budget.Set(ctx, outsider, p.ID, 1000); !errors.Is(err, entity.ErrNotFound) {
		t.Fatal("cross-owner edit", err)
	}
	for _, limit := range []int64{-1, 100000001} {
		if err = budget.Set(ctx, user, p.ID, limit); !errors.Is(err, ErrBudgetLimit) {
			t.Fatal("invalid cap", err)
		}
	}
	shots := make([]string, 2)
	for i := range shots {
		if err = db.QueryRowContext(ctx, `INSERT INTO storyboard_shots(project_id,position,title,prompt)VALUES($1,$2,'Shot','Prompt')RETURNING id`, p.ID, i+1).Scan(&shots[i]); err != nil {
			t.Fatal(err)
		}
	}
	jobs := NewGenerationRepository(db, testBudgetRates)
	makeJob := func(i int) *entity.GenerationJob {
		return &entity.GenerationJob{ShotID: shots[i], Kind: "image", RequestKey: shots[i], Prompt: "Prompt", Status: "submitting", Outputs: []string{}}
	}
	if _, err = jobs.Claim(ctx, makeJob(0)); !errors.Is(err, ErrBudgetUnset) {
		t.Fatal("unfunded request", err)
	}
	if err = budget.Set(ctx, user, p.ID, 20); err != nil {
		t.Fatal(err)
	}
	if _, err = NewGenerationRepository(db).Claim(ctx, makeJob(0)); !errors.Is(err, ErrBudgetRate) {
		t.Fatal("unpriced request", err)
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM generation_jobs WHERE shot_id=ANY($1::uuid[])`, fmt.Sprintf("{%s,%s}", shots[0], shots[1])).Scan(&count); err != nil || count != 0 {
		t.Fatal("blocked request persisted", count, err)
	}
	var wait sync.WaitGroup
	results := make(chan *entity.GenerationJob, 2)
	errs := make(chan error, 2)
	for i := range shots {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			j := makeJob(i)
			ok, err := jobs.Claim(ctx, j)
			if ok {
				results <- j
			}
			errs <- err
		}(i)
	}
	wait.Wait()
	close(results)
	close(errs)
	denied := 0
	for err := range errs {
		if errors.Is(err, ErrBudgetExceeded) {
			denied++
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if len(results) != 1 || denied != 1 {
		t.Fatal("concurrent overspend", len(results), denied)
	}
	j := <-results
	assertBudget := func(reserved, spent, available int64) {
		t.Helper()
		b, err := budget.Get(ctx, user, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if b.ReservedCents != reserved || b.SpentCents != spent || b.AvailableCents != available {
			t.Fatalf("incorrect totals: %+v", b)
		}
	}
	assertBudget(20, 0, 0)
	if claimed, err := jobs.Claim(ctx, j); err != nil || claimed {
		t.Fatal("idempotent claim", claimed, err)
	}
	assertBudget(20, 0, 0)
	if err = budget.Set(ctx, user, p.ID, 19); !errors.Is(err, ErrBudgetLimit) {
		t.Fatal("lower cap bypass", err)
	}
	j.Status = "unknown"
	if err = jobs.Save(ctx, j); err != nil {
		t.Fatal(err)
	}
	assertBudget(20, 0, 0)
	j.Status = "failed"
	j.SubmissionRejected = true
	if err = jobs.Save(ctx, j); err != nil {
		t.Fatal(err)
	}
	assertBudget(0, 0, 20)
	next := makeJob(0)
	if next.ShotID == j.ShotID {
		next = makeJob(1)
	}
	if claimed, err := jobs.Claim(ctx, next); err != nil || !claimed {
		t.Fatal("released reservation", claimed, err)
	}
	next.Status = "completed"
	next.ProviderID = "accepted-request"
	next.Outputs = []string{"https://example.com/image.png"}
	if err = jobs.Save(ctx, next); err != nil {
		t.Fatal(err)
	}
	assertBudget(0, 20, 0)
	// Late polls cannot refund or double charge a terminal request.
	next.Status = "failed"
	next.SubmissionRejected = true
	if err = jobs.Save(ctx, next); err != nil {
		t.Fatal(err)
	}
	assertBudget(0, 20, 0)
	if err = budget.Set(ctx, user, p.ID, 40); err != nil {
		t.Fatal(err)
	}
	failed := makeJob(0)
	failed.RequestKey = p.ID
	if ok, err := jobs.Claim(ctx, failed); err != nil || !ok {
		t.Fatal("failed request claim", ok, err)
	}
	failed.Status = "failed"
	failed.ProviderID = "accepted-but-failed"
	if err = jobs.Save(ctx, failed); err != nil {
		t.Fatal(err)
	}
	assertBudget(0, 40, 0)
	b, err := budget.Get(ctx, user, p.ID)
	if err != nil || len(b.Usage) != 3 {
		t.Fatal("ledger", b, err)
	}
}
