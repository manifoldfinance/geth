package core

import (
  "github.com/ethereum/go-ethereum/core/state/snapshot"
  "github.com/ethereum/go-ethereum/ethdb/cdc"
  "github.com/Shopify/sarama"
)

func TapSnaps(bc *BlockChain, brokerURL, topic string) error {
  brokers, config := cdc.ParseKafkaURL(brokerURL)
  if err := cdc.CreateTopicIfDoesNotExist(brokerURL, topic, 0, nil); err != nil {
    return err
  }
  config.Producer.MaxMessageBytes = 5000012
  producer, err := sarama.NewAsyncProducer(brokers, config)
  if err != nil { return err }
  bc.snaps = snapshot.KafkaCDCTree(bc.snaps, producer, topic)
  return nil
}
