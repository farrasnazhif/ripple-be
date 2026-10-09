package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/r2"
	"github.com/farrasnazhif/ripple-be/internal/repository"
	"github.com/farrasnazhif/ripple-be/internal/service"
)

func seedReviews(ctx context.Context, db *sql.DB, storage *r2.Client, user string) error {
	projects := repository.NewProjectRepository(db)
	briefs := repository.NewBriefRepository(db)
	shots := repository.NewStoryboardRepository(db)
	jobs := repository.NewGenerationRepository(db)
	reviews := repository.NewReviewRepository(db)
	for _, key := range []string{"images", "mixed"} {
		project := demoID(user, key)
		p, err := projects.Get(ctx, user, project)
		if err != nil {
			return err
		}
		b, err := briefs.Get(ctx, user, p.BriefID)
		if err != nil {
			return err
		}
		state, err := reviews.Get(ctx, user, project)
		if err != nil {
			return err
		}
		// Preserve demo projects once this workflow fixture exists or the user has revised the brief.
		if len(state.Points) > 0 || len(state.Changes) > 0 || b.Version > 1 {
			fmt.Printf("Preserved review fixture %s\n", project)
			continue
		}
		plan, err := shots.List(ctx, user, project)
		if err != nil {
			return err
		}
		if key == "images" && len(plan) == 0 && len(state.Finals) > 0 {
			if len(state.Finals) == 1 {
				if _, err = db.ExecContext(ctx, `INSERT INTO project_final_outputs(project_id,parent_id,request_key,brief_version,kind,job_ids,outputs,prompt,reason,accepted)VALUES($1,$2,gen_random_uuid(),$3,'image','[]','["/demo-assets/landscape-3.png"]','DEMO: add more space for campaign copy','DEMO: illustrated revision, no provider called',false)`, project, state.Finals[0].ID, b.Version); err != nil {
					return err
				}
			}
			fmt.Printf("Preserved direct-image fixture %s\n", project)
			continue
		}
		if len(plan) != 2 {
			return fmt.Errorf("demo plan changed; refusing to replace user work")
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		output := "image"
		name := "DEMO 2 — Accepted image collection"
		description := "Brief → summary → final images. Open Final Output to inspect saved versions and refine an image directly."
		if key == "mixed" {
			output = "video"
			name = "DEMO 3 — Video change impact"
			description = "Brief → summary → storyboard → clip versions → final video. A local lighting change affects Clip 1; Clip 2 stays unchanged. Review the new Clip 1 version."
		}
		if _, err = tx.ExecContext(ctx, `UPDATE projects SET output_type=$3,name=$4,description=$5 WHERE id=$1 AND user_id=$2`, project, user, output, name, description); err != nil {
			return err
		}
		points := []entity.BriefPoint{{Key: "palette", Value: "Natural greens and warm gold", Scope: "global"}, {Key: "lighting", Value: "Warm morning light", Scope: "local"}, {Key: "detail", Value: "Clean composition and gentle flower motion", Scope: "local"}}
		for _, point := range points {
			if _, err = tx.ExecContext(ctx, `INSERT INTO brief_points(brief_id,key,value,scope)VALUES($1,$2,$3,$4)`, b.ID, point.Key, point.Value, point.Scope); err != nil {
				return err
			}
		}
		for i, shot := range plan {
			requirement := "lighting"
			if i == 1 {
				requirement = "detail"
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO shot_requirements(shot_id,point_key)VALUES($1,$2) ON CONFLICT DO NOTHING`, shot.ID, requirement); err != nil {
				return err
			}
		}
		payload, _ := json.Marshal(points)
		if _, err = tx.ExecContext(ctx, `INSERT INTO brief_versions(brief_id,version,raw_text,summary,points,attachments,actor_id) SELECT id,version,raw_text,COALESCE(summary,''),$2,COALESCE((SELECT jsonb_agg(id) FROM brief_attachments WHERE brief_id=briefs.id AND status='completed'),'[]'),user_id FROM briefs WHERE id=$1 ON CONFLICT(brief_id,version)DO UPDATE SET points=excluded.points,attachments=excluded.attachments`, b.ID, string(payload)); err != nil {
			return err
		}
		if key == "images" {
			if err = insertJob(ctx, tx, plan[0].ID, "image", "DEMO REFINEMENT: Preserve warm greens and add more open space for campaign copy.", "completed", []string{"/demo-assets/landscape-3.png"}); err != nil {
				return err
			}
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		saved, err := jobs.List(ctx, user, project)
		if err != nil {
			return err
		}
		for _, shot := range plan {
			var selected entity.GenerationJob
			for _, job := range saved {
				if job.ShotID == shot.ID && job.Kind == output && job.Status == "completed" {
					selected = job
				}
			}
			if selected.ID == "" {
				return fmt.Errorf("missing demo output")
			}
			if err = reviews.Decide(ctx, user, project, shot.ID, selected.ID, "accept", "DEMO: reviewed sample against the confirmed requirements"); err != nil {
				return err
			}
		}
		finals := service.NewFinalOutputService(reviews, projects, briefs, shots, jobs, storage)
		if _, err = finals.Create(ctx, user, project, output); err != nil {
			return err
		}
		if key == "mixed" {
			summary := ""
			if b.Summary != nil {
				summary = *b.Summary
			}
			points[1].Value = "Soft evening light"
			reviewService := service.NewReviewService(reviews, projects, briefs, shots, storage)
			if err = reviewService.Edit(ctx, user, project, entity.BriefEdit{ExpectedVersion: b.Version, RawText: b.RawText, Summary: summary, Points: points, Note: "DEMO: director requests evening light for the garden introduction only"}); err != nil {
				return err
			}
			if err = reviewService.Decide(ctx, user, project, plan[0].ID, "", "refine", "DEMO: refine Clip 1 for evening light; preserve Clip 2"); err != nil {
				return err
			}
			var id string
			err = db.QueryRowContext(ctx, `INSERT INTO generation_jobs(shot_id,request_key,kind,prompt,status,outputs,brief_version) VALUES($1,gen_random_uuid(),'video',$2,'completed','["/demo-assets/flower.mp4"]',$3)RETURNING id`, plan[0].ID, "DEMO REFINED VERSION: garden introduction with soft evening light. Public-domain sample illustrates the review workflow; it is not an AI-generated revision.", b.Version+1).Scan(&id)
			if err != nil {
				return err
			}
		}
		fmt.Printf("Updated %s (%s)\n", name, project)
	}
	return nil
}
