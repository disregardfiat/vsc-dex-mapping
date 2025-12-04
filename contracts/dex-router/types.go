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

type ReturnAddress struct {
	Chain   string `json:"chain"`
	Address string `json:"address"`
}
