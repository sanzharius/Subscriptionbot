package config

import (
	"fmt"
	"github.com/caarlos0/env/v8"
	"github.com/joho/godotenv"
	"os"
	"subscriptionbot/apperrors"
)

type Config struct {
	TelegramBotAPI              string `env:"TELEGRAM_BOT_API"`
	TelegramBotTok              string `env:"TELEGRAM_BOT_TOKEN"`
	Port                        string `env:"PORT"`
	WeatherApiHost              string `env:"WEATHER_API_HOST"`
	AppId                       string `env:"APPID"`
	LogLevel                    string `env:"LOGLEVEL"`
	TelegramMessageTimeOutInSec int    `env:"TELEGRAM_MESSAGE_TIME_OUT_IN_SEC"`
	DbHost                      string `env:"MONGODB_URI"`
	Collection                  string `env:"COLLECTION"`
	Db                          string `env:"DB"`
	DbTimeout                   int    `env:"DB_TIMEOUT"`
	ParsedTime                  string
}

func NewConfig(path string) (*Config, error) {
	err := godotenv.Load(path)
	if err != nil {
		return nil, apperrors.ConfigReadErr.AppendMessage(err)
	}
	port := os.Getenv("PORT")
	weatherApiHost := os.Getenv("WEATHERAPIHOST")
	dbHost := os.Getenv("MONGODB_URI")
	fmt.Printf("Port: %s; WeatherApiHost: %s; DbHost: %s", port, weatherApiHost, dbHost)

	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return nil, apperrors.ConfigReadErr.AppendMessage(err)
	}
	fmt.Printf("%+v\n", cfg)
	return &cfg, nil
}
