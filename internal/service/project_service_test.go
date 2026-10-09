package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

type projectRepoStub struct {
	userID, name, description string
}

func (r *projectRepoStub) Create(_ context.Context, userID, name, description string) (*entity.Project, error) {
	r.userID, r.name, r.description = userID, name, description
	return &entity.Project{ID: "project-1", Name: name, Description: description}, nil
}
func (r *projectRepoStub) List(_ context.Context, userID string) ([]entity.Project, error) {
	r.userID = userID
	return []entity.Project{}, nil
}
func (r *projectRepoStub) Get(_ context.Context, userID, projectID string) (*entity.Project, error) {
	r.userID = userID
	return &entity.Project{ID: projectID}, nil
}

func TestProjectCreateValidatesAndScopesOwner(t *testing.T) {
	repo := &projectRepoStub{}
	svc := NewProjectService(repo)
	project, err := svc.Create(context.Background(), "user-1", "  My campaign  ", "  Launch work  ")
	if err != nil || project.Name != "My campaign" || repo.userID != "user-1" || repo.description != "Launch work" {
		t.Fatalf("create did not trim or scope the project: project=%+v repo=%+v err=%v", project, repo, err)
	}
	for _, input := range []struct{ name, description string }{
		{" ", ""},
		{strings.Repeat("a", 101), ""},
		{"Valid", strings.Repeat("b", 501)},
	} {
		if _, err := svc.Create(context.Background(), "user-1", input.name, input.description); !errors.Is(err, ErrInvalidProject) {
			t.Fatalf("invalid project accepted: %+v, err=%v", input, err)
		}
	}
	if _, err := svc.List(context.Background(), "user-2"); err != nil || repo.userID != "user-2" {
		t.Fatalf("list did not scope owner: %v", err)
	}
	if _, err := svc.Get(context.Background(), "user-3", "project-1"); err != nil || repo.userID != "user-3" {
		t.Fatalf("get did not scope owner: %v", err)
	}
}

func (r *projectRepoStub) Update(ctx context.Context, user, project, name, description string) (*entity.Project, error) {
	return r.Create(ctx, user, name, description)
}
func (r *projectRepoStub) Delete(_ context.Context, user, project string) error {
	r.userID = user
	return nil
}
func TestProjectEditValidationAndDeleteOwner(t *testing.T) {
	r := &projectRepoStub{}
	s := NewProjectService(r)
	p, err := s.Update(context.Background(), "owner", "project", " New title ", " New description ")
	if err != nil || p.Name != "New title" || r.userID != "owner" || p.Description != "New description" {
		t.Fatal(p, err)
	}
	if _, err = s.Update(context.Background(), "owner", "project", " ", ""); !errors.Is(err, ErrInvalidProject) {
		t.Fatal("blank edit accepted")
	}
	if err = s.Delete(context.Background(), "owner", "project"); err != nil || r.userID != "owner" {
		t.Fatal("delete owner", err)
	}
}
