package handlers

import (
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
	taskusecase "example.com/taskservice/internal/usecase/task"
)

type recurrenceDTO struct {
	RecurrenceType string     `json:"recurrence_type"`
	IntervalDays   int        `json:"interval_days,omitempty"`
	MonthDays      []int      `json:"month_days,omitempty"`
	SpecificDates  []string   `json:"specific_dates,omitempty"`
	OddEvenType    string     `json:"odd_even_type,omitempty"`
	StartDate      string     `json:"start_date"`
	EndDate        *string    `json:"end_date,omitempty"`
}

type taskMutationDTO struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      taskdomain.Status `json:"status"`
	Recurrence  *recurrenceDTO    `json:"recurrence,omitempty"`
}

type taskDTO struct {
	ID          int64             `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      taskdomain.Status `json:"status"`
	IsTemplate  bool              `json:"is_template"`
	ParentID    *int64            `json:"parent_id,omitempty"`
	DueDate     *string           `json:"due_date,omitempty"`
	Recurrence  *recurrenceDTO    `json:"recurrence,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

func newTaskDTO(task *taskdomain.Task, rec *taskdomain.Recurrence) taskDTO {
	dto := taskDTO{
		ID:          task.ID,
		Title:       task.Title,
		Description: task.Description,
		Status:      task.Status,
		IsTemplate:  task.IsTemplate,
		ParentID:    task.ParentID,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
	}

	if task.DueDate != nil {
		d := task.DueDate.Format("2006-01-02")
		dto.DueDate = &d
	}

	if rec != nil {
		dto.Recurrence = newRecurrenceDTO(rec)
	}

	return dto
}

func newRecurrenceDTO(rec *taskdomain.Recurrence) *recurrenceDTO {
	dto := &recurrenceDTO{
		RecurrenceType: string(rec.RecurrenceType),
		IntervalDays:   rec.IntervalDays,
		MonthDays:      rec.MonthDays,
		OddEvenType:    string(rec.OddEvenType),
		StartDate:      rec.StartDate.Format("2006-01-02"),
	}

	if len(rec.SpecificDates) > 0 {
		dto.SpecificDates = make([]string, len(rec.SpecificDates))
		for i, d := range rec.SpecificDates {
			dto.SpecificDates[i] = d.Format("2006-01-02")
		}
	}

	if rec.EndDate != nil {
		d := rec.EndDate.Format("2006-01-02")
		dto.EndDate = &d
	}

	return dto
}

func toRecurrenceInput(dto *recurrenceDTO) (*taskusecase.RecurrenceInput, error) {
	if dto == nil {
		return nil, nil
	}

	input := &taskusecase.RecurrenceInput{
		RecurrenceType: taskdomain.RecurrenceType(dto.RecurrenceType),
		IntervalDays:   dto.IntervalDays,
		MonthDays:      dto.MonthDays,
		OddEvenType:    taskdomain.OddEvenType(dto.OddEvenType),
	}

	startDate, err := time.Parse("2006-01-02", dto.StartDate)
	if err != nil {
		return nil, err
	}
	input.StartDate = startDate

	if dto.EndDate != nil {
		endDate, err := time.Parse("2006-01-02", *dto.EndDate)
		if err != nil {
			return nil, err
		}
		input.EndDate = &endDate
	}

	if len(dto.SpecificDates) > 0 {
		input.SpecificDates = make([]time.Time, len(dto.SpecificDates))
		for i, ds := range dto.SpecificDates {
			d, err := time.Parse("2006-01-02", ds)
			if err != nil {
				return nil, err
			}
			input.SpecificDates[i] = d
		}
	}

	return input, nil
}
