package replica

import (
  "math/big"
  "github.com/ethereum/go-ethereum/common"
  "github.com/ethereum/go-ethereum/core"
  "github.com/ethereum/go-ethereum/trie"
  // "github.com/ethereum/go-ethereum/event"
  "github.com/ethereum/go-ethereum/core/types"
  gethLog "github.com/ethereum/go-ethereum/log"
  // "github.com/Shopify/sarama/mocks"
  "github.com/Shopify/sarama"
  "math/rand"
  "reflect"
  "runtime"
  "testing"
  "time"
  "fmt"
  "os"
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

func getTestConsumer(lastEmittedBlock common.Hash) (*KafkaEventConsumer, chan *ChainEvents, func()) {
  consumer := &KafkaEventConsumer{
    cet: &chainEventTracker {
      topic: "test",
      chainEvents: make(map[common.Hash]*ChainEvent),
      receiptCounter: make(map[common.Hash]int),
      logCounter: make(map[common.Hash]int),
      earlyReceipts: make(map[common.Hash]map[common.Hash]*ReceiptMeta),
      earlyLogs: make(map[common.Hash]map[common.Hash]map[uint]*types.Log),
      earlyTd: make(map[common.Hash]*big.Int),
      finished: make(map[common.Hash]bool),
      oldFinished: make(map[common.Hash]bool),
      skipped: make(map[common.Hash]bool),
      finishedLimit: 128,
      lastEmittedBlock: lastEmittedBlock,
      pendingEmits: make(map[common.Hash]map[common.Hash]struct{}),
      pendingHashes: make(map[common.Hash]struct{}),
      chainEventPartitions: make(map[int32]int64),
      blockTime: make(map[common.Hash]time.Time),
    },

    startingOffsets: make(map[int32]int64),
    consumers: []sarama.PartitionConsumer{},
    topic: "test",
    ready: make(chan struct{}),
  }
  chainEventsCh := make(chan *ChainEvents, 100)
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


  block := types.NewBlock(header, []*types.Transaction{tx}, []*types.Header{}, []*types.Receipt{&receipt}, new(trie.Trie))
  rmetas := make(map[common.Hash]*ReceiptMeta)
  rmetas[tx.Hash()] = &ReceiptMeta{
    ContractAddress: receipt.ContractAddress,
    CumulativeGasUsed: receipt.CumulativeGasUsed,
    GasUsed: receipt.GasUsed,
    Status: receipt.Status,
    LogsBloom: receipt.Bloom,
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
  consumer, _, close := getTestConsumer(common.Hash{})
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
  consumer, _, close := getTestConsumer(common.Hash{})
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

func reorgTester(t *testing.T, messages []chainEventMessage, expectedEvents []*ChainEvents, lastEmitted common.Hash) *KafkaEventConsumer {
  outputs := []*ChainEvents{}
  consumer, _, close := getTestConsumer(lastEmitted)
  defer close()
  for i, msg := range messages {
    chainEvents, err := consumer.cet.HandleMessage(msg.key, msg.value, 0, int64(i))
    if err != nil { t.Errorf("Error handling message: %v", err) }
    if chainEvents != nil { outputs = append(outputs, chainEvents) }
  }
  if len(outputs) != len(expectedEvents) {
    newBlockNums := make([][]uint64, len(outputs))
    revertedBlockNums := make([][]uint64, len(outputs))
    for i, ces := range outputs {
      newBlockNums[i] = make([]uint64, len(ces.New))
      revertedBlockNums[i] = make([]uint64, len(ces.Reverted))
      for j, ce := range ces.New {
        newBlockNums[i][j] = ce.Block.NumberU64()
      }
      for j, ce := range ces.Reverted {
        revertedBlockNums[i][j] = ce.Block.NumberU64()
      }
    }
    t.Fatalf("Expected %v outputs, got %v (%v / %v) (skipped: %v) (finished: %v) (oldFinished: %v)", len(expectedEvents), len(outputs), newBlockNums, revertedBlockNums, len(consumer.cet.skipped), len(consumer.cet.finished), len(consumer.cet.oldFinished))
  }
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
  return consumer
}
func shuffledTester(t *testing.T, messages []chainEventMessage, expectedEvents []*ChainEvent, lastEmitted common.Hash) *KafkaEventConsumer {
  outputs := []*ChainEvent{}
  consumer, _, close := getTestConsumer(lastEmitted)
  defer close()
  for i, msg := range messages {
    chainEvents, err := consumer.cet.HandleMessage(msg.key, msg.value, 0, int64(i))
    if err != nil { t.Errorf("Error handling message: %v", err) }
    if chainEvents != nil {
      outputs = append(outputs, chainEvents.New...)
      if len(chainEvents.Reverted) > 0 { t.Errorf("Got reverted blocks, expected none") }
    }
  }
  if len(outputs) != len(expectedEvents) {
    blockNums := make([]uint64, len(outputs))
    for i, ce := range outputs {
      blockNums[i] = ce.Block.NumberU64()
    }
    t.Fatalf("Expected %v outputs, got %v (%v) (skipped: %v) (finished: %v) (oldFinished: %v)", len(expectedEvents), len(outputs), blockNums, len(consumer.cet.skipped), len(consumer.cet.finished), len(consumer.cet.oldFinished))
  }
  for i, chainEvent := range expectedEvents {
    if chainEvent.Block.Hash() != outputs[i].Block.Hash() { t.Errorf("Got new chain events out of order o[%v]", i) }
  }
  return consumer
}

func TestReorg(t *testing.T) {
  a := getTestChainEvent(0, 0, nil)
  b := getTestChainEvent(1, 0, a.Block.Header())
  c := getTestChainEvent(1, 1, a.Block.Header())
  d := getTestChainEvent(2, 1, c.Block.Header())
  e := getTestChainEvent(2, 0, b.Block.Header())
  f := getTestChainEvent(3, 0, e.Block.Header())
  g := getTestChainEvent(2, 2, b.Block.Header())
  h := getTestChainEvent(3, 1, g.Block.Header())
  i := getTestChainEvent(4, 0, h.Block.Header())

  // g := getTestChainEvent(4, 0, f.Block.Header())
  if b.Block.ParentHash() != a.Block.Hash() { t.Fatalf("b should be child of a") }
  if c.Block.ParentHash() != a.Block.Hash() { t.Fatalf("c should be child of a") }
  if d.Block.ParentHash() != c.Block.Hash() { t.Fatalf("d should be child of c") }
  if e.Block.ParentHash() != b.Block.Hash() { t.Fatalf("e should be child of b") }
  if f.Block.ParentHash() != e.Block.Hash() { t.Fatalf("f should be child of e") }
  if g.Block.ParentHash() != b.Block.Hash() { t.Fatalf("g should be child of b") }
  if h.Block.ParentHash() != g.Block.Hash() { t.Fatalf("h should be child of g") }
  if i.Block.ParentHash() != h.Block.Hash() { t.Fatalf("i should be child of h") }
  // if g.Block.ParentHash() != f.Block.Hash() { t.Fatalf("g should be child of e") }

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
      common.Hash{},
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
      common.Hash{},
    )
  })
  t.Run("Reorg ACDB", func(t *testing.T) {
    reorgTester(
      t,
      append(append(append(a.getMessages(), c.getMessages()...), d.getMessages()...), b.getMessages()...),
      []*ChainEvents{
        &ChainEvents{New: []*ChainEvent{a}},
        &ChainEvents{New: []*ChainEvent{c}},
        &ChainEvents{New: []*ChainEvent{d}},
      },
      common.Hash{},
    )
  })
  t.Run("Reorg ACDB", func(t *testing.T) {
    reorgTester(
      t,
      append(append(append(a.getMessages(), c.getMessages()...), d.getMessages()...), b.getMessages()...),
      []*ChainEvents{
        &ChainEvents{New: []*ChainEvent{a}},
        &ChainEvents{New: []*ChainEvent{c}},
        &ChainEvents{New: []*ChainEvent{d}},
      },
      common.Hash{},
    )
  })
  t.Run("Reorg ABCDEF", func(t *testing.T) {
    reorgTester(
      t,
      append(append(append(append(append(a.getMessages(), b.getMessages()...), c.getMessages()...), d.getMessages()...), e.getMessages()...), f.getMessages()...),
      []*ChainEvents{
        &ChainEvents{New: []*ChainEvent{a}},
        &ChainEvents{New: []*ChainEvent{b}},
        &ChainEvents{New: []*ChainEvent{c, d}, Reverted: []*ChainEvent{b}},
        &ChainEvents{New: []*ChainEvent{b, e, f}, Reverted: []*ChainEvent{c, d}},
      },
      common.Hash{},
    )
  })
  t.Run("Reorg AEGFBHI", func(t *testing.T) {
    glogger := gethLog.NewGlogHandler(gethLog.StreamHandler(os.Stderr, gethLog.TerminalFormat(false)))
    glogger.Verbosity(gethLog.LvlCrit)
    glogger.Vmodule("")
    gethLog.Root().SetHandler(glogger)
    reorgTester(
      t,
      append(append(append(append(append(append(a.getMessages(), e.getMessages()...), g.getMessages()...), f.getMessages()...), b.getMessages()...), h.getMessages()...), i.getMessages()...),
      []*ChainEvents{
        &ChainEvents{New: []*ChainEvent{a}},
        &ChainEvents{New: []*ChainEvent{b, e, f}},
        &ChainEvents{New: []*ChainEvent{g, h, i}, Reverted:[]*ChainEvent{e, f}},
      },
      common.Hash{},
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
      common.Hash{},
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

func TestDelayedMessages(t *testing.T) {
  glogger := gethLog.NewGlogHandler(gethLog.StreamHandler(os.Stderr, gethLog.TerminalFormat(false)))
  glogger.Verbosity(gethLog.LvlCrit)
  glogger.Vmodule("")
  gethLog.Root().SetHandler(glogger)


  ces := make([]*ChainEvent, 512)
  ces[0] = getTestChainEvent(0, 0, nil)
  for i := 1; i < cap(ces); i++ {
    ces[i] = getTestChainEvent(int64(i), 0, ces[i-1].Block.Header())
  }
  messages := []chainEventMessage{}
  for i := 0 ; i < 256; i ++ {
    messages = append(messages, ces[i].getMessages()...)
  }
  for i := 0 ; i < 256; i ++ {
    messages = append(messages, ces[i].getMessages()...)
  }
  for i := 256 ; i < len(ces); i ++ {
    messages = append(messages, ces[i].getMessages()...)
  }
  outputs := make([]*ChainEvents, len(ces))
  for i, ce := range ces {
    outputs[i] = &ChainEvents{New: []*ChainEvent{ce}}
  }
  t.Run("Test big replay", func(t *testing.T) {
    consumer := reorgTester(
      t,
      messages,
      outputs,
      common.Hash{},
    )
    if len(consumer.cet.skipped) != 127 {
      t.Errorf("Expected 256 skipped items, got %v", len(consumer.cet.skipped))
    }
    if len(consumer.cet.chainEvents) > consumer.cet.finishedLimit * 2{
      t.Errorf("Expected no more than %v tracked events, got %v", consumer.cet.finishedLimit * 2, len(consumer.cet.chainEvents))
    }
  })
  t.Run("Test big replay scrambled", func(t *testing.T) {
    rand.Seed(time.Now().UnixNano())
    rand.Shuffle(len(messages), func(i, j int) { messages[i], messages[j] = messages[j], messages[i] })
    o2 := make([]*ChainEvent, len(outputs[1:]))
    for i, ce := range outputs[1:] {
      o2[i] = ce.New[0]
    }
    shuffledTester(
      t,
      messages,
      o2,
      ces[0].Block.Hash(),
    )
  })
  t.Run("Test delayed Reorg BCDEFA", func(t *testing.T) {
    gethLog.Debug("---- Reorg BCDEFA ----")
    x := getTestChainEvent(0, 0, nil)
    a := getTestChainEvent(1, 0, x.Block.Header())
    b := getTestChainEvent(2, 0, a.Block.Header())
    c := getTestChainEvent(2, 1, a.Block.Header())
    d := getTestChainEvent(3, 1, c.Block.Header())
    e := getTestChainEvent(3, 0, b.Block.Header())
    f := getTestChainEvent(4, 0, e.Block.Header())
    reorgTester(
      t,
      append(append(append(append(append(b.getMessages(), c.getMessages()...), d.getMessages()...), e.getMessages()...), f.getMessages()...), a.getMessages()...),
      []*ChainEvents{
        &ChainEvents{New: []*ChainEvent{a, b, e, f}},
      },
      a.Block.ParentHash(),
    )
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
