/*
 * Copyright 2019-2020, Offchain Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ethbridge

import (
	"context"
	"math"
	"math/big"

	"github.com/offchainlabs/arbitrum/packages/arb-validator-core/message"

	errors2 "github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/offchainlabs/arbitrum/packages/arb-util/common"
)

type globalInbox struct {
	*globalInboxWatcher
	auth *TransactAuth
}

func newGlobalInbox(address ethcommon.Address, rollupAddress ethcommon.Address, client *ethclient.Client, auth *TransactAuth) (*globalInbox, error) {
	watcher, err := newGlobalInboxWatcher(address, rollupAddress, client)
	if err != nil {
		return nil, errors2.Wrap(err, "Failed to connect to GlobalInbox")
	}
	return &globalInbox{watcher, auth}, nil
}

func (con *globalInbox) SendTransactionMessage(ctx context.Context, data []byte, vmAddress common.Address, contactAddress common.Address, amount *big.Int, seqNumber *big.Int) error {
	con.auth.Lock()
	defer con.auth.Unlock()
	tx, err := con.GlobalInbox.SendTransactionMessage(
		con.auth.getAuth(ctx),
		vmAddress.ToEthAddress(),
		contactAddress.ToEthAddress(),
		seqNumber,
		amount,
		data,
	)
	if err != nil {
		return err
	}
	return con.waitForReceipt(ctx, tx, "SendTransactionMessage")
}

func (con *globalInbox) deliverTransactionBatch(
	ctx context.Context,
	chain common.Address,
	transactions []message.BatchTx,
) (*types.Transaction, error) {
	data := make([]byte, 0)
	for _, tx := range transactions {
		if len(tx.Data) > math.MaxUint16 {
			continue
		}
		data = append(data, tx.ToBytes()...)
	}
	con.auth.Lock()
	return con.GlobalInbox.DeliverTransactionBatch(
		con.auth.getAuth(ctx),
		chain.ToEthAddress(),
		data,
	)
}

func (con *globalInbox) DeliverTransactionBatch(
	ctx context.Context,
	chain common.Address,
	transactions []message.BatchTx,
) error {
	tx, err := con.deliverTransactionBatch(ctx, chain, transactions)
	if err != nil {
		return err
	}
	defer con.auth.Unlock()

	return con.waitForReceipt(ctx, tx, "DeliverTransactionBatch")
}

func (con *globalInbox) DeliverTransactionBatchNoWait(
	ctx context.Context,
	chain common.Address,
	transactions []message.BatchTx,
) error {
	_, err := con.deliverTransactionBatch(ctx, chain, transactions)
	if err != nil {
		return err
	}
	con.auth.Unlock()
	return err
}

func (con *globalInbox) DepositEthMessage(
	ctx context.Context,
	vmAddress common.Address,
	destination common.Address,
	value *big.Int,
) error {

	tx, err := con.GlobalInbox.DepositEthMessage(
		&bind.TransactOpts{
			From:     con.auth.auth.From,
			Signer:   con.auth.auth.Signer,
			GasLimit: con.auth.auth.GasLimit,
			Value:    value,
			Context:  ctx,
		},
		vmAddress.ToEthAddress(),
		destination.ToEthAddress(),
	)

	if err != nil {
		return err
	}

	return con.waitForReceipt(ctx, tx, "DepositEthMessage")
}

func (con *globalInbox) DepositERC20Message(
	ctx context.Context,
	vmAddress common.Address,
	tokenAddress common.Address,
	destination common.Address,
	value *big.Int,
) error {
	con.auth.Lock()
	defer con.auth.Unlock()
	tx, err := con.GlobalInbox.DepositERC20Message(
		con.auth.getAuth(ctx),
		vmAddress.ToEthAddress(),
		tokenAddress.ToEthAddress(),
		destination.ToEthAddress(),
		value,
	)

	if err != nil {
		return err
	}

	return con.waitForReceipt(ctx, tx, "DepositERC20Message")
}

func (con *globalInbox) DepositERC721Message(
	ctx context.Context,
	vmAddress common.Address,
	tokenAddress common.Address,
	destination common.Address,
	value *big.Int,
) error {
	con.auth.Lock()
	defer con.auth.Unlock()
	tx, err := con.GlobalInbox.DepositERC721Message(
		con.auth.getAuth(ctx),
		vmAddress.ToEthAddress(),
		tokenAddress.ToEthAddress(),
		destination.ToEthAddress(),
		value,
	)

	if err != nil {
		return err
	}

	return con.waitForReceipt(ctx, tx, "DepositERC721Message")
}

func (con *globalInbox) waitForReceipt(ctx context.Context, tx *types.Transaction, methodName string) error {
	return waitForReceipt(ctx, con.client, con.auth.auth.From, tx, methodName)
}
