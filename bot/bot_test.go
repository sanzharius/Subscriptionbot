package bot

import (
	"context"
	"encoding/json"
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/mock/gomock"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strings"
	"subscriptionbot/config"
	"subscriptionbot/database"
	mock_database "subscriptionbot/get/mocks"
	"subscriptionbot/httpclient"
	"testing"
	"time"
)

type testTransport struct {
	responses []*http.Response
	index     int
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.index >= len(t.responses) {
		return nil, errors.New("no more responses")
	}

	response := t.responses[t.index]
	t.index++
	return response, nil
}

func fakeBotWithWeatherClient(t *testing.T, weatherClient *httpclient.WeatherClient) *Bot {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_database.NewMockSubscriptionRepository(ctrl)
	bot := &Bot{
		db: mockRepo,
	}

	apiToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	testConfig := &config.Config{
		TelegramBotTok: apiToken,
	}

	response, err := generateBotOkJsonApiResponse()
	if err != nil {
		log.Fatal(err)
		return nil
	}

	testTgClientHttp := fakeHTTPBotClient(200, response)
	tgClient, err := tgbotapi.NewBotAPIWithClient(testConfig.TelegramBotTok, "https://api.telegram.org/bot%s/%s", testTgClientHttp)
	if err != nil {
		log.Fatal(err)
	}

	bot, err = NewBot(testConfig, weatherClient, tgClient, bot.db)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	return bot
}

func fakeHTTPBotClient(statusCode int, jsonResponse string) *http.Client {
	return &http.Client{
		Transport: &testTransport{
			responses: []*http.Response{
				{
					StatusCode: statusCode,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(strings.NewReader(jsonResponse)),
				},
			},
			index: 0,
		},
	}
}

func fakeHTTPBotClientWithMultipleResponses(responses []*http.Response) *http.Client {
	return &http.Client{
		Transport: &testTransport{
			responses: responses,
			index:     0,
		},
	}
}

func fakeBotWithWeatherClientMultipleResponses(t *testing.T, weatherClient *httpclient.WeatherClient, responses []*http.Response) *Bot {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_database.NewMockSubscriptionRepository(ctrl)
	bot := &Bot{
		db: mockRepo,
	}

	apiToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	testConfig := &config.Config{
		TelegramBotTok: apiToken,
	}

	testTgClientHttp := fakeHTTPBotClientWithMultipleResponses(responses)
	tgClient, err := tgbotapi.NewBotAPIWithClient(testConfig.TelegramBotTok, "https://api.telegram.org/bot%s/%s", testTgClientHttp)
	if err != nil {
		log.Fatal(err)
	}

	bot, err = NewBot(testConfig, weatherClient, tgClient, bot.db)
	if err != nil {
		log.Fatal(err)
		return nil
	}

	return bot
}

func generateBotOkJsonApiResponse() (string, error) {
	testUser := tgbotapi.User{
		ID:        123,
		FirstName: "John",
		LastName:  "Doe",
	}

	user, err := json.Marshal(&testUser)
	if err != nil {
		return "", err
	}

	testApiResponse := tgbotapi.APIResponse{
		Ok:          true,
		Result:      user,
		ErrorCode:   0,
		Description: "",
		Parameters:  nil,
	}

	response, err := json.Marshal(&testApiResponse)
	if err != nil {
		return "", nil
	}

	return string(response), nil
}

func TestNewBot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_database.NewMockSubscriptionRepository(ctrl)
	bot := &Bot{
		db: mockRepo,
	}
	apiToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	testConfig := &config.Config{
		TelegramBotTok: apiToken,
	}

	response, err := generateBotOkJsonApiResponse()
	if err != nil {
		log.Fatal(err)
		return
	}

	testTgClientHttp := fakeHTTPBotClient(200, response)
	tgClient, err := tgbotapi.NewBotAPIWithClient(testConfig.TelegramBotTok, "https://api.telegram.org/bot%s/%s", testTgClientHttp)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := NewBot(testConfig, nil, tgClient, bot.db); err != nil {
		t.Log(err)
	}
}

