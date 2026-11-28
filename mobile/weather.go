package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/imhalid/celestine/internal/library"
	"github.com/imhalid/celestine/mobile/locales"
)

// Live local weather & solar times (sunrise/sunset) via Open-Meteo (zero API key, zero GPS permission popup).
// Cached on disk in weather.json and refreshed silently every 30 minutes so the paper stays fast and offline-safe.

type weatherKind string

const (
	weatherClear  weatherKind = "clear"
	weatherPartly weatherKind = "partly"
	weatherCloudy weatherKind = "cloudy"
	weatherFog    weatherKind = "fog"
	weatherRain   weatherKind = "rain"
	weatherStorm  weatherKind = "storm"
	weatherSnow   weatherKind = "snow"
)

const (
	weatherRefresh  = 30 * time.Minute
	locationRefresh = 6 * time.Hour
	weatherHTTPWait = 6 * time.Second
)

// weatherInfo is persisted in weather.json and shapes the paper's sky gradient and solar transition times.
type weatherInfo struct {
	Lat        float64     `json:"lat,omitempty"`
	Lon        float64     `json:"lon,omitempty"`
	City       string      `json:"city,omitempty"`
	LocAt      time.Time   `json:"locAt,omitempty"`
	Kind       weatherKind `json:"kind,omitempty"`
	TempC      float64     `json:"tempC,omitempty"`
	CloudPct   int         `json:"cloudPct,omitempty"`
	SunriseHr  float32     `json:"sunriseHr,omitempty"` // local hour, e.g. 7.45 for 07:27
	SunsetHr   float32     `json:"sunsetHr,omitempty"`  // local hour, e.g. 17.63 for 17:38
	SolarDay   string      `json:"solarDay,omitempty"`  // YYYY-MM-DD of sunrise/sunset
	UpdatedAt  time.Time   `json:"updatedAt,omitempty"`
	HasWeather bool        `json:"hasWeather,omitempty"`
}

type weatherState struct {
	info  weatherInfo
	busy  bool
	tried time.Time
}

func (a *app) weatherFile() string { return filepath.Join(a.dir, "weather.json") }

func (a *app) loadWeather() {
	if b, err := os.ReadFile(a.weatherFile()); err == nil {
		json.Unmarshal(b, &a.wx.info)
	}
}

func (a *app) saveWeather() {
	if b, err := json.Marshal(a.wx.info); err == nil {
		library.WriteFilePrivate(a.weatherFile(), b)
	}
}

// solarWindow returns today's sunrise and sunset hours (local time), falling back to calm seasonal defaults if no
// live weather has been fetched yet.
func (w weatherInfo) solarWindow(now time.Time) (sunrise, sunset float32) {
	if w.SunriseHr > 3 && w.SunriseHr < 11 && w.SunsetHr > 14 && w.SunsetHr < 23 &&
		w.SolarDay == startOfDay(now).Format("2006-01-02") {
		return w.SunriseHr, w.SunsetHr
	}
	return 6.5, 18.25
}

// label returns the localized short weather description, or "" when no live weather is cached.
func (w weatherInfo) label() string {
	if !w.HasWeather {
		return ""
	}
	var name string
	switch w.Kind {
	case weatherClear:
		name = locales.WeatherClear()
	case weatherPartly:
		name = locales.WeatherPartly()
	case weatherCloudy:
		name = locales.WeatherCloudy()
	case weatherFog:
		name = locales.WeatherFog()
	case weatherRain:
		name = locales.WeatherRain()
	case weatherStorm:
		name = locales.WeatherStorm()
	case weatherSnow:
		name = locales.WeatherSnow()
	default:
		return ""
	}
	return fmt.Sprintf("%d° %s", int(math.Round(w.TempC)), name)
}

// weatherTick refreshes weather in the background every 30 minutes when running with a live window (skipped in tests).
func (a *app) weatherTick(now time.Time) {
	if a.win == nil || a.wx.busy {
		return
	}
	if !a.wx.tried.IsZero() && now.Sub(a.wx.tried) < 3*time.Minute {
		return
	}
	if a.wx.info.HasWeather && a.now().Sub(a.wx.info.UpdatedAt) < weatherRefresh &&
		a.wx.info.SolarDay == startOfDay(a.now()).Format("2006-01-02") {
		return
	}
	a.refreshWeather()
}

func (a *app) refreshWeather() {
	if a.win == nil || a.wx.busy {
		return
	}
	a.wx.busy = true
	a.wx.tried = time.Now()
	cur := a.wx.info
	go func() {
		next, ok := fetchLiveWeather(cur)
		a.update(func() {
			a.wx.busy = false
			if ok {
				a.wx.info = next
				a.saveWeather()
			}
		})
	}()
}

