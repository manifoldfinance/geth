package filters

import (
	"context"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/rpc"

)

type dropNotification struct {
	TxHash common.Hash `json:"txhash"`
	Reason string `json:"reason"`
}

type rejectNotification struct {
	Tx *ethapi.RPCTransaction `json:"tx"`
	Reason string `json:"reason"`
}

// newRPCTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func newRPCPendingTransaction(tx *types.Transaction) *ethapi.RPCTransaction {
	var signer types.Signer = types.FrontierSigner{}
	if tx.Protected() {
		signer = types.NewEIP155Signer(tx.ChainId())
	}
	from, _ := types.Sender(signer, tx)
	v, r, s := tx.RawSignatureValues()

	result := &ethapi.RPCTransaction{
		From:     from,
		Gas:      hexutil.Uint64(tx.Gas()),
		GasPrice: (*hexutil.Big)(tx.GasPrice()),
		Hash:     tx.Hash(),
		Input:    hexutil.Bytes(tx.Data()),
		Nonce:    hexutil.Uint64(tx.Nonce()),
		To:       tx.To(),
		Value:    (*hexutil.Big)(tx.Value()),
		V:        (*hexutil.Big)(v),
		R:        (*hexutil.Big)(r),
		S:        (*hexutil.Big)(s),
	}
	return result
}


// DroppedTransactions send a notification each time a transaction is dropped from the mempool
func (api *PublicFilterAPI) DroppedTransactions(ctx context.Context) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		dropped := make(chan core.DropTxsEvent)
		droppedSub := api.backend.SubscribeDropTxsEvent(dropped)

		for {
			select {
			case d := <-dropped:
				for _, tx := range d.Txs {
					notifier.Notify(rpcSub.ID, &dropNotification{TxHash: tx.Hash(), Reason: d.Reason})
				}
			case <-rpcSub.Err():
				droppedSub.Unsubscribe()
				return
			case <-notifier.Closed():
				droppedSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

// RejectedTransactions send a notification each time a transaction is rejected from entering the mempool
func (api *PublicFilterAPI) RejectedTransactions(ctx context.Context) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		rejected := make(chan core.RejectedTxEvent)
		rejectedSub := api.backend.SubscribeRejectedTxEvent(rejected)

		for {
			select {
			case d := <-rejected:
				reason := ""
				if d.Reason != nil {
					reason = d.Reason.Error()
				}
				notifier.Notify(rpcSub.ID, &rejectNotification{Tx: newRPCPendingTransaction(d.Tx), Reason: reason})
			case <-rpcSub.Err():
				rejectedSub.Unsubscribe()
				return
			case <-notifier.Closed():
				rejectedSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}
