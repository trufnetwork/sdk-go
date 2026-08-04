// Package forecast turns prediction-market order book state into a single
// implied value.
//
// Prediction markets price RANGES. A five-bucket EPS market says "38% chance EPS
// lands between $4.33 and $4.62"; it never says "EPS will be $4.47". This
// package inverts that. Given the order books across every bucket of one market,
// it produces the single number the book is collectively implying, together with
// how tightly the book pins that number down.
//
//	market says            ->   this package says
//	"34% between 2.40-2.63"     "2.43, margin of error 0.02"
//
// # How it works
//
// A market is N buckets, each its own query_id and its own binary book, and the
// outer two are open-ended:
//
//	bucket 1  (-inf, B1)      bucket 2  [B1, B2)   ...   bucket N  [Bn-1, +inf)
//
// Bounds are HALF-OPEN, matching how the markets are actually created
// (value < upper; value >= lower AND value < upper; value >= lower). So the
// buckets are mutually exclusive and exhaustive, and a value landing exactly on
// a boundary resolves the UPPER bucket only.
//
// Each bucket's book implies a probability. Two-sided is the mid, with
// confidence from how tight the spread is. One-sided takes the quoted side and
// steps across by the market's own median half-spread, at reduced confidence: on
// real books the open tail buckets are often bid-only at a penny, and treating
// that as the midpoint of [0.01, 1.0] calls a dead tail a coin flip. That single
// mistake handed MSFT's two tails 27% each and buried the real distribution, so
// the rule matters more than it looks.
//
// Bucket probabilities are normalised to sum to 1, since independent books never
// sum to exactly 1 on their own because of spreads and staleness. A bucket with
// no orders at all takes the residual the quoted buckets leave behind rather
// than an invented value. Those give the implied CDF at each interior boundary.
//
// The point estimate is the MEDIAN read directly off that CDF (rank method):
// walk up the cumulative probabilities until they cross 50% and interpolate
// within the bucket where the crossing happens. No distribution shape is assumed
// for anything interior. The published band is P10..P90 read the same way. When
// a requested percentile falls inside an OPEN-ENDED outer bucket the books
// contain no information about how far out it lies, so an exponential tail
// matched to the adjacent bucket's density supplies a number, and the result is
// flagged (ValueBasis / P10Basis / P90Basis = "tail") so consumers can decide
// whether to show it. Markets struck so that the mass stays between the outer
// boundaries never need the tail model at all.
//
// Confidence per bucket is (1 - spread) * min(1, n / median(n)) with n the
// thinner side's committed dollars, standardised against the market's own depth
// so no external constant is needed. It does not move the median; it is
// published as book quality and drives warnings.
package forecast

// Translated from truflation/prediction-bots @ main 21fe4f3 (PR #37), upstream
// path market-maker-bot/src/market_maker_bot/forecast.py, by way of the
// byte-identical mirror in trufnetwork/sdk-py (src/trufnetwork_sdk_py/forecast.py).
//
// sdk-py can keep a verbatim copy and re-sync with a diff. Go cannot, so this is
// a hand translation and drift has to be caught by BEHAVIOUR instead:
// forecast_test.go and forecast_depth_test.go are ports of the upstream suites,
// including the frozen live NVDA book, and are the contract that keeps the SDKs
// agreeing. Change the algorithm upstream first, then port it here and re-run
// both suites.
//
// Verified against sdk-py over 39 shared cases (best-quote and depth paths,
// tails, the discrete fallback, dutch books, dust griefing, the frozen NVDA
// book) and against a live mainnet market.
//
// The residual is ~1e-16, worst case 6.5 ULP, from two causes, neither of them
// fixable and neither of them a translation error:
//
//  1. CPython 3.12 gave sum() compensated (Neumaier) summation for floats, so
//     the Python sums are slightly MORE accurate than the plain left-to-right
//     ones here. Reproducing that would mean a compensated adder at every sum
//     site, coupling this file to a CPython implementation detail that
//     upstream's source does not state -- it just says sum(...) -- and making
//     the translation harder to read against the original, which is the whole
//     maintenance strategy. sdk-js made the same call.
//
//  2. math.Log is not bit-identical across runtimes. This shows up ONLY in
//     percentiles whose basis is BasisTail, the exponential tail model being
//     the sole caller of a transcendental. On one case Go's p90 lands an ULP
//     off CPython's and JavaScript's, which agree with each other. Nothing can
//     be done about this from inside the algorithm.
//
// Both are fourteen orders of magnitude below the published margin of error.
// Neither is purely invisible: a sum landing within 1e-16 of a rounding
// boundary can move the last digit of a warning STRING. The forecast numbers
// are unaffected.
//
// Go needs no tie-breaking helper of its own: fmt rounds half to even, which is
// what Python's format spec does. (sdk-js needs one, because toFixed rounds ties
// away from zero.)
//
// One deliberate omission: upstream still carries _fit_normal, the
// weighted-least-squares normal fit on the probit scale that the rank method
// replaced. It is unreachable there -- nothing calls it -- so it is not
// translated, which also spares this package an inverse-normal CDF. If it is
// ever wired back up upstream, it has to be written here from scratch, not
// ported.

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	// minConfidence is the point below which a bracket contributes essentially
	// nothing.
	minConfidence = 1e-3

	// DefaultHalfSpread completes a one-sided quote when the market gives us
	// nothing better. The bounds keep a pathological book from imputing a silly
	// step.
	DefaultHalfSpread = 0.06
	minHalfSpread     = 0.01
	maxHalfSpread     = 0.20

	// MinQuoteNotionalCentShares is the floor below which a top-of-book quote is
	// treated as absent. Size does not enter the forecast itself, so without a
	// floor a single tiny order at a silly price moves the published number as
	// much as a real one: one 90c bid on a dead tail shifted a bimodal market by
	// 8 units, for 90c of risk. Only applied when sizes are supplied; callers
	// that pass none keep the unfiltered behaviour.
	MinQuoteNotionalCentShares = 100

	// dutchBookTolerance bounds how far the book may stray before we say so. The
	// buckets are mutually exclusive and exhaustive, so the asks must cost at
	// least 1 and the bids must not pay more than 1. Outside these, someone is
	// giving money away. Warned on rather than normalised silently.
	dutchBookTolerance = 0.02

	// DepthMinSideNotionalUSD applies to the depth path only. A side whose ENTIRE
	// ladder holds less than this many dollars is treated as absent: with the
	// full book summed, dust cannot hide in front of a real order, so a small
	// floor is enough to make sub-dollar griefing quotes weightless. Tunable.
	DepthMinSideNotionalUSD = 5.0

	// LowTotalNotionalWarnUSD is a disclosure threshold only, never part of the
	// math: a market whose books hold less than this in total gets a warning
	// attached, because relative standardisation cannot see absolute poverty.
	LowTotalNotionalWarnUSD = 50.0

	// PeakProminence is the fraction of the tallest peak a second density peak
	// must reach to count as real multimodality; below it is quote noise.
	PeakProminence = 0.20
)

