/*
 * Copyright 2020, Offchain Labs, Inc.
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

package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/offchainlabs/arbitrum/packages/arb-util/common"
	"github.com/offchainlabs/arbitrum/packages/arb-util/machine"
	"github.com/offchainlabs/arbitrum/packages/arb-util/protocol"
	"github.com/offchainlabs/arbitrum/packages/arb-util/value"
	"github.com/offchainlabs/arbitrum/packages/arb-validator-core/ethbridge"
	"github.com/offchainlabs/arbitrum/packages/arb-validator-core/evm"
	"github.com/offchainlabs/arbitrum/packages/arb-validator-core/message"
	"github.com/offchainlabs/arbitrum/packages/arb-validator/loader"
	"github.com/offchainlabs/arbitrum/packages/arb-validator/structures"
	"log"
	"math"
	"math/big"
	"os"
)

func main() {
	filename := "messages.dat"
	//if err := getMessages(
	//	"https://ropsten.infura.io/v3/5851cb75448e4f8da37c5015006589a6",
	//	common.HexToAddress("0xEb57E0FB729E892E84259B4e6Dc08442Aa6E9Ee4"),
	//	filename,
	//); err != nil {
	//	panic(err)
	//}

	if err := testMessages(filename, "contract.ao"); err != nil {
		panic(err)
	}
}

func toEth(val *big.Int) *big.Float {
	fbalance := new(big.Float)
	fbalance.SetString(val.String())
	return new(big.Float).Quo(fbalance, big.NewFloat(math.Pow10(18)))
}

func runMessage(mach machine.Machine, msg message.Delivered) (evm.Result, error) {
	tb := protocol.NewRandomTimeBounds()
	vmInbox := structures.NewVMInbox()
	vmInbox.DeliverMessage(msg)
	assertion, _ := mach.ExecuteAssertion(
		100000,
		tb,
		vmInbox.AsValue(),
		1000,
	)
	//log.Println("ran assertion")
	logs := assertion.ParseLogs()
	if len(logs) != 1 {
		log.Fatal("returned incorrect log count")
	}
	evmResult, err := evm.ProcessLog(logs[0])
	if err != nil {
		return nil, err
	}
	return evmResult, nil
}

func testMessages(filename string, contract string) error {
	messages, err := loadMessages(filename)
	if err != nil {
		return err
	}

	mach, err := loader.LoadMachineFromFile(contract, false, "cpp")
	if err != nil {
		return err
	}

	//tb := protocol.NewRandomTimeBounds()

	addressesMap := make(map[common.Address]bool)
	singleMessages := make([]message.Delivered, 0)
	for _, msg := range messages {
		for _, del := range msg.VMInboxMessages() {
			singleMessages = append(singleMessages, message.Delivered{
				Message:      del.Message,
				DeliveryInfo: del.DeliveryInfo,
			})
			addressesMap[del.Message.DestAddress()] = true
			addressesMap[del.Message.SenderAddress()] = true
		}
	}
	addresses := make([]common.Address, 0, len(addressesMap))
	for address := range addressesMap {
		addresses = append(addresses, address)
	}

	totalSupplyData, _ := hexutil.Decode("0x18160ddd")
	totalSupplyCall := message.Call{
		To:   common.HexToAddress("0x3c1be20be169df0d99cca3730aae70580c3edf9a"),
		From: common.Address{},
		Data: totalSupplyData,
	}

	prevEthBalances := make(map[common.Address]*big.Int)
	prevTokenBalances := make(map[common.Address]*big.Int)
	for _, address := range addresses {
		prevEthBalances[address] = big.NewInt(0)
		prevTokenBalances[address] = big.NewInt(0)
	}
	prevTotalSupply := big.NewInt(0)

	runMsg := func(msg message.Delivered) error {
		log.Println()
		log.Println(msg.Message)

		txReturn, err := runMessage(mach, msg)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("tx result", txReturn)

		tokenSupplyResult, err := runMessage(mach.Clone(), message.Delivered{
			Message:      totalSupplyCall,
			DeliveryInfo: msg.DeliveryInfo,
		})
		if err != nil {
			log.Fatal(err)
		}
		totalBalance := new(big.Int).SetBytes(tokenSupplyResult.GetReturnData())
		log.Println("total supply", toEth(totalBalance))
		ethBalances := make(map[common.Address]*big.Int)
		tokenBalances := make(map[common.Address]*big.Int)
		for _, address := range addresses {
			getTokenBalanceData, _ := hexutil.Decode("0x70a08231000000000000000000000000" + address.String()[2:])
			getTokenBalanceCall := message.Call{
				To:   common.HexToAddress("0x716f0d674efeeca329f141d0ca0d97a98057bdbf"),
				From: common.Address{},
				Data: getTokenBalanceData,
			}
			tokenBalanceResult, err := runMessage(mach.Clone(), message.Delivered{
				Message:      getTokenBalanceCall,
				DeliveryInfo: msg.DeliveryInfo,
			})
			getEthBalanceData, _ := hexutil.Decode("0xf8b2cb4f000000000000000000000000" + address.String()[2:])
			call := message.Call{
				To:   common.HexToAddress("0x0000000000000000000000000000000000000065"),
				From: common.Address{},
				Data: getEthBalanceData,
			}
			ethBalanceResult, err := runMessage(mach.Clone(), message.Delivered{
				Message:      call,
				DeliveryInfo: msg.DeliveryInfo,
			})
			if err != nil {
				log.Fatal(err)
			}
			ethBalances[address] = new(big.Int).SetBytes(ethBalanceResult.GetReturnData())
			tokenBalances[address] = new(big.Int).SetBytes(tokenBalanceResult.GetReturnData())

			log.Println("balance", address, toEth(ethBalances[address]), toEth(tokenBalances[address]))
		}

		blocked := mach.IsBlocked(common.NewTimeBlocksInt(0), true)
		if blocked != nil {
			return fmt.Errorf("machine is blocked: %v", blocked)
		}
		if txReturn.Type() == evm.RevertCode {
			for _, address := range addresses {
				if ethBalances[address].Cmp(prevEthBalances[address]) != 0 {
					log.Fatal("eth balance changed after revert")
				}
				if tokenBalances[address].Cmp(prevTokenBalances[address]) != 0 {
					log.Fatal("token balance changed after revert")
				}
			}
			if prevTotalSupply.Cmp(totalBalance) != 0 {
				log.Fatal("total supply changed after revert")
			}
		}
		prevEthBalances = ethBalances
		prevTokenBalances = tokenBalances
		prevTotalSupply = totalBalance
		return nil
	}

	for i, msg := range singleMessages {
		//if i == 35 {
		//	break
		//}
		log.Println(i)
		if err := runMsg(msg); err != nil {
			log.Println(err)
			return nil
		}
	}

	//lastMessage := singleMessages[34]
	//_, err = runMessage(mach, lastMessage)
	//if err != nil {
	//	log.Fatal(err)
	//}

	//info := lastMessage.DeliveryInfo
	//msg := lastMessage.Message.(message.Transaction)
	//msg.SequenceNum = big.NewInt(0)

	//vmInbox := structures.NewVMInbox()
	//vmInbox.DeliverMessage(message.Delivered{
	//	Message:      msg,
	//	DeliveryInfo: info,
	//})
	//val := vmInbox.AsValue()
	//log.Println(hexutil.Encode(value.MarshalValueToBytes(val)))

	//log.Println("Delivering crash message", msg)
	//tokenBalanceFinalMessage := message.Delivered{
	//	Message:      getTokenBalanceCall,
	//	DeliveryInfo: lastMessage.DeliveryInfo,
	//}
	//vmInbox := structures.NewVMInbox()
	//vmInbox.DeliverMessage(tokenBalanceFinalMessage)
	//i := 0
	//_, _ = mach.ExecuteAssertion(
	//	1,
	//	tb,
	//	vmInbox.AsValue(),
	//	1000,
	//)
	//for {
	//	mach.PrintState()
	//	blocked := mach.IsBlocked(common.NewTimeBlocksInt(0), false)
	//	if blocked != nil {
	//		log.Printf("machine after %v steps is blocked: %v\n", i, blocked)
	//		return nil
	//	}
	//	_, _ = mach.ExecuteAssertion(
	//		1,
	//		tb,
	//		value.NewEmptyTuple(),
	//		1000,
	//	)
	//	i++
	//}
	return nil
}

func loadMessages(filename string) ([]message.Delivered, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	messagesStackVal, err := value.UnmarshalValue(f)
	if err != nil {
		return nil, err
	}

	messageVals, err := message.StackValueToList(messagesStackVal)
	if err != nil {
		return nil, err
	}

	received := make([]message.Received, 0, len(messageVals))
	for _, val := range messageVals {
		msg, err := message.UnmarshalReceivedFromCheckpoint(val)
		if err != nil {
			return nil, err
		}
		received = append(received, msg)
	}

	log.Println("Got", len(received), "messages")

	inbox := structures.NewInbox()
	for _, msg := range received {
		inbox.DeliverMessage(msg)
	}

	return inbox.GetAllMessages(), nil
}

func getMessages(ethURL string, rollupAddress common.Address, filename string) error {
	ctx := context.Background()

	ethclint, err := ethclient.Dial(ethURL)
	if err != nil {
		return err
	}

	client := ethbridge.NewEthClient(ethclint)
	rollupWatcher, err := client.NewRollupWatcher(rollupAddress)
	if err != nil {
		return err
	}

	inboxAddress, err := rollupWatcher.InboxAddress(ctx)
	if err != nil {
		return err
	}

	inboxWatcher, err := client.NewGlobalInboxWatcher(inboxAddress, rollupAddress)
	if err != nil {
		return err
	}

	_, blockId, _, err := rollupWatcher.GetCreationInfo(ctx)
	if err != nil {
		return err
	}

	received, err := inboxWatcher.GetAllReceived(ctx, blockId.Height.AsInt(), nil)
	if err != nil {
		return err
	}

	log.Println("Got", len(received), "messages")

	values := make([]value.Value, 0, len(received))
	for _, msg := range received {
		values = append(values, msg.CheckpointValue())
	}

	messagesStackVal := message.ListToStackValue(values)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	if err := value.MarshalValue(messagesStackVal, f); err != nil {
		return err
	}
	return nil
}
