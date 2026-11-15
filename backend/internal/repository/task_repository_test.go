package repository

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"

	"github.com/example/task-management/backend/internal/domain"
)

func TestMapWriteError(t *testing.T) {
	dup := &mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'x' for key 'tasks.uq_tasks_active_title'"}
	if err := mapWriteError(dup, "create"); !errors.Is(err, domain.ErrDuplicateTitle) {
		t.Fatalf("1062 mapped to %v", err)
	}
	other := &mysql.MySQLError{Number: 1452, Message: "fk"}
	if err := mapWriteError(other, "create"); errors.Is(err, domain.ErrDuplicateTitle) || !errors.Is(err, other) {
		t.Fatalf("1452 mapped to %v", err)
	}
}

func TestEscapeLike(t *testing.T) {
	if got := escapeLike(`50%_off\`); got != `50\%\_off\\` {
		t.Fatalf("got %q", got)
	}
}
