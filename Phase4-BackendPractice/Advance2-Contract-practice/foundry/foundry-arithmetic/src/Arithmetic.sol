pragma solidity ^0.8.20;


contract Arithmetic{
    function add(uint256 a, uint256 b) public pure returns(uint256){
        return a + b;
    }
        function sub(uint256 a, uint256 b) public pure returns(uint256){
        return a - b;
    }
}