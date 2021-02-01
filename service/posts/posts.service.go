package posts

import (
	"context"
	"encoding/json"
	"jt-api/config"
	"jt-api/service/notification"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"firebase.google.com/go/messaging"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Post is Common post model for database
type Post struct {
	ID        primitive.ObjectID    `json:"_id,omitempty" bson:"_id,omitempty"`
	Title     string                `json:"title" bson:"title"`
	Content   *[]interface{}        `json:"content" bson:"content"`
	Date      primitive.DateTime    `json:"date,omitempty" bson:"date,omitempty"`
	Author    primitive.ObjectID    `json:"author,omitempty" bson:"author,omitempty"`
	Community primitive.ObjectID    `json:"community,omitempty" bson:"community,omitempty"`
	Images    *[]string             `json:"images" bson:"images"`
	Tags      *[]string             `json:"tags" bson:"tags"`
	Upvotes   *[]primitive.ObjectID `json:"upvotes" bson:"upvotes"`
	Answers   *[]primitive.ObjectID `json:"answers" bson:"answers"`
}

// PostActionModel is model for upvoting and downvoting actions
type PostActionModel struct {
	ID primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
}

// CreatePost creates post and registeres to the database
func CreatePost(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")

		authID, _, ok := request.BasicAuth()
		oID, _ := primitive.ObjectIDFromHex(authID)
		communityID, _ := primitive.ObjectIDFromHex("60049bc9888d8b3284e5cb4f")

		if ok {
			collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("posts")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			defer cancel()

			var post Post
			json.NewDecoder(request.Body).Decode(&post)

			if post.Tags == nil {
				post.Tags = &[]string{}
			}
			if post.Images == nil {
				post.Images = &[]string{}
			}

			post.Upvotes = &[]primitive.ObjectID{}
			post.Answers = &[]primitive.ObjectID{}
			post.Date = primitive.NewDateTimeFromTime(time.Now())
			post.Author = oID
			post.Community = communityID

			result, err := collection.InsertOne(ctx, post)
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			json.NewEncoder(response).Encode(result)
		}

	}
}

// GetPost fetch single post from database
func GetPost(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")

		authID, _, ok := request.BasicAuth()
		params := mux.Vars(request)

		id, err := primitive.ObjectIDFromHex(params["id"])
		if err != nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		if ok {
			collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("posts")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			defer cancel()

			oID, _ := primitive.ObjectIDFromHex(authID)

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
						primitive.E{Key: "community", Value: "$community"},
						primitive.E{Key: "images", Value: "$images"},
						primitive.E{Key: "tags", Value: "$tags"},
						primitive.E{Key: "title", Value: "$title"},
						primitive.E{Key: "content", Value: "$content"},
						primitive.E{Key: "date", Value: "$date"},
						primitive.E{Key: "author", Value: "$author"},
						primitive.E{Key: "upvoted", Value: bson.D{
							primitive.E{Key: "$in", Value: []interface{}{oID, "$upvotes"}},
						}},
						primitive.E{Key: "answers", Value: bson.D{
							primitive.E{Key: "$size", Value: "$answers"},
						}},
						primitive.E{Key: "upvotes", Value: bson.D{
							primitive.E{Key: "$size", Value: "$upvotes"},
						}},
					},
				},
			}

			lookupAuthor := bson.D{
				primitive.E{
					Key: "$lookup",
					Value: bson.M{
						"from": "users",
						"let": bson.D{
							primitive.E{Key: "author", Value: "$author"},
						},
						"pipeline": []interface{}{

							bson.D{
								primitive.E{Key: "$project", Value: bson.D{
									primitive.E{Key: "_id", Value: "$_id"},
									primitive.E{Key: "fullname", Value: "$fullname"},
									primitive.E{Key: "verified", Value: "$verified"},
									primitive.E{Key: "username", Value: "$username"},
									primitive.E{Key: "image", Value: "$image"},
								}},
							},
							bson.D{
								primitive.E{Key: "$match", Value: bson.D{
									primitive.E{
										Key:   "username",
										Value: "enes.aydin",
									},
								}},
							},
						},
						"as": "author",
					},
				},
			}

			lookupCommunity := bson.D{
				primitive.E{
					Key: "$lookup",
					Value: bson.M{
						"from": "communities",
						"let": bson.D{
							primitive.E{Key: "community", Value: "$community"},
						},
						"pipeline": []interface{}{
							bson.D{
								primitive.E{Key: "$project", Value: bson.D{
									primitive.E{Key: "_id", Value: "$_id"},
									primitive.E{Key: "title", Value: "$title"},
									primitive.E{Key: "image", Value: "$image"},
									primitive.E{Key: "members", Value: bson.D{
										primitive.E{Key: "$size", Value: "$members"},
									}},
								}},
							},
						},
						"as": "community",
					},
				},
			}

			opts := options.Aggregate().SetMaxTime(2 * time.Second)

			cursor, err := collection.Aggregate(ctx, mongo.Pipeline{
				match,
				project,
				lookupAuthor,
				lookupCommunity,
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

			json.NewEncoder(response).Encode(results[0])
		}
	}
}