// Basis values, recording where a percentile came from and therefore how far to
// trust it.
const (
	// BasisInterior means the value was read between real strikes with no shape
	// assumption.
	BasisInterior = "interior"
	// BasisTail means the value fell inside an open-ended outer bucket and rests
	// on the exponential tail model.
	BasisTail = "tail"
	// BasisUnresolved means the market has too few strikes to place the value.
	BasisUnresolved = "unresolved"
)

// Method values, recording how the point estimate was produced.
const (
	// MethodRank is the normal path: percentiles read off the implied CDF.
	MethodRank = "rank"
	// MethodDiscrete is the fallback for a book the rank read cannot handle.
	MethodDiscrete = "discrete"
)

// Ptr returns a pointer to v. Bucket bounds and quotes are optional, so they are
// pointers; this saves callers a temporary on every literal.
//
//	forecast.BucketBook{Lower: nil, Upper: forecast.Ptr(4.04)}
func Ptr[T any](v T) *T {
	return &v
}

// BookLevel is one resting level: price in cents (positive, 1-99), size in
// shares.
type BookLevel struct {
	Price float64
	Size  float64
}

// BucketBook is one bucket's bounds and its best two-sided quote, in cents.
//
// Lower/Upper are nil for the open-ended outer buckets. Prices are the positive
// 1-99 cent convention for the bucket's YES side; nil means no resting order on
// that side.
type BucketBook struct {
	Lower   *float64
	Upper   *float64
	BestBid *float64
	BestAsk *float64
	QueryID *int
	// BidSize and AskSize are the shares at the best quote. Supply them to
	// enable the dust floor; leave them nil to skip it.
	BidSize *float64
	AskSize *float64
}

// usableQuote drops a top-of-book quote too small to mean anything.
//
// LIMITATION: this only sees the best level. If a dust order is sitting in front
// of a real one, dropping it here marks the whole bucket unpriced rather than
// falling back to the real quote behind it, which can be worse than not
// filtering at all. Callers that hold the full book should instead pass the best
// level whose size clears the floor, and leave the size fields nil. This is a
// backstop, not the fix.
func usableQuote(price, size *float64) *float64 {
	if price == nil || size == nil {
		return price
	}
	if *price*(*size) >= MinQuoteNotionalCentShares {
		return price
	}
	return nil
}

// UsableBid returns the best bid, or nil when it is absent or below the dust
// floor.
func (b BucketBook) UsableBid() *float64 { return usableQuote(b.BestBid, b.BidSize) }

// UsableAsk returns the best ask, or nil when it is absent or below the dust
// floor.
func (b BucketBook) UsableAsk() *float64 { return usableQuote(b.BestAsk, b.AskSize) }

// BucketDepth is one bucket's FULL ladders on all four sides.
//
// The YES and NO books are consolidated inside this package because the
// inversion is executable, not merely informative. Verified on testnet
// (2026-08-03): the protocol's fill hierarchy matches EXACT inverses. A resting
// BUY NO at p mint-matches an incoming BUY YES at 100-p (one pair minted for
// $1), and a resting SELL NO at p burn-matches an incoming SELL YES at 100-p
// (one pair merged for $1). So:
//
//	consolidated bids = YesBids + (100 - p for every NO ask)
//	consolidated asks = YesAsks + (100 - p for every NO bid)
//
// both sides hittable at exactly the inverted price. There is no
// price-improvement crossing: near-inverses (sums other than 100) rest.
type BucketDepth struct {
	Lower   *float64
	Upper   *float64
	YesBids []BookLevel
	YesAsks []BookLevel
	NoBids  []BookLevel
	NoAsks  []BookLevel
	QueryID *int
}

