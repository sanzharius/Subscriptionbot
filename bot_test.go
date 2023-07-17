package main

import (
	"context"
	"encoding/json"
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type TestTransport struct {
	responses []*http.Response
	index     int
}

func (t *TestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.index >= len(t.responses) {
		return nil, errors.New("no more responses")
	}
	response := t.responses[t.index]
	t.index++
	return response, nil
}

func fakeBotWithWeatherClient(weatherClient *WeatherClient, db *DbClient) *Bot {
	apiToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	testConfig := &Config{
		TelegramBotTok: apiToken,
	}

	response, err := generateBotOkApiResponse()
	if err != nil {
		log.Fatal(err)
		return nil
	}

	testTgClientHttp := fakeHTTPBotClient(200, response)
	tgClient, err := tgbotapi.NewBotAPIWithClient(testConfig.TelegramBotTok, "https://api.telegram.org/bot%s/%s", testTgClientHttp)
	if err != nil {
		log.Fatal(err)
	}

	bot, err := NewBot(testConfig, weatherClient, tgClient, db)
	if err != nil {
		log.Fatal(err)
		return nil
	}
	return bot

}
func fakeHTTPBotClient(statusCode int, jsonResponse string) *http.Client {
	return &http.Client{
		Transport: &TestTransport{
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

func generateBotOkApiResponse() (string, error) {
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
		return "", err
	}
	return string(response), nil
}

func fakeHTTPBotClientWithMultipleResponse(response []*http.Response) *http.Client {
	return &http.Client{
		Transport: &TestTransport{
			responses: response,
			index:     0,
		},
	}

}

func fakeBotWithWeatherClientMultipleResponses(weatherClient *WeatherClient, responses []*http.Response, db *DbClient) *Bot {
	apiToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	testConfig := &Config{
		TelegramBotTok: apiToken,
	}

	testTgClientHttp := fakeHTTPBotClientWithMultipleResponse(responses)
	tgClient, err := tgbotapi.NewBotAPIWithClient(testConfig.TelegramBotTok, "https://api.telegram.org/bot%s/%s", testTgClientHttp)
	if err != nil {
		log.Fatal(err)
	}

	bot, err := NewBot(testConfig, weatherClient, tgClient, db)
	if err != nil {
		log.Fatal(err)
		return nil
	}
	return bot
}

func TestNewBot(t *testing.T) {
	apiToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	testConfig := &Config{
		TelegramBotTok: apiToken,
	}

	response, err := generateBotOkApiResponse()
	if err != nil {
		t.Error(err)
		return
	}

	testTgClientHttp := fakeHTTPBotClient(200, response)
	tgClient, err := tgbotapi.NewBotAPIWithClient(testConfig.TelegramBotTok, "https://api.telegram.org/bot%s/%s", testTgClientHttp)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := NewBot(testConfig, nil, tgClient, nil); err != nil {
		t.Log(err)
	}
}

func TestReplyingOnMessages(t *testing.T) {
	authResponse, err := generateBotOkApiResponse()
	if err != nil {
		t.Error(err)
		return
	}

	messageApi := []*tgbotapi.Update{{
		UpdateID: 300,
		Message: &tgbotapi.Message{
			MessageID: 100,
			Chat: &tgbotapi.Chat{
				ID: 250,
			},
			Text: "",
			Location: &tgbotapi.Location{
				Longitude: 76.889709,
				Latitude:  43.238949,
			},
		},
	},
	}

	messageApiResponse, err := json.Marshal(&messageApi)
	if err != nil {
		t.Error(err)
		return
	}

	apiResponse := &tgbotapi.APIResponse{
		Ok:          true,
		Result:      messageApiResponse,
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
		givenWeatherResponse *GetWeatherResponse
		botResponses         []*http.Response
		ctx                  context.Context
		message              Message
	}{
		{
			"existing location command",
			&GetWeatherResponse{
				Name: "Almaty",
				Main: &MainForecast{
					Temp:      30,
					FeelsLike: 35,
				},
				Weather: []*Weather{
					{Description: "Welcome to Almaty"},
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
			nil,
			Message{
				Id: &tgbotapi.Chat{
					ID: 1,
				},
			},
		},
	}

	testConfig, err := NewConfig("../.env")
	if err != nil {
		log.Fatal(err)
	}

	for _, tc := range ttPass {

		responseJSON, err := json.Marshal(tc.givenWeatherResponse)
		if err != nil {
			t.Fatal(err)
		}

		httpWeatherClient := fakeHTTPBotClient(200, string(responseJSON))
		weatherClient := NewWeatherClient(testConfig, httpWeatherClient)
		bot := fakeBotWithWeatherClientMultipleResponses(weatherClient, tc.botResponses, nil)
		bot.tgClient.Debug = true
		bot.ReplyingOnMessages(tc.ctx, tc.message)
		time.Sleep(time.Second * 1)
	}

}

func TestGetMessageByUpdate(t *testing.T) {

	ttPass := []struct {
		name                 string
		messageChatId        int64
		givenWeatherResponse *GetWeatherResponse
		givenMessage         *tgbotapi.Update
		botReply             *tgbotapi.MessageConfig
		ctx                  context.Context
		message              Message
		db                   *DbClient
	}{
		{
			"existing location command",
			300,
			&GetWeatherResponse{
				Name: "Warsaw",
				Main: &MainForecast{
					Temp:      10,
					FeelsLike: 15,
				},
				Weather: []*Weather{
					{Description: "Welcome to Almaty"},
				},
			},
			&tgbotapi.Update{
				UpdateID: 0,
				Message: &tgbotapi.Message{
					Chat: &tgbotapi.Chat{
						ID: 300,
					},
					Location: &tgbotapi.Location{
						Longitude: 76.889709,
						Latitude:  43.238949,
					},
				},
			},
			&tgbotapi.MessageConfig{
				Text:      "",
				ParseMode: "HTML",
			},
			nil,
			Message{
				Id: &tgbotapi.Chat{
					ID: 1,
				},
			},
			&DbClient{
				Message: Message{
					Id: nil,
				},
			},
		},
	}

	testConfig, err := NewConfig("../.env")
	if err != nil {
		log.Fatal(err)
	}

	for _, tc := range ttPass {

		responseJSON, err := json.Marshal(tc.givenWeatherResponse)
		if err != nil {
			t.Fatal(err)
		}

		httpClient := fakeHTTPBotClient(200, string(responseJSON))
		weatherClient := NewWeatherClient(testConfig, httpClient)
		bot := fakeBotWithWeatherClient(weatherClient, tc.db)
		bot.tgClient.Debug = true
		msg, err := bot.GetMessageByUpdate(tc.givenMessage, tc.ctx, tc.message)
		if err != nil {
			t.Error(err)
		}

		expMessage := MapGetWeatherResponseHTML(tc.givenWeatherResponse)
		if msg != nil && msg.Text != expMessage {
			t.Errorf("bot reply should be %s, but got %s", tc.botReply.Text, msg.Text)
		}
	}
}
