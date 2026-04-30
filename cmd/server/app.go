package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	pb "github.com/azzykesuma/warframeMarket/api/proto"
	"github.com/joho/godotenv"
)

const defaultBaseURL = "https://api.warframe.market/v2/"

// watcherServer is the application service. It coordinates auth, WFM access, gRPC, and REST adapters.
type watcherServer struct {
	pb.UnimplementedWarframeMarketWatcherServer
	baseURL       string
	jwtToken      string
	language      string
	appUsername   string
	appPassword   string
	sessions      map[string]struct{}
	sessionMu     sync.RWMutex
	inventory     InventoryRepository
	httpClient    *http.Client
	rateMu        sync.Mutex
	lastRequest   time.Time
	minRequestGap time.Duration
}

func newWatcherServer() *watcherServer {
	_ = godotenv.Load()
	return newWatcherServerFromConfigWithAuth(
		os.Getenv("BASE_URL"),
		os.Getenv("JWT_TOKEN"),
		os.Getenv("LANGUAGE"),
		os.Getenv("APP_USERNAME"),
		os.Getenv("APP_PASSWORD"),
		&http.Client{Timeout: 10 * time.Second},
	)
}

func newWatcherServerFromConfig(baseURL, jwtToken, language string, client *http.Client) *watcherServer {
	return newWatcherServerFromConfigWithAuth(baseURL, jwtToken, language, os.Getenv("APP_USERNAME"), os.Getenv("APP_PASSWORD"), client)
}

func newWatcherServerFromConfigWithAuth(baseURL, jwtToken, language, appUsername, appPassword string, client *http.Client) *watcherServer {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	if language == "" {
		language = "en"
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if jwtToken == "" {
		log.Println("WARNING: JWT_TOKEN is not set; authenticated endpoints will fail")
	}
	if appUsername == "" || appPassword == "" {
		log.Println("WARNING: APP_USERNAME or APP_PASSWORD is not set; app login will fail")
	}
	inventory, err := newInventoryRepository(context.Background())
	if err != nil {
		log.Printf("WARNING: Firebase inventory repository is not available: %v", err)
		inventory = unavailableInventoryRepository{err: err}
	}

	return &watcherServer{
		baseURL:       baseURL,
		jwtToken:      jwtToken,
		language:      language,
		appUsername:   appUsername,
		appPassword:   appPassword,
		sessions:      make(map[string]struct{}),
		inventory:     inventory,
		httpClient:    client,
		minRequestGap: 350 * time.Millisecond,
	}
}