// fetchLiveWeather fetches approximate IP coordinates (cached 6h) and Open-Meteo current weather + sunrise/sunset.
func fetchLiveWeather(cur weatherInfo) (weatherInfo, bool) {
	cl := &http.Client{Timeout: weatherHTTPWait}
	out := cur

	// 1. Refresh city-level coordinates if missing or stale.
	if (out.Lat == 0 && out.Lon == 0) || time.Since(out.LocAt) > locationRefresh {
		if lat, lon, city, ok := fetchIPCoords(cl); ok {
			out.Lat, out.Lon, out.City, out.LocAt = lat, lon, city, time.Now()
		}
	}
	if out.Lat == 0 && out.Lon == 0 {
		return out, false
	}

	// 2. Fetch live Open-Meteo current conditions + today's exact sunrise/sunset.
	u := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,weather_code,cloud_cover&daily=sunrise,sunset&timezone=auto&forecast_days=1",
		out.Lat, out.Lon,
	)
	resp, err := cl.Get(u)
	if err != nil {
		return out, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, false
	}
	var payload struct {
		Current struct {
			TempC    float64 `json:"temperature_2m"`
			Code     int     `json:"weather_code"`
			CloudPct int     `json:"cloud_cover"`
		} `json:"current"`
		Daily struct {
			Sunrise []string `json:"sunrise"`
			Sunset  []string `json:"sunset"`
		} `json:"daily"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return out, false
	}
	out.Kind = wmoToWeatherKind(payload.Current.Code, payload.Current.CloudPct)
	out.TempC = payload.Current.TempC
	out.CloudPct = payload.Current.CloudPct
	out.UpdatedAt = time.Now()
	out.HasWeather = true
	if len(payload.Daily.Sunrise) > 0 {
		if hr, day, ok := parseLocalISOHour(payload.Daily.Sunrise[0]); ok {
			out.SunriseHr = hr
			out.SolarDay = day
		}
	}
	if len(payload.Daily.Sunset) > 0 {
		if hr, _, ok := parseLocalISOHour(payload.Daily.Sunset[0]); ok {
			out.SunsetHr = hr
		}
	}
	return out, true
}

func fetchIPCoords(cl *http.Client) (lat, lon float64, city string, ok bool) {
	// Primary: get.geojs.io
	if resp, err := cl.Get("https://get.geojs.io/v1/ip/geo.json"); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var g struct {
				Lat  string `json:"latitude"`
				Lon  string `json:"longitude"`
				City string `json:"city"`
			}
			if json.NewDecoder(resp.Body).Decode(&g) == nil {
				la, e1 := strconv.ParseFloat(g.Lat, 64)
				lo, e2 := strconv.ParseFloat(g.Lon, 64)
				if e1 == nil && e2 == nil && (la != 0 || lo != 0) {
					return la, lo, g.City, true
				}
			}
		}
	}
	// Fallback: ipwho.is
	if resp, err := cl.Get("https://ipwho.is/"); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var g struct {
				Success bool    `json:"success"`
				Lat     float64 `json:"latitude"`
				Lon     float64 `json:"longitude"`
				City    string  `json:"city"`
			}
			if json.NewDecoder(resp.Body).Decode(&g) == nil && g.Success && (g.Lat != 0 || g.Lon != 0) {
				return g.Lat, g.Lon, g.City, true
			}
		}
	}
	return 0, 0, "", false
}

// parseLocalISOHour parses Open-Meteo's "2026-11-28T07:54" into fractional local hour (7.9) and date string.
func parseLocalISOHour(s string) (hr float32, day string, ok bool) {
	datePart, timePart, found := strings.Cut(s, "T")
	if !found {
		return 0, "", false
	}
	hm := strings.Split(timePart, ":")
	if len(hm) < 2 {
		return 0, "", false
	}
	h, e1 := strconv.Atoi(hm[0])
	m, e2 := strconv.Atoi(hm[1])
	if e1 != nil || e2 != nil {
		return 0, "", false
	}
	return float32(h) + float32(m)/60.0, datePart, true
}

// wmoToWeatherKind maps WMO weather interpretation codes to poetic paper weather states.
func wmoToWeatherKind(code, cloudPct int) weatherKind {
	switch {
	case code >= 95:
		return weatherStorm
	case (code >= 71 && code <= 77) || code == 85 || code == 86:
		return weatherSnow
	case (code >= 51 && code <= 67) || (code >= 80 && code <= 82):
		return weatherRain
	case code == 45 || code == 48:
		return weatherFog
	case code == 3 || cloudPct >= 80:
		return weatherCloudy
	case code == 2 || cloudPct >= 40:
		return weatherPartly
	default:
		return weatherClear
	}
}
