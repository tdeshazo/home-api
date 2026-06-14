package db

import (
	"slices"
	"testing"

	"github.com/lib/pq"
)

func TestTaskDaysOfWeekScansFromPostgresArray(t *testing.T) {
	var task Task

	if err := pq.Array(&task.DaysOfWeek).Scan("{1,3,5}"); err != nil {
		t.Fatalf("scan days_of_week: %v", err)
	}

	if want := []int64{1, 3, 5}; !slices.Equal(task.DaysOfWeek, want) {
		t.Fatalf("days_of_week = %v, want %v", task.DaysOfWeek, want)
	}
}
