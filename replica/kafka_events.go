package replica

import (
  "github.com/Shopify/sarama"
  "fmt"
  "compress/zlib"
  "github.com/ethereum/go-ethereum/common"
  "github.com/ethereum/go-ethereum/core"
  "github.com/ethereum/go-ethereum/core/rawdb"
  "github.com/ethereum/go-ethereum/core/types"
  "github.com/ethereum/go-ethereum/event"
  "github.com/ethereum/go-ethereum/rlp"
  "github.com/ethereum/go-ethereum/log"
  "github.com/ethereum/go-ethereum/ethdb/cdc"
  "github.com/ethereum/go-ethereum/ethdb"
  // "encoding/hex"
)

type MsgType byte

const (
  BlockMsg MsgType = iota
  ReceiptMsg
  LogMsg
)

func compress(data []byte) []byte {
  if len(data) == 0 { return data }
  var b bytes.Buffer
  w := zlib.NewWriter(&b)
  w.Write(data)
  w.Close()
  return b.Bytes()
}

func decompress(data []byte) ([]byte, error) {
  if len(data) == 0 { return data, nil }
  r, err := zlib.NewReader(bytes.NewBuffer(data))
  if err != nil { return []byte{}, err }
  return ioutil.ReadAll(r)
}

type receiptMeta struct {
  contractAddress common.Address
  cumulativeGasUsed uint64
  gasUsed uint64
  status uint64
  logCount uint64
  logsBloom types.Bloom
}

type rlpReceiptMeta struct {
  ContractAddress common.Address
  CumulativeGasUsed uint64
  GasUsed uint64
  Status uint64
  LogCount uint64
  LogsBloom []byte
}

func (r *receiptMeta) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, rlpReceiptMeta{
    ContractAddress: r.contractAddress,
    CumulativeGasUsed: r.cumulativeGasUsed,
    GasUsed: r.gasUsed,
    Status: r.status,
    LogCount: r.logCount,
    LogsBloom: compress(r.logsBloom.Bytes()),
  })
}

func (r *receiptMeta) DecodeRLP(s *rlp.Stream) error {
  var dec rlpReceiptMeta
	err := s.Decode(&dec)
	if err == nil {
		r.contractAddress, r.cumulativeGasUsed, r.gasUsed, r.status, r.logCount = dec.ContractAddress, dec.CumulativeGasUseddec.GasUsed, dec.Status, dec.LogCount
    r.logsBloom, err = decompress(dec.LogsBloom)
	}
	return err
}

type ChainEvent struct {
  Block *types.Block
  receiptMeta map[common.Hash]*receiptMeta
  logs map[common.Hash][]*types.Log
}

type chainEventProvider interface {
  GetChainEvent(common.Hash, uint64) (*ChainEvent, error)
  GetBlock(common.Hash) (*types.Block, error)
  GetHeadBlockHash() (common.Hash)
  GetFullChainEvent(ce core.ChainEvent) (*ChainEvent, error)
}

type dbChainEventProvider struct {
  db ethdb.Database
}

func (cep *dbChainEventProvider) GetHeadBlockHash() common.Hash {
  return rawdb.ReadHeadBlockHash(cep.db)
}

