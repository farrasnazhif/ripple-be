package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/repository"
)

var ErrInvalidReview = errors.New("provide valid requirements, a saved brief version and a reason for keeping or refining an output")
var pointKey = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`)

type ReviewService struct {
	repo     repository.ReviewRepository
	projects repository.ProjectRepository
	briefs   repository.BriefRepository
	shots    repository.StoryboardRepository
	storage  BriefStorage
}

func NewReviewService(r repository.ReviewRepository, p repository.ProjectRepository, b repository.BriefRepository, s repository.StoryboardRepository, storage BriefStorage) *ReviewService {
	return &ReviewService{r, p, b, s, storage}
}
func (s *ReviewService) Get(ctx context.Context, user, project string) (*entity.ProjectReview, error) {
	state, err := s.repo.Get(ctx, user, project)
	if err != nil {
		return nil, err
	}
	for i := range state.Finals {
		if state.Finals[i].ObjectKey != "" && s.storage != nil {
			url, err := s.storage.PresignDownload(ctx, state.Finals[i].ObjectKey)
			if err != nil {
				return nil, err
			}
			state.Finals[i].DownloadURL = url
		}
	}
	return state, nil
}
func CalculateChange(old, next []entity.BriefPoint, shots []entity.StoryboardShot, narrative bool) entity.ProjectChange {
	change := entity.ProjectChange{Diffs: []entity.PointDiff{}, Impacts: []entity.ShotReview{}}
	before := map[string]entity.BriefPoint{}
	after := map[string]entity.BriefPoint{}
	order := []string{}
	for _, p := range old {
		before[p.Key] = p
		order = append(order, p.Key)
	}
	for _, p := range next {
		after[p.Key] = p
		if _, ok := before[p.Key]; !ok {
			order = append(order, p.Key)
		}
	}
	for _, key := range order {
		a, b := before[key], after[key]
		if a.Value != b.Value || a.Scope != b.Scope {
			change.Diffs = append(change.Diffs, entity.PointDiff{Key: key, Old: a.Value, New: b.Value})
		}
	}
	for _, shot := range shots {
		impact := entity.ShotReview{ShotID: shot.ID, Status: "unchanged", Reasons: []string{}}
		for _, d := range change.Diffs {
			linked := before[d.Key].Scope == "global" || after[d.Key].Scope == "global"
			for _, key := range shot.RequirementKeys {
				linked = linked || key == d.Key
			}
			if linked {
				impact.Reasons = append(impact.Reasons, fmt.Sprintf("%s changed from %q to %q", d.Key, d.Old, d.New))
			}
		}
		if narrative && len(change.Diffs) == 0 {
			impact.Reasons = append(impact.Reasons, "Source content changed without confirmed requirement changes. Review this shot manually.")
		}
		if len(impact.Reasons) > 0 {
			impact.Status = "needs_review"
		}
		change.Impacts = append(change.Impacts, impact)
	}
	return change
}
func (s *ReviewService) Edit(ctx context.Context, user, project string, input entity.BriefEdit) error {
	if input.ExpectedVersion < 1 || len(input.RawText) > 100000 || len(input.Summary) > 100000 || len(input.Note) > 2000 || len(input.Points) > 50 || len(input.RemoveAttachments) > 100 {
		return ErrInvalidReview
	}
	seen := map[string]bool{}
	for _, p := range input.Points {
		if !pointKey.MatchString(p.Key) || seen[p.Key] || strings.TrimSpace(p.Value) == "" || len(p.Value) > 2000 || (p.Scope != "local" && p.Scope != "global") {
			return ErrInvalidReview
		}
		seen[p.Key] = true
	}
	p, err := s.projects.Get(ctx, user, project)
	if err != nil {
		return err
	}
	b, err := s.briefs.Get(ctx, user, p.BriefID)
	if err != nil {
		return err
	}
	state, err := s.repo.Get(ctx, user, project)
	if err != nil {
		return err
	}
	shots, err := s.shots.List(ctx, user, project)
	if err != nil {
		return err
	}
	saved := ""
	if b.Summary != nil {
		saved = *b.Summary
	}
	attachmentsChanged := len(input.RemoveAttachments) > 0
	attachments, err := s.briefs.ListAttachments(ctx, user, p.BriefID)
	if err != nil {
		return err
	}
	for _, version := range state.Versions {
		if version.Version == b.Version {
			savedIDs := map[string]bool{}
			for _, id := range version.Attachments {
				savedIDs[id] = true
			}
			for _, file := range attachments {
				if !savedIDs[file.ID] {
					attachmentsChanged = true
				}
			}
			break
		}
	}
	change := CalculateChange(state.Points, input.Points, shots, b.RawText != input.RawText || saved != input.Summary || attachmentsChanged)
	if input.Points == nil {
		input.Points = []entity.BriefPoint{}
	}
	return s.repo.Edit(ctx, user, project, input, change)
}
func (s *ReviewService) SetType(ctx context.Context, user, project, kind string) error {
	if kind != "image" && kind != "video" {
		return ErrInvalidReview
	}
	p, err := s.projects.Get(ctx, user, project)
	if err != nil {
		return err
	}
	b, err := s.briefs.Get(ctx, user, p.BriefID)
	if err != nil {
		return err
	}
	if b.Summary == nil || strings.TrimSpace(*b.Summary) == "" {
		return ErrInvalidReview
	}
	return s.repo.SetType(ctx, user, project, kind)
}
func (s *ReviewService) Link(ctx context.Context, user, project, shot string, keys []string) error {
	if len(keys) > 50 {
		return ErrInvalidReview
	}
	seen := map[string]bool{}
	for _, key := range keys {
		if !pointKey.MatchString(key) || seen[key] {
			return ErrInvalidReview
		}
		seen[key] = true
	}
	return s.repo.Link(ctx, user, project, shot, keys)
}
func (s *ReviewService) Decide(ctx context.Context, user, project, shot, job, action, reason string) error {
	reason = strings.TrimSpace(reason)
	if len(reason) > 2000 || (action != "accept" && action != "keep" && action != "refine") || ((action == "keep" || action == "refine") && reason == "") || ((action == "keep" || action == "accept") && job == "") {
		return ErrInvalidReview
	}
	return s.repo.Decide(ctx, user, project, shot, job, action, reason)
}
