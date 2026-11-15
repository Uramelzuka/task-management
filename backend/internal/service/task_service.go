// Package service holds the task use cases: validation, normalisation and
// cache orchestration around the repository.
package service

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/example/task-management/backend/internal/cache"
	"github.com/example/task-management/backend/internal/domain"
	"github.com/example/task-management/backend/internal/repository"
)

const (
	DefaultPage  = 1
	DefaultLimit = 10
	MaxLimit     = 100
	DefaultSort  = "-created_at"

	maxTitleLen    = 200
	maxAssigneeLen = 100
	maxKeywordLen  = 100
	maxDescLen     = 5000
)

type TaskService struct {
	repo  repository.TaskRepository
	cache cache.TaskCache
	log   *slog.Logger
}

func NewTaskService(repo repository.TaskRepository, c cache.TaskCache, log *slog.Logger) *TaskService {
	if log == nil {
		log = slog.Default()
	}
	return &TaskService{repo: repo, cache: c, log: log}
}

// RawListQuery is the list query exactly as it arrived on the URL.
type RawListQuery struct {
	Status, Keyword, Assignee, Page, Limit, Sort string
}

// NormalizeListQuery validates and canonicalises a raw list query.
func NormalizeListQuery(q RawListQuery) (domain.ListParams, error) {
	p := domain.ListParams{Page: DefaultPage, Limit: DefaultLimit}

	if s := strings.ToLower(strings.TrimSpace(q.Status)); s != "" {
		if !domain.Status(s).Valid() {
			return p, &domain.ValidationError{Message: "status must be one of: todo, in_progress, done"}
		}
		p.Status = domain.Status(s)
	}

	// Keyword search is case-insensitive in MySQL (ai_ci collation), so
	// lower-casing here only merges cache entries, it never changes results.
	p.Keyword = strings.ToLower(strings.Join(strings.Fields(q.Keyword), " "))
	if utf8.RuneCountInString(p.Keyword) > maxKeywordLen {
		return p, &domain.ValidationError{Message: "keyword must be at most 100 characters"}
	}
	p.Assignee = strings.TrimSpace(q.Assignee)

	var err error
	if p.Page, err = parsePositive(q.Page, DefaultPage, "page"); err != nil {
		return p, err
	}
	if p.Limit, err = parsePositive(q.Limit, DefaultLimit, "limit"); err != nil {
		return p, err
	}
	if p.Limit > MaxLimit {
		return p, &domain.ValidationError{Message: "limit must be at most 100"}
	}

	sort := strings.ToLower(strings.TrimSpace(q.Sort))
	if sort == "" {
		sort = DefaultSort
	}
	if strings.HasPrefix(sort, "-") {
		p.SortDesc = true
		sort = sort[1:]
	}
	if _, ok := domain.SortField[sort]; !ok {
		return p, &domain.ValidationError{Message: "sort must be one of: id, title, status, assignee, due_date, created_at, updated_at (prefix with - for descending)"}
	}
	p.SortBy = sort
	return p, nil
}

func parsePositive(raw string, def int, name string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, &domain.ValidationError{Message: name + " must be a positive integer"}
	}
	return n, nil
}

// List serves from cache when possible. Redis failures are logged and the
// request falls through to MySQL: the cache is an optimisation, not a
// dependency.
func (s *TaskService) List(ctx context.Context, p domain.ListParams) (*domain.TaskPage, error) {
	// Read the version once and use the same key for get and set. If a write
	// lands between our DB read and our SET, we store under the old version,
	// which nobody reads any more, so stale data cannot be served.
	key := ""
	if v, err := s.cache.Version(ctx); err != nil {
		s.log.Warn("cache version read failed", "err", err)
	} else {
		key = cache.ListKey(v, p)
		page, hit, err := s.cache.GetList(ctx, key)
		if err != nil {
			s.log.Warn("cache get failed", "key", key, "err", err)
		} else if hit {
			return page, nil
		}
	}

	tasks, total, err := s.repo.List(ctx, p)
	if err != nil {
		return nil, err
	}
	page := &domain.TaskPage{
		Data: tasks,
		Meta: domain.PageMeta{
			Page:       p.Page,
			Limit:      p.Limit,
			Total:      total,
			TotalPages: int((total + int64(p.Limit) - 1) / int64(p.Limit)),
		},
	}
	if key != "" {
		if err := s.cache.SetList(ctx, key, page); err != nil {
			s.log.Warn("cache set failed", "key", key, "err", err)
		}
	}
	return page, nil
}

func (s *TaskService) Get(ctx context.Context, id int64) (*domain.Task, error) {
	return s.repo.Get(ctx, id)
}

func (s *TaskService) Create(ctx context.Context, in domain.TaskInput) (*domain.Task, error) {
	in, err := normalizeInput(in)
	if err != nil {
		return nil, err
	}
	id, err := s.repo.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	s.invalidate(ctx)
	return s.repo.Get(ctx, id)
}

func (s *TaskService) Update(ctx context.Context, id int64, in domain.TaskInput) (*domain.Task, error) {
	in, err := normalizeInput(in)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, id, in); err != nil {
		return nil, err
	}
	s.invalidate(ctx)
	return s.repo.Get(ctx, id)
}

func (s *TaskService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}

// invalidate runs after a successful write. If Redis is down, cached lists
// can be stale for at most one TTL (60s); we accept that rather than failing
// a write that has already committed.
func (s *TaskService) invalidate(ctx context.Context) {
	if err := s.cache.Invalidate(ctx); err != nil {
		s.log.Error("cache invalidation failed; lists may be stale until TTL", "err", err)
	}
}

func normalizeInput(in domain.TaskInput) (domain.TaskInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return in, &domain.ValidationError{Message: "title is required"}
	}
	if utf8.RuneCountInString(in.Title) > maxTitleLen {
		return in, &domain.ValidationError{Message: "title must be at most 200 characters"}
	}
	if in.Status == "" {
		in.Status = domain.StatusTodo
	}
	if !in.Status.Valid() {
		return in, &domain.ValidationError{Message: "status must be one of: todo, in_progress, done"}
	}
	in.Description = trimOptional(in.Description)
	if in.Description != nil && utf8.RuneCountInString(*in.Description) > maxDescLen {
		return in, &domain.ValidationError{Message: "description must be at most 5000 characters"}
	}
	in.Assignee = trimOptional(in.Assignee)
	if in.Assignee != nil && utf8.RuneCountInString(*in.Assignee) > maxAssigneeLen {
		return in, &domain.ValidationError{Message: "assignee must be at most 100 characters"}
	}
	return in, nil
}

// trimOptional treats a blank string as "not set".
func trimOptional(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}
