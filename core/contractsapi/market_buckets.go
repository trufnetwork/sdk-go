package contractsapi

import (
	"fmt"
	"math"
	"strconv"

	"github.com/trufnetwork/sdk-go/core/forecast"
)

// BucketBoundsFromMarketData turns one bucket market's decoded data into its
// half-open [lower, upper) bounds.
//
// A nil bound means open-ended, which is how the outer two buckets of a market
// are always struck. Bounds are half-open upstream, so a value landing exactly
// on a boundary resolves the upper bucket only.
//
// "above", "below" and "between" markets are struck in the stream's own units;
// "change_between" markets are struck in percent, against the stream's value one
// time_interval earlier. This function does not distinguish them -- a caller
// comparing bounds across markets has to know it is comparing like with like.
//
// This is SDK-side glue, deliberately kept OUT of the forecast package: that
// package is a translation of the upstream algorithm in
// truflation/prediction-bots and must stay comparable against it (and against
// the sdk-py mirror). Nothing here is part of the forecast maths.
//
// Returns an error if the market type cannot describe a bucket, or the
// thresholds needed for that type are missing.
func BucketBoundsFromMarketData(market *MarketData) (lower, upper *float64, err error) {
	if market == nil {
		return nil, nil, fmt.Errorf("market data is required to derive bucket bounds")
	}

	threshold := func(index int) (float64, error) {
		if len(market.Thresholds) <= index {
			return 0, fmt.Errorf(
				"a %q market needs at least %d threshold(s), got %d",
				market.Type, index+1, len(market.Thresholds))
		}
		value, parseErr := strconv.ParseFloat(market.Thresholds[index], 64)
		if parseErr != nil {
			return 0, fmt.Errorf(
				"threshold %d of a %q market is not a number: %q",
				index, market.Type, market.Thresholds[index])
		}
		// ParseFloat accepts "NaN", "Inf" and "+Inf" without complaint. Those
		// would flow all the way into the forecast and surface as a NaN value
		// rather than as this market being unreadable.
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, fmt.Errorf(
				"threshold %d of a %q market is not finite: %q",
				index, market.Type, market.Thresholds[index])
		}
		return value, nil
	}

	// optionalThreshold reads a slot that is allowed to be struck open. An open
	// tail decodes to an empty string in place rather than a missing slot, so a
	// short slice is still a malformed market.
	optionalThreshold := func(index int) (*float64, error) {
		if len(market.Thresholds) <= index {
			return nil, fmt.Errorf(
				"a %q market needs %d threshold slot(s), got %d",
				market.Type, index+1, len(market.Thresholds))
		}
		if market.Thresholds[index] == "" {
			return nil, nil
		}
		value, tErr := threshold(index)
		if tErr != nil {
			return nil, tErr
		}
		return forecast.Ptr(value), nil
	}

	switch market.Type {
	case "below":
		upperBound, tErr := threshold(0)
		if tErr != nil {
			return nil, nil, tErr
		}
		return nil, forecast.Ptr(upperBound), nil

	case "above":
		lowerBound, tErr := threshold(0)
		if tErr != nil {
			return nil, nil, tErr
		}
		return forecast.Ptr(lowerBound), nil, nil

	case "between":
		lowerBound, tErr := threshold(0)
		if tErr != nil {
			return nil, nil, tErr
		}
		upperBound, tErr := threshold(1)
		if tErr != nil {
			return nil, nil, tErr
		}
		// Bounds are half-open [lower, upper), so lower == upper is an empty
		// bucket and lower > upper is an inverted one. Neither can hold an
		// outcome, and both would quietly distort the tiling.
		if lowerBound >= upperBound {
			return nil, nil, fmt.Errorf(
				"a %q market needs lower < upper, got [%v, %v)",
				market.Type, lowerBound, upperBound)
		}
		return forecast.Ptr(lowerBound), forecast.Ptr(upperBound), nil

	case "change_between":
		// Percentage-change buckets, already half-open upstream and already in
		// the open-ended shape this function returns, so the bounds pass
		// through rather than being derived.
		lowerBound, lErr := optionalThreshold(0)
		if lErr != nil {
			return nil, nil, lErr
		}
		upperBound, uErr := optionalThreshold(1)
		if uErr != nil {
			return nil, nil, uErr
		}
		// Both tails open would describe the whole number line. The node action
		// refuses to be created that way, so a market reaching here like that is
		// malformed rather than unbounded.
		if lowerBound == nil && upperBound == nil {
			return nil, nil, fmt.Errorf(
				"a %q market needs at least one bound, got neither", market.Type)
		}
		if lowerBound != nil && upperBound != nil && *lowerBound >= *upperBound {
			return nil, nil, fmt.Errorf(
				"a %q market needs lower < upper, got [%v, %v)",
				market.Type, *lowerBound, *upperBound)
		}
		return lowerBound, upperBound, nil

	case "equals":
		// Thresholds are (target, tolerance), NOT (lower, upper). Reading them
		// positionally the way "between" is read would give an inverted bucket
		// and be silently wrong rather than loud.
		target, tErr := threshold(0)
		if tErr != nil {
			return nil, nil, tErr
		}
		tolerance, tErr := threshold(1)
		if tErr != nil {
			return nil, nil, tErr
		}
		if tolerance <= 0 {
			return nil, nil, fmt.Errorf(
				"an %q market needs a positive tolerance, got %v",
				market.Type, tolerance)
		}
		lowerBound, upperBound := target-tolerance, target+tolerance
		// A positive tolerance does not guarantee a non-empty bucket. Near the
		// float limits the sum can overflow to Inf, and a tolerance small enough
		// relative to the target is absorbed entirely, collapsing both edges
		// onto the same value: 1e300 +/- 1e-300 is 1e300 twice.
		if math.IsInf(lowerBound, 0) || math.IsInf(upperBound, 0) ||
			lowerBound >= upperBound {
			return nil, nil, fmt.Errorf(
				"an %q market with target %v and tolerance %v does not describe "+
					"a usable bucket: [%v, %v)",
				market.Type, target, tolerance, lowerBound, upperBound)
		}
		return forecast.Ptr(lowerBound), forecast.Ptr(upperBound), nil
	}

	return nil, nil, fmt.Errorf("cannot derive bucket bounds from a %q market", market.Type)
}
