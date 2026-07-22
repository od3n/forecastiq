package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/forecastiq/forecastiq/internal/catalog/domain"
)

func TestSchedule_Valid(t *testing.T) {
	assert.True(t, domain.DefaultSchedule().Valid())
	assert.False(t, domain.Schedule{Interval: "daily", MinuteOffset: 0}.Valid())
	assert.False(t, domain.Schedule{Interval: "hourly", MinuteOffset: 60}.Valid())
}

func TestSchedule_SlotTimes(t *testing.T) {
	from := time.Date(2026, 7, 22, 10, 30, 0, 0, time.UTC)
	to := time.Date(2026, 7, 22, 13, 0, 0, 0, time.UTC)

	got := domain.DefaultSchedule().SlotTimes(from, to)
	want := []time.Time{
		time.Date(2026, 7, 22, 11, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC),
	}
	assert.Equal(t, want, got)
}

func TestSchedule_SlotTimes_WithOffset(t *testing.T) {
	from := time.Date(2026, 7, 22, 10, 30, 0, 0, time.UTC)
	to := time.Date(2026, 7, 22, 13, 0, 0, 0, time.UTC)
	s := domain.Schedule{Interval: "hourly", MinuteOffset: 5}

	got := s.SlotTimes(from, to)
	want := []time.Time{
		time.Date(2026, 7, 22, 11, 5, 0, 0, time.UTC),
		time.Date(2026, 7, 22, 12, 5, 0, 0, time.UTC),
	}
	assert.Equal(t, want, got)
}

func TestSchedule_SlotTimes_EmptyWindow(t *testing.T) {
	from := time.Date(2026, 7, 22, 13, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 22, 10, 0, 0, 0, time.UTC) // to before from
	assert.Nil(t, domain.DefaultSchedule().SlotTimes(from, to))
}
