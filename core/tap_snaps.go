package core

import (
  "github.com/ethereum/go-ethereum/core/state/snapshot"
  "github.com/ethereum/go-ethereum/ethdb/cdc"
)

func TapSnaps(bc *BlockChain, brokerURL, topic string) error {
  brokers, config := ParseKafkaURL(brokerURL)
  if err := CreateTopicIfDoesNotExist(brokerURL, topic, 0, nil); err != nil {
    return err
  }
  config.Producer.MaxMessageBytes = 5000012
  producer, err := sarama.NewAsyncProducer(brokers, config)
  if err != nil { return nil, err }
  bc.snaps = snapshot.KafkaCDCTree(bc.snaps, producer, topic)
  return nil
}
