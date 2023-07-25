package main

import (
	"context"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"time"
)

var subKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("/subscribe"),
		tgbotapi.NewKeyboardButton("/unsubscribe")),
)

type Bot struct {
	cfg           *Config
	weatherClient *WeatherClient
	tgClient      *tgbotapi.BotAPI
	db            *DbClient
}

func NewBot(config *Config, weatherClient *WeatherClient, tgClient *tgbotapi.BotAPI, db *DbClient) (*Bot, error) {
	log.Printf("Authorized on account %s", tgClient.Self.UserName)
	return &Bot{
		cfg:           config,
		weatherClient: weatherClient,
		tgClient:      tgClient,
		db:            db,
	}, nil
}

func (bot *Bot) ReplyingOnMessages(ctx context.Context, message Message) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = bot.cfg.TelegramMessageTimeOutInSec

	updates := bot.tgClient.GetUpdatesChan(u)
	for update := range updates {
		msg, err := bot.GetMessageByUpdate(&update, ctx, message)
		if err != nil {
			log.Error(err)
			continue
		}
		_, err = bot.tgClient.Send(msg)
		if err != nil {
			log.Error(err)
		}
	}
}

func (bot *Bot) GetMessageByUpdate(update *tgbotapi.Update, ctx context.Context, message Message) (*tgbotapi.MessageConfig, error) {
	if update.Message == nil {
		return nil, nil
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
	txt := tgbotapi.NewMessage(update.Message.Chat.ID, "send your location to subscribe")
	txt2 := tgbotapi.NewMessage(update.Message.Chat.ID, "You are unsubscribed")

	switch update.Message.Text {
	case "/start":
		msg.ReplyMarkup = subKeyboard
	case "/close":
		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	case subKeyboard.Keyboard[0][0].Text:
		fmt.Printf("message: %s\n", update.Message.Text)
		if _, err := bot.tgClient.Send(txt); err != nil {
			log.Panic(err)
		}
	case subKeyboard.Keyboard[0][1].Text:
		fmt.Printf("message: %s\n", update.Message.Text)
		if _, err := bot.tgClient.Send(txt2); err != nil {
			log.Panic(err)
		}
		err := bot.Unsubscribe(ctx, message.Id.ID)
		if err != nil {
			log.Fatal(err)
		}
	}
	if update.Message.Location != nil {
		getWeatherResponse, err := bot.Subscribe(ctx, message, update.Message.Location)
		if err != nil {
			return nil, err
		}

		msg.Text = MapGetWeatherResponseHTML(getWeatherResponse)
		msg.ParseMode = "HTML"
	}
	return &msg, nil
}

func (bot *Bot) Subscribe(ctx context.Context, msg Message, loc *tgbotapi.Location) (*GetWeatherResponse, error) {
	sub := Subscription{
		ChatId: msg.Id.ID,
		Lat:    loc.Latitude,
		Lon:    loc.Longitude,
	}
	_, err := bot.db.UpsertOne(ctx, sub)
	return nil, err
}

func (bot *Bot) Unsubscribe(ctx context.Context, chatId int64) error {
	filter := bson.D{{"chat_id", chatId}}
	sub, err := bot.db.FindOne(ctx, filter)
	if err != nil {
		log.Fatal(err)
	}
	err = bot.db.DeleteOne(ctx, sub.ID)
	if err != nil {
	}
	log.Fatal(err)
	return nil
}

func (bot *Bot) GetSubscriptions(ctx context.Context, update int) ([]*Subscription, error) {
	filter := bson.D{{"update_time", update}}
	subs, err := bot.db.Find(ctx, filter)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	return subs, nil
}

func (bot *Bot) PushWeatherUpdates(ctx context.Context, msg Message) (*GetWeatherResponse, error) {
	ticker := time.NewTicker(time.Second * 10)
	ans := tgbotapi.NewMessage(msg.Id.ID, msg.Answer)

	utcLocation, err := time.LoadLocation("UTC")
	if err != nil {
		log.Fatal(err)
	}

	for t := range ticker.C {
		utcTime := t.In(utcLocation)
		min := utcTime.Minute()
		hour := utcTime.Hour()
		updateTime := hour*100 + min
		subs, err := bot.GetSubscriptions(ctx, updateTime)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		pushAns, err := bot.weatherClient.GetWeatherForecast(subs)
		if err != nil {
			log.Error(err)
		}
		_, err = bot.tgClient.Send(ans)
		if err != nil {
			log.Error(err)
		}
		ans.Text = MapGetWeatherResponseHTML(pushAns)
		ans.ParseMode = "HTML"

	}
	return nil, nil
}

func MapGetWeatherResponseHTML(list *GetWeatherResponse) string {
	message := "<b>%s</b>: <b>%.2fdegC</b>\n" + "Feels like <b>%.2fdegC</b>.%s\n"

	reply := fmt.Sprintf(message, list.Name, list.Main.Temp, list.Main.Temp, list.Weather[0].Description)
	return reply
}
