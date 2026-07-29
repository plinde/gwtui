package cache

import (
	"testing"
	"time"
)

type payload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func newOpts(t *testing.T, ttl time.Duration) Options {
	t.Helper()
	return Options{Dir: t.TempDir(), TTL: ttl}
}

func TestSetGetRoundTrip(t *testing.T) {
	o := newOpts(t, time.Minute)
	want := payload{Name: "gwtui", Count: 7}
	if err := Set(o, "repos-plinde", want); err != nil {
		t.Fatalf("Set: %v", err)
	}

	var got payload
	hit, age := Get(o, "repos-plinde", &got)
	if !hit {
		t.Fatal("expected fresh hit")
	}
	if age < 0 || age > time.Minute {
		t.Fatalf("unexpected age %v", age)
	}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestGetMissOnMissingKey(t *testing.T) {
	o := newOpts(t, time.Minute)
	var got payload
	if hit, _ := Get(o, "nope", &got); hit {
		t.Fatal("expected miss for absent key")
	}
}

func TestGetMissWhenExpired(t *testing.T) {
	o := newOpts(t, time.Minute)
	// Write with a WrittenAt well in the past.
	old := nowFunc
	nowFunc = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	if err := Set(o, "k", payload{Name: "x"}); err != nil {
		nowFunc = old
		t.Fatalf("Set: %v", err)
	}
	nowFunc = old

	var got payload
	hit, age := Get(o, "k", &got)
	if hit {
		t.Fatal("expected miss for expired entry")
	}
	if age < time.Hour {
		t.Fatalf("expected age > 1h, got %v", age)
	}
}

func TestGetStaleReturnsExpiredEntry(t *testing.T) {
	o := newOpts(t, time.Minute)
	old := nowFunc
	nowFunc = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	_ = Set(o, "k", payload{Name: "stale", Count: 3})
	nowFunc = old

	// Fresh Get misses...
	var fresh payload
	if hit, _ := Get(o, "k", &fresh); hit {
		t.Fatal("expected fresh Get to miss")
	}
	// ...but GetStale serves it (the 429 fallback path).
	var got payload
	hit, _ := GetStale(o, "k", &got)
	if !hit {
		t.Fatal("expected GetStale hit")
	}
	if got.Name != "stale" || got.Count != 3 {
		t.Fatalf("got %+v", got)
	}
}

func TestForceBypassesFreshRead(t *testing.T) {
	o := newOpts(t, time.Minute)
	_ = Set(o, "k", payload{Name: "cached"})

	o.Force = true
	var got payload
	if hit, _ := Get(o, "k", &got); hit {
		t.Fatal("Force should bypass fresh read")
	}
	// GetStale still works under Force (serve-on-error must remain available).
	if hit, _ := GetStale(o, "k", &got); !hit {
		t.Fatal("GetStale should still hit under Force")
	}
}

func TestDisabledNoReadNoWrite(t *testing.T) {
	o := newOpts(t, time.Minute)
	o.Disabled = true
	if err := Set(o, "k", payload{Name: "x"}); err != nil {
		t.Fatalf("Set under Disabled should no-op cleanly: %v", err)
	}
	var got payload
	if hit, _ := Get(o, "k", &got); hit {
		t.Fatal("Disabled Get must miss")
	}
	if hit, _ := GetStale(o, "k", &got); hit {
		t.Fatal("Disabled GetStale must miss")
	}
}

func TestZeroTTLNeverFresh(t *testing.T) {
	o := newOpts(t, 0)
	_ = Set(o, "k", payload{Name: "x"})
	var got payload
	if hit, _ := Get(o, "k", &got); hit {
		t.Fatal("TTL=0 should never report fresh")
	}
	// But the write happened and stale read works.
	if hit, _ := GetStale(o, "k", &got); !hit {
		t.Fatal("TTL=0 still write-through; GetStale should hit")
	}
}

func TestKeyWithSlashesSanitized(t *testing.T) {
	o := newOpts(t, time.Minute)
	if err := Set(o, "prs-elastic/cloud", payload{Name: "cloud"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	var got payload
	if hit, _ := Get(o, "prs-elastic/cloud", &got); !hit || got.Name != "cloud" {
		t.Fatalf("slash key round-trip failed: hit=%v got=%+v", hit, got)
	}
}
