// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package miner implements Ethereum block creation and mining.
package miner

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params/types/ctypes"
	"github.com/ethereum/go-ethereum/params/vars"
)

// Backend wraps all methods required for mining.
type Backend interface {
	BlockChain() *core.BlockChain
	TxPool() *core.TxPool
}

// Config is the configuration parameters of mining.
type Config struct {
	Etherbase common.Address `toml:",omitempty"` // Public address for block mining rewards (default = first account)
	Notify    []string       `toml:",omitempty"` // HTTP URL list to be notified of new work packages(only useful in ethash).
	ExtraData hexutil.Bytes  `toml:",omitempty"` // Block extra data set by the miner
	GasFloor  uint64         // Target gas floor for mined blocks.
	GasCeil   uint64         // Target gas ceiling for mined blocks.
	GasPrice  *big.Int       // Minimum gas price for mining a transaction
	Recommit  time.Duration  // The time interval for miner to re-create mining work.
	Noverify  bool           // Disable remote mining solution verification(only useful in ethash).
}

// Miner creates blocks and searches for proof-of-work values.
type Miner struct {
	mux      *event.TypeMux
	worker   *worker
	coinbase common.Address
	eth      Backend
	engine   consensus.Engine
	exitCh   chan struct{}
	startCh  chan common.Address
	stopCh   chan struct{}
}

func New(eth Backend, config *Config, chainConfig ctypes.ChainConfigurator, mux *event.TypeMux, engine consensus.Engine, isLocalBlock func(block *types.Block) bool) *Miner {
	miner := &Miner{
		eth:     eth,
		mux:     mux,
		engine:  engine,
		exitCh:  make(chan struct{}),
		startCh: make(chan common.Address),
		stopCh:  make(chan struct{}),
		worker:  newWorker(config, chainConfig, engine, eth, mux, isLocalBlock, true),
	}
	go miner.update()

	return miner
}

// update handles miner start/stop/exit events, as well as a temporary subscription to downloader events.
// Downloader events are used to pause and restart mining pending a successful sync operation.
func (miner *Miner) update() {
	downloaderEvents := miner.mux.Subscribe(downloader.StartEvent{}, downloader.DoneEvent{}, downloader.FailedEvent{})
	defer func() {
		if !downloaderEvents.Closed() {
			downloaderEvents.Unsubscribe()
		}
	}()

	miningRequested := false
	canStart := true

	// handleDownloaderEvents handles the downloader events. Please be aware that this is a one-shot type of logic.
	// As soon as `Done` has been broadcast subsequent downloader events are not handled, and the event subscription is closed.
	//
	// This prevents a major security vulnerability where external parties can DOS you with blocks
	// and halt your mining operation for as long as the DOS continues.
	handleDownloaderEvents := func(ev *event.TypeMuxEvent) {
		switch ev.Data.(type) {
		case downloader.StartEvent:
			wasMining := miner.Mining()
			miner.worker.stop()
			canStart = false
			if wasMining {
				// Resume mining after sync was finished
				miningRequested = true
				log.Info("Mining aborted due to sync")
			}
		case downloader.FailedEvent:
			canStart = true
			if miningRequested {
				miner.SetEtherbase(miner.coinbase)
				miner.worker.start()
			}
		case downloader.DoneEvent:
			canStart = true
			if miningRequested {
				miner.SetEtherbase(miner.coinbase)
				miner.worker.start()
			}
			downloaderEvents.Unsubscribe()
		}
	}

	// Handle downloader events while subscription is open.
	go func() {
		for ev := range downloaderEvents.Chan() {
			if ev == nil {
				return
			}
			handleDownloaderEvents(ev)
		}
	}()

	for {
		select {
		case addr := <-miner.startCh:
			if canStart {
				miner.SetEtherbase(addr)
				miner.worker.start()
			}
			miningRequested = true
		case <-miner.stopCh:
			miningRequested = false
			miner.worker.stop()
		case <-miner.exitCh:
			miner.worker.close()
			return
		}
	}
}

func (miner *Miner) Start(coinbase common.Address) {
	miner.startCh <- coinbase
}

func (miner *Miner) Stop() {
	miner.stopCh <- struct{}{}
}

func (miner *Miner) Close() {
	close(miner.exitCh)
}

func (miner *Miner) Mining() bool {
	return miner.worker.isRunning()
}

func (miner *Miner) HashRate() uint64 {
	if pow, ok := miner.engine.(consensus.PoW); ok {
		return uint64(pow.Hashrate())
	}
	return 0
}

func (miner *Miner) SetExtra(extra []byte) error {
	if uint64(len(extra)) > vars.MaximumExtraDataSize {
		return fmt.Errorf("extra exceeds max length. %d > %v", len(extra), vars.MaximumExtraDataSize)
	}
	miner.worker.setExtra(extra)
	return nil
}

// SetRecommitInterval sets the interval for sealing work resubmitting.
func (miner *Miner) SetRecommitInterval(interval time.Duration) {
	miner.worker.setRecommitInterval(interval)
}

// Pending returns the currently pending block and associated state.
func (miner *Miner) Pending() (*types.Block, *state.StateDB) {
	return miner.worker.pending()
}

// PendingBlock returns the currently pending block.
//
// Note, to access both the pending block and the pending state
// simultaneously, please use Pending(), as the pending state can
// change between multiple method calls
func (miner *Miner) PendingBlock() *types.Block {
	return miner.worker.pendingBlock()
}

func (miner *Miner) SetEtherbase(addr common.Address) {
	miner.coinbase = addr
	miner.worker.setEtherbase(addr)
}

// EnablePreseal turns on the preseal mining feature. It's enabled by default.
// Note this function shouldn't be exposed to API, it's unnecessary for users
// (miners) to actually know the underlying detail. It's only for outside project
// which uses this library.
func (miner *Miner) EnablePreseal() {
	miner.worker.enablePreseal()
}

// DisablePreseal turns off the preseal mining feature. It's necessary for some
// fake consensus engine which can seal blocks instantaneously.
// Note this function shouldn't be exposed to API, it's unnecessary for users
// (miners) to actually know the underlying detail. It's only for outside project
// which uses this library.
func (miner *Miner) DisablePreseal() {
	miner.worker.disablePreseal()
}

// SubscribePendingLogs starts delivering logs from pending transactions
// to the given channel.
func (miner *Miner) SubscribePendingLogs(ch chan<- []*types.Log) event.Subscription {
	return miner.worker.pendingLogsFeed.Subscribe(ch)
}
