// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract MockV3Aggregator {
    int256 private _answer;
    uint8 private _decimals;

    constructor(uint8 decimals_, int256 initialAnswer) {
        _decimals = decimals_;
        _answer = initialAnswer;
    }

    function decimals() external view returns (uint8) {
        return _decimals;
    }

    function latestRoundData()
        external
        view
        returns (
            uint80,
            int256 answer,
            uint256,
            uint256,
            uint80
        )
    {
        return (0, _answer, 0, 0, 0);
    }

    function updateAnswer(int256 newAnswer) external {
        _answer = newAnswer;
    }
}
