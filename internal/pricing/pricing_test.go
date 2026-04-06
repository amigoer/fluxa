package pricing

import "testing"

func TestLookup_ExactMatch(t *testing.T) {
	p, ok := Lookup("gpt-4o")
	if !ok {
		t.Fatal("gpt-4o not found")
	}
	if p.InputPerMillion != 2.50 || p.OutputPerMillion != 10.00 {
		t.Errorf("unexpected price: %+v", p)
	}
}

func TestLookup_PrefixMatch(t *testing.T) {
	// Dated variant should fall back to the canonical entry.
	p, ok := Lookup("gpt-4o-2024-11-20")
	if !ok || p.InputPerMillion != 2.50 {
		t.Errorf("prefix match failed: %+v ok=%v", p, ok)
	}
}

func TestLookup_LongestPrefix(t *testing.T) {
	// "claude-3-5-sonnet-20241022" must resolve to claude-3-5-sonnet,
	// not the shorter "claude-3-sonnet".
	p, ok := Lookup("claude-3-5-sonnet-20241022")
	if !ok {
		t.Fatal("not found")
	}
	if p.InputPerMillion != 3.00 || p.OutputPerMillion != 15.00 {
		t.Errorf("wrong prefix resolution: %+v", p)
	}
}

func TestLookup_Unknown(t *testing.T) {
	if _, ok := Lookup("my-fancy-model"); ok {
		t.Error("should not have matched")
	}
}

func TestCost(t *testing.T) {
	// gpt-4o: 1000 prompt + 500 completion
	// = 1000 * 2.50/1e6 + 500 * 10/1e6 = 0.0025 + 0.005 = 0.0075
	got := Cost("gpt-4o", 1000, 500)
	if got < 0.00749 || got > 0.00751 {
		t.Errorf("cost = %f", got)
	}
}

func TestCost_UnknownModelReturnsZero(t *testing.T) {
	if got := Cost("unknown", 1000, 1000); got != 0 {
		t.Errorf("unknown should be 0, got %f", got)
	}
}
