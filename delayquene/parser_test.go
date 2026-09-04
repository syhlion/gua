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
	base := time.Date(2026, 6, 24, 9, 15, 30, 0, time.UTC)
	cases := []struct {
		spec string
		from time.Time
		want time.Time
	}{
		// 6-field form: second minute hour dom month dow
		{"0 30 9 * * *", base, time.Date(2026, 6, 24, 9, 30, 0, 0, time.UTC)},
		{"0 0 * * * *", base, time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)},
		{"*/5 * * * * *", base, time.Date(2026, 6, 24, 9, 15, 35, 0, time.UTC)},
		{"0 0 0 1 * *", base, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
		{"0 0 9 * * mon", base, time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)},
		// 5-field form: classic crontab, minute hour dom month dow (seconds = 0)
		{"30 9 * * *", base, time.Date(2026, 6, 24, 9, 30, 0, 0, time.UTC)},
		{"*/5 * * * *", base, time.Date(2026, 6, 24, 9, 20, 0, 0, time.UTC)},
		{"0 0 1 * *", base, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
		{"0 9 * * 1", base, time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)},
		// descriptors
		{"@hourly", base, time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)},
		{"@daily", base, time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)},
		{"@weekly", base, time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)},
		{"@monthly", base, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
		{"@yearly", base, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		got := mustParse(t, c.spec).Next(c.from)
		if !got.Equal(c.want) {
			t.Errorf("Parse(%q).Next(%v) = %v, want %v", c.spec, c.from, got, c.want)
		}
	}
}

// The same wall-clock intent written in 5 and 6 fields must agree.
func TestParseFiveAndSixFieldsAgree(t *testing.T) {
	base := time.Date(2026, 6, 24, 9, 15, 30, 0, time.UTC)
	pairs := [][2]string{
		{"*/5 * * * *", "0 */5 * * * *"},
		{"30 9 * * *", "0 30 9 * * *"},
		{"0 0 * * 0", "0 0 0 * * 0"},
	}
	for _, p := range pairs {
		a := mustParse(t, p[0]).Next(base)
		b := mustParse(t, p[1]).Next(base)
		if !a.Equal(b) {
			t.Errorf("%q -> %v but %q -> %v", p[0], a, p[1], b)
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
	for _, spec := range []string{
		"", " ", "not-a-cron",
		"60 * * * * *",     // second out of range
		"* * * * * * *",    // 7 fields
		"* * * *",          // 4 fields
		"* * * * 13",       // month out of range
		"* * * * * 7",      // dow is 0..6
		"@nope",            // unknown descriptor
		"@every",           // missing duration
		"@every 5 minutes", // not a Go duration
	} {
		if _, err := Parse(spec); err == nil {
			t.Errorf("Parse(%q) expected error, got nil", spec)
		}
	}
}

func TestValidatePattern(t *testing.T) {
	for _, ok := range []string{"", "@once", "@every 1s", "*/5 * * * *", "0 0 9 * * *", "@daily"} {
		if err := ValidatePattern(ok); err != nil {
			t.Errorf("ValidatePattern(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"@twice", "* * *", "x"} {
		if err := ValidatePattern(bad); err == nil || !errorsIs(err, ErrInvalid) {
			t.Errorf("ValidatePattern(%q) = %v, want ErrInvalid", bad, err)
		}
	}
}

func TestSplitRequestURL(t *testing.T) {
	for _, c := range []struct{ in, kind, target string }{
		{"HTTP@http://x/y?z=1", "HTTP", "http://x/y?z=1"},
		{"GRPC@host:6666", "GRPC", "host:6666"},
	} {
		k, tg, err := SplitRequestURL(c.in)
		if err != nil || k != c.kind || tg != c.target {
			t.Errorf("SplitRequestURL(%q) = %q,%q,%v", c.in, k, tg, err)
		}
	}
	for _, bad := range []string{"", "HTTP@", "GRPC@", "HTTP@ ", "HTTP@a b", "http@x", "FTP@x", "x"} {
		if _, _, err := SplitRequestURL(bad); !errorsIs(err, ErrInvalid) {
			t.Errorf("SplitRequestURL(%q) = %v, want ErrInvalid", bad, err)
		}
	}
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"a", "A_1", "abcdefghijklmnopqrstuv"} {
		if err := ValidateName("x", ok); err != nil {
			t.Errorf("ValidateName(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "a-b", "a b", "中文", "abcdefghijklmnopqrstuvw"} {
		if err := ValidateName("x", bad); !errorsIs(err, ErrInvalid) {
			t.Errorf("ValidateName(%q) = %v, want ErrInvalid", bad, err)
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