func (cep *dbChainEventProvider) GetBlock(h common.Hash) (*types.Block, error) {
  n := *rawdb.ReadHeaderNumber(cep.db, h)
  block := rawdb.ReadBlock(cep.db, h, n)
  if block == nil { return nil, fmt.Errorf("Error retrieving block %#x", h)}
  return block, nil
}
func (cep *dbChainEventProvider) GetChainEvent(h common.Hash, n uint64) (*ChainEvent, error) {
  block := rawdb.ReadBlock(cep.db, h, n)
  if block == nil { return nil, fmt.Errorf("Block %#x missing from database", h)}
  genesisHash := rawdb.ReadCanonicalHash(cep.db, 0)
  chainConfig := rawdb.ReadChainConfig(cep.db, genesisHash)
  receipts := rawdb.ReadReceipts(cep.db, h, n, chainConfig)
  logs := make(map[common.Hash][]*types.Log)
  receiptMeta := make(map[common.Hash][]*receiptMeta)
  if receipts != nil {
    // Receipts will be nil if the list is empty, so this is not an error condition
    for _, receipt := range receipts {
      logs[receipt.Hash] = receipt.Logs
      receiptMeta[receipt.Hash] = &receiptMeta{
        ContractAddress: receipt.ContractAddress,
        CumulativeGasUsed: receipt.CumulativeGasUsed,
        GasUsed: receipt.GasUsed,
        Status: receipt.Status,
        LogsBloom: receipt.Bloom,
        LogCount: len(receipt.Logs),
      }
    }
  }
  return &ChainEvent{Block: block, receiptMeta: receiptMeta, logs: logs}, nil
}

func (cep *dbChainEventProvider) GetFullChainEvent(ce core.ChainEvent) (*ChainEvent, error) {
  genesisHash := rawdb.ReadCanonicalHash(cep.db, 0)
  chainConfig := rawdb.ReadChainConfig(cep.db, genesisHash)
  receipts := rawdb.ReadReceipts(cep.db, ce.Block.Hash(), ce.Block.NumberU64(), chainConfig)
  logs := make(map[common.Hash][]*types.Log)
  receiptMeta := make(map[common.Hash][]*receiptMeta)
  if receipts != nil {
    for _, receipt := range receipts {
      logs[receipt.Hash] = receipt.Logs
      receiptMeta[receipt.Hash] = &receiptMeta{
        ContractAddress: receipt.ContractAddress,
        CumulativeGasUsed: receipt.CumulativeGasUsed,
        GasUsed: receipt.GasUsed,
        Status: receipt.Status,
        LogsBloom: receipt.Bloom,
        LogCount: len(receipt.Logs),
      }
    }
  }
  return &ChainEvent{Block: ce.Block, receiptMeta: receiptMeta, logs: logs}, nil
}

func (chainEvent *ChainEvent) getMessages(cep chainEventProvider) ([][]byte, error) {
  blockBytes, err := rlp.EncodeToBytes(chainEvent.Block)
  if err != nil { return nil, err }
  result := [][]byte{append([]byte{BlockMsg}, blockBytes...)}
  for i, transaction := range chainEvent.Block.Transactions() {
    result = append(result, append(append(append([]byte{ReceiptMsg}, chainEvent.Block.Hash().Bytes()...), transaction.Hash().Bytes()...), rlp.EncodeToBytes(chainEvent.receiptMeta[transaction.Hash()])...))
    for _, logRecord := range chainEvent.Logs[transaction.Hash()] {
      logBytes, err := rlp.EncodeToBytes(rlpLog{logRecord, logRecord.BlockNumber, logRecord.TxHash, logRecord.TxIndex, logRecord.BlockHash, logRecord.Index})
      if err != nil { return result, err }
      result = append(result, append([]byte{LogMsg}, logBytes...))
    }
  }
  return result, nil
}

type chainEventTracker struct {
  chainEvents map[common.Hash]*ChainEvent
  receiptCounter map[common.Hash]int
  logCounter map[common.Hash]int
  earlyReceipts map[common.Hash]map[common.Hash]*receiptMeta
  earlyLogs map[common.Hash]map[common.Hash]map[uint]*types.Log
  finished map[common.Hash]bool
  oldFinished map[common.Hash]bool
  finishedLimit int
  lastEmittedBlock common.Hash
  pendingEmits map[common.Hash]common.Hash
  feed event.Feed
}

