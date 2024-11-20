package marzban

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

	fmt.Printf("Создание пользователя в Marzban. URL: %s, Запрос: %s\n", url, string(body))

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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения тела ответа: %v", err)
	}

	fmt.Printf("Ответ от сервера: %s\n", string(respBody))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("неудачный статус ответа: %d, тело: %s", resp.StatusCode, string(respBody))
	}

	// Временная структура для декодирования полного ответа
	var fullResp struct {
		Links []string `json:"links"`
	}

	if err := json.Unmarshal(respBody, &fullResp); err != nil {
		return nil, fmt.Errorf("ошибка обработки ответа: %v", err)
	}

	// Проверяем, есть ли хотя бы одна ссылка
	var firstLink string
	if len(fullResp.Links) > 0 {
		firstLink = fullResp.Links[0]
	} else {
		return nil, fmt.Errorf("в ответе отсутствуют ссылки")
	}

	// Создаём и возвращаем UserResponse с первой ссылкой
	userResp := &UserResponse{
		Success: true,
		Message: firstLink,
	}

	return userResp, nil
}

// DeleteUser отправляет DELETE-запрос для удаления пользователя на сервере Marzban.
func DeleteUser(apiURL, apiKey, username string) error {
	url := fmt.Sprintf("%s/api/user/%s", apiURL, username)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("ошибка создания DELETE-запроса: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка выполнения DELETE-запроса: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("неудачный статус ответа: %d", resp.StatusCode)
	}

	return nil
}
