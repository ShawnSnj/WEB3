// SPDX-License-Identifier: MIT
pragma solidity >=0.8.0 <0.9.0;

contract RomanConvert{
    function romanToInt(string memory s) public pure returns(uint256){
        bytes memory str = bytes(s);
        uint len = str.length;
        uint result = 0;
        uint i = 0;
        while(i < len) {
            uint256 current = valueOf(str[i]);
            if(i<len-1){
                uint256 next = valueOf(str[i+1]);
                if(current < next){
                    result += next - current;
                    i += 2; 
                    continue ;
                }
            }
            result += current;
            i++;
        }
        return result;
    }

    function valueOf(bytes1 c) internal pure returns (uint256) {
        if (c == "I") return 1;
        if (c == "V") return 5;
        if (c == "X") return 10;
        if (c == "L") return 50;
        if (c == "C") return 100;
        if (c == "D") return 500;
        if (c == "M") return 1000;
        revert("Invalid Roman numeral");
    }
}
