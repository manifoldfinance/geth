package replica

import (
  "math/big"
  "github.com/ethereum/go-ethereum/common"
  "github.com/ethereum/go-ethereum/core"
  // "github.com/ethereum/go-ethereum/event"
  "github.com/ethereum/go-ethereum/core/types"
  // "github.com/Shopify/sarama/mocks"
  "github.com/Shopify/sarama"
  // "math/rand"
  "reflect"
  "runtime"
  "testing"
  // "time"
  "fmt"
)

// type chainEventProvider interface {
//   GetChainEvent(common.Hash, uint64) (core.ChainEvent, error)
//   GetBlock(common.Hash) (*types.Block, error)
//   GetHeadBlockHash() (common.Hash)
//   GetFullChainEvent(ce core.ChainEvent) (*ChainEvent, error)
// }


type mockChainEventProvider struct {
  kv map[common.Hash]*ChainEvent
}

func (cep *mockChainEventProvider) GetBlock(h common.Hash) (*types.Block, error) {
  if ce, ok := cep.kv[h]; ok { return ce.Block, nil }
  return nil, fmt.Errorf("Block Not found %#x", h)
}
func (cep *mockChainEventProvider) GetChainEvent(h common.Hash, n uint64) (core.ChainEvent, error) {
  if ce, ok := cep.kv[h]; ok {
    logs := []*types.Log{}
    for _, logRecords := range ce.Logs {
      logs = append(logs, logRecords...)
    }
    return core.ChainEvent{Block: ce.Block, Hash: ce.Block.Hash(), Logs: logs}, nil
  }
  return core.ChainEvent{}, fmt.Errorf("CE Not found %#x", h)
}

func (cep *mockChainEventProvider) GetHeadBlockHash() common.Hash {
  var highest *ChainEvent
  for _, v := range cep.kv {
    if highest == nil || highest.Block.Hash() == (common.Hash{}) || v.Block.NumberU64() > highest.Block.NumberU64() {
      highest = v
    }
  }
  return highest.Block.Hash()
}

func (cep *mockChainEventProvider) GetFullChainEvent(ce core.ChainEvent) (*ChainEvent, error) {
  if fce, ok := cep.kv[ce.Block.Hash()]; ok { return fce, nil }
  return nil, fmt.Errorf("CE Not found %#x", ce.Block.Hash())
}


func getTestProducer(ces []*ChainEvent) *KafkaEventProducer {
  kv := make(map[common.Hash]*ChainEvent)
  for _, ce := range ces {
    kv[ce.Block.Hash()] = ce
  }
  return &KafkaEventProducer{
    nil,
    "",
    false,
    &mockChainEventProvider{kv},
  }
}

func getTestConsumer() (*KafkaEventConsumer, chan ChainEvents, func()) {
  consumer := &KafkaEventConsumer{
    cet: &chainEventTracker {
      topic: "test",
      chainEvents: make(map[common.Hash]*ChainEvent),
      receiptCounter: make(map[common.Hash]int),
      logCounter: make(map[common.Hash]int),
      earlyReceipts: make(map[common.Hash]map[common.Hash]*receiptMeta),
      earlyLogs: make(map[common.Hash]map[common.Hash]map[uint]*types.Log),
      earlyTd: make(map[common.Hash]*big.Int),
      finished: make(map[common.Hash]bool),
      oldFinished: make(map[common.Hash]bool),
      finishedLimit: 128,
      lastEmittedBlock: common.Hash{},
      pendingEmits: make(map[common.Hash]common.Hash),
      chainEventPartitions: make(map[int32]int64),
    },

    startingOffsets: make(map[int32]int64),
    consumers: []sarama.PartitionConsumer{},
    topic: "test",
    ready: make(chan struct{}),
  }
  chainEventsCh := make(chan ChainEvents, 100)
  chainEventsSub := consumer.SubscribeChainEvents(chainEventsCh)
  close := func() {
    chainEventsSub.Unsubscribe()
  }
  runtime.Gosched()
  return consumer, chainEventsCh, close
}

func getTestHeader(blockNo int64, nonce uint64, h *types.Header) *types.Header {
  parentHash := common.Hash{}
  if h != nil {
    parentHash = h.Hash()
  }
  return &types.Header{
  	ParentHash:  parentHash,
  	UncleHash:   common.Hash{},
  	Coinbase:    common.Address{},
  	Root:        common.Hash{},
  	TxHash:      common.Hash{},
  	ReceiptHash: common.Hash{},
  	Bloom:       types.Bloom{},
  	Difficulty:  big.NewInt(0),
  	Number:      big.NewInt(blockNo),
  	GasLimit:    0,
  	GasUsed:     0,
  	Time:        0,
  	Extra:       []byte{},
  	MixDigest:   common.Hash{},
  	Nonce:       types.EncodeNonce(nonce),
  }
}

func getTestLog(block *types.Block) *types.Log {
  return &types.Log {
  	Address: common.Address{},
  	Topics: []common.Hash{},
  	Data: []byte{},
  	BlockNumber: block.Number().Uint64(),
  	TxHash: common.Hash{},
  	TxIndex: 0,
  	BlockHash: block.Hash(),
  	Index: 0,
  }
}

func getTestChainEvent(blockNo int64, nonce uint64, h *types.Header) *ChainEvent {
  header := getTestHeader(blockNo, nonce, h)
  logs := make(map[common.Hash][]*types.Log)
  tx := types.NewTransaction(0, common.Address{}, big.NewInt(0), 21000, big.NewInt(1), []byte{})
  logs[tx.Hash()] = []*types.Log{
    &types.Log{
      Address: common.Address{},
      Topics: []common.Hash{},
      Data: []byte{},
      BlockNumber: uint64(blockNo),
      TxHash: tx.Hash(),
      TxIndex: 0,
      Index: 0,
    },
  }
  bloom := types.Bloom{}
  bloom.Add(types.LogsBloom(logs[tx.Hash()]))
  receipt := types.Receipt{
    PostState: []byte{},
    Status: 1,
    CumulativeGasUsed: 21000,
    Bloom: bloom,
    Logs: logs[tx.Hash()],
    TxHash: tx.Hash(),
    GasUsed: 21000,
    BlockNumber: big.NewInt(blockNo),
    TransactionIndex: 0,
  }


  block := types.NewBlock(header, []*types.Transaction{tx}, []*types.Header{}, []*types.Receipt{&receipt})
  rmetas := make(map[common.Hash]*receiptMeta)
  rmetas[tx.Hash()] = &receiptMeta{
    contractAddress: receipt.ContractAddress,
    cumulativeGasUsed: receipt.CumulativeGasUsed,
    gasUsed: receipt.GasUsed,
    status: receipt.Status,
    logsBloom: receipt.Bloom,
    logCount: uint(len(receipt.Logs)),
  }
  logs[tx.Hash()][0].BlockHash = block.Hash()
  return &ChainEvent{Block: block, ReceiptMeta: rmetas, Logs: logs, Td: big.NewInt(blockNo) } // Use blockno as a proxy for TD, as it should increase accordingly for the purposes of these tests
}

func TestGetProducerMessages(t *testing.T) {
  // producer := getTestProducer(nil)
  event := getTestChainEvent(0, 0, nil)
  // event := core.ChainEvent{Block: block, Hash: block.Hash(), Logs: logs[tx.Hash()]}
  messages := event.getMessages()
  if n := len(messages); n != 4 {
    t.Fatalf("Expected 4 messages, got %v", n)
  }
  if MsgType(messages[0].key[0]) != BlockMsg { t.Errorf("Message 0 should be block Msg, got %v", messages[0].key[0])}
  if MsgType(messages[1].key[0]) != TdMsg { t.Errorf("Message 1 should be block Msg, got %v", messages[1].key[0])}
  if MsgType(messages[2].key[0]) != ReceiptMsg { t.Errorf("Message 2 should be log Msg, got %v", messages[2].key[0])}
  if MsgType(messages[3].key[0]) != LogMsg { t.Errorf("Message 3 should be log Msg, got %v", messages[3].key[0])}
}

