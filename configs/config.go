package config

import (
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Bot struct {
		Token   string `mapstructure:"token"`
		AdminID int64  `mapstructure:"admin_id"`
	} `mapstructure:"bot"`
	Database struct {
		Driver string `mapstructure:"driver"`
		DSN    string `mapstructure:"dsn"`
	} `mapstructure:"database"`
	Marzban struct {
		APIURL   string `mapstructure:"api_url"`
		APIKey   string `mapstructure:"api_key"`
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
	} `mapstructure:"marzban"`
	Payments struct {
		WebhookSecret string `mapstructure:"webhook_secret"`
		Provider      string `mapstructure:"provider"`
	} `mapstructure:"payments"`
	App struct {
		TestPeriodDays        int `mapstructure:"test_period_days"`
		DefaultTrafficLimitGB int `mapstructure:"default_traffic_limit_gb"`
		CheckIntervalMinutes  int `mapstructure:"check_interval_minutes"`
	} `mapstructure:"app"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("configs") // Поиск файла конфигурации в текущей директории

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	log.Println("Конфигурация загружена успешно")
	return &config, nil
}
