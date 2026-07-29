package tui

import (
	"testing"
	"time"

	"github.com/plinde/gwtui/internal/cache"
	gh "github.com/plinde/gwtui/internal/github"
)

// This test exists because the bug it guards against was not a wrong value
// anywhere. Every function returned correct data; each unit passed its own
// tests. What went wrong was a *rate* — an emergent product of
//
//	repos × pages-per-repo × refreshes-per-hour
//
// which no single unit owns, and which nothing asserted. gwtui spent roughly
// 22,000 GraphQL requests/hour and exhausted the account's entire 5000/hour
// budget in about twenty minutes, repeatedly, while every test stayed green.
//
// So this asserts the product directly, over the constants that determine it.
// It fails the moment someone raises the fetch limit past a page, shortens the
// cache TTL, removes the cache, or unbounds the fan-out — the four ways this
// regresses.

// graphQLBudgetPerHour is GitHub's GraphQL allowance. It is **per account**,
// shared across every tool, machine and token — not per process. That sharing
// is what made this a whole-account outage rather than one slow TUI.
const graphQLBudgetPerHour = 5000

// toolBudgetShare is the most of that pool a single background TUI may plan to
// consume. Well under the limit on purpose: the budget is shared, and several
// of these tools run at once.
const toolBudgetShare = graphQLBudgetPerHour / 10 // 500/hr

// representativeRepoCount is a realistic large org root. The actual figure is a
// runtime property, so the guard is written against a plausible worst case.
const representativeRepoCount = 25

// graphQLPages is how many API requests one `gh pr list` costs. GitHub caps a
// connection page at 100 nodes, so the CLI paginates above that — the single
// most easily missed multiplier here, because it is invisible at the call site.
func graphQLPages(limit int) int {
	if limit <= 0 {
		return 1
	}
	return (limit + 99) / 100
}

// projectedRequestsPerHour is the steady-state cost of leaving the TUI open.
//
// The cache TTL, not the refresh interval, sets how often the API is actually
// touched — that substitution is the entire fix. With no cache the divisor
// would be the 15s interval instead, which is how the original 22k/hour arose.
func projectedRequestsPerHour(repos int, ttl time.Duration, limit int) int {
	if ttl <= 0 {
		ttl = autoRefreshInterval // no caching ⇒ every tick hits the API
	}
	liveFetchesPerHour := int(time.Hour / ttl)
	return repos * liveFetchesPerHour * graphQLPages(limit)
}

func TestSteadyStateStaysWithinRateBudget(t *testing.T) {
	got := projectedRequestsPerHour(representativeRepoCount, cache.DefaultTTL, gh.PRFetchLimit)
	if got > toolBudgetShare {
		t.Errorf(
			"projected %d GraphQL requests/hour for %d repos (TTL %s, limit %d, %d page(s)); budget is %d/hr.\n"+
				"The GraphQL pool is %d/hr shared across the whole account — this tool must not plan to use more than a tenth of it.",
			got, representativeRepoCount, cache.DefaultTTL, gh.PRFetchLimit,
			graphQLPages(gh.PRFetchLimit), toolBudgetShare, graphQLBudgetPerHour,
		)
	}
	t.Logf("steady state: %d requests/hour for %d repos (%.1f%% of the account pool)",
		got, representativeRepoCount, 100*float64(got)/graphQLBudgetPerHour)
}

// The regression this is really guarding: removing the cache. Reproduce the old
// configuration and confirm the guard would have rejected it.
func TestUncachedConfigurationWouldBlowTheBudget(t *testing.T) {
	old := projectedRequestsPerHour(17, 0 /* no cache */, 200)
	if old <= toolBudgetShare {
		t.Fatalf("the pre-fix configuration projects %d/hr, which the guard would have allowed — the guard is too loose", old)
	}
	if old <= graphQLBudgetPerHour {
		t.Errorf("the pre-fix configuration projects %d/hr; it exhausted the whole %d/hr pool in ~20 minutes, so the model is wrong",
			old, graphQLBudgetPerHour)
	}
	t.Logf("pre-fix configuration projected %d requests/hour — %.1fx the account pool, and %.0fx this tool's share", old, float64(old)/graphQLBudgetPerHour, float64(old)/toolBudgetShare)
}

// Each individual knob must stay in its safe range, so a regression names the
// specific cause rather than only the aggregate.
func TestRateKnobsIndividually(t *testing.T) {
	if p := graphQLPages(gh.PRFetchLimit); p != 1 {
		t.Errorf("PRFetchLimit %d costs %d GraphQL pages; keep it within one page", gh.PRFetchLimit, p)
	}
	if cache.DefaultTTL < 5*time.Minute {
		t.Errorf("cache TTL %s is short enough to approach uncached spend", cache.DefaultTTL)
	}
	if repoFetchConcurrency > 10 {
		t.Errorf("repoFetchConcurrency %d risks overshooting the cap with in-flight requests", repoFetchConcurrency)
	}
	if autoRefreshInterval < 5*time.Second {
		t.Errorf("autoRefreshInterval %s is aggressive even for local-only work", autoRefreshInterval)
	}
}
