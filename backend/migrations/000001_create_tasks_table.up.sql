CREATE TABLE IF NOT EXISTS tasks (
    id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    title        VARCHAR(200)    NOT NULL,
    description  TEXT            NULL,
    status       ENUM('todo', 'in_progress', 'done') NOT NULL DEFAULT 'todo',
    assignee     VARCHAR(100)    NULL,
    due_date     DATE            NULL,
    created_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at   DATETIME(3)     NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    deleted_at   DATETIME(3)     NULL,

    -- Only live rows carry a value; soft-deleted rows become NULL and
    -- InnoDB unique indexes allow any number of NULLs. This lets a title be
    -- reused after the original task has been soft-deleted.
    active_title VARCHAR(200) AS (IF(deleted_at IS NULL, title, NULL)) STORED,

    PRIMARY KEY (id),
    UNIQUE KEY uq_tasks_active_title (active_title),
    KEY idx_tasks_deleted_status (deleted_at, status),
    KEY idx_tasks_deleted_assignee (deleted_at, assignee),
    KEY idx_tasks_deleted_created (deleted_at, created_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_0900_ai_ci;
