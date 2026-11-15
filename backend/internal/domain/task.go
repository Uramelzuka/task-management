// Package domain holds the core types shared by every layer.
package domain

import (
	"errors"
	"time"
)

type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

func (s Status) Valid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusDone:
		return true
	}
	return false
}

// Date is a calendar date serialised as "YYYY-MM-DD".
type Date struct{ time.Time }

const dateLayout = "2006-01-02"

func (d Date) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.Format(dateLayout) + `"`), nil
}

func (d *Date) UnmarshalJSON(b []byte) error {
	s := string(b)
	if len(s) < 2 || s[0] != '"' || s[len(s)-1] != '"' {
		return errors.New("date must be a string in YYYY-MM-DD format")
	}
	t, err := time.Parse(dateLayout, s[1:len(s)-1])
	if err != nil {
		return errors.New("date must be in YYYY-MM-DD format")
	}
	d.Time = t
	return nil
}

type Task struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	Status      Status    `json:"status"`
	Assignee    *string   `json:"assignee"`
	DueDate     *Date     `json:"due_date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TaskInput is the validated payload for create and update (PUT = full replace).
type TaskInput struct {
	Title       string
	Description *string
	Status      Status
	Assignee    *string
	DueDate     *Date
}

// SortField maps a public sort key to a trusted SQL column. Only keys present
// here can ever reach an ORDER BY clause.
var SortField = map[string]string{
	"id":         "id",
	"title":      "title",
	"status":     "status",
	"assignee":   "assignee",
	"due_date":   "due_date",
	"created_at": "created_at",
	"updated_at": "updated_at",
}

// ListParams is the normalised form of the list query. Two requests that mean
// the same thing produce identical ListParams, which is what makes it safe to
// use as a cache key.
type ListParams struct {
	Status   Status // empty = any
	Keyword  string // empty = any
	Assignee string // empty = any
	Page     int
	Limit    int
	SortBy   string // key of SortField
	SortDesc bool
}

type PageMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type TaskPage struct {
	Data []Task   `json:"data"`
	Meta PageMeta `json:"meta"`
}

var (
	ErrNotFound       = errors.New("task not found")
	ErrDuplicateTitle = errors.New("a task with this title already exists")
)

// ValidationError carries a client-facing message about bad input.
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }
