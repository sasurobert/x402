package client

import (
	"context"
	"strings"
	"testing"

	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-sdk-go/core"
	"github.com/multiversx/mx-sdk-go/data"

	"github.com/coinbase/x402/go/mechanisms/multiversx"

	"github.com/coinbase/x402/go/types"
)

// MockSigner matches ClientMultiversXSigner
type MockSigner struct {
	addr string
}

func (m *MockSigner) Address() string {
	return m.addr
}
func (s *MockSigner) PrivateKey() []byte {
	// Valid 32-byte seed
	return []byte{
		0x41, 0x3f, 0x42, 0x57, 0x5f, 0x7f, 0x26, 0xfa,
		0xd3, 0x31, 0x7a, 0x77, 0x87, 0x71, 0x21, 0x2f,
		0xdb, 0x80, 0x24, 0x58, 0x50, 0x98, 0x1e, 0x48,
		0xb5, 0x8a, 0x4f, 0x25, 0xe3, 0x44, 0xe8, 0xf9,
	}
}

func (m *MockSigner) Sign(ctx context.Context, message []byte) ([]byte, error) {
	return []byte("signature"), nil
}

const (
	// Valid Bech32 Addresses for Testing (Alice/Bob Devnet)
	testPayTo  = "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx" // Alice (using Bob's valid address)
	testSender = "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx" // Bob
	testAsset  = "TEST-123456"
	testAmount = "1000000000000000000" // 1 EGLD
)

// MockProxy implements Proxy interface
type MockProxy struct {
	nonce uint64
	err   error
}

// GetAccount must match blockchain.Proxy interface
func (m *MockProxy) GetAccount(ctx context.Context, address core.AddressHandler) (*data.Account, error) {
	return &data.Account{
		Nonce: m.nonce,
	}, m.err
}

func (m *MockProxy) SendTransaction(ctx context.Context, tx *transaction.FrontendTransaction) (string, error) {
	return "txHash", nil
}

func (m *MockProxy) GetNetworkConfig(ctx context.Context) (*data.NetworkConfig, error) {
	return &data.NetworkConfig{
		MinGasLimit: 50000,
		MinGasPrice: 1000000000,
		ChainID:     "D",
	}, nil
}

// IsInterfaceNil required by Proxy interface
func (m *MockProxy) IsInterfaceNil() bool {
	return m == nil
}

// Helpers methods required by Proxy interface (stubs)
func (m *MockProxy) SendTransactions(ctx context.Context, txs []*transaction.FrontendTransaction) ([]string, error) {
	return []string{"txHash"}, nil
}
func (m *MockProxy) GetGuardianData(ctx context.Context, address core.AddressHandler) (*api.GuardianData, error) {
	return nil, nil // Not used
}
func (m *MockProxy) ExecuteVMQuery(ctx context.Context, vmRequest *data.VmValueRequest) (*data.VmValuesResponseData, error) {
	return nil, nil // Not used
}
func (m *MockProxy) FilterLogs(ctx context.Context, filter *core.FilterQuery) ([]*transaction.Events, error) {
	return nil, nil // Not used
}

func TestCreatePaymentPayload_EGLD(t *testing.T) {
	signer := &MockSigner{addr: testSender}
	mockProxy := &MockProxy{nonce: 15}
	scheme, _ := NewExactMultiversXScheme(signer, "multiversx:D", WithProxy(mockProxy))

	req := types.PaymentRequirements{
		PayTo:   testPayTo,
		Amount:  "100",
		Asset:   "EGLD",
		Network: "multiversx:D",
		Extra: map[string]interface{}{
			"relayer": testSender,
		},
	}

	payload, err := scheme.CreatePaymentPayload(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to create payload: %v", err)
	}

	// Verify structure
	rpPtr, err := multiversx.PayloadFromMap(payload.Payload)
	if err != nil {
		t.Fatalf("Failed to parse payload: %v", err)
	}
	rp := *rpPtr

	if rp.Receiver != testPayTo {
		t.Errorf("Wrong receiver: %s", rp.Receiver)
	}
	if rp.Value != "100" {
		t.Errorf("Wrong value: %s", rp.Value)
	}
	if rp.Data != "" {
		t.Errorf("Expected empty data for EGLD, got %s", rp.Data)
	}
	if rp.Nonce != 15 {
		t.Errorf("Wrong nonce: %d", rp.Nonce)
	}
}

func TestCreatePaymentPayload_EGLD_WithScFunction(t *testing.T) {
	signer := &MockSigner{addr: testSender}
	mockProxy := &MockProxy{nonce: 15}
	scheme, _ := NewExactMultiversXScheme(signer, "multiversx:D", WithProxy(mockProxy))

	req := types.PaymentRequirements{
		PayTo:   testPayTo,
		Amount:  "100",
		Asset:   "EGLD",
		Network: "multiversx:D",
		Extra: map[string]interface{}{
			"scFunction": "buy",
			"arguments":  []string{"01", "02"},
			"relayer":    testSender,
		},
	}

	payload, err := scheme.CreatePaymentPayload(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to create payload: %v", err)
	}

	rpPtr, err := multiversx.PayloadFromMap(payload.Payload)
	if err != nil {
		t.Fatalf("Failed to parse payload: %v", err)
	}
	rp := *rpPtr

	// Should be plain text for native: buy@01@02
	expectedData := "buy@01@02"
	if rp.Data != expectedData {
		t.Errorf("Wrong data: expected %s, got %s", expectedData, rp.Data)
	}
}

