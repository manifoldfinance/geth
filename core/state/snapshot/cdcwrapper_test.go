package snapshot

import (
  "bytes"
  "github.com/ethereum/go-ethereum/event"
  "github.com/ethereum/go-ethereum/core/rawdb"
  "github.com/ethereum/go-ethereum/common"
  "github.com/VictoriaMetrics/fastcache"
  "testing"
  "runtime"
)


type mockStateDeltaEmitter struct {
  feed event.Feed
}

func (e *mockStateDeltaEmitter) Emit(m []stateDeltaMessage) error {
  e.feed.Send(m)
  return nil
}

func getMockCDCTrie() (*CDCTree, chan []stateDeltaMessage) {
  base := &diskLayer{
		diskdb: rawdb.NewMemoryDatabase(),
		root: common.HexToHash("0x01"),
		cache: fastcache.New(1024 * 500),
	}
	snaps := &Tree{
		layers: map[common.Hash]snapshot{
			base.root: base,
		},
	}
	// Retrieve a reference to the base and commit a diff on top
  emitter := &mockStateDeltaEmitter{}
  ch := make(chan []stateDeltaMessage, 2)
  emitter.feed.Subscribe(ch)
  return &CDCTree{tree: snaps, emitter: emitter}, ch
}

func getDeltaTracker() (*deltaTracker, SnapshotTree) {
  base := &diskLayer{
		diskdb: rawdb.NewMemoryDatabase(),
		root: common.HexToHash("0x01"),
		cache: fastcache.New(1024 * 500),
	}
	snaps := &Tree{
		layers: map[common.Hash]snapshot{
			base.root: base,
		},
	}
  dt := &deltaTracker{
    tree: snaps,
    deltas: make(map[common.Hash]*delta),
    pendingEmits: make(map[common.Hash][]common.Hash),
    deltaPartitions: make(map[int32]int64),
    finished: make(map[common.Hash]bool),
    oldFinished: make(map[common.Hash]bool),
    finishedLimit: 128,
    earlyDestructs: make(map[common.Hash]map[common.Hash]struct{}),
    earlyAccounts: make(map[common.Hash]map[common.Hash][]byte),
    earlyStorage: make(map[common.Hash]map[common.Hash]map[common.Hash][]byte),
  }
  dt.finished[common.HexToHash("0x01")] = true
  return dt, snaps
}

func getEmptyMockCDCTrie() (*CDCTree, chan []stateDeltaMessage) {
  emitter := &mockStateDeltaEmitter{}
  ch := make(chan []stateDeltaMessage, 2)
  emitter.feed.Subscribe(ch)
  return &CDCTree{tree: nil, emitter: emitter}, ch
}


