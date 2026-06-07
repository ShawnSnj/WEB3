package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

type Config struct {
	RPCURL       string `json:"rpc_url"`
	PrivateKey   string `json:"private_key"`
	FlashbotsURL string `json:"flashbots_url"`
	Dex1Address  string `json:"dex1_address"`
	Dex2Address  string `json:"dex2_address"`
	ArbContract  string `json:"arb_contract"`
	GasLimit     uint64 `json:"gas_limit"`
	LoopSeconds  int    `json:"loop_seconds"`
	FeeBps       uint64 `json:"fee_bps"`
	MinProfitBps uint64 `json:"min_profit_bps"`
}

func mustReadConfig(path string) Config {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read config %s: %v", path, err)
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		log.Fatalf("parse config: %v", err)
	}
	return cfg
}

func parseHexAddr(s string) common.Address {
	return common.HexToAddress(s)
}

func callReserve(ctx context.Context, contract *bind.BoundContract, blockNum *big.Int, method string) (uint64, error) {
	var out []interface{}
	err := contract.Call(&bind.CallOpts{Context: ctx, BlockNumber: blockNum}, &out, method)
	if err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, fmt.Errorf("no output from %s", method)
	}
	bi, ok := out[0].(*big.Int)
	if !ok || bi == nil {
		return 0, fmt.Errorf("unexpected output type from %s", method)
	}
	return bi.Uint64(), nil
}

func fetchReservesAtBlock(ctx context.Context, c *bind.BoundContract, blockNum *big.Int) (x uint64, y uint64, err error) {
	x, err = callReserve(ctx, c, blockNum, "reserveX")
	if err != nil {
		return 0, 0, err
	}
	y, err = callReserve(ctx, c, blockNum, "reserveY")
	if err != nil {
		return 0, 0, err
	}
	return x, y, nil
}

func main() {
	cfgPath := flag.String("config", "config/config.json", "path to config.json")
	flag.Parse()

	cfg := mustReadConfig(*cfgPath)
	if cfg.GasLimit == 0 {
		cfg.GasLimit = 350000
	}
	if cfg.LoopSeconds <= 0 {
		cfg.LoopSeconds = 5
	}
	if cfg.FeeBps == 0 {
		cfg.FeeBps = 30 // 0.3%
	}
	if cfg.MinProfitBps == 0 {
		cfg.MinProfitBps = 0
	}
	feeFloat := float64(cfg.FeeBps) / 10000.0

	eth, err := NewEthClient(cfg.RPCURL, cfg.PrivateKey)
	if err != nil {
		log.Fatalf("new eth client: %v", err)
	}

	dex1Addr := parseHexAddr(cfg.Dex1Address)
	dex2Addr := parseHexAddr(cfg.Dex2Address)
	arbAddr := parseHexAddr(cfg.ArbContract)
	if err := eth.EnsureNonZeroAddress(dex1Addr, "dex1"); err != nil {
		log.Fatal(err)
	}
	if err := eth.EnsureNonZeroAddress(dex2Addr, "dex2"); err != nil {
		log.Fatal(err)
	}
	if err := eth.EnsureNonZeroAddress(arbAddr, "arb_contract"); err != nil {
		log.Fatal(err)
	}

	simpleDEXABIJSON := `[{"inputs":[],"name":"reserveX","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"reserveY","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`
	flashArbABIJSON := `[{"inputs":[{"internalType":"address","name":"dex1","type":"address"},{"internalType":"address","name":"dex2","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"executeArbitrage","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

	simpleABI, err := abi.JSON(strings.NewReader(simpleDEXABIJSON))
	if err != nil {
		log.Fatalf("parse simple abi: %v", err)
	}
	arbABI, err := abi.JSON(strings.NewReader(flashArbABIJSON))
	if err != nil {
		log.Fatalf("parse arb abi: %v", err)
	}

	dex1 := bind.NewBoundContract(dex1Addr, simpleABI, eth.client, eth.client, eth.client)
	dex2 := bind.NewBoundContract(dex2Addr, simpleABI, eth.client, eth.client, eth.client)

	flashbots := NewFlashbotsClient(cfg.FlashbotsURL, eth.privKey)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	log.Printf("bot started from=%s dex1=%s dex2=%s arb=%s chainID=%s",
		eth.from.Hex(), dex1Addr.Hex(), dex2Addr.Hex(), arbAddr.Hex(), eth.chainID.String())

	ticker := time.NewTicker(time.Duration(cfg.LoopSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := step(ctx, eth, flashbots, dex1, dex2, arbABI, dex1Addr, dex2Addr, arbAddr, cfg.GasLimit, cfg.FeeBps, cfg.MinProfitBps, feeFloat)
		if err != nil {
			log.Printf("step error: %v", err)
		}

		<-ticker.C
	}
}

func step(
	ctx context.Context,
	eth *EthClient,
	flashbots *FlashbotsClient,
	dex1 *bind.BoundContract,
	dex2 *bind.BoundContract,
	arbABI abi.ABI,
	dex1Addr, dex2Addr, arbAddr common.Address,
	gasLimit uint64,
	feeBps uint64,
	minProfitBps uint64,
	feeFloat float64,
) error {
	latest, err := eth.GetLatestBlockNumber(ctx)
	if err != nil {
		return err
	}
	stateBlock := new(big.Int).SetUint64(latest)
	targetBlock := latest + 1

	// Fetch pool reserves from the simulation state block to keep consistency.
	x1, y1, err := fetchReservesAtBlock(ctx, dex1, stateBlock)
	if err != nil {
		return fmt.Errorf("dex1 reserves: %w", err)
	}
	x2, y2, err := fetchReservesAtBlock(ctx, dex2, stateBlock)
	if err != nil {
		return fmt.Errorf("dex2 reserves: %w", err)
	}
	if x1 == 0 || y1 == 0 || x2 == 0 || y2 == 0 {
		return nil
	}

	// Profit happens when pool2's X-per-Y is higher/lower than pool1 implied price;
	// the provided OptimalDx assumes we sell Y at pool2's current X/Y.
	priceSell := float64(x2) / float64(y2) // X per Y
	dxF := OptimalDx(float64(x1), float64(y1), priceSell, feeFloat)
	if dxF <= 0 {
		return nil
	}

	amountIn := uint64(dxF)
	if amountIn == 0 {
		return nil
	}

	// Estimate profit off-chain; Flashbots simulation will be the final guard.
	_, profit := EstimateProfit(x1, y1, x2, y2, amountIn, feeBps)
	minProfit := (int64(amountIn) * int64(minProfitBps)) / 10000
	if profit < minProfit || profit <= 0 {
		return nil
	}

	amountBig := new(big.Int).SetUint64(amountIn)
	data, err := arbABI.Pack("executeArbitrage", dex1Addr, dex2Addr, amountBig)
	if err != nil {
		return err
	}

	rawTx, err := eth.BuildSignedTx(ctx, arbAddr, data, gasLimit)
	if err != nil {
		return err
	}
	signedTxs := [][]byte{rawTx}

	// 1) Simulate first.
	callRes, err := flashbots.CallBundle(ctx, signedTxs, targetBlock, latest)
	if err != nil {
		return err
	}
	if callRes.RevertReason != "" {
		return nil
	}

	// 2) Only send if profitable (simulation did not revert and we expect positive profit).
	bh, err := flashbots.SendBundle(ctx, signedTxs, targetBlock)
	if err != nil {
		return err
	}
	log.Printf("sent bundle=%s amountIn=%d profitEst=%d targetBlock=%d",
		bh, amountIn, profit, targetBlock)
	return nil
}