// GetPersonal fetch personal posts from database
func GetPersonal(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")

		params := mux.Vars(request)
		page, err := strconv.Atoi(params["page"])
		if err != nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}
		limit, _ := strconv.Atoi(os.Getenv("POST_LIMIT"))
		authID, _, ok := request.BasicAuth()

		if ok {
			usersCollection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
			postsCollection := db.Database(os.Getenv("DATABASE_NAME")).Collection("posts")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			defer cancel()

			communityID, _ := primitive.ObjectIDFromHex("60049bc9888d8b3284e5cb4f")
			oID, _ := primitive.ObjectIDFromHex(authID)

			var user bson.M
			userOptions := options.FindOne().SetProjection(bson.D{primitive.E{Key: "follows", Value: "$follows"}})
			err := usersCollection.FindOne(ctx, bson.D{primitive.E{Key: "_id", Value: oID}}, userOptions).Decode(&user)
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			authors := make([]interface{}, len(user["follows"].(primitive.A))+1)
			for i, v := range user["follows"].(primitive.A) {
				authors[i] = v
			}
			authors = append(authors, "$author")

			match := bson.D{
				primitive.E{
					Key: "$match",
					Value: bson.D{
						primitive.E{Key: "community", Value: communityID},
						primitive.E{Key: "author", Value: bson.D{primitive.E{
							Key:   "$in",
							Value: authors,
						}}},
					},
				},
			}

			project := bson.D{
				primitive.E{
					Key: "$project",
					Value: bson.D{
						primitive.E{Key: "_id", Value: "$_id"},
						primitive.E{Key: "community", Value: "$community"},
						primitive.E{Key: "images", Value: "$images"},
						primitive.E{Key: "tags", Value: "$tags"},
						primitive.E{Key: "title", Value: "$title"},
						primitive.E{Key: "content", Value: "$content"},
						primitive.E{Key: "date", Value: "$date"},
						primitive.E{Key: "author", Value: "$author"},
						primitive.E{Key: "upvoted", Value: bson.D{
							primitive.E{Key: "$in", Value: []interface{}{oID, "$upvotes"}},
						}},
						primitive.E{Key: "answers", Value: bson.D{
							primitive.E{Key: "$size", Value: "$answers"},
						}},
						primitive.E{Key: "upvotes", Value: bson.D{
							primitive.E{Key: "$size", Value: "$upvotes"},
						}},
					},
				},
			}

			lookupAuthor := bson.D{
				primitive.E{
					Key: "$lookup",
					Value: bson.M{
						"from":         "users",
						"localField":   "author",
						"foreignField": "_id",
						"as":           "author",
					},
				},
			}

			lookupCommunity := bson.D{
				primitive.E{
					Key: "$lookup",
					Value: bson.M{
						"from":         "communities",
						"localField":   "community",
						"foreignField": "_id",
						"as":           "community",
					},
				},
			}

			sort := bson.D{
				primitive.E{
					Key: "$sort",
					Value: bson.D{primitive.E{
						Key:   "date",
						Value: -1,
					}},
				},
			}

			skip := bson.D{
				primitive.E{
					Key:   "$skip",
					Value: (page - 1) * limit,
				},
			}

			limit := bson.D{
				primitive.E{
					Key:   "$limit",
					Value: limit,
				},
			}

			opts := options.Aggregate().SetMaxTime(2 * time.Second)

			cursor, err := postsCollection.Aggregate(ctx, mongo.Pipeline{
				match,
				project,
				lookupAuthor,
				lookupCommunity,
				sort,
				skip,
				limit,
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

			mapped := make([]bson.M, len(results))
			for i, v := range results {
				v["author"] = formatAuthor(v["author"].(primitive.A)[0].(primitive.M))
				v["community"] = formatCommunity(v["community"].(primitive.A)[0].(primitive.M))
				mapped[i] = v
			}

			json.NewEncoder(response).Encode(mapped)
		}
	}
}

