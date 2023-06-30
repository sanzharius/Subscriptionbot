package main

import (
	"encoding/json"
	"fmt"
	"github.com/caarlos0/env/v8"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

type Config struct {
	TelegramHost                string `env:"TELEGRAM_HOST"`
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
}

type GetWeatherResponse struct {
	Name    string        `json:"name"`
	Main    *MainForecast `json:"main"`
	Wind    *Wind         `json:"wind"`
	Weather []*Weather    `json:"weather"`
}

type Weather struct {
	Id          int    `json:"id,omitempty"`
	Main        string `json:"main"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

type MainForecast struct {
	Temp      float64 `json:"temp"`
	FeelsLike float64 `json:"feels_like"`
	TempMin   float64 `json:"temp_min"`
	TempMax   float64 `json:"temp_max"`
	Pressure  int     `json:"pressure"`
	Humidity  int     `json:"humidity"`
}

type Wind struct {
	Speed float64 `json:"speed"`
	Deg   int     `json:"deg"`
}

type WeatherClient struct {
	Config *Config
	client *http.Client
}

type Message struct {
	Id  *tgbotapi.Chat
	Loc *tgbotapi.Location
}

func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: time.Minute * 10,
	}
}

func NewWeatherClient(config *Config, httpClient *http.Client) *WeatherClient {
	return &WeatherClient{
		Config: config,
		client: httpClient,
	}
}

func NewConfig(path string) (*Config, error) {
	err := godotenv.Load(path)
	if err != nil {
		log.Fatal(err)
	}
	port := os.Getenv("PORT")
	weatherApiHost := os.Getenv("WEATHERAPIHOST")
	dbHost := os.Getenv("MONGODB_URI")
	fmt.Printf("Port: %s; WeatherApiHost: %s; DbHost: %s", port, weatherApiHost, dbHost)

	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		fmt.Printf("%+v\n", err)
	}
	fmt.Printf("%+v\n", cfg)
	return &cfg, nil
}

func (weatherClient *WeatherClient) AppendQueryParamsToGetWeather(loc []*Subscription) (parsed string) {
	URL, err := url.Parse(weatherClient.Config.WeatherApiHost)
	if err != nil {
		log.Fatal(err)
	}

	r := url.Values{}
	r.Add("appid", weatherClient.Config.AppId)
	r.Add("lat", fmt.Sprint(loc[0].Loc.Latitude))
	r.Add("lon", fmt.Sprint(loc[1].Loc.Longitude))
	r.Add("units", "metric")

	URL.RawQuery = r.Encode()
	parsed = URL.String()
	return parsed
}

func (weatherClient *WeatherClient) GetWeatherForecast(loc []*Subscription) (*GetWeatherResponse, error) {
	weatherURL := weatherClient.AppendQueryParamsToGetWeather(loc)
	resp, err := weatherClient.client.Get(weatherURL)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		switch resp.StatusCode {
		case http.StatusNotFound:
			log.Fatalf("couldn't unmarshal a response: %s", err)
		default:
			log.Fatalf("Got unknown err, while calling API to get weather forecast. HTTP code: %s", err)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("couldn't unmarshal a response: %s", err)
	}
	var list GetWeatherResponse
	err = json.Unmarshal(body, &list)
	if err != nil {
		log.Fatalf("couldn't unmarshal a response: %s", err)
	}
	log.Printf("%+v\n", list)
	return &list, nil
}
