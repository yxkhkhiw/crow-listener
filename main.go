package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var transferEventSignature = []byte("Transfer(address,address,uint256)")

func weiToEther(wei *big.Int) float64 {
	ether := new(big.Float).SetInt(wei)
	decimal := new(big.Float).SetFloat64(1e18)
	ether.Quo(ether, decimal)

	etherFloat, _ := ether.Float64()
	return etherFloat
}

func connectToClient(wsURL string) (*ethclient.Client, error) {
	fmt.Printf("Attempting to connect to WebSocket URL: %s\n", wsURL)
	client, err := ethclient.Dial(wsURL)
	if err != nil {
		return nil, err
	}
	fmt.Println("Successfully connected to the Ethereum client.")
	return client, nil
}

func main() {
	const (
		wsURL           = "wss://ws.wemix.com"
		contractAddress = "0x770d9d14c4ae2f78dca810958c1d9b7ea4620289"
		nightCrow       = "0x0000000000000000000000000000000000000000"
		devNightCrow    = "0x26e07c47ef5925dafbcb9eb2525e013b1e5ec85d"
	)

	client, err := connectToClient(wsURL)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	defer client.Close()

	contractAddr := common.HexToAddress(contractAddress)
	transferEventHash := crypto.Keccak256Hash(transferEventSignature)

	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddr},
	}

	logs := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatalf("Failed to subscribe to logs: %v", err)
	}
	defer sub.Unsubscribe()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

loop:
	for {
		select {
		case err := <-sub.Err():
			log.Printf("Subscription error: %v.\nReconnecting...", err)
			sub.Unsubscribe()
			client.Close()

			client, err = connectToClient(wsURL)
			if err != nil {
				log.Fatalf("Failed to reconnect to the Ethereum client: %v", err)
			}

			sub, err = client.SubscribeFilterLogs(context.Background(), query, logs)
			if err != nil {
				log.Fatalf("Failed to resubscribe to logs: %v", err)
			}

		case vLog := <-logs:
			if vLog.Topics[0] == transferEventHash {
				from := common.HexToAddress(vLog.Topics[1].Hex())
				to := common.HexToAddress(vLog.Topics[2].Hex())
				value := new(big.Int).SetBytes(vLog.Data)

				etherValue := weiToEther(value)

				nightCrowAddr := common.HexToAddress(nightCrow)
				devNightCrowAddr := common.HexToAddress(devNightCrow)
				if from == nightCrowAddr && to != devNightCrowAddr {
					fmt.Printf("%s Mint %s CROW!!!\n", to.Hex(), fmt.Sprintf("%.5f", etherValue))
				} else if to == nightCrowAddr && from != devNightCrowAddr {
					fmt.Printf("%s Burn %s CROW!!!\n", from.Hex(), fmt.Sprintf("%.5f", etherValue))
				} else {
					fmt.Printf("%s -> %s %s CROW\n", from.Hex(), to.Hex(), fmt.Sprintf("%.5f", etherValue))
				}
			}

		case <-signalChan:
			fmt.Println("Interrupt signal received. Shutting down...")
			break loop
		}
	}

	fmt.Println("Exiting...")
}
