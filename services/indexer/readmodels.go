package indexer

import (
	"encoding/json"
	"sync"
)

// TransactionInfo represents a DEX transaction
type TransactionInfo struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // "swap", "deposit", "withdrawal"
	PoolID      string                 `json:"pool_id"`
	User        string                 `json:"user"`
	BlockHeight uint64                 `json:"block_height"`
	Timestamp   string                 `json:"timestamp"`
	Details     map[string]interface{} `json:"details"`
}

// LiquidityPosition represents a user's liquidity position in a pool
type LiquidityPosition struct {
	User   string  `json:"user"`
	PoolID string  `json:"pool_id"`
	Amount uint64  `json:"amount"`
	Share  float64 `json:"share"` // Percentage of total pool liquidity
}

// DexReadModel implements read model for DEX operations
type DexReadModel struct {
	mu           sync.RWMutex
	pools        map[string]PoolInfo
	transactions []TransactionInfo
	positions    map[string][]LiquidityPosition // pool_id -> []positions
}

// NewDexReadModel creates a new DEX read model
func NewDexReadModel() *DexReadModel {
	return &DexReadModel{
		pools:        make(map[string]PoolInfo),
		transactions: make([]TransactionInfo, 0),
		positions:    make(map[string][]LiquidityPosition),
	}
}

// HandleEvent processes VSC events and updates read models
func (dm *DexReadModel) HandleEvent(event VSCEvent) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	switch event.Contract {
	case "dex-router":
		return dm.handleDexRouterEvent(event)
	}

	return nil
}

// handleDexRouterEvent processes DEX router events
func (dm *DexReadModel) handleDexRouterEvent(event VSCEvent) error {
	// Extract common transaction info
	txInfo := TransactionInfo{
		ID:          event.TxID,
		BlockHeight: event.BlockHeight,
		Timestamp:   "", // Would need to be populated from block data
	}

	// Handle pool creation, liquidity changes, and swaps from unified contract
	switch event.Method {
	case "pool_created":
		var args struct {
			PoolID string  `json:"pool_id"`
			Asset0 string  `json:"asset0"`
			Asset1 string  `json:"asset1"`
			Fee    float64 `json:"fee"`
		}
		if err := json.Unmarshal(event.Args, &args); err != nil {
			return err
		}

		dm.pools[args.PoolID] = PoolInfo{
			ID:       args.PoolID,
			Asset0:   args.Asset0,
			Asset1:   args.Asset1,
			Fee:      args.Fee,
			Reserve0: 0,
			Reserve1: 0,
		}

		txInfo.Type = "pool_created"
		txInfo.PoolID = args.PoolID
		txInfo.Details = map[string]interface{}{
			"asset0": args.Asset0,
			"asset1": args.Asset1,
			"fee":    args.Fee,
		}

	case "liquidity_added":
		var args struct {
			PoolID   string `json:"pool_id"`
			User     string `json:"user,omitempty"`
			Amount0  uint64 `json:"amount0"`
			Amount1  uint64 `json:"amount1"`
			LPTokens uint64 `json:"lp_tokens,omitempty"`
		}
		if err := json.Unmarshal(event.Args, &args); err != nil {
			return err
		}

		if pool, exists := dm.pools[args.PoolID]; exists {
			pool.Reserve0 += args.Amount0
			pool.Reserve1 += args.Amount1
			// Backward compatibility: if no lp_tokens specified, use amount0 as before
			lpTokens := args.LPTokens
			if lpTokens == 0 {
				lpTokens = args.Amount0 // Maintain old test behavior
			}
			pool.TotalSupply += lpTokens
			dm.pools[args.PoolID] = pool

			// Update liquidity position only if user is specified
			if args.User != "" {
				dm.updateLiquidityPosition(args.PoolID, args.User, lpTokens, true)
			}
		}

		txInfo.Type = "deposit"
		txInfo.PoolID = args.PoolID
		txInfo.User = args.User
		txInfo.Details = map[string]interface{}{
			"amount0":   args.Amount0,
			"amount1":   args.Amount1,
			"lp_tokens": args.LPTokens,
		}

	case "liquidity_removed":
		var args struct {
			PoolID   string `json:"pool_id"`
			User     string `json:"user"`
			Amount0  uint64 `json:"amount0"`
			Amount1  uint64 `json:"amount1"`
			LPTokens uint64 `json:"lp_tokens"`
		}
		if err := json.Unmarshal(event.Args, &args); err != nil {
			return err
		}

		if pool, exists := dm.pools[args.PoolID]; exists {
			pool.Reserve0 -= args.Amount0
			pool.Reserve1 -= args.Amount1
			pool.TotalSupply -= args.LPTokens
			dm.pools[args.PoolID] = pool

			// Update liquidity position
			dm.updateLiquidityPosition(args.PoolID, args.User, args.LPTokens, false)
		}

		txInfo.Type = "withdrawal"
		txInfo.PoolID = args.PoolID
		txInfo.User = args.User
		txInfo.Details = map[string]interface{}{
			"amount0":   args.Amount0,
			"amount1":   args.Amount1,
			"lp_tokens": args.LPTokens,
		}

	case "swap_executed":
		var args struct {
			PoolID    string `json:"pool_id"`
			User      string `json:"user,omitempty"`
			Amount0   int64  `json:"amount0,omitempty"` // Reserve delta for asset0 (backward compatibility)
			Amount1   int64  `json:"amount1,omitempty"` // Reserve delta for asset1 (backward compatibility)
			AmountIn  uint64 `json:"amount_in,omitempty"`
			AmountOut uint64 `json:"amount_out,omitempty"`
			AssetIn   string `json:"asset_in,omitempty"`
			AssetOut  string `json:"asset_out,omitempty"`
		}
		if err := json.Unmarshal(event.Args, &args); err != nil {
			return err
		}

		if pool, exists := dm.pools[args.PoolID]; exists {
			// Handle backward compatibility: if amount0/amount1 are provided, treat as deltas
			if args.Amount0 != 0 || args.Amount1 != 0 {
				pool.Reserve0 = uint64(int64(pool.Reserve0) + args.Amount0)
				pool.Reserve1 = uint64(int64(pool.Reserve1) + args.Amount1)
			} else {
				// New format: update reserves based on swap direction
				if args.AssetIn == pool.Asset0 {
					pool.Reserve0 += args.AmountIn
					pool.Reserve1 -= args.AmountOut
				} else {
					pool.Reserve1 += args.AmountIn
					pool.Reserve0 -= args.AmountOut
				}
			}
			dm.pools[args.PoolID] = pool
		}

		txInfo.Type = "swap"
		txInfo.PoolID = args.PoolID
		txInfo.User = args.User
		txInfo.Details = map[string]interface{}{
			"amount0":    args.Amount0,
			"amount1":    args.Amount1,
			"amount_in":  args.AmountIn,
			"amount_out": args.AmountOut,
			"asset_in":   args.AssetIn,
			"asset_out":  args.AssetOut,
		}
	}

	// Add transaction to history (keep last 1000 transactions)
	dm.transactions = append(dm.transactions, txInfo)
	if len(dm.transactions) > 1000 {
		dm.transactions = dm.transactions[1:]
	}

	return nil
}

