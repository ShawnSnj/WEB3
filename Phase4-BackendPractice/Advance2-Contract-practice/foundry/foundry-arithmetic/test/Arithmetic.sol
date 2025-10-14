// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../src/Arithmetic.sol";

contract ArithmeticTest is Test {
    Arithmetic arithmetic;

    function setUp() public {
        arithmetic = new Arithmetic();
    }

    function testAdd() public {
        uint256 result = arithmetic.add(10, 20);
        assertEq(result, 30);
    }

    function testSub() public {
        uint256 result = arithmetic.sub(20, 10);
        assertEq(result, 10);
    }
}
