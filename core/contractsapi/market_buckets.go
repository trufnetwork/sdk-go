package contractsapi

import (
	"fmt"
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
		return value, nil
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
		return forecast.Ptr(lowerBound), forecast.Ptr(upperBound), nil

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
		return forecast.Ptr(target - tolerance), forecast.Ptr(target + tolerance), nil
	}

	return nil, nil, fmt.Errorf("cannot derive bucket bounds from a %q market", market.Type)
}
