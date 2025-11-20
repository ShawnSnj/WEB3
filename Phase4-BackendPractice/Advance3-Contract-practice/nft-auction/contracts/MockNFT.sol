// contracts/MockNFT.sol
pragma solidity ^0.8.28;

import "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract MockNFT is ERC721, Ownable {
    uint256 private _nextTokenId;

    constructor() ERC721("MockNFT", "MNFT") Ownable() {}

    function mintTo(address to) public onlyOwner returns (uint256) {
        uint256 tokenId = _nextTokenId;
        _safeMint(to, tokenId);
        _nextTokenId++;
        return tokenId;
    }

    function totalSupply() public view returns (uint256) {
        return _nextTokenId;
    }
}
