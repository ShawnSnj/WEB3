package integration

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// FlashLoanArbitrage represents the flash loan arbitrage contract
type FlashLoanArbitrage struct {
	Address common.Address
	ABI     abi.ABI
	Client  *ethclient.Client
}

// FlashLoanArbitrageABI is the ABI for the UniswapFlashLoanArbitrage contract
const FlashLoanArbitrageABI = `[
	{
		"inputs": [
			{"internalType": "address", "name": "_token", "type": "address"},
			{"internalType": "uint256", "name": "_amount", "type": "uint256"},
			{"internalType": "address", "name": "_tokenIn", "type": "address"},
			{"internalType": "address", "name": "_tokenOut", "type": "address"},
			{"internalType": "uint256", "name": "_expectedProfit", "type": "uint256"}
		],
		"name": "requestArbitrageLoan",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "address", "name": "_token", "type": "address"},
			{"internalType": "uint256", "name": "_amount", "type": "uint256"},
			{"internalType": "address[]", "name": "_swapPath", "type": "address[]"},
			{"internalType": "uint256", "name": "_expectedProfit", "type": "uint256"}
		],
		"name": "requestArbitrageLoanWithPath",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"internalType": "address", "name": "_tokenAddress", "type": "address"}],
		"name": "getBalance",
		"outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [{"internalType": "address", "name": "_tokenAddress", "type": "address"}],
		"name": "withdrawProfit",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "owner",
		"outputs": [{"internalType": "address", "name": "", "type": "address"}],
		"stateMutability": "view",
		"type": "function"
	}
]`

// NewFlashLoanArbitrage creates a new flash loan arbitrage instance
func NewFlashLoanArbitrage(client *ethclient.Client, contractAddress string) (*FlashLoanArbitrage, error) {
	address := common.HexToAddress(contractAddress)

	parsedABI, err := abi.JSON(strings.NewReader(FlashLoanArbitrageABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	return &FlashLoanArbitrage{
		Address: address,
		ABI:     parsedABI,
		Client:  client,
	}, nil
}

// ExecuteArbitrage executes a flash loan arbitrage
func (fl *FlashLoanArbitrage) ExecuteArbitrage(
	ctx context.Context,
	auth *bind.TransactOpts,
	borrowToken common.Address,
	borrowAmount *big.Int,
	tokenIn common.Address,
	tokenOut common.Address,
	expectedProfit *big.Int,
) (*common.Hash, error) {
	// Pack the function call
	data, err := fl.ABI.Pack("requestArbitrageLoan", borrowToken, borrowAmount, tokenIn, tokenOut, expectedProfit)
	if err != nil {
		return nil, fmt.Errorf("failed to pack function data: %w", err)
	}

	// Create transaction
	tx := types.NewTransaction(
		auth.Nonce.Uint64(),
		fl.Address,
		big.NewInt(0),
		auth.GasLimit,
		auth.GasPrice,
		data,
	)

	// Sign transaction
	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	err = fl.Client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	hash := signedTx.Hash()
	return &hash, nil
}

// ExecuteArbitrageWithPath executes a flash loan arbitrage with custom path
func (fl *FlashLoanArbitrage) ExecuteArbitrageWithPath(
	ctx context.Context,
	auth *bind.TransactOpts,
	borrowToken common.Address,
	borrowAmount *big.Int,
	swapPath []common.Address,
	expectedProfit *big.Int,
) (*common.Hash, error) {
	// Pack the function call
	data, err := fl.ABI.Pack("requestArbitrageLoanWithPath", borrowToken, borrowAmount, swapPath, expectedProfit)
	if err != nil {
		return nil, fmt.Errorf("failed to pack function data: %w", err)
	}

	// Create transaction
	tx := types.NewTransaction(
		auth.Nonce.Uint64(),
		fl.Address,
		big.NewInt(0),
		auth.GasLimit,
		auth.GasPrice,
		data,
	)

	// Sign transaction
	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	err = fl.Client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	hash := signedTx.Hash()
	return &hash, nil
}

// GetContractBalance gets the contract's balance of a token
func (fl *FlashLoanArbitrage) GetContractBalance(
	ctx context.Context,
	tokenAddress common.Address,
) (*big.Int, error) {
	// Pack the function call
	data, err := fl.ABI.Pack("getBalance", tokenAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to pack function data: %w", err)
	}

	// Call the contract
	result, err := fl.Client.CallContract(ctx, ethereum.CallMsg{
		To:   &fl.Address,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call contract: %w", err)
	}

	// Unpack the result
	var balance *big.Int
	err = fl.ABI.UnpackIntoInterface(&balance, "getBalance", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result: %w", err)
	}

	return balance, nil
}

// WithdrawProfit withdraws profits from contract (owner only)
func (fl *FlashLoanArbitrage) WithdrawProfit(
	ctx context.Context,
	auth *bind.TransactOpts,
	tokenAddress common.Address,
) (*common.Hash, error) {
	// Pack the function call
	data, err := fl.ABI.Pack("withdrawProfit", tokenAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to pack function data: %w", err)
	}

	// Create transaction
	tx := types.NewTransaction(
		auth.Nonce.Uint64(),
		fl.Address,
		big.NewInt(0),
		auth.GasLimit,
		auth.GasPrice,
		data,
	)

	// Sign transaction
	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	err = fl.Client.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	hash := signedTx.Hash()
	return &hash, nil
}

// GetOwner gets the contract owner address
func (fl *FlashLoanArbitrage) GetOwner(ctx context.Context) (common.Address, error) {
	// Pack the function call
	data, err := fl.ABI.Pack("owner")
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to pack function data: %w", err)
	}

	// Call the contract
	result, err := fl.Client.CallContract(ctx, ethereum.CallMsg{
		To:   &fl.Address,
		Data: data,
	}, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to call contract: %w", err)
	}

	// Unpack the result
	var owner common.Address
	err = fl.ABI.UnpackIntoInterface(&owner, "owner", result)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to unpack result: %w", err)
	}

	return owner, nil
}

// LoadFlashLoanContract loads the flash loan contract from environment
func LoadFlashLoanContract(client *ethclient.Client) (*FlashLoanArbitrage, error) {
	contractAddress := os.Getenv("FLASHLOAN_CONTRACT_ADDRESS")
	if contractAddress == "" {
		return nil, fmt.Errorf("FLASHLOAN_CONTRACT_ADDRESS not set in environment")
	}

	return NewFlashLoanArbitrage(client, contractAddress)
}
