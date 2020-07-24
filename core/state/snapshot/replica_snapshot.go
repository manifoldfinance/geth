package snapshot

import (
	"fmt"
	"errors"
	"math/big"
	"bytes"
	"github.com/VictoriaMetrics/fastcache"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/log"
)


func accountData(value []byte) ([]byte, common.Hash, error) {
  var acc struct {
    Nonce    uint64
    Balance  *big.Int
    Root     common.Hash
    CodeHash []byte
  }
  if err := rlp.DecodeBytes(value, &acc); err != nil {
    return nil, common.Hash{}, err
  }
  return SlimAccountRLP(acc.Nonce, acc.Balance, acc.Root, acc.CodeHash), acc.Root, nil
}

type compareRoots struct {
  old common.Hash
  new common.Hash
  account common.Hash
}

// Update(blockRoot common.Hash, parentRoot common.Hash, destructs map[common.Hash]struct{}, accounts map[common.Hash][]byte, storage map[common.Hash]map[common.Hash][]byte)

// DiffTries returns the destructs, accounts, and storage maps to transition the
// trie at blockRoot into the trie at parentRoot.
func DiffTries(db ethdb.KeyValueStore, blockRoot, parentRoot common.Hash) (map[common.Hash]struct{}, map[common.Hash][]byte, map[common.Hash]map[common.Hash][]byte, error) {
  tdb := trie.NewDatabase(db)
  blockTrie, err := trie.New(blockRoot, tdb)
  if err != nil { return nil, nil, nil, err}
  parentTrie, err := trie.New(parentRoot, tdb)
  if err != nil { return nil, nil, nil, err}
  destructs := make(map[common.Hash]struct{})
  accounts := make(map[common.Hash][]byte)
  accountRoots := []compareRoots{}
  blockIterator := blockTrie.NodeIterator([]byte{})
  parentIterator := parentTrie.NodeIterator([]byte{})
  blockNext := blockIterator.Next(true)
  parentNext := parentIterator.Next(true)
  for blockNext && parentNext {
    for blockNext && parentNext && bytes.Compare(blockIterator.Path(), parentIterator.Path()) < 0 {
      // blockIterator is behind parentIterator. Any leaves we find before
      // catching up are new accounts
  		if blockIterator.Leaf() {
				log.Info("Found new account", "path", blockIterator.Path(), "parentPath", parentIterator.Path())
        var root common.Hash
        accounts[common.BytesToHash(blockIterator.LeafKey())], root, err = accountData(blockIterator.LeafBlob())
        if err != nil { return nil, nil, nil, err }
        accountRoots = append(accountRoots, compareRoots{new: root, account: common.BytesToHash(blockIterator.LeafKey())})
      }
			log.Info("Advancing block iterator", "path", blockIterator.Path(), "parentPath", parentIterator.Path())
      blockNext = blockIterator.Next(true)
  	}
    for blockNext && parentNext && bytes.Compare(blockIterator.Path(), parentIterator.Path()) == 0 {
      if bytes.Compare(blockIterator.Hash().Bytes(), parentIterator.Hash().Bytes()) == 0 {
        // Hashes match, no difference between subtrees, no need to compare
        // further
				if blockIterator.Leaf() && bytes.Compare(blockIterator.LeafBlob(), parentIterator.LeafBlob()) != 0 {
					log.Info("Found differing leaf", "path", blockIterator.Path())
          var oldRoot, newRoot common.Hash
          _, oldRoot, err = accountData(parentIterator.LeafBlob())
          if err != nil { return nil, nil, nil, err }
          accounts[common.BytesToHash(blockIterator.LeafKey())], newRoot, err = accountData(blockIterator.LeafBlob())
          if err != nil { return nil, nil, nil, err }
          accountRoots = append(accountRoots, compareRoots{old: oldRoot, new: newRoot, account: common.BytesToHash(blockIterator.LeafKey())})
				} else {
					log.Info("Nodes match, so will their children, advancing both", "path", blockIterator.Path(), "parentPath", parentIterator.Path())
				}
        blockNext = blockIterator.Next(false)
        parentNext = parentIterator.Next(false)
      } else {
        // Paths match but hashes don't, continue exploring the subtree
				log.Info("Paths match, hashes don't, advancing both", "path", blockIterator.Path(), "parentPath", parentIterator.Path(), "hash", blockIterator.Hash(), "parentHash", parentIterator.Hash())
        if blockIterator.Leaf(){
					log.Info("Found differing leaf", "path", blockIterator.Path())
          var oldRoot, newRoot common.Hash
          _, oldRoot, err = accountData(parentIterator.LeafBlob())
          if err != nil { return nil, nil, nil, err }
          accounts[common.BytesToHash(blockIterator.LeafKey())], newRoot, err = accountData(blockIterator.LeafBlob())
          if err != nil { return nil, nil, nil, err }
          accountRoots = append(accountRoots, compareRoots{old: oldRoot, new: newRoot, account: common.BytesToHash(blockIterator.LeafKey())})
        }
        blockNext = blockIterator.Next(true)
        parentNext = parentIterator.Next(true)
      }
    }
    for parentNext && blockNext && bytes.Compare(blockIterator.Path(), parentIterator.Path()) > 0 {
      // parentIterator is behind blockIterator. Any leaves we find before
      // catching up are destructed accounts
      if parentIterator.Leaf() {
				log.Info("Found destruct", "path", blockIterator.Path(), "parentPath", parentIterator.Path())
        destructs[common.BytesToHash(parentIterator.LeafKey())] = struct{}{}
      }
			log.Info("Advancing parent iterator", "path", blockIterator.Path(), "parentPath", parentIterator.Path())
      parentNext = parentIterator.Next(true)
    }
  }
	if err := blockIterator.Error(); err != nil { return nil, nil, nil, err }
	if err := parentIterator.Error(); err != nil { return nil, nil, nil, err }
  for ; blockNext; blockNext = blockIterator.Next(true) {
    // The parent iterator is done, but we still have more in the block. The
    // rest are new accounts.
		log.Info("Parent done, advancing block", "path", blockIterator.Path())
    if blockIterator.Leaf() {
			log.Info("Parent done, found new account", "path", blockIterator.Path())
      var root common.Hash
      accounts[common.BytesToHash(blockIterator.LeafKey())], root, err = accountData(blockIterator.LeafBlob())
      if err != nil { return nil, nil, nil, err }
      accountRoots = append(accountRoots, compareRoots{new: root, account: common.BytesToHash(blockIterator.LeafKey())})
    }
  }
  for ; parentNext; parentNext = parentIterator.Next(true) {
    // The block iterator is done, but we still have more in the parent. The
    // rest are destructed accounts.
    if parentIterator.Leaf() {
			log.Info("Block done, found destruct", "path", parentIterator.Path())
      destructs[common.BytesToHash(parentIterator.LeafKey())] = struct{}{}
    }
  }
	log.Info("Diffing blocks", "destructs", len(destructs), "accounts", len(accounts))
  storage := make(map[common.Hash]map[common.Hash][]byte)
  for _, roots := range accountRoots {
		log.Info("Diffing account", "oldRoot", roots.old, "newRoot", roots.new, "account", roots.account)
    if roots.old == (common.Hash{}) {
      accountTrie, err  := trie.New(roots.new, tdb)
      if err != nil { return nil, nil, nil, err}
      accountIter := accountTrie.NodeIterator([]byte{})
      // Iterate over accountTrie, adding every leaf to storage
      storage[roots.account] = make(map[common.Hash][]byte)
      for accountIter.Next(true) {
        if accountIter.Leaf() {
          storage[roots.account][common.BytesToHash(accountIter.LeafKey())] = accountIter.LeafBlob()
        }
      }
      if err := accountIter.Error(); err != nil { return nil, nil, nil, err}
    } else {
      newTrie, err := trie.New(roots.new, tdb)
      if err != nil { return nil, nil, nil, err}
      oldTrie, err := trie.New(roots.old, tdb)
      if err != nil { return nil, nil, nil, err}
      newIter := newTrie.NodeIterator([]byte{})
      oldIter := oldTrie.NodeIterator([]byte{})
      // Compare new to old, adding any leaves to storage (values from newTrie, empty if present in oldTrie and not in newTrie)
      nextNew := newIter.Next(true)
      nextOld := oldIter.Next(true)
      for nextNew && nextOld {
        for nextNew && nextOld && bytes.Compare(newIter.Path(), oldIter.Path()) < 0 {
          if newIter.Leaf() {
            storage[roots.account][common.BytesToHash(newIter.LeafKey())] = newIter.LeafBlob()
          }
          nextNew = newIter.Next(true)
        }
        for nextNew && nextOld && bytes.Compare(newIter.Path(), oldIter.Path()) == 0 {
          if bytes.Compare(newIter.Hash().Bytes(), oldIter.Hash().Bytes()) == 0 {
            // Hashes match, no difference between subtrees, no need to compare
            // further
            nextNew = newIter.Next(false)
            nextOld = oldIter.Next(false)
          } else {
            // Paths match but hashes don't, continue exploring the subtree
            if newIter.Leaf(){
              storage[roots.account][common.BytesToHash(newIter.LeafKey())] = newIter.LeafBlob()
            }
            nextNew = newIter.Next(false)
            nextOld = oldIter.Next(false)
          }
        }
        for nextNew && nextOld && bytes.Compare(newIter.Path(), oldIter.Path()) > 0 {
          if oldIter.Leaf() {
            // It exists in old but not new, so it's been zeroed out
            storage[roots.account][common.BytesToHash(oldIter.LeafKey())] = []byte{}
          }
          nextOld = oldIter.Next(true)
        }
      }
      for ; nextNew ; nextNew = newIter.Next(true) {
        if newIter.Leaf() {
          storage[roots.account][common.BytesToHash(newIter.LeafKey())] = newIter.LeafBlob()
        }
      }
      for ; nextOld ; nextOld = oldIter.Next(true) {
        if oldIter.Leaf() {
          storage[roots.account][common.BytesToHash(oldIter.LeafKey())] = []byte{}
        }
      }
    }
  }
  return destructs, accounts, storage, nil
}