func (cet.chainEventTracker) HandleMessage(data []byte) error {
  var blockhash common.Hash
  switch data[0] {
  case BlockMsg:
    block := &types.Block{}
    if err := rlp.DecodeBytes(data[1:], block); err != nil {
      return fmt.Errorf("Error decoding block")
    }
    blockhash := block.Hash()
    if _, ok := cet.chainEvents[blockhash]; ok { return nil } // We've already seen this block. Ignore
    cet.chainEvents[blockhash] = &ChainEvent{
      Block: block,
      receiptMeta: make(map[common.Hash]receiptMeta),
      logs: make(map[common.Hash][]*types.Log),
      receiptCounter: len(block.Transactions()),
    }
    if earlyReceipts, ok := cet.earlyReceipts[blockhash]; ok {
      for txhash, rmeta := range earlyReceipts {
        cet.HandleReceipt(block.Hash(), txhash, rmeta)
      }
      delete(cet.earlyReceipts, blockhash)
    }
  case ReceiptMsg:
    blockhash = common.BytesToHash(data[1:33])
    txhash := common.BytesToHash(data[33:65])
    rmeta := &receiptMeta{}
    if err := rlp.DecodeBytes(data[65:], rmeta); err != nil {
      return fmt.Errorf("Error decoding receipt: %v", err.Error())
    }
    if _, ok := cet.chainEvents[blockhash]; !ok {
      if _, ok := cet.earlyReceipts[blockhash]; !ok {
        cet.earlyReceipts[blockhash] = make(map[common.Hash]*receiptMeta)
      }
      cet.earlyReceipts[blockhash][txhash] = rmeta
      return nil
    }
    cet.HandleReceipt(blockhash, txhash, rmeta)
  case LogMsg:
    logRlp := &rlpLog{}
    if err := rlp.DecodeBytes(data[1:], logRlp); err != nil {
      return fmt.Errorf("Error decoding log: %v", err.Error())
    }
    logRecord := logRlp.Log
    logRecord.BlockNumber = logRlp.BlockNumber
    logRecord.TxHash = logRlp.TxHash
    logRecord.TxIndex = logRlp.TxIndex
    logRecord.BlockHash = logRlp.BlockHash
    logRecord.Index = logRlp.Index
    blockhash = logRlp.BlockHash
    txhash := logRlp.TxHash
    if _, ok := cet.chainEvents[blockhash]; !ok {
      cet.HandleEarlyLog(blockhash, txhash, logRecord)
      return nil // Log is early, nothing else to do
    }
    if _, ok := cet.chainEvents[blockhash].receiptMeta[txhash]; !ok {
      cet.HandleEarlyLog(blockhash, txhash, logRecord)
      return nil // Log is early, nothing else to do
    }
    if cet.chainEvents[blockhash].logs[txhash][log.TransactionIndex] != nil {
      return nil // Log is already present, nothing else to do
    }
    cet.chainEvents[blockhash].logs[txhash][log.TransactionIndex] = logRecord
    cet.logCounter[blockhash]--
  }
  if cet.logCounter[blockhash] == 0 && cet.receiptCounter[blockhash] == 0 {
    // Last message of block. Emit the chain event on appropriate feeds.
    ce := cet.chainEvents[blockhash]
    if ce.Block.Hash() == cet.lastEmittedBlock {
      return nil
    }
    if ce.Block.ParentHash() == cet.lastEmittedBlock || consumer.lastEmittedBlock == (common.Hash{}) {
      cet.Emit([]*ChainEvent{ce}, []*ChainEvent{})
    } else {
      lastce := cet.chainEvents[cet.lastEmittedBlock]
      if ce.Block.TotalDifficulty().Cmp(lastce.Block.TotalDifficulty()) <= 0 {
        // Don't emit reorgs until there's a block with a higher difficulty
        return nil
      }
    }
    //   revertBlocks, newBlocks, err := findCommonAncestor(event, lastEmittedEvent, []map[common.Hash]*core.ChainEvent{consumer.currentMap, consumer.oldMap})
    //   if err != nil {
    //     log.Error("Error finding common ancestor", "newBlock", event.Hash, "oldBlock", consumer.lastEmittedBlock, "error", err)
    //     return err
    //   }
    //   if len(newBlocks) > 0 {
    //     // If we have only revert blocks, this is just an out-of-order
    //     // block, and should be ignored.
    //     consumer.Emit(newBlocks, revertBlocks)
    //   }
    //   if len(consumer.currentMap) > consumer.recoverySize {
    //     consumer.oldMap = consumer.currentMap
    //     consumer.currentMap = make(map[common.Hash]*core.ChainEvent)
    //     consumer.currentMap[consumer.lastEmittedBlock] = consumer.oldMap[consumer.lastEmittedBlock]
    //   }
    // }
  }
}

