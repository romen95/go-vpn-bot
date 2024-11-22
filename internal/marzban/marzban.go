package marzban

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/spf13/viper"
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

func GetAPIKey(apiURL, username, password string) (string, error) {
	apiEndpoint := fmt.Sprintf("%s/api/admin/token", apiURL)

	// Формируем тело запроса
	formData := url.Values{}
	formData.Set("grant_type", "")
	formData.Set("username", username)
	formData.Set("password", password)
	formData.Set("scope", "")
	formData.Set("client_id", "")
	formData.Set("client_secret", "")

	// Создаём HTTP-запрос
	req, err := http.NewRequest("POST", apiEndpoint, bytes.NewBufferString(formData.Encode()))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("accept", "application/json")

	// Выполняем запрос
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка чтения тела ответа: %v", err)
	}

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("неудачный статус ответа: %d, тело: %s", resp.StatusCode, string(respBody))
	}

	// Декодируем JSON-ответ
	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", fmt.Errorf("ошибка обработки ответа: %v", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("токен отсутствует в ответе")
	}

	return tokenResp.AccessToken, nil
}

func UpdateAPIKey(configPath, newAPIKey string) error {
	// Загружаем текущую конфигурацию
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("ошибка чтения конфигурации: %v", err)
	}

	// Обновляем значение APIKey
	viper.Set("marzban.api_key", newAPIKey)

	// Сохраняем изменения в файл
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("ошибка записи конфигурации: %v", err)
	}

	return nil
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
