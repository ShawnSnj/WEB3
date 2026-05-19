package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"defi-pnl/internal/storage"
)

const startMessageTemplate = `🔥 Smart Money Signals

Subscription:
$10 / month

Accepted payment:
USDT on Arbitrum only

Wallet:%s

⚠️ Do NOT send from other networks.

After payment send:

/paid your_tx_hash`

func startMessage() string {
	wallet := os.Getenv("PAYMENT_WALLET")
	if wallet == "" {
		wallet = "(payment wallet not configured)"
	}
	return fmt.Sprintf(startMessageTemplate, wallet)
}

func StartTelegramBot() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Printf("telegram: TELEGRAM_BOT_TOKEN is empty, bot disabled")
		return
	}
	log.Printf("telegram: started, polling getUpdates")

	url := fmt.Sprintf(
		"https://api.telegram.org/bot%s/getUpdates",
		token,
	)

	offset := 0

	for {
		resp, err := http.Get(
			url + fmt.Sprintf("?offset=%d", offset),
		)

		if err != nil {
			log.Printf("telegram: getUpdates: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

		var data map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&data)

		resp.Body.Close()

		results, ok := data["result"].([]interface{})
		if !ok {
			time.Sleep(2 * time.Second)
			continue
		}

		for _, r := range results {
			update, ok := r.(map[string]interface{})
			if !ok {
				continue
			}

			if id, ok := update["update_id"].(float64); ok {
				offset = int(id) + 1
			}

			msg, ok := update["message"].(map[string]interface{})
			if !ok {
				continue
			}
			chat, ok := msg["chat"].(map[string]interface{})
			if !ok {
				continue
			}
			rawID, ok := chat["id"].(float64)
			if !ok {
				continue
			}
			chatID := int64(rawID)

			text, _ := msg["text"].(string)
			if text == "" {
				continue
			}

			handleCommand(chatID, text)
		}

		time.Sleep(2 * time.Second)
	}
}

func handleCommand(chatID int64, text string) {
	switch {
	case text == "/start":
		log.Printf("telegram: cmd=/start chat=%d", chatID)
		handleStart(chatID)
	case strings.HasPrefix(text, "/paid"):
		log.Printf("telegram: cmd=/paid chat=%d", chatID)
		handlePaid(chatID, text)
	}
}

func handleStart(chatID int64) {
	paymentWallet := os.Getenv("PAYMENT_WALLET")
	if err := storage.UpsertSubscriberOnStart(chatID, paymentWallet); err != nil {
		log.Printf("telegram: /start upsert chat=%d: %v", chatID, err)
	}
	sendTelegramMessage(chatID, startMessage())
}

func handlePaid(chatID int64, text string) {
	parts := strings.Fields(text)
	if len(parts) != 2 {
		sendTelegramMessage(chatID, "Usage: /paid your_tx_hash")
		return
	}
	txHash := parts[1]

	paymentWallet := os.Getenv("PAYMENT_WALLET")
	if paymentWallet == "" {
		log.Printf("telegram: /paid chat=%d: PAYMENT_WALLET not configured", chatID)
		sendTelegramMessage(chatID, "Payments are temporarily unavailable. Please try again later.")
		return
	}

	if err := storage.SavePaymentTx(chatID, txHash, paymentWallet); err != nil {
		log.Printf("telegram: /paid save chat=%d hash=%s: %v", chatID, txHash, err)
		sendTelegramMessage(chatID, "Could not record payment, please try again later.")
		return
	}
	sendTelegramMessage(chatID, "Payment submitted. Waiting for verification.")
}

// SendMessage delivers a plain-text Telegram message to a single chat.
// Exposed so other packages (e.g. the daily signals job) can reuse the same
// send pipeline without re-implementing the Bot API call.
func SendMessage(chatID int64, text string) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")

	url := fmt.Sprintf(
		"https://api.telegram.org/bot%s/sendMessage",
		token,
	)

	body := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	jsonBody, _ := json.Marshal(body)

	http.Post(
		url,
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
}

// sendTelegramMessage is kept as the lowercase alias used inside this package
// (handlers, etc.) so the existing call sites stay short.
func sendTelegramMessage(chatID int64, text string) { SendMessage(chatID, text) }
