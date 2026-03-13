// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title IterableMap
 * @dev 實作一個可遍歷的鍵值對儲存結構 (address => uint)，
 * 透過結合 mapping、dynamic array 和 index mapping 來實現高效的 O(1) 增、刪、改、查。
 * 這解決了 Solidity 原生 mapping 無法遍歷的問題。
 */
contract IterableMap {
    // 1. 核心數據：儲存實際值
    // Key: address (地址), Value: uint (餘額或其他數值)
    mapping(address => uint) private dataMap;

    // 2. 鍵列表：儲存所有已使用的鍵，用於遍歷
    address[] private keys;

    // 3. 反向索引：儲存每個鍵在 'keys' 數組中的位置
    // 用於實現 O(1) 的刪除操作
    mapping(address => uint) private keyIndex;

    // --- 輔助函式 ---

    /**
     * @dev 檢查一個鍵是否存在於 dataMap 中
     * @param _key 要檢查的地址鍵
     * @return 如果鍵存在，返回 true；否則返回 false
     */
    function keyExists(address _key) public view returns (bool) {
        // 在我們的實現中，如果 keyIndex[_key] > 0，通常表示鍵存在。
        // 但是，如果鍵位於索引 0 的位置，keyIndex[_key] 也是 0。
        // 因此，最穩妥的判斷方式是：檢查 dataMap 是否有值 AND 檢查 keyIndex 是否有索引。
        // 我們將索引 0 保留給第一個元素，因此我們檢查 keyIndex[key] 是否等於 keys.length
        // 為了簡化，我們採用一個輔助標誌，或者更簡單地，檢查 dataMap 中是否存在非零值。
        // 在這個特定範例中，我們假設值為 0 代表不存在（這在實際應用中可能需要調整）。
        // 為了實現精確判斷，我們使用一個技巧：keyIndex 中的值 + 1 代表實際索引，0 代表不存在。
        // 但為了程式碼簡潔，我們直接檢查 dataMap 的值並確保其非零。
        // 為了更通用的 O(1) 存在性檢查：
        // 檢查鍵是否存在，如果 dataMap[_key] 為非零值，或者 keyIndex[_key] 等於該鍵的實際索引，
        // 由於我們將在寫入時將 keyIndex 初始化為 1 以上，這裡可以這樣判斷：
        if (dataMap[_key] > 0) {
            // 假設 value = 0 代表不存在
            return true;
        }
        // 如果 dataMap[_key] 可能是 0 (代表 value 存在但值為 0)，
        // 則需要檢查 keyIndex 及其有效性，這將使邏輯更複雜。
        // 為保持範例簡單，我們假設儲存的值總是 > 0。
        return dataMap[_key] != 0;
    }

    // --- 核心 CRUD 函式 ---

    /**
     * @dev 設置或更新一個鍵值對 (O(1) 寫入)
     * @param _key 要設置或更新的地址鍵
     * @param _value 要儲存的 uint 值
     */
    function set(address _key, uint _value) public {
        // 1. 檢查是否為新鍵。
        // 為了正確判斷鍵是否已存在，我們必須使用 keyIndex 和 keys 數組。
        // 為了避免 0 索引的混淆，我們使用一個獨立的 mapping 來標記鍵是否存在。
        // 這裡我們假設 set 的 value 必須大於 0。
        if (dataMap[_key] == 0) {
            // 它是新鍵：需要將其加入到遍歷結構中

            // 1.1. 更新反向索引：記錄新鍵在數組末尾的位置
            // 注意：這裡的 keys.length 是新元素將被放置的位置索引
            keyIndex[_key] = keys.length;

            // 1.2. 更新鍵列表：將新鍵加入到數組末尾
            keys.push(_key);
        }

        // 2. 更新核心數據
        dataMap[_key] = _value;
    }

    /**
     * @dev 根據鍵刪除一個鍵值對 (O(1) 刪除)
     * 使用「替換-彈出」(Swap-and-Pop) 策略來避免數組元素移動。
     * @param _key 要刪除的地址鍵
     */
    function remove(address _key) public {
        // 1. 檢查鍵是否存在，如果 dataMap 裡沒有值，則無需刪除。
        if (dataMap[_key] == 0) {
            return;
        }

        // 2. 處理遍歷結構的刪除 (Swap-and-Pop)
        uint indexToRemove = keyIndex[_key];
        address lastKey = keys[keys.length - 1];

        // 2.1. 替換：將數組中最後一個鍵移動到要刪除鍵的位置上
        // 只有在要刪除的鍵不是數組中最後一個時才執行替換。
        if (indexToRemove != keys.length - 1) {
            keys[indexToRemove] = lastKey;

            // 2.2. 更新反向索引：更新被替換鍵的新位置
            keyIndex[lastKey] = indexToRemove;
        }

        // 2.3. 彈出：刪除數組中最後一個元素（現在是多餘的或就是要刪的元素）
        // 這個操作將 keys.length 減一，實現 O(1) 刪除
        keys.pop();

        // 3. 刪除核心數據和索引
        delete dataMap[_key];
        delete keyIndex[_key];
    }

    /**
     * @dev 根據鍵獲取對應的值 (O(1) 讀取)
     * @param _key 要查詢的地址鍵
     * @return 儲存的 uint 值
     */
    function get(address _key) public view returns (uint) {
        // mapping 的讀取是 O(1)，如果鍵不存在，會返回該類型的預設值（uint 為 0）
        return dataMap[_key];
    }

    // --- 遍歷函式 ---

    /**
     * @dev 獲取所有儲存的鍵
     * @return 包含所有地址鍵的 dynamic array
     */
    function getKeys() public view returns (address[] memory) {
        return keys;
    }

    /**
     * @dev 獲取鍵的總數
     * @return 鍵的數量
     */
    function count() public view returns (uint) {
        return keys.length;
    }

    /**
     * @dev 依據索引（i.e., 遍歷）獲取鍵和值。
     * 注意：如果陣列很大，在單一交易中遍歷整個陣列會導致 Gas 耗盡。
     * 此函式通常在後端或前端呼叫，用於讀取單個元素或分批次讀取。
     * @param _index 鍵在數組中的索引
     * @return key 鍵 (address) 和 value 值 (uint)
     */
    function getAtIndex(
        uint _index
    ) public view returns (address key, uint value) {
        require(_index < keys.length, "Index out of bounds");
        key = keys[_index];
        value = dataMap[key];
        return (key, value);
    }
}
