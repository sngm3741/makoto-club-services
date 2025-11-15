package config

import (
	"log"
	"os"
	"strings"
	"time"
)

// JWTConfig は JWT 認証で利用する発行者と秘密鍵の組を表す。
// 境界づけられたコンテキストごとに異なる IdP を扱うため、複数エントリを想定する。
type JWTConfig struct {
	Issuer string
	Secret []byte
}

// Config は API 全体で利用する実行時設定の集合。
// DDD では Infrastructure 層として、アプリケーション層へ環境依存を注入するための契約となる。
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
	HelpfulCookieSecret          []byte
	HelpfulCookieSecure          bool
	FailedNotificationCollection string
}

// Load は環境変数を読み込んで Config を構築する。
// 外部環境の差異を 1 箇所へ集約し、境界づけられたコンテキストが OS 依存を意識せずに済むようにする。
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
		HelpfulCookieSecret:          []byte(helperSecret),
		HelpfulCookieSecure:          helperCookieSecure,
		FailedNotificationCollection: failedNotifications,
	}

	cfg.ServerLog.Printf("loaded config: adminReviewBaseURL=%q messengerEndpoint=%q destination=%q", adminReviewBaseURL, messengerEndpoint, messengerDestination)

	return cfg
}

// envOrDefault は環境変数が未設定の場合にフォールバック値を返すヘルパー。
// Config.Load 内の重複したチェックを減らし、設定の意図を明確にする。
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parseList はカンマ区切りの環境変数を分割し、空要素を除外したスライスを返す。
// API_ALLOWED_ORIGINS のような複数値設定で再利用する。
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