// round4 keeps the inverted cent prices off float dust.
func round4(v float64) float64 { return math.Round(v*1e4) / 1e4 }

// ConsolidatedBids returns the bucket's executable bid ladder: its YES bids plus
// every NO ask inverted to the YES price it is hittable at, best (highest)
// first.
func (d BucketDepth) ConsolidatedBids() []BookLevel {
	levels := make([]BookLevel, 0, len(d.YesBids)+len(d.NoAsks))
	for _, l := range d.YesBids {
		if l.Size > 0 {
			levels = append(levels, l)
		}
	}
	for _, l := range d.NoAsks {
		if l.Size > 0 {
			levels = append(levels, BookLevel{Price: round4(100 - l.Price), Size: l.Size})
		}
	}
	sort.SliceStable(levels, func(i, j int) bool { return levels[i].Price > levels[j].Price })
	return levels
}

// ConsolidatedAsks returns the bucket's executable ask ladder: its YES asks plus
// every NO bid inverted to the YES price it is hittable at, best (lowest) first.
func (d BucketDepth) ConsolidatedAsks() []BookLevel {
	levels := make([]BookLevel, 0, len(d.YesAsks)+len(d.NoBids))
	for _, l := range d.YesAsks {
		if l.Size > 0 {
			levels = append(levels, l)
		}
	}
	for _, l := range d.NoBids {
		if l.Size > 0 {
			levels = append(levels, BookLevel{Price: round4(100 - l.Price), Size: l.Size})
		}
	}
	sort.SliceStable(levels, func(i, j int) bool { return levels[i].Price < levels[j].Price })
	return levels
}

// BestView returns the consolidated best quotes, for the spread and dutch-book
// helpers.
func (d BucketDepth) BestView() BucketBook {
	bids, asks := d.ConsolidatedBids(), d.ConsolidatedAsks()
	view := BucketBook{Lower: d.Lower, Upper: d.Upper, QueryID: d.QueryID}
	if len(bids) > 0 {
		view.BestBid = Ptr(bids[0].Price)
	}
	if len(asks) > 0 {
		view.BestAsk = Ptr(asks[0].Price)
	}
	return view
}

// BucketEstimate is what one bucket's book implies about its probability.
type BucketEstimate struct {
	QueryID *int
	Lower   *float64
	Upper   *float64
	// Probability is normalised; it sums to 1 across the market.
	Probability float64
	// RawProbability is the value before normalisation.
	RawProbability float64
	// Confidence runs 0 (no usable quote) to 1 (tight two-sided).
	Confidence float64
	OneSided   bool
	Quoted     bool
}

// MarketForecast is the single value a market's order books imply.
//
// Value is the MEDIAN of the implied distribution. P10/P90 are the published
// band. Each carries a basis flag: BasisInterior means it was read between real
// strikes with no shape assumption; BasisTail means it fell inside an
// open-ended outer bucket and rests on the exponential tail model;
// BasisUnresolved means the market has too few strikes to place it at all.
//
// MarginOfError is kept for downstream compatibility as half the P10..P90 band;
// Sigma as the band scaled to a normal-equivalent standard deviation
// (band / 2.5631). Prefer the quantiles directly.
type MarketForecast struct {
	Value float64
	// MarginOfError is half the P10..P90 band. NOT a standard error.
	MarginOfError float64
	Sigma         float64
	Method        string
	Buckets       []BucketEstimate
	Warnings      []string
	P10           *float64
	P90           *float64
	ValueBasis    string
	P10Basis      string
	P90Basis      string
	Multimodal    bool
	NPeaks        int
}

// Low returns Value - MarginOfError.
func (f MarketForecast) Low() float64 { return f.Value - f.MarginOfError }

// High returns Value + MarginOfError.
func (f MarketForecast) High() float64 { return f.Value + f.MarginOfError }

// AsMap returns the flat form, for SDKs and the indexer.
func (f MarketForecast) AsMap() map[string]any {
	probabilities := make([]float64, len(f.Buckets))
	for i, b := range f.Buckets {
		probabilities[i] = b.Probability
	}
	warnings := make([]string, len(f.Warnings))
	copy(warnings, f.Warnings)
	return map[string]any{
		"value":           f.Value,
		"value_basis":     f.ValueBasis,
		"p10":             f.P10,
		"p90":             f.P90,
		"p10_basis":       f.P10Basis,
		"p90_basis":       f.P90Basis,
		"margin_of_error": f.MarginOfError,
		"low":             f.Low(),
		"high":            f.High(),
		"sigma":           f.Sigma,
		"method":          f.Method,
		"multimodal":      f.Multimodal,
		"probabilities":   probabilities,
		"warnings":        warnings,
	}
}

