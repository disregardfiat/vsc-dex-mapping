package oracle

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/hasura/go-graphql-client"
	"github.com/vsc-eco/hivego"
)

// Service handles Bitcoin header submission to VSC btc-mapping contract
type Service struct {
	btcClient *rpcclient.Client
	vscClient *hivego.HiveRpc
	vscConfig VSCConfig
}

type VSCConfig struct {
	Endpoint string
	Key      string
	Username string
}

// NewService creates a new oracle service
func NewService(btcConfig *rpcclient.ConnConfig, vscConfig VSCConfig) (*Service, error) {
	btcClient, err := rpcclient.New(btcConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create BTC client: %w", err)
	}

	vscClient := hivego.NewHiveRpc(vscConfig.Endpoint)

	return &Service{
		btcClient: btcClient,
		vscClient: vscClient,
		vscConfig: vscConfig,
	}, nil
}

// SubmitHeaders fetches new Bitcoin headers and submits them to the VSC contract
func (s *Service) SubmitHeaders(ctx context.Context) error {
	// Get latest block count
	latestHeight, err := s.btcClient.GetBlockCount()
	if err != nil {
		return fmt.Errorf("failed to get block count: %w", err)
	}

	// Get current contract tip from VSC
	contractTip := s.getContractTip(ctx) // TODO: implement

	// Submit headers from contractTip+1 to latestHeight-6 (confirmations)
	startHeight := contractTip + 1
	endHeight := latestHeight - 6 // Require 6 confirmations

	if startHeight > endHeight {
		log.Printf("No new headers to submit (start: %d, end: %d)", startHeight, endHeight)
		return nil
	}

	headers, err := s.fetchHeaders(startHeight, endHeight)
	if err != nil {
		return fmt.Errorf("failed to fetch headers: %w", err)
	}

	// Submit to contract
	return s.submitHeadersToContract(ctx, headers)
}

// fetchHeaders retrieves block headers from Bitcoin node
func (s *Service) fetchHeaders(startHeight, endHeight int64) ([]*wire.BlockHeader, error) {
	headers := make([]*wire.BlockHeader, 0, endHeight-startHeight+1)

	for height := startHeight; height <= endHeight; height++ {
		hash, err := s.btcClient.GetBlockHash(height)
		if err != nil {
			return nil, fmt.Errorf("failed to get block hash at height %d: %w", height, err)
		}

		header, err := s.btcClient.GetBlockHeader(hash)
		if err != nil {
			return nil, fmt.Errorf("failed to get block header for hash %s: %w", hash.String(), err)
		}

		headers = append(headers, header)
	}

	return headers, nil
}

// submitHeadersToContract submits headers to the VSC btc-mapping contract
func (s *Service) submitHeadersToContract(ctx context.Context, headers []*wire.BlockHeader) error {
	// Serialize headers
	var buf bytes.Buffer
	for _, header := range headers {
		if err := header.Serialize(&buf); err != nil {
			return fmt.Errorf("failed to serialize header: %w", err)
		}
	}

	headerBytes := buf.Bytes()

	// Call contract via VSC GraphQL mutation
	contractCall := fmt.Sprintf(`{
		"contract": "%s",
		"method": "submitHeaders",
		"args": {
			"headers": "%x"
		}
	}`, "btc-mapping-contract", headerBytes)

	// Broadcast transaction
	// TODO: Implement actual VSC transaction broadcasting
	log.Printf("Submitting %d headers to btc-mapping contract", len(headers))

	// For now, simulate success
	return nil
}

// getContractTip retrieves the current tip height from the btc-mapping contract
func (s *Service) getContractTip(ctx context.Context) int64 {
	// Query contract state via GraphQL
	// TODO: Implement actual GraphQL query to btc-mapping contract

	// For now, simulate a GraphQL query to get contract state
	// In production, this would query the VSC GraphQL API for contract state
	query := `
		query GetContractTip($contractId: String!) {
			contract(id: $contractId) {
				state
			}
		}
	`

	// Mock contract state response
	// In production, this would parse the actual contract state
	mockContractState := map[string]interface{}{
		"tipHeight": uint32(800000), // Mock current tip
	}

	if tipHeight, ok := mockContractState["tipHeight"].(uint32); ok {
		return int64(tipHeight)
	}

	// Fallback to reasonable default
	return 800000
}

// VerifyDepositProof verifies a Bitcoin deposit proof and submits to contract
func (s *Service) VerifyDepositProof(ctx context.Context, proof []byte) error {
	if len(proof) < 44 {
		return fmt.Errorf("invalid proof length")
	}

	// Parse proof: [txid(32)][vout(4)][amount(8)][block_header(80)]
	txid := proof[0:32]
	vout := uint32(proof[32]) | uint32(proof[33])<<8 | uint32(proof[34])<<16 | uint32(proof[35])<<24
	amount := uint64(proof[36]) | uint64(proof[37])<<8 | uint64(proof[38])<<16 | uint64(proof[39])<<24 |
	          uint64(proof[40])<<32 | uint64(proof[41])<<40 | uint64(proof[42])<<48 | uint64(proof[43])<<56
	blockHeader := proof[len(proof)-80:]

	// Verify block header exists in our Bitcoin node
	header, err := s.btcClient.GetBlockHeader(hex.EncodeToString(blockHeader))
	if err != nil {
		return fmt.Errorf("block header not found in Bitcoin network: %w", err)
	}

	// Check confirmations
	blockHeight := header.Height
	tipHeight, err := s.btcClient.GetBlockCount()
	if err != nil {
		return fmt.Errorf("failed to get tip height: %w", err)
	}

	if tipHeight < blockHeight+6 {
		return fmt.Errorf("insufficient confirmations: %d < %d", tipHeight-blockHeight, 6)
	}

	// Submit proof to contract via GraphQL
	gqlClient := graphql.NewClient("http://localhost:7080/api/v1/graphql", nil)

	// Create mock signed transaction (same as SDK implementation)
	// TODO: Implement proper transaction creation and signing
	mockTx := []byte("mock_deposit_transaction")
	mockSig := []byte("mock_deposit_signature")

	txStr := base64.StdEncoding.EncodeToString(mockTx)
	sigStr := base64.StdEncoding.EncodeToString(mockSig)

	var mutation struct {
		SubmitTransactionV1 struct {
			Id graphql.String `graphql:"id"`
		} `graphql:"submitTransactionV1(tx: $tx, sig: $sig)"`
	}

	err := gqlClient.Query(ctx, &mutation, map[string]interface{}{
		"tx":  graphql.String(txStr),
		"sig": graphql.String(sigStr),
	})

	if err != nil {
		return fmt.Errorf("failed to submit deposit proof: %w", err)
	}

	log.Printf("Deposit proof submitted successfully for txid %x vout %d amount %d, tx ID: %s",
		txid, vout, amount, mutation.SubmitTransactionV1.Id)

	return nil
}

// Close shuts down the service
func (s *Service) Close() {
	s.btcClient.Shutdown()
}
