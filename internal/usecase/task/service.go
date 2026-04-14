package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

const defaultHorizonDays = 30

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error) {
	normalized, err := validateCreateInput(input)
	if err != nil {
		return nil, err
	}

	isTemplate := normalized.Recurrence != nil

	model := &taskdomain.Task{
		Title:       normalized.Title,
		Description: normalized.Description,
		Status:      normalized.Status,
		IsTemplate:  isTemplate,
	}
	now := s.now()
	model.CreatedAt = now
	model.UpdatedAt = now

	created, err := s.repo.Create(ctx, model)
	if err != nil {
		return nil, err
	}

	if isTemplate {
		rec := toRecurrence(created.ID, normalized.Recurrence)
		if err := validateRecurrence(rec); err != nil {
			_ = s.repo.Delete(ctx, created.ID)
			return nil, err
		}
		if err := s.repo.CreateRecurrence(ctx, rec); err != nil {
			_ = s.repo.Delete(ctx, created.ID)
			return nil, err
		}

		// Generate initial instances
		if err := s.generateInstancesForTemplate(ctx, created, rec); err != nil {
			return nil, err
		}

		// Reload to get updated recurrence info
		created, err = s.repo.GetByID(ctx, created.ID)
		if err != nil {
			return nil, err
		}
	}

	return created, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	normalized, err := validateUpdateInput(input)
	if err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	model := &taskdomain.Task{
		ID:          id,
		Title:       normalized.Title,
		Description: normalized.Description,
		Status:      normalized.Status,
		IsTemplate:  existing.IsTemplate,
		ParentID:    existing.ParentID,
		DueDate:     existing.DueDate,
		UpdatedAt:   s.now(),
	}

	updated, err := s.repo.Update(ctx, model)
	if err != nil {
		return nil, err
	}

	// Handle recurrence update for templates
	if existing.IsTemplate {
		if normalized.Recurrence != nil {
			rec := toRecurrence(id, normalized.Recurrence)
			if err := validateRecurrence(rec); err != nil {
				return nil, err
			}

			// Delete future new instances and regenerate
			now := s.now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			_ = s.repo.DeleteFutureInstancesByParent(ctx, id, today)

			if err := s.repo.UpdateRecurrence(ctx, rec); err != nil {
				return nil, err
			}

			if err := s.generateInstancesForTemplate(ctx, updated, rec); err != nil {
				return nil, err
			}
		} else {
			// Removing recurrence from template — convert to regular task
			model.IsTemplate = false
			_ = s.repo.DeleteRecurrence(ctx, id)
			_ = s.repo.DeleteInstancesByParent(ctx, id)

			updated, err = s.repo.Update(ctx, model)
			if err != nil {
				return nil, err
			}
		}
	}

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if existing.IsTemplate {
		// Delete all instances and recurrence settings
		_ = s.repo.DeleteInstancesByParent(ctx, id)
		_ = s.repo.DeleteRecurrence(ctx, id)
	}

	return s.repo.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]taskdomain.Task, error) {
	return s.repo.List(ctx)
}

func (s *Service) ListTemplates(ctx context.Context) ([]taskdomain.Task, error) {
	return s.repo.ListTemplates(ctx)
}

// GenerateInstances generates task instances for all templates within the horizon.
// This is called by the scheduler.
func (s *Service) GenerateInstances(ctx context.Context, horizonDays int) error {
	if horizonDays <= 0 {
		horizonDays = defaultHorizonDays
	}

	templates, err := s.repo.ListTemplates(ctx)
	if err != nil {
		return err
	}

	for i := range templates {
		rec, err := s.repo.GetRecurrence(ctx, templates[i].ID)
		if err != nil || rec == nil {
			continue
		}

		if err := s.generateInstancesForTemplate(ctx, &templates[i], rec); err != nil {
			continue
		}
	}

	return nil
}