func expectToConsume(name string, ch interface{}, count int, t *testing.T) {
  chanval := reflect.ValueOf(ch)

  for i := 0; i < count; i++ {
    runtime.Gosched() // Let other goroutines populate the channel between selects
    chosen, _, _ := reflect.Select([]reflect.SelectCase{
      reflect.SelectCase{Dir: reflect.SelectRecv, Chan: chanval},
      reflect.SelectCase{Dir: reflect.SelectDefault},
    })
    if chosen == 1 {
      t.Errorf("%v: Expected %v items, got %v", name, count, i)
    }
  }
  chosen, _, _ := reflect.Select([]reflect.SelectCase{
    reflect.SelectCase{Dir: reflect.SelectRecv, Chan: chanval},
    reflect.SelectCase{Dir: reflect.SelectDefault},
  })
  if chosen == 0 {
    t.Errorf("%v: Expected %v items, got %v", name, count, count+1)
  }
}

// func getMessages(header *types.Header, producer *KafkaEventProducer, kv map[common.Hash]core.ChainEvent, t *testing.T) [][]byte {
//   event := getChainEvent(header)
//   messages, err := producer.getMessages(event)
//   if err != nil { t.Errorf(err.Error()) }
//   if kv != nil {
//     kv[event.Hash] = event
//   }
//   return messages
// }
//
// func getChainEvent(header *types.Header) core.ChainEvent {
//   block := types.NewBlock(header, []*types.Transaction{}, []*types.Header{}, []*types.Receipt{})
//   return core.ChainEvent{Block: block, Hash: block.Hash(), Logs: []*types.Log{getTestLog(block)}}
// }
//
func TestChainEventTracker(t *testing.T) {
  event := getTestChainEvent(0, 0, nil)
  messages := event.getMessages()
  // rand.Seed(time.Now().UnixNano())
  // rand.Shuffle(len(messages), func(i, j int) { messages[i], messages[j] = messages[j], messages[i] })
  consumer, _, close := getTestConsumer()
  defer close()
  var chainEvents *ChainEvents
  for i, msg := range messages {
    var err error
    chainEvents, err = consumer.cet.HandleMessage(msg.key, msg.value, 0, int64(i))
    if err != nil { t.Errorf("Error handling message: %v", err) }
    if chainEvents != nil && i < 3 { t.Errorf("Got chainEvents before anticipated: %v", chainEvents) }
  }
  if chainEvents == nil {
    t.Fatalf("Expected chainEvents object: %v", consumer.cet)
  }
  if reverted := len(chainEvents.Reverted); reverted != 0 { t.Errorf("Expected 0 reverts, got %v", reverted)}
  if newCount := len(chainEvents.New); newCount != 1 { t.Errorf("Expected 1 new, got %v", newCount)}
  if offset, ok := chainEvents.Partitions[0]; !ok || offset !=3 { t.Errorf("Expected offset to be 3") }
  blockhash := chainEvents.New[0].Block.Hash()
  if !consumer.cet.finished[blockhash] { t.Errorf("Block should be marked finished in CET") }
  if _, ok := consumer.cet.logCounter[blockhash]; ok {t.Errorf("logCounter should be gone for blockhash")}
  if _, ok := consumer.cet.receiptCounter[blockhash]; ok {t.Errorf("receiptCounter should be gone for blockhash")}
}
func handleReorderedMessages(t *testing.T, order []int) {
  event := getTestChainEvent(0, 0, nil)
  messages := event.getMessages()
  consumer, _, close := getTestConsumer()
  defer close()
  var chainEvents *ChainEvents
  for j, i := range order {
    msg := messages[i]
    var err error
    chainEvents, err = consumer.cet.HandleMessage(msg.key, msg.value, 0, int64(j))
    if err != nil { t.Errorf("Error handling message: %v", err) }
  }
  if chainEvents == nil {
    t.Fatalf("Expected chainEvents object: %v", consumer.cet)
  }
  if reverted := len(chainEvents.Reverted); reverted != 0 { t.Errorf("Expected 0 reverts, got %v", reverted)}
  if newCount := len(chainEvents.New); newCount != 1 { t.Errorf("Expected 1 new, got %v", newCount)}
  if offset, ok := chainEvents.Partitions[0]; !ok || offset != int64(len(order) - 1) { t.Errorf("Expected offset to be %v", len(order) - 1) }
  blockhash := chainEvents.New[0].Block.Hash()
  if !consumer.cet.finished[blockhash] { t.Errorf("Block should be marked finished in CET") }
  if _, ok := consumer.cet.logCounter[blockhash]; ok {t.Errorf("logCounter should be gone for blockhash")}
  if _, ok := consumer.cet.receiptCounter[blockhash]; ok {t.Errorf("receiptCounter should be gone for blockhash")}
}
func TestReorderedEventTracker(t *testing.T) {
  handleReorderedMessages(t, []int{3, 2, 1, 0})
  handleReorderedMessages(t, []int{0, 3, 2, 1})
  handleReorderedMessages(t, []int{0, 3, 2, 3, 1})
  handleReorderedMessages(t, []int{0, 3, 2, 3, 0, 2, 1})
}

