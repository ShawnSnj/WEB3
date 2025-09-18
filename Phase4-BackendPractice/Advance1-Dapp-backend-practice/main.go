package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"task2/counter"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	client, err := ethclient.Dial("https://sepolia.infura.io/v3/59b2ab9d2eda4fa086cdd7db0e4c1a3e")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	privateKey, err := crypto.HexToECDSA("4943c8f28bd54bc4d7397aedeb250501eedb2c29ec4ef780d8637ed273339eb2")
	if err != nil {
		log.Fatal(err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert publicKey to ECDSA")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Fatal(err)
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(11155111)) // Sepolia chain ID
	if err != nil {
		log.Fatal(err)
	}
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)     // in wei
	auth.GasLimit = uint64(300000) // in units
	auth.GasPrice = gasPrice

	contractAddress := common.HexToAddress("0x3E96B6E756F7b67fA32Ea29503F3faDD873C3C92")
	instance, err := counter.NewCounter(contractAddress, client)
	if err != nil {
		log.Fatal(err)
	}
	tx, err := instance.Increment(auth)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Increment tx sent:", tx.Hash().Hex())

	// call getCount
	count, err := instance.GetCount(&bind.CallOpts{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Current count:", count)
}