func (cet.chainEventTracker) HandleEarlyLog(blockhash, txhash common.Hash, logRecord *types.Log) {
  if _, ok := cet.earlyLogs[blockhash]; !ok {
    cet.earlyLogs[blockhash] = make(map[common.Hash][]*types.Log)
  }
  if _, ok := cet.earlyLogs[blockhash][txhash]; !ok {
    cet.earlyLogs[blockhash][txhash] = make(map[uint]*types.Log)
  }
  cet.earlyLogs[blockhash][txhash][logRecord.TransactionIndex] = logRecord
}

func (cet.chainEventTracker) HandleReceipt(blockhash, txhash common.Hash, rmeta *receiptMeta) {
  if _, ok := cet.chainEvents[blockhash].receiptMeta[txhash]; ok { return } // We already have this receipt
  cet.chainEvents[blockhash].receiptMeta[txhash] = rmeta
  cet.chainEvents[blockhash].logs[txhash] = make([]*types.Log, rmeta.logCount)
  cet.logCounter[blockhash] += rmeta.logCount
  if earlyLogs, ok := cet.earlyLogs[blockhash]; ok {
    if logs, ok := earlyLogs[txhash]; ok {
      for _, log := range logs {
        cet.chainEvents[blockhash].logs[txhash][log.TransactionIndex] = log
        cet.logCounter[blockhash]--
      }
    }
    delete(earlyLogs, txhash)
    if len(earlyLogs) == 0 {
      delete(cet.earlyLogs, blockhash) // This was the last receipt with early logs, so clean up
    }
  }
  cet.receiptCounter[blockhash]--
}



type KafkaEventProducer struct {
  producer sarama.SyncProducer
  topic string
  closed bool
  cep chainEventProvider
}

func (producer *KafkaEventProducer) Close() {
  producer.closed = true
  producer.producer.Close()
}

type rlpLog struct {
  Log *types.Log
	BlockNumber uint64 `json:"blockNumber"`
	TxHash common.Hash `json:"transactionHash" gencodec:"required"`
	TxIndex uint `json:"transactionIndex" gencodec:"required"`
	BlockHash common.Hash `json:"blockHash"`
	Index uint `json:"logIndex" gencodec:"required"`
}

func (producer *KafkaEventProducer) getMessages(chainEvent core.ChainEvent) ([][]byte, error) {
  blockBytes, err := rlp.EncodeToBytes(chainEvent.Block)
  if err != nil { return nil, err }
  result := [][]byte{append([]byte{BlockMsg}, blockBytes...)}
  for _, logRecord := range chainEvent.Logs {
    logBytes, err := rlp.EncodeToBytes(rlpLog{logRecord, logRecord.BlockNumber, logRecord.TxHash, logRecord.TxIndex, logRecord.BlockHash, logRecord.Index})
    if err != nil { return result, err }
    result = append(result, append([]byte{LogMsg}, logBytes...))
  }
  result = append(result, append([]byte{EmitMsg}, chainEvent.Hash[:]...))
  return result, nil
}