// One of the key things we need to test are the handling of out-of-order
// delivery of messages in reorg scenarios. Given four blocks:
// A: The root of all blocks
// B: Child of A
// C: child of A
// D: Child of C
//
// We need to ensure the following correct deliveries:
// ABCD: (A), (B), (CD, -B)
// ABDC: (A), (B), (CD, -B)
// ACDB: (A), (C), (D)
// ADC: (A), (CD)

func reorgTester(t *testing.T, messages []chainEventMessage, expectedEvents []*ChainEvents) {
  outputs := []*ChainEvents{}
  consumer, _, close := getTestConsumer()
  defer close()
  for i, msg := range messages {
    chainEvents, err := consumer.cet.HandleMessage(msg.key, msg.value, 0, int64(i))
    if err != nil { t.Errorf("Error handling message: %v", err) }
    if chainEvents != nil { outputs = append(outputs, chainEvents) }
  }
  if len(outputs) != len(expectedEvents) { t.Fatalf("Expected %v outputs, got %v", len(expectedEvents), len(outputs))}
  for i, chainEvents := range expectedEvents {
    if len(chainEvents.New) != len(outputs[i].New) { t.Fatalf("Expected events[%v]: %v New, got %v", i, len(chainEvents.New), len(outputs[i].New))}
    for j, chainEvent := range chainEvents.New {
      if chainEvent.Block.Hash() != outputs[i].New[j].Block.Hash() { t.Errorf("Got new chain events out of order o[%v][%v]", i, j) }
    }
    if len(chainEvents.Reverted) != len(outputs[i].Reverted) { t.Fatalf("Expected events[%v]: %v Reverted, got %v", i, len(chainEvents.Reverted), len(outputs[i].Reverted))}
    for j, chainEvent := range chainEvents.Reverted {
      if chainEvent.Block.Hash() != outputs[i].Reverted[j].Block.Hash() { t.Errorf("Got reverted chain events out of order o[%v][%v]", i, j) }
    }
  }
}

