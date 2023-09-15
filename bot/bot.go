package bot

import (
	"context"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strings"
	"subscriptionbot/apperrors"
	"subscriptionbot/config"
	"subscriptionbot/database"
	"subscriptionbot/httpclient"
	"time"
)

var subKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("/subscribe"),
		tgbotapi.NewKeyboardButton("/unsubscribe")),
)

const layout = time.TimeOnly

type Bot struct {
	cfg           *config.Config
	weatherClient *httpclient.WeatherClient
	tgClient      *tgbotapi.BotAPI
	db            *database.SubscriptionStorage
	subs          *database.Subscription
}

func NewBot(config *config.Config, weatherClient *httpclient.WeatherClient, tgClient *tgbotapi.BotAPI, db *database.SubscriptionStorage) (*Bot, error) {
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
	txt := tgbotapi.NewMessage(update.Message.Chat.ID, "send your location to subscribe and then write UTC time")
	txt2 := tgbotapi.NewMessage(update.Message.Chat.ID, "You are unsubscribed")

	switch update.Message.Text {
	case "/start":
		msg.ReplyMarkup = subKeyboard
	case "/close":
		msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
	case subKeyboard.Keyboard[0][0].Text:
		log.Printf("message: %s\n", update.Message.Text)
		if _, err := bot.tgClient.Send(txt); err != nil {
			return nil, apperrors.MessageUnmarshallingError.AppendMessage(err)
		}
	case subKeyboard.Keyboard[0][1].Text:
		log.Printf("message: %s\n", update.Message.Text)
		if _, err := bot.tgClient.Send(txt2); err != nil {
			return nil, apperrors.MessageUnmarshallingError.AppendMessage(err)
		}
		err := bot.Unsubscribe(ctx, update.Message.Chat.ID)
		if err != nil {
			return nil, apperrors.MongoDBDataNotFoundErr.AppendMessage(err)
		}

	}

	if update.Message.Location != nil {
		err := bot.Subscribe(ctx, update.Message)
		if err != nil {
			return nil, apperrors.MongoDBUpdateErr.AppendMessage(err)
		}
	}

	if parsedTime, err := parseTime(update.Message.Text); err != nil {
		err := bot.UpdateTimeSubscriptionByChatID(ctx, update.Message.Chat.ID, parsedTime)
		if err != nil {
			return nil, apperrors.MongoDBUpdateErr.AppendMessage(err)
		}
	}

	return &msg, nil
}

func (bot *Bot) Subscribe(ctx context.Context, message *tgbotapi.Message) error {
	sub := database.Subscription{
		ChatId: message.Chat.ID,
		Lat:    message.Location.Latitude,
		Lon:    message.Location.Longitude,
	}

	_, err := bot.db.UpsertOne(ctx, &sub)
	if err != nil {
		return apperrors.MongoDBUpdateErr.AppendMessage(err)
	}

	return nil
}

func (bot *Bot) UpdateTimeSubscriptionByChatID(ctx context.Context, chatID int64, updateTime time.Time) error {
	primitiveUpdatedTime := primitive.NewDateTimeFromTime(updateTime)
	sub := database.Subscription{
		ChatId:     chatID,
		UpdateTime: primitiveUpdatedTime,
	}

	_, err := bot.db.UpsertOne(ctx, &sub)
	if err != nil {
		return apperrors.MongoDBUpdateErr.AppendMessage(err)
	}

	return nil
}

func (bot *Bot) Unsubscribe(ctx context.Context, chatId int64) error {
	sub, err := bot.db.FindSubscriptionByChatID(ctx, chatId)
	if err != nil {
		return apperrors.MongoDBFindOneErr.AppendMessage(err)
	}

	err = bot.db.DeleteOne(ctx, sub.ID)
	if err != nil {
		return apperrors.MongoDBDataNotFoundErr.AppendMessage(err)
	}

	return nil
}

func (bot *Bot) GetSubscriptions(ctx context.Context, update primitive.DateTime) ([]*database.Subscription, error) {
	filter := bson.D{{"update_time", update}}
	subs, err := bot.db.Find(ctx, filter)
	if err != nil {
		log.Error(err)
		return nil, apperrors.MongoDBFindErr.AppendMessage(err)
	}

	return subs, nil
}

func (bot *Bot) PushWeatherUpdates(ctx context.Context) {

	ticker := time.NewTicker(time.Minute * 1)

	for range ticker.C {

		subs, err := bot.GetSubscriptions(ctx, bot.subs.UpdateTime)
		if err != nil {
			log.Error(err)
		}

		err = bot.SendWeatherUpdate(subs)
		if err != nil {
			log.Error(err)
			return
		}
	}
}

func (bot *Bot) SendWeatherUpdate(sub []*database.Subscription) error {
	pushAns, err := bot.weatherClient.GetWeatherForecast(sub[2].Lat, sub[3].Lon)
	if err != nil {
		return apperrors.MessageUnmarshallingError.AppendMessage(err)
	}

	reply := MapGetWeatherResponseHTML(pushAns)
	message := tgbotapi.NewMessage(sub[0].ChatId, reply)
	message.ParseMode = "HTML"
	_, err = bot.tgClient.Send(message)
	if err != nil {
		return apperrors.MessageUnmarshallingError.AppendMessage(err)
	}

	return apperrors.DataNotFoundErr.AppendMessage(err)
}

func MapGetWeatherResponseHTML(list *httpclient.GetWeatherResponse) string {
	message := "<b>%s</b>: <b>%.2fdegC</b>\n" + "Feels like <b>%.2fdegC</b>.%s\n"

	reply := fmt.Sprintf(message, list.Name, list.Main.Temp, list.Main.Temp, list.Weather[0].Description)
	return reply
}

func parseTime(text string) (time.Time, error) {
	fmt.Println("text=", text)
	inputTime := strings.TrimSpace(text)
	fmt.Println("inputTime=", inputTime)
	parsedTime, err := time.Parse(layout, inputTime)
	fmt.Println("parsedTime=", parsedTime)
	if err != nil {
		log.Error(err)
		return time.Time{}, err
	}

	parsedTime = parsedTime.UTC()
	return parsedTime, nil
}
