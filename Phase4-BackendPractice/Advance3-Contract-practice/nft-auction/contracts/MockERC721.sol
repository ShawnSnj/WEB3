// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

import "@openzeppelin/contracts/token/ERC721/ERC721.sol";

/**
 * @title MockERC721
 * @dev Simple ERC721 for testing.
 */
contract MockERC721 is ERC721 {
    uint256 public nextTokenId = 1;

    constructor() ERC721("MockNFT", "MNFT") {}

    /**
     * @notice Mints a new NFT to the recipient.
     */
    function mint(address to) public returns (uint256) {
        uint256 tokenId = nextTokenId++;
        _safeMint(to, tokenId);
        return tokenId;
    }
}