// clamp01 pins a probability into [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// grouped0 renders a dollar amount with thousands separators, matching Python's
// "{:,.0f}". Go's fmt has no grouping verb.
func grouped0(v float64) string {
	s := fmt.Sprintf("%.0f", v)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// TypicalHalfSpread returns half the market's median two-sided spread, as a
// probability.
//
// Used to complete one-sided quotes. A missing side is usually an artifact of
// the market maker's own quoting (no inventory to sell, or the self-cross guard
// suppressing a leg) rather than a statement about probability, so the other
// side is still informative once shifted by the spread this market actually
// trades.
func TypicalHalfSpread(buckets []BucketBook) float64 {
	var spreads []float64
	for _, b := range buckets {
		if b.BestBid == nil || b.BestAsk == nil || *b.BestAsk < *b.BestBid {
			continue
		}
		spreads = append(spreads, (*b.BestAsk-*b.BestBid)/100)
	}
	if len(spreads) == 0 {
		return DefaultHalfSpread
	}
	sort.Float64s(spreads)
	mid := len(spreads) / 2
	var median float64
	if len(spreads)%2 == 1 {
		median = spreads[mid]
	} else {
		median = (spreads[mid-1] + spreads[mid]) / 2
	}
	return math.Max(minHalfSpread, math.Min(median/2, maxHalfSpread))
}

// BucketProbability turns one bucket's best quote into a probability,
// confidence and flags.
//
// Two-sided is the easy case: the mid, with confidence from how tight the
// spread is.
//
// One-sided needs care. Treating a lone 1c bid as the midpoint of [0.01, 1.0]
// would call it a coin flip, which is badly wrong: on real mainnet books the
// open tail buckets are frequently bid-only at a penny, and the midpoint rule
// handed them 27% each and buried the actual distribution. The quoted side is
// the only real information, so we take it and step across by the market's own
// half-spread, then mark the confidence down.
//
// An unquoted bucket returns a nil probability. It is NOT 0.5: the caller
// assigns it the residual probability the quoted buckets leave behind, which is
// what the market actually implies about it.
func BucketProbability(bestBid, bestAsk *float64, halfSpread float64) (
	probability *float64, confidence float64, oneSided, quoted bool,
) {
	quoted = bestBid != nil || bestAsk != nil
	if !quoted {
		return nil, 0, false, false
	}

	oneSided = bestBid == nil || bestAsk == nil

	if !oneSided {
		lo := clamp01(*bestBid / 100)
		hi := clamp01(*bestAsk / 100)
		if hi < lo {
			// Crossed book: bid above ask. Either stale, or free money. Keeping
			// the midpoint at low confidence would still let the bogus
			// probability enter the normalisation and shift every CDF point,
			// because confidence only affects weighting. Treat it as unquoted so
			// the caller can assign it the residual instead.
			return nil, 0, oneSided, quoted
		}
		return Ptr((lo + hi) / 2), 1 - (hi - lo), oneSided, quoted
	}

	var p float64
	if bestBid != nil {
		p = *bestBid/100 + halfSpread
	} else {
		p = *bestAsk/100 - halfSpread
	}
	// Confidence of a two-sided book at this spread, halved: we inferred one
	// side rather than observing it.
	return Ptr(clamp01(p)), math.Max(minConfidence, (1-2*halfSpread)/2), oneSided, quoted
}

// bucketMeta is one bucket's identity, as assemble needs it.
type bucketMeta struct {
	lower   *float64
	upper   *float64
	queryID *int
}

// discreteFallback returns probability-weighted bucket midpoints, for when the
// rank read cannot run.
//
// The open tails have no midpoint, so they are represented by their finite edge
// pushed out by half the average interior width. Cruder than a rank read and
// biased toward the centre, which is why it is only a fallback. Returns NaN when
// the buckets cannot support an estimate.
func discreteFallback(estimates []BucketEstimate, boundaries []float64) (value, sigma float64) {
	var widths []float64
	for i := 0; i+1 < len(boundaries); i++ {
		widths = append(widths, boundaries[i+1]-boundaries[i])
	}
	pad := 0.0
	if len(widths) > 0 {
		var total float64
		for _, w := range widths {
			total += w
		}
		pad = total / float64(len(widths)) / 2
	}

	var points, probs []float64
	for _, est := range estimates {
		if est.Lower == nil && est.Upper == nil {
			continue
		}
		var point float64
		switch {
		case est.Lower == nil:
			point = *est.Upper - pad
		case est.Upper == nil:
			point = *est.Lower + pad
		default:
			point = (*est.Lower + *est.Upper) / 2
		}
		points = append(points, point)
		probs = append(probs, est.Probability)
	}

	var total float64
	for _, p := range probs {
		total += p
	}
	if total <= 0 || len(points) == 0 {
		return math.NaN(), math.NaN()
	}

	var weighted float64
	for i, x := range points {
		weighted += probs[i] * x
	}
	mean := weighted / total

	var variance float64
	for i, x := range points {
		variance += probs[i] * (x - mean) * (x - mean)
	}
	variance /= total
	return mean, math.Sqrt(math.Max(variance, 0))
}

// discreteMargin returns a margin of error for the discrete fallback.
//
// Publishing sigma here would be wrong: sigma is how uncertain the OUTCOME is,
// not how well the book pins our estimate. The fallback also represents each
// open tail by an invented point, so its centre is genuinely less trustworthy
// than a rank read. Scale sigma down by the number of buckets we could actually
// read, and floor it so a fallback can never claim to be tighter than a real
// read.
func discreteMargin(sigma float64, estimates []BucketEstimate) float64 {
	quoted := 0
	var confidence float64
	for _, e := range estimates {
		if e.Quoted {
			quoted++
		}
		confidence += e.Confidence
	}
	if quoted == 0 {
		quoted = 1
	}
	conf := confidence / math.Max(float64(len(estimates)), 1)
	return sigma * (1 - 0.5*conf) / math.Sqrt(float64(quoted))
}

// dutchBookWarning reports a book that gives money away.
//
// Buying YES in every bucket guarantees exactly 1, so the asks must sum to at
// least 1. Minting a pair in every bucket and selling all the YES also settles
// at 1, so the bids must not sum to more than 1. A violation is risk-free money
// against us.
func dutchBookWarning(buckets []BucketBook) string {
	asksComplete, bidsComplete := true, true
	var askTotal, bidTotal float64
	for _, b := range buckets {
		ask, bid := b.UsableAsk(), b.UsableBid()
		if ask == nil {
			asksComplete = false
		} else {
			askTotal += *ask
		}
		if bid == nil {
			bidsComplete = false
		} else {
			bidTotal += *bid
		}
	}
	if asksComplete {
		total := askTotal / 100
		if total < 1-dutchBookTolerance {
			return fmt.Sprintf(
				"DUTCH BOOK: asks sum to %.3f; buying every bucket costs "+
					"%.3f and settles at 1.000, a risk-free "+
					"%.1fc per dollar against us", total, total, (1-total)*100)
		}
	}
	if bidsComplete {
		total := bidTotal / 100
		if total > 1+dutchBookTolerance {
			return fmt.Sprintf(
				"DUTCH BOOK: bids sum to %.3f; minting and selling every "+
					"bucket pays %.3f against a cost of 1.000, a risk-free "+
					"%.1fc per dollar against us", total, total, (total-1)*100)
		}
	}
	return ""
}

// rankQuantile reads percentile q directly off the piecewise-linear implied CDF.
//
// Interior crossings interpolate between real strikes and assume nothing beyond
// uniform density within the bucket. A crossing inside an open-ended outer
// bucket needs a shape for the tail; we use an exponential decay matched to the
// adjacent interior bucket's density, and flag the result BasisTail so consumers
// know it rests on that assumption. With fewer than two boundaries there is no
// interior at all and no density to anchor a tail, so the answer is unresolved.
func rankQuantile(bounds, probs []float64, q float64) (*float64, string) {
	k := len(probs)
	cum := make([]float64, 0, k-1)
	run := 0.0
	for i := 0; i < k-1; i++ {
		run += probs[i]
		cum = append(cum, run)
	}
	if len(bounds) < 2 {
		return nil, BasisUnresolved
	}

	for i := 1; i < k-1; i++ { // interior buckets
		loC, hiC := cum[i-1], cum[i]
		if loC <= q && q <= hiC {
			loB, hiB := bounds[i-1], bounds[i]
			if hiC <= loC {
				return Ptr((loB + hiB) / 2), BasisInterior
			}
			frac := (q - loC) / (hiC - loC)
			return Ptr(loB + frac*(hiB-loB)), BasisInterior
		}
	}

	widths := make([]float64, 0, len(bounds)-1)
	for i := 0; i+1 < len(bounds); i++ {
		widths = append(widths, bounds[i+1]-bounds[i])
	}

	if q < cum[0] { // lower open tail
		dens := 0.0
		if widths[0] > 0 {
			dens = probs[1] / widths[0]
		}
		if dens <= 0 || cum[0] <= 0 {
			return nil, BasisUnresolved
		}
		lam := dens / cum[0]
		return Ptr(bounds[0] + math.Log(q/cum[0])/lam), BasisTail
	}

	// upper open tail
	upperMass := probs[k-1]
	dens := 0.0
	if widths[len(widths)-1] > 0 {
		dens = probs[k-2] / widths[len(widths)-1]
	}
	if dens <= 0 || upperMass <= 0 {
		return nil, BasisUnresolved
	}
	lam := dens / upperMass
	return Ptr(bounds[len(bounds)-1] - math.Log(math.Max(1-q, 1e-12)/upperMass)/lam), BasisTail
}

// countPeaks returns the prominent local maxima of the interior density profile.
//
// A peak only counts if it reaches PeakProminence of the tallest one, so a
// penny-noise bump next to a 96% bucket does not read as bimodality. Open tails
// carry no density profile, so tail-heavy bimodality is seen through the
// interior buckets adjacent to the tails.
func countPeaks(bounds, probs []float64) int {
	dens := make([]float64, 0, len(bounds)-1)
	for i := 0; i+1 < len(bounds); i++ {
		w := bounds[i+1] - bounds[i]
		if w > 0 {
			dens = append(dens, probs[i+1]/w)
		} else {
			dens = append(dens, 0)
		}
	}
	if len(dens) == 0 {
		return 1
	}
	top := dens[0]
	for _, d := range dens {
		if d > top {
			top = d
		}
	}
	if top <= 0 {
		return 1
	}
	peaks := 0
	for i, d := range dens {
		isMax := (i == 0 || d > dens[i-1]) && (i == len(dens)-1 || d >= dens[i+1])
		if isMax && d >= PeakProminence*top {
			peaks++
		}
	}
	if peaks < 1 {
		return 1
	}
	return peaks
}

// assemble is the shared back half of the pipeline: normalise the per-bucket
// estimates, build the implied CDF, read the percentiles off it, or fall back.
//
// bidsCents is each bucket's best bid, used only to bound what an unpriced
// bucket could claim.
func assemble(
	meta []bucketMeta,
	bidsCents []*float64,
	raw []*float64,
	confs []float64,
	oneSidedFlags []bool,
	quotedFlags []bool,
	warnings []string,
) *MarketForecast {
	anyQuoted := false
	nUnquoted := 0
	for _, q := range quotedFlags {
		if q {
			anyQuoted = true
		} else {
			nUnquoted++
		}
	}
	if !anyQuoted {
		return nil
	}

	nMissing := 0
	for _, p := range raw {
		if p == nil {
			nMissing++
		}
	}
	nCrossed := nMissing - nUnquoted
	nOneSided := 0
	for _, o := range oneSidedFlags {
		if o {
			nOneSided++
		}
	}

	if nUnquoted > 0 {
		warnings = append(warnings, fmt.Sprintf("%d of %d buckets unquoted", nUnquoted, len(meta)))
	}
	if nCrossed > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%d bucket(s) crossed (bid above ask); treated as unpriced", nCrossed))
	}
	if nOneSided > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%d bucket(s) one-sided; margin of error widened", nOneSided))
	}

	var quotedTotal float64
	for _, p := range raw {
		if p != nil {
			quotedTotal += *p
		}
	}
	if quotedTotal <= 0 {
		return nil
	}
	if math.Abs(quotedTotal-1) > dutchBookTolerance {
		// Independent books never sum to exactly 1. Report the SIGNED deviation
		// rather than a wide dead band: the direction says whether the book is
		// over- or under-priced, and a wide window hides real boxes.
		warnings = append(warnings, fmt.Sprintf(
			"bucket mids sum to %.3f (%+.1fc per dollar) before normalising",
			quotedTotal, (quotedTotal-1)*100))
	}

	filled := make([]float64, len(raw))
	if nMissing > 0 {
		// An unpriced bucket is UNKNOWN, not impossible. Its probability is
		// bounded above by whatever the other buckets' BIDS do not already
		// claim, since bids are a lower bound on their true probabilities. Take
		// the midpoint of that interval. Asserting zero is a strong claim the
		// book never made.
		var bidClaim float64
		for i, p := range raw {
			if p != nil && bidsCents[i] != nil {
				bidClaim += *bidsCents[i] / 100
			}
		}
		headroom := math.Max(0, 1-bidClaim)
		est := headroom / (2 * float64(nMissing))
		for i, p := range raw {
			if p == nil {
				filled[i] = est
			} else {
				filled[i] = *p
			}
		}
	} else {
		for i, p := range raw {
			filled[i] = *p
		}
	}

	var total float64
	for _, p := range filled {
		total += p
	}
	if total <= 0 {
		return nil
	}
	probs := make([]float64, len(filled))
	for i, p := range filled {
		probs[i] = p / total
	}

	estimates := make([]BucketEstimate, len(meta))
	for i, m := range meta {
		rawProbability := 0.0
		if raw[i] != nil {
			rawProbability = *raw[i]
		}
		estimates[i] = BucketEstimate{
			QueryID:        m.queryID,
			Lower:          m.lower,
			Upper:          m.upper,
			Probability:    probs[i],
			RawProbability: rawProbability,
			Confidence:     confs[i],
			OneSided:       oneSidedFlags[i],
			Quoted:         quotedFlags[i],
		}
	}

	var bounds []float64
	for _, m := range meta[:len(meta)-1] {
		if m.upper != nil {
			bounds = append(bounds, *m.upper)
		}
	}

	fallback := func() *MarketForecast {
		value, sigma := discreteFallback(estimates, bounds)
		if math.IsNaN(value) {
			return nil
		}
		return &MarketForecast{
			Value:         value,
			MarginOfError: discreteMargin(sigma, estimates),
			Sigma:         sigma,
			Method:        MethodDiscrete,
			Buckets:       estimates,
			Warnings:      warnings,
			ValueBasis:    BasisInterior,
			P10Basis:      BasisInterior,
			P90Basis:      BasisInterior,
			NPeaks:        1,
		}
	}

	if len(bounds) != len(meta)-1 {
		warnings = append(warnings, "bucket bounds incomplete; falling back to discrete estimate")
		return fallback()
	}

	// Rank method: read the percentiles straight off the implied CDF.
	med, medBasis := rankQuantile(bounds, probs, 0.50)
	p10, p10Basis := rankQuantile(bounds, probs, 0.10)
	p90, p90Basis := rankQuantile(bounds, probs, 0.90)

	if med == nil {
		warnings = append(warnings,
			"median unresolvable from these strikes; falling back to discrete estimate")
		return fallback()
	}

	if medBasis == BasisTail {
		outside := probs[len(probs)-1]
		if probs[0] >= 0.5 {
			outside = probs[0]
		}
		warnings = append(warnings, fmt.Sprintf(
			"median lies beyond the outer strikes (%.0f%% of mass in an "+
				"open-ended bucket); value depends on the assumed tail shape",
			outside*100))
	}
	for _, pair := range []struct {
		name  string
		basis string
	}{{"p10", p10Basis}, {"p90", p90Basis}} {
		switch pair.basis {
		case BasisTail:
			warnings = append(warnings, pair.name+" falls in an open tail; tail-model dependent")
		case BasisUnresolved:
			warnings = append(warnings, pair.name+" unresolvable from these strikes")
		}
	}

	nPeaks := countPeaks(bounds, probs)
	multimodal := nPeaks >= 2
	if multimodal {
		warnings = append(warnings, fmt.Sprintf(
			"market shows %d probability clusters; a single point forecast "+
				"misrepresents it, publish the clusters instead", nPeaks))
	}

	var margin, sigma float64
	if p10 != nil && p90 != nil {
		band := math.Max(*p90-*p10, 0)
		margin = band / 2
		sigma = band / 2.5631 // P10..P90 of a normal spans 2.5631 sigma
	} else {
		warnings = append(warnings, "band unresolvable; margin and sigma reported as 0")
	}

	return &MarketForecast{
		Value:         *med,
		MarginOfError: margin,
		Sigma:         sigma,
		Method:        MethodRank,
		Buckets:       estimates,
		Warnings:      warnings,
		P10:           p10,
		P90:           p90,
		ValueBasis:    medBasis,
		P10Basis:      p10Basis,
		P90Basis:      p90Basis,
		Multimodal:    multimodal,
		NPeaks:        nPeaks,
	}
}