func TestCreatePaymentPayload_ESDT(t *testing.T) {
	signer := &MockSigner{addr: testSender}
	mockProxy := &MockProxy{nonce: 20}
	scheme, _ := NewExactMultiversXScheme(signer, "multiversx:D", WithProxy(mockProxy))

	req := types.PaymentRequirements{
		PayTo:   testPayTo,
		Amount:  "100",
		Asset:   testAsset,
		Network: "multiversx:D",
		Extra: map[string]interface{}{
			"relayer": testSender,
		},
	}

	payload, err := scheme.CreatePaymentPayload(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to create payload: %v", err)
	}

	rpPtr, err := multiversx.PayloadFromMap(payload.Payload)
	if err != nil {
		t.Fatalf("Failed to parse payload: %v", err)
	}
	rp := *rpPtr

	// ESDT check: Receiver should be Sender (Self-transfer)
	if rp.Receiver != testSender {
		t.Errorf("ESDT tx receiver should be sender, got %s", rp.Receiver)
	}
	if rp.Value != "0" {
		t.Errorf("ESDT tx value should be 0 EGLD, got %s", rp.Value)
	}
	// Check Nonce
	if rp.Nonce != 20 {
		t.Errorf("Wrong nonce: %d", rp.Nonce)
	}

	// Check Data field contains "MultiESDTNFTTransfer"
	if !strings.HasPrefix(rp.Data, "MultiESDTNFTTransfer") {
		t.Errorf("Data should start with MultiESDTNFTTransfer, got %s", rp.Data)
	}
}

func TestCreatePaymentPayload_ESDT_WithResourceID(t *testing.T) {
	signer := &MockSigner{addr: testSender}
	mockProxy := &MockProxy{nonce: 25}
	scheme, _ := NewExactMultiversXScheme(signer, "multiversx:D", WithProxy(mockProxy))

	req := types.PaymentRequirements{
		PayTo:   testPayTo,
		Amount:  "100",
		Asset:   testAsset,
		Network: "multiversx:D",
		Extra: map[string]interface{}{
			"scFunction": "inv_123",
			"relayer":    testSender,
		},
	}

	payload, err := scheme.CreatePaymentPayload(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to create payload: %v", err)
	}

	rpPtr, err := multiversx.PayloadFromMap(payload.Payload)
	if err != nil {
		t.Fatalf("Failed to parse payload: %v", err)
	}
	rp := *rpPtr

	// Check encoded resource ID "inv_123" -> hex "696e765f313233"
	// Should be at the end
	expectedRidHex := "696e765f313233"
	if !strings.HasSuffix(rp.Data, expectedRidHex) {
		t.Errorf("Data should end with scFunction hex %s, got %s", expectedRidHex, rp.Data)
	}
}

func TestCreatePaymentPayload_EGLD_Alias(t *testing.T) {
	signer := &MockSigner{addr: testSender}
	mockProxy := &MockProxy{nonce: 30}
	scheme, _ := NewExactMultiversXScheme(signer, "multiversx:D", WithProxy(mockProxy))

	req := types.PaymentRequirements{
		PayTo:   testPayTo,
		Amount:  "100",
		Asset:   "EGLD-000000", // Should be treated as EGLD if handled or ESDT token otherwise
		Network: "multiversx:D",
		Extra: map[string]interface{}{
			"relayer": testSender,
		},
	}

	payload, err := scheme.CreatePaymentPayload(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to create payload: %v", err)
	}

	rpPtr, err := multiversx.PayloadFromMap(payload.Payload)
	if err != nil {
		t.Fatalf("Failed to parse payload: %v", err)
	}
	rp := *rpPtr

	// Should be ESDT transfer (MultiESDTNFTTransfer)
	// Because we treat EGLD-000000 as a token identifier for MultiESDT.

	// Value should be 0 (Native EGLD not sent via Value field in MultiESDT usually, unless implied?)
	// Actually, if using EGLD-000000 in MultiESDT, the 'value' of tx is 0, and amount is in data.
	if rp.Value != "0" {
		t.Errorf("Value should be 0 for MultiESDT, got %s", rp.Value)
	}

	if !strings.HasPrefix(rp.Data, "MultiESDTNFTTransfer") {
		t.Errorf("Data should start with MultiESDTNFTTransfer, got %s", rp.Data)
	}

	// Check token hex for EGLD-000000
	// "EGLD-000000" -> 45474c442d303030303030
	tokenHex := "45474c442d303030303030"
	if !strings.Contains(rp.Data, tokenHex) {
		t.Errorf("Data should contain EGLD-000000 hex %s, got %s", tokenHex, rp.Data)
	}
}