func TestReplyingOnMessages(t *testing.T) {
	authResponse, err := generateBotOkJsonApiResponse()
	if err != nil {
		t.Error(err)
		return
	}

	messageAPI := []*tgbotapi.Update{
		{
			UpdateID: 300,
			Message: &tgbotapi.Message{
				MessageID: 100,
				Chat: &tgbotapi.Chat{
					ID: 200,
				},
				Text: "",
				Location: &tgbotapi.Location{
					Longitude: 21.017532,
					Latitude:  52.237049,
				},
			},
		},
	}

	messageAPIResponse, err := json.Marshal(&messageAPI)
	if err != nil {
		t.Error(err)
		return
	}

	apiResponse := &tgbotapi.APIResponse{
		Ok:          true,
		Result:      messageAPIResponse,
		ErrorCode:   0,
		Description: "",
		Parameters:  nil,
	}

	apiResponseJSON, err := json.Marshal(&apiResponse)
	if err != nil {
		t.Error(err)
		return
	}

	ttPass := []struct {
		name                 string
		givenWeatherResponse *httpclient.GetWeatherResponse
		botResponses         []*http.Response
	}{
		{
			"existing location command",
			&httpclient.GetWeatherResponse{
				Name: "Warsaw",
				Main: &httpclient.MainForecast{
					Temp:      10,
					FeelsLike: 15,
				},
				Weather: []*httpclient.Weather{
					{Description: "It's Warsaw, baby :)"},
				},
			},
			[]*http.Response{
				{
					StatusCode: 200,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(strings.NewReader(authResponse)),
				},
				{
					StatusCode: 200,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: io.NopCloser(strings.NewReader(string(apiResponseJSON))),
				},
			},
		},
	}

	testConfig, err := config.NewConfig("../.env")
	if err != nil {
		log.Fatal(err)
	}

	for _, tc := range ttPass {
		responseJSON, err := json.Marshal(tc.givenWeatherResponse)
		if err != nil {
			t.Fatal(err)
		}

		httpWeatherClient := fakeHTTPBotClient(200, string(responseJSON))
		weatherClient := httpclient.NewWeatherClient(testConfig, httpWeatherClient)
		bot := fakeBotWithWeatherClientMultipleResponses(t, weatherClient, tc.botResponses)
		bot.tgClient.Debug = true
		bot.ReplyingOnMessages(nil)
		time.Sleep(time.Second * 1)
	}
}

func TestGetMessageByUpdate(t *testing.T) {
	ttPass := []struct {
		name                 string
		messageChatId        int64
		givenWeatherResponse *httpclient.GetWeatherResponse
		givenMessage         *tgbotapi.Update
		botReply             *tgbotapi.MessageConfig
	}{
		{
			"existing location command",
			300,
			&httpclient.GetWeatherResponse{
				Name: "Warsaw",
				Main: &httpclient.MainForecast{
					Temp:      10,
					FeelsLike: 15,
				},
				Weather: []*httpclient.Weather{
					{Description: "It's Warsaw, baby!"},
				},
			},
			&tgbotapi.Update{
				UpdateID: 0,
				Message: &tgbotapi.Message{
					Chat: &tgbotapi.Chat{
						ID: 300,
					},
					Location: &tgbotapi.Location{
						Longitude: 21.017532,
						Latitude:  52.237049,
					},
				},
			},
			&tgbotapi.MessageConfig{
				Text:      "",
				ParseMode: "HTML",
			},
		},
	}

	testConfig, err := config.NewConfig("../.env")
	if err != nil {
		log.Fatal(err)
	}

	for _, tc := range ttPass {
		responseJSON, err := json.Marshal(tc.givenWeatherResponse)
		if err != nil {
			t.Fatal(err)
		}

		httpClient := fakeHTTPBotClient(200, string(responseJSON))
		weatherClient := httpclient.NewWeatherClient(testConfig, httpClient)
		bot := fakeBotWithWeatherClient(t, weatherClient)
		bot.tgClient.Debug = true
		msg, err := bot.GetMessageByUpdate(context.Background(), tc.givenMessage)
		if err != nil {
			t.Error(err)
		}
		expMessage := MapGetWeatherResponseHTML(tc.givenWeatherResponse)
		if msg != nil && msg.Text != expMessage {
			t.Errorf("bot reply should be %s, but was %s", tc.botReply.Text, msg.Text)
		}
	}

}

