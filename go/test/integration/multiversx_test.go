package integration_test

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	x402 "github.com/coinbase/x402/go"
	"github.com/coinbase/x402/go/mechanisms/multiversx"
	"github.com/coinbase/x402/go/mechanisms/multiversx/exact/client"
	"github.com/coinbase/x402/go/mechanisms/multiversx/exact/facilitator"
	"github.com/coinbase/x402/go/mechanisms/multiversx/exact/server"
	mxsigners "github.com/coinbase/x402/go/signers/multiversx"
	"github.com/coinbase/x402/go/types"
	"github.com/multiversx/mx-chain-core-go/data/transaction"
	"github.com/multiversx/mx-sdk-go/blockchain"
	"github.com/multiversx/mx-sdk-go/core"
	"github.com/multiversx/mx-sdk-go/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real Test Signer using Alice's Devnet Key
type RealSigner struct {
	privKey ed25519.PrivateKey
	address string
}

func NewRealSigner(privKeyHex string) (*RealSigner, error) {
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return nil, err
	}
	if len(privKeyBytes) != 32 {
		return nil, fmt.Errorf("invalid keys size: %d (expected 32 bytes)", len(privKeyBytes))
	}
	// Ed25519 private key from seed
	privKey := ed25519.NewKeyFromSeed(privKeyBytes)

	// Derive Address from Public Key
	pubKey := privKey.Public().(ed25519.PublicKey)
	addressHandler := data.NewAddressFromBytes(pubKey)
	address, err := addressHandler.AddressAsBech32String()
	if err != nil {
		return nil, fmt.Errorf("failed to encode address: %v", err)
	}

	return &RealSigner{
		privKey: privKey,
		address: address,
	}, nil
}

func (s *RealSigner) Address() string {
	return s.address
}

func (s *RealSigner) Sign(ctx context.Context, message []byte) ([]byte, error) {
	return ed25519.Sign(s.privKey, message), nil
}

func (s *RealSigner) PrivateKey() []byte {
	return s.privKey
}

// realFacilitatorMultiversXSigner implements multiversx.FacilitatorMultiversXSigner
type realFacilitatorMultiversXSigner struct {
	privKey ed25519.PrivateKey
	address string
	proxy   blockchain.Proxy
}

func newRealFacilitatorMultiversXSigner(privKeyHex string, apiUrl string) (*realFacilitatorMultiversXSigner, error) {
	privKeyBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		return nil, err
	}
	privKey := ed25519.NewKeyFromSeed(privKeyBytes)
	pubKey := privKey.Public().(ed25519.PublicKey)
	address, _ := data.NewAddressFromBytes(pubKey).AddressAsBech32String()

	args := blockchain.ArgsProxy{
		ProxyURL:            apiUrl,
		Client:              nil,
		SameScState:         false,
		ShouldBeSynced:      false,
		FinalityCheck:       false,
		EntityType:          core.Proxy,
		CacheExpirationTime: time.Second,
	}
	proxy, err := blockchain.NewProxy(args)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy: %w", err)
	}

	return &realFacilitatorMultiversXSigner{
		privKey: privKey,
		address: address,
		proxy:   proxy,
	}, nil
}

func (s *realFacilitatorMultiversXSigner) GetAddresses() []string {
	return []string{s.address}
}

func (s *realFacilitatorMultiversXSigner) Sign(ctx context.Context, tx *transaction.FrontendTransaction) (string, error) {
	cryptoHolder, err := multiversx.NewSimpleCryptoHolderFromBytes(s.privKey)
	if err != nil {
		return "", fmt.Errorf("failed to create crypto holder: %w", err)
	}

	// Sign as Relayer (since this is the facilitator signer)
	// If tx.RelayerAddr is set, we assume we are signing as relayer?
	// The user flow uses s.signer (user).
	// Facilitator flow uses s (relayer).
	// We should check if we are acting as relayer.
	// For RelayedV3, facilitator IS the relayer.
	asRelayer := tx.RelayerAddr != "" // If relayer holds address, we check if it matches?
	// But in test, we want to simulate the facilitator signing as relayer for RelayedV3.
	// Actually, the facilitator interface is generic.
	// For RelayedV3, the facilitator calls Sign on the transaction that already has user signature.
	// So we should sign as relayer.
	if tx.Version >= 2 && tx.RelayerAddr != "" {
		asRelayer = true
	}

	err = multiversx.SignTransactionWithBuilder(cryptoHolder, tx, asRelayer)
	if err != nil {
		return "", err
	}

	if asRelayer {
		return tx.RelayerSignature, nil
	}
	return tx.Signature, nil
}

