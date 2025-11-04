package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang/api"
	"golang/db"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func main() {
	// Get MongoDB connection string from environment variable or use default
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		// Default connection string - replace with your actual MongoDB connection string
		uri = "mongodb://localhost:27017" // For local MongoDB
		// For MongoDB Atlas, use: "mongodb+srv://username:password@cluster.mongodb.net"
	}

	// Database name
	dbName := os.Getenv("MONGODB_DATABASE")
	if dbName == "" {
		dbName = "test_database" // Default database name
	}

	// Connect to MongoDB
	mongoClient, err := db.Connect(uri, dbName)
	if err != nil {
		log.Fatal(err)
	}

	// Ensure connection is closed when main function exits
	defer func() {
		if err := mongoClient.Disconnect(); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	// Test the connection by pinging the database
	err = pingDatabase(mongoClient)
	if err != nil {
		log.Fatal(err)
	}

	// Create a sample collection and insert some data to make the database visible
	err = createSampleData(mongoClient)
	if err != nil {
		log.Printf("Error creating sample data: %v", err)
	}

	// Example: List collections in the database
	collections, err := listCollections(mongoClient)
	if err != nil {
		log.Printf("Error listing collections: %v", err)
	} else {
		fmt.Printf("Collections in database '%s': %v\n", dbName, collections)
	}

	fmt.Println("Successfully connected to MongoDB and created sample data!")

	// Start HTTP server for CRUD API
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	router := api.NewRouter(mongoClient)
	log.Printf("Starting API server on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("API server failed: %v", err)
	}
}

// pingDatabase tests the database connection
func pingDatabase(client *db.MongoClient) error {
	err := client.Client.Ping(context.TODO(), nil)
	if err != nil {
		return fmt.Errorf("failed to ping MongoDB: %v", err)
	}
	fmt.Println("Pinged your deployment. You successfully connected to MongoDB!")
	return nil
}

// listCollections lists all collections in the database
func listCollections(client *db.MongoClient) ([]string, error) {
	collections, err := client.DB.ListCollectionNames(context.TODO(), bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %v", err)
	}
	return collections, nil
}

// createSampleData creates a sample collection and inserts a document
func createSampleData(client *db.MongoClient) error {
	// Create a collection named "users" and insert a sample document
	collection := client.DB.Collection("users")

	// Sample document to insert
	sampleDoc := bson.M{
		"name":  "John Doe",
		"email": "john.doe@example.com",
		"age":   30,
		// use a proper time.Time so the driver encodes it as BSON datetime
		"created_at": time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	// Insert the document
	result, err := collection.InsertOne(context.TODO(), sampleDoc)
	if err != nil {
		return fmt.Errorf("failed to insert document: %v", err)
	}

	fmt.Printf("Inserted document with ID: %v\n", result.InsertedID)

	// Also demonstrate how to find the document
	var foundDoc bson.M
	err = collection.FindOne(context.TODO(), bson.M{"_id": result.InsertedID}).Decode(&foundDoc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Println("No document found")
		} else {
			return fmt.Errorf("error finding document: %v", err)
		}
	} else {
		fmt.Printf("Found document: %+v\n", foundDoc)
	}

	return nil
}
