package communities

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Community common community model
type Community struct {
	ID      primitive.ObjectID    `json:"_id,omitempty" bson:"_id,omitempty"`
	Title   string                `json:"title" bson:"title"`
	Bio     string                `json:"bio" bson:"bio"`
	Date    primitive.DateTime    `json:"date,omitempty" bson:"date,omitempty"`
	Founder primitive.ObjectID    `json:"founder,omitempty" bson:"founder,omitempty"`
	Mods    *[]primitive.ObjectID `json:"mods" bson:"mods"`
	Members *[]primitive.ObjectID `json:"members" bson:"members"`
	Image   string                `json:"image" bson:"image"`
	Banner  string                `json:"banner" bson:"banner"`
}

// CommunityActionModel is model for joining and leaving actions
type CommunityActionModel struct {
	ID primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
}

// CreateCommunity creates a new community and registers it to database
func CreateCommunity(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json; charset=utf-8")

		collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("communities")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		defer cancel()

		authID, _, ok := request.BasicAuth()
		oID, _ := primitive.ObjectIDFromHex(authID)

		if ok {
			var community Community
			json.NewDecoder(request.Body).Decode(&community)

			if community.Title == "" || community.Bio == "" {
				response.WriteHeader(http.StatusBadRequest)
				response.Write([]byte(`{ "message": "Title and bio of the community must be provided" }`))
				return
			}

			community.Founder = oID
			community.Date = primitive.NewDateTimeFromTime(time.Now())
			community.Mods = &[]primitive.ObjectID{oID}
			community.Members = &[]primitive.ObjectID{oID}

			if community.Image == "" {
				community.Image = "https://justhink.s3.eu-central-1.amazonaws.com/default-community.png"
			}
			if community.Banner == "" {
				community.Banner = "https://justhink.s3.eu-central-1.amazonaws.com/default-community-banner.png"
			}

			result, err := collection.InsertOne(ctx, community)
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			json.NewEncoder(response).Encode(result)
		}
	}
}

// GetCommunity fetch given community from database
func GetCommunity(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json; charset=utf-8")
		params := mux.Vars(request)

		authID, _, ok := request.BasicAuth()
		id, err := primitive.ObjectIDFromHex(params["id"])
		if err != nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		if ok {
			oID, _ := primitive.ObjectIDFromHex(authID)
			var action CommunityActionModel
			err := json.NewDecoder(request.Body).Decode(&action)

			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("communities")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			defer cancel()

			match := bson.D{
				primitive.E{
					Key: "$match",
					Value: bson.D{
						primitive.E{Key: "_id", Value: id},
					},
				},
			}

			project := bson.D{
				primitive.E{
					Key: "$project",
					Value: bson.D{
						primitive.E{Key: "_id", Value: "$_id"},
						primitive.E{Key: "title", Value: "$title"},
						primitive.E{Key: "image", Value: "$image"},
						primitive.E{Key: "banner", Value: "$banner"},
						primitive.E{Key: "bio", Value: "$bio"},
						primitive.E{Key: "founder", Value: "$founder"},
						primitive.E{Key: "joined", Value: bson.D{
							primitive.E{Key: "$in", Value: []interface{}{oID, "$members"}},
						}},
						primitive.E{Key: "members", Value: bson.D{
							primitive.E{Key: "$size", Value: "$members"},
						}},
					},
				},
			}

			lookup := bson.D{
				primitive.E{
					Key: "$lookup",
					Value: bson.M{
						"from":         "users",
						"localField":   "founder",
						"foreignField": "_id",
						"as":           "founder",
					},
				},
			}

			opts := options.Aggregate().SetMaxTime(2 * time.Second)

			cursor, err := collection.Aggregate(ctx, mongo.Pipeline{
				match,
				project,
				lookup,
			}, opts)

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

			results[0]["founder"] = formatFounder(results[0]["founder"].(primitive.A)[0].(primitive.M))

			json.NewEncoder(response).Encode(results[0])
		}
	}
}

