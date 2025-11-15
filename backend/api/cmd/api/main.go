package main

import (
	"context"
	"log"

	"github.com/sngm3741/makoto-club-services/api/internal/config"
	"github.com/sngm3741/makoto-club-services/api/internal/server"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	clientOptions := options.Client().ApplyURI(cfg.MongoURI).SetServerAPIOptions(options.ServerAPI(options.ServerAPIVersion1))
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		cfg.ServerLog.Fatalf("MongoDB 接続に失敗しました: %v", err)
	}

	app := server.New(cfg, client)
	if err := app.Run(); err != nil {
		log.Fatalf("サーバー起動に失敗: %v", err)
	}
}