// FromBuckets collapses one market's bucket books into a single implied value,
// from best quotes only.
//
// Buckets must be supplied in ascending order and span the whole line, with the
// first open below and the last open above. Returns nil when the book carries no
// usable information at all. When full ladders are available, prefer FromDepth,
// which weights by committed notional.
func FromBuckets(buckets []BucketBook) *MarketForecast {
	var warnings []string

	if len(buckets) < 2 {
		return nil
	}

	half := TypicalHalfSpread(buckets)

	if dutch := dutchBookWarning(buckets); dutch != "" {
		warnings = append(warnings, dutch)
	}

	raw := make([]*float64, len(buckets))
	confs := make([]float64, len(buckets))
	oneSidedFlags := make([]bool, len(buckets))
	quotedFlags := make([]bool, len(buckets))
	meta := make([]bucketMeta, len(buckets))
	bidsCents := make([]*float64, len(buckets))
	for i, b := range buckets {
		p, conf, oneSided, quoted := BucketProbability(b.UsableBid(), b.UsableAsk(), half)
		raw[i] = p
		confs[i] = conf
		oneSidedFlags[i] = oneSided
		quotedFlags[i] = quoted
		meta[i] = bucketMeta{lower: b.Lower, upper: b.Upper, queryID: b.QueryID}
		bidsCents[i] = b.UsableBid()
	}

	return assemble(meta, bidsCents, raw, confs, oneSidedFlags, quotedFlags, warnings)
}

