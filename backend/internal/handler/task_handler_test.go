package handler_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/example/task-management/backend/internal/cache"
	"github.com/example/task-management/backend/internal/domain"
	"github.com/example/task-management/backend/internal/handler"
	"github.com/example/task-management/backend/internal/service"
)

type env struct {
	t    *testing.T
	srv  *httptest.Server
	repo *fakeRepo
	mr   *miniredis.Miniredis
}

func setup(t *testing.T) *env {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { rdb.Close() })

	repo := newFakeRepo()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewTaskService(repo, cache.NewRedisTaskCache(rdb, 60*time.Second), log)
	srv := httptest.NewServer(handler.NewRouter(handler.NewTaskHandler(svc)))
	t.Cleanup(srv.Close)
	return &env{t: t, srv: srv, repo: repo, mr: mr}
}

func (e *env) do(method, path string, body any) (int, []byte) {
	e.t.Helper()
	var r io.Reader
	if body != nil {
		if s, ok := body.(string); ok {
			r = strings.NewReader(s)
		} else {
			b, _ := json.Marshal(body)
			r = bytes.NewReader(b)
		}
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, r)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func (e *env) create(title string, extra map[string]any) domain.Task {
	e.t.Helper()
	body := map[string]any{"title": title}
	for k, v := range extra {
		body[k] = v
	}
	code, b := e.do(http.MethodPost, "/api/tasks", body)
	if code != http.StatusCreated {
		e.t.Fatalf("create %q: status %d body %s", title, code, b)
	}
	var out struct{ Data domain.Task }
	mustJSON(e.t, b, &out)
	return out.Data
}

func (e *env) list(query string) domain.TaskPage {
	e.t.Helper()
	code, b := e.do(http.MethodGet, "/api/tasks"+query, nil)
	if code != http.StatusOK {
		e.t.Fatalf("list %s: status %d body %s", query, code, b)
	}
	var page domain.TaskPage
	mustJSON(e.t, b, &page)
	return page
}

func mustJSON(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("decode %s: %v", b, err)
	}
}

// assertError checks both the status and the standard error envelope.
func assertError(t *testing.T, code int, body []byte, wantStatus int, wantCode string) {
	t.Helper()
	if code != wantStatus {
		t.Fatalf("status = %d, want %d (body %s)", code, wantStatus, body)
	}
	var env struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	mustJSON(t, body, &env)
	if env.Error.Code != wantCode {
		t.Fatalf("error.code = %q, want %q (body %s)", env.Error.Code, wantCode, body)
	}
	if env.Error.Message == "" {
		t.Fatalf("error.message is empty (body %s)", body)
	}
}

func titles(p domain.TaskPage) []string {
	out := make([]string, len(p.Data))
	for i, t := range p.Data {
		out[i] = t.Title
	}
	return out
}

func path(id int64) string { return "/api/tasks/" + strconv.FormatInt(id, 10) }

// ---------------------------------------------------------------- update

