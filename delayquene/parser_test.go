package delayquene

import (
	"testing"
	"time"
)

func mustParse(t *testing.T, spec string) Schedule {
	t.Helper()
	s, err := Parse(spec)
	if err != nil {
		t.Fatalf("Parse(%q) unexpected error: %v", spec, err)
	}
	return s
}

func TestParseNextCron(t *testing.T) {
	// 6-field form: second minute hour dom month dow
	base := time.Date(2026, 6, 24, 9, 15, 30, 0, time.UTC)
	cases := []struct {
		spec string
		from time.Time
		want time.Time
	}{
		{"0 30 9 * * *", base, time.Date(2026, 6, 24, 9, 30, 0, 0, time.UTC)},
		{"0 0 * * * *", base, time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)},
		{"@hourly", base, time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)},
		{"@daily", base, time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)},
		{"0 0 0 1 * *", base, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		got := mustParse(t, c.spec).Next(c.from)
		if !got.Equal(c.want) {
			t.Errorf("Parse(%q).Next(%v) = %v, want %v", c.spec, c.from, got, c.want)
		}
	}
}

func TestParseEvery(t *testing.T) {
	base := time.Date(2026, 6, 24, 9, 15, 30, 0, time.UTC)
	got := mustParse(t, "@every 1h30m").Next(base)
	want := base.Add(90 * time.Minute)
	if !got.Equal(want) {
		t.Errorf("@every 1h30m Next = %v, want %v", got, want)
	}
}

func TestEveryRoundsToSecond(t *testing.T) {
	// sub-second delays round up to 1s; sub-second fields are truncated.
	if d := Every(500 * time.Millisecond).Delay; d != time.Second {
		t.Errorf("Every(500ms).Delay = %v, want 1s", d)
	}
	if d := Every(90500 * time.Millisecond).Delay; d != 90*time.Second {
		t.Errorf("Every(90.5s).Delay = %v, want 90s", d)
	}
}

func TestParseErrors(t *testing.T) {
	for _, spec := range []string{"", "not-a-cron", "60 * * * * *", "* * * * * * *", "@nope"} {
		if _, err := Parse(spec); err == nil {
			t.Errorf("Parse(%q) expected error, got nil", spec)
		}
	}
}

func TestNextRecurringStability(t *testing.T) {
	// Calling Next repeatedly from each result should yield a fixed cadence.
	s := mustParse(t, "0 0 * * * *") // top of every hour
	cur := time.Date(2026, 6, 24, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		nxt := s.Next(cur)
		if want := cur.Add(time.Hour); !nxt.Equal(want) {
			t.Fatalf("iteration %d: Next(%v) = %v, want %v", i, cur, nxt, want)
		}
		cur = nxt
	}
}