func TestReorg(t *testing.T) {
  a := getTestChainEvent(0, 0, nil)
  b := getTestChainEvent(1, 0, a.Block.Header())
  c := getTestChainEvent(1, 1, a.Block.Header())
  d := getTestChainEvent(2, 1, c.Block.Header())
  if b.Block.ParentHash() != a.Block.Hash() { t.Fatalf("b should be child of a") }
  if c.Block.ParentHash() != a.Block.Hash() { t.Fatalf("c should be child of a") }
  if d.Block.ParentHash() != c.Block.Hash() { t.Fatalf("d should be child of c") }

  // TODO: Try more out-of-order messages (instead of whole out-of-order blocks)
  t.Run("Reorg ABCD", func(t *testing.T) {
    reorgTester(
      t,
      append(append(append(a.getMessages(), b.getMessages()...), c.getMessages()...), d.getMessages()...),
      []*ChainEvents{
        &ChainEvents{New: []*ChainEvent{a}},
        &ChainEvents{New: []*ChainEvent{b}},
        &ChainEvents{New: []*ChainEvent{c, d}, Reverted: []*ChainEvent{b}},
      },
    )
  })
  t.Run("Reorg ABDC", func(t *testing.T) {
    reorgTester(
      t,
      append(append(append(a.getMessages(), b.getMessages()...), d.getMessages()...), c.getMessages()...),
      []*ChainEvents{
        &ChainEvents{New: []*ChainEvent{a}},
        &ChainEvents{New: []*ChainEvent{b}},
        &ChainEvents{New: []*ChainEvent{c, d}, Reverted: []*ChainEvent{b}},
      },
    )
  })
  t.Run("Reorg ACDB", func(t *testing.T) {
    reorgTester(
      t,
      append(append(append(a.getMessages(), c.getMessages()...), d.getMessages()...), b.getMessages()...),
      []*ChainEvents{
        &ChainEvents{New: []*ChainEvent{a}},
        &ChainEvents{New: []*ChainEvent{c}},
        &ChainEvents{New: []*ChainEvent{d},},
      },
    )
  })
  t.Run("Reorg adc", func(t *testing.T) {
    reorgTester(
      t,
      append(append(a.getMessages(), d.getMessages()...), c.getMessages()...),
      []*ChainEvents{
        &ChainEvents{New: []*ChainEvent{a}},
        &ChainEvents{New: []*ChainEvent{c, d}},
      },
    )
  })
  t.Run("Test replay", func(t *testing.T) {
    producer := getTestProducer([]*ChainEvent{a, b, c, d})
    events := make(chan core.ChainEvent)
    go producer.ReprocessEvents(events, 3)
    runtime.Gosched()
    expectToConsume("replay events", events, 3, t)
  })
}

//
// func TestReprocessEvents(t *testing.T) {
//   kv := make(map[common.Hash]core.ChainEvent)
//   producer := getTestProducer(kv)
//   root := getTestHeader(0, 0, nil)
//   rootCe := getChainEvent(root)
//   kv[rootCe.Hash] = rootCe
//   child := getChainEvent(getTestHeader(1, 1, root))
//   kv[child.Hash] = child
//   grandchild := getChainEvent(getTestHeader(2, 1, child.Block.Header()))
//   kv[grandchild.Hash] = grandchild
//   ch := make(chan core.ChainEvent, 10)
//   if err := producer.ReprocessEvents(ch, 3); err != nil { t.Fatalf(err.Error())}
//   if h := (<-ch).Hash; h != rootCe.Hash { t.Errorf("First result should match root, %#x != %#x", h, rootCe.Hash)}
//   if h := (<-ch).Hash; h != child.Hash { t.Errorf("Second result should match child, %#x != %#x", h, child.Hash)}
//   if h := (<-ch).Hash; h != grandchild.Hash { t.Errorf("Third result should match grandchild, %#x != %#x", h, grandchild.Hash)}
//   select {
//   case <-ch:
//     t.Errorf("Unexpected fourth result")
//   default:
//   }
//   if err := producer.ReprocessEvents(ch, 2); err != nil { t.Fatalf(err.Error())}
//   if h := (<-ch).Hash; h != child.Hash { t.Errorf("Fourth result should match child, %#x != %#x", h, child.Hash)}
//   if h := (<-ch).Hash; h != grandchild.Hash { t.Errorf("Tfifth result should match grandchild, %#x != %#x", h, grandchild.Hash)}
//   select {
//   case <-ch:
//     t.Errorf("Unexpected fourth result")
//   default:
//   }
// }