func TestUpdateTask(t *testing.T) {
	t.Run("replaces all fields and returns the updated task", func(t *testing.T) {
		e := setup(t)
		task := e.create("Write docs", map[string]any{"assignee": "alice", "description": "draft"})

		code, b := e.do(http.MethodPut, path(task.ID), map[string]any{
			"title": "  Write README  ", "status": "in_progress", "assignee": "bob", "due_date": "2026-12-01",
		})
		if code != http.StatusOK {
			t.Fatalf("status %d body %s", code, b)
		}
		var out struct{ Data domain.Task }
		mustJSON(t, b, &out)
		got := out.Data
		if got.Title != "Write README" || got.Status != domain.StatusInProgress ||
			got.Assignee == nil || *got.Assignee != "bob" || got.Description != nil ||
			got.DueDate == nil || got.DueDate.Format("2006-01-02") != "2026-12-01" {
			t.Fatalf("unexpected task after update: %+v", got)
		}
	})

	t.Run("same values again is still 200, not 404", func(t *testing.T) {
		e := setup(t)
		task := e.create("Idempotent", nil)
		for i := 0; i < 2; i++ {
			code, b := e.do(http.MethodPut, path(task.ID), map[string]any{"title": "Idempotent", "status": "todo"})
			if code != http.StatusOK {
				t.Fatalf("attempt %d: status %d body %s", i, code, b)
			}
		}
	})

	t.Run("duplicate title returns 409 DUPLICATE_TITLE", func(t *testing.T) {
		e := setup(t)
		e.create("Alpha", nil)
		beta := e.create("Beta", nil)
		code, b := e.do(http.MethodPut, path(beta.ID), map[string]any{"title": "alpha", "status": "todo"})
		assertError(t, code, b, http.StatusConflict, handler.CodeDuplicateTitle)
	})

	t.Run("title of a soft-deleted task can be reused", func(t *testing.T) {
		e := setup(t)
		old := e.create("Reusable", nil)
		other := e.create("Other", nil)
		if code, b := e.do(http.MethodDelete, path(old.ID), nil); code != http.StatusNoContent {
			t.Fatalf("delete: %d %s", code, b)
		}
		code, b := e.do(http.MethodPut, path(other.ID), map[string]any{"title": "Reusable", "status": "todo"})
		if code != http.StatusOK {
			t.Fatalf("status %d body %s", code, b)
		}
	})

	t.Run("validation errors use the standard envelope", func(t *testing.T) {
		e := setup(t)
		task := e.create("Valid", nil)
		cases := []struct {
			name string
			body any
			code string
		}{
			{"missing title", map[string]any{"status": "todo"}, handler.CodeValidation},
			{"blank title", map[string]any{"title": "   "}, handler.CodeValidation},
			{"bad status", map[string]any{"title": "x", "status": "blocked"}, handler.CodeValidation},
			{"bad date", map[string]any{"title": "x", "due_date": "01/12/2026"}, handler.CodeInvalidJSON},
			{"unknown field", map[string]any{"title": "x", "priority": 1}, handler.CodeInvalidJSON},
			{"malformed json", `{"title":`, handler.CodeInvalidJSON},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				code, b := e.do(http.MethodPut, path(task.ID), tc.body)
				assertError(t, code, b, http.StatusBadRequest, tc.code)
			})
		}
	})

	t.Run("unknown, soft-deleted and invalid ids", func(t *testing.T) {
		e := setup(t)
		gone := e.create("Gone", nil)
		e.do(http.MethodDelete, path(gone.ID), nil)
		body := map[string]any{"title": "x", "status": "todo"}

		code, b := e.do(http.MethodPut, "/api/tasks/999", body)
		assertError(t, code, b, http.StatusNotFound, handler.CodeNotFound)
		code, b = e.do(http.MethodPut, path(gone.ID), body)
		assertError(t, code, b, http.StatusNotFound, handler.CodeNotFound)
		code, b = e.do(http.MethodPut, "/api/tasks/abc", body)
		assertError(t, code, b, http.StatusBadRequest, handler.CodeValidation)
	})
}

// ---------------------------------------------------------------- create / delete

func TestCreateDuplicateTitleReturns409(t *testing.T) {
	e := setup(t)
	e.create("Unique", nil)
	code, b := e.do(http.MethodPost, "/api/tasks", map[string]any{"title": "Unique"})
	assertError(t, code, b, http.StatusConflict, handler.CodeDuplicateTitle)
}

func TestSoftDelete(t *testing.T) {
	e := setup(t)
	keep := e.create("Keep", nil)
	drop := e.create("Drop", nil)

	if code, b := e.do(http.MethodDelete, path(drop.ID), nil); code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", code, b)
	}
	page := e.list("")
	if page.Meta.Total != 1 || page.Data[0].ID != keep.ID {
		t.Fatalf("soft-deleted task still listed: %+v", titles(page))
	}
	code, b := e.do(http.MethodGet, path(drop.ID), nil)
	assertError(t, code, b, http.StatusNotFound, handler.CodeNotFound)
	code, b = e.do(http.MethodDelete, path(drop.ID), nil)
	assertError(t, code, b, http.StatusNotFound, handler.CodeNotFound)
}

