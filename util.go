package util

import (
	"time"
)

// PersonioDateMax is the maximum representable time.Time value for the Personio API
var PersonioDateMax, _ = time.Parse(time.RFC3339, "9999-12-31T23:59:59.999Z")

// GetTimeIntersection returns the intersection of two ranges or the distance between the ranges as a negative number
func GetTimeIntersection(start1 time.Time, end1 time.Time, start2 time.Time, end2 time.Time) time.Duration {

	endMin := end1
	if end2.Before(end1) {
		endMin = end2
	}

	startMax := start1
	if start2.After(start1) {
		startMax = start2
	}

	return endMin.Sub(startMax)
}