// generateInstancesForTemplate creates task instances for a template within the horizon.
func (s *Service) generateInstancesForTemplate(ctx context.Context, template *taskdomain.Task, rec *taskdomain.Recurrence) error {
	now := s.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	horizonEnd := today.AddDate(0, 0, defaultHorizonDays)

	// Determine the effective start date
	startDate := rec.StartDate
	if startDate.Before(today) {
		startDate = today
	}

	// Determine the effective end date
	var endDate time.Time
	if rec.EndDate != nil && rec.EndDate.Before(horizonEnd) {
		endDate = *rec.EndDate
	} else {
		endDate = horizonEnd
	}

	// Find already existing instances in the date range
	existing, err := s.repo.FindInstancesByParentAndDateRange(ctx, template.ID, startDate, endDate)
	if err != nil {
		return err
	}

	existingDates := make(map[string]bool, len(existing))
	for _, inst := range existing {
		if inst.DueDate != nil {
			existingDates[inst.DueDate.Format("2006-01-02")] = true
		}
	}

	// Generate instances for each matching date
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateKey := d.Format("2006-01-02")
		if existingDates[dateKey] {
			continue
		}

		if !rec.MatchesDate(d) {
			continue
		}

		instance := &taskdomain.Task{
			Title:       template.Title,
			Description: template.Description,
			Status:      taskdomain.StatusNew,
			IsTemplate:  false,
			ParentID:    &template.ID,
			DueDate:     &d,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if _, err := s.repo.Create(ctx, instance); err != nil {
			return err
		}
	}

	return nil
}

func toRecurrence(taskID int64, input *RecurrenceInput) *taskdomain.Recurrence {
	return &taskdomain.Recurrence{
		TaskID:         taskID,
		RecurrenceType: input.RecurrenceType,
		IntervalDays:   input.IntervalDays,
		MonthDays:      input.MonthDays,
		SpecificDates:  input.SpecificDates,
		OddEvenType:    input.OddEvenType,
		StartDate:      input.StartDate,
		EndDate:        input.EndDate,
	}
}

func validateRecurrence(rec *taskdomain.Recurrence) error {
	if !rec.RecurrenceType.Valid() {
		return fmt.Errorf("%w: invalid recurrence_type", ErrInvalidInput)
	}

	if rec.StartDate.IsZero() {
		return fmt.Errorf("%w: start_date is required for recurrence", ErrInvalidInput)
	}

	if rec.EndDate != nil && rec.EndDate.Before(rec.StartDate) {
		return fmt.Errorf("%w: end_date must be after start_date", ErrInvalidInput)
	}

	switch rec.RecurrenceType {
	case taskdomain.RecurrenceDaily:
		if rec.IntervalDays <= 0 {
			return fmt.Errorf("%w: interval_days must be positive for daily recurrence", ErrInvalidInput)
		}
	case taskdomain.RecurrenceMonthly:
		if len(rec.MonthDays) == 0 {
			return fmt.Errorf("%w: month_days is required for monthly recurrence", ErrInvalidInput)
		}
		for _, d := range rec.MonthDays {
			if d < 1 || d > 31 {
				return fmt.Errorf("%w: month_days must be between 1 and 31", ErrInvalidInput)
			}
		}
	case taskdomain.RecurrenceSpecificDates:
		if len(rec.SpecificDates) == 0 {
			return fmt.Errorf("%w: specific_dates is required for specific_dates recurrence", ErrInvalidInput)
		}
	case taskdomain.RecurrenceOddEven:
		if !rec.OddEvenType.Valid() {
			return fmt.Errorf("%w: odd_even_type must be 'odd' or 'even'", ErrInvalidInput)
		}
	}

	return nil
}

func validateCreateInput(input CreateInput) (CreateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return CreateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if input.Status == "" {
		input.Status = taskdomain.StatusNew
	}

	if !input.Status.Valid() {
		return CreateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	return input, nil
}

func validateUpdateInput(input UpdateInput) (UpdateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return UpdateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if !input.Status.Valid() {
		return UpdateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	return input, nil
}
