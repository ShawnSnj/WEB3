package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type FlashbotsClient struct {
	url      string
	privKey  *ecdsa.PrivateKey
	signer   common.Address
	http     *http.Client
}

func NewFlashbotsClient(flashbotsURL string, privKey *ecdsa.PrivateKey) *FlashbotsClient {
	return &FlashbotsClient{
		url: flashbotsURL,
		privKey: privKey,
		signer: crypto.PubkeyToAddress(privKey.PublicKey),
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Error   *rpcError      `json:"error"`
	Result  json.RawMessage `json:"result"`
}

type FlashbotsCallBundleResult struct {
	BundleHash   string `json:"bundleHash"`
	StateBlock   string `json:"stateBlockNumber"`
	RawResult    map[string]any
	RevertReason string
}

func (fb *FlashbotsClient) signHeader(payload []byte) (string, error) {
	hash := accounts.TextHash(payload)
	sig, err := crypto.Sign(hash, fb.privKey)
	if err != nil {
		return "", err
	}
	sigHex := "0x" + hex.EncodeToString(sig)
	return fb.signer.Hex() + ":" + sigHex, nil
}

func (fb *FlashbotsClient) postJSONRPC(ctx context.Context, method string, params any) (json.RawMessage, error) {
	bodyObj := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  []any{params},
	}
	bodyBytes, err := json.Marshal(bodyObj)
	if err != nil {
		return nil, err
	}
	sigHeader, err := fb.signHeader(bodyBytes)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fb.url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Flashbots-Signature", sigHeader)

	resp, err := fb.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var b bytes.Buffer
		_, _ = b.ReadFrom(resp.Body)
		return nil, fmt.Errorf("flashbots http %d: %s", resp.StatusCode, b.String())
	}

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("flashbots rpc error: %d %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

func encodeTxs(txs [][]byte) []string {
	out := make([]string, 0, len(txs))
	for _, tx := range txs {
		out = append(out, "0x"+hex.EncodeToString(tx))
	}
	return out
}

// CallBundle executes the bundle via `eth_callBundle`.
// It returns (ok=false) if the relay reports a revert or call error.
func (fb *FlashbotsClient) CallBundle(
	ctx context.Context,
	signedTxs [][]byte,
	blockNumber uint64,
	stateBlockNumber uint64,
) (*FlashbotsCallBundleResult, error) {
	params := map[string]any{
		"txs":             encodeTxs(signedTxs),
		"blockNumber":    fmt.Sprintf("0x%x", blockNumber),
		"stateBlockNumber": fmt.Sprintf("0x%x", stateBlockNumber),
	}
	raw, err := fb.postJSONRPC(ctx, "eth_callBundle", params)
	if err != nil {
		return nil, err
	}

	// The shape varies depending on relay; keep parsing resilient.
	var resultMap map[string]any
	_ = json.Unmarshal(raw, &resultMap)

	out := &FlashbotsCallBundleResult{
		RawResult: resultMap,
	}
	if v, ok := resultMap["bundleHash"].(string); ok {
		out.BundleHash = v
	}
	if v, ok := resultMap["stateBlockNumber"].(string); ok {
		out.StateBlock = v
	}

	// Common error patterns.
	if v, ok := resultMap["error"]; ok && v != nil {
		// Relay usually returns `error` as a string, but keep this resilient.
		if s, ok := v.(string); ok {
			out.RevertReason = s
		} else {
			out.RevertReason = fmt.Sprintf("%v", v)
		}
		if out.RevertReason != "" {
			return out, nil
		}
	}
	if results, ok := resultMap["results"].([]any); ok {
		for _, r := range results {
			if rm, ok := r.(map[string]any); ok {
				if rr, ok := rm["revertReason"].(string); ok && rr != "" {
					out.RevertReason = rr
					return out, nil
				}
			}
		}
	}
	return out, nil
}

// SendBundle submits the bundle via `eth_sendBundle`.
func (fb *FlashbotsClient) SendBundle(
	ctx context.Context,
	signedTxs [][]byte,
	blockNumber uint64,
) (string, error) {
	params := map[string]any{
		"txs":          encodeTxs(signedTxs),
		"blockNumber": fmt.Sprintf("0x%x", blockNumber),
	}
	raw, err := fb.postJSONRPC(ctx, "eth_sendBundle", params)
	if err != nil {
		return "", err
	}

	var resultMap map[string]any
	_ = json.Unmarshal(raw, &resultMap)
	if v, ok := resultMap["bundleHash"].(string); ok && v != "" {
		return v, nil
	}
	// Some relay versions nest under `result`.
	return string(raw), nil
}

