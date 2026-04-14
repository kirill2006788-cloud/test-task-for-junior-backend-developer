package task

import (
	"context"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository interface {
	Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]taskdomain.Task, error)
	ListTemplates(ctx context.Context) ([]taskdomain.Task, error)
	GetRecurrence(ctx context.Context, taskID int64) (*taskdomain.Recurrence, error)
	CreateRecurrence(ctx context.Context, rec *taskdomain.Recurrence) error
	UpdateRecurrence(ctx context.Context, rec *taskdomain.Recurrence) error
	DeleteRecurrence(ctx context.Context, taskID int64) error
	FindInstancesByParentAndDateRange(ctx context.Context, parentID int64, from, to time.Time) ([]taskdomain.Task, error)
	DeleteInstancesByParent(ctx context.Context, parentID int64) error
	DeleteFutureInstancesByParent(ctx context.Context, parentID int64, afterDate time.Time) error
}

type Usecase interface {
	Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]taskdomain.Task, error)
	ListTemplates(ctx context.Context) ([]taskdomain.Task, error)
	GenerateInstances(ctx context.Context, horizonDays int) error
}

type RecurrenceInput struct {
	RecurrenceType taskdomain.RecurrenceType `json:"recurrence_type"`
	IntervalDays   int                       `json:"interval_days,omitempty"`
	MonthDays      []int                     `json:"month_days,omitempty"`
	SpecificDates  []time.Time               `json:"specific_dates,omitempty"`
	OddEvenType    taskdomain.OddEvenType    `json:"odd_even_type,omitempty"`
	StartDate      time.Time                 `json:"start_date"`
	EndDate        *time.Time                `json:"end_date,omitempty"`
}

type CreateInput struct {
	Title       string
	Description string
	Status      taskdomain.Status
	Recurrence  *RecurrenceInput
}

type UpdateInput struct {
	Title       string
	Description string
	Status      taskdomain.Status
	Recurrence  *RecurrenceInput
}
