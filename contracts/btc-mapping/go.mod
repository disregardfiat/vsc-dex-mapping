module github.com/vsc-eco/vsc-dex-mapping/contracts/btc-mapping

go 1.24

replace vsc-node => ../../../go-vsc-node/

require vsc-node v0.0.0

require (
	github.com/btcsuite/btcd v0.24.2
	github.com/btcsuite/btcd/btcutil v1.1.6
	github.com/btcsuite/btcd/chaincfg/chainhash v1.1.0
	github.com/CosmWasm/tinyjson v0.9.0
	github.com/stretchr/testify v1.9.0
)