// GetNew fetch personal posts from database
func GetNew(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")

		params := mux.Vars(request)
		page, err := strconv.Atoi(params["page"])
		if err != nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}
		limit, _ := strconv.Atoi(os.Getenv("POST_LIMIT"))
		authID, _, ok := request.BasicAuth()

		if ok {
			collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("posts")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			defer cancel()

			communityID, _ := primitive.ObjectIDFromHex("60049bc9888d8b3284e5cb4f")
			oID, _ := primitive.ObjectIDFromHex(authID)

			match := bson.D{
				primitive.E{
					Key: "$match",
					Value: bson.D{
						primitive.E{Key: "community", Value: communityID},
					},
				},
			}

			project := bson.D{
				primitive.E{
					Key: "$project",
					Value: bson.D{
						primitive.E{Key: "_id", Value: "$_id"},
						primitive.E{Key: "community", Value: "$community"},
						primitive.E{Key: "images", Value: "$images"},
						primitive.E{Key: "tags", Value: "$tags"},
						primitive.E{Key: "title", Value: "$title"},
						primitive.E{Key: "content", Value: "$content"},
						primitive.E{Key: "date", Value: "$date"},
						primitive.E{Key: "author", Value: "$author"},
						primitive.E{Key: "upvoted", Value: bson.D{
							primitive.E{Key: "$in", Value: []interface{}{oID, "$upvotes"}},
						}},
						primitive.E{Key: "answers", Value: bson.D{
							primitive.E{Key: "$size", Value: "$answers"},
						}},
						primitive.E{Key: "upvotes", Value: bson.D{
							primitive.E{Key: "$size", Value: "$upvotes"},
						}},
					},
				},
			}

			lookupAuthor := bson.D{
				primitive.E{
					Key: "$lookup",
					Value: bson.M{
						"from":         "users",
						"localField":   "author",
						"foreignField": "_id",
						"as":           "author",
					},
				},
			}

			lookupCommunity := bson.D{
				primitive.E{
					Key: "$lookup",
					Value: bson.M{
						"from":         "communities",
						"localField":   "community",
						"foreignField": "_id",
						"as":           "community",
					},
				},
			}

			sort := bson.D{
				primitive.E{
					Key: "$sort",
					Value: bson.D{primitive.E{
						Key:   "date",
						Value: -1,
					}},
				},
			}

			skip := bson.D{
				primitive.E{
					Key:   "$skip",
					Value: (page - 1) * limit,
				},
			}

			limit := bson.D{
				primitive.E{
					Key:   "$limit",
					Value: limit,
				},
			}

			opts := options.Aggregate().SetMaxTime(2 * time.Second)

			cursor, err := collection.Aggregate(ctx, mongo.Pipeline{
				match,
				project,
				lookupAuthor,
				lookupCommunity,
				sort,
				skip,
				limit,
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

			mapped := make([]bson.M, len(results))
			for i, v := range results {
				v["author"] = formatAuthor(v["author"].(primitive.A)[0].(primitive.M))
				v["community"] = formatCommunity(v["community"].(primitive.A)[0].(primitive.M))
				mapped[i] = v
			}

			json.NewEncoder(response).Encode(mapped)
		}
	}
}

