// SPDX-License-Identifier: MIT
pragma solidity >0.8.0 <0.9.0;

import "@openzeppelin/contracts/token/ERC721/extensions/ERC721URIStorage.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract MyNFT is ERC721URIStorage,Ownable{
    constructor(string memory name, string memory symbol)
    ERC721(name,symbol)
    Ownable(msg.sender)
    {}
     uint256 private _tokenIds;
function mintNFT(address recipient, string memory tokenURI) public onlyOwner returns(uint256){
    _tokenIds++;
    uint256 newItemId = _tokenIds;
    _mint(recipient, newItemId);
    _setTokenURI(newItemId, tokenURI);
    return newItemId;
    }

}