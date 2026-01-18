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

// FlashLoanLiquidation represents the flash loan liquidation contract
type FlashLoanLiquidation struct {
	Address  common.Address
	ABI      abi.ABI
	Client   *ethclient.Client
}

// FlashLoanLiquidationABI is the ABI for the FlashLoanLiquidation contract
// This is a minimal ABI with only the functions we need
const FlashLoanLiquidationABI = `[
	{
		"inputs": [
			{"internalType": "address", "name": "_token", "type": "address"},
			{"internalType": "uint256", "name": "_amount", "type": "uint256"},
			{"internalType": "address", "name": "_victim", "type": "address"},
			{"internalType": "address", "name": "_collateralAsset", "type": "address"}
		],
		"name": "requestLiquidationLoanSimple",
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
		"inputs": [{"internalType": "address", "name": "_aTokenAddress", "type": "address"}],
		"name": "withdrawAToken",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [{"internalType": "address", "name": "_tokenAddress", "type": "address"}],
		"name": "withdraw",
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

// NewFlashLoanLiquidation creates a new flash loan liquidation instance
func NewFlashLoanLiquidation(client *ethclient.Client, contractAddress string) (*FlashLoanLiquidation, error) {
	address := common.HexToAddress(contractAddress)
	
	parsedABI, err := abi.JSON(strings.NewReader(FlashLoanLiquidationABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	return &FlashLoanLiquidation{
		Address: address,
		ABI:     parsedABI,
		Client:  client,
	}, nil
}

// ExecuteLiquidation executes a flash loan liquidation
func (fl *FlashLoanLiquidation) ExecuteLiquidation(
	ctx context.Context,
	auth *bind.TransactOpts,
	debtToken common.Address,
	debtAmount *big.Int,
	victim common.Address,
	collateralToken common.Address,
) (*common.Hash, error) {
	// Pack the function call
	data, err := fl.ABI.Pack("requestLiquidationLoanSimple", debtToken, debtAmount, victim, collateralToken)
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
func (fl *FlashLoanLiquidation) GetContractBalance(
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

	// Unpack the result directly into big.Int
	var balance *big.Int
	err = fl.ABI.UnpackIntoInterface(&balance, "getBalance", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result: %w", err)
	}

	return balance, nil
}

// WithdrawAToken withdraws aToken from contract (owner only)
func (fl *FlashLoanLiquidation) WithdrawAToken(
	ctx context.Context,
	auth *bind.TransactOpts,
	aTokenAddress common.Address,
) (*common.Hash, error) {
	// Pack the function call
	data, err := fl.ABI.Pack("withdrawAToken", aTokenAddress)
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

// Withdraw withdraws tokens from contract (owner only)
func (fl *FlashLoanLiquidation) Withdraw(
	ctx context.Context,
	auth *bind.TransactOpts,
	tokenAddress common.Address,
) (*common.Hash, error) {
	// Pack the function call
	data, err := fl.ABI.Pack("withdraw", tokenAddress)
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
func (fl *FlashLoanLiquidation) GetOwner(ctx context.Context) (common.Address, error) {
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
func LoadFlashLoanContract(client *ethclient.Client) (*FlashLoanLiquidation, error) {
	contractAddress := os.Getenv("FLASHLOAN_CONTRACT_ADDRESS")
	if contractAddress == "" {
		return nil, fmt.Errorf("FLASHLOAN_CONTRACT_ADDRESS not set in environment")
	}

	return NewFlashLoanLiquidation(client, contractAddress)
}
