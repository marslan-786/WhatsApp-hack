package main

import (
	"context"
	"fmt"
	"time"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var mongoClient *mongo.Client

func connectDatabases() {
	// MongoDB Connection
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var err error
	mongoClient, err = mongo.Connect(ctx, options.Client().ApplyURI(Config.MongoURI))
	if err != nil {
		fmt.Println("❌ MongoDB Failed:", err)
	} else {
		fmt.Println("✅ MongoDB Connected")
	}
}