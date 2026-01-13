package multiversx

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type NetworkConfig struct {
	APIUrl string
}

type Verifier struct {
	config NetworkConfig
	client *http.Client
}

func NewVerifier(apiUrl string) *Verifier {
	return &Verifier{
		config: NetworkConfig{APIUrl: apiUrl},
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// VerifyPayment checks validity of the payment
func (v *Verifier) VerifyPayment(token string, expectedReceiver string, resourceId string, expectedAmount string, tokenIdentifier string) (bool, error) {
	// 1. Fetch Transaction
	resp, err := v.client.Get(fmt.Sprintf("%s/transactions/%s", v.config.APIUrl, token))
	if err != nil {
		return false, fmt.Errorf("failed to fetch tx: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("tx not found or api error: %d", resp.StatusCode)
	}

	var tx TransactionAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&tx); err != nil {
		return false, fmt.Errorf("failed to decode tx: %v", err)
	}

	// 2. Status Check
	if tx.Status != "success" {
		return false, fmt.Errorf("transaction not successful: status=%s", tx.Status)
	}

	// 3. Replay Protection / Resource ID Check
	resourceIdHex := hex.EncodeToString([]byte(resourceId))
	foundResource := false
	isCorrectReceiver := false

	// Check Receiver
	// Case A: Direct
	if tx.Receiver == expectedReceiver {
		isCorrectReceiver = true
	}
	// Case B: Action Receiver (ESDT)
	if !isCorrectReceiver && tx.Action.Receiver == expectedReceiver {
		isCorrectReceiver = true
	}

	// Check Resource ID in Data (Naive V1 check)
	// Ideally we scan Logs for the Payment Event, but Data check is sufficient if the SC execution was successful (Status=success)
	// and the SC enforces the input data structure.
	if strings.Contains(tx.Data, resourceIdHex) {
		foundResource = true
	}
	// Also check event logs just in case
	for _, evt := range tx.Logs.Events {
		if strings.Contains(evt.Data, resourceIdHex) {
			foundResource = true
		}
	}

	if !isCorrectReceiver {
		return false, errors.New("invalid receiver")
	}
	if !foundResource {
		return false, errors.New("resource_id mismatch")
	}

	// 4. Value Check
	if tokenIdentifier == "EGLD" {
		if tx.Value != expectedAmount {
			return false, fmt.Errorf("insufficient value: %s != %s", tx.Value, expectedAmount)
		}
	} else {
		// ESDT Check
		validTokenTransfer := false
		for _, transfer := range tx.Action.Arguments.Transfers {
			if transfer.Identifier == tokenIdentifier && transfer.Value == expectedAmount {
				validTokenTransfer = true
			}
		}
		if !validTokenTransfer {
			return false, errors.New("insufficient esdt value")
		}
	}

	return true, nil
}
