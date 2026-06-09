package util

// Modular Agent Address (MAA) derivation.
//
// These three pure functions let an SDK caller compute an agent wallet's identifiers OFF-CHAIN,
// before the wallet exists on-chain — so a creator can publish a rule_id and a funder can know the
// exact MAA address to fund before sending a single token. They are a byte-exact mirror of the node
// precompiles (tn_utils.compute_rules_hash / derive_rule_id / derive_maa_address, migration 048):
//
//   ComputeRulesHash(fee_mode, fee_bps, fee_flat, namespaces, actions, body_hashes)
//       -> keccak256(RULES_PREIMAGE)                       (32 bytes; token-agnostic, NO bridge field)
//   DeriveRuleID(restricted, rules_hash, salt)
//       -> keccak256(RULE_ID_PREIMAGE)                     (32 bytes, NOT truncated — an identifier)
//   DeriveMAAAddress(unrestricted, restricted, rule_id)
//       -> keccak256(ADDRESS_PREIMAGE)[12:32]              (20-byte ETH address that holds funds)
//
// The exact byte layout is frozen and shared across every SDK and the node: one byte of disagreement
// derives a different address and would send funds to the wrong wallet. keccak256 is Ethereum/legacy
// Keccak (go-ethereum crypto.Keccak256), NOT NIST SHA3-256, NOT sha256.

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/big"
	"sort"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/pkg/errors"
)

const (
	maaRulesVersion  byte = 0x01 // RULES_PREIMAGE leading version byte
	maaRuleIDVersion byte = 0x01 // RULE_ID_PREIMAGE leading version byte
	maaAddrVersion   byte = 0x01 // ADDRESS_PREIMAGE leading version byte
)

type maaAllowEntry struct {
	namespace string
	action    string
	bodyHash  []byte // nil/empty = unpinned, else 32 bytes
}

// ComputeRulesHash builds the canonical RULES_PREIMAGE and returns its keccak256 (32 bytes).
//
// Layout: version(0x01) ‖ fee_mode(0x00 bps / 0x01 flat) ‖ fee_bps(uint32 BE) ‖ fee_flat(uint256 BE,
// 32 bytes) ‖ count(uint16 BE) ‖ for each canonical allow-list entry: u8len+namespace ‖ u8len+action ‖
// has_body(0x00 absent / 0x01 present) ‖ [body_hash 32 bytes if present]. Entries are canonicalized by
// (1) deduplicating on (namespace, action) — last write wins for the body_hash — then (2) sorting
// ascending bytewise on the raw UTF-8 namespace, then action. fee_flat is a base-unit decimal string.
//
// body_hashes may be nil for a non-empty allow-list, meaning "all entries unpinned"; otherwise it must
// be the same length as namespaces/actions (a nil element is an unpinned entry).
func ComputeRulesHash(feeMode string, feeBps int64, feeFlat string, namespaces, actions []string, bodyHashes [][]byte) ([]byte, error) {
	// Convenience mirroring the node handler: a nil/empty body_hashes alongside a non-empty allow-list
	// is treated as all-unpinned, so callers need not build a slice of nils.
	if len(bodyHashes) == 0 && len(namespaces) > 0 {
		bodyHashes = make([][]byte, len(namespaces))
	}
	if len(namespaces) != len(actions) || len(namespaces) != len(bodyHashes) {
		return nil, errors.Errorf("namespaces/actions/body_hashes must be equal length (%d/%d/%d)",
			len(namespaces), len(actions), len(bodyHashes))
	}

	var b bytes.Buffer
	b.WriteByte(maaRulesVersion)

	switch feeMode {
	case "bps":
		b.WriteByte(0x00)
	case "flat":
		b.WriteByte(0x01)
	default:
		return nil, errors.Errorf("fee_mode must be 'bps' or 'flat', got %q", feeMode)
	}

	if feeBps < 0 || feeBps > math.MaxUint32 {
		return nil, errors.Errorf("fee_bps out of uint32 range: %d", feeBps)
	}
	var bps [4]byte
	binary.BigEndian.PutUint32(bps[:], uint32(feeBps))
	b.Write(bps[:])

	feeFlatInt, err := maaParseFeeFlat(feeFlat)
	if err != nil {
		return nil, err
	}
	var ff [32]byte
	feeFlatInt.FillBytes(ff[:]) // big-endian, left-zero-padded
	b.Write(ff[:])

	// Canonicalize: dedup by (namespace, action) last-write-wins, then sort bytewise on raw UTF-8.
	dedup := make(map[string]maaAllowEntry, len(namespaces))
	for i := range namespaces {
		e := maaAllowEntry{namespace: namespaces[i], action: actions[i], bodyHash: bodyHashes[i]}
		dedup[e.namespace+"\x00"+e.action] = e
	}
	entries := make([]maaAllowEntry, 0, len(dedup))
	for _, e := range dedup {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].namespace != entries[j].namespace {
			return entries[i].namespace < entries[j].namespace
		}
		return entries[i].action < entries[j].action
	})

	if len(entries) > 0xffff {
		return nil, errors.Errorf("too many allow-list entries: %d", len(entries))
	}
	var cnt [2]byte
	binary.BigEndian.PutUint16(cnt[:], uint16(len(entries)))
	b.Write(cnt[:])

	for _, e := range entries {
		if err := maaWriteLP8(&b, []byte(e.namespace)); err != nil {
			return nil, errors.Wrapf(err, "namespace %q", e.namespace)
		}
		if err := maaWriteLP8(&b, []byte(e.action)); err != nil {
			return nil, errors.Wrapf(err, "action %q", e.action)
		}
		switch len(e.bodyHash) {
		case 0:
			b.WriteByte(0x00)
		case 32:
			b.WriteByte(0x01)
			b.Write(e.bodyHash)
		default:
			return nil, errors.Errorf("body_hash for %s.%s must be 32 bytes, got %d", e.namespace, e.action, len(e.bodyHash))
		}
	}

	return ethcrypto.Keccak256(b.Bytes()), nil
}

