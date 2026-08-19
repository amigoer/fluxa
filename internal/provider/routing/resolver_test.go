package routing

import (
	"testing"

	"github.com/amigoer/fluxa/internal/provider/types"
)

// The bug this guards: in whole cents, tokens * cents_per_1M / 1_000_000
// truncated toward zero, and at ordinary chat sizes the whole result was
// what got truncated. Every one of these used to come back 0.
func TestEstimateCostChargesForSmallCalls(t *testing.T) {
	// ¥10 per 1M input, ¥20 per 1M output.
	model := types.Model{InputPriceCentsPer1M: 1000, OutputPriceCentsPer1M: 2000}

	for _, tc := range []struct {
		name                   string
		inputTokens, outputTok int
		wantMicroCents         int64
	}{
		// 800 * 1000/1M = 0.8 cents in, 400 * 2000/1M = 0.8 cents out.
		{"a short exchange", 800, 400, 16_000},
		// The smallest thing anyone sends: 1 token each way.
		{"one token each way", 1, 1, 30},
		{"a medium exchange", 5_000, 2_000, 90_000},
		{"a long document", 100_000, 20_000, 1_400_000},
	} {
		got := EstimateCostMicroCents(model, tc.inputTokens, tc.outputTok)
		if got != tc.wantMicroCents {
			t.Errorf("%s: cost = %d micro-cents, want %d", tc.name, got, tc.wantMicroCents)
		}
		if got == 0 {
			t.Errorf("%s: a call that cost real money was billed nothing", tc.name)
		}
	}
}

// A thousand small calls have to add up to what one call of the same
// total size costs, or spend drifts with traffic shape rather than
// volume. Per-call truncation is exactly what broke this.
func TestEstimateCostDoesNotDriftAcrossManySmallCalls(t *testing.T) {
	model := types.Model{InputPriceCentsPer1M: 1000, OutputPriceCentsPer1M: 2000}

	var accumulated int64
	for range 1_000 {
		accumulated += EstimateCostMicroCents(model, 800, 400)
	}
	oneBigCall := EstimateCostMicroCents(model, 800_000, 400_000)

	if accumulated != oneBigCall {
		t.Errorf("1000 small calls = %d micro-cents, one equivalent call = %d", accumulated, oneBigCall)
	}
}

func TestEstimateCostIsFreeWhenTheModelIsFree(t *testing.T) {
	if got := EstimateCostMicroCents(types.Model{}, 10_000, 5_000); got != 0 {
		t.Errorf("cost = %d, want 0 for a model with no price set", got)
	}
}

// Rounding is to nearest, not toward zero, so the residual error is
// centred rather than always shaving spend downward.
func TestEstimateCostRoundsToNearestMicroCent(t *testing.T) {
	// 1 token at 149 cents/1M -> 1.49 micro-cents -> 1
	if got := EstimateCostMicroCents(types.Model{InputPriceCentsPer1M: 149}, 1, 0); got != 1 {
		t.Errorf("cost = %d, want 1", got)
	}
	// 1 token at 151 cents/1M -> 1.51 micro-cents -> 2
	if got := EstimateCostMicroCents(types.Model{InputPriceCentsPer1M: 151}, 1, 0); got != 2 {
		t.Errorf("cost = %d, want 2", got)
	}
}

// A month of heavy traffic must stay far inside int64.
func TestEstimateCostHandlesLargeVolumeWithoutOverflow(t *testing.T) {
	// ¥1000 per 1M tokens, a 2M-token context.
	got := EstimateCostMicroCents(types.Model{InputPriceCentsPer1M: 100_000}, 2_000_000, 0)
	if got != 2_000_000_000 {
		t.Errorf("cost = %d micro-cents, want 2000000000 (¥2000)", got)
	}
}
