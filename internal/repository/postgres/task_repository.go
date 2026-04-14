package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		INSERT INTO tasks (title, description, status, is_template, parent_id, due_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, title, description, status, is_template, parent_id, due_date, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		task.Title, task.Description, task.Status,
		task.IsTemplate, task.ParentID, task.DueDate,
		task.CreatedAt, task.UpdatedAt,
	)
	created, err := scanTask(row)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, is_template, parent_id, due_date, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return found, nil
}

func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		UPDATE tasks
		SET title = $1,
			description = $2,
			status = $3,
			is_template = $4,
			parent_id = $5,
			due_date = $6,
			updated_at = $7
		WHERE id = $8
		RETURNING id, title, description, status, is_template, parent_id, due_date, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		task.Title, task.Description, task.Status,
		task.IsTemplate, task.ParentID, task.DueDate,
		task.UpdatedAt, task.ID,
	)
	updated, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return updated, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}

func (r *Repository) List(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, is_template, parent_id, due_date, created_at, updated_at
		FROM tasks
		WHERE is_template = FALSE
		ORDER BY due_date ASC NULLS LAST, id DESC
	`

	return queryTasks(ctx, r.pool, query)
}

func (r *Repository) ListTemplates(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, is_template, parent_id, due_date, created_at, updated_at
		FROM tasks
		WHERE is_template = TRUE
		ORDER BY id DESC
	`

	return queryTasks(ctx, r.pool, query)
}

func (r *Repository) GetRecurrence(ctx context.Context, taskID int64) (*taskdomain.Recurrence, error) {
	const query = `
		SELECT task_id, recurrence_type, interval_days, month_days, specific_dates, odd_even_type, start_date, end_date
		FROM task_recurrence
		WHERE task_id = $1
	`

	row := r.pool.QueryRow(ctx, query, taskID)
	var (
		rec           taskdomain.Recurrence
		endDate       *time.Time
		intervalDays  *int
		monthDays     []int
		specificDates []time.Time
		oddEvenType   *string
		recType       string
		startDate     time.Time
	)

	if err := row.Scan(
		&rec.TaskID, &recType, &intervalDays,
		&monthDays, &specificDates, &oddEvenType,
		&startDate, &endDate,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	rec.RecurrenceType = taskdomain.RecurrenceType(recType)
	rec.StartDate = startDate
	if intervalDays != nil {
		rec.IntervalDays = *intervalDays
	}
	rec.MonthDays = monthDays
	rec.SpecificDates = specificDates
	if oddEvenType != nil {
		rec.OddEvenType = taskdomain.OddEvenType(*oddEvenType)
	}
	rec.EndDate = endDate
	return &rec, nil
}

func (r *Repository) CreateRecurrence(ctx context.Context, rec *taskdomain.Recurrence) error {
	const query = `
		INSERT INTO task_recurrence (task_id, recurrence_type, interval_days, month_days, specific_dates, odd_even_type, start_date, end_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.pool.Exec(ctx, query,
		rec.TaskID, rec.RecurrenceType,
		nullInt(rec.IntervalDays), nullSlice(rec.MonthDays), nullSlice(rec.SpecificDates), nullString(string(rec.OddEvenType)),
		rec.StartDate, rec.EndDate,
	)
	return err
}

func (r *Repository) UpdateRecurrence(ctx context.Context, rec *taskdomain.Recurrence) error {
	const query = `
		UPDATE task_recurrence
		SET recurrence_type = $2, interval_days = $3, month_days = $4,
		    specific_dates = $5, odd_even_type = $6, start_date = $7, end_date = $8
		WHERE task_id = $1
	`

	_, err := r.pool.Exec(ctx, query,
		rec.TaskID, rec.RecurrenceType,
		nullInt(rec.IntervalDays), nullSlice(rec.MonthDays), nullSlice(rec.SpecificDates), nullString(string(rec.OddEvenType)),
		rec.StartDate, rec.EndDate,
	)
	return err
}

func (r *Repository) DeleteRecurrence(ctx context.Context, taskID int64) error {
	const query = `DELETE FROM task_recurrence WHERE task_id = $1`

	_, err := r.pool.Exec(ctx, query, taskID)
	return err
}

func (r *Repository) FindInstancesByParentAndDateRange(ctx context.Context, parentID int64, from, to time.Time) ([]taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, is_template, parent_id, due_date, created_at, updated_at
		FROM tasks
		WHERE parent_id = $1 AND due_date >= $2 AND due_date <= $3
		ORDER BY due_date ASC
	`

	rows, err := r.pool.Query(ctx, query, parentID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (r *Repository) DeleteInstancesByParent(ctx context.Context, parentID int64) error {
	const query = `DELETE FROM tasks WHERE parent_id = $1`

	_, err := r.pool.Exec(ctx, query, parentID)
	return err
}

func (r *Repository) DeleteFutureInstancesByParent(ctx context.Context, parentID int64, afterDate time.Time) error {
	const query = `DELETE FROM tasks WHERE parent_id = $1 AND due_date >= $2 AND status = 'new'`

	_, err := r.pool.Exec(ctx, query, parentID, afterDate)
	return err
}

// nullInt returns nil for zero int, otherwise a pointer to the value.
func nullInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

// nullString returns nil for empty string, otherwise the value.
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullSlice returns nil for empty/nil slice, otherwise the slice.
func nullSlice(v any) any {
	if v == nil {
		return nil
	}
	switch s := v.(type) {
	case []int:
		if len(s) == 0 {
			return nil
		}
		return s
	case []time.Time:
		if len(s) == 0 {
			return nil
		}
		return s
	}
	return v
}

func queryTasks(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) ([]taskdomain.Task, error) {
	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		task       taskdomain.Task
		status     string
		parentID   *int64
		dueDate    *time.Time
	)

	if err := scanner.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&status,
		&task.IsTemplate,
		&parentID,
		&dueDate,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		return nil, err
	}

	task.Status = taskdomain.Status(status)
	task.ParentID = parentID
	task.DueDate = dueDate

	return &task, nil
}
