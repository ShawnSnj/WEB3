// SPDX-License-Identifier: MIT
pragma solidity >0.8.0 <0.9.0;

contract DigitsToRomanConvert{
    function convertDigitsToRoman(uint256 number) public pure returns(string memory) {
        require(number > 0 && number < 4000, "Number must be between 1 and 3999"); // Roman numerals only go up to 3999

        uint16[13] memory values = [1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1];
        string[13] memory romans = [
            "M", "CM", "D", "CD", "C", "XC", "L",
            "XL", "X", "IX", "V", "IV", "I"
        ];

        bytes memory result;
        for (uint i=0; i<values.length; i++) 
        {
            while(number>=values[i]){
                result = abi.encodePacked(result, romans[i]);
                number -= values[i]; 
            }
        }
        return string(result);
    }
}
