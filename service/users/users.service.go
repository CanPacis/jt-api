package users

import (
	"context"
	"encoding/json"
	"jt-api/config"
	"jt-api/service/notification"
	"log"
	"net/http"
	"os"
	"time"

	"firebase.google.com/go/messaging"
	"golang.org/x/crypto/bcrypt"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// User is Common user model for database
type User struct {
	ID            primitive.ObjectID    `json:"_id,omitempty" bson:"_id,omitempty"`
	Fullname      string                `json:"fullname,omitempty" bson:"fullname,omitempty"`
	Username      string                `json:"username,omitempty" bson:"username,omitempty"`
	Email         string                `json:"email,omitempty" bson:"email,omitempty"`
	Password      string                `json:"password,omitempty" bson:"password,omitempty"`
	Image         string                `json:"image,omitempty" bson:"image,omitempty"`
	Bio           string                `json:"bio" bson:"bio"`
	Language      string                `json:"language,omitempty" bson:"language,omitempty"`
	Verified      bool                  `json:"verified" bson:"verified"`
	FCMToken      string                `json:"fcmtoken,omitempty" bson:"fcmtoken,omitempty"`
	Rank          int                   `json:"rank" bson:"rank"`
	Type          int                   `json:"type" bson:"type"`
	Followers     *[]primitive.ObjectID `json:"followers,omitempty" bson:"followers,omitempty"`
	Follows       *[]primitive.ObjectID `json:"follows,omitempty" bson:"follows,omitempty"`
	Communities   *[]primitive.ObjectID `json:"communities,omitempty" bson:"communities,omitempty"`
	Notifications *[]interface{}        `json:"notifications,omitempty" bson:"notifications,omitempty"`
}

// UserUpdate is model for user edits
type UserUpdate struct {
	Fullname string `json:"fullname,omitempty" bson:"fullname,omitempty"`
	Username string `json:"username,omitempty" bson:"username,omitempty"`
	Email    string `json:"email,omitempty" bson:"email,omitempty"`
	Password string `json:"password,omitempty" bson:"password,omitempty"`
	Image    string `json:"image,omitempty" bson:"image,omitempty"`
	Bio      string `json:"bio,omitempty" bson:"bio,omitempty"`
}

// TokenUpdate is model for updating fcm token
type TokenUpdate struct {
	Token string `json:"token,omitempty" bson:"token,omitempty"`
}

// UserActionModel is model for following and unfollowing actions
type UserActionModel struct {
	ID primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
}

// GetUser fetch single user from database
func GetUser(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json; charset=utf-8")
		params := mux.Vars(request)
		id, err := primitive.ObjectIDFromHex(params["id"])
		if err != nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		authID, _, ok := request.BasicAuth()

		if ok {
			collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			defer cancel()

			match := bson.D{
				primitive.E{Key: "$match", Value: bson.D{primitive.E{Key: "_id", Value: id}}},
			}

			oID, _ := primitive.ObjectIDFromHex(authID)

			project := bson.D{
				primitive.E{
					Key: "$project",
					Value: bson.D{
						primitive.E{Key: "_id", Value: "$_id"},
						primitive.E{Key: "username", Value: "$username"},
						primitive.E{Key: "fullname", Value: "$fullname"},
						primitive.E{Key: "email", Value: "$email"},
						primitive.E{Key: "image", Value: "$image"},
						primitive.E{Key: "bio", Value: "$bio"},
						primitive.E{Key: "verified", Value: "$verified"},
						primitive.E{Key: "followed", Value: bson.D{
							primitive.E{Key: "$in", Value: []interface{}{oID, "$followers"}},
						}},
						primitive.E{Key: "followers", Value: bson.D{
							primitive.E{Key: "$size", Value: "$followers"},
						}},
						primitive.E{Key: "follows", Value: bson.D{
							primitive.E{Key: "$size", Value: "$follows"},
						}},
					},
				},
			}
			opts := options.Aggregate().SetMaxTime(2 * time.Second)

			cursor, err := collection.Aggregate(ctx, mongo.Pipeline{match, project}, opts)
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			var results []bson.M
			if err = cursor.All(context.TODO(), &results); err != nil {
				log.Fatal(err)
			}

			if len(results) > 0 {
				json.NewEncoder(response).Encode(results[0])
			} else {
				response.WriteHeader(http.StatusNotFound)
				response.Write([]byte(`{ "message": "Not Found" }`))
				return
			}
		}
	}
}

// CreateUser create user and register to database
func CreateUser(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json; charset=utf-8")

		collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		defer cancel()

		var user User
		json.NewDecoder(request.Body).Decode(&user)

		user.Image = "https://justhink.s3.eu-central-1.amazonaws.com/default-user.png"
		user.Verified = false
		user.Bio = ""
		user.Rank = 0
		user.Type = 0
		user.Followers = &[]primitive.ObjectID{}
		user.Follows = &[]primitive.ObjectID{}
		user.Communities = &[]primitive.ObjectID{}
		user.Notifications = &[]interface{}{}
		user.Language = "tr"

		hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), 5)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		user.Password = string(hash)

		result, err := collection.InsertOne(ctx, user)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		json.NewEncoder(response).Encode(result)
	}
}

