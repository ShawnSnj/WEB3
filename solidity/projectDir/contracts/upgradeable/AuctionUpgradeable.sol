pragma solidity ^0.8.20;

import "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/access/OwnableUpgradeable.sol";

contract AuctionUpgradeable is UUPSUpgradeable, OwnableUpgradeable {
    uint256 public data;
    function initialize(uint256 _data) public initializer {
        __Ownable_init();
    }
    function _authorizeUpgrade(
        address newImplementation
    ) internal override onlyOwner {}
    function setData(uint256 _data) public onlyOwner {
        data = _data;
    }
}
