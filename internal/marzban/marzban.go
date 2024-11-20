package marzban

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// UserRequest представляет тело запроса для создания нового пользователя.
type UserRequest struct {
	Username  string                 `json:"username"`
	Proxies   map[string]interface{} `json:"proxies"`
	Inbounds  map[string][]string    `json:"inbounds"`
	Expire    int                    `json:"expire"`     // 0 — без ограничения по времени
	DataLimit int                    `json:"data_limit"` // 0 — без ограничения по трафику
}

// UserResponse представляет возможный ответ от API после создания пользователя.
type UserResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// CreateUser отправляет запрос для создания нового пользователя на сервере Marzban.
func CreateUser(apiURL, apiKey, username string) (*UserResponse, error) {
	url := fmt.Sprintf("%s/api/user", apiURL)

	reqBody := UserRequest{
		Username:  username,
		Proxies:   map[string]interface{}{"shadowsocks": map[string]interface{}{}},
		Inbounds:  map[string][]string{"shadowsocks": {"Shadowsocks TCP"}},
		Expire:    0, // Бессрочный доступ
		DataLimit: 0, // Неограниченный трафик
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ошибка формирования запроса: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp UserResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err != nil {
			return nil, fmt.Errorf("ошибка API: не удалось декодировать ответ")
		}
		return &errorResp, fmt.Errorf("ошибка API: %s", errorResp.Message)
	}

	var userResp UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return nil, fmt.Errorf("ошибка обработки ответа: %v", err)
	}

	return &userResp, nil
}
