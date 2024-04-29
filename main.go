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
)

var (
	roomName = envRequired("PEEPHOLE_ROOM_NAME")
	httpAddr = envOrDefault("PEEPHOLE_HTTP_ADDR", ":9339")

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

type room struct {
	RoomName     string `json:"room_name"`
	Participants int    `json:"participants"`
	CreatedTime  int64  `json:"created_time,omitempty"`
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

func peephole(w http.ResponseWriter) error {
	// Fetch "room census" from internal API
	resp, err := http.Get(roomCensusURL)
	if err != nil {
		return fmt.Errorf("failed to fetch room census from %q: %w", roomCensusURL, err)
	}
	defer resp.Body.Close()

	// Parse JSON response payload into roomList
	var payload struct {
		RoomCensus roomList `json:"room_census,omitempty"`
	}
	err = json.NewDecoder(resp.Body).Decode(&payload)
	if err != nil {
		return fmt.Errorf("failed to parse room census payload: %w", err)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(found)
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
