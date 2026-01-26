package server

import (
	"context"
	"testing"

	x402 "github.com/coinbase/x402/go"
	"github.com/coinbase/x402/go/types"
)

func TestParsePrice(t *testing.T) {
	scheme := NewExactMultiversXScheme()

	// Register a dummy parser for USDC (6 decimals)
	scheme.RegisterMoneyParser(func(amount float64, network x402.Network) (*x402.AssetAmount, error) {
		// Mock: if amount is small, we assume it's for USDC test
		if amount == 123.456 {
			return &x402.AssetAmount{
				Amount: "123456000", // 123.456 * 10^6
				Asset:  "USDC-123456",
			}, nil
		}
		return nil, nil
	})

	tests := []struct {
		name      string
		price     interface{}
		wantAsset string
		wantAmt   string
		wantErr   bool
	}{
		{
			name:      "String EGLD (Default)",
			price:     "1.5",
			wantAsset: "EGLD",
			wantAmt:   "1500000000000000000", // 1.5 * 10^18
			wantErr:   false,
		},
		{
			name:      "Float EGLD",
			price:     1.5,
			wantAsset: "EGLD",
			wantAmt:   "1500000000000000000",
			wantErr:   false,
		},
		{
			name:      "Map with String Amount (Raw)",
			price:     map[string]interface{}{"amount": "100", "asset": "EGLD"},
			wantAsset: "EGLD",
			wantAmt:   "100",
			wantErr:   false,
		},
		{
			name:      "Custom Parser (USDC)",
			price:     123.456,
			wantAsset: "USDC-123456",
			wantAmt:   "123456000",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scheme.ParsePrice(tt.price, "multiversx:D")
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePrice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Asset != tt.wantAsset {
					t.Errorf("ParsePrice() asset = %v, want %v", got.Asset, tt.wantAsset)
				}
				if got.Amount != tt.wantAmt {
					t.Errorf("ParsePrice() amount = %v, want %v", got.Amount, tt.wantAmt)
				}
			}
		})
	}
}

func TestValidatePaymentRequirements(t *testing.T) {
	scheme := NewExactMultiversXScheme()

	tests := []struct {
		name    string
		req     x402.PaymentRequirements
		wantErr bool
	}{
		{
			name: "Valid EGLD Request",
			req: x402.PaymentRequirements{
				PayTo:  "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx",
				Amount: "1000",
				Asset:  "EGLD",
			},
			wantErr: false,
		},
		{
			name: "Valid ESDT Request",
			req: x402.PaymentRequirements{
				PayTo:  "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx",
				Amount: "1000",
				Asset:  "USDC-123456",
			},
			wantErr: false,
		},
		{
			name: "Invalid Address",
			req: x402.PaymentRequirements{
				PayTo:  "erd1invalid",
				Amount: "1000",
				Asset:  "EGLD",
			},
			wantErr: true,
		},
		{
			name: "Invalid TokenID",
			req: x402.PaymentRequirements{
				PayTo:  "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx",
				Amount: "1000",
				Asset:  "INVALID-TOKEN-ID-TOO-LONG", // Invalid
			},
			wantErr: true,
		},
		{
			name: "Missing Amount",
			req: x402.PaymentRequirements{
				PayTo:  "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx",
				Amount: "", // Required
				Asset:  "EGLD",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := scheme.ValidatePaymentRequirements(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePaymentRequirements() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnhancePaymentRequirements(t *testing.T) {
	scheme := NewExactMultiversXScheme()

	req := types.PaymentRequirements{
		PayTo: "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx",
		Asset: "EGLD", // Explicitly provided as default is removed
	}

	got, err := scheme.EnhancePaymentRequirements(context.Background(), req, types.SupportedKind{}, nil)
	if err != nil {
		t.Fatalf("EnhancePaymentRequirements error: %v", err)
	}

	if got.Extra["assetTransferMethod"] != "direct" {
		t.Errorf("Expected transfer method direct, got %v", got.Extra["assetTransferMethod"])
	}
}
