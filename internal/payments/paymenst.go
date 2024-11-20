package payments

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go-vpn-bot/internal/database"
)

type PaymentNotification struct {
	UserID int64   `json:"user_id"`
	Amount float64 `json:"amount"`
}

func HandleWebhook(db *database.DB, w http.ResponseWriter, r *http.Request) {
	var notification PaymentNotification

	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Обновляем баланс пользователя
	err := db.UpdateUserBalance(notification.UserID, notification.Amount)
	if err != nil {
		http.Error(w, "Failed to update balance", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Balance updated for user %d", notification.UserID)
}