func LoadDiskLayerSnapshot(diskdb ethdb.Database, triedb *trie.Database, cache int) (*Tree, error) {
	baseRoot := rawdb.ReadSnapshotRoot(diskdb)
	if baseRoot == (common.Hash{}) {
		return nil, errors.New("missing or corrupted snapshot")
	}
	snap := &Tree{
		diskdb: diskdb,
		triedb: triedb,
		cache:  cache,
		layers: make(map[common.Hash]snapshot),
	}
	base := &diskLayer{
		diskdb: diskdb,
		triedb: triedb,
		cache:  fastcache.New(cache * 1024 * 1024),
		root:   baseRoot,
	}
	snap.layers[baseRoot] = base
	return snap, nil
}

func LoadSnapshotWithoutJournal(diskdb ethdb.Database, triedb *trie.Database, cache int) (*Tree, error) {
	baseRoot := rawdb.ReadSnapshotRoot(diskdb)
	if baseRoot == (common.Hash{}) {
		return nil, errors.New("missing or corrupted snapshot")
	}
	snap := &Tree{
		diskdb: diskdb,
		triedb: triedb,
		cache:  cache,
		layers: make(map[common.Hash]snapshot),
	}
	base := &diskLayer{
		diskdb: diskdb,
		triedb: triedb,
		cache:  fastcache.New(cache * 1024 * 1024),
		root:   baseRoot,
	}
	snap.layers[baseRoot] = base

	headHash := rawdb.ReadHeadHeaderHash(diskdb)
	headNumber := *(rawdb.ReadHeaderNumber(diskdb, headHash))
	header := rawdb.ReadHeader(diskdb, headHash, headNumber)
	headers := []types.Header{*header}
	limit := 256
	for header.Root != baseRoot {
		header = rawdb.ReadHeader(diskdb, header.ParentHash, header.Number.Uint64() - 1)
		headers = append([]types.Header{*header}, headers...)
		if len(headers) > limit { return nil, fmt.Errorf("No header matching root %#x within limit", baseRoot)}
	}
	log.Info("Loaded disk layer", "diffLayers", len(headers), "hash", header.Hash(), "num", header.Number.Uint64(), "root", header.Root)
	for i := 1; i < len(headers); i++ {
		destructs, accounts, storage, err := DiffTries(diskdb, headers[i].Root, headers[i-1].Root)
		log.Info("Diffed tries", "layer", i, "root", headers[i].Root, "parentRoot", headers[i-1].Root, "err", err)
		if err != nil { return nil, err }
		snap.Update(headers[i-1].Root, headers[i].Root, destructs, accounts, storage)
	}
	return snap, nil
}
