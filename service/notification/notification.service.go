package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/api/option"
)

// Notification common notification model
type Notification struct {
	ID     primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Title  string             `json:"title,omitempty" bson:"title,omitempty"`
	Body   string             `json:"body,omitempty" bson:"body,omitempty"`
	Date   primitive.DateTime `json:"date,omitempty" bson:"date,omitempty"`
	Data   interface{}        `json:"data" bson:"data"`
	Opened bool               `json:"opened" bson:"opened"`
}

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

// GetNotifications fetch personal notifications from database
func GetNotifications(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json; charset=utf-8")

		authID, _, ok := request.BasicAuth()

		if ok {
			oID, _ := primitive.ObjectIDFromHex(authID)
			collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			defer cancel()

			match := bson.D{
				primitive.E{
					Key: "$match",
					Value: bson.D{
						primitive.E{
							Key:   "_id",
							Value: oID,
						},
					},
				},
			}

			project := bson.D{
				primitive.E{
					Key: "$project",
					Value: bson.D{
						primitive.E{Key: "notifications", Value: "$notifications"},
					},
				},
			}

			unwind := bson.D{
				primitive.E{
					Key:   "$unwind",
					Value: "$notifications",
				},
			}

			sort := bson.D{
				primitive.E{
					Key: "$sort",
					Value: bson.D{primitive.E{
						Key:   "notifications.date",
						Value: -1,
					},
					},
				},
			}

			group := bson.D{
				primitive.E{
					Key: "$group",
					Value: bson.D{
						primitive.E{Key: "_id", Value: "$_id"},
						primitive.E{
							Key: "notifications",
							Value: bson.D{
								primitive.E{Key: "$push", Value: "$notifications"},
							},
						},
					},
				},
			}

			opts := options.Aggregate().SetMaxTime(2 * time.Second)

			cursor, err := collection.Aggregate(ctx, mongo.Pipeline{match, project, unwind, sort, group}, opts)
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			results := []bson.M{}
			if err = cursor.All(context.TODO(), &results); err != nil {
				log.Fatal(err)
			}

			if len(results) == 0 {
				response.WriteHeader(http.StatusNotFound)
				response.Write([]byte(`{ "message": "Not Found" }`))
				return
			}

			json.NewEncoder(response).Encode(results[0]["notifications"])
		}
	}
}

// SendToUsername sends a notification to given user
func SendToUsername(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json; charset=utf-8")

		params := mux.Vars(request)
		ctx := context.Background()
		client, err := app.Messaging(ctx)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
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
		response.Header().Add("content-type", "application/json; charset=utf-8")

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

	collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
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

	save := Notification{
		Title:  notification.Title,
		Body:   notification.Body,
		Date:   primitive.NewDateTimeFromTime(time.Now()),
		Data:   map[string]string{},
		Opened: false,
	}

	filter := bson.D{primitive.E{Key: "_id", Value: id}}
	update := bson.D{
		primitive.E{
			Key:   "$push",
			Value: bson.D{primitive.E{Key: "notifications", Value: save}},
		},
	}
	collection.FindOneAndUpdate(ctx, filter, update)

	return nil
}