func (s *realFacilitatorMultiversXSigner) SendTransaction(ctx context.Context, tx *transaction.FrontendTransaction) (string, error) {
	hash, err := s.proxy.SendTransaction(ctx, tx)
	if err != nil {
		fmt.Printf("TEST DEBUG: SendTransaction Error: %v\n", err)
	}
	fmt.Printf("TEST DEBUG: SendTransaction Hash: %s\n", hash)
	return hash, err
}

func (s *realFacilitatorMultiversXSigner) GetAccount(ctx context.Context, address string) (*data.Account, error) {
	addrObj, _ := data.NewAddressFromBech32String(address)
	return s.proxy.GetAccount(ctx, addrObj)
}

func (s *realFacilitatorMultiversXSigner) GetTransactionStatus(ctx context.Context, txHash string) (string, error) {
	// Status check would go here
	return "success", nil
}

var _ multiversx.FacilitatorMultiversXSigner = (*realFacilitatorMultiversXSigner)(nil)

func TestIntegration_AliceFlow(t *testing.T) {
	// 1. Setup Alice Signer (Standard Devnet Alice)
	// Secret Key: 413f42575f7f26fad3317a778771212fdb80245850981e48b58a4f25e344e8f9
	aliceSK := "413f42575f7f26fad3317a778771212fdb80245850981e48b58a4f25e344e8f9"
	signer, err := NewRealSigner(aliceSK)
	if err != nil {
		t.Fatalf("Failed to create Alice signer: %v", err)
	}

	// 2. Setup Components
	ctx := context.Background()
	cScheme, err := client.NewExactMultiversXScheme(signer, "multiversx:D")
	if err != nil {
		t.Fatalf("Failed to create client scheme: %v", err)
	}

	devnetURL := multiversx.GetAPIURL(multiversx.ChainIDDevnet)
	fScheme, _ := facilitator.NewExactMultiversXScheme(devnetURL, nil)
	sScheme := server.NewExactMultiversXScheme() // Server

	// Fetch Real Nonce for Alice
	// Use SDK Proxy directly
	args := blockchain.ArgsProxy{
		ProxyURL:            devnetURL,
		Client:              nil,
		SameScState:         false,
		ShouldBeSynced:      false,
		FinalityCheck:       false,
		EntityType:          core.Proxy,
		CacheExpirationTime: time.Second,
	}
	proxy, err := blockchain.NewProxy(args)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}
	// Need sender address handler
	senderAddrStruct, _ := data.NewAddressFromBech32String(signer.Address())
	account, err := proxy.GetAccount(ctx, senderAddrStruct)

	realNonce := uint64(100)
	if err != nil {
		t.Logf("Failed to fetch real Alice nonce, using fallback 100: %v", err)
	} else {
		realNonce = account.Nonce
		t.Logf("Fetched real Alice nonce: %d", realNonce)
	}

	// 3. Define Base Requirements (Server Side Input)
	bobAddr := "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx"
	baseReq := types.PaymentRequirements{
		PayTo:             bobAddr,
		Amount:            "1000000000000000000", // 1 EGLD
		Asset:             multiversx.NativeTokenTicker,
		Network:           "multiversx:D",
		MaxTimeoutSeconds: 3600,
		Extra: map[string]interface{}{
			// Server might add these or validate them.
			// Client usually adds Nonce, but here we simulate full context prep
			"resourceId": "test-resource-alice",
			"nonce":      realNonce,
			"gasLimit":   1000000,
			"relayer":    signer.Address(),
		},
	}

	// 4. Server Enhances Requirements
	// This simulates the "Instruction" phase where Server preps the requirements for Client
	enhancedReq, err := sScheme.EnhancePaymentRequirements(ctx, baseReq, types.SupportedKind{}, nil)
	if err != nil {
		t.Fatalf("Server failed to enhance requirements: %v", err)
	}

	// Server Validate (Double check)
	if err := sScheme.ValidatePaymentRequirements(enhancedReq); err != nil {
		t.Fatalf("Server validation failed: %v", err)
	}

	// 5. Create Payload (Client) implements these requirements
	payload, err := cScheme.CreatePaymentPayload(ctx, enhancedReq)
	if err != nil {
		t.Fatalf("Client failed to create payload: %v", err)
	}

	// 6. Verify Payload (Facilitator) checks against Requirements
	// This will hit Devnet API /transaction/simulate
	resp, err := fScheme.Verify(ctx, payload, enhancedReq)
	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	} else {
		if !resp.IsValid {
			t.Fatalf("Verification returned invalid (check logs for details)")
		} else {
			t.Log("Verification Successful via Devnet Simulation!")
		}
	}
}

