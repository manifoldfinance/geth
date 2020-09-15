package snapshot

import (
  "github.com/ethereum/go-ethereum/event"
  "github.com/ethereum/go-ethereum/core/rawdb"
  "github.com/ethereum/go-ethereum/common"
  "github.com/VictoriaMetrics/fastcache"
  "testing"
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
		root:   common.HexToHash("0x01"),
		cache:  fastcache.New(1024 * 500),
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
