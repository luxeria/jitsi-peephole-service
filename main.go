package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

var (
	roomName    = envRequired("PEEPHOLE_ROOM_NAME")
	httpAddr    = envOrDefault("PEEPHOLE_HTTP_ADDR", ":9339")
	cacheExpiry = envDuration("PEEPHOLE_CACHE_EXPIRY", 5*time.Second)

	prosodyHTTPHost = envOrDefault("XMPP_SERVER", "xmpp.meet.jitsi")
	prosodyHTTPPort = envOrDefault("PROSODY_HTTP_PORT", "5280")

	roomCensusURL = (&url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(prosodyHTTPHost, prosodyHTTPPort),
		Path:   "/room-census",
	}).String()
)

func envRequired(name string) string {
	val := os.Getenv(name)
	if val == "" {
		slog.Error("missing environment variable", slog.String("name", name))
		os.Exit(1)
	}
	return val
}

func envOrDefault(name, fallback string) string {
	val := os.Getenv(name)
	if val == "" {
		return fallback
	}
	return val
}

func envDuration(name string, fallback time.Duration) time.Duration {
	val := os.Getenv(name)
	if val == "" {
		return fallback
	}

	t, err := time.ParseDuration(val)
	if err != nil {
		slog.Error("invalid environment variable", slog.String("name", name), slog.Any("error", err))
		os.Exit(1)
	}

	return t
}

type room struct {
	RoomName     string `json:"room_name"`
	Participants int    `json:"participants"`
	CreatedTime  int64  `json:"created_time,string,omitempty"`
}

type roomList []room

func (l *roomList) UnmarshalJSON(data []byte) error {
	// Attempt to unmarshal data as a array of rooms
	err := json.Unmarshal(data, (*[]room)(l))
	if err != nil {
		// Check if the empty list is represented as `{}`
		if emptyErr := json.Unmarshal(data, &struct{}{}); emptyErr == nil {
			return nil
		}
	}
	return err
}

var cache struct {
	sync.Mutex
	lastUpdated time.Time
	value       room
}

func fetchRoom() (*room, error) {
	cache.Lock()
	defer cache.Unlock()

	// Return cached value if not yet expired
	if time.Since(cache.lastUpdated) < cacheExpiry {
		return &cache.value, nil
	}

	// Fetch "room census" from internal API
	resp, err := http.Get(roomCensusURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch room census from %q: %w", roomCensusURL, err)
	}
	defer resp.Body.Close()

	// Parse JSON response payload into roomList
	var payload struct {
		RoomCensus roomList `json:"room_census,omitempty"`
	}
	err = json.NewDecoder(resp.Body).Decode(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse room census payload: %w", err)
	}

	// Extract configured room from room list. If no census for the configured
	// room is found, we fall back on just displaying zero participants
	var found = room{
		RoomName:     roomName,
		Participants: 0,
	}
	for _, r := range payload.RoomCensus {
		if r.RoomName == roomName {
			found = r
			break
		}
	}

	cache.lastUpdated = time.Now()
	cache.value = found

	return &found, nil
}

func peephole(w http.ResponseWriter) error {
	room, err := fetchRoom()
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(room)
}

func main() {
	slog.Info("starting HTTP server", "addr", httpAddr)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if err := peephole(w); err != nil {
			slog.Error("failed to serve request", slog.Any("error", err))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	})
	err := http.ListenAndServe(httpAddr, handler)
	if !errors.Is(err, http.ErrServerClosed) {
		slog.Error("listener failed", slog.Any("error", err))
		os.Exit(1)
	}
}
