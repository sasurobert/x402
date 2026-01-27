package multiversx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientSigner(t *testing.T) {
	// Alice's Devnet Key (Seed)
	aliceSK := "413f42575f7f26fad3317a778771212fdb80245850981e48b58a4f25e344e8f9"
	expectedAddress := "erd1qyu5wthldzr8wx5c9ucg8kjagg0jfs53s8nr3zpz3hypefsdd8ssycr6th" // Address for this specific seed

	t.Run("NewClientSignerFromPrivateKey", func(t *testing.T) {
		signer, err := NewClientSignerFromPrivateKey(aliceSK)
		require.NoError(t, err)
		assert.Equal(t, expectedAddress, signer.Address())
	})

	t.Run("Invalid Key", func(t *testing.T) {
		_, err := NewClientSignerFromPrivateKey("invalid")
		assert.Error(t, err)

		_, err = NewClientSignerFromPrivateKey("1234") // too short
		assert.Error(t, err)
	})

	t.Run("Sign", func(t *testing.T) {
		signer, err := NewClientSignerFromPrivateKey(aliceSK)
		require.NoError(t, err)

		message := []byte("hello world")
		signature, err := signer.Sign(context.Background(), message)
		require.NoError(t, err)
		assert.NotEmpty(t, signature)
		assert.Equal(t, 64, len(signature))
	})
}