func TestSubscribe(t *testing.T) {
	type mockSubBehavior func(r *mock_database.MockSubscriptionRepository, sub *database.Subscription)
	ttPass := []struct {
		name            string
		message         *tgbotapi.Message
		inputSub        *database.Subscription
		mockSubBehavior mockSubBehavior
	}{
		{
			"Ok",
			&tgbotapi.Message{
				Chat: &tgbotapi.Chat{
					ID: 123,
				},
				Location: &tgbotapi.Location{
					Latitude:  52.237049,
					Longitude: 21.017532,
				},
			},
			&database.Subscription{
				ID:     primitive.NewObjectID(),
				ChatId: 123,
				Lat:    52.237049,
				Lon:    21.017532,
			},
			func(r *mock_database.MockSubscriptionRepository, sub *database.Subscription) {
				r.EXPECT().InsertOne(gomock.Any(), sub).Return(primitive.NewObjectID(), nil)
			},
		},
	}

	for _, tc := range ttPass {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockRepo := mock_database.NewMockSubscriptionRepository(ctrl)
			tc.mockSubBehavior(mockRepo, tc.inputSub)
			bot := &Bot{
				db: mockRepo,
			}

			err := bot.Subscribe(context.Background(), tc.message)
			if err != nil {
				t.Errorf("Expected no error, but got error %v", err)
			}
		})
	}

}

func TestUnsubscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	chatID := int64(123)
	mockRepo := mock_database.NewMockSubscriptionRepository(ctrl)
	bot := &Bot{
		db: mockRepo,
	}
	mockRepo.EXPECT().DeleteOne(gomock.Any(), primitive.NilObjectID).Return(nil)
	err := bot.Unsubscribe(context.Background(), chatID)
	if err != nil {
		t.Errorf("")
	}
}

func TestGetSubscriptions(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_database.NewMockSubscriptionRepository(ctrl)
	bot := &Bot{
		db: mockRepo,
	}
	expectedSubscriptions := []*database.Subscription{
		{
			ID:     primitive.ObjectID{},
			ChatId: int64(123),
			Lat:    52.237049,
			Lon:    21.017532,
		},
		{
			ID:     primitive.ObjectID{},
			ChatId: int64(456),
			Lat:    52.237049,
			Lon:    21.017532,
		},
	}
	mockRepo.EXPECT().Find(gomock.Any(), gomock.Any()).Return(expectedSubscriptions, nil)
	subs, err := bot.GetSubscriptions(context.Background())
	if err != nil {
		t.Errorf("Expected no error, but got error: %v", err)
	}

	if !reflect.DeepEqual(subs, expectedSubscriptions) {
		t.Errorf("Subscription do not match expected value")
	}
}

func TestSendWeatherUpdate(t *testing.T) {
	ttPass := []struct {
		name                 string
		givenWeatherResponse *httpclient.GetWeatherResponse
		givenSubscription    *database.Subscription
	}{
		{
			"existing location command",
			&httpclient.GetWeatherResponse{
				Name: "Warsaw",
				Main: &httpclient.MainForecast{
					Temp:      10,
					FeelsLike: 15,
				},
				Weather: []*httpclient.Weather{
					{Description: "It's Warsaw, baby!"},
				},
			},
			&database.Subscription{
				ChatId: int64(123),
				Lat:    52.237049,
				Lon:    21.017532,
			},
		},
	}
	testConfig, err := config.NewConfig("../.env")
	if err != nil {
		log.Fatal(err)
	}

	for _, tc := range ttPass {
		responseJSON, err := json.Marshal(tc.givenWeatherResponse)
		if err != nil {
			t.Fatal(err)
		}

		httpClient := fakeHTTPBotClient(200, string(responseJSON))
		weatherClient := httpclient.NewWeatherClient(testConfig, httpClient)
		bot := fakeBotWithWeatherClient(t, weatherClient)
		bot.tgClient.Debug = true
		subs := bot.SendWeatherUpdate([]*database.Subscription{tc.givenSubscription})
		if !reflect.DeepEqual(subs, tc.givenSubscription) {
			t.Errorf("Subscription do not match given value")
		}
	}
}
