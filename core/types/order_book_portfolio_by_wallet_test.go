package types

import "testing"

// The address-parameterized portfolio getters accept a wallet with OR without a
// 0x prefix (the node action normalizes either form), so validation must too.

func TestGetPositionsByWalletInput_Validate(t *testing.T) {
	cases := []struct {
		name    string
		input   GetPositionsByWalletInput
		wantErr bool
	}{
		{"0x-prefixed", GetPositionsByWalletInput{WalletHex: "0x12aae9a9cf034cb71cbf17cfa1e9612cda8e8a87"}, false},
		{"bare hex (no 0x)", GetPositionsByWalletInput{WalletHex: "12aae9a9cf034cb71cbf17cfa1e9612cda8e8a87"}, false},
		{"empty", GetPositionsByWalletInput{WalletHex: ""}, true},
		{"too short", GetPositionsByWalletInput{WalletHex: "0x1234"}, true},
		{"non-hex", GetPositionsByWalletInput{WalletHex: "0xZZae9a9cf034cb71cbf17cfa1e9612cda8e8a87"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.input.Validate()
			if c.wantErr && err == nil {
				t.Fatalf("Validate(%q) = nil, want error", c.input.WalletHex)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("Validate(%q) = %v, want nil", c.input.WalletHex, err)
			}
		})
	}
}

func TestGetCollateralByWalletInput_Validate(t *testing.T) {
	cases := []struct {
		name    string
		input   GetCollateralByWalletInput
		wantErr bool
	}{
		{"valid 0x + bridge", GetCollateralByWalletInput{WalletHex: "0x12aae9a9cf034cb71cbf17cfa1e9612cda8e8a87", Bridge: "hoodi_tt"}, false},
		{"valid bare + bridge", GetCollateralByWalletInput{WalletHex: "12aae9a9cf034cb71cbf17cfa1e9612cda8e8a87", Bridge: "eth_truf"}, false},
		{"missing bridge", GetCollateralByWalletInput{WalletHex: "0x12aae9a9cf034cb71cbf17cfa1e9612cda8e8a87", Bridge: ""}, true},
		{"bad wallet", GetCollateralByWalletInput{WalletHex: "nope", Bridge: "hoodi_tt"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.input.Validate()
			if c.wantErr && err == nil {
				t.Fatalf("Validate(%+v) = nil, want error", c.input)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("Validate(%+v) = %v, want nil", c.input, err)
			}
		})
	}
}
