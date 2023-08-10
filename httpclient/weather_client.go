package httpclient

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"net/url"
	"subscriptionbot/apperrors"
	"subscriptionbot/config"
)

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
	Config *config.Config
	client *http.Client
}

func NewWeatherClient(config *config.Config, httpClient *http.Client) *WeatherClient {
	return &WeatherClient{
		Config: config,
		client: httpClient,
	}
}

func (weatherClient *WeatherClient) AppendQueryParamsToGetWeather(latitude float64, longitude float64) (parsed string) {
	URL, err := url.Parse(weatherClient.Config.WeatherApiHost)
	if err != nil {
		log.Fatal(apperrors.ConfigReadErr.AppendMessage(err))
	}

	r := url.Values{}
	r.Add("appid", weatherClient.Config.AppId)
	r.Add("lat", fmt.Sprint(latitude))
	r.Add("lon", fmt.Sprint(longitude))
	r.Add("units", "metric")

	URL.RawQuery = r.Encode()
	parsed = URL.String()
	return parsed
}

func (weatherClient *WeatherClient) GetWeatherForecast(latitude float64, longitude float64) (*GetWeatherResponse, error) {
	weatherURL := weatherClient.AppendQueryParamsToGetWeather(latitude, longitude)
	resp, err := weatherClient.client.Get(weatherURL)
	if err != nil {
		return nil, apperrors.MessageUnmarshallingError.AppendMessage(err)
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
			return nil, apperrors.DataNotFoundErr.AppendMessage(err)
		default:
			errMsg := fmt.Sprintf("Got unknown err, while calling API to get weather forecast.HTTP code: %v", resp.StatusCode)
			return nil, apperrors.APICallingErr.AppendMessage(errMsg)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.MessageUnmarshallingError.AppendMessage(err)
	}
	var list GetWeatherResponse
	err = json.Unmarshal(body, &list)
	if err != nil {
		return nil, apperrors.MessageUnmarshallingError.AppendMessage(err)
	}
	log.Printf("%+v\n", list)
	return &list, nil
}
