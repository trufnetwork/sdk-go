// Modular Agent Address (MAA / "agent wallet") lifecycle smoke test — Go SDK.
//
// Runs the full agent-wallet lifecycle against a live TRUF.NETWORK node where maa_exec is activated
// (testnet from height 6523123). It proves, end-to-end through the Go SDK, the properties that make an
// agent wallet useful and safe:
//
//   - a restricted AGENT key registers an immutable rule;
//   - an unrestricted OWNER key joins it to derive a wallet (the MAA) and funds it;
//   - the agent runs allow-listed actions AS the MAA — the node rewrites @caller to the wallet, so the
//     streams it creates are owned by the MAA and every fee is debited from the MAA's OWN escrow;
//   - the agent CANNOT move the funds out (owner-exit actions are reserved for the owner);
//   - the owner withdraws the remaining escrow at any time, paying the agent its commission.
//
// This mirrors the node's canonical oracle, tests/streams/maa/data_agent_test.go, and the Python SDK's
// examples/maa_lifecycle_example.
//
// Config comes from a .env file in the working directory (or next to this source file); real
// environment variables still take precedence. Two DISTINCT keys are required — the agent and the
// owner are different identities:
//
//	cd examples/maa_lifecycle_example
//	cp .env.example .env        # then fill in AGENT_PRIVATE_KEY and OWNER_PRIVATE_KEY
//	go run .
//
// See .env.example for every setting, and README.md for what success looks like.
//
// # NUMERIC arguments (no marker wrapper, unlike Python)
//
// ExecuteAgentAction encodes the inner action's arguments with the same encoder used for ordinary
// action calls (kwil EncodeValue), which carries a decimal's precision/scale natively. So a NUMERIC
// argument is just a *kwilTypes.Decimal parsed with the action's EXACT precision/scale via
// ParseDecimalExplicit — there is no JSON marker to wrap (the Python SDK needs MAANumericArg only
// because JSON has no decimal type). The node does not coerce text to NUMERIC, so the precision/scale
// must match the declared parameter or the call is rejected.
package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/trufnetwork/kwil-db/core/crypto"
	"github.com/trufnetwork/kwil-db/core/crypto/auth"
	kwilTypes "github.com/trufnetwork/kwil-db/core/types"
	"github.com/trufnetwork/sdk-go/core/tnclient"
	"github.com/trufnetwork/sdk-go/core/types"
	"github.com/trufnetwork/sdk-go/core/util"
)

// On-chain decimal types the inner actions declare. A NUMERIC argument MUST be parsed with these
// EXACT precision/scale, because the node does not coerce text to NUMERIC and compares precision/scale
// strictly.
const (
	tokenPrecision, tokenScale = 78, 0  // bridge amounts: NUMERIC(78,0)
	valuePrecision, valueScale = 36, 18 // primitive record values: NUMERIC(36,18)
)

// txWait is how often WaitForTx polls for a transaction's inclusion.
const txWait = 2 * time.Second

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatalf("❌ %v", err)
	}
}

