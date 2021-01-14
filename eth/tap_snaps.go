package eth

import (
  "github.com/ethereum/go-ethereum/core"
)

func TapSnaps(ethBackend *EthAPIBackend, brokerURL, topic string) error {
  return core.TapSnaps(ethBackend.eth.BlockChain(), brokerURL, topic)
}
