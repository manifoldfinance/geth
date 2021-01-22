#!/bin/bash
ALLOWED_ORIGINS="*"
WS_APIS=eth,net,web3
while true
do
    if [ ! `pgrep geth` ] ; then
        geth --ethstats "$INSTANCE_NAME:$WS_SECRET@$WS_SERVER:3000" --cache=512 --rpcvhosts='*' --rpc --rpcaddr '0.0.0.0' --rpccorsdomain '*' --ws --wsapi $WS_APIS --wsaddr 0.0.0.0 --wsorigins $ALLOWED_ORIGINS
    fi
    sleep 10
done