// sideStats returns the VWAP in cents, the notional in dollars, and the shares
// over a full ladder.
//
// The full-ladder VWAP no longer prices anything (see walkPrice); dollars are
// the currency for confidence and the dust floors, since posting many shares of
// a cheap side is not the same commitment as posting the same dollars.
func sideStats(levels []BookLevel) (vwapCents, notionalUSD, shares float64) {
	var centShares float64
	for _, l := range levels {
		centShares += l.Price * l.Size
		shares += l.Size
	}
	return centShares / shares, centShares / 100, shares
}

// walkPrice returns the dollar-weighted price of executing targetUSD from the
// touch.
//
// Walk the ladder best-first, consuming each level's own notional until the
// target is reached; the last level fills partially. Each committed dollar
// carries the same weight regardless of the price it rests at, which is the
// whole point: per-dollar influence must not depend on denomination, or penny
// ladders (25 shares per dollar at 4c) out-vote real money.
//
// Callers guarantee the ladder holds at least targetUSD in total (the side
// floor); if it somehow does not, the walk prices what is there.
func walkPrice(levels []BookLevel, targetUSD float64) float64 {
	remaining := targetUSD * 100 // cent-shares
	var weighted, taken float64
	for _, l := range levels {
		if remaining <= 0 {
			break
		}
		take := math.Min(l.Price*l.Size, remaining)
		weighted += take * l.Price
		taken += take
		remaining -= take
	}
	return weighted / taken
}