func (producer *KafkaEventProducer) Emit(chainEvent core.ChainEvent) error {
  events, err := producer.getMessages(chainEvent)
  if err != nil { return err }
  for _, msg := range events {
    if _, _, err = producer.producer.SendMessage(&sarama.ProducerMessage{Topic: producer.topic, Value: sarama.ByteEncoder(msg)}); err != nil { return err }
  }
  return nil
}

type ChainEventSubscriber interface {
  SubscribeChainEvent(chan<- core.ChainEvent) event.Subscription
}

func (producer *KafkaEventProducer) ReprocessEvents(ceCh chan<- core.ChainEvent, n int) error {
  hash := producer.cep.GetHeadBlockHash()
  block, err := producer.cep.GetBlock(hash)
  if err != nil { return err }
  events := make([]core.ChainEvent, n)
  events[n-1], err = producer.cep.GetChainEvent(block.Hash(), block.NumberU64())
  if err != nil { return err }
  for i := n - 1; i > 0 && events[i].Block.NumberU64() > 0; i-- {
    events[i-1], err =  producer.cep.GetChainEvent(events[i].Block.ParentHash(), events[i].Block.NumberU64() - 1)
    if err != nil { return err }
  }
  for _, ce := range events {
    ceCh <- ce
  }
  return nil
}

func (producer *KafkaEventProducer) RelayEvents(bc ChainEventSubscriber) {
  go func() {
    ceCh := make(chan core.ChainEvent, 100)
    go producer.ReprocessEvents(ceCh, 10)
    subscription := bc.SubscribeChainEvent(ceCh)
    recentHashes := make(map[common.Hash]struct{})
    olderHashes := make(map[common.Hash]struct{})
    lastEmitted := common.Hash{}
    setTest := func (k common.Hash) bool {
      if _, ok := recentHashes[k]; ok { return true }
      _, ok := olderHashes[k]
      return ok
    }
    setAdd := func(k common.Hash) {
      recentHashes[k] = struct{}{}
      if len(recentHashes) > 128 {
        olderHashes = recentHashes
        recentHashes = make(map[common.Hash]struct{})
      }
      lastEmitted = k
    }
    first := true
    for ce := range ceCh {
      if first || setTest(ce.Block.ParentHash()) {
        if err := producer.Emit(ce); err != nil {
          log.Error("Failed to produce event log: %v", err.Error())
        }
        setAdd(ce.Hash)
        first = false
      } else {
        newBlocks, err := producer.getNewBlockAncestors(ce, lastEmitted)
        if err != nil {
          log.Error("Failed to find new block ancestors", "block", ce.Hash, "parent", ce.Block.ParentHash(), "le", lastEmitted, "error", err)
          continue
        }
        for _, pce := range newBlocks {
          if !setTest(pce.Hash) {
            if err := producer.Emit(pce); err != nil {
              log.Error("Failed to produce event log: %v", err.Error())
            }
            setAdd(pce.Hash)
          }
        }
      }
    }
    log.Warn("Event emitter shutting down")
    subscription.Unsubscribe()
  }()
}

func (producer *KafkaEventProducer) getNewBlockAncestors(ce core.ChainEvent, h common.Hash) ([]core.ChainEvent, error) {
  var err error
  oldBlock, err := producer.cep.GetBlock(h)
  if err != nil { return nil, err }
  newBlocks := []core.ChainEvent{ce}
  for {
    if oldBlock.Hash() == ce.Hash {
      // If we have a match, we're done, return them.
      return newBlocks, nil
    } else if ce.Block.NumberU64() <= oldBlock.NumberU64() {
      // oldBlock has a higher or equal number, but the blocks aren't equal.
      // Walk back the oldBlock
      oldBlock, err = producer.cep.GetBlock(oldBlock.ParentHash())
      if err != nil { return nil, err }
    } else if ce.Block.NumberU64() > oldBlock.NumberU64() {
      // the new block has a higher number, walk it back
      ce, err = producer.cep.GetChainEvent(ce.Block.ParentHash(), ce.Block.NumberU64() - 1)
      if err != nil { return nil, err }
      newBlocks = append([]core.ChainEvent{ce}, newBlocks...)
    }
  }
}

