package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/handler"
	"github.com/golang-jwt/jwt"
)

type testUserRepo struct{}

func (testUserRepo) Create(context.Context, *entity.User) error { return nil }
func (testUserRepo) GetByEmail(context.Context, string) (*entity.User, error) {
	return nil, entity.ErrNotFound
}
func (testUserRepo) GetByID(_ context.Context, id string) (*entity.User, error) {
	if id == "user-1" {
		return &entity.User{ID: id, IsActive: true}, nil
	}
	if id == "inactive-user" {
		return &entity.User{ID: id}, nil
	}
	return nil, entity.ErrNotFound
}

func TestWorkspaceAPIRequiresLogin(t *testing.T) {
	mux := New(Config{Review: handler.NewReviewHandler(nil, nil, nil), Workflow: handler.NewWorkflowHandler(nil, nil), JWTSecret: "test-secret", RateLimit: 100, RegisterLimit: 100, RateLimitWindow: time.Minute}, nil, nil, nil, nil, testUserRepo{})
	for _, path := range []string{"/api/v1/demo/graph", "/api/v1/impact/preview", "/api/v1/projects", "/api/v1/projects/", "/api/v1/briefs/storage", "/api/v1/projects/11111111-1111-1111-1111-111111111111/storyboard", "/api/v1/projects/11111111-1111-1111-1111-111111111111/generations", "/api/v1/projects/11111111-1111-1111-1111-111111111111/generations/22222222-2222-2222-2222-222222222222"} {
		method := http.MethodGet
		if path == "/api/v1/impact/preview" {
			method = http.MethodPost
		}
		request := httptest.NewRequest(method, path, nil)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s without login: got %d", path, response.Code)
		}
	}

	for _, route := range []struct{ method, path string }{{"GET", "review"}, {"PUT", "brief"}, {"PUT", "output-type"}, {"PUT", "shots/22222222-2222-2222-2222-222222222222/requirements"}, {"POST", "shots/22222222-2222-2222-2222-222222222222/decisions"}, {"POST", "plan/draft"}, {"POST", "summary/refine"}, {"POST", "final"}} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(route.method, "/api/v1/projects/11111111-1111-1111-1111-111111111111/"+route.path, nil))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("review endpoint exposed: %s got %d", route.path, response.Code)
		}
	}

	for _, method := range []string{"PATCH", "DELETE"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(method, "/api/v1/projects/11111111-1111-1111-1111-111111111111", nil))
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("project mutation exposed: %s got %d", method, response.Code)
		}
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "user-1", "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/demo/graph", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("workspace API with login: got %d: %s", response.Code, response.Body.String())
	}

	inactiveToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": "inactive-user", "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/demo/graph", nil)
	request.Header.Set("Authorization", "Bearer "+inactiveToken)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("inactive account reached workspace API: got %d", response.Code)
	}
}
