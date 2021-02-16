package replica

import (
  "github.com/ethereum/go-ethereum/core"
  "github.com/ethereum/go-ethereum/core/types"
  "github.com/ethereum/go-ethereum/event"
)

type EventProducer interface {
  Emit(chainEvent core.ChainEvent) error
  RelayEvents(bc ChainEventSubscriber)
  Close()
}

type EventConsumer interface {
  SubscribeChainEvents(ch chan<- *ChainEvents) event.Subscription
  Start()
  Ready() chan struct{}
  Close()
}

type TransactionProducer interface {
  Emit(marshall(*types.Transaction)) error
  RelayTransactions(*core.TxPool)
  Close()
}

type TransactionConsumer interface {
  Messages() <-chan *types.Transaction
  Close()
}