// EstimateFromDepth returns one bucket's implied probability from its full
// consolidated ladders.
//
// The probability is the midpoint of the two executable walk prices: the
// dollar-weighted price of selling DepthMinSideNotionalUSD into the consolidated
// bids and of buying the same from the consolidated asks, clamped into
// [best bid, best ask]. A book only pins the truth to that interval, so the
// estimate must be a selection from it; depth beyond the first walk dollars is
// inventory, not opinion, and moves confidence only.
//
// This replaced a full-book share-weighted microprice, which let cheap ladders
// out-vote real money (a dollar at 4c is 25 shares, at 30c it is 3) and on live
// books pushed every sub-50c bucket above its own best ask (prediction-bots
// issue #36). Ladder imbalance moves the walk mid not at all: on this venue all
// resting flow is the market maker's own, so imbalance carries no information
// and would only be a free manipulation lever.
//
// quality is the spread component of confidence: (1 - spread) two-sided, halved
// for a one-sided book whose missing side was inferred. notionalUSD is the
// committed dollars backing the estimate: the THINNER side when both exist, the
// quoted side otherwise. The caller standardises notional against the market's
// own median depth to finish the confidence; no external constant is involved.
//
// Sides whose whole ladder holds under DepthMinSideNotionalUSD are treated as
// absent: with the full book summed, dust cannot hide in front of a real order.
// Trades are deliberately NOT used: resting depth is live, at-risk capital, and
// the venue's only taker is a designed noise trader.
func EstimateFromDepth(depth BucketDepth, halfSpread float64) (
	probability *float64, quality, notionalUSD float64, oneSided, quoted bool,
) {
	bids := depth.ConsolidatedBids()
	asks := depth.ConsolidatedAsks()
	var bidNotional, askNotional float64

	if len(bids) > 0 {
		_, bidNotional, _ = sideStats(bids)
		if bidNotional < DepthMinSideNotionalUSD {
			bids, bidNotional = nil, 0
		}
	}
	if len(asks) > 0 {
		_, askNotional, _ = sideStats(asks)
		if askNotional < DepthMinSideNotionalUSD {
			asks, askNotional = nil, 0
		}
	}

	quoted = len(bids) > 0 || len(asks) > 0
	if !quoted {
		return nil, 0, 0, false, false
	}
	oneSided = !(len(bids) > 0 && len(asks) > 0)

	if !oneSided {
		bestBid, bestAsk := bids[0].Price, asks[0].Price
		if bestBid >= bestAsk {
			// Crossed at the consolidated touch: stale or free money.
			return nil, 0, 0, oneSided, quoted
		}
		walkBid := walkPrice(bids, DepthMinSideNotionalUSD)
		walkAsk := walkPrice(asks, DepthMinSideNotionalUSD)
		p := (walkBid + walkAsk) / 2
		p = math.Max(bestBid, math.Min(bestAsk, p)) / 100
		spread := (bestAsk - bestBid) / 100
		return Ptr(clamp01(p)),
			math.Max(1-spread, minConfidence),
			math.Min(bidNotional, askNotional),
			oneSided, quoted
	}

	var p, notional float64
	if len(bids) > 0 {
		p = walkPrice(bids, DepthMinSideNotionalUSD)/100 + halfSpread
		notional = bidNotional
	} else {
		p = walkPrice(asks, DepthMinSideNotionalUSD)/100 - halfSpread
		notional = askNotional
	}
	q := (1 - 2*halfSpread) / 2
	return Ptr(clamp01(p)), math.Max(q, minConfidence), notional, oneSided, quoted
}