// Helper struct for Simulation Mock
type SimulationResponse struct {
	Data struct {
		Result struct {
			Status string `json:"status"`
			Hash   string `json:"hash"`
		} `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
}

func TestFacilitatorVerify_ESDT_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SimulationResponse{}
		resp.Data.Result.Status = "success"
		resp.Data.Result.Hash = "mock_esdt_hash"
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	scheme, _ := facilitator.NewExactMultiversXScheme(server.URL, nil)

	// Generate Keys
	privKey := ed25519.NewKeyFromSeed([]byte("01234567890123456789012345678901")) // 32 bytes seed
	pubKey := privKey.Public().(ed25519.PublicKey)
	senderAddr := data.NewAddressFromBytes(pubKey)
	senderBech32, _ := senderAddr.AddressAsBech32String()

	// Data: "MultiESDTNFTTransfer@<receiver_hex>@01@<token_hex>@00@<amount_hex>"
	// We must use the generated sender as the mocked payload sender to match signature
	// We'll effectively send to self for simplicity or just use the generated addr as "Bob"
	payTo := senderBech32
	payToHex := hex.EncodeToString(senderAddr.AddressBytes())

	tokenHex := hex.EncodeToString([]byte("USDC-123"))
	amountHex := "64"
	dataString := fmt.Sprintf("MultiESDTNFTTransfer@%s@01@%s@00@%s", payToHex, tokenHex, amountHex)

	rp := multiversx.ExactRelayedPayload{}
	rp.Data = dataString
	rp.Value = "0"
	rp.Receiver = payTo
	rp.Sender = payTo
	rp.ChainID = "D"
	rp.Version = 1

	// Sign
	tx := rp.ToTransaction()
	txBytes, _ := multiversx.SerializeTransaction(&tx)
	sig := ed25519.Sign(privKey, txBytes)
	rp.Signature = hex.EncodeToString(sig)

	payloadBytes, _ := json.Marshal(rp)
	var rpMap map[string]interface{}
	json.Unmarshal(payloadBytes, &rpMap)

	paymentPayload := types.PaymentPayload{
		Payload: rpMap,
	}

	req := types.PaymentRequirements{
		PayTo:  payTo, // Bech32
		Amount: "100",
		Asset:  "USDC-123",
		Extra: map[string]interface{}{
			"assetTransferMethod": multiversx.TransferMethodESDT,
		},
	}

	resp, err := scheme.Verify(context.Background(), paymentPayload, req)
	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}
	if !resp.IsValid {
		t.Error("IsValid should be true")
	}
}

func TestFacilitatorVerify_EGLD_Alias_MultiESDT(t *testing.T) {
	// Verify that EGLD-000000 via MultiESDT payload is accepted
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := SimulationResponse{}
		resp.Data.Result.Status = "success"
		resp.Data.Result.Hash = "mock_egld_alias_hash"
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	scheme, _ := facilitator.NewExactMultiversXScheme(server.URL, nil)

	// Generate Keys
	privKey := ed25519.NewKeyFromSeed([]byte("01234567890123456789012345678901"))
	pubKey := privKey.Public().(ed25519.PublicKey)
	senderAddr := data.NewAddressFromBytes(pubKey)
	senderBech32, _ := senderAddr.AddressAsBech32String()

	payTo := senderBech32
	payToHex := hex.EncodeToString(senderAddr.AddressBytes())

	// Token: EGLD-000000
	tokenHex := hex.EncodeToString([]byte("EGLD-000000"))
	amountHex := "64" // 100

	dataString := fmt.Sprintf("MultiESDTNFTTransfer@%s@01@%s@00@%s", payToHex, tokenHex, amountHex)

	rp := multiversx.ExactRelayedPayload{}
	rp.Data = dataString
	rp.Value = "0"
	rp.Receiver = payTo
	rp.Sender = payTo
	rp.ChainID = "D"
	rp.Version = 1

	// Sign
	tx := rp.ToTransaction()
	txBytes, _ := multiversx.SerializeTransaction(&tx)
	sig := ed25519.Sign(privKey, txBytes)
	rp.Signature = hex.EncodeToString(sig)

	payloadBytes, _ := json.Marshal(rp)
	var rpMap map[string]interface{}
	json.Unmarshal(payloadBytes, &rpMap)

	paymentPayload := types.PaymentPayload{
		Payload: rpMap,
	}

	req := types.PaymentRequirements{
		PayTo:  payTo, // Bech32
		Amount: "100",
		Asset:  "EGLD-000000",
		Extra: map[string]interface{}{
			"assetTransferMethod": multiversx.TransferMethodESDT,
		},
	}

	resp, err := scheme.Verify(context.Background(), paymentPayload, req)
	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}
	if !resp.IsValid {
		t.Error("IsValid should be true")
	}
}

// localMultiversXFacilitatorClient for testing
type localMultiversXFacilitatorClient struct {
	facilitator *x402.X402Facilitator
}

func (l *localMultiversXFacilitatorClient) Verify(ctx context.Context, payloadBytes []byte, requirementsBytes []byte) (*x402.VerifyResponse, error) {
	return l.facilitator.Verify(ctx, payloadBytes, requirementsBytes)
}

func (l *localMultiversXFacilitatorClient) Settle(ctx context.Context, payloadBytes []byte, requirementsBytes []byte) (*x402.SettleResponse, error) {
	return l.facilitator.Settle(ctx, payloadBytes, requirementsBytes)
}

func (l *localMultiversXFacilitatorClient) GetSupported(ctx context.Context) (x402.SupportedResponse, error) {
	return l.facilitator.GetSupported(), nil
}

func TestMultiversXIntegrationV2(t *testing.T) {
	// Skip if environment variables not set
	aliceSK := "413f42575f7f26fad3317a778771212fdb80245850981e48b58a4f25e344e8f9"
	bobAddr := "erd1spyavw0956vq68xj8y4tenjpq2wd5a9p2c6j8gsz7ztyrnpxrruqzu66jx"
	apiUrl := multiversx.GetAPIURL(multiversx.ChainIDDevnet)

	signer, _ := NewRealSigner(aliceSK)
	aliceAddr := signer.Address()

	// Proxy setup shared
	args := blockchain.ArgsProxy{
		ProxyURL:            apiUrl,
		Client:              nil,
		SameScState:         false,
		ShouldBeSynced:      false,
		FinalityCheck:       false,
		EntityType:          core.Proxy,
		CacheExpirationTime: time.Second,
	}
	proxy, err := blockchain.NewProxy(args)
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}
	senderAddrStruct, _ := data.NewAddressFromBech32String(aliceAddr)

	t.Run("Full V2 Flow", func(t *testing.T) {
		ctx := context.Background()

		// 1. Setup Client
		clientSigner, _ := mxsigners.NewClientSignerFromPrivateKey(aliceSK)
		clientScheme, _ := client.NewExactMultiversXScheme(clientSigner, "multiversx:D")
		x402Client := x402.Newx402Client()
		x402Client.Register("multiversx:D", clientScheme)

		// 2. Setup Facilitator
		facilitatorSigner, _ := newRealFacilitatorMultiversXSigner(aliceSK, apiUrl)
		x402Facilitator := x402.Newx402Facilitator()
		fScheme, _ := facilitator.NewExactMultiversXScheme(apiUrl, facilitatorSigner)
		x402Facilitator.Register([]x402.Network{"multiversx:D"}, fScheme)

		// 3. Setup Resource Server
		facilitatorClient := &localMultiversXFacilitatorClient{facilitator: x402Facilitator}

		sScheme := server.NewExactMultiversXScheme()
		x402Server := x402.Newx402ResourceServer(
			x402.WithFacilitatorClient(facilitatorClient),
		)
		x402Server.Register("multiversx:D", sScheme)

		err := x402Server.Initialize(ctx)
		require.NoError(t, err)

		// Refresh Nonce
		account, _ := proxy.GetAccount(ctx, senderAddrStruct)
		currentNonce := account.Nonce

		// 4. Server - Create Requirement
		accepts := []types.PaymentRequirements{
			{
				Scheme:  multiversx.SchemeExact,
				Network: "multiversx:D",
				Asset:   multiversx.NativeTokenTicker,
				Amount:  "1000",
				PayTo:   bobAddr,
				Extra: map[string]interface{}{
					"nonce":    currentNonce,
					"relayer":  aliceAddr,
					"gasLimit": 250000,
				},
			},
		}
		resource := &types.ResourceInfo{URL: "https://api.example.com", Description: "Test"}
		paymentRequired := x402Server.CreatePaymentRequiredResponse(accepts, resource, "", nil)

		// 5. Client - Create Payload
		selected, err := x402Client.SelectPaymentRequirements(paymentRequired.Accepts)
		require.NoError(t, err)
		payload, err := x402Client.CreatePaymentPayload(ctx, selected, paymentRequired.Resource, paymentRequired.Extensions)
		require.NoError(t, err)

		// 6. Server - Process Payment
		matching := x402Server.FindMatchingRequirements(accepts, payload)
		require.NotNil(t, matching)

		verifyResp, err := x402Server.VerifyPayment(ctx, payload, *matching)
		require.NoError(t, err)
		assert.True(t, verifyResp.IsValid)

		// 7. Settle
		settleResp, err := x402Server.SettlePayment(ctx, payload, *matching)
		require.NoError(t, err)
		assert.True(t, settleResp.Success)
		assert.NotEmpty(t, settleResp.Transaction)
	})

	t.Run("Full V2 Flow - ESDT", func(t *testing.T) {
		t.Skip("Skipping ESDT integration test due to missing funds (WEGLD/USDC) on Devnet Alice wallet")
		ctx := context.Background()

		// 1. Setup Client
		clientSigner, _ := mxsigners.NewClientSignerFromPrivateKey(aliceSK)
		clientScheme, _ := client.NewExactMultiversXScheme(clientSigner, "multiversx:D")
		x402Client := x402.Newx402Client()
		x402Client.Register("multiversx:D", clientScheme)

		// 2. Setup Facilitator
		facilitatorSigner, _ := newRealFacilitatorMultiversXSigner(aliceSK, apiUrl)
		x402Facilitator := x402.Newx402Facilitator()
		fScheme, _ := facilitator.NewExactMultiversXScheme(apiUrl, facilitatorSigner)
		x402Facilitator.Register([]x402.Network{"multiversx:D"}, fScheme)

		// 3. Setup Resource Server
		facilitatorClient := &localMultiversXFacilitatorClient{facilitator: x402Facilitator}

		sScheme := server.NewExactMultiversXScheme()
		x402Server := x402.Newx402ResourceServer(
			x402.WithFacilitatorClient(facilitatorClient),
		)
		x402Server.Register("multiversx:D", sScheme)

		err := x402Server.Initialize(ctx)
		require.NoError(t, err)

		// Refresh Nonce
		account, _ := proxy.GetAccount(ctx, senderAddrStruct)
		currentNonce := account.Nonce

		// 4. Server - Create Requirement (ESDT)
		tokenID := "WEGLD-bd4d79"
		accepts := []types.PaymentRequirements{
			{
				Scheme:            multiversx.SchemeExact,
				Network:           "multiversx:D",
				Asset:             tokenID,
				Amount:            "1000",
				PayTo:             bobAddr,
				MaxTimeoutSeconds: 3600,
				Extra: map[string]interface{}{
					"assetTransferMethod": multiversx.TransferMethodESDT,
					"relayer":             aliceAddr,
					"gasLimit":            60000000 + 100000,
					"nonce":               currentNonce,
				},
			},
		}
		resource := &types.ResourceInfo{URL: "https://api.example.com", Description: "Test ESDT"}
		paymentRequired := x402Server.CreatePaymentRequiredResponse(accepts, resource, "", nil)

		// 5. Client - Create Payload
		selected, err := x402Client.SelectPaymentRequirements(paymentRequired.Accepts)
		require.NoError(t, err)
		payload, err := x402Client.CreatePaymentPayload(ctx, selected, paymentRequired.Resource, paymentRequired.Extensions)
		require.NoError(t, err)

		// 6. Server - Process Payment
		matching := x402Server.FindMatchingRequirements(accepts, payload)
		require.NotNil(t, matching)

		verifyResp, err := x402Server.VerifyPayment(ctx, payload, *matching)
		require.NoError(t, err)
		assert.True(t, verifyResp.IsValid)

		// 7. Settle
		settleResp, err := x402Server.SettlePayment(ctx, payload, *matching)
		require.NoError(t, err)
		assert.True(t, settleResp.Success)
		assert.NotEmpty(t, settleResp.Transaction)
	})
}
