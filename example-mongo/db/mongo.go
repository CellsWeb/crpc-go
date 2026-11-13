package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

var (
	Client   *mongo.Client
	Database *mongo.Database
)

// Connect initializes the connection to MongoDB
func Connect() error {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	dbName := os.Getenv("MONGODB_DB")
	if dbName == "" {
		dbName = "crpc_mongo_test"
	}

	clientOpts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(200).
		SetMinPoolSize(20).
		SetMaxConnIdleTime(60 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(clientOpts)
	if err != nil {
		return fmt.Errorf("failed creating mongo client: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return fmt.Errorf("failed pinging mongo database: %w", err)
	}

	Client = client
	Database = client.Database(dbName)
	return nil
}

// Users returns the users collection
func Users() *mongo.Collection {
	return Database.Collection("users")
}

// Disconnect terminates the connection
func Disconnect() error {
	if Client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return Client.Disconnect(ctx)
	}
	return nil
}
