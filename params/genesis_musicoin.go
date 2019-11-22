package params

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// MusicoinGenesisBlock returns the Musicoin main net genesis block.
func DefaultMusicoinGenesisBlock() *Genesis {
	return &Genesis{
		Config:     MusicoinChainConfig,
		Timestamp:  0,
		Nonce:      42,
		ExtraData:  nil,
		Mixhash:    common.HexToHash("0x00000000000000000000000000000000000000647572616c65787365646c6578"),
		GasLimit:   8000000,
		Difficulty: big.NewInt(4000000),
		Alloc:      decodePrealloc(MusicoinAllocData),
	}
}
