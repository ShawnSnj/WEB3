// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package ethereum

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// EthereumMetaData contains all meta data concerning the Ethereum contract.
var EthereumMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"auctionId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"seller\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"nftContract\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"startPrice\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"endTime\",\"type\":\"uint256\"}],\"name\":\"AuctionCreated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"auctionId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"winner\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"finalPrice\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"seller\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"time\",\"type\":\"uint256\"}],\"name\":\"AuctionEnded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"auctionId\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"bidder\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"refundedBidder\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"refundedAmount\",\"type\":\"uint256\"}],\"name\":\"BidPlaced\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"MIN_BID_INCREMENT\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"auctions\",\"outputs\":[{\"internalType\":\"addresspayable\",\"name\":\"seller\",\"type\":\"address\"},{\"internalType\":\"contractIERC721\",\"name\":\"nftContract\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"startPrice\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"highestBid\",\"type\":\"uint256\"},{\"internalType\":\"addresspayable\",\"name\":\"highestBidder\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"endTime\",\"type\":\"uint256\"},{\"internalType\":\"bool\",\"name\":\"ended\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_nftContract\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_tokenId\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_startPrice\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_duration\",\"type\":\"uint256\"}],\"name\":\"createAuction\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_auctionId\",\"type\":\"uint256\"}],\"name\":\"endAuction\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"fundsToWithdraw\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"nextAuctionId\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_auctionId\",\"type\":\"uint256\"}],\"name\":\"placeBid\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_auctionId\",\"type\":\"uint256\"}],\"name\":\"withdraw\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// EthereumABI is the input ABI used to generate the binding from.
// Deprecated: Use EthereumMetaData.ABI instead.
var EthereumABI = EthereumMetaData.ABI

// Ethereum is an auto generated Go binding around an Ethereum contract.
type Ethereum struct {
	EthereumCaller     // Read-only binding to the contract
	EthereumTransactor // Write-only binding to the contract
	EthereumFilterer   // Log filterer for contract events
}

// EthereumCaller is an auto generated read-only Go binding around an Ethereum contract.
type EthereumCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EthereumTransactor is an auto generated write-only Go binding around an Ethereum contract.
type EthereumTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EthereumFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type EthereumFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// EthereumSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type EthereumSession struct {
	Contract     *Ethereum         // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// EthereumCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type EthereumCallerSession struct {
	Contract *EthereumCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts   // Call options to use throughout this session
}

// EthereumTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type EthereumTransactorSession struct {
	Contract     *EthereumTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// EthereumRaw is an auto generated low-level Go binding around an Ethereum contract.
type EthereumRaw struct {
	Contract *Ethereum // Generic contract binding to access the raw methods on
}

// EthereumCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type EthereumCallerRaw struct {
	Contract *EthereumCaller // Generic read-only contract binding to access the raw methods on
}

// EthereumTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type EthereumTransactorRaw struct {
	Contract *EthereumTransactor // Generic write-only contract binding to access the raw methods on
}

