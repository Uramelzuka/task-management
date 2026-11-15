// Package repository persists tasks in MySQL.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"

	"github.com/example/task-management/backend/internal/domain"
)

// TaskRepository is the storage contract the service depends on.
type TaskRepository interface {
	List(ctx context.Context, p domain.ListParams) ([]domain.Task, int64, error)
	Get(ctx context.Context, id int64) (*domain.Task, error)
	Create(ctx context.Context, in domain.TaskInput) (int64, error)
	Update(ctx context.Context, id int64, in domain.TaskInput) error
	SoftDelete(ctx context.Context, id int64) error
}

const mysqlErrDuplicateEntry = 1062

type MySQLTaskRepository struct{ db *sql.DB }

func NewMySQLTaskRepository(db *sql.DB) *MySQLTaskRepository {
	return &MySQLTaskRepository{db: db}
}

const taskColumns = "id, title, description, status, assignee, due_date, created_at, updated_at"

// Every read path filters on deleted_at IS NULL so soft-deleted rows never leak.
const liveOnly = "deleted_at IS NULL"

func (r *MySQLTaskRepository) List(ctx context.Context, p domain.ListParams) ([]domain.Task, int64, error) {
	where := []string{liveOnly}
	args := []any{}
	if p.Status != "" {
		where = append(where, "status = ?")
		args = append(args, string(p.Status))
	}
	if p.Assignee != "" {
		where = append(where, "assignee = ?")
		args = append(args, p.Assignee)
	}
	if p.Keyword != "" {
		like := "%" + escapeLike(p.Keyword) + "%"
		where = append(where, "(title LIKE ? OR description LIKE ?)")
		args = append(args, like, like)
	}
	whereSQL := strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}
	if total == 0 {
		return []domain.Task{}, 0, nil
	}

	// SortBy has already been checked against domain.SortField; look it up
	// again here so this layer never interpolates an untrusted string.
	col, ok := domain.SortField[p.SortBy]
	if !ok {
		col = "created_at"
	}
	dir := "ASC"
	if p.SortDesc {
		dir = "DESC"
	}
	query := fmt.Sprintf("SELECT %s FROM tasks WHERE %s ORDER BY %s %s, id %s LIMIT ? OFFSET ?",
		taskColumns, whereSQL, col, dir, dir)
	args = append(args, p.Limit, (p.Page-1)*p.Limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	tasks := []domain.Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	return tasks, total, nil
}

func (r *MySQLTaskRepository) Get(ctx context.Context, id int64) (*domain.Task, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id = ? AND "+liveOnly, id)
	t, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	return t, err
}

func (r *MySQLTaskRepository) Create(ctx context.Context, in domain.TaskInput) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO tasks (title, description, status, assignee, due_date) VALUES (?, ?, ?, ?, ?)",
		in.Title, in.Description, string(in.Status), in.Assignee, dateArg(in.DueDate))
	if err != nil {
		return 0, mapWriteError(err, "create task")
	}
	return res.LastInsertId()
}

// Update relies on clientFoundRows=true in the DSN: without it MySQL reports
// 0 affected rows when the new values equal the old ones, which would be
// misread as "not found".
func (r *MySQLTaskRepository) Update(ctx context.Context, id int64, in domain.TaskInput) error {
	res, err := r.db.ExecContext(ctx,
		"UPDATE tasks SET title = ?, description = ?, status = ?, assignee = ?, due_date = ? WHERE id = ? AND "+liveOnly,
		in.Title, in.Description, string(in.Status), in.Assignee, dateArg(in.DueDate), id)
	if err != nil {
		return mapWriteError(err, "update task")
	}
	return requireAffected(res)
}

func (r *MySQLTaskRepository) SoftDelete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx,
		"UPDATE tasks SET deleted_at = CURRENT_TIMESTAMP(3) WHERE id = ? AND "+liveOnly, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return requireAffected(res)
}

type scanner interface{ Scan(dest ...any) error }

func scanTask(s scanner) (*domain.Task, error) {
	var (
		t       domain.Task
		status  string
		desc    sql.NullString
		assign  sql.NullString
		dueDate sql.NullTime
	)
	if err := s.Scan(&t.ID, &t.Title, &desc, &status, &assign, &dueDate, &t.CreatedAt, &t.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("scan task: %w", err)
	}
	t.Status = domain.Status(status)
	if desc.Valid {
		t.Description = &desc.String
	}
	if assign.Valid {
		t.Assignee = &assign.String
	}
	if dueDate.Valid {
		t.DueDate = &domain.Date{Time: dueDate.Time}
	}
	return &t, nil
}

func requireAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// mapWriteError turns MySQL error 1062 (duplicate key on uq_tasks_active_title)
// into domain.ErrDuplicateTitle.
func mapWriteError(err error, op string) error {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == mysqlErrDuplicateEntry {
		return domain.ErrDuplicateTitle
	}
	return fmt.Errorf("%s: %w", op, err)
}

func dateArg(d *domain.Date) any {
	if d == nil {
		return nil
	}
	return d.Format("2006-01-02")
}

// escapeLike makes %, _ and \ in user input match literally.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
