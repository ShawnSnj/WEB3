pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract MyNFT is ERC721, Ownable {
    uint256 public tokenIdCounter;

    constructor() ERC721("AuctionNFT", "AUC") {}

    function mint(address to) external onlyOwner returns (uint256) {
        uint256 newId = tokenIdCounter;
        _safeMint(to, newId);
        tokenIdCounter++;
        return newId;
    }
}