func TestUnpopulatedCDCTreeJournal(t *testing.T) {
  tree, ch := getEmptyMockCDCTrie()
  if _, err := tree.Journal(common.HexToHash("0x01")); err != nil { t.Errorf(err.Error()) }
  select {
  case <-ch:
    t.Errorf("Unexpected channel message")
  default:
  }
}
func TestUnpopulatedCDCTreeRebuild(t *testing.T) {
  tree, ch := getEmptyMockCDCTrie()
  tree.Rebuild(common.HexToHash("0x01"))
  select {
  case <-ch:
    t.Errorf("Unexpected channel message")
  default:
  }
}
func TestPopulatedCDCTreeAccountIterator(t *testing.T) {
  tree, ch := getMockCDCTrie()
  if _, err :=tree.AccountIterator(common.HexToHash("0x01"), common.Hash{}); err != nil {
    t.Errorf(err.Error())
  }
  select {
  case <-ch:
    t.Errorf("Unexpected channel message")
  default:
  }
}
func TestUnpopulatedCDCTreeAccountIterator(t *testing.T) {
  tree, ch := getEmptyMockCDCTrie()
  if _, err := tree.AccountIterator(common.HexToHash("0x01"), common.Hash{}); err == nil {
    t.Errorf("Expected iterator error")
  }
  select {
  case <-ch:
    t.Errorf("Unexpected channel message")
  default:
  }
}
func TestUnpopulatedCDCTreeStorageIterator(t *testing.T) {
  tree, ch := getEmptyMockCDCTrie()
  if _, err := tree.StorageIterator(common.HexToHash("0x01"), common.HexToHash("0x00"), common.Hash{}); err == nil {
    t.Errorf("Expected error")
  }
  select {
  case <-ch:
    t.Errorf("Unexpected channel message")
  default:
  }
}
func TestPopulatedCDCTreeStorageIterator(t *testing.T) {
  tree, ch := getMockCDCTrie()
  if _, err := tree.StorageIterator(common.HexToHash("0x01"), common.HexToHash("0x00"), common.Hash{}); err != nil {
    t.Errorf(err.Error())
  }
  select {
  case <-ch:
    t.Errorf("Unexpected channel message")
  default:
  }
}
func TestUnpopulatedCDCTreeSnapshot(t *testing.T) {
  tree, ch := getEmptyMockCDCTrie()
  snap := tree.Snapshot(common.HexToHash("0x01"))
  if snap.Root() != common.HexToHash("0x01") {
    t.Errorf("Unexpected hash: %#x", snap.Root())
  }
  select {
  case <-ch:
    t.Errorf("Unexpected channel message")
  default:
  }
}
func TestPopulatedCDCTreeSnapshot(t *testing.T) {
  tree, ch := getMockCDCTrie()
  snap := tree.Snapshot(common.HexToHash("0x01"))
  if snap.Root() != common.HexToHash("0x01") {
    t.Errorf("Unexpected hash: %#x", snap.Root())
  }
  select {
  case <-ch:
    t.Errorf("Unexpected channel message")
  default:
  }
}
func TestUnpopulatedCDCTreeUpdate(t *testing.T) {
  tree, ch := getEmptyMockCDCTrie()
  tree.Update(common.HexToHash("0x02"), common.HexToHash("0x01"), map[common.Hash]struct{}{common.HexToHash("0xff"): struct{}{}}, map[common.Hash][]byte{common.HexToHash("0xEE"): []byte{0, 1, 2}}, map[common.Hash]map[common.Hash][]byte{common.HexToHash("0xEE"): map[common.Hash][]byte{common.HexToHash("AA"): []byte{20, 30, 40}}})
  select {
  case msgs := <-ch:
    if MsgType(msgs[0].key[0]) != DeltaMsg { t.Errorf("Unexpected message type: %v", msgs[0].key[0]) }
    if MsgType(msgs[1].key[0]) != DestructMsg { t.Errorf("Unexpected message type: %v", msgs[1].key[0]) }
    if MsgType(msgs[2].key[0]) != AccountMsg { t.Errorf("Unexpected message type: %v", msgs[2].key[0]) }
    if MsgType(msgs[3].key[0]) != StorageMsg { t.Errorf("Unexpected message type: %v", msgs[3].key[0]) }
  default:
    t.Errorf("Channel message missing")
  }
}
func TestPopulatedCDCTreeUpdate(t *testing.T) {
  tree, ch := getMockCDCTrie()
  tree.Update(common.HexToHash("0x02"), common.HexToHash("0x01"), map[common.Hash]struct{}{common.HexToHash("0xff"): struct{}{}}, map[common.Hash][]byte{common.HexToHash("0xEE"): []byte{0, 1, 2}}, map[common.Hash]map[common.Hash][]byte{common.HexToHash("0xEE"): map[common.Hash][]byte{common.HexToHash("AA"): []byte{20, 30, 40}}})
  select {
  case msgs := <-ch:
    if MsgType(msgs[0].key[0]) != DeltaMsg { t.Errorf("Unexpected message type: %v", msgs[0].key[0]) }
    if MsgType(msgs[1].key[0]) != DestructMsg { t.Errorf("Unexpected message type: %v", msgs[1].key[0]) }
    if MsgType(msgs[2].key[0]) != AccountMsg { t.Errorf("Unexpected message type: %v", msgs[2].key[0]) }
    if MsgType(msgs[3].key[0]) != StorageMsg { t.Errorf("Unexpected message type: %v", msgs[3].key[0]) }
  default:
    t.Errorf("Channel message missing")
  }
}
func TestUnpopulatedCDCTreeCap(t *testing.T) {
  tree, ch := getEmptyMockCDCTrie()
  tree.Cap(common.HexToHash("0x01"), 10)
  select {
  case <-ch:
    t.Errorf("Unexpected channel message")
  default:
  }
}
func TestPopulatedCDCTreeCap(t *testing.T) {
  tree, ch := getMockCDCTrie()
  tree.Cap(common.HexToHash("0x01"), 10)
  select {
  case <-ch:
    t.Errorf("Unexpected channel message")
  default:
  }
}