// DeriveRuleID builds RULE_ID_PREIMAGE = version(0x01) ‖ restricted(20) ‖ rules_hash(32) ‖ salt and
// returns the FULL 32-byte keccak256. rule_id is an identifier (the handle a funder passes to
// maa_join), not a fundable address, so it is NOT truncated. salt may be empty.
func DeriveRuleID(restricted, rulesHash, salt []byte) ([]byte, error) {
	if len(restricted) != 20 {
		return nil, errors.Errorf("restricted must be 20 bytes, got %d", len(restricted))
	}
	if len(rulesHash) != 32 {
		return nil, errors.Errorf("rules_hash must be 32 bytes, got %d", len(rulesHash))
	}

	var buf bytes.Buffer
	buf.WriteByte(maaRuleIDVersion)
	buf.Write(restricted)
	buf.Write(rulesHash)
	buf.Write(salt) // last; variable length; empty is valid

	return ethcrypto.Keccak256(buf.Bytes()), nil // 32 bytes, untruncated
}

// DeriveMAAAddress builds ADDRESS_PREIMAGE = version(0x01) ‖ unrestricted(20) ‖ restricted(20) ‖
// rule_id(32) and returns the low 20 bytes of its keccak256 — the Ethereum-style MAA address that
// holds funds. The composite (unrestricted, restricted, rule_id) means one rule can be funded by many
// owners, each producing a distinct wallet.
func DeriveMAAAddress(unrestricted, restricted, ruleID []byte) ([]byte, error) {
	if len(unrestricted) != 20 {
		return nil, errors.Errorf("unrestricted must be 20 bytes, got %d", len(unrestricted))
	}
	if len(restricted) != 20 {
		return nil, errors.Errorf("restricted must be 20 bytes, got %d", len(restricted))
	}
	if len(ruleID) != 32 {
		return nil, errors.Errorf("rule_id must be 32 bytes, got %d", len(ruleID))
	}

	var buf bytes.Buffer
	buf.WriteByte(maaAddrVersion)
	buf.Write(unrestricted)
	buf.Write(restricted)
	buf.Write(ruleID)

	full := ethcrypto.Keccak256(buf.Bytes()) // 32 bytes
	out := make([]byte, 20)
	copy(out, full[12:32]) // low 20 bytes
	return out, nil
}

// maaParseFeeFlat parses a base-unit decimal string into a non-negative big.Int that fits in 256 bits.
// An empty string is 0.
func maaParseFeeFlat(s string) (*big.Int, error) {
	if s == "" {
		return big.NewInt(0), nil
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, errors.Errorf("fee_flat is not a base-10 integer: %q", s)
	}
	if v.Sign() < 0 {
		return nil, errors.Errorf("fee_flat must be non-negative: %s", s)
	}
	if v.BitLen() > 256 {
		return nil, errors.Errorf("fee_flat exceeds 2^256: %s", s)
	}
	return v, nil
}

// maaWriteLP8 writes a uint8 length prefix followed by the bytes (length-prefixed UTF-8 fields).
func maaWriteLP8(buf *bytes.Buffer, p []byte) error {
	if len(p) > 0xff {
		return errors.Errorf("length-prefixed field exceeds 255 bytes (got %d)", len(p))
	}
	buf.WriteByte(byte(len(p)))
	buf.Write(p)
	return nil
}