// QueryPools returns all indexed pools
func (dm *DexReadModel) QueryPools() ([]PoolInfo, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	pools := make([]PoolInfo, 0, len(dm.pools))
	for _, pool := range dm.pools {
		pools = append(pools, pool)
	}

	return pools, nil
}

// GetPool returns a specific pool by ID
func (dm *DexReadModel) GetPool(poolID string) (PoolInfo, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	pool, exists := dm.pools[poolID]
	return pool, exists
}

// updateLiquidityPosition updates a user's liquidity position
func (dm *DexReadModel) updateLiquidityPosition(poolID, user string, amount uint64, isAdd bool) {
	positions := dm.positions[poolID]
	found := false

	for i, pos := range positions {
		if pos.User == user {
			if isAdd {
				pos.Amount += amount
			} else {
				if pos.Amount > amount {
					pos.Amount -= amount
				} else {
					pos.Amount = 0
				}
			}
			positions[i] = pos
			found = true
			break
		}
	}

	if !found && isAdd && amount > 0 {
		positions = append(positions, LiquidityPosition{
			User:   user,
			PoolID: poolID,
			Amount: amount,
		})
	}

	// Update shares for all positions in this pool
	totalLP := dm.pools[poolID].TotalSupply
	for i := range positions {
		if totalLP > 0 {
			positions[i].Share = float64(positions[i].Amount) / float64(totalLP) * 100
		} else {
			positions[i].Share = 0
		}
	}

	dm.positions[poolID] = positions
}

// QueryTransactions returns recent transactions with optional filtering
func (dm *DexReadModel) QueryTransactions(poolID string, txType string, limit int) ([]TransactionInfo, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	var filtered []TransactionInfo

	for i := len(dm.transactions) - 1; i >= 0; i-- {
		tx := dm.transactions[i]

		if poolID != "" && tx.PoolID != poolID {
			continue
		}
		if txType != "" && tx.Type != txType {
			continue
		}

		filtered = append(filtered, tx)
		if len(filtered) >= limit {
			break
		}
	}

	return filtered, nil
}

// GetTransaction returns a specific transaction by ID
func (dm *DexReadModel) GetTransaction(txID string) (TransactionInfo, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	for _, tx := range dm.transactions {
		if tx.ID == txID {
			return tx, true
		}
	}
	return TransactionInfo{}, false
}

// QueryLiquidityPositions returns liquidity positions for a pool
func (dm *DexReadModel) QueryLiquidityPositions(poolID string) ([]LiquidityPosition, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	positions, exists := dm.positions[poolID]
	if !exists {
		return []LiquidityPosition{}, nil
	}

	// Return copy to avoid external modification
	result := make([]LiquidityPosition, len(positions))
	copy(result, positions)
	return result, nil
}

// QueryRichList returns top liquidity holders for a pool with pagination
func (dm *DexReadModel) QueryRichList(poolID string, offset, limit int) ([]LiquidityPosition, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	positions, exists := dm.positions[poolID]
	if !exists {
		return []LiquidityPosition{}, nil
	}

	// Sort by amount descending (simple sort for demo)
	for i := 0; i < len(positions)-1; i++ {
		for j := i + 1; j < len(positions); j++ {
			if positions[i].Amount < positions[j].Amount {
				positions[i], positions[j] = positions[j], positions[i]
			}
		}
	}

	// Apply pagination
	start := offset
	end := offset + limit
	if start >= len(positions) {
		return []LiquidityPosition{}, nil
	}
	if end > len(positions) {
		end = len(positions)
	}

	result := make([]LiquidityPosition, end-start)
	copy(result, positions[start:end])
	return result, nil
}
