// SPDX-License-Identifier: MIT
pragma solidity ^0.8.28;

/**
 * @title MockV3Aggregator
 * @dev Mocks the Chainlink AggregatorV3Interface for testing purposes.
 */
contract MockV3Aggregator {
    // 價格使用 8 位小數（USD/ETH 預言機常見）
    uint8 public constant decimals = 8;
    int256 private s_answer;

    constructor(int256 initialAnswer) {
        s_answer = initialAnswer;
    }

    function latestRoundData()
        external
        view
        returns (
            uint80 roundId,
            int256 answer,
            uint256 startedAt,
            uint256 updatedAt,
            uint80 answeredInRound
        )
    {
        // 模擬一個有效的最新輪次數據
        return (
            1, // roundId
            s_answer, // answer (the price)
            block.timestamp, // startedAt
            block.timestamp, // updatedAt
            1 // answeredInRound
        );
    }

    /**
     * @notice Allows the owner to set a new mock price.
     */
    function setLatestAnswer(int256 newAnswer) external {
        s_answer = newAnswer;
    }
}
