package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type geocodeHit struct {
	Label string  `json:"label"`
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
}

// geocodeSearch proxies OpenStreetMap Nominatim so the SPA can search places
// without CORS issues and with a proper User-Agent.
func (s *Server) geocodeSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, []geocodeHit{})
		return
	}
	if len([]rune(q)) < 2 {
		writeJSON(w, http.StatusOK, []geocodeHit{})
		return
	}

	limit := 5
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 10 {
			limit = n
		}
	}

	params := url.Values{}
	params.Set("q", q)
	params.Set("format", "jsonv2")
	params.Set("limit", strconv.Itoa(limit))
	params.Set("addressdetails", "0")
	if viewbox := strings.TrimSpace(r.URL.Query().Get("viewbox")); viewbox != "" {
		params.Set("viewbox", viewbox)
		params.Set("bounded", "0")
	}

	req, err := http.NewRequestWithContext(
		r.Context(),
		http.MethodGet,
		"https://nominatim.openstreetmap.org/search?"+params.Encode(),
		nil,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build geocode request")
		return
	}
	req.Header.Set("User-Agent", "CampfireEventManager/1.0 (https://github.com/topi314/campfire-event-manager)")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", r.Header.Get("Accept-Language"))

	res, err := s.httpClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "geocode request failed")
		return
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to read geocode response")
		return
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("geocode upstream status %d", res.StatusCode))
		return
	}

	var raw []struct {
		DisplayName string `json:"display_name"`
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		writeError(w, http.StatusBadGateway, "invalid geocode response")
		return
	}

	out := make([]geocodeHit, 0, len(raw))
	for _, item := range raw {
		lat, errLat := strconv.ParseFloat(item.Lat, 64)
		lng, errLng := strconv.ParseFloat(item.Lon, 64)
		if errLat != nil || errLng != nil {
			continue
		}
		label := strings.TrimSpace(item.DisplayName)
		if label == "" {
			continue
		}
		out = append(out, geocodeHit{Label: label, Lat: lat, Lng: lng})
	}
	writeJSON(w, http.StatusOK, out)
}