// FromDepth collapses one market's bucket books into a single implied value,
// using FULL YES and NO ladders weighted by committed notional.
//
// This is the preferred entry point. The YES/NO consolidation happens here, per
// bucket, because the inversion is executable on this venue (exact inverse
// mint/burn matching, verified on testnet 2026-08-03), so callers should not
// have to know the rule.
func FromDepth(buckets []BucketDepth) *MarketForecast {
	var warnings []string

	if len(buckets) < 2 {
		return nil
	}

	views := make([]BucketBook, len(buckets))
	for i, b := range buckets {
		views[i] = b.BestView()
	}
	half := TypicalHalfSpread(views)

	if dutch := dutchBookWarning(views); dutch != "" {
		warnings = append(warnings, dutch)
	}

	raw := make([]*float64, len(buckets))
	qualities := make([]float64, len(buckets))
	notionals := make([]float64, len(buckets))
	oneSidedFlags := make([]bool, len(buckets))
	quotedFlags := make([]bool, len(buckets))
	for i, b := range buckets {
		p, quality, nUSD, oneSided, quoted := EstimateFromDepth(b, half)
		raw[i] = p
		qualities[i] = quality
		notionals[i] = nUSD
		oneSidedFlags[i] = oneSided
		quotedFlags[i] = quoted
	}

	// Confidence = quality * min(1, n / median(n)), the market standardised
	// against its own depth. Median rather than max or sum so a single whale
	// bucket cannot crush its siblings' credibility.
	var pricedN []float64
	for i, n := range notionals {
		if raw[i] != nil && n > 0 {
			pricedN = append(pricedN, n)
		}
	}
	sort.Float64s(pricedN)
	medN := 0.0
	if len(pricedN) > 0 {
		mid := len(pricedN) / 2
		if len(pricedN)%2 == 1 {
			medN = pricedN[mid]
		} else {
			medN = (pricedN[mid-1] + pricedN[mid]) / 2
		}
	}
	confs := make([]float64, len(buckets))
	for i, p := range raw {
		switch {
		case p == nil:
			confs[i] = 0
		case medN > 0:
			confs[i] = math.Max(qualities[i]*math.Min(1, notionals[i]/medN), minConfidence)
		default:
			confs[i] = qualities[i]
		}
	}

	// Disclosure, never math: relative standardisation cannot see absolute
	// poverty, so a market that is thin EVERYWHERE gets a visible caveat.
	var totalUSD float64
	for _, b := range buckets {
		for _, side := range [][]BookLevel{b.ConsolidatedBids(), b.ConsolidatedAsks()} {
			if len(side) > 0 {
				_, notional, _ := sideStats(side)
				totalUSD += notional
			}
		}
	}
	if totalUSD < LowTotalNotionalWarnUSD {
		warnings = append(warnings, fmt.Sprintf(
			"total resting notional only $%s across all buckets", grouped0(totalUSD)))
	}

	meta := make([]bucketMeta, len(buckets))
	bidsCents := make([]*float64, len(buckets))
	for i, b := range buckets {
		meta[i] = bucketMeta{lower: b.Lower, upper: b.Upper, queryID: b.QueryID}
		bidsCents[i] = views[i].BestBid
	}

	return assemble(meta, bidsCents, raw, confs, oneSidedFlags, quotedFlags, warnings)
}
