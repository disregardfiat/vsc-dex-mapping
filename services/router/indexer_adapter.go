package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// IndexerPoolQuerier implements PoolQuerier by querying the indexer HTTP API
type IndexerPoolQuerier struct {
	indexerEndpoint string
	httpClient      *http.Client
}

// NewIndexerPoolQuerier creates a new indexer-based pool querier
func NewIndexerPoolQuerier(indexerEndpoint string) *IndexerPoolQuerier {
	return &IndexerPoolQuerier{
		indexerEndpoint: indexerEndpoint,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// IndexerPoolInfo represents pool info from the indexer API
type IndexerPoolInfo struct {
	ID          string  `json:"id"`
	Asset0      string  `json:"asset0"`
	Asset1      string  `json:"asset1"`
	Reserve0    uint64  `json:"reserve0"`
	Reserve1    uint64  `json:"reserve1"`
	Fee         float64 `json:"fee"`
	TotalSupply uint64  `json:"total_supply"`
}

// GetPoolByID retrieves a pool by its contract ID
func (q *IndexerPoolQuerier) GetPoolByID(poolID string) (*PoolInfoWithReserves, error) {
	url := fmt.Sprintf("%s/api/v1/pools/%s", q.indexerEndpoint, poolID)
	
	resp, err := q.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("pool not found: %s", poolID)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("indexer returned status %d", resp.StatusCode)
	}

	var indexerPool IndexerPoolInfo
	if err := json.NewDecoder(resp.Body).Decode(&indexerPool); err != nil {
		return nil, fmt.Errorf("failed to decode pool response: %w", err)
	}

	// Convert indexer pool to router pool format
	// Fee is stored as float64 in indexer (e.g., 0.08 for 0.08%), convert to basis points (uint64)
	// 0.08% = 8 basis points, so multiply by 100
	feeBps := uint64(indexerPool.Fee * 100)

	return &PoolInfoWithReserves{
		ContractId: indexerPool.ID,
		Asset0:     indexerPool.Asset0,
		Asset1:     indexerPool.Asset1,
		Reserve0:   indexerPool.Reserve0,
		Reserve1:   indexerPool.Reserve1,
		Fee:        feeBps,
	}, nil
}

// GetPoolsByAsset retrieves all pools containing the specified asset
func (q *IndexerPoolQuerier) GetPoolsByAsset(asset string) ([]PoolInfoWithReserves, error) {
	url := fmt.Sprintf("%s/api/v1/pools", q.indexerEndpoint)
	
	resp, err := q.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to query indexer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("indexer returned status %d", resp.StatusCode)
	}

	var indexerPools []IndexerPoolInfo
	if err := json.NewDecoder(resp.Body).Decode(&indexerPools); err != nil {
		return nil, fmt.Errorf("failed to decode pools response: %w", err)
	}

	// Filter pools that contain the specified asset
	var matchingPools []PoolInfoWithReserves
	for _, indexerPool := range indexerPools {
		if indexerPool.Asset0 == asset || indexerPool.Asset1 == asset {
			feeBps := uint64(indexerPool.Fee * 100) // Convert decimal (0.08) to basis points (8)
			matchingPools = append(matchingPools, PoolInfoWithReserves{
				ContractId: indexerPool.ID,
				Asset0:     indexerPool.Asset0,
				Asset1:     indexerPool.Asset1,
				Reserve0:   indexerPool.Reserve0,
				Reserve1:   indexerPool.Reserve1,
				Fee:        feeBps,
			})
		}
	}

	return matchingPools, nil
}