func NewKafkaEventProducerFromURLs(brokerURL, topic string, db ethdb.Database) (EventProducer, error) {
  configEntries := make(map[string]*string)
  brokers, config := cdc.ParseKafkaURL(brokerURL)
  if err := cdc.CreateTopicIfDoesNotExist(brokerURL, topic, 1, configEntries); err != nil {
    return nil, err
  }
  config.Producer.Return.Successes=true
  producer, err := sarama.NewSyncProducer(brokers, config)
  if err != nil {
    return nil, err
  }
  return NewKafkaEventProducer(producer, topic, &dbChainEventProvider{db}), nil
}

func NewKafkaEventProducer(producer sarama.SyncProducer, topic string, cep chainEventProvider) (EventProducer) {
  return &KafkaEventProducer{producer, topic, false, cep}
}

type KafkaEventConsumer struct {
  recoverySize int
  logsFeed event.Feed
  removedLogsFeed event.Feed
  chainFeed event.Feed
  chainHeadFeed event.Feed
  chainSideFeed event.Feed
  offsetFeed event.Feed
  startingOffset int64
  consumer sarama.PartitionConsumer
  oldMap map[common.Hash]*core.ChainEvent
  currentMap map[common.Hash]*core.ChainEvent
  topic string
  ready chan struct{}
  lastEmittedBlock common.Hash
  latestEmitOffset OffsetHash
}

func (consumer *KafkaEventConsumer) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
  return consumer.logsFeed.Subscribe(ch)
}
func (consumer *KafkaEventConsumer) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
  return consumer.removedLogsFeed.Subscribe(ch)
}
func (consumer *KafkaEventConsumer) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
  return consumer.chainFeed.Subscribe(ch)
}
func (consumer *KafkaEventConsumer) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
  return consumer.chainHeadFeed.Subscribe(ch)
}
func (consumer *KafkaEventConsumer) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
  return consumer.chainSideFeed.Subscribe(ch)
}
func (consumer *KafkaEventConsumer) SubscribeOffsets(ch chan<- OffsetHash) event.Subscription {
  return consumer.offsetFeed.Subscribe(ch)
}

