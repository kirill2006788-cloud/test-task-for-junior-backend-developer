package task

import "time"

// RecurrenceType defines the type of recurrence for a task.
type RecurrenceType string

const (
	RecurrenceDaily         RecurrenceType = "daily"
	RecurrenceMonthly       RecurrenceType = "monthly"
	RecurrenceSpecificDates RecurrenceType = "specific_dates"
	RecurrenceOddEven       RecurrenceType = "odd_even"
)

// OddEvenType defines whether a task recurs on odd or even days of the month.
type OddEvenType string

const (
	OddEvenOdd  OddEvenType = "odd"
	OddEvenEven OddEvenType = "even"
)

// Recurrence holds the recurrence settings for a task.
type Recurrence struct {
	TaskID         int64          `json:"task_id"`
	RecurrenceType RecurrenceType `json:"recurrence_type"`
	IntervalDays   int            `json:"interval_days,omitempty"`   // daily: every N-th day
	MonthDays      []int          `json:"month_days,omitempty"`     // monthly: days of month [1..30]
	SpecificDates  []time.Time    `json:"specific_dates,omitempty"`  // specific_dates: exact dates
	OddEvenType    OddEvenType    `json:"odd_even_type,omitempty"`  // odd_even: "odd" or "even"
	StartDate      time.Time      `json:"start_date"`               // when recurrence starts
	EndDate        *time.Time     `json:"end_date,omitempty"`       // when recurrence ends (nil = infinite)
}

func (r RecurrenceType) Valid() bool {
	switch r {
	case RecurrenceDaily, RecurrenceMonthly, RecurrenceSpecificDates, RecurrenceOddEven:
		return true
	default:
		return false
	}
}

func (o OddEvenType) Valid() bool {
	switch o {
	case OddEvenOdd, OddEvenEven:
		return true
	default:
		return false
	}
}

// MatchesDate checks if the recurrence rule generates a task on the given date.
func (r *Recurrence) MatchesDate(d time.Time) bool {
	if d.Before(r.StartDate) {
		return false
	}
	if r.EndDate != nil && d.After(*r.EndDate) {
		return false
	}

	switch r.RecurrenceType {
	case RecurrenceDaily:
		if r.IntervalDays <= 0 {
			return false
		}
		diff := d.Sub(r.StartDate).Hours() / 24
		daysSinceStart := int(diff)
		return daysSinceStart >= 0 && daysSinceStart%r.IntervalDays == 0

	case RecurrenceMonthly:
		day := d.Day()
		for _, md := range r.MonthDays {
			if md == day {
				return true
			}
		}
		return false

	case RecurrenceSpecificDates:
		for _, sd := range r.SpecificDates {
			if sameDate(d, sd) {
				return true
			}
		}
		return false

	case RecurrenceOddEven:
		day := d.Day()
		if r.OddEvenType == OddEvenEven {
			return day%2 == 0
		}
		return day%2 == 1
	}

	return false
}

func sameDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}