// GetLiked fetch personal posts from database
func GetLiked(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")

		params := mux.Vars(request)
		page, err := strconv.Atoi(params["page"])
		if err != nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}
		limit, _ := strconv.Atoi(os.Getenv("POST_LIMIT"))
		authID, _, ok := request.BasicAuth()

		if ok {
			collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("posts")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			defer cancel()

			communityID, _ := primitive.ObjectIDFromHex("60049bc9888d8b3284e5cb4f")
			oID, _ := primitive.ObjectIDFromHex(authID)

			match := bson.D{
				primitive.E{
					Key: "$match",
					Value: bson.D{
						primitive.E{Key: "community", Value: communityID},
					},
				},
			}

			project := bson.D{
				primitive.E{
					Key: "$project",
					Value: bson.D{
						primitive.E{Key: "_id", Value: "$_id"},
						primitive.E{Key: "community", Value: "$community"},
						primitive.E{Key: "images", Value: "$images"},
						primitive.E{Key: "tags", Value: "$tags"},
						primitive.E{Key: "title", Value: "$title"},
						primitive.E{Key: "content", Value: "$content"},
						primitive.E{Key: "date", Value: "$date"},
						primitive.E{Key: "author", Value: "$author"},
						primitive.E{Key: "upvoted", Value: bson.D{
							primitive.E{Key: "$in", Value: []interface{}{oID, "$upvotes"}},
						}},
						primitive.E{Key: "answers", Value: bson.D{
							primitive.E{Key: "$size", Value: "$answers"},
						}},
						primitive.E{Key: "upvotes", Value: bson.D{
							primitive.E{Key: "$size", Value: "$upvotes"},
						}},
					},
				},
			}

			lookupAuthor := bson.D{
				primitive.E{
					Key: "$lookup",
					Value: bson.M{
						"from":         "users",
						"localField":   "author",
						"foreignField": "_id",
						"as":           "author",
					},
				},
			}

			lookupCommunity := bson.D{
				primitive.E{
					Key: "$lookup",
					Value: bson.M{
						"from":         "communities",
						"localField":   "community",
						"foreignField": "_id",
						"as":           "community",
					},
				},
			}

			sort := bson.D{
				primitive.E{
					Key: "$sort",
					Value: bson.D{primitive.E{
						Key:   "upvotes",
						Value: -1,
					}, primitive.E{
						Key:   "date",
						Value: -1,
					}},
				},
			}

			skip := bson.D{
				primitive.E{
					Key:   "$skip",
					Value: (page - 1) * limit,
				},
			}

			limit := bson.D{
				primitive.E{
					Key:   "$limit",
					Value: limit,
				},
			}

			opts := options.Aggregate().SetMaxTime(2 * time.Second)

			cursor, err := collection.Aggregate(ctx, mongo.Pipeline{
				match,
				project,
				lookupAuthor,
				lookupCommunity,
				sort,
				skip,
				limit,
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

			mapped := make([]bson.M, len(results))
			for i, v := range results {
				v["author"] = formatAuthor(v["author"].(primitive.A)[0].(primitive.M))
				v["community"] = formatCommunity(v["community"].(primitive.A)[0].(primitive.M))
				mapped[i] = v
			}

			json.NewEncoder(response).Encode(mapped)
		}
	}
}

// DeletePost delete post from database
func DeletePost(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")
		params := mux.Vars(request)

		postCollection := db.Database(os.Getenv("DATABASE_NAME")).Collection("posts")
		commentsCollection := db.Database(os.Getenv("DATABASE_NAME")).Collection("comments")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		defer cancel()

		authID, _, ok := request.BasicAuth()

		if ok {
			oID, _ := primitive.ObjectIDFromHex(authID)

			id, err := primitive.ObjectIDFromHex(params["id"])
			if err != nil {
				response.WriteHeader(http.StatusBadRequest)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			var result bson.M
			postFilter := bson.D{primitive.E{Key: "_id", Value: id}}
			opts := options.FindOne().SetProjection(bson.D{primitive.E{Key: "author", Value: "$author"}})
			postCollection.FindOne(ctx, postFilter, opts).Decode(&result)

			if result["author"] == nil {
				response.WriteHeader(http.StatusNotFound)
				response.Write([]byte(`{ "message": "Post Not Found" }`))
				return
			}

			if result["author"] != oID {
				response.WriteHeader(http.StatusUnauthorized)
				response.Write([]byte(`{ "message": "Unauthorized" }`))
				return
			}

			commentFilter := bson.D{primitive.E{Key: "post", Value: id}}
			commentsCollection.DeleteMany(ctx, commentFilter)
			postCollection.FindOneAndDelete(ctx, postFilter)

			response.Write([]byte(`{ "message": "OK" }`))
		}

	}
}