func (consumer *KafkaEventConsumer) processEvent(msgType byte, msg []byte) error {
  if msgType == BlockMsg {
    // Message contains the block. Set up a ChainEvent for this block
    block := &types.Block{}
    if err := rlp.DecodeBytes(msg, block); err != nil {
      return fmt.Errorf("Error decoding block")
    }
    hash := block.Hash()
    if _, ok := consumer.currentMap[hash]; !ok {
      // First time we've seen the block.
      consumer.currentMap[hash] = &core.ChainEvent{Block: block, Hash: hash, Logs: []*types.Log{}}
    }
  } else if msgType == LogMsg{
    // Message contains a log. Add it to the chain event for the block.
    logRlp := &rlpLog{}
    if err := rlp.DecodeBytes(msg, logRlp); err != nil {
      return fmt.Errorf("Error decoding log")
    }
    logRecord := logRlp.Log
    logRecord.BlockNumber = logRlp.BlockNumber
    logRecord.TxHash = logRlp.TxHash
    logRecord.TxIndex = logRlp.TxIndex
    logRecord.BlockHash = logRlp.BlockHash
    logRecord.Index = logRlp.Index
    if _, ok := consumer.currentMap[logRecord.BlockHash]; !ok {
      if ce, ok := consumer.oldMap[logRecord.BlockHash]; ok {
        consumer.currentMap[logRecord.BlockHash] = ce
      } else {
        return fmt.Errorf("Received log for unknown block %#x", logRecord.BlockHash[:])
      }
    }
    for _, l := range consumer.currentMap[logRecord.BlockHash].Logs {
      // TODO: Consider some separate map for faster lookups.  Not an
      // immediate concern, as block log counts are in the low hundreds,
      // and this implementation should be O(n*log(n))
      if l.Index == logRecord.Index {
        // Log is already in the list, don't add it again
        return nil
      }
    }
    consumer.currentMap[logRecord.BlockHash].Logs = append(consumer.currentMap[logRecord.BlockHash].Logs, logRecord)
  } else if msgType == EmitMsg {
    // Last message of block. Emit the chain event on appropriate feeds.
    hash := common.BytesToHash(msg)
    event, ok := consumer.currentMap[hash]
    if !ok {
      event, ok = consumer.oldMap[hash]
      if !ok {
        return fmt.Errorf("Received emit for unknown block %#x", hash[:])
      }
    }
    emptyHash := common.Hash{}
    if event.Hash == consumer.lastEmittedBlock {
      // Given multiple masters, we'll see blocks repeat
      return nil
    }
    if event.Block.ParentHash() == consumer.lastEmittedBlock || consumer.lastEmittedBlock == emptyHash {
      // This is the next logical block or we're just booting up, just emit everything.
      consumer.Emit([]core.ChainEvent{*event}, []core.ChainEvent{})
    } else {
      lastEmittedEvent := consumer.currentMap[consumer.lastEmittedBlock]
      if event.Block.Number().Cmp(lastEmittedEvent.Block.Number()) <= 0 {
        // Don't emit reorgs until there's a new block
        return nil
      }
      revertBlocks, newBlocks, err := findCommonAncestor(event, lastEmittedEvent, []map[common.Hash]*core.ChainEvent{consumer.currentMap, consumer.oldMap})
      if err != nil {
        log.Error("Error finding common ancestor", "newBlock", event.Hash, "oldBlock", consumer.lastEmittedBlock, "error", err)
        return err
      }
      if len(newBlocks) > 0 {
        // If we have only revert blocks, this is just an out-of-order
        // block, and should be ignored.
        consumer.Emit(newBlocks, revertBlocks)
      }
      if len(consumer.currentMap) > consumer.recoverySize {
        consumer.oldMap = consumer.currentMap
        consumer.currentMap = make(map[common.Hash]*core.ChainEvent)
        consumer.currentMap[consumer.lastEmittedBlock] = consumer.oldMap[consumer.lastEmittedBlock]
      }
    }
  } else {
    return fmt.Errorf("Unknown message type %v", msgType)
  }
  return nil
}

func (consumer *KafkaEventConsumer) Ready() chan struct{} {
  return consumer.ready
}

type OffsetHash struct {
  Offset int64
  Hash common.Hash
}

func (consumer *KafkaEventConsumer) Start() {
  inputChannel := consumer.consumer.Messages()
  go func() {
    consumer.oldMap = make(map[common.Hash]*core.ChainEvent)
    consumer.currentMap = make(map[common.Hash]*core.ChainEvent)
    for input := range inputChannel {
      if consumer.ready != nil {
        if consumer.consumer.HighWaterMarkOffset() - input.Offset <= 1 {
          consumer.ready <- struct{}{}
          consumer.ready = nil
        }
      }
      msgType := input.Value[0]
      msg := input.Value[1:]
      if msgType == EmitMsg && input.Offset < consumer.startingOffset {
        // During the initial startup, we start several thousand or so messages
        // before we actually want to resume to make sure we have the blocks in
        // memory to handle a reorg. We don't want to re-emit these blocks, we
        // just want to populate our caches.
        continue
      }
      if msgType == EmitMsg {
        consumer.latestEmitOffset = OffsetHash{input.Offset, common.BytesToHash(msg)}
      }
      if err := consumer.processEvent(msgType, msg); err != nil {
        if input.Offset >= consumer.startingOffset {
          // Don't bother logging errors if we haven't reached the starting offset.
          log.Error("Error processing input:", "err", err, "msgType", msgType, "msg", msg, "offset", input.Offset)
        }
      }
    }
  }()
}