// EditUser edit user and register to database
func EditUser(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json; charset=utf-8")

		collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		defer cancel()

		authID, _, ok := request.BasicAuth()
		if ok {
			var updateObject UserUpdate
			err := json.NewDecoder(request.Body).Decode(&updateObject)
			id, _ := primitive.ObjectIDFromHex(authID)

			if err != nil {
				response.WriteHeader(http.StatusBadRequest)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			updateValue := bson.D{}

			if updateObject.Fullname != "" {
				updateValue = append(updateValue, primitive.E{Key: "fullname", Value: updateObject.Fullname})
			}
			if updateObject.Username != "" {
				updateValue = append(updateValue, primitive.E{Key: "username", Value: updateObject.Username})
			}
			if updateObject.Email != "" {
				updateValue = append(updateValue, primitive.E{Key: "email", Value: updateObject.Email})
			}
			if updateObject.Image != "" {
				updateValue = append(updateValue, primitive.E{Key: "image", Value: updateObject.Image})
			}
			if updateObject.Bio != "" {
				updateValue = append(updateValue, primitive.E{Key: "bio", Value: updateObject.Bio})
			}
			if updateObject.Password != "" {
				hash, err := bcrypt.GenerateFromPassword([]byte(updateObject.Password), 5)
				if err != nil {
					response.WriteHeader(http.StatusInternalServerError)
					response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
					return
				}
				updateValue = append(updateValue, primitive.E{Key: "password", Value: string(hash)})
			}

			opts := options.FindOneAndUpdate().SetUpsert(true)
			filter := bson.D{primitive.E{Key: "_id", Value: id}}
			update := bson.D{primitive.E{Key: "$set", Value: updateValue}}
			var updatedDocument bson.M
			err = collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedDocument)
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			response.Write([]byte(`{ "message": "OK" }`))
		}
	}
}

// UpdateFCMToken updates firebase cloud messaging token at each login
func UpdateFCMToken(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json; charset=utf-8")

		collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		defer cancel()

		authID, _, ok := request.BasicAuth()
		if ok {
			id, _ := primitive.ObjectIDFromHex(authID)

			var updateObject TokenUpdate
			err := json.NewDecoder(request.Body).Decode(&updateObject)
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			if updateObject.Token == "" {
				response.WriteHeader(http.StatusBadRequest)
				response.Write([]byte(`{ "message": "Token not provided" }`))
				return
			}

			opts := options.FindOneAndUpdate().SetUpsert(true)
			filter := bson.D{primitive.E{Key: "_id", Value: id}}
			update := bson.D{primitive.E{
				Key: "$set", Value: bson.D{primitive.E{Key: "FCMToken", Value: updateObject.Token}},
			}}
			var updatedDocument bson.M
			err = collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedDocument)
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			response.Write([]byte(`{ "message": "OK" }`))
		}
	}
}

