package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/api/option"
)

var app *firebase.App

// InitFirebase initializes the firebase admin
func InitFirebase() {
	opt := option.WithCredentialsFile("./firebase-service-account-key.json")
	var err error
	app, err = firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		fmt.Println("Failed to initialize firebase")
		log.Fatal(err)
	}
	fmt.Println("Initialized firebase admin")
}

// SendToUsername sends a notification to given user
func SendToUsername(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")

		params := mux.Vars(request)
		ctx := context.Background()
		client, err := app.Messaging(ctx)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		collection := db.Database("justhink-dev").Collection("users")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		defer cancel()

		var user bson.M
		err = collection.FindOne(ctx, bson.D{
			primitive.E{Key: "username", Value: params["username"]},
		}).Decode(&user)

		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		var notification messaging.Notification
		err = json.NewDecoder(request.Body).Decode(&notification)

		if err != nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		if notification.Title == "" || notification.Body == "" {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "Missing Parameters" }`))
			return
		}

		message := &messaging.Message{
			Notification: &notification,
			Data: map[string]string{
				"click_action": "FLUTTER_NOTIFICATION_CLICK",
				"sound":        "default",
			},
			Android: &messaging.AndroidConfig{
				Priority: "high",
			},
			Token: user["FCMToken"].(string),
		}

		result, err := client.Send(ctx, message)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		response.Write([]byte(`{ "message": "` + result + `" }`))
	}
}

// SendToID sends a notification to given user
func SendToID(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")

		params := mux.Vars(request)
		fmt.Println(app, params["id"])

		json.NewEncoder(response).Encode([]int{})
	}
}

// SendNotification a service for sending notifications by server itself
func SendNotification(id primitive.ObjectID, notification messaging.Notification, db *mongo.Client) error {
	ctx := context.Background()
	client, err := app.Messaging(ctx)
	if err != nil {
		return err
	}

	collection := db.Database("justhink-dev").Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	var user bson.M
	err = collection.FindOne(ctx, bson.D{
		primitive.E{Key: "_id", Value: id},
	}).Decode(&user)

	if err != nil {
		return err
	}

	message := &messaging.Message{
		Notification: &notification,
		Data: map[string]string{
			"click_action": "FLUTTER_NOTIFICATION_CLICK",
			"sound":        "default",
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
		Token: user["FCMToken"].(string),
	}

	_, err = client.Send(ctx, message)
	if err != nil {
		return err
	}

	return nil
}