func getFromMappings(key common.Hash, mappings []map[common.Hash]*core.ChainEvent) *core.ChainEvent {
  for _, mapping := range mappings {
    if val, ok := mapping[key]; ok {
      return val
    }
  }
  return nil
}

func findCommonAncestor(newHead, oldHead *core.ChainEvent, mappings []map[common.Hash]*core.ChainEvent) ([]core.ChainEvent, []core.ChainEvent, error) {
  reverted := []core.ChainEvent{}
  newBlocks := []core.ChainEvent{*newHead}
  if oldHead == nil {
    return reverted, newBlocks, nil
  }
  for {
    for newHead.Block.NumberU64() > oldHead.Block.NumberU64() + 1 {
      parentHash := newHead.Block.ParentHash()
      newHead = getFromMappings(parentHash, mappings)
      if newHead == nil {
        return reverted, newBlocks, fmt.Errorf("Block %#x missing from history", parentHash)
      }
      newBlocks = append([]core.ChainEvent{*newHead}, newBlocks...)
    }
    if(oldHead.Block.Hash() == newHead.Block.ParentHash())  {
      return reverted, newBlocks, nil
    }
    reverted = append([]core.ChainEvent{*oldHead}, reverted...)
    oldHead = getFromMappings(oldHead.Block.ParentHash(), mappings)
    if oldHead == nil {
      return reverted, newBlocks, fmt.Errorf("Reached genesis without finding common ancestor")
    }
  }
}

func (consumer *KafkaEventConsumer) Emit(add []core.ChainEvent, remove []core.ChainEvent) {
  for _, revert := range remove {
    if len(revert.Logs) > 0 {
      consumer.removedLogsFeed.Send(core.RemovedLogsEvent{revert.Logs})
    }
    consumer.chainSideFeed.Send(core.ChainSideEvent{Block: revert.Block})
  }
  for _, newEvent := range add {
    if len(newEvent.Logs) > 0 {
      consumer.logsFeed.Send(newEvent.Logs)
    }
    consumer.chainHeadFeed.Send(core.ChainHeadEvent{Block: newEvent.Block})
    consumer.chainFeed.Send(newEvent)
    consumer.lastEmittedBlock = newEvent.Hash
  }
  consumer.offsetFeed.Send(consumer.latestEmitOffset)
}

func NewKafkaEventConsumerFromURLs(brokerURL, topic string, lastEmittedBlock common.Hash, offset int64) (EventConsumer, error) {
  brokers, config := cdc.ParseKafkaURL(brokerURL)
  if err := cdc.CreateTopicIfDoesNotExist(brokerURL, topic, 1, nil); err != nil {
    return nil, err
  }
  config.Version = sarama.V2_1_0_0
  client, err := sarama.NewClient(brokers, config)
  if err != nil {
    return nil, err
  }
  consumer, err := sarama.NewConsumerFromClient(client)
  if err != nil {
    return nil, err
  }
  startOffset := offset
  if startOffset > 5000 {
    startOffset -= 5000
  }
  partitionConsumer, err := consumer.ConsumePartition(topic, 0, startOffset)
  if err != nil {
    // We may not have been able to roll back 1000 messages, so just try with
    // the provided offset
    partitionConsumer, err = consumer.ConsumePartition(topic, 0, offset)
    if err != nil {
      return nil, err
    }
  }
  return &KafkaEventConsumer{
    recoverySize: 128, // Geth keeps 128 generations of state trie to handle reorgs, we'll keep at least 128 blocks in memory to be able to handle reorgs.
    consumer: partitionConsumer,
    oldMap: make(map[common.Hash]*core.ChainEvent),
    currentMap: make(map[common.Hash]*core.ChainEvent),
    ready: make(chan struct{}),
    lastEmittedBlock: common.Hash{},
    startingOffset: offset,
  }, nil
}
