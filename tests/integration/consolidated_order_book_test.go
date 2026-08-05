// Live-network check that the SDK folds a market's two outcome books together
// the right way round.
//
// The unit tests under core/forecast prove the mapping on hand-written numbers.
// What they cannot prove is that the SDK reads the CHAIN correctly: if the node
// flipped the sign convention on bids, or get_market_depth swapped its two
// volume columns, every one of them would still pass while a NO ask surfaced on
// the wrong side of the YES ladder. That is the gap this file covers.
//
// Gated on an env var rather than the kwiltest build tag, because it wants a
// network carrying live markets with real orders rather than a fresh local node:
//
//	TN_LIVE_ENDPOINT=https://gateway.mainnet.truf.network \
//	    go test ./tests/integration/ -run TestConsolidatedOrderBookLive -v
//
// All calls are view actions, so the signing key needs no funds and controls
// nothing.
//
// These assert INVARIANTS against the raw depth reads, not numbers. Live books
// move minute to minute, so pinning expected volumes would fail by tomorrow.

package integration

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kwilcrypto "github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	"github.com/trufnetwork/sdk-go/core/forecast"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
)

const (
	// maxMarketsProbed bounds discovery, which costs two calls per active
	// market. Mainnet carried ~130 active markets when this was written.
	maxMarketsProbed = 200

	// stableReadAttempts is how many times to re-read a market that moved
	// underneath the comparison before giving up and skipping.
	stableReadAttempts = 3
)