// UserExists find if user exists based on username or email
func UserExists(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json; charset=utf-8")
		params := mux.Vars(request)

		collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		defer cancel()

		var match bson.D

		if params["type"] == "username" {
			match = bson.D{
				primitive.E{Key: "$match", Value: bson.D{primitive.E{Key: "username", Value: params["query"]}}},
			}
		} else if params["type"] == "email" {
			match = bson.D{
				primitive.E{Key: "$match", Value: bson.D{primitive.E{Key: "email", Value: params["query"]}}},
			}
		} else {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "Unknown Parameter Type" }`))
			return
		}

		project := bson.D{
			primitive.E{
				Key: "$project",
				Value: bson.D{
					primitive.E{Key: "_id", Value: "$_id"},
				},
			},
		}
		opts := options.Aggregate().SetMaxTime(2 * time.Second)

		cursor, err := collection.Aggregate(ctx, mongo.Pipeline{match, project}, opts)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		var results []bson.M
		if err = cursor.All(context.TODO(), &results); err != nil {
			log.Fatal(err)
		}

		if len(results) > 0 {
			response.Write([]byte(`{ "found": true }`))
		} else {
			response.Write([]byte(`{ "found": false }`))
		}
	}
}

// UserAction is for following and unfollowing users
func UserAction(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json; charset=utf-8")
		actionType := mux.Vars(request)["type"]

		authID, _, ok := request.BasicAuth()

		if ok {
			oID, _ := primitive.ObjectIDFromHex(authID)
			var action UserActionModel
			err := json.NewDecoder(request.Body).Decode(&action)

			if err == nil {
				if actionType == "follow" {
					follow(db, response, request, oID, action.ID)
					return
				} else if actionType == "unfollow" {
					unfollow(db, response, request, oID, action.ID)
					return
				}
			} else {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

		}

		response.WriteHeader(http.StatusNotFound)
		response.Write([]byte(`{ "message": "Not Found" }`))
		return
	}
}

func follow(
	db *mongo.Client,
	response http.ResponseWriter,
	request *http.Request,
	followerID primitive.ObjectID,
	followeeID primitive.ObjectID,
) error {
	if followerID != followeeID {
		collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		defer cancel()

		results := followed(db, response, request, followerID, followeeID)

		if len(results) > 0 {
			if results[0]["followed"].(bool) {
				response.WriteHeader(http.StatusBadRequest)
				response.Write([]byte(`{ "message": "User already followed" }`))
				return nil
			}
			updateOpts := options.FindOneAndUpdate().SetUpsert(true)

			followeeFilter := bson.D{primitive.E{Key: "_id", Value: followeeID}}
			followeeUpdate := bson.D{primitive.E{
				Key: "$push", Value: bson.D{primitive.E{Key: "followers", Value: followerID}},
			}}
			collection.FindOneAndUpdate(ctx, followeeFilter, followeeUpdate, updateOpts)

			followerFilter := bson.D{primitive.E{Key: "_id", Value: followerID}}
			followerUpdate := bson.D{primitive.E{
				Key: "$push", Value: bson.D{primitive.E{Key: "follows", Value: followeeID}},
			}}
			collection.FindOneAndUpdate(ctx, followerFilter, followerUpdate, updateOpts)

			// Send notification
			var follower, followee bson.M

			opts := options.FindOne().SetProjection(bson.D{
				primitive.E{Key: "fullname", Value: "$fullname"},
				primitive.E{Key: "username", Value: "$username"},
				primitive.E{Key: "language", Value: "$language"},
			})
			collection.FindOne(ctx, bson.D{primitive.E{Key: "_id", Value: followerID}}, opts).Decode(&follower)
			collection.FindOne(ctx, bson.D{primitive.E{Key: "_id", Value: followeeID}}, opts).Decode(&followee)

			notification.SendNotification(followeeID, messaging.Notification{
				Title: config.Languages[followee["language"].(string)].NewFollow(),
				Body:  config.Languages[followee["language"].(string)].FollowStart(follower["fullname"].(string) + " (@" + follower["username"].(string) + ")"),
			}, db)

			response.Write([]byte(`{ "message": "OK" }`))
			return nil
		}

		response.WriteHeader(http.StatusNotFound)
		response.Write([]byte(`{ "message": "User Not Found" }`))
		return nil
	}
	return nil
}

func unfollow(
	db *mongo.Client,
	response http.ResponseWriter,
	request *http.Request,
	followerID primitive.ObjectID,
	followeeID primitive.ObjectID,
) error {
	collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	results := followed(db, response, request, followerID, followeeID)

	if len(results) > 0 {
		if results[0]["followed"].(bool) == false {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "User already unfollowed" }`))
			return nil
		}
		updateOpts := options.FindOneAndUpdate().SetUpsert(true)

		followeeFilter := bson.D{primitive.E{Key: "_id", Value: followeeID}}
		followeeUpdate := bson.D{primitive.E{
			Key: "$pull", Value: bson.D{primitive.E{Key: "followers", Value: followerID}},
		}}
		collection.FindOneAndUpdate(ctx, followeeFilter, followeeUpdate, updateOpts)

		followerFilter := bson.D{primitive.E{Key: "_id", Value: followerID}}
		followerUpdate := bson.D{primitive.E{
			Key: "$pull", Value: bson.D{primitive.E{Key: "follows", Value: followeeID}},
		}}
		collection.FindOneAndUpdate(ctx, followerFilter, followerUpdate, updateOpts)

		response.Write([]byte(`{ "message": "OK" }`))
		return nil
	}

	response.WriteHeader(http.StatusNotFound)
	response.Write([]byte(`{ "message": "User Not Found" }`))
	return nil
}

func followed(
	db *mongo.Client,
	response http.ResponseWriter,
	request *http.Request,
	followerID primitive.ObjectID,
	followeeID primitive.ObjectID,
) []bson.M {
	collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	aggregateOpts := options.Aggregate().SetMaxTime(2 * time.Second)
	match := bson.D{
		primitive.E{Key: "$match", Value: bson.D{primitive.E{Key: "_id", Value: followeeID}}},
	}
	project := bson.D{
		primitive.E{
			Key: "$project",
			Value: bson.D{
				primitive.E{Key: "followed", Value: bson.D{
					primitive.E{Key: "$in", Value: []interface{}{followerID, "$followers"}},
				}},
			},
		},
	}
	cursor, err := collection.Aggregate(ctx, mongo.Pipeline{match, project}, aggregateOpts)

	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
		return nil
	}

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		log.Fatal(err)
	}

	return results
}