// PostAction is for upvoting and downvoting posts
func PostAction(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")
		actionType := mux.Vars(request)["type"]

		authID, _, ok := request.BasicAuth()

		if ok {
			oID, _ := primitive.ObjectIDFromHex(authID)
			var action PostActionModel
			err := json.NewDecoder(request.Body).Decode(&action)

			if err == nil {
				if actionType == "upvote" {
					upvote(db, response, request, oID, action.ID)
					return
				} else if actionType == "downvote" {
					downvote(db, response, request, oID, action.ID)
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

func upvote(
	db *mongo.Client,
	response http.ResponseWriter,
	request *http.Request,
	upvoterID primitive.ObjectID,
	postID primitive.ObjectID,
) error {
	collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("posts")
	usersCollection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	results := upvoted(db, response, request, upvoterID, postID)

	if len(results) > 0 {
		if results[0]["upvoted"].(bool) {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "Post already upvoted" }`))
			return nil
		}
		updateOpts := options.FindOneAndUpdate().SetUpsert(true)

		filter := bson.D{primitive.E{Key: "_id", Value: postID}}
		update := bson.D{primitive.E{
			Key: "$push", Value: bson.D{primitive.E{Key: "upvotes", Value: upvoterID}},
		}}
		collection.FindOneAndUpdate(ctx, filter, update, updateOpts)

		// Send notification
		var upvoter, upvotee, post bson.M

		readOpts := options.FindOne().SetProjection(bson.D{primitive.E{Key: "author", Value: "$author"}})
		filter = bson.D{primitive.E{Key: "_id", Value: postID}}
		collection.FindOne(ctx, filter, readOpts).Decode(&post)

		opts := options.FindOne().SetProjection(bson.D{
			primitive.E{Key: "_id", Value: "$_id"},
			primitive.E{Key: "fullname", Value: "$fullname"},
			primitive.E{Key: "username", Value: "$username"},
			primitive.E{Key: "language", Value: "$language"},
		})
		usersCollection.FindOne(ctx, bson.D{primitive.E{Key: "_id", Value: upvoterID}}, opts).Decode(&upvoter)
		usersCollection.FindOne(ctx, bson.D{primitive.E{Key: "_id", Value: post["author"]}}, opts).Decode(&upvotee)

		if upvoter["_id"] != upvotee["_id"] {
			notification.SendNotification(post["author"].(primitive.ObjectID), messaging.Notification{
				Title: config.Languages[upvotee["language"].(string)].UpvoteTitle(),
				Body:  config.Languages[upvotee["language"].(string)].PostUpvote(upvoter["fullname"].(string) + " (@" + upvoter["username"].(string) + ")"),
			}, db)
		}

		response.Write([]byte(`{ "message": "OK" }`))
		return nil
	}

	response.WriteHeader(http.StatusNotFound)
	response.Write([]byte(`{ "message": "Post Not Found" }`))
	return nil
}

func downvote(
	db *mongo.Client,
	response http.ResponseWriter,
	request *http.Request,
	upvoterID primitive.ObjectID,
	postID primitive.ObjectID,
) error {
	collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	results := upvoted(db, response, request, upvoterID, postID)

	if len(results) > 0 {
		if results[0]["upvoted"].(bool) == false {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "Post already downvoted" }`))
			return nil
		}
		updateOpts := options.FindOneAndUpdate().SetUpsert(true)

		filter := bson.D{primitive.E{Key: "_id", Value: postID}}
		update := bson.D{primitive.E{
			Key: "$pull", Value: bson.D{primitive.E{Key: "upvotes", Value: upvoterID}},
		}}
		collection.FindOneAndUpdate(ctx, filter, update, updateOpts)

		response.Write([]byte(`{ "message": "OK" }`))
		return nil
	}

	response.WriteHeader(http.StatusNotFound)
	response.Write([]byte(`{ "message": "Post Not Found" }`))
	return nil
}

func upvoted(
	db *mongo.Client,
	response http.ResponseWriter,
	request *http.Request,
	upvoterID primitive.ObjectID,
	postID primitive.ObjectID,
) []bson.M {
	collection := db.Database(os.Getenv("DATABASE_NAME")).Collection("posts")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	aggregateOpts := options.Aggregate().SetMaxTime(2 * time.Second)
	match := bson.D{
		primitive.E{Key: "$match", Value: bson.D{primitive.E{Key: "_id", Value: postID}}},
	}
	project := bson.D{
		primitive.E{
			Key: "$project",
			Value: bson.D{
				primitive.E{Key: "upvoted", Value: bson.D{
					primitive.E{Key: "$in", Value: []interface{}{upvoterID, "$upvotes"}},
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

func formatAuthor(author primitive.M) primitive.M {
	result := primitive.M{}

	result["_id"] = author["_id"]
	result["fullname"] = author["fullname"]
	result["username"] = author["username"]
	result["image"] = author["image"]
	result["verified"] = author["verified"]

	return result
}

func formatCommunity(community primitive.M) primitive.M {
	result := primitive.M{}

	result["_id"] = community["_id"]
	result["title"] = community["title"]
	result["image"] = community["image"]

	return result
}
