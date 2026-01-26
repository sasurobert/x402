package multiversx

import (
	"testing"
)

func TestGetMultiversXChainId(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		hasError bool
	}{
		{"mainnet", "1", false},
		{"multiversx-devnet", "D", false},
		{"multiversx:T", "T", false},
		{"multiversx:1", "1", false},
		{"multiversx:Custom", "Custom", false},
		{"invalid", "", true},
		{"multiversx-invalid", "", true},
	}

	for _, tc := range tests {
		res, err := GetMultiversXChainId(tc.input)
		if tc.hasError {
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.input, err)
			}
			if res != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, res)
			}
		}
	}
}

func TestIsValidAddress(t *testing.T) {
	tests := []struct {
		addr  string
		valid bool
	}{
		// Valid Addresses (examples from Docs or generated)
		// Bob: erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx
		{"erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx", true},

		// Invalid Length
		{"erd1short", false},
		{"erd1toolonggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg", false},

		// Invalid HRP
		{"btc1qyu5wthldzr8wx5c9ucg83cq4jgy80zy85ryfx475fsz99m4h39s292042", false},

		// Invalid Checksum (last char changed 2 -> 3)
		{"erd1qyu5wthldzr8wx5c9ucg83cq4jgy80zy85ryfx475fsz99m4h39s292043", false},

		{"ERD1QYU5WTHLDZR8WX5C9UCG83CQ4JGY80ZY85RYFX475FSZ99M4H39S292042", false},
	}

	for _, tc := range tests {
		if res := IsValidAddress(tc.addr); res != tc.valid {
			// Debug failure
			_, _, err := DecodeBech32(tc.addr)
			t.Errorf("IsValidAddress(%s) = %v; expected %v. Error: %v", tc.addr, res, tc.valid, err)
		}
	}
}

func TestCheckAmount(t *testing.T) {
	_, err := CheckAmount("1000")
	if err != nil {
		t.Errorf("Basic integer failed")
	}
	_, err = CheckAmount("abc")
	if err == nil {
		t.Errorf("Invalid string passed")
	}
}

func TestIsValidTokenID(t *testing.T) {
	tests := []struct {
		name    string
		tokenID string
		valid   bool
	}{
		{"EGLD", "EGLD", false}, // EGLD is not an ESDT TokenID
		{"Valid USDC", "USDC-123456", true},
		{"Valid WEGLD", "WEGLD-abcdef", true},
		{"Too Short Ticker", "A-123456", false}, // Ticker < 3 chars
		{"Invalid Nonce Length", "USDC-12345", false},
		{"Invalid Nonce Length Long", "USDC-1234567", false},
		{"Invalid Nonce Char", "USDC-12345G", false},
		{"No Dash", "USDC123456", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidTokenID(tt.tokenID); got != tt.valid {
				t.Errorf("IsValidTokenID(%q) = %v; want %v", tt.tokenID, got, tt.valid)
			}
		})
	}
}

func TestCalculateGasLimit(t *testing.T) {
	tests := []struct {
		name         string
		data         []byte
		numTransfers int
		expected     uint64
	}{
		{
			name:         "Base case (no data, no transfers)",
			data:         []byte{},
			numTransfers: 0,
			expected:     100000, // 50k base + 50k relayed
		},
		{
			name:         "One transfer (no data)",
			data:         []byte{},
			numTransfers: 1,
			expected:     300000, // 50k base + 200k transfer + 50k relayed
		},
		{
			name:         "Two transfers (no data)",
			data:         []byte{},
			numTransfers: 2,
			expected:     500000, // 50k base + 400k transfers + 50k relayed
		},
		{
			name:         "Data check (10 bytes)",
			data:         make([]byte, 10),
			numTransfers: 0,
			expected:     115000, // 50k base + 15k data (1.5k*10) + 50k relayed
		},
		{
			name:         "Complex case",
			data:         make([]byte, 10),
			numTransfers: 1,
			expected:     315000, // 50k + 15k + 200k + 50k
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateGasLimit(tt.data, tt.numTransfers)
			if got != tt.expected {
				t.Errorf("CalculateGasLimit() = %v; want %v", got, tt.expected)
			}
		})
	}
}