func TestConsolidatedOrderBookLive(t *testing.T) {
	endpoint := os.Getenv("TN_LIVE_ENDPOINT")
	if endpoint == "" {
		t.Skip("live network not configured; set TN_LIVE_ENDPOINT to run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), liveTestTimeout)
	defer cancel()

	key, err := kwilcrypto.Secp256k1PrivateKeyFromHex(liveReadOnlyPK)
	require.NoError(t, err, "failed to parse the read-only key")

	client, err := tnclient.NewClient(ctx, endpoint, tnclient.WithSigner(auth.GetUserSigner(key)))
	require.NoError(t, err, "failed to create client")

	ob, err := client.LoadOrderBook()
	require.NoError(t, err, "failed to load order book")

	queryID, found := findQuotedMarket(ctx, t, ob)
	if !found {
		t.Skip("no active market on this network has any resting order")
	}
	t.Logf("reading market %d", queryID)

	yesDepth, noDepth, book, noBook, ok := readStableBooks(ctx, t, ob, queryID)
	if !ok {
		t.Skipf("market %d kept moving across %d attempts; nothing to compare against",
			queryID, stableReadAttempts)
	}

	require.NotNil(t, book)
	assert.Equal(t, queryID, book.QueryID)
	assert.True(t, book.Outcome)

	// Bids are this outcome's buys plus the OTHER outcome's sells inverted; asks
	// are this outcome's sells plus the other outcome's buys. Getting either
	// pairing backwards is the failure this whole file exists to catch.
	checkSide(t, "bids", book.Bids, buyVolumes(yesDepth), sellVolumes(noDepth), true)
	checkSide(t, "asks", book.Asks, sellVolumes(yesDepth), buyVolumes(noDepth), false)

	if book.IsCrossed {
		require.NotEmpty(t, book.Bids)
		require.NotEmpty(t, book.Asks)
		assert.GreaterOrEqual(t, book.Bids[0].Price, book.Asks[0].Price)
	} else if len(book.Bids) > 0 && len(book.Asks) > 0 {
		assert.Less(t, book.Bids[0].Price, book.Asks[0].Price)
	}

	// The NO-framed book is the YES-framed book reflected: price q becomes
	// 100-q, bids become asks, and native volume becomes inverse volume.
	require.NotNil(t, noBook)
	assert.False(t, noBook.Outcome)
	assertReflects(t, book.Bids, noBook.Asks)
	assertReflects(t, book.Asks, noBook.Bids)
}

// readStableBooks reads the raw depth and both framed books, and reports whether
// the market held still while it did so.
//
// Every read here is a separate view call, and get_market_depth takes no block
// height, so there is no snapshot to pin them to. On a live market an order can
// land between two of them and the comparison would fail on drift rather than on
// a real defect. Re-reading the raw depth afterwards and requiring it to match
// closes that: if the book is unchanged either side of the sequence, the reads in
// between saw the same state.
func readStableBooks(ctx context.Context, t *testing.T, ob types.IOrderBook, queryID int) (
	yesDepth, noDepth []types.DepthLevel,
	book, noBook *forecast.ConsolidatedOrderBook,
	ok bool,
) {
	t.Helper()

	for attempt := 1; attempt <= stableReadAttempts; attempt++ {
		var err error
		yesDepth, err = ob.GetMarketDepth(ctx, types.GetMarketDepthInput{QueryID: queryID, Outcome: true})
		require.NoError(t, err)
		noDepth, err = ob.GetMarketDepth(ctx, types.GetMarketDepthInput{QueryID: queryID, Outcome: false})
		require.NoError(t, err)

		book, err = ob.GetConsolidatedOrderBook(ctx, types.GetConsolidatedOrderBookInput{
			QueryID: queryID, Outcome: true,
		})
		require.NoError(t, err)
		noBook, err = ob.GetConsolidatedOrderBook(ctx, types.GetConsolidatedOrderBookInput{
			QueryID: queryID, Outcome: false,
		})
		require.NoError(t, err)

		yesAfter, err := ob.GetMarketDepth(ctx, types.GetMarketDepthInput{QueryID: queryID, Outcome: true})
		require.NoError(t, err)
		noAfter, err := ob.GetMarketDepth(ctx, types.GetMarketDepthInput{QueryID: queryID, Outcome: false})
		require.NoError(t, err)

		if reflect.DeepEqual(yesDepth, yesAfter) && reflect.DeepEqual(noDepth, noAfter) {
			return yesDepth, noDepth, book, noBook, true
		}
		t.Logf("market %d moved during attempt %d; re-reading", queryID, attempt)
	}

	return nil, nil, nil, nil, false
}

// checkSide verifies one consolidated side against the raw depth it came from.
//
// native maps price to the volume resting in this outcome's own book; opposite
// maps price to the volume resting in the other outcome's book, which belongs at
// 100 minus that price.
func checkSide(
	t *testing.T,
	name string,
	levels []forecast.ConsolidatedLevel,
	native, opposite map[float64]float64,
	descending bool,
) {
	t.Helper()

	seenNative := 0
	seenOpposite := 0
	for i, level := range levels {
		assert.Greater(t, level.Price, 0.0, "%s[%d] price", name, i)
		assert.Less(t, level.Price, 100.0, "%s[%d] price", name, i)
		assert.Greater(t, level.Total, 0.0, "%s[%d] is an empty level", name, i)
		assert.Equal(t, level.Native+level.Inverse, level.Total, "%s[%d] total", name, i)

		assert.Equal(t, native[level.Price], level.Native, "%s at %v native", name, level.Price)
		assert.Equal(t, opposite[100-level.Price], level.Inverse, "%s at %v inverse", name, level.Price)
		if level.Native > 0 {
			seenNative++
		}
		if level.Inverse > 0 {
			seenOpposite++
		}

		if i > 0 && descending {
			assert.Greater(t, levels[i-1].Price, level.Price, "%s must be best-highest first", name)
		} else if i > 0 {
			assert.Less(t, levels[i-1].Price, level.Price, "%s must be best-lowest first", name)
		}
	}

	// Nothing on the chain may go missing on the way through.
	assert.Equal(t, len(native), seenNative, "%s dropped a native level", name)
	assert.Equal(t, len(opposite), seenOpposite, "%s dropped an inverse level", name)
}

// assertReflects checks that one frame's side is the other frame's opposite side
// mirrored across 100.
func assertReflects(t *testing.T, from, to []forecast.ConsolidatedLevel) {
	t.Helper()

	require.Len(t, to, len(from))
	for i, level := range from {
		assert.Equal(t, forecast.ConsolidatedLevel{
			Price:   100 - level.Price,
			Total:   level.Total,
			Native:  level.Inverse,
			Inverse: level.Native,
		}, to[i])
	}
}

// findQuotedMarket returns an active market carrying resting orders, preferring
// one quoted on BOTH outcomes since that is what actually exercises the fold.
func findQuotedMarket(ctx context.Context, t *testing.T, ob types.IOrderBook) (int, bool) {
	t.Helper()

	// FALSE means active only, per 032-order-book-actions.sql:358. Settled
	// markets cannot move and would only waste probes.
	active := false
	offset, scanned := 0, 0
	fallback, haveFallback := 0, false

	for scanned < maxMarketsProbed {
		limit := 100
		page, err := ob.ListMarkets(ctx, types.ListMarketsInput{
			SettledFilter: &active, Limit: &limit, Offset: &offset,
		})
		require.NoError(t, err)
		if len(page) == 0 {
			break
		}

		for _, summary := range page {
			scanned++
			yesQuoted := hasResting(ctx, ob, summary.ID, true)
			noQuoted := hasResting(ctx, ob, summary.ID, false)
			if yesQuoted && noQuoted {
				return summary.ID, true
			}
			if (yesQuoted || noQuoted) && !haveFallback {
				fallback, haveFallback = summary.ID, true
			}
		}

		if len(page) < limit {
			break
		}
		offset += limit
	}

	return fallback, haveFallback
}

// hasResting reports whether an outcome has any order resting on it.
func hasResting(ctx context.Context, ob types.IOrderBook, queryID int, outcome bool) bool {
	best, err := ob.GetBestPrices(ctx, types.GetBestPricesInput{QueryID: queryID, Outcome: outcome})
	return err == nil && best != nil && (best.BestBid != nil || best.BestAsk != nil)
}

func buyVolumes(depth []types.DepthLevel) map[float64]float64 {
	out := map[float64]float64{}
	for _, level := range depth {
		if level.BuyVolume > 0 {
			out[float64(level.Price)] = float64(level.BuyVolume)
		}
	}
	return out
}

func sellVolumes(depth []types.DepthLevel) map[float64]float64 {
	out := map[float64]float64{}
	for _, level := range depth {
		if level.SellVolume > 0 {
			out[float64(level.Price)] = float64(level.SellVolume)
		}
	}
	return out
}
