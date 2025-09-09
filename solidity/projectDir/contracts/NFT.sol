pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC721/extensions/ERC721URIStorage.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract NFT is ERC721URIStorage,Ownable {
 uint256 public nextTokenId;
 constructor() ERC721("AuctionNFT","AUC") Ownable(msg.sender) {}
 function mint(string memory _uri) external{
    _safeMint(msg.sender,nextTokenId);
    _setTokenURI(nextTokenId,_uri);
    nextTokenId++;
 }
}