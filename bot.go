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

func (bot *Bot) ReplyingOnMessages(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = bot.cfg.TelegramMessageTimeOutInSec

	updates := bot.tgClient.GetUpdatesChan(u)
	for update := range updates {
		msg, err := bot.GetMessageByUpdate(ctx, &update)
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

func (bot *Bot) GetMessageByUpdate(ctx context.Context, update *tgbotapi.Update) (*tgbotapi.MessageConfig, error) {
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
		err := bot.Unsubscribe(ctx, update.Message.Chat.ID)
		if err != nil {
			log.Fatal(err)
		}
	}
	if update.Message.Location != nil {
		err := bot.Subscribe(ctx, update.Message)
		if err != nil {
			return nil, err
		}

		getWeatherResponse, err := bot.weatherClient.GetWeatherForecast(update.Message.Location.Latitude, update.Message.Location.Longitude)
		if err != nil {
			return nil, err
		}

		msg.Text = MapGetWeatherResponseHTML(getWeatherResponse)
		msg.ParseMode = "HTML"
	}
	return &msg, nil
}

func (bot *Bot) Subscribe(ctx context.Context, message *tgbotapi.Message) error {
	sub := Subscription{
		ChatId: message.Chat.ID,
		Lat:    message.Location.Latitude,
		Lon:    message.Location.Longitude,
	}
	_, err := bot.db.UpsertOne(ctx, sub)
	return err
	//fix getweatherreponse
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

func (bot *Bot) PushWeatherUpdates(ctx context.Context, upd *tgbotapi.Update) {
	ticker := time.NewTicker(time.Second * 10)
	ans := tgbotapi.NewMessage(upd.Message.Chat.ID, upd.Message.Text)

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
		}
		for _, s := range subs {
			pushAns, err := bot.weatherClient.GetWeatherForecast(s.Lat, s.Lon)
			if err != nil {
				log.Error(err)
				return
			}
			_, err = bot.tgClient.Send(ans)
			if err != nil {
				log.Error(err)
				return
			}

			ans.Text = MapGetWeatherResponseHTML(pushAns)
			ans.ParseMode = "HTML"
		}

		/*pushAns, err := bot.weatherClient.GetWeatherForecast(subs)
		//for each on subs for each sub use Send
		if err != nil {
			log.Error(err)
			return
		}

		_, err = bot.tgClient.Send(ans)
		if err != nil {
			log.Error(err)
			return
		}

		ans.Text = MapGetWeatherResponseHTML(pushAns)
		ans.ParseMode = "HTML"*/

	}
}

func MapGetWeatherResponseHTML(list *GetWeatherResponse) string {
	message := "<b>%s</b>: <b>%.2fdegC</b>\n" + "Feels like <b>%.2fdegC</b>.%s\n"

	reply := fmt.Sprintf(message, list.Name, list.Main.Temp, list.Main.Temp, list.Weather[0].Description)
	return reply
}
