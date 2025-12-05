package main

// DEX Instruction Schema
//
//tinyjson:json
type DexInstruction struct {
	Type          string                 `json:"type"`
	Version       string                 `json:"version"`
	AssetIn       string                 `json:"asset_in"`
	AssetOut      string                 `json:"asset_out"`
	Recipient     string                 `json:"recipient"`
	SlippageBps   *int                   `json:"slippage_bps,omitempty"`
	MinAmountOut  *int64                 `json:"min_amount_out,omitempty"`
	Beneficiary   *string                `json:"beneficiary,omitempty"`
	RefBps        *int                   `json:"ref_bps,omitempty"`
	ReturnAddress *ReturnAddress         `json:"return_address,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

//tinyjson:json
type CreatePoolParams struct {
	Asset0 string `json:"asset0"`
	Asset1 string `json:"asset1"`
	FeeBps uint64 `json:"fee_bps"`
}

//tinyjson:json
type PoolInfo struct {
	Asset0   string `json:"asset0"`
	Asset1   string `json:"asset1"`
	Reserve0 uint64 `json:"reserve0"`
	Reserve1 uint64 `json:"reserve1"`
	Fee      uint64 `json:"fee"`
	TotalLp  uint64 `json:"total_lp"`
}

//tinyjson:json
type ReturnAddress struct {
	Chain   string `json:"chain"`
	Address string `json:"address"`
}