func TestDeltaTracker(t *testing.T) {
  tree, ch := getMockCDCTrie()
  tree.Update(
    common.HexToHash("0x02"),
    common.HexToHash("0x01"),
    map[common.Hash]struct{}{},
    map[common.Hash][]byte{common.HexToHash("0xEE"): []byte{0, 1, 2}},
    map[common.Hash]map[common.Hash][]byte{common.HexToHash("0xEE"): map[common.Hash][]byte{common.HexToHash("0xAA"): []byte{20, 30, 40}}},
  )
  tree.Update(
    common.HexToHash("0x03"),
    common.HexToHash("0x02"),
    map[common.Hash]struct{}{},
    map[common.Hash][]byte{common.HexToHash("0xEF"): []byte{0, 1, 3}},
    map[common.Hash]map[common.Hash][]byte{common.HexToHash("0xEF"): map[common.Hash][]byte{common.HexToHash("0xAA"): []byte{20, 30, 40}}},
  )
  messages := []stateDeltaMessage{}
  runtime.Gosched()
  MESSAGES:
  for {
    select {
    case msgs := <-ch:
      messages = append(messages, msgs...)
      runtime.Gosched()
    default:
      break MESSAGES
    }
  }
  if len(messages) != 6 {
    t.Fatalf("Unexpected message count %v", len(messages))
  }
  dtTester := func(t *testing.T, messages []stateDeltaMessage, order []int) {
    dt, tree2 := getDeltaTracker()
    emitCount := 0
    for j, i := range order {
      emitted, err := dt.handleMessage(messages[i].key, messages[i].value, 0, int64(j))
      if err != nil { t.Errorf("error handling message: %v", err.Error()) }
      if emitted { emitCount++ }
    }
    if emitCount != 2 { t.Errorf("Emitted unexpected number of entries: %v", emitCount)}
    snap := tree2.Snapshot(common.HexToHash("0x02"))
    if snap == nil {t.Fatalf("Got nil snap for hash")}
    data, err := snap.Storage(common.HexToHash("0xEE"), common.HexToHash("0xAA"))
    if err != nil { t.Errorf(err.Error()) }
    if !bytes.Equal(data, []byte{20, 30, 40}) {
      t.Errorf("Unexpected storage value %#x", data)
    }
    data, err = snap.Storage(common.HexToHash("0xEF"), common.HexToHash("0xAA"))
    if len(data) != 0 { t.Errorf("Expected hash to be missing from this root")}
    data, err = snap.AccountRLP(common.HexToHash("0xEE"))
    if !bytes.Equal(data, []byte{0, 1, 2}) {
      t.Errorf("Unexpected account value %#x", data)
    }
    snap = tree2.Snapshot(common.HexToHash("0x03"))
    data, err = snap.Storage(common.HexToHash("0xEE"), common.HexToHash("0xAA"))
    if err != nil { t.Errorf(err.Error()) }
    if !bytes.Equal(data, []byte{20, 30, 40}) {
      t.Errorf("Unexpected storage value %#x", data)
    }
    data, err = snap.AccountRLP(common.HexToHash("0xEE"))
    if !bytes.Equal(data, []byte{0, 1, 2}) {
      t.Errorf("Unexpected account value %#x", data)
    }
    data, err = snap.Storage(common.HexToHash("0xEF"), common.HexToHash("0xAA"))
    if err != nil { t.Errorf(err.Error()) }
    if !bytes.Equal(data, []byte{20, 30, 40}) {
      t.Errorf("Unexpected storage value %#x", data)
    }
    data, err = snap.AccountRLP(common.HexToHash("0xEF"))
    if !bytes.Equal(data, []byte{0, 1, 3}) {
      t.Errorf("Unexpected account value %#x", data)
    }
  }
  dtTester(t, messages, []int{0, 1, 2, 3, 4, 5})
  dtTester(t, messages, []int{5, 4, 3, 2, 1, 0})
  dtTester(t, messages, []int{1, 0, 1, 5, 2, 3, 2, 4, 5})
}
