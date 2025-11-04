package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang/db"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// User represents a user stored in MongoDB
type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name      string             `bson:"name,omitempty" json:"name,omitempty"`
	Email     string             `bson:"email,omitempty" json:"email,omitempty"`
	Age       int                `bson:"age,omitempty" json:"age,omitempty"`
	CreatedAt time.Time          `bson:"created_at,omitempty" json:"created_at,omitempty"`
}

// NewRouter returns an http.Handler with user CRUD routes registered.
func NewRouter(mc *db.MongoClient) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			listUsers(mc, w, r)
		case http.MethodPost:
			createUser(mc, w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Routes with ID: /users/{id}
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getUser(mc, w, r)
		case http.MethodPut:
			updateUser(mc, w, r)
		case http.MethodDelete:
			deleteUser(mc, w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return mux
}

// Helper: write JSON
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// createUser - POST /users
func createUser(mc *db.MongoClient, w http.ResponseWriter, r *http.Request) {
	var in User
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now().UTC()
	}

	coll := mc.DB.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := coll.InsertOne(ctx, in)
	if err != nil {
		http.Error(w, fmt.Sprintf("insert error: %v", err), http.StatusInternalServerError)
		return
	}

	id := ""
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		id = oid.Hex()
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// listUsers - GET /users
func listUsers(mc *db.MongoClient, w http.ResponseWriter, r *http.Request) {
	coll := mc.DB.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cur, err := coll.Find(ctx, bson.M{})
	if err != nil {
		http.Error(w, fmt.Sprintf("find error: %v", err), http.StatusInternalServerError)
		return
	}
	defer cur.Close(ctx)

	var out []map[string]any
	for cur.Next(ctx) {
		var raw bson.M
		if err := cur.Decode(&raw); err != nil {
			http.Error(w, fmt.Sprintf("decode error: %v", err), http.StatusInternalServerError)
			return
		}

		m := map[string]any{}
		// id
		if idv, ok := raw["_id"].(primitive.ObjectID); ok {
			m["id"] = idv.Hex()
		} else {
			m["id"] = ""
		}
		// name, email, age
		if v, ok := raw["name"].(string); ok {
			m["name"] = v
		}
		if v, ok := raw["email"].(string); ok {
			m["email"] = v
		}
		if v, ok := raw["age"].(int32); ok {
			m["age"] = int(v)
		} else if v, ok := raw["age"].(int); ok {
			m["age"] = v
		} else if v, ok := raw["age"].(float64); ok {
			m["age"] = int(v)
		}

		// normalize created_at to RFC3339 string when possible
		if cat, exists := raw["created_at"]; exists {
			switch t := cat.(type) {
			case primitive.DateTime:
				m["created_at"] = t.Time().UTC().Format(time.RFC3339)
			case time.Time:
				m["created_at"] = t.UTC().Format(time.RFC3339)
			case map[string]any:
				// May come from older inserted doc like {"$date":"..."}
				if s, ok := t["$date"].(string); ok {
					if parsed, err := time.Parse(time.RFC3339, s); err == nil {
						m["created_at"] = parsed.UTC().Format(time.RFC3339)
					} else {
						m["created_at"] = s
					}
				} else {
					m["created_at"] = t
				}
			default:
				m["created_at"] = t
			}
		}

		out = append(out, m)
	}

	writeJSON(w, http.StatusOK, out)
}

// getUser - GET /users/{id}
func getUser(mc *db.MongoClient, w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/users/")
	oid, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	coll := mc.DB.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var raw bson.M
	err = coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&raw)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("find error: %v", err), http.StatusInternalServerError)
		return
	}

	resp := map[string]any{"id": ""}
	if idv, ok := raw["_id"].(primitive.ObjectID); ok {
		resp["id"] = idv.Hex()
	}
	if v, ok := raw["name"].(string); ok {
		resp["name"] = v
	}
	if v, ok := raw["email"].(string); ok {
		resp["email"] = v
	}
	if v, ok := raw["age"].(int32); ok {
		resp["age"] = int(v)
	} else if v, ok := raw["age"].(int); ok {
		resp["age"] = v
	} else if v, ok := raw["age"].(float64); ok {
		resp["age"] = int(v)
	}
	if cat, exists := raw["created_at"]; exists {
		switch t := cat.(type) {
		case primitive.DateTime:
			resp["created_at"] = t.Time().UTC().Format(time.RFC3339)
		case time.Time:
			resp["created_at"] = t.UTC().Format(time.RFC3339)
		case map[string]any:
			if s, ok := t["$date"].(string); ok {
				if parsed, err := time.Parse(time.RFC3339, s); err == nil {
					resp["created_at"] = parsed.UTC().Format(time.RFC3339)
				} else {
					resp["created_at"] = s
				}
			} else {
				resp["created_at"] = t
			}
		default:
			resp["created_at"] = t
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// updateUser - PUT /users/{id}
func updateUser(mc *db.MongoClient, w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/users/")
	oid, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	// Remove id if present
	delete(body, "id")

	coll := mc.DB.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = coll.UpdateByID(ctx, oid, bson.M{"$set": body})
	if err != nil {
		http.Error(w, fmt.Sprintf("update error: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": oid.Hex()})
}

// deleteUser - DELETE /users/{id}
func deleteUser(mc *db.MongoClient, w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/users/")
	oid, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	coll := mc.DB.Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = coll.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		http.Error(w, fmt.Sprintf("delete error: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": oid.Hex()})
}