// NewEthereum creates a new instance of Ethereum, bound to a specific deployed contract.
func NewEthereum(address common.Address, backend bind.ContractBackend) (*Ethereum, error) {
	contract, err := bindEthereum(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Ethereum{EthereumCaller: EthereumCaller{contract: contract}, EthereumTransactor: EthereumTransactor{contract: contract}, EthereumFilterer: EthereumFilterer{contract: contract}}, nil
}

// NewEthereumCaller creates a new read-only instance of Ethereum, bound to a specific deployed contract.
func NewEthereumCaller(address common.Address, caller bind.ContractCaller) (*EthereumCaller, error) {
	contract, err := bindEthereum(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &EthereumCaller{contract: contract}, nil
}

// NewEthereumTransactor creates a new write-only instance of Ethereum, bound to a specific deployed contract.
func NewEthereumTransactor(address common.Address, transactor bind.ContractTransactor) (*EthereumTransactor, error) {
	contract, err := bindEthereum(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &EthereumTransactor{contract: contract}, nil
}

// NewEthereumFilterer creates a new log filterer instance of Ethereum, bound to a specific deployed contract.
func NewEthereumFilterer(address common.Address, filterer bind.ContractFilterer) (*EthereumFilterer, error) {
	contract, err := bindEthereum(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &EthereumFilterer{contract: contract}, nil
}

// bindEthereum binds a generic wrapper to an already deployed contract.
func bindEthereum(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := EthereumMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Ethereum *EthereumRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Ethereum.Contract.EthereumCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Ethereum *EthereumRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Ethereum.Contract.EthereumTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Ethereum *EthereumRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Ethereum.Contract.EthereumTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Ethereum *EthereumCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Ethereum.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Ethereum *EthereumTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Ethereum.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Ethereum *EthereumTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Ethereum.Contract.contract.Transact(opts, method, params...)
}

// MINBIDINCREMENT is a free data retrieval call binding the contract method 0x71943bce.
//
// Solidity: function MIN_BID_INCREMENT() view returns(uint256)
func (_Ethereum *EthereumCaller) MINBIDINCREMENT(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Ethereum.contract.Call(opts, &out, "MIN_BID_INCREMENT")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// MINBIDINCREMENT is a free data retrieval call binding the contract method 0x71943bce.
//
// Solidity: function MIN_BID_INCREMENT() view returns(uint256)
func (_Ethereum *EthereumSession) MINBIDINCREMENT() (*big.Int, error) {
	return _Ethereum.Contract.MINBIDINCREMENT(&_Ethereum.CallOpts)
}

// MINBIDINCREMENT is a free data retrieval call binding the contract method 0x71943bce.
//
// Solidity: function MIN_BID_INCREMENT() view returns(uint256)
func (_Ethereum *EthereumCallerSession) MINBIDINCREMENT() (*big.Int, error) {
	return _Ethereum.Contract.MINBIDINCREMENT(&_Ethereum.CallOpts)
}

// Auctions is a free data retrieval call binding the contract method 0x571a26a0.
//
// Solidity: function auctions(uint256 ) view returns(address seller, address nftContract, uint256 tokenId, uint256 startPrice, uint256 highestBid, address highestBidder, uint256 endTime, bool ended)
func (_Ethereum *EthereumCaller) Auctions(opts *bind.CallOpts, arg0 *big.Int) (struct {
	Seller        common.Address
	NftContract   common.Address
	TokenId       *big.Int
	StartPrice    *big.Int
	HighestBid    *big.Int
	HighestBidder common.Address
	EndTime       *big.Int
	Ended         bool
}, error) {
	var out []interface{}
	err := _Ethereum.contract.Call(opts, &out, "auctions", arg0)

	outstruct := new(struct {
		Seller        common.Address
		NftContract   common.Address
		TokenId       *big.Int
		StartPrice    *big.Int
		HighestBid    *big.Int
		HighestBidder common.Address
		EndTime       *big.Int
		Ended         bool
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Seller = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.NftContract = *abi.ConvertType(out[1], new(common.Address)).(*common.Address)
	outstruct.TokenId = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.StartPrice = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)
	outstruct.HighestBid = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)
	outstruct.HighestBidder = *abi.ConvertType(out[5], new(common.Address)).(*common.Address)
	outstruct.EndTime = *abi.ConvertType(out[6], new(*big.Int)).(**big.Int)
	outstruct.Ended = *abi.ConvertType(out[7], new(bool)).(*bool)

	return *outstruct, err

}

// Auctions is a free data retrieval call binding the contract method 0x571a26a0.
//
// Solidity: function auctions(uint256 ) view returns(address seller, address nftContract, uint256 tokenId, uint256 startPrice, uint256 highestBid, address highestBidder, uint256 endTime, bool ended)
func (_Ethereum *EthereumSession) Auctions(arg0 *big.Int) (struct {
	Seller        common.Address
	NftContract   common.Address
	TokenId       *big.Int
	StartPrice    *big.Int
	HighestBid    *big.Int
	HighestBidder common.Address
	EndTime       *big.Int
	Ended         bool
}, error) {
	return _Ethereum.Contract.Auctions(&_Ethereum.CallOpts, arg0)
}

// Auctions is a free data retrieval call binding the contract method 0x571a26a0.
//
// Solidity: function auctions(uint256 ) view returns(address seller, address nftContract, uint256 tokenId, uint256 startPrice, uint256 highestBid, address highestBidder, uint256 endTime, bool ended)
func (_Ethereum *EthereumCallerSession) Auctions(arg0 *big.Int) (struct {
	Seller        common.Address
	NftContract   common.Address
	TokenId       *big.Int
	StartPrice    *big.Int
	HighestBid    *big.Int
	HighestBidder common.Address
	EndTime       *big.Int
	Ended         bool
}, error) {
	return _Ethereum.Contract.Auctions(&_Ethereum.CallOpts, arg0)
}

// FundsToWithdraw is a free data retrieval call binding the contract method 0x05caa4a8.
//
// Solidity: function fundsToWithdraw(uint256 , address ) view returns(uint256)
func (_Ethereum *EthereumCaller) FundsToWithdraw(opts *bind.CallOpts, arg0 *big.Int, arg1 common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Ethereum.contract.Call(opts, &out, "fundsToWithdraw", arg0, arg1)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// FundsToWithdraw is a free data retrieval call binding the contract method 0x05caa4a8.
//
// Solidity: function fundsToWithdraw(uint256 , address ) view returns(uint256)
func (_Ethereum *EthereumSession) FundsToWithdraw(arg0 *big.Int, arg1 common.Address) (*big.Int, error) {
	return _Ethereum.Contract.FundsToWithdraw(&_Ethereum.CallOpts, arg0, arg1)
}

// FundsToWithdraw is a free data retrieval call binding the contract method 0x05caa4a8.
//
// Solidity: function fundsToWithdraw(uint256 , address ) view returns(uint256)
func (_Ethereum *EthereumCallerSession) FundsToWithdraw(arg0 *big.Int, arg1 common.Address) (*big.Int, error) {
	return _Ethereum.Contract.FundsToWithdraw(&_Ethereum.CallOpts, arg0, arg1)
}

// NextAuctionId is a free data retrieval call binding the contract method 0xfc528482.
//
// Solidity: function nextAuctionId() view returns(uint256)
func (_Ethereum *EthereumCaller) NextAuctionId(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Ethereum.contract.Call(opts, &out, "nextAuctionId")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NextAuctionId is a free data retrieval call binding the contract method 0xfc528482.
//
// Solidity: function nextAuctionId() view returns(uint256)
func (_Ethereum *EthereumSession) NextAuctionId() (*big.Int, error) {
	return _Ethereum.Contract.NextAuctionId(&_Ethereum.CallOpts)
}

// NextAuctionId is a free data retrieval call binding the contract method 0xfc528482.
//
// Solidity: function nextAuctionId() view returns(uint256)
func (_Ethereum *EthereumCallerSession) NextAuctionId() (*big.Int, error) {
	return _Ethereum.Contract.NextAuctionId(&_Ethereum.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Ethereum *EthereumCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Ethereum.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Ethereum *EthereumSession) Owner() (common.Address, error) {
	return _Ethereum.Contract.Owner(&_Ethereum.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Ethereum *EthereumCallerSession) Owner() (common.Address, error) {
	return _Ethereum.Contract.Owner(&_Ethereum.CallOpts)
}

// CreateAuction is a paid mutator transaction binding the contract method 0x61beb1d7.
//
// Solidity: function createAuction(address _nftContract, uint256 _tokenId, uint256 _startPrice, uint256 _duration) returns()
func (_Ethereum *EthereumTransactor) CreateAuction(opts *bind.TransactOpts, _nftContract common.Address, _tokenId *big.Int, _startPrice *big.Int, _duration *big.Int) (*types.Transaction, error) {
	return _Ethereum.contract.Transact(opts, "createAuction", _nftContract, _tokenId, _startPrice, _duration)
}

// CreateAuction is a paid mutator transaction binding the contract method 0x61beb1d7.
//
// Solidity: function createAuction(address _nftContract, uint256 _tokenId, uint256 _startPrice, uint256 _duration) returns()
func (_Ethereum *EthereumSession) CreateAuction(_nftContract common.Address, _tokenId *big.Int, _startPrice *big.Int, _duration *big.Int) (*types.Transaction, error) {
	return _Ethereum.Contract.CreateAuction(&_Ethereum.TransactOpts, _nftContract, _tokenId, _startPrice, _duration)
}

// CreateAuction is a paid mutator transaction binding the contract method 0x61beb1d7.
//
// Solidity: function createAuction(address _nftContract, uint256 _tokenId, uint256 _startPrice, uint256 _duration) returns()
func (_Ethereum *EthereumTransactorSession) CreateAuction(_nftContract common.Address, _tokenId *big.Int, _startPrice *big.Int, _duration *big.Int) (*types.Transaction, error) {
	return _Ethereum.Contract.CreateAuction(&_Ethereum.TransactOpts, _nftContract, _tokenId, _startPrice, _duration)
}

// EndAuction is a paid mutator transaction binding the contract method 0xb9a2de3a.
//
// Solidity: function endAuction(uint256 _auctionId) returns()
func (_Ethereum *EthereumTransactor) EndAuction(opts *bind.TransactOpts, _auctionId *big.Int) (*types.Transaction, error) {
	return _Ethereum.contract.Transact(opts, "endAuction", _auctionId)
}

// EndAuction is a paid mutator transaction binding the contract method 0xb9a2de3a.
//
// Solidity: function endAuction(uint256 _auctionId) returns()
func (_Ethereum *EthereumSession) EndAuction(_auctionId *big.Int) (*types.Transaction, error) {
	return _Ethereum.Contract.EndAuction(&_Ethereum.TransactOpts, _auctionId)
}

// EndAuction is a paid mutator transaction binding the contract method 0xb9a2de3a.
//
// Solidity: function endAuction(uint256 _auctionId) returns()
func (_Ethereum *EthereumTransactorSession) EndAuction(_auctionId *big.Int) (*types.Transaction, error) {
	return _Ethereum.Contract.EndAuction(&_Ethereum.TransactOpts, _auctionId)
}

// PlaceBid is a paid mutator transaction binding the contract method 0x9979ef45.
//
// Solidity: function placeBid(uint256 _auctionId) payable returns()
func (_Ethereum *EthereumTransactor) PlaceBid(opts *bind.TransactOpts, _auctionId *big.Int) (*types.Transaction, error) {
	return _Ethereum.contract.Transact(opts, "placeBid", _auctionId)
}

// PlaceBid is a paid mutator transaction binding the contract method 0x9979ef45.
//
// Solidity: function placeBid(uint256 _auctionId) payable returns()
func (_Ethereum *EthereumSession) PlaceBid(_auctionId *big.Int) (*types.Transaction, error) {
	return _Ethereum.Contract.PlaceBid(&_Ethereum.TransactOpts, _auctionId)
}

// PlaceBid is a paid mutator transaction binding the contract method 0x9979ef45.
//
// Solidity: function placeBid(uint256 _auctionId) payable returns()
func (_Ethereum *EthereumTransactorSession) PlaceBid(_auctionId *big.Int) (*types.Transaction, error) {
	return _Ethereum.Contract.PlaceBid(&_Ethereum.TransactOpts, _auctionId)
}

// Withdraw is a paid mutator transaction binding the contract method 0x2e1a7d4d.
//
// Solidity: function withdraw(uint256 _auctionId) returns()
func (_Ethereum *EthereumTransactor) Withdraw(opts *bind.TransactOpts, _auctionId *big.Int) (*types.Transaction, error) {
	return _Ethereum.contract.Transact(opts, "withdraw", _auctionId)
}

// Withdraw is a paid mutator transaction binding the contract method 0x2e1a7d4d.
//
// Solidity: function withdraw(uint256 _auctionId) returns()
func (_Ethereum *EthereumSession) Withdraw(_auctionId *big.Int) (*types.Transaction, error) {
	return _Ethereum.Contract.Withdraw(&_Ethereum.TransactOpts, _auctionId)
}

// Withdraw is a paid mutator transaction binding the contract method 0x2e1a7d4d.
//
// Solidity: function withdraw(uint256 _auctionId) returns()
func (_Ethereum *EthereumTransactorSession) Withdraw(_auctionId *big.Int) (*types.Transaction, error) {
	return _Ethereum.Contract.Withdraw(&_Ethereum.TransactOpts, _auctionId)
}

// EthereumAuctionCreatedIterator is returned from FilterAuctionCreated and is used to iterate over the raw logs and unpacked data for AuctionCreated events raised by the Ethereum contract.
type EthereumAuctionCreatedIterator struct {
	Event *EthereumAuctionCreated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EthereumAuctionCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EthereumAuctionCreated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EthereumAuctionCreated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EthereumAuctionCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EthereumAuctionCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EthereumAuctionCreated represents a AuctionCreated event raised by the Ethereum contract.
type EthereumAuctionCreated struct {
	AuctionId   *big.Int
	Seller      common.Address
	NftContract common.Address
	TokenId     *big.Int
	StartPrice  *big.Int
	EndTime     *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterAuctionCreated is a free log retrieval operation binding the contract event 0x06b9e486c68303eb64052e0493f906f3d93a1b7149b6b8dcff221aebd16c3513.
//
// Solidity: event AuctionCreated(uint256 auctionId, address indexed seller, address nftContract, uint256 tokenId, uint256 startPrice, uint256 endTime)
func (_Ethereum *EthereumFilterer) FilterAuctionCreated(opts *bind.FilterOpts, seller []common.Address) (*EthereumAuctionCreatedIterator, error) {

	var sellerRule []interface{}
	for _, sellerItem := range seller {
		sellerRule = append(sellerRule, sellerItem)
	}

	logs, sub, err := _Ethereum.contract.FilterLogs(opts, "AuctionCreated", sellerRule)
	if err != nil {
		return nil, err
	}
	return &EthereumAuctionCreatedIterator{contract: _Ethereum.contract, event: "AuctionCreated", logs: logs, sub: sub}, nil
}

// WatchAuctionCreated is a free log subscription operation binding the contract event 0x06b9e486c68303eb64052e0493f906f3d93a1b7149b6b8dcff221aebd16c3513.
//
// Solidity: event AuctionCreated(uint256 auctionId, address indexed seller, address nftContract, uint256 tokenId, uint256 startPrice, uint256 endTime)
func (_Ethereum *EthereumFilterer) WatchAuctionCreated(opts *bind.WatchOpts, sink chan<- *EthereumAuctionCreated, seller []common.Address) (event.Subscription, error) {

	var sellerRule []interface{}
	for _, sellerItem := range seller {
		sellerRule = append(sellerRule, sellerItem)
	}

	logs, sub, err := _Ethereum.contract.WatchLogs(opts, "AuctionCreated", sellerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EthereumAuctionCreated)
				if err := _Ethereum.contract.UnpackLog(event, "AuctionCreated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAuctionCreated is a log parse operation binding the contract event 0x06b9e486c68303eb64052e0493f906f3d93a1b7149b6b8dcff221aebd16c3513.
//
// Solidity: event AuctionCreated(uint256 auctionId, address indexed seller, address nftContract, uint256 tokenId, uint256 startPrice, uint256 endTime)
func (_Ethereum *EthereumFilterer) ParseAuctionCreated(log types.Log) (*EthereumAuctionCreated, error) {
	event := new(EthereumAuctionCreated)
	if err := _Ethereum.contract.UnpackLog(event, "AuctionCreated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EthereumAuctionEndedIterator is returned from FilterAuctionEnded and is used to iterate over the raw logs and unpacked data for AuctionEnded events raised by the Ethereum contract.
type EthereumAuctionEndedIterator struct {
	Event *EthereumAuctionEnded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EthereumAuctionEndedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EthereumAuctionEnded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EthereumAuctionEnded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EthereumAuctionEndedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EthereumAuctionEndedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EthereumAuctionEnded represents a AuctionEnded event raised by the Ethereum contract.
type EthereumAuctionEnded struct {
	AuctionId  *big.Int
	Winner     common.Address
	FinalPrice *big.Int
	Seller     common.Address
	Time       *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterAuctionEnded is a free log retrieval operation binding the contract event 0xe5475ecf90fac88802f904d6cb8390f7106dec2526f70b078ce579d6f7221251.
//
// Solidity: event AuctionEnded(uint256 indexed auctionId, address winner, uint256 finalPrice, address seller, uint256 time)
func (_Ethereum *EthereumFilterer) FilterAuctionEnded(opts *bind.FilterOpts, auctionId []*big.Int) (*EthereumAuctionEndedIterator, error) {

	var auctionIdRule []interface{}
	for _, auctionIdItem := range auctionId {
		auctionIdRule = append(auctionIdRule, auctionIdItem)
	}

	logs, sub, err := _Ethereum.contract.FilterLogs(opts, "AuctionEnded", auctionIdRule)
	if err != nil {
		return nil, err
	}
	return &EthereumAuctionEndedIterator{contract: _Ethereum.contract, event: "AuctionEnded", logs: logs, sub: sub}, nil
}

// WatchAuctionEnded is a free log subscription operation binding the contract event 0xe5475ecf90fac88802f904d6cb8390f7106dec2526f70b078ce579d6f7221251.
//
// Solidity: event AuctionEnded(uint256 indexed auctionId, address winner, uint256 finalPrice, address seller, uint256 time)
func (_Ethereum *EthereumFilterer) WatchAuctionEnded(opts *bind.WatchOpts, sink chan<- *EthereumAuctionEnded, auctionId []*big.Int) (event.Subscription, error) {

	var auctionIdRule []interface{}
	for _, auctionIdItem := range auctionId {
		auctionIdRule = append(auctionIdRule, auctionIdItem)
	}

	logs, sub, err := _Ethereum.contract.WatchLogs(opts, "AuctionEnded", auctionIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EthereumAuctionEnded)
				if err := _Ethereum.contract.UnpackLog(event, "AuctionEnded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAuctionEnded is a log parse operation binding the contract event 0xe5475ecf90fac88802f904d6cb8390f7106dec2526f70b078ce579d6f7221251.
//
// Solidity: event AuctionEnded(uint256 indexed auctionId, address winner, uint256 finalPrice, address seller, uint256 time)
func (_Ethereum *EthereumFilterer) ParseAuctionEnded(log types.Log) (*EthereumAuctionEnded, error) {
	event := new(EthereumAuctionEnded)
	if err := _Ethereum.contract.UnpackLog(event, "AuctionEnded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// EthereumBidPlacedIterator is returned from FilterBidPlaced and is used to iterate over the raw logs and unpacked data for BidPlaced events raised by the Ethereum contract.
type EthereumBidPlacedIterator struct {
	Event *EthereumBidPlaced // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *EthereumBidPlacedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(EthereumBidPlaced)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(EthereumBidPlaced)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *EthereumBidPlacedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *EthereumBidPlacedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// EthereumBidPlaced represents a BidPlaced event raised by the Ethereum contract.
type EthereumBidPlaced struct {
	AuctionId      *big.Int
	Bidder         common.Address
	Amount         *big.Int
	RefundedBidder common.Address
	RefundedAmount *big.Int
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterBidPlaced is a free log retrieval operation binding the contract event 0x2e296671c28b83e813c76e2acf7481f5a2cc46aaeb9bcf33b3e048f50e9c33e9.
//
// Solidity: event BidPlaced(uint256 indexed auctionId, address indexed bidder, uint256 amount, address refundedBidder, uint256 refundedAmount)
func (_Ethereum *EthereumFilterer) FilterBidPlaced(opts *bind.FilterOpts, auctionId []*big.Int, bidder []common.Address) (*EthereumBidPlacedIterator, error) {

	var auctionIdRule []interface{}
	for _, auctionIdItem := range auctionId {
		auctionIdRule = append(auctionIdRule, auctionIdItem)
	}
	var bidderRule []interface{}
	for _, bidderItem := range bidder {
		bidderRule = append(bidderRule, bidderItem)
	}

	logs, sub, err := _Ethereum.contract.FilterLogs(opts, "BidPlaced", auctionIdRule, bidderRule)
	if err != nil {
		return nil, err
	}
	return &EthereumBidPlacedIterator{contract: _Ethereum.contract, event: "BidPlaced", logs: logs, sub: sub}, nil
}

// WatchBidPlaced is a free log subscription operation binding the contract event 0x2e296671c28b83e813c76e2acf7481f5a2cc46aaeb9bcf33b3e048f50e9c33e9.
//
// Solidity: event BidPlaced(uint256 indexed auctionId, address indexed bidder, uint256 amount, address refundedBidder, uint256 refundedAmount)
func (_Ethereum *EthereumFilterer) WatchBidPlaced(opts *bind.WatchOpts, sink chan<- *EthereumBidPlaced, auctionId []*big.Int, bidder []common.Address) (event.Subscription, error) {

	var auctionIdRule []interface{}
	for _, auctionIdItem := range auctionId {
		auctionIdRule = append(auctionIdRule, auctionIdItem)
	}
	var bidderRule []interface{}
	for _, bidderItem := range bidder {
		bidderRule = append(bidderRule, bidderItem)
	}

	logs, sub, err := _Ethereum.contract.WatchLogs(opts, "BidPlaced", auctionIdRule, bidderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(EthereumBidPlaced)
				if err := _Ethereum.contract.UnpackLog(event, "BidPlaced", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseBidPlaced is a log parse operation binding the contract event 0x2e296671c28b83e813c76e2acf7481f5a2cc46aaeb9bcf33b3e048f50e9c33e9.
//
// Solidity: event BidPlaced(uint256 indexed auctionId, address indexed bidder, uint256 amount, address refundedBidder, uint256 refundedAmount)
func (_Ethereum *EthereumFilterer) ParseBidPlaced(log types.Log) (*EthereumBidPlaced, error) {
	event := new(EthereumBidPlaced)
	if err := _Ethereum.contract.UnpackLog(event, "BidPlaced", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