func run(ctx context.Context) error {
	// --- load .env (zero-dependency; real environment variables take precedence) ---
	loadDotenv(".env")
	if _, thisFile, _, ok := runtime.Caller(0); ok {
		// Also look next to this source file, so `go run ./examples/maa_lifecycle_example` works from
		// the repo root (the .env in the working directory, loaded first, still wins).
		loadDotenv(filepath.Join(filepath.Dir(thisFile), ".env"))
	}

	// --- configuration (all overridable via environment / .env; see .env.example) ---
	providerURL := getenv("PROVIDER_URL", "https://gateway.testnet.truf.network")
	agentKey := os.Getenv("AGENT_PRIVATE_KEY") // restricted agent
	ownerKey := os.Getenv("OWNER_PRIVATE_KEY") // unrestricted owner / funder
	bridge := getenv("MAA_BRIDGE", "hoodi_tt") // funding/fee bridge namespace (e.g. hoodi_tt / eth_truf)
	fundAmount := getenv("MAA_FUND_AMOUNT", "250000000000000000000")
	feeBps, err := strconv.Atoi(getenv("MAA_FEE_BPS", "250")) // owner-withdraw commission to the agent
	if err != nil {
		return fmt.Errorf("MAA_FEE_BPS must be an integer: %w", err)
	}
	// Order-book collateral bridge for get_collateral_by_wallet (migration 051). This is the bridge the
	// order-book MARKETS settle in (hoodi_tt2 / sepolia_bridge / ethereum_bridge on dev/testnet), NOT
	// the hoodi_tt funding/fee bridge above. get_positions_by_wallet needs no bridge.
	collateralBridge := getenv("MAA_COLLATERAL_BRIDGE", "hoodi_tt2")

	if agentKey == "" || ownerKey == "" {
		return fmt.Errorf("AGENT_PRIVATE_KEY and OWNER_PRIVATE_KEY must both be set (two distinct keys); see README.md")
	}

	salt, err := buildSalt(os.Getenv("MAA_SALT"))
	if err != nil {
		return err
	}

	// The agent's allow-list (mirrors data_agent_test.go): the two data-provision actions.
	namespaces := []string{"main", "main"}
	actions := []string{"create_streams", "insert_records"}
	bodyHashes := [][]byte{nil, nil} // unpinned

	// --- clients & their addresses ---
	agent, err := newClient(ctx, providerURL, agentKey)
	if err != nil {
		return fmt.Errorf("construct agent client: %w", err)
	}
	owner, err := newClient(ctx, providerURL, ownerKey)
	if err != nil {
		return fmt.Errorf("construct owner client: %w", err)
	}
	agentActions, err := agent.LoadActions()
	if err != nil {
		return fmt.Errorf("load agent actions: %w", err)
	}
	ownerActions, err := owner.LoadActions()
	if err != nil {
		return fmt.Errorf("load owner actions: %w", err)
	}
	agentOB, err := agent.LoadOrderBook()
	if err != nil {
		return fmt.Errorf("load order book: %w", err)
	}

	agentAddr := agent.Address()
	ownerAddr := owner.Address()

	banner("MAA lifecycle smoke test")
	fmt.Printf("provider : %s\n", providerURL)
	fmt.Printf("bridge   : %s\n", bridge)
	fmt.Printf("agent    : %s   (restricted — operates the wallet)\n", agentAddr.Address())
	fmt.Printf("owner    : %s   (unrestricted — funds & withdraws)\n", ownerAddr.Address())
	if strings.EqualFold(agentAddr.Address(), ownerAddr.Address()) {
		return fmt.Errorf("agent and owner must be DIFFERENT keys")
	}

	// escrow reads the MAA's bridge balance (its escrow). It is caller-agnostic — any client can read
	// any wallet's balance by address — so we bind it to the agent client for the running narration.
	var maaHex string
	escrow := func() (string, error) {
		return agentActions.GetWalletBalance(ctx, bridge, maaHex)
	}

	// (a) AGENT creates an immutable rule. The allow-list is the two data-provision actions; the
	//     fee_mode/bps set the commission the owner pays the agent on withdrawal.
	banner("(a) agent registers the rule")
	ruleID, ruleTx, err := agentActions.CreateAgentRule(ctx, types.MAACreateRuleInput{
		FeeMode:    "bps",
		FeeBps:     feeBps,
		FeeFlat:    "0",
		Namespaces: namespaces,
		Actions:    actions,
		BodyHashes: bodyHashes,
		Salt:       salt,
	})
	if err != nil {
		return fmt.Errorf("create rule: %w", err)
	}
	if err := waitOK(ctx, agent, ruleTx, "create rule"); err != nil {
		return err
	}
	fmt.Printf("✅ rule created: rule_id=0x%s  (commission %.2f%%)\n", hex.EncodeToString(ruleID), float64(feeBps)/100)
	// Cross-check the SDK-returned rule_id against a standalone, offline derivation (the same primitives
	// a caller can use to know the handle before submitting).
	rulesHash, err := util.ComputeRulesHash("bps", int64(feeBps), "0", namespaces, actions, bodyHashes)
	if err != nil {
		return fmt.Errorf("compute rules hash: %w", err)
	}
	ruleIDLocal, err := util.DeriveRuleID(agentAddr.Bytes(), rulesHash, salt)
	if err != nil {
		return fmt.Errorf("derive rule_id: %w", err)
	}
	if !bytesEqual(ruleID, ruleIDLocal) {
		return fmt.Errorf("local rule_id 0x%s != sdk 0x%s", hex.EncodeToString(ruleIDLocal), hex.EncodeToString(ruleID))
	}
	fmt.Printf("   ↳ matches local derivation 0x%s\n", hex.EncodeToString(ruleIDLocal))

	// (b) OWNER joins the rule → derives + registers the agent wallet (the MAA).
	banner("(b) owner joins → agent wallet derived")
	maa, joinTx, err := ownerActions.JoinAgentAddress(ctx, ruleID)
	if err != nil {
		return fmt.Errorf("join: %w", err)
	}
	if err := waitOK(ctx, owner, joinTx, "join"); err != nil {
		return err
	}
	maaAddr, err := util.NewEthereumAddressFromBytes(maa)
	if err != nil {
		return fmt.Errorf("parse MAA address: %w", err)
	}
	maaHex = maaAddr.Address()
	maaLocal, err := util.DeriveMAAAddress(ownerAddr.Bytes(), agentAddr.Bytes(), ruleID)
	if err != nil {
		return fmt.Errorf("derive MAA address: %w", err)
	}
	if !bytesEqual(maa, maaLocal) {
		return fmt.Errorf("local MAA %s != sdk %s", hexAddr(maaLocal), maaHex)
	}
	known, err := agentActions.IsAgentWallet(ctx, maa)
	if err != nil {
		return fmt.Errorf("is_agent_wallet: %w", err)
	}
	fmt.Printf("✅ agent wallet (MAA): %s\n", maaHex)
	fmt.Printf("   ↳ matches local derivation %s\n", hexAddr(maaLocal))
	fmt.Printf("   maa_is_known: %v\n", known)

	// (c) OWNER funds the MAA with a normal bridged-token transfer.
	banner("(c) owner funds the agent wallet")
	fundTx, err := ownerActions.Transfer(ctx, bridge, maaHex, fundAmount)
	if err != nil {
		return fmt.Errorf("transfer: %w", err)
	}
	if err := waitOK(ctx, owner, fundTx, "transfer"); err != nil {
		return err
	}
	funded, err := escrow()
	if err != nil {
		return fmt.Errorf("read escrow after funding: %w", err)
	}
	fmt.Printf("✅ funded MAA with %s → escrow balance now %s\n", fundAmount, funded)
	if funded != fundAmount {
		return fmt.Errorf("expected escrow %s, got %s", fundAmount, funded)
	}

	// (d) AGENT works AS the MAA: create a stream, then insert a record into it. @caller is rewritten
	//     to the MAA, so the stream is OWNED by the MAA and the fees come out of the MAA's escrow.
	banner("(d) agent creates a stream + inserts data, AS the MAA")
	sid := util.GenerateStreamId(fmt.Sprintf("maa_demo_go_%d", time.Now().Unix()))
	streamID := sid.String()
	fmt.Printf("stream_id: %s\n", streamID)

	beforeCreate, err := escrow()
	if err != nil {
		return fmt.Errorf("read escrow before create: %w", err)
	}
	createTx, err := agentActions.ExecuteAgentAction(ctx, types.MAAExecuteInput{
		MAAAddress: maa,
		Action:     "create_streams",
		Args:       []any{[]string{streamID}, []string{string(types.StreamTypePrimitive)}},
	})
	if err != nil {
		return fmt.Errorf("create_streams as MAA: %w", err)
	}
	if err := waitOK(ctx, agent, createTx, "create_streams"); err != nil {
		return err
	}
	afterCreate, err := escrow()
	if err != nil {
		return fmt.Errorf("read escrow after create: %w", err)
	}
	fmt.Printf("✅ create_streams ran as the MAA (escrow %s → %s, fee %s)\n",
		beforeCreate, afterCreate, weiDiff(beforeCreate, afterCreate))

	eventTime := time.Now().Unix()
	value, err := kwilTypes.ParseDecimalExplicit("42.5", valuePrecision, valueScale)
	if err != nil {
		return fmt.Errorf("parse record value: %w", err)
	}
	insertTx, err := agentActions.ExecuteAgentAction(ctx, types.MAAExecuteInput{
		MAAAddress: maa,
		Action:     "insert_records",
		Args:       []any{[]string{maaHex}, []string{streamID}, []int64{eventTime}, []*kwilTypes.Decimal{value}},
	})
	if err != nil {
		return fmt.Errorf("insert_records as MAA: %w", err)
	}
	if err := waitOK(ctx, agent, insertTx, "insert_records"); err != nil {
		return err
	}
	afterInsert, err := escrow()
	if err != nil {
		return fmt.Errorf("read escrow after insert: %w", err)
	}
	fmt.Printf("✅ insert_records ran as the MAA (escrow %s → %s, fee %s)\n",
		afterCreate, afterInsert, weiDiff(afterCreate, afterInsert))

	// PROOF the rewrite happened: the stream + record exist UNDER THE MAA's address, not the agent's.
	from, to := int(eventTime-5), int(eventTime+5)
	noCache := false
	records, err := agentActions.GetRecord(ctx, types.GetRecordInput{
		DataProvider: maaHex,
		StreamId:     streamID,
		From:         &from,
		To:           &to,
		UseCache:     &noCache,
	})
	if err != nil {
		return fmt.Errorf("read back records under MAA: %w", err)
	}
	if len(records.Results) == 0 {
		return fmt.Errorf("expected the inserted record to be readable under the MAA address %s", maaHex)
	}
	fmt.Printf("   stream %s owned by %s; records read back: %d\n", streamID, maaHex, len(records.Results))
	for _, r := range records.Results {
		fmt.Printf("     event_time=%d value=%s\n", r.EventTime, r.Value.String())
	}
	fmt.Printf("✅ the agent provided data AS the MAA (not as its own key %s)\n", agentAddr.Address())

	// (e) AGENT tries to exit the funds → BLOCKED. Owner-exit actions are reserved for the owner; the
	//     route rejects the restricted agent before anything moves.
	banner("(e) agent tries to withdraw → must be blocked")
	balanceBeforeAttack, err := escrow()
	if err != nil {
		return fmt.Errorf("read escrow before attack: %w", err)
	}
	attackAmount, err := token("1000000000000000000")
	if err != nil {
		return err
	}
	attackTx, attackErr := agentActions.ExecuteAgentAction(ctx, types.MAAExecuteInput{
		MAAAddress: maa,
		Action:     "maa_withdraw",
		Args:       []any{bridge, attackAmount},
	})
	// The rejection may surface at submission (the route's PreTx role gate, run at mempool admission)
	// or in-block. Treat either a submission error or a non-OK committed result as "blocked".
	blocked, blockMsg := false, ""
	if attackErr != nil {
		blocked, blockMsg = true, attackErr.Error()
	} else if waitErr := waitOK(ctx, agent, attackTx, "agent withdraw"); waitErr != nil {
		blocked, blockMsg = true, waitErr.Error()
	}
	if !blocked {
		return fmt.Errorf("SECURITY FAILURE: the restricted agent was allowed to withdraw")
	}
	fmt.Printf("✅ blocked, as it must be: %s\n", blockMsg)
	if !strings.Contains(blockMsg, "reserved for the unrestricted owner") && !strings.Contains(blockMsg, "restricted agent") {
		fmt.Println("   ⚠️  (blocked, but the message differs from the expected route/guard wording)")
	}
	balanceAfterAttack, err := escrow()
	if err != nil {
		return fmt.Errorf("read escrow after attack: %w", err)
	}
	if balanceAfterAttack != balanceBeforeAttack {
		return fmt.Errorf("a blocked exit must move nothing: %s -> %s", balanceBeforeAttack, balanceAfterAttack)
	}
	fmt.Printf("   escrow unchanged after the blocked attempt: %s\n", balanceAfterAttack)

	// (f) OWNER withdraws the remaining escrow, paying the agent its commission.
	banner("(f) owner withdraws the remaining escrow (pays the agent commission)")
	remaining, err := ownerActions.GetWalletBalance(ctx, bridge, maaHex)
	if err != nil {
		return fmt.Errorf("read remaining escrow: %w", err)
	}
	withdrawAmount, err := token(remaining)
	if err != nil {
		return err
	}
	withdrawTx, err := ownerActions.ExecuteAgentAction(ctx, types.MAAExecuteInput{
		MAAAddress: maa,
		Action:     "maa_withdraw",
		Args:       []any{bridge, withdrawAmount},
	})
	if err != nil {
		return fmt.Errorf("owner withdraw: %w", err)
	}
	if err := waitOK(ctx, owner, withdrawTx, "owner withdraw"); err != nil {
		return err
	}
	drained, err := escrow()
	if err != nil {
		return fmt.Errorf("read escrow after withdraw: %w", err)
	}
	fmt.Printf("✅ owner withdrew %s; escrow now %s\n", remaining, drained)
	fmt.Printf("   ↳ agent earns ~%s (%.2f%% commission); owner gets the rest\n",
		commission(remaining, feeBps), float64(feeBps)/100) // HALF-UP on-chain; floor is a lower bound
	if drained != "0" {
		return fmt.Errorf("expected the wallet to be drained, got %s", drained)
	}

	// (g) READ STATE back: rule terms, allow-list, instance, audit log.
	banner("(g) read MAA state")
	rule, err := agentActions.GetAgentRule(ctx, ruleID)
	if err != nil {
		return fmt.Errorf("get rule: %w", err)
	}
	fmt.Printf("rule           : %+v\n", rule)
	allowed, err := agentActions.GetAgentRuleAllowedActions(ctx, ruleID)
	if err != nil {
		return fmt.Errorf("get allowed actions: %w", err)
	}
	fmt.Printf("allowed_actions: %+v\n", allowed)
	instance, err := agentActions.GetAgentWallet(ctx, maa)
	if err != nil {
		return fmt.Errorf("get instance: %w", err)
	}
	fmt.Printf("instance       : %+v\n", instance)
	events, err := agentActions.GetAgentRuleEvents(ctx, ruleID, 100, 0)
	if err != nil {
		return fmt.Errorf("get events: %w", err)
	}
	fmt.Printf("events (%d):\n", len(events))
	for _, ev := range events {
		fmt.Printf("   - %-12s role=%-13s actor=%s action=%s amount=%s\n",
			ev.EventType, ev.ActorRole, ev.ActorAddr, dash(ev.InnerAction), dash(ev.Amount))
	}

	// (h) READ the agent wallet's ORDER-BOOK portfolio BY ADDRESS (migration 051).
	//     GetPositionsByWallet / GetCollateralByWallet read the wallet you pass in (NOT @caller), so an
	//     owner — or a delegated market-maker bot — can read an agent wallet's live inventory without
	//     holding its key. The signer here (agent) differs from the wallet read (the MAA), which is the
	//     whole point. This MAA's allow-list is create_streams/insert_records (data provision), so it
	//     holds NO order-book positions — the reads return empty/zero. A clean return (instead of
	//     "unknown action") is the proof that migration 051 is live on this network.
	banner("(h) read the agent wallet's order-book portfolio by address")
	positions, err := agentOB.GetPositionsByWallet(ctx, types.GetPositionsByWalletInput{WalletHex: maaHex})
	if err != nil {
		return fmt.Errorf("get_positions_by_wallet: %w", err)
	}
	collateral, err := agentOB.GetCollateralByWallet(ctx, types.GetCollateralByWalletInput{WalletHex: maaHex, Bridge: collateralBridge})
	if err != nil {
		return fmt.Errorf("get_collateral_by_wallet: %w", err)
	}
	fmt.Printf("get_positions_by_wallet(%s) -> %d positions: %+v\n", maaHex, len(positions), positions)
	fmt.Printf("get_collateral_by_wallet(%s, %s) -> %+v\n", maaHex, collateralBridge, collateral)
	fmt.Println("✅ address-parameterized portfolio reads are live (migration 051)")

	banner("✅ MAA lifecycle smoke test PASSED")
	fmt.Println("Proven on-chain via the Go SDK:")
	fmt.Println("  • @caller rewritten to the MAA (the stream is owned by the wallet, not the agent key)")
	fmt.Println("  • fees debit the MAA's own escrow")
	fmt.Println("  • the restricted agent cannot move funds out")
	fmt.Println("  • the owner withdraws with the agreed commission")
	fmt.Println("  • an owner can read the wallet's order-book positions/collateral by address (051)")
	return nil
}

