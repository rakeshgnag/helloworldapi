package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
	"github.com/joho/godotenv"
)


type TrafficResponse struct {
	Level string `json:"level"`
	Note  string `json:"note"`
}

type CitySuggestion struct {
	Name    string `json:"name"`
	Country string `json:"country"`
}

type geoAPIResponse []struct {
	Name    string `json:"name"`
	Country string `json:"country"`
}


type WeatherResponse struct {
	City       string  `json:"city"`
	Country    string  `json:"country"`
	Temp       float64 `json:"temp"`
	FeelsLike  float64 `json:"feels_like"`
	Humidity   int     `json:"humidity"`
	Condition  string  `json:"condition"`
}

// OpenWeather raw response (partial)
type openWeatherAPIResponse struct {
	Name string `json:"name"`
	Coord struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"coord"`
	Sys struct {
		Country string `json:"country"`
	} `json:"sys"`
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		Humidity  int     `json:"humidity"`
	} `json:"main"`
	Weather []struct {
		Description string `json:"description"`
	} `json:"weather"`
}


type uvAPIResponse struct {
	Current struct {
		UVI float64 `json:"uvi"`
	} `json:"current"`
}

type UVResponse struct {
	Value float64 `json:"value"`
	Risk  string  `json:"risk"`
}

type AQIResponse struct {
	AQI      int    `json:"aqi"`
	Category string `json:"category"`
}

type waqiAPIResponse struct {
	Status string `json:"status"`
	Data   struct {
		AQI int `json:"aqi"`
	} `json:"data"`
}

type CityInfoResponse struct {
	City       string          `json:"city"`
	Country    string          `json:"country"`
	Weather    WeatherResponse `json:"weather"`
	AirQuality AQIResponse     `json:"air_quality"`
	UV         UVResponse      `json:"uv_index"`
		Traffic    TrafficResponse `json:"traffic"`

}

func citySearchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "query parameter is required", http.StatusBadRequest)
		return
	}

	apiKey := os.Getenv("OPENWEATHER_API_KEY")
	if apiKey == "" {
		http.Error(w, "API key not configured", http.StatusInternalServerError)
		return
	}

	geoURL := fmt.Sprintf(
		"https://api.openweathermap.org/geo/1.0/direct?q=%s&limit=5&appid=%s",
		url.QueryEscape(query),
		apiKey,
	)

	resp, err := http.Get(geoURL)
	if err != nil {
		http.Error(w, "Failed to fetch cities", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var raw geoAPIResponse
	json.NewDecoder(resp.Body).Decode(&raw)

	var cities []CitySuggestion
	for _, c := range raw {
		cities = append(cities, CitySuggestion{
			Name:    c.Name,
			Country: c.Country,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cities)
}


func uvRisk(uvi float64) string {
	switch {
	case uvi < 3:
		return "Low"
	case uvi < 6:
		return "Moderate"
	case uvi < 8:
		return "High"
	case uvi < 11:
		return "Very High"
	default:
		return "Extreme"
	}
}


func aqiCategory(aqi int) string {
	switch {
	case aqi <= 50:
		return "Good"
	case aqi <= 100:
		return "Moderate"
	case aqi <= 150:
		return "Unhealthy for Sensitive Groups"
	case aqi <= 200:
		return "Unhealthy"
	case aqi <= 300:
		return "Very Unhealthy"
	default:
		return "Hazardous"
	}
}



func weatherHandler(w http.ResponseWriter, r *http.Request) {
	city := r.URL.Query().Get("city")
	if city == "" {
		http.Error(w, "city parameter is required", http.StatusBadRequest)
		return
	}

	apiKey := os.Getenv("OPENWEATHER_API_KEY")
	if apiKey == "" {
		http.Error(w, "API key not configured", http.StatusInternalServerError)
		return
	}

	apiURL := fmt.Sprintf(
		"https://api.openweathermap.org/data/2.5/weather?q=%s&units=metric&appid=%s",
		url.QueryEscape(city),
		apiKey,
	)

	resp, err := http.Get(apiURL)
	if err != nil {
		http.Error(w, "Failed to fetch weather data", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Invalid city or API error", http.StatusBadRequest)
		return
	}

	var raw openWeatherAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		http.Error(w, "Failed to parse weather data", http.StatusInternalServerError)
		return
	}

	result := WeatherResponse{
		City:      raw.Name,
		Country:   raw.Sys.Country,
		Temp:      raw.Main.Temp,
		FeelsLike: raw.Main.FeelsLike,
		Humidity:  raw.Main.Humidity,
		Condition: raw.Weather[0].Description,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func cityInfoHandler(w http.ResponseWriter, r *http.Request) {
	city := r.URL.Query().Get("city")
	if city == "" {
		http.Error(w, "city parameter is required", http.StatusBadRequest)
		return
	}

	// --- WEATHER ---
	weatherKey := os.Getenv("OPENWEATHER_API_KEY")
	if weatherKey == "" {
		http.Error(w, "Weather API key not configured", http.StatusInternalServerError)
		return
	}

	weatherURL := fmt.Sprintf(
		"https://api.openweathermap.org/data/2.5/weather?q=%s&units=metric&appid=%s",
		url.QueryEscape(city),
		weatherKey,
	)

	weatherResp, err := http.Get(weatherURL)
	if err != nil || weatherResp.StatusCode != http.StatusOK {
		http.Error(w, "Failed to fetch weather", http.StatusBadRequest)
		return
	}
	defer weatherResp.Body.Close()

	var weatherRaw openWeatherAPIResponse
	json.NewDecoder(weatherResp.Body).Decode(&weatherRaw)

	uvURL := fmt.Sprintf(
	"https://api.openweathermap.org/data/3.0/onecall?lat=%f&lon=%f&exclude=minutely,hourly,daily,alerts&appid=%s",
	weatherRaw.Coord.Lat,
	weatherRaw.Coord.Lon,
	weatherKey,
)

uvResp, err := http.Get(uvURL)
if err != nil {
	http.Error(w, "Failed to fetch UV index", http.StatusInternalServerError)
	return
}
defer uvResp.Body.Close()

var uvRaw uvAPIResponse
json.NewDecoder(uvResp.Body).Decode(&uvRaw)

uv := UVResponse{
	Value: uvRaw.Current.UVI,
	Risk:  uvRisk(uvRaw.Current.UVI),
}


	weather := WeatherResponse{
		City:      weatherRaw.Name,
		Country:   weatherRaw.Sys.Country,
		Temp:      weatherRaw.Main.Temp,
		FeelsLike: weatherRaw.Main.FeelsLike,
		Humidity:  weatherRaw.Main.Humidity,
		Condition: weatherRaw.Weather[0].Description,
	}

	// --- AQI ---
	waqiKey := os.Getenv("WAQI_API_KEY")
	if waqiKey == "" {
		http.Error(w, "AQI API key not configured", http.StatusInternalServerError)
		return
	}

	aqiURL := fmt.Sprintf(
		"https://api.waqi.info/feed/%s/?token=%s",
		url.QueryEscape(city),
		waqiKey,
	)

	aqiResp, err := http.Get(aqiURL)
	if err != nil {
		http.Error(w, "Failed to fetch AQI", http.StatusInternalServerError)
		return
	}
	defer aqiResp.Body.Close()

	var aqiRaw waqiAPIResponse
	json.NewDecoder(aqiResp.Body).Decode(&aqiRaw)

	aqi := AQIResponse{
		AQI:      aqiRaw.Data.AQI,
		Category: aqiCategory(aqiRaw.Data.AQI),
	}

	response := CityInfoResponse{
	City:       weather.City,
	Country:    weather.Country,
	Weather:    weather,
	AirQuality: aqi,
	UV:         uv,
		Traffic:    trafficLevel(),

	
}


	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func trafficLevel() TrafficResponse {
	now := time.Now()
	hour := now.Hour()
	weekday := now.Weekday()

	isWeekend := weekday == time.Saturday || weekday == time.Sunday

	level := "Low"

	if isWeekend {
		switch {
		case hour >= 11 && hour <= 20:
			level = "Moderate"
		default:
			level = "Low"
		}
	} else {
		switch {
		case hour >= 8 && hour <= 10:
			level = "High"
		case hour >= 17 && hour <= 20:
			level = "High"
		case hour >= 11 && hour <= 16:
			level = "Moderate"
		default:
			level = "Low"
		}
	}

	return TrafficResponse{
		Level: level,
		Note:  "Estimated based on time and day (weekend/weekday)",
	}
}





func main() {
	godotenv.Load() 

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("DailyWeather API running. Use /weather?city=CityName"))
	})

	http.HandleFunc("/weather", weatherHandler)
	http.HandleFunc("/city-info", cityInfoHandler)
	http.HandleFunc("/cities", citySearchHandler)



	log.Println("Server running on port", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

