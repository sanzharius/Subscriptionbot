package main

import (
	"context"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func main() {
	cfg, err := NewConfig(".env")
	if err != nil {
		log.Fatal(err)
	}

	InitLog(cfg)

	httpWeatherClient := NewHTTPClient()
	weatherClient := NewWeatherClient(cfg, httpWeatherClient)

	db := NewDbClient(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	httpTgClient := NewHTTPClient()
	tgClient, err := tgbotapi.NewBotAPIWithClient(cfg.TelegramBotTok, "https://api.telegram.org/bot%s/%s", httpTgClient)
	if err != nil {
		log.Fatal(err)
	}
	tgClient.Debug = true

	tgBot, err := NewBot(cfg, weatherClient, tgClient, db)
	if err != nil {
		log.Fatal(err)
	}

	tgBot.ReplyingOnMessages(ctx, db.Message)
	errCh := make(chan error)
	go func() {
		if _, err := tgBot.PushWeatherUpdates(ctx, db.Message); err != nil {
			errCh <- err
		}
	}()

	err = <-errCh
	if err != nil {
		log.Println("error occurred while pushing updates", err)
	}
	//go tgBot.PushWeatherUpdates(ctx, db.Message)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", cfg.Port), nil))
}
