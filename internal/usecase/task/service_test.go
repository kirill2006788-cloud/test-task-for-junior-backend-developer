package task

import (
	"context"
	"testing"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

// mockRepo implements the Repository interface for testing.
type mockRepo struct {
	tasks       map[int64]*taskdomain.Task
	recurrences map[int64]*taskdomain.Recurrence
	nextID      int64
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		tasks:       make(map[int64]*taskdomain.Task),
		recurrences: make(map[int64]*taskdomain.Recurrence),
		nextID:      1,
	}
}

func (m *mockRepo) Create(_ context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	task.ID = m.nextID
	m.nextID++
	clone := *task
	m.tasks[clone.ID] = &clone
	return &clone, nil
}

func (m *mockRepo) GetByID(_ context.Context, id int64) (*taskdomain.Task, error) {
	t, ok := m.tasks[id]
	if !ok {
		return nil, taskdomain.ErrNotFound
	}
	clone := *t
	return &clone, nil
}

func (m *mockRepo) Update(_ context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	if _, ok := m.tasks[task.ID]; !ok {
		return nil, taskdomain.ErrNotFound
	}
	clone := *task
	m.tasks[clone.ID] = &clone
	return &clone, nil
}

func (m *mockRepo) Delete(_ context.Context, id int64) error {
	if _, ok := m.tasks[id]; !ok {
		return taskdomain.ErrNotFound
	}
	delete(m.tasks, id)
	delete(m.recurrences, id)
	return nil
}