// CommunityAction is for joining and leaving communities
func CommunityAction(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json; charset=utf-8")
		actionType := mux.Vars(request)["type"]

		authID, _, ok := request.BasicAuth()

		if ok {
			oID, _ := primitive.ObjectIDFromHex(authID)
			var action CommunityActionModel
			err := json.NewDecoder(request.Body).Decode(&action)

			if err == nil {
				if actionType == "join" {
					join(db, response, request, oID, action.ID)
					return
				} else if actionType == "leave" {
					leave(db, response, request, oID, action.ID)
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

// GetUsersCommunities is for fetching communities of given user
func GetUsersCommunities(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json; charset=utf-8")
		params := mux.Vars(request)

		collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("communities")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		defer cancel()

		authID, _, ok := request.BasicAuth()
		oID, _ := primitive.ObjectIDFromHex(authID)
		ID, err := primitive.ObjectIDFromHex(params["id"])
		if err != nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		if ok {
			opts := options.Aggregate().SetMaxTime(2 * time.Second)

			match := bson.D{
				primitive.E{
					Key: "$match",
					Value: bson.D{
						primitive.E{
							Key: "members",
							Value: bson.D{primitive.E{
								Key:   "$in",
								Value: []interface{}{ID},
							}},
						},
					},
				},
			}

			project := bson.D{
				primitive.E{
					Key: "$project",
					Value: bson.D{
						primitive.E{Key: "_id", Value: "$_id"},
						primitive.E{Key: "title", Value: "$title"},
						primitive.E{Key: "image", Value: "$image"},
						primitive.E{Key: "members", Value: bson.D{
							primitive.E{Key: "$size", Value: "$members"},
						}},
						primitive.E{Key: "joined", Value: bson.D{
							primitive.E{Key: "$in", Value: []interface{}{oID, "$members"}},
						}},
					},
				},
			}

			sort := bson.D{
				primitive.E{
					Key: "$sort",
					Value: bson.D{primitive.E{
						Key:   "members",
						Value: -1,
					}},
				},
			}

			cursor, err := collection.Aggregate(ctx, mongo.Pipeline{match, project, sort}, opts)

			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			results := []bson.M{}
			if err = cursor.All(context.TODO(), &results); err != nil {
				log.Fatal(err)
			}

			json.NewEncoder(response).Encode(results)
		}
	}
}

func join(
	db *mongo.Client,
	response http.ResponseWriter,
	request *http.Request,
	joinerID primitive.ObjectID,
	communityID primitive.ObjectID,
) error {
	communitiesCollection := db.Database(os.Getenv("DATABASE_NAME")).Collection("communities")
	usersCollection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	results := joined(db, response, request, joinerID, communityID)

	if len(results) > 0 {
		if results[0]["joined"].(bool) {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "Community already joined" }`))
			return nil
		}
		updateOpts := options.FindOneAndUpdate().SetUpsert(true)

		filter := bson.D{primitive.E{Key: "_id", Value: communityID}}
		update := bson.D{primitive.E{
			Key: "$push", Value: bson.D{primitive.E{Key: "members", Value: joinerID}},
		}}
		communitiesCollection.FindOneAndUpdate(ctx, filter, update, updateOpts)

		filter = bson.D{primitive.E{Key: "_id", Value: joinerID}}
		update = bson.D{primitive.E{
			Key: "$push", Value: bson.D{primitive.E{Key: "communities", Value: communityID}},
		}}
		usersCollection.FindOneAndUpdate(ctx, filter, update, updateOpts)

		response.Write([]byte(`{ "message": "OK" }`))
		return nil
	}

	response.WriteHeader(http.StatusNotFound)
	response.Write([]byte(`{ "message": "Post Not Found" }`))
	return nil
}

func leave(
	db *mongo.Client,
	response http.ResponseWriter,
	request *http.Request,
	joinerID primitive.ObjectID,
	communityID primitive.ObjectID,
) error {
	communitiesCollection := db.Database(os.Getenv("DATABASE_NAME")).Collection("communities")
	usersCollection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	results := joined(db, response, request, joinerID, communityID)

	if len(results) > 0 {
		if results[0]["joined"].(bool) == false {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "Community already left" }`))
			return nil
		}
		updateOpts := options.FindOneAndUpdate().SetUpsert(true)

		filter := bson.D{primitive.E{Key: "_id", Value: communityID}}
		update := bson.D{primitive.E{
			Key: "$pull", Value: bson.D{primitive.E{Key: "members", Value: joinerID}},
		}}
		communitiesCollection.FindOneAndUpdate(ctx, filter, update, updateOpts)

		filter = bson.D{primitive.E{Key: "_id", Value: joinerID}}
		update = bson.D{primitive.E{
			Key: "$pull", Value: bson.D{primitive.E{Key: "communities", Value: communityID}},
		}}
		usersCollection.FindOneAndUpdate(ctx, filter, update, updateOpts)

		response.Write([]byte(`{ "message": "OK" }`))
		return nil
	}

	response.WriteHeader(http.StatusNotFound)
	response.Write([]byte(`{ "message": "Post Not Found" }`))
	return nil
}

func joined(
	db *mongo.Client,
	response http.ResponseWriter,
	request *http.Request,
	joinerID primitive.ObjectID,
	communityID primitive.ObjectID,
) []bson.M {
	collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	aggregateOpts := options.Aggregate().SetMaxTime(2 * time.Second)
	match := bson.D{
		primitive.E{Key: "$match", Value: bson.D{primitive.E{Key: "_id", Value: joinerID}}},
	}
	project := bson.D{
		primitive.E{
			Key: "$project",
			Value: bson.D{
				primitive.E{Key: "joined", Value: bson.D{
					primitive.E{Key: "$in", Value: []interface{}{communityID, "$communities"}},
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

func formatFounder(author primitive.M) primitive.M {
	result := primitive.M{}

	result["_id"] = author["_id"]
	result["fullname"] = author["fullname"]
	result["username"] = author["username"]
	result["image"] = author["image"]
	result["verified"] = author["verified"]

	return result
}
