package main

import (
	"context"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
	"net/http"
	"subscriptionbot/bot"
	"subscriptionbot/config"
	"subscriptionbot/database"
	"subscriptionbot/httpclient"
	"subscriptionbot/logger"
)

func main() {
	cfg, err := config.NewConfig(".env")
	if err != nil {
		log.Fatal(err)
	}

	logger.InitLog(cfg)

	httpWeatherClient := httpclient.NewHTTPClient()
	weatherClient := httpclient.NewWeatherClient(cfg, httpWeatherClient)

	db := database.NewSubscriptionStorage(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpTgClient := httpclient.NewHTTPClient()
	tgClient, err := tgbotapi.NewBotAPIWithClient(cfg.TelegramBotTok, "https://api.telegram.org/bot%s/%s", httpTgClient)
	if err != nil {
		log.Fatal(err)
	}
	tgClient.Debug = true

	tgBot, err := bot.NewBot(cfg, weatherClient, tgClient, db)
	if err != nil {
		log.Fatal(err)
	}

	tgBot.ReplyingOnMessages(ctx)
	go tgBot.PushWeatherUpdates(ctx)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", cfg.Port), nil))
}