func (m *mockRepo) List(_ context.Context) ([]taskdomain.Task, error) {
	var result []taskdomain.Task
	for _, t := range m.tasks {
		if !t.IsTemplate {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (m *mockRepo) ListTemplates(_ context.Context) ([]taskdomain.Task, error) {
	var result []taskdomain.Task
	for _, t := range m.tasks {
		if t.IsTemplate {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (m *mockRepo) GetRecurrence(_ context.Context, taskID int64) (*taskdomain.Recurrence, error) {
	r, ok := m.recurrences[taskID]
	if !ok {
		return nil, nil
	}
	clone := *r
	return &clone, nil
}

func (m *mockRepo) CreateRecurrence(_ context.Context, rec *taskdomain.Recurrence) error {
	clone := *rec
	m.recurrences[clone.TaskID] = &clone
	return nil
}

func (m *mockRepo) UpdateRecurrence(_ context.Context, rec *taskdomain.Recurrence) error {
	clone := *rec
	m.recurrences[clone.TaskID] = &clone
	return nil
}

func (m *mockRepo) DeleteRecurrence(_ context.Context, taskID int64) error {
	delete(m.recurrences, taskID)
	return nil
}

func (m *mockRepo) FindInstancesByParentAndDateRange(_ context.Context, parentID int64, from, to time.Time) ([]taskdomain.Task, error) {
	var result []taskdomain.Task
	for _, t := range m.tasks {
		if t.ParentID != nil && *t.ParentID == parentID && t.DueDate != nil {
			if !t.DueDate.Before(from) && !t.DueDate.After(to) {
				result = append(result, *t)
			}
		}
	}
	return result, nil
}

func (m *mockRepo) DeleteInstancesByParent(_ context.Context, parentID int64) error {
	for id, t := range m.tasks {
		if t.ParentID != nil && *t.ParentID == parentID {
			delete(m.tasks, id)
		}
	}
	return nil
}

func (m *mockRepo) DeleteFutureInstancesByParent(_ context.Context, parentID int64, afterDate time.Time) error {
	for id, t := range m.tasks {
		if t.ParentID != nil && *t.ParentID == parentID && t.DueDate != nil {
			if !t.DueDate.Before(afterDate) && t.Status == taskdomain.StatusNew {
				delete(m.tasks, id)
			}
		}
	}
	return nil
}

func fixedNow() time.Time {
	return time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)
}

func TestCreateTaskWithoutRecurrence(t *testing.T) {
	repo := newMockRepo()
	svc := &Service{repo: repo, now: fixedNow}

	task, err := svc.Create(context.Background(), CreateInput{
		Title:       "Simple task",
		Description: "One-time task",
		Status:      taskdomain.StatusNew,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.IsTemplate {
		t.Error("task should not be a template")
	}
	if task.ParentID != nil {
		t.Error("task should not have parent_id")
	}
	if task.DueDate != nil {
		t.Error("task should not have due_date")
	}
}

func TestCreateDailyRecurrence(t *testing.T) {
	repo := newMockRepo()
	svc := &Service{repo: repo, now: fixedNow}

	startDate := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	task, err := svc.Create(context.Background(), CreateInput{
		Title:       "Daily standup",
		Description: "Daily team standup",
		Status:      taskdomain.StatusNew,
		Recurrence: &RecurrenceInput{
			RecurrenceType: taskdomain.RecurrenceDaily,
			IntervalDays:   2,
			StartDate:      startDate,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !task.IsTemplate {
		t.Error("task should be a template")
	}

	// Check that instances were generated
	instances, _ := repo.List(context.Background())
	if len(instances) == 0 {
		t.Error("expected instances to be generated")
	}

	// Verify recurrence was saved
	rec, _ := repo.GetRecurrence(context.Background(), task.ID)
	if rec == nil {
		t.Fatal("recurrence should be saved")
	}
	if rec.RecurrenceType != taskdomain.RecurrenceDaily {
		t.Error("recurrence type should be daily")
	}
	if rec.IntervalDays != 2 {
		t.Error("interval_days should be 2")
	}
}

func TestCreateMonthlyRecurrence(t *testing.T) {
	repo := newMockRepo()
	svc := &Service{repo: repo, now: fixedNow}

	startDate := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	task, err := svc.Create(context.Background(), CreateInput{
		Title:       "Monthly report",
		Description: "Generate monthly report",
		Status:      taskdomain.StatusNew,
		Recurrence: &RecurrenceInput{
			RecurrenceType: taskdomain.RecurrenceMonthly,
			MonthDays:      []int{1, 15},
			StartDate:      startDate,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !task.IsTemplate {
		t.Error("task should be a template")
	}

	rec, _ := repo.GetRecurrence(context.Background(), task.ID)
	if rec == nil {
		t.Fatal("recurrence should be saved")
	}
	if rec.RecurrenceType != taskdomain.RecurrenceMonthly {
		t.Error("recurrence type should be monthly")
	}
	if len(rec.MonthDays) != 2 || rec.MonthDays[0] != 1 || rec.MonthDays[1] != 15 {
		t.Error("month_days should be [1, 15]")
	}
}

func TestCreateOddEvenRecurrence(t *testing.T) {
	repo := newMockRepo()
	svc := &Service{repo: repo, now: fixedNow}

	startDate := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	task, err := svc.Create(context.Background(), CreateInput{
		Title:       "Even day task",
		Description: "Task on even days",
		Status:      taskdomain.StatusNew,
		Recurrence: &RecurrenceInput{
			RecurrenceType: taskdomain.RecurrenceOddEven,
			OddEvenType:    taskdomain.OddEvenEven,
			StartDate:      startDate,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rec, _ := repo.GetRecurrence(context.Background(), task.ID)
	if rec == nil {
		t.Fatal("recurrence should be saved")
	}
	if rec.OddEvenType != taskdomain.OddEvenEven {
		t.Error("odd_even_type should be even")
	}
}

func TestCreateSpecificDatesRecurrence(t *testing.T) {
	repo := newMockRepo()
	svc := &Service{repo: repo, now: fixedNow}

	d1 := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	task, err := svc.Create(context.Background(), CreateInput{
		Title:       "Specific dates task",
		Description: "Task on specific dates",
		Status:      taskdomain.StatusNew,
		Recurrence: &RecurrenceInput{
			RecurrenceType: taskdomain.RecurrenceSpecificDates,
			SpecificDates:  []time.Time{d1, d2, d3},
			StartDate:      d1,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rec, _ := repo.GetRecurrence(context.Background(), task.ID)
	if rec == nil {
		t.Fatal("recurrence should be saved")
	}
	if len(rec.SpecificDates) != 3 {
		t.Error("should have 3 specific dates")
	}
}

func TestCreateRecurrenceValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   RecurrenceInput
		errMsg  string
	}{
		{
			name: "daily without interval_days",
			input: RecurrenceInput{
				RecurrenceType: taskdomain.RecurrenceDaily,
				IntervalDays:   0,
				StartDate:      time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
			},
			errMsg: "interval_days",
		},
		{
			name: "monthly without month_days",
			input: RecurrenceInput{
				RecurrenceType: taskdomain.RecurrenceMonthly,
				StartDate:      time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
			},
			errMsg: "month_days",
		},
		{
			name: "specific_dates without dates",
			input: RecurrenceInput{
				RecurrenceType: taskdomain.RecurrenceSpecificDates,
				StartDate:      time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
			},
			errMsg: "specific_dates",
		},
		{
			name: "odd_even without type",
			input: RecurrenceInput{
				RecurrenceType: taskdomain.RecurrenceOddEven,
				StartDate:      time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
			},
			errMsg: "odd_even_type",
		},
		{
			name: "invalid recurrence type",
			input: RecurrenceInput{
				RecurrenceType: "weekly",
				StartDate:      time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
			},
			errMsg: "recurrence_type",
		},
		{
			name: "end_date before start_date",
			input: RecurrenceInput{
				RecurrenceType: taskdomain.RecurrenceDaily,
				IntervalDays:   1,
				StartDate:      time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
				EndDate:        ptrTime(time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)),
			},
			errMsg: "end_date",
		},
		{
			name: "month_days out of range",
			input: RecurrenceInput{
				RecurrenceType: taskdomain.RecurrenceMonthly,
				MonthDays:      []int{32},
				StartDate:      time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
			},
			errMsg: "month_days",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockRepo()
			svc := &Service{repo: repo, now: fixedNow}

			_, err := svc.Create(context.Background(), CreateInput{
				Title:      "Test",
				Status:     taskdomain.StatusNew,
				Recurrence: &tt.input,
			})
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestDeleteTemplateDeletesInstances(t *testing.T) {
	repo := newMockRepo()
	svc := &Service{repo: repo, now: fixedNow}

	startDate := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	template, _ := svc.Create(context.Background(), CreateInput{
		Title:       "Daily task",
		Status:      taskdomain.StatusNew,
		Recurrence: &RecurrenceInput{
			RecurrenceType: taskdomain.RecurrenceDaily,
			IntervalDays:   1,
			StartDate:      startDate,
		},
	})

	instances, _ := repo.List(context.Background())
	if len(instances) == 0 {
		t.Fatal("expected instances")
	}

	err := svc.Delete(context.Background(), template.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instances, _ = repo.List(context.Background())
	if len(instances) != 0 {
		t.Error("all instances should be deleted when template is deleted")
	}
}

func TestGenerateInstancesIdempotent(t *testing.T) {
	repo := newMockRepo()
	svc := &Service{repo: repo, now: fixedNow}

	startDate := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	_, err := svc.Create(context.Background(), CreateInput{
		Title:       "Daily task",
		Status:      taskdomain.StatusNew,
		Recurrence: &RecurrenceInput{
			RecurrenceType: taskdomain.RecurrenceDaily,
			IntervalDays:   1,
			StartDate:      startDate,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instances1, _ := repo.List(context.Background())

	// Run GenerateInstances again — should not create duplicates
	err = svc.GenerateInstances(context.Background(), 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instances2, _ := repo.List(context.Background())
	if len(instances2) != len(instances1) {
		t.Errorf("expected %d instances, got %d — GenerateInstances should be idempotent", len(instances1), len(instances2))
	}
}

func TestMatchesDateDaily(t *testing.T) {
	startDate := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	rec := &taskdomain.Recurrence{
		RecurrenceType: taskdomain.RecurrenceDaily,
		IntervalDays:   3,
		StartDate:      startDate,
	}

	tests := []struct {
		date time.Time
		want bool
	}{
		{time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC), true},  // day 0
		{time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC), false}, // day 1
		{time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC), false}, // day 2
		{time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC), true},  // day 3
		{time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), true},  // day 6
		{time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC), false}, // before start
	}

	for _, tt := range tests {
		got := rec.MatchesDate(tt.date)
		if got != tt.want {
			t.Errorf("MatchesDate(%v) = %v, want %v", tt.date, got, tt.want)
		}
	}
}

func TestMatchesDateMonthly(t *testing.T) {
	startDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	rec := &taskdomain.Recurrence{
		RecurrenceType: taskdomain.RecurrenceMonthly,
		MonthDays:      []int{1, 15},
		StartDate:      startDate,
	}

	tests := []struct {
		date time.Time
		want bool
	}{
		{time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), true},
		{time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC), true},
		{time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), false},
		{time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), true},
		{time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC), true},
	}

	for _, tt := range tests {
		got := rec.MatchesDate(tt.date)
		if got != tt.want {
			t.Errorf("MatchesDate(%v) = %v, want %v", tt.date, got, tt.want)
		}
	}
}

func TestMatchesDateOddEven(t *testing.T) {
	startDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	evenRec := &taskdomain.Recurrence{
		RecurrenceType: taskdomain.RecurrenceOddEven,
		OddEvenType:    taskdomain.OddEvenEven,
		StartDate:      startDate,
	}

	oddRec := &taskdomain.Recurrence{
		RecurrenceType: taskdomain.RecurrenceOddEven,
		OddEvenType:    taskdomain.OddEvenOdd,
		StartDate:      startDate,
	}

	if !evenRec.MatchesDate(time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)) {
		t.Error("2nd should match even")
	}
	if evenRec.MatchesDate(time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)) {
		t.Error("3rd should not match even")
	}
	if !oddRec.MatchesDate(time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)) {
		t.Error("3rd should match odd")
	}
	if oddRec.MatchesDate(time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)) {
		t.Error("2nd should not match odd")
	}
}

func TestMatchesDateWithEndDate(t *testing.T) {
	startDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	rec := &taskdomain.Recurrence{
		RecurrenceType: taskdomain.RecurrenceDaily,
		IntervalDays:   1,
		StartDate:      startDate,
		EndDate:        &endDate,
	}

	if !rec.MatchesDate(time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)) {
		t.Error("April 5 should match (within range)")
	}
	if rec.MatchesDate(time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)) {
		t.Error("April 11 should not match (after end_date)")
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