// ---------------------------------------------------------------- search / list

func TestListTasks(t *testing.T) {
	e := setup(t)
	e.create("Fix login bug", map[string]any{"status": "todo", "assignee": "alice"})
	e.create("Write tests", map[string]any{"status": "in_progress", "assignee": "bob", "description": "cover the LOGIN flow"})
	e.create("Deploy", map[string]any{"status": "done", "assignee": "alice"})
	e.create("100% coverage", map[string]any{"status": "todo"})

	cases := []struct {
		name, query string
		want        []string
	}{
		{"keyword matches title, case-insensitive", "?keyword=LOGIN&sort=id", []string{"Fix login bug", "Write tests"}},
		{"keyword is trimmed", "?keyword=%20%20deploy%20&sort=id", []string{"Deploy"}},
		{"LIKE wildcards are literal", "?keyword=%25", []string{"100% coverage"}},
		{"underscore is literal", "?keyword=_", nil},
		{"status filter", "?status=todo&sort=id", []string{"Fix login bug", "100% coverage"}},
		{"assignee filter", "?assignee=alice&sort=id", []string{"Fix login bug", "Deploy"}},
		{"combined filters", "?keyword=login&assignee=bob", []string{"Write tests"}},
		{"sort by title asc", "?sort=title", []string{"100% coverage", "Deploy", "Fix login bug", "Write tests"}},
		{"default sort is newest first", "", []string{"100% coverage", "Deploy", "Write tests", "Fix login bug"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := titles(e.list(tc.query))
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("pagination meta", func(t *testing.T) {
		p := e.list("?limit=3&page=2&sort=id")
		if p.Meta != (domain.PageMeta{Page: 2, Limit: 3, Total: 4, TotalPages: 2}) || len(p.Data) != 1 {
			t.Fatalf("meta %+v data %v", p.Meta, titles(p))
		}
	})

	t.Run("empty result is [] not null", func(t *testing.T) {
		_, b := e.do(http.MethodGet, "/api/tasks?keyword=nothing-matches", nil)
		if !strings.Contains(string(b), `"data":[]`) {
			t.Fatalf("body %s", b)
		}
	})

	t.Run("invalid params return 400", func(t *testing.T) {
		for _, q := range []string{
			"?status=blocked", "?page=0", "?page=abc", "?limit=101",
			"?sort=password", "?sort=title%3BDROP%20TABLE%20tasks",
		} {
			code, b := e.do(http.MethodGet, "/api/tasks"+q, nil)
			assertError(t, code, b, http.StatusBadRequest, handler.CodeValidation)
		}
	})
}

// ---------------------------------------------------------------- cache

func TestListCache(t *testing.T) {
	t.Run("second identical request is served from Redis", func(t *testing.T) {
		e := setup(t)
		e.create("Cached", nil)
		e.list("")
		e.list("")
		if n := e.repo.ListCalls(); n != 1 {
			t.Fatalf("repo.List called %d times, want 1", n)
		}
		if ttl := e.mr.TTL(cache.ListKey(1, mustParams(t, service.RawListQuery{}))); ttl != 60*time.Second {
			t.Fatalf("cache TTL = %v, want 60s", ttl)
		}
	})

	t.Run("equivalent queries share one key", func(t *testing.T) {
		e := setup(t)
		e.list("?keyword=Foo&status=todo")
		e.list("?status=TODO&keyword=%20foo%20&page=1&limit=10&sort=-created_at")
		if n := e.repo.ListCalls(); n != 1 {
			t.Fatalf("repo.List called %d times, want 1", n)
		}
	})

	t.Run("different queries do not collide", func(t *testing.T) {
		e := setup(t)
		e.list("?page=1")
		e.list("?page=2")
		if n := e.repo.ListCalls(); n != 2 {
			t.Fatalf("repo.List called %d times, want 2", n)
		}
	})

	t.Run("update bumps version and the next list sees new data", func(t *testing.T) {
		e := setup(t)
		task := e.create("Before", nil)
		if got := titles(e.list("")); got[0] != "Before" {
			t.Fatalf("got %v", got)
		}
		before, _ := e.mr.Get(cache.VersionKey)

		code, b := e.do(http.MethodPut, path(task.ID), map[string]any{"title": "After", "status": "done"})
		if code != http.StatusOK {
			t.Fatalf("update: %d %s", code, b)
		}
		after, _ := e.mr.Get(cache.VersionKey)
		if before == after {
			t.Fatalf("tasks:version not bumped (still %s)", after)
		}
		if got := titles(e.list("")); got[0] != "After" {
			t.Fatalf("stale list after update: %v", got)
		}
		if n := e.repo.ListCalls(); n != 2 {
			t.Fatalf("repo.List called %d times, want 2", n)
		}
	})

	t.Run("create and delete invalidate", func(t *testing.T) {
		e := setup(t)
		a := e.create("A", nil)
		if p := e.list(""); p.Meta.Total != 1 {
			t.Fatalf("total %d", p.Meta.Total)
		}
		e.create("B", nil)
		if p := e.list(""); p.Meta.Total != 2 {
			t.Fatalf("after create: total %d", p.Meta.Total)
		}
		e.do(http.MethodDelete, path(a.ID), nil)
		if p := e.list(""); p.Meta.Total != 1 {
			t.Fatalf("after delete: total %d", p.Meta.Total)
		}
	})

	t.Run("failed writes do not invalidate", func(t *testing.T) {
		e := setup(t)
		e.create("Dup", nil)
		before, _ := e.mr.Get(cache.VersionKey)
		e.do(http.MethodPost, "/api/tasks", map[string]any{"title": "Dup"})
		e.do(http.MethodPut, "/api/tasks/999", map[string]any{"title": "x"})
		e.do(http.MethodDelete, "/api/tasks/999", nil)
		if after, _ := e.mr.Get(cache.VersionKey); after != before {
			t.Fatalf("version changed on failed writes: %s -> %s", before, after)
		}
	})

	t.Run("entry expires after TTL", func(t *testing.T) {
		e := setup(t)
		e.list("")
		e.mr.FastForward(61 * time.Second)
		e.list("")
		if n := e.repo.ListCalls(); n != 2 {
			t.Fatalf("repo.List called %d times, want 2", n)
		}
	})

	t.Run("Redis down: reads and writes still work", func(t *testing.T) {
		e := setup(t)
		e.mr.Close()
		task := e.create("No cache", nil)
		if p := e.list(""); p.Meta.Total != 1 {
			t.Fatalf("total %d", p.Meta.Total)
		}
		if code, b := e.do(http.MethodPut, path(task.ID), map[string]any{"title": "Still works"}); code != http.StatusOK {
			t.Fatalf("update: %d %s", code, b)
		}
	})
}

func mustParams(t *testing.T, q service.RawListQuery) domain.ListParams {
	t.Helper()
	p, err := service.NormalizeListQuery(q)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// ---------------------------------------------------------------- routing

func TestUnknownRouteUsesErrorEnvelope(t *testing.T) {
	e := setup(t)
	code, b := e.do(http.MethodGet, "/api/nope", nil)
	assertError(t, code, b, http.StatusNotFound, handler.CodeNotFound)
	code, b = e.do(http.MethodPatch, "/api/tasks/1", nil)
	assertError(t, code, b, http.StatusMethodNotAllowed, handler.CodeMethodNotAllowed)
}
