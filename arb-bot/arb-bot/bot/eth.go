package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type EthClient struct {
	client   *ethclient.Client
	privKey  *ecdsa.PrivateKey
	from     common.Address
	chainID  *big.Int
	httpTime time.Duration
}

func NewEthClient(rpcURL, privateKeyHex string) (*EthClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial rpc: %w", err)
	}

	keyHex := strings.TrimPrefix(privateKeyHex, "0x")
	priv, err := crypto.HexToECDSA(keyHex)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	from := crypto.PubkeyToAddress(priv.PublicKey)

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("chain id: %w", err)
	}

	return &EthClient{
		client:   client,
		privKey:  priv,
		from:     from,
		chainID:  chainID,
		httpTime: 15 * time.Second,
	}, nil
}

func (e *EthClient) From() common.Address { return e.from }

func (e *EthClient) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	h, err := e.client.BlockNumber(ctx)
	if err != nil {
		return 0, err
	}
	return h, nil
}

// BuildSignedTx builds and signs a transaction calling `to` with `data`.
// It uses EIP-1559 if the latest block header contains a baseFee; otherwise it falls back to legacy.
func (e *EthClient) BuildSignedTx(ctx context.Context, to common.Address, data []byte, gasLimit uint64) ([]byte, error) {
	nonce, err := e.client.PendingNonceAt(ctx, e.from)
	if err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}

	header, err := e.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}

	// Prefer EIP-1559 if baseFee is present.
	if header.BaseFee != nil {
		tipCap, err := e.client.SuggestGasTipCap(ctx)
		if err != nil {
			return nil, fmt.Errorf("tip cap: %w", err)
		}

		// maxFeePerGas = baseFee * 2 + tip
		two := big.NewInt(2)
		feeCap := new(big.Int).Mul(header.BaseFee, two)
		feeCap = feeCap.Add(feeCap, tipCap)

		tx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   e.chainID,
			Nonce:     nonce,
			To:        &to,
			Value:     big.NewInt(0),
			Data:      data,
			Gas:       gasLimit,
			GasTipCap: tipCap,
			GasFeeCap: feeCap,
		})

		signer := types.LatestSignerForChainID(e.chainID)
		signedTx, err := types.SignTx(tx, signer, e.privKey)
		if err != nil {
			return nil, fmt.Errorf("sign tx: %w", err)
		}

		raw, err := signedTx.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("marshal tx: %w", err)
		}
		return raw, nil
	}

	// Legacy fallback.
	gasPrice, err := e.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("gas price: %w", err)
	}
	tx := types.NewTransaction(nonce, to, big.NewInt(0), gasLimit, gasPrice, data)
	signer := types.LatestSignerForChainID(e.chainID)
	signedTx, err := types.SignTx(tx, signer, e.privKey)
	if err != nil {
		return nil, fmt.Errorf("sign tx: %w", err)
	}
	raw, err := signedTx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal tx: %w", err)
	}
	return raw, nil
}

func (e *EthClient) EnsureNonZeroAddress(addr common.Address, name string) error {
	if addr == (common.Address{}) {
		return errors.New(name + ": empty address")
	}
	return nil
}

