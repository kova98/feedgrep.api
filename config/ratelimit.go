package config

import (
	"fmt"
	"time"
)

const (
	RateIDSmartFilterGeneration = "smart_filter_generation"
)

type RateLimitPolicy struct {
	RateID    string
	Limit     int
	Window    string
	WindowKey func(time.Time) string
}

const (
	RateLimitWindowDaily  = "daily"
	RateLimitWindowHourly = "hourly"
	RateLimitWindowWeekly = "weekly"
)

var RateLimits = map[string]RateLimitPolicy{
	RateIDSmartFilterGeneration: {
		RateID:    RateIDSmartFilterGeneration,
		Limit:     3,
		Window:    RateLimitWindowDaily,
		WindowKey: DailyWindowKey,
	},
}

func DailyWindowKey(now time.Time) string {
	return now.UTC().Format("2006-01-02")
}

func HourlyWindowKey(now time.Time) string {
	return now.UTC().Format("2006-01-02T15")
}

func WeeklyWindowKey(now time.Time) string {
	year, week := now.UTC().ISOWeek()
	return fmt.Sprintf("%04d-W%02d", year, week)
}
