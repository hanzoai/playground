// Package kms provides an HTTP client for Lux KMS MPC wallet operations
// and org-scoped secret management. This is a standalone reimplementation
// of the wire protocol — it does not import luxfi/kms.
package kms

import (
	"fmt"
	"time"
)

// Wallet represents an MPC wallet returned by the KMS daemon.
type Wallet struct {
	ID           string   `json:"id"`
	WalletID     string   `json:"walletId"`
	VaultID      string   `json:"vaultId"`
	Name         *string  `json:"name"`
	KeyType      string   `json:"keyType"`
	Protocol     string   `json:"protocol"`
	ECDSAPubkey  *string  `json:"ecdsaPubkey"`
	EDDSAPubkey  *string  `json:"eddsaPubkey"`
	EthAddress   *string  `json:"ethAddress"`
	BtcAddress   *string  `json:"btcAddress"`
	SolAddress   *string  `json:"solAddress"`
	Threshold    int      `json:"threshold"`
	Participants []string `json:"participants"`
	Version      int      `json:"version"`
	Status       string   `json:"status"`
}

// KeygenRequest is the body sent to POST /api/v1/vaults/{vaultID}/wallets.
type KeygenRequest struct {
	Name     string `json:"name"`
	KeyType  string `json:"key_type"`
	Protocol string `json:"protocol"`
}

// SignResult is the response from a threshold signing operation.
type SignResult struct {
	R         string `json:"r,omitempty"`
	S         string `json:"s,omitempty"`
	Signature string `json:"signature,omitempty"`
}

// ReshareRequest is the body sent to POST /api/v1/wallets/{id}/reshare.
type ReshareRequest struct {
	NewThreshold    int      `json:"new_threshold"`
	NewParticipants []string `json:"new_participants"`
}

// ClusterStatus is the response from GET /api/v1/status.
type ClusterStatus struct {
	NodeID         string `json:"node_id"`
	Mode           string `json:"mode"`
	ExpectedPeers  int    `json:"expected_peers"`
	ConnectedPeers int    `json:"connected_peers"`
	Ready          bool   `json:"ready"`
	Threshold      int    `json:"threshold"`
	Version        string `json:"version"`
}

// SecretMetadata describes a stored secret without exposing its value.
type SecretMetadata struct {
	Key       string    `json:"key"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// APIError represents an error response from the KMS API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("kms api: %d %s", e.StatusCode, e.Message)
}
