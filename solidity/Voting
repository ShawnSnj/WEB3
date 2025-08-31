// SPDX-License-Identifier: MIT
pragma solidity >=0.8.0 < 0.9.0;

contract Voting{
    mapping(string => uint256) private votes;
    string[] private candidates;
    mapping(string => bool) private isCandidate;

   function vote(string memory candidate) public{
            if (!isCandidate[candidate]){
                candidates.push(candidate);
                isCandidate[candidate] = true;
            }
            votes[candidate] += 1;
   }

   function getVotes(string memory candidate) public view returns (uint256){
           return votes[candidate];
   }


   address public owner;

   constructor(){
       owner = msg.sender;
   }

   function resetVotes() public{
    require(msg.sender == owner, "Only owner can reset votes");
    for (uint256 i = 0; i < candidates.length; i++) {
        votes[candidates[i]] = 0;
    }
   }


   function getAllCandidates() public view returns (string[] memory){
      return candidates;
   }

   function reverseString(string memory str) public pure returns (string memory){
      bytes memory strBytes = bytes(str);
      uint len = strBytes.length;
      for (uint i=0;i<len/2;i++){
        bytes1 temp = strBytes[i];
        strBytes[i] = strBytes[len-i-1];
        strBytes[len-i-1] = temp;
      }
      return string(strBytes);
   }

   
}
