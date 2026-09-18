package bango

import (
	"testing"
	"time"
)

func TestElapsedReadsAsOneUnit(t *testing.T) {
	for _, one := range []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{45 * time.Second, "45s"},
		{90 * time.Second, "1m"},
		{3 * time.Hour, "3h"},
		{50 * time.Hour, "2d"},
		{400 * 24 * time.Hour, "1y"},
	} {
		if got := Elapsed(one.d); got != one.want {
			t.Errorf("Elapsed(%v) = %q, want %q", one.d, got, one.want)
		}
	}
}

func TestElapsedNeverRunsBackwards(t *testing.T) {
	if got := Elapsed(-5 * time.Second); got != "0s" {
		t.Errorf("Elapsed(-5s) = %q — a clock a little ahead of ours is not five seconds of future", got)
	}
	if got := Since(time.Now().Add(time.Hour)); got != "0s" {
		t.Errorf("Since(an hour from now) = %q", got)
	}
}
