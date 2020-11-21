package core

import (
  "github.com/ethereum/go-ethereum/core/state/snapshot"
  "github.com/ethereum/go-ethereum/ethdb/cdc"
  "github.com/ethereum/go-ethereum/log"
  "github.com/Shopify/sarama"
)

func TapSnaps(bc *BlockChain, brokerURL, topic string) error {
  brokers, config := cdc.ParseKafkaURL(brokerURL)
  if err := cdc.CreateTopicIfDoesNotExist(brokerURL, topic, 0, nil); err != nil {
    return err
  }
  config.Producer.MaxMessageBytes = 5000012
  config.Producer.Return.Successes = true
  producer, err := sarama.NewAsyncProducer(brokers, config)
  if err != nil { return err }
  bc.snaps = snapshot.KafkaCDCTree(bc.snaps, producer, topic)
  log.Info("Tapping snapshotter", "broker", brokerURL, "topic", topic)
  return nil
}

func TapSnapsFile(bc *BlockChain, path string) (error, func()) {
  tree, err, closer := snapshot.FileCDCTree(bc.snaps, path)
  if tree != nil { bc.snaps = tree }
  return err, closer
}
