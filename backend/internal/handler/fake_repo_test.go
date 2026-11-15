package handler_test

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/example/task-management/backend/internal/domain"
)

// fakeRepo is an in-memory repository.TaskRepository that mimics the MySQL
// semantics the API depends on: soft delete, unique title among live rows,
// case-insensitive LIKE search, whitelisted sort and paging.
type fakeRepo struct {
	mu        sync.Mutex
	nextID    int64
	rows      map[int64]*row
	listCalls int
}

type row struct {
	task    domain.Task
	deleted bool
}

func newFakeRepo() *fakeRepo { return &fakeRepo{nextID: 1, rows: map[int64]*row{}} }

func (f *fakeRepo) ListCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.listCalls
}

func (f *fakeRepo) titleTaken(title string, exceptID int64) bool {
	for id, r := range f.rows {
		if !r.deleted && id != exceptID && strings.EqualFold(r.task.Title, title) {
			return true
		}
	}
	return false
}

func (f *fakeRepo) List(_ context.Context, p domain.ListParams) ([]domain.Task, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls++

	var out []domain.Task
	for _, r := range f.rows {
		t := r.task
		if r.deleted {
			continue
		}
		if p.Status != "" && t.Status != p.Status {
			continue
		}
		if p.Assignee != "" && (t.Assignee == nil || *t.Assignee != p.Assignee) {
			continue
		}
		if p.Keyword != "" {
			hay := strings.ToLower(t.Title)
			if t.Description != nil {
				hay += "\n" + strings.ToLower(*t.Description)
			}
			if !strings.Contains(hay, strings.ToLower(p.Keyword)) {
				continue
			}
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		var less, eq bool
		switch p.SortBy {
		case "title":
			less, eq = a.Title < b.Title, a.Title == b.Title
		default: // id / created_at: insertion order
			less, eq = a.ID < b.ID, a.ID == b.ID
		}
		if eq {
			less = a.ID < b.ID
		}
		if p.SortDesc {
			return !less
		}
		return less
	})
	total := int64(len(out))
	start := (p.Page - 1) * p.Limit
	if start > len(out) {
		start = len(out)
	}
	end := start + p.Limit
	if end > len(out) {
		end = len(out)
	}
	return append([]domain.Task{}, out[start:end]...), total, nil
}

func (f *fakeRepo) Get(_ context.Context, id int64) (*domain.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[id]
	if !ok || r.deleted {
		return nil, domain.ErrNotFound
	}
	t := r.task
	return &t, nil
}

func (f *fakeRepo) Create(_ context.Context, in domain.TaskInput) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.titleTaken(in.Title, 0) {
		return 0, domain.ErrDuplicateTitle
	}
	id := f.nextID
	f.nextID++
	now := time.Now().UTC()
	f.rows[id] = &row{task: domain.Task{
		ID: id, Title: in.Title, Description: in.Description, Status: in.Status,
		Assignee: in.Assignee, DueDate: in.DueDate, CreatedAt: now, UpdatedAt: now,
	}}
	return id, nil
}

func (f *fakeRepo) Update(_ context.Context, id int64, in domain.TaskInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[id]
	if !ok || r.deleted {
		return domain.ErrNotFound
	}
	if f.titleTaken(in.Title, id) {
		return domain.ErrDuplicateTitle
	}
	r.task.Title, r.task.Description, r.task.Status = in.Title, in.Description, in.Status
	r.task.Assignee, r.task.DueDate, r.task.UpdatedAt = in.Assignee, in.DueDate, time.Now().UTC()
	return nil
}

func (f *fakeRepo) SoftDelete(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[id]
	if !ok || r.deleted {
		return domain.ErrNotFound
	}
	r.deleted = true
	return nil
}
