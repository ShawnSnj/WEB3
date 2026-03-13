package contracts

import (
	"fmt"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// UniswapV2Router02ABI is the ABI for Uniswap V2 Router
var UniswapV2Router02ABI = `[
	{
		"inputs": [
			{"internalType": "uint256", "name": "amountIn", "type": "uint256"},
			{"internalType": "address[]", "name": "path", "type": "address[]"}
		],
		"name": "getAmountsOut",
		"outputs": [{"internalType": "uint256[]", "name": "amounts", "type": "uint256[]"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "uint256", "name": "amountIn", "type": "uint256"},
			{"internalType": "uint256", "name": "amountOutMin", "type": "uint256"},
			{"internalType": "address[]", "name": "path", "type": "address[]"},
			{"internalType": "address", "name": "to", "type": "address"},
			{"internalType": "uint256", "name": "deadline", "type": "uint256"}
		],
		"name": "swapExactTokensForTokens",
		"outputs": [{"internalType": "uint256[]", "name": "amounts", "type": "uint256[]"}],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "uint256", "name": "amountIn", "type": "uint256"},
			{"internalType": "uint256", "name": "amountOutMin", "type": "uint256"},
			{"internalType": "address[]", "name": "path", "type": "address[]"},
			{"internalType": "address", "name": "to", "type": "address"},
			{"internalType": "uint256", "name": "deadline", "type": "uint256"}
		],
		"name": "swapExactETHForTokens",
		"outputs": [{"internalType": "uint256[]", "name": "amounts", "type": "uint256[]"}],
		"stateMutability": "payable",
		"type": "function"
	},
	{
		"inputs": [
			{"internalType": "uint256", "name": "amountIn", "type": "uint256"},
			{"internalType": "uint256", "name": "amountOutMin", "type": "uint256"},
			{"internalType": "address[]", "name": "path", "type": "address[]"},
			{"internalType": "address", "name": "to", "type": "address"},
			{"internalType": "uint256", "name": "deadline", "type": "uint256"}
		],
		"name": "swapExactTokensForETH",
		"outputs": [{"internalType": "uint256[]", "name": "amounts", "type": "uint256[]"}],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`

// UniswapV2Router represents a Uniswap V2 Router contract
type UniswapV2Router struct {
	Address common.Address
	ABI     abi.ABI
	Backend bind.ContractBackend
}

// NewUniswapV2Router creates a new Uniswap V2 Router instance
func NewUniswapV2Router(address common.Address, backend bind.ContractBackend) (*UniswapV2Router, error) {
	parsedABI, err := abi.JSON(strings.NewReader(UniswapV2Router02ABI))
	if err != nil {
		return nil, err
	}

	return &UniswapV2Router{
		Address: address,
		ABI:     parsedABI,
		Backend: backend,
	}, nil
}

// GetAmountsOut gets the output amounts for a given input amount and path
func (r *UniswapV2Router) GetAmountsOut(opts *bind.CallOpts, amountIn *big.Int, path []common.Address) ([]*big.Int, error) {
	data, err := r.ABI.Pack("getAmountsOut", amountIn, path)
	if err != nil {
		return nil, err
	}

	caller := r.Backend.(bind.ContractCaller)
	callMsg := ethereum.CallMsg{
		To:   &r.Address,
		Data: data,
	}

	result, err := caller.CallContract(opts.Context, callMsg, nil)
	if err != nil {
		return nil, fmt.Errorf("contract call failed: %w", err)
	}

	// Check if result is empty (no pool exists)
	if len(result) == 0 {
		return nil, fmt.Errorf("no liquidity pool found for this token pair")
	}

	// Unpack the result - getAmountsOut returns uint[] (dynamic array)
	// Use Unpack which returns []interface{} for dynamic types
	unpacked, err := r.ABI.Unpack("getAmountsOut", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result (pool may not exist or have no liquidity): %w", err)
	}

	// The result is a slice containing the amounts array
	if len(unpacked) == 0 {
		return nil, fmt.Errorf("no data returned from router")
	}

	// The first element should be the []*big.Int array
	// But it might be wrapped differently, so let's check the type
	var amounts []*big.Int
	
	switch v := unpacked[0].(type) {
	case []*big.Int:
		amounts = v
	case []interface{}:
		// Convert []interface{} to []*big.Int
		amounts = make([]*big.Int, len(v))
		for i, item := range v {
			if bi, ok := item.(*big.Int); ok {
				amounts[i] = bi
			} else {
				return nil, fmt.Errorf("unexpected type in amounts array at index %d: %T", i, item)
			}
		}
	default:
		return nil, fmt.Errorf("unexpected return type from router (expected []*big.Int or []interface{}, got %T)", unpacked[0])
	}

	if len(amounts) < 2 {
		return nil, fmt.Errorf("invalid amounts returned from router (got %d amounts, expected at least 2)", len(amounts))
	}

	// Check if amounts are zero (indicates no liquidity)
	if amounts[0].Cmp(big.NewInt(0)) == 0 || amounts[1].Cmp(big.NewInt(0)) == 0 {
		return nil, fmt.Errorf("pool has no liquidity (amounts are zero)")
	}

	return amounts, nil
}

// SwapExactTokensForTokens executes a swap
func (r *UniswapV2Router) SwapExactTokensForTokens(
	opts *bind.TransactOpts,
	amountIn *big.Int,
	amountOutMin *big.Int,
	path []common.Address,
	to common.Address,
	deadline *big.Int,
) (*types.Transaction, error) {
	// Create bound contract for transaction
	boundContract := bind.NewBoundContract(r.Address, r.ABI, r.Backend, r.Backend, r.Backend)
	return boundContract.Transact(opts, "swapExactTokensForTokens", amountIn, amountOutMin, path, to, deadline)
}