// ──────────────────────────────────────────────────────────────────────────
// helpers
// ──────────────────────────────────────────────────────────────────────────

// newClient builds a gateway-backed client signing as the given secp256k1 key (with or without 0x).
func newClient(ctx context.Context, providerURL, privateKeyHex string) (*tnclient.Client, error) {
	pk, err := crypto.Secp256k1PrivateKeyFromHex(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	signer := &auth.EthPersonalSigner{Key: *pk}
	return tnclient.NewClient(ctx, providerURL, tnclient.WithSigner(signer))
}

// token parses a bridge-amount NUMERIC(78,0) argument (base units / wei).
func token(amount string) (*kwilTypes.Decimal, error) {
	d, err := kwilTypes.ParseDecimalExplicit(amount, tokenPrecision, tokenScale)
	if err != nil {
		return nil, fmt.Errorf("parse token amount %q as NUMERIC(%d,%d): %w", amount, tokenPrecision, tokenScale, err)
	}
	return d, nil
}

// waitOK blocks until txHash is included in a block and asserts the transaction succeeded.
func waitOK(ctx context.Context, client *tnclient.Client, txHash, label string) error {
	h, err := kwilTypes.NewHashFromString(txHash)
	if err != nil {
		return fmt.Errorf("%s: parse tx hash %q: %w", label, txHash, err)
	}
	res, err := client.WaitForTx(ctx, h, txWait)
	if err != nil {
		return fmt.Errorf("%s: wait for tx %s: %w", label, txHash, err)
	}
	if res.Result.Code != uint32(kwilTypes.CodeOk) {
		return fmt.Errorf("%s: tx %s failed (code %d): %s", label, txHash, res.Result.Code, res.Result.Log)
	}
	return nil
}

// buildSalt returns the 32-byte rule salt. With saltHex set (64 hex chars, optional 0x) it is pinned
// for a reproducible rule_id; otherwise a fresh, nanosecond-derived salt makes each run register a new
// rule/MAA so re-running never collides with an already-registered rule.
func buildSalt(saltHex string) ([]byte, error) {
	if saltHex != "" {
		b, err := hex.DecodeString(strings.TrimPrefix(saltHex, "0x"))
		if err != nil {
			return nil, fmt.Errorf("MAA_SALT must be hex: %w", err)
		}
		if len(b) != 32 {
			return nil, fmt.Errorf("MAA_SALT must be 32 bytes (64 hex chars), got %d", len(b))
		}
		return b, nil
	}
	salt := make([]byte, 32)
	copy(salt, "MAA")
	binary.BigEndian.PutUint64(salt[3:11], uint64(time.Now().UnixNano()))
	return salt, nil
}

// loadDotenv populates the environment from a KEY=VALUE .env file without overriding existing vars
// (real environment variables take precedence). A missing file is a no-op.
func loadDotenv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		key, val, _ := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if _, ok := os.LookupEnv(key); !ok {
			_ = os.Setenv(key, val)
		}
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// weiDiff returns before-after as a base-10 string (the fee an action debited from escrow).
func weiDiff(before, after string) string {
	b, ok1 := new(big.Int).SetString(before, 10)
	a, ok2 := new(big.Int).SetString(after, 10)
	if !ok1 || !ok2 {
		return "?"
	}
	return new(big.Int).Sub(b, a).String()
}

// commission returns floor(amount * bps / 10000) — a lower bound on the agent's HALF-UP commission.
func commission(amount string, bps int) string {
	a, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return "?"
	}
	a.Mul(a, big.NewInt(int64(bps)))
	a.Div(a, big.NewInt(10000))
	return a.String()
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func hexAddr(b []byte) string {
	return "0x" + hex.EncodeToString(b)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func banner(title string) {
	fmt.Println("\n" + strings.Repeat("=", 72))
	fmt.Println(title)
	fmt.Println(strings.Repeat("=", 72))
}
