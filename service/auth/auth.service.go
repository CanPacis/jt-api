package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"

	"jt-api/service/users"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

// LoginUser login user model
type LoginUser struct {
	ID        primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Fullname  string             `json:"fullname,omitempty" bson:"fullname,omitempty"`
	Username  string             `json:"username,omitempty" bson:"username,omitempty"`
	Password  string             `json:"password,omitempty" bson:"password,omitempty"`
	Email     string             `json:"email,omitempty" bson:"email,omitempty"`
	Image     string             `json:"image,omitempty" bson:"image,omitempty"`
	Bio       string             `json:"bio" bson:"bio"`
	Verified  bool               `json:"verified" bson:"verified"`
	Type      int                `json:"type" bson:"type"`
	Followers int                `json:"followers" bson:"followers"`
	Follows   int                `json:"follows" bson:"follows"`
}

// LoginResponse response model for user login
type LoginResponse struct {
	Token string    `json:"token,omitempty" bson:"token,omitempty"`
	User  LoginUser `json:"user,omitempty" bson:"user,omitempty"`
}

// Login authenticates user
func Login(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")

		var user users.User
		json.NewDecoder(request.Body).Decode(&user)

		collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		defer cancel()

		match := bson.D{
			primitive.E{Key: "$match", Value: bson.D{primitive.E{Key: "username", Value: user.Username}}},
		}

		project := bson.D{
			primitive.E{
				Key: "$project",
				Value: bson.D{
					primitive.E{Key: "_id", Value: "$_id"},
					primitive.E{Key: "username", Value: "$username"},
					primitive.E{Key: "fullname", Value: "$fullname"},
					primitive.E{Key: "password", Value: "$password"},
					primitive.E{Key: "email", Value: "$email"},
					primitive.E{Key: "image", Value: "$image"},
					primitive.E{Key: "bio", Value: "$bio"},
					primitive.E{Key: "type", Value: "$type"},
					primitive.E{Key: "verified", Value: "$verified"},
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

		var results []LoginUser
		if err = cursor.All(context.TODO(), &results); err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		if len(results) > 0 {
			err = bcrypt.CompareHashAndPassword([]byte((results[0].Password)), []byte(user.Password))
			if err != nil {
				response.WriteHeader(http.StatusBadRequest)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			results[0].Password = ""

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"user": results[0],
			})
			tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			result := LoginResponse{User: results[0], Token: tokenString}

			json.NewEncoder(response).Encode(result)
		} else {
			response.WriteHeader(http.StatusNotFound)
			response.Write([]byte(`{ "message": "Not Found" }`))
			return
		}
	}
}
