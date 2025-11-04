package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoClient holds the MongoDB client instance
type MongoClient struct {
	Client *mongo.Client
	DB     *mongo.Database
}

var clientInstance *MongoClient

// Connect connects to MongoDB and returns a MongoClient instance
func Connect(uri string, dbName string) (*MongoClient, error) {
	if clientInstance != nil {
		return clientInstance, nil
	}

	// Set client options
	clientOptions := options.Client().ApplyURI(uri)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %v", err)
	}

	// Check the connection
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %v", err)
	}

	clientInstance = &MongoClient{
		Client: client,
		DB:     client.Database(dbName),
	}

	log.Println("Connected to MongoDB!")
	return clientInstance, nil
}

// Disconnect closes the MongoDB connection
func (mc *MongoClient) Disconnect() error {
	if mc.Client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		err := mc.Client.Disconnect(ctx)
		if err != nil {
			return fmt.Errorf("failed to disconnect from MongoDB: %v", err)
		}
		
		clientInstance = nil
		log.Println("Disconnected from MongoDB!")
	}
	return nil
}