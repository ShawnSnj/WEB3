// SPDX-License-Identifier: MIT
pragma solidity >0.8.0 <0.9.0;

contract MergeSortedArrays{
    function merge(uint[] memory arr1, uint[] memory arr2) public pure returns (uint[] memory) {
        uint  i = 0;
        uint  j=0;
        uint  k=0;
        uint  n = arr1.length;
        uint  m = arr2.length;

        uint[] memory arr3 = new uint[](n+m);

        while(i<n && j<m){
            if(arr1[i]<=arr2[j]){
                arr3[k] = arr1[i];
                i++;
            }else{
                arr3[k] = arr2[j];
                j++;
            }
            k++;
        }

        while (i < n) {
            arr3[k] = arr1[i];
            i++;
            k++;
        }
        while (j < m){
            arr3[k]=arr2[j];
            j++;
            k++;
        }

        return arr3;
    }
}
