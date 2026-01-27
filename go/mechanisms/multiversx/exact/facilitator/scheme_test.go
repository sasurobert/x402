package facilitator

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"crypto/ed25519"

	"github.com/multiversx/mx-chain-core-go/data/api"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-sdk-go/blockchain"
	"github.com/multiversx/mx-sdk-go/core"
	"github.com/multiversx/mx-sdk-go/data"

	"github.com/coinbase/x402/go/mechanisms/multiversx"
	"github.com/coinbase/x402/go/types"
)

// MockSigner implements FacilitatorMultiversXSigner
type MockSigner struct{}

func (s *MockSigner) GetAddresses() []string {
	return []string{"erd1test"}
}
func (s *MockSigner) Sign(ctx context.Context, tx *transaction.FrontendTransaction) (string, error) {
	return "mock_signature", nil
}
func (s *MockSigner) SendTransaction(ctx context.Context, tx *transaction.FrontendTransaction) (string, error) {
	return "mock_tx_hash", nil
}
func (s *MockSigner) GetAccount(ctx context.Context, address string) (*data.Account, error) {
	return &data.Account{}, nil
}
func (s *MockSigner) GetTransactionStatus(ctx context.Context, txHash string) (string, error) {
	return "success", nil
}

// Keys
func TestVerify_EGLD_Direct_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"result":{"status":"success","hash":"sim_hash"}},"error":""}`))
	}))
	defer server.Close()

	scheme, _ := NewExactMultiversXScheme(server.URL, &MockSigner{})

	pubKey, privKey, _ := ed25519.GenerateKey(nil)
	senderAddr, _ := data.NewAddressFromBytes(pubKey).AddressAsBech32String()

	payload := multiversx.ExactRelayedPayload{
		Nonce:       10,
		Value:       "1000",
		Receiver:    senderAddr,
		Sender:      senderAddr,
		GasPrice:    1000000000,
		GasLimit:    50000,
		Data:        "",
		ChainID:     "D",
		Version:     1,
		Options:     0,
		ValidAfter:  uint64(time.Now().Unix() - 100),
		ValidBefore: uint64(time.Now().Unix() + 3600),
	}

	tx := payload.ToTransaction()
	txBytes, _ := multiversx.SerializeTransaction(&tx)
	sig := ed25519.Sign(privKey, txBytes)
	payload.Signature = hex.EncodeToString(sig)

	pBytes, _ := json.Marshal(payload)
	var pMap map[string]interface{}
	json.Unmarshal(pBytes, &pMap)

	req := types.PaymentRequirements{
		PayTo:  senderAddr,
		Amount: "1000",
		Asset:  multiversx.NativeTokenTicker,
		Extra: map[string]interface{}{
			"assetTransferMethod": multiversx.TransferMethodDirect,
		},
	}

	resp, err := scheme.Verify(context.Background(), types.PaymentPayload{Payload: pMap}, req)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !resp.IsValid {
		t.Error("Expected valid")
	}
}

func TestVerify_AssetMismatch(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()
	scheme, _ := NewExactMultiversXScheme(server.URL, &MockSigner{})

	pubKey, privKey, _ := ed25519.GenerateKey(nil)
	// Create Sender Address
	senderAddr, _ := data.NewAddressFromBytes(pubKey).AddressAsBech32String()

	payload := multiversx.ExactRelayedPayload{
		Nonce:    1,
		Value:    "1000",
		Receiver: senderAddr,
		Sender:   senderAddr,
		GasPrice: 1000000000,
		GasLimit: 50000,
		ChainID:  "D",
		Version:  1,
	}
	tx := payload.ToTransaction()
	txBytes, _ := multiversx.SerializeTransaction(&tx)
	sig := ed25519.Sign(privKey, txBytes)
	payload.Signature = hex.EncodeToString(sig)

	pBytes, _ := json.Marshal(payload)
	var pMap map[string]interface{}
	json.Unmarshal(pBytes, &pMap)

	// Req expects wrong amount
	req := types.PaymentRequirements{
		PayTo:  senderAddr,
		Amount: "2000",
		Asset:  multiversx.NativeTokenTicker,
		Extra: map[string]interface{}{
			"assetTransferMethod": multiversx.TransferMethodDirect,
		},
	}

	resp, err := scheme.Verify(context.Background(), types.PaymentPayload{Payload: pMap}, req)
	if err == nil {
		t.Fatal("Expected mismatch error")
	}
	if resp != nil {
		t.Errorf("Expected nil resp on mismatch error")
	}
}

// MockProxy implements ProxyWithStatus
type MockProxy struct {
	blockchain.Proxy
	statusResponses []transaction.TxStatus
	statusIndex     int
	sendHash        string
	sendErr         error
}

func (m *MockProxy) SendTransaction(ctx context.Context, tx *transaction.FrontendTransaction) (string, error) {
	return m.sendHash, m.sendErr
}

func (m *MockProxy) GetTransactionStatus(ctx context.Context, txHash string) (string, error) {
	if m.statusIndex < len(m.statusResponses) {
		s := m.statusResponses[m.statusIndex]
		m.statusIndex++
		return string(s), nil
	}
	if len(m.statusResponses) > 0 {
		return string(m.statusResponses[len(m.statusResponses)-1]), nil
	}
	return string(transaction.TxStatusPending), nil
}

func (m *MockProxy) IsInterfaceNil() bool {
	return m == nil
}

// Helpers required by Proxy interface (stubs)
func (m *MockProxy) GetNetworkConfig(ctx context.Context) (*data.NetworkConfig, error) {
	return nil, nil
}
func (m *MockProxy) GetAccount(ctx context.Context, address core.AddressHandler) (*data.Account, error) {
	return nil, nil
}
func (m *MockProxy) GetGuardianData(ctx context.Context, address core.AddressHandler) (*api.GuardianData, error) {
	return nil, nil
}
func (m *MockProxy) ExecuteVMQuery(ctx context.Context, vmRequest *data.VmValueRequest) (*data.VmValuesResponseData, error) {
	return nil, nil
}
func (m *MockProxy) FilterLogs(ctx context.Context, filter *core.FilterQuery) ([]*transaction.Events, error) {
	return nil, nil
}

func (m *MockProxy) GetTransactionInfo(ctx context.Context, hash string) (*data.TransactionInfo, error) {
	return &data.TransactionInfo{}, nil
}

func (m *MockProxy) GetTransactionInfoWithResults(ctx context.Context, hash string) (*data.TransactionInfo, error) {
	return &data.TransactionInfo{}, nil
}

func TestSettle_Success(t *testing.T) {
	mockProxy := &MockProxy{
		sendHash:        "tx_hash_123",
		statusResponses: []transaction.TxStatus{transaction.TxStatusSuccess},
	}
	scheme := &ExactMultiversXScheme{
		proxy: mockProxy,
	}

	payload := types.PaymentPayload{
		Payload: map[string]interface{}{
			"nonce":    uint64(10),
			"value":    "1000",
			"receiver": "erd1...",
			"sender":   "erd1...",
			"chainID":  "D",
		},
	}

	resp, err := scheme.Settle(context.Background(), payload, types.PaymentRequirements{})
	if err != nil {
		t.Fatalf("Settle failed: %v", err)
	}
	if !resp.Success {
		t.Error("Expected success")
	}
	if resp.Transaction != "tx_hash_123" {
		t.Errorf("Expected hash tx_hash_123, got %s", resp.Transaction)
	}
}

func TestSettle_Failure(t *testing.T) {
	mockProxy := &MockProxy{
		sendHash:        "tx_hash_456",
		statusResponses: []transaction.TxStatus{transaction.TxStatusFail},
	}
	scheme := &ExactMultiversXScheme{
		proxy: mockProxy,
	}

	payload := types.PaymentPayload{
		Payload: map[string]interface{}{},
	}

	_, err := scheme.Settle(context.Background(), payload, types.PaymentRequirements{})
	if err == nil {
		t.Fatal("Expected error on tx failure")
	}
}

func TestSettle_Polling(t *testing.T) {
	// Mock returns Pending once, then Success
	mockProxy := &MockProxy{
		sendHash: "tx_hash_polling",
		statusResponses: []transaction.TxStatus{
			transaction.TxStatusPending,
			transaction.TxStatusSuccess,
		},
	}
	scheme := &ExactMultiversXScheme{
		proxy: mockProxy,
	}

	payload := types.PaymentPayload{
		Payload: map[string]interface{}{},
	}

	// Override ticker/delay if necessary? The implementation uses 2s ticker.
	// We can't easily override it without changing the code, but we can verify it eventually returns.
	// Since we are in a unit test, we should be careful about time.

	resp, err := scheme.Settle(context.Background(), payload, types.PaymentRequirements{})
	if err != nil {
		t.Fatalf("Settle failed: %v", err)
	}
	if !resp.Success {
		t.Error("Expected success")
	}
	if mockProxy.statusIndex != 2 {
		t.Errorf("Expected 2 status checks, got %d", mockProxy.statusIndex)
	}
}
