package config

import (
	"log"
	"os"
	"strings"
	"time"
)

// JWTConfig defines issuer/secret pair for auth verification.
type JWTConfig struct {
	Issuer string
	Secret []byte
}

// Config holds runtime configuration shared across the application.
type Config struct {
	Addr                         string
	MongoURI                     string
	MongoDatabase                string
	PingCollection               string
	StoreCollection              string
	ReviewCollection             string
	HelpfulVoteCollection        string
	Timeout                      time.Duration
	Timezone                     string
	ServerLog                    *log.Logger
	JWTConfigs                   []JWTConfig
	JWTAudience                  string
	MessengerEndpoint            string
	MessengerDestination         string
	DiscordDestination           string
	SlackDestination             string
	MessengerTimeout             time.Duration
	AdminReviewBaseURL           string
	AllowedOrigins               []string
	MediaBaseURL                 string
	HelpfulCookieSecret          []byte
	HelpfulCookieSecure          bool
	FailedNotificationCollection string
}

// Load reads environment variables and returns a fully populated Config.
func Load() Config {
	timeout := 10 * time.Second
	if v := os.Getenv("MONGO_CONNECT_TIMEOUT"); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			timeout = parsed
		}
	}

	messengerEndpoint := strings.TrimSpace(os.Getenv("MESSENGER_GATEWAY_URL"))
	if messengerEndpoint == "" {
		messengerEndpoint = "http://messenger-gateway:3000"
	}

	messengerDestination := strings.TrimSpace(os.Getenv("MESSENGER_GATEWAY_DESTINATION"))
	if messengerDestination == "" {
		messengerDestination = "line"
	}

	discordDestination := strings.TrimSpace(os.Getenv("MESSENGER_DISCORD_INCOMING_DESTINATION"))

	slackDestination := strings.TrimSpace(os.Getenv("MESSENGER_SLACK_DESTINATION"))

	messengerTimeout := 3 * time.Second
	if raw := strings.TrimSpace(os.Getenv("MESSENGER_GATEWAY_TIMEOUT")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			messengerTimeout = parsed
		}
	}

	allowedOrigins := parseList("API_ALLOWED_ORIGINS", []string{"*"})
	adminReviewBaseURL := strings.TrimSpace(os.Getenv("ADMIN_REVIEW_BASE_URL"))
	helperSecret := strings.TrimSpace(os.Getenv("HELPFUL_VOTER_SECRET"))
	if helperSecret == "" {
		log.Fatal("HELPFUL_VOTER_SECRET must be configured")
	}
	helperCookieSecure := strings.EqualFold(strings.TrimSpace(os.Getenv("HELPFUL_COOKIE_SECURE")), "true")

	var jwtConfigs []JWTConfig
	if secret := strings.TrimSpace(os.Getenv("AUTH_LINE_JWT_SECRET")); secret != "" {
		jwtConfigs = append(jwtConfigs, JWTConfig{
			Issuer: envOrDefault("AUTH_LINE_JWT_ISSUER", "makoto-club-auth"),
			Secret: []byte(secret),
		})
	}
	if secret := strings.TrimSpace(os.Getenv("AUTH_TWITTER_JWT_SECRET")); secret != "" {
		jwtConfigs = append(jwtConfigs, JWTConfig{
			Issuer: envOrDefault("AUTH_TWITTER_JWT_ISSUER", "auth-twitter"),
			Secret: []byte(secret),
		})
	}

	if len(jwtConfigs) == 0 {
		log.Fatal("JWT secrets not configured. Set AUTH_TWITTER_JWT_SECRET or AUTH_LINE_JWT_SECRET.")
	}

	jwtAudience := strings.TrimSpace(os.Getenv("AUTH_JWT_AUDIENCE"))
	if jwtAudience == "" {
		jwtAudience = strings.TrimSpace(os.Getenv("AUTH_LINE_JWT_AUDIENCE"))
	}
	if jwtAudience == "" {
		jwtAudience = strings.TrimSpace(os.Getenv("AUTH_TWITTER_JWT_AUDIENCE"))
	}

	storeCollection := envOrDefault("STORE_COLLECTION", "stores")
	reviewCollection := strings.TrimSpace(os.Getenv("REVIEW_COLLECTION"))
	if reviewCollection == "" {
		reviewCollection = envOrDefault("SURVEY_COLLECTION", "reviews")
	}

	failedNotifications := envOrDefault("FAILED_NOTIFICATION_COLLECTION", "failed_notifications")

	cfg := Config{
		Addr:                         envOrDefault("HTTP_ADDR", ":8080"),
		MongoURI:                     envOrDefault("MONGO_URI", "mongodb://mongo:27017"),
		MongoDatabase:                envOrDefault("MONGO_DB", "makoto-club"),
		StoreCollection:              storeCollection,
		ReviewCollection:             reviewCollection,
		HelpfulVoteCollection:        envOrDefault("HELPFUL_VOTE_COLLECTION", "survey_helpful_votes"),
		PingCollection:               envOrDefault("PING_COLLECTION", "pings"),
		Timeout:                      timeout,
		Timezone:                     envOrDefault("TIMEZONE", "Asia/Tokyo"),
		ServerLog:                    log.New(os.Stdout, "[makoto-club-api] ", log.LstdFlags|log.Lshortfile),
		JWTConfigs:                   jwtConfigs,
		JWTAudience:                  jwtAudience,
		MessengerEndpoint:            messengerEndpoint,
		MessengerDestination:         messengerDestination,
		DiscordDestination:           discordDestination,
		SlackDestination:             slackDestination,
		MessengerTimeout:             messengerTimeout,
		AdminReviewBaseURL:           adminReviewBaseURL,
		AllowedOrigins:               allowedOrigins,
		MediaBaseURL:                 strings.TrimSpace(os.Getenv("MEDIA_BASE_URL")),
		HelpfulCookieSecret:          []byte(helperSecret),
		HelpfulCookieSecure:          helperCookieSecure,
		FailedNotificationCollection: failedNotifications,
	}

	cfg.ServerLog.Printf("loaded config: adminReviewBaseURL=%q messengerEndpoint=%q destination=%q", adminReviewBaseURL, messengerEndpoint, messengerDestination)

	return cfg
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseList(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}

	if len(values) == 0 {
		return fallback
	}
	return values
}
