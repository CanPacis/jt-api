package comments

import (
	"context"
	"encoding/json"
	"jt-api/config"
	"jt-api/service/notification"
	"log"
	"net/http"
	"time"

	"firebase.google.com/go/messaging"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CreateCommentModel common comment model for creating new comment
type CreateCommentModel struct {
	ID     primitive.ObjectID `json:"_id" bson:"_id"`
	Answer Comment            `json:"answer" bson:"answer"`
}

// Comment common comment model
type Comment struct {
	ID      primitive.ObjectID    `json:"_id,omitempty" bson:"_id,omitempty"`
	Post    primitive.ObjectID    `json:"post,omitempty" bson:"post,omitempty"`
	Author  primitive.ObjectID    `json:"author,omitempty" bson:"author,omitempty"`
	Date    primitive.DateTime    `json:"date,omitempty" bson:"date,omitempty"`
	Parent  primitive.ObjectID    `json:"parent" bson:"parent"`
	Content *[]interface{}        `json:"content" bson:"content"`
	Upvotes *[]primitive.ObjectID `json:"upvotes" bson:"upvotes"`
	Answers *[]Comment            `json:"answers" bson:"answers"`
}

// CommentActionModel is model for upvoting and downvoting actions
type CommentActionModel struct {
	ID primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
}

// GetComments fetch comments of a post from database
func GetComments(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")
		params := mux.Vars(request)

		collection := db.Database("justhink-dev").Collection("comments")
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

			match := bson.D{
				primitive.E{
					Key: "$match",
					Value: bson.D{
						primitive.E{Key: "post", Value: id},
					},
				},
			}

			project := bson.D{
				primitive.E{
					Key: "$project",
					Value: bson.D{
						primitive.E{Key: "_id", Value: "$_id"},
						primitive.E{Key: "author", Value: "$author"},
						primitive.E{Key: "content", Value: "$content"},
						primitive.E{Key: "date", Value: "$date"},
						primitive.E{Key: "post", Value: "$post"},
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

			lookup := bson.D{
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
									primitive.E{Key: "_id", Value: bson.D{primitive.E{Key: "$toString", Value: "$_id"}}},
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
										Value: "can_pacis",
									},
								}},
							},
						},
						"as": "author",
					},
				},
			}

			opts := options.Aggregate().SetMaxTime(2 * time.Second)

			cursor, err := collection.Aggregate(ctx, mongo.Pipeline{match, project, lookup}, opts)

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

// CreateComment create comment and register to database
func CreateComment(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")

		postsCollection := db.Database("justhink-dev").Collection("posts")
		commentsCollection := db.Database("justhink-dev").Collection("comments")
		usersCollection := db.Database("justhink-dev").Collection("users")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		defer cancel()

		authID, _, ok := request.BasicAuth()

		if ok {
			oID, _ := primitive.ObjectIDFromHex(authID)

			var comment CreateCommentModel
			err := json.NewDecoder(request.Body).Decode(&comment)

			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
				return
			}

			if comment.ID.Hex() == "000000000000000000000000" {
				response.WriteHeader(http.StatusBadRequest)
				response.Write([]byte(`{ "message": "PostID is not given" }`))
				return
			}

			if comment.Answer.Content == nil {
				response.WriteHeader(http.StatusBadRequest)
				response.Write([]byte(`{ "message": "Content is not given" }`))
				return
			}

			comment.Answer.ID = primitive.NewObjectID()
			comment.Answer.Author = oID
			comment.Answer.Post = comment.ID
			comment.Answer.Date = primitive.NewDateTimeFromTime(time.Now())
			comment.Answer.Upvotes = &[]primitive.ObjectID{}
			comment.Answer.Answers = &[]Comment{}

			opts := options.FindOneAndUpdate().SetUpsert(true)
			filter := bson.D{
				primitive.E{Key: "_id", Value: comment.ID},
			}

			update := bson.D{
				primitive.E{
					Key: "$push", Value: bson.D{bson.E{
						Key:   "answers",
						Value: comment.Answer.ID,
					}},
				},
			}

			postsCollection.FindOneAndUpdate(ctx, filter, update, opts)
			commentsCollection.InsertOne(ctx, comment.Answer)

			// Send notification
			var commentator, commentee, post bson.M

			readOpts := options.FindOne().SetProjection(bson.D{primitive.E{Key: "author", Value: "$author"}})
			filter = bson.D{primitive.E{Key: "_id", Value: comment.ID}}
			postsCollection.FindOne(ctx, filter, readOpts).Decode(&post)

			notificationOpts := options.FindOne().SetProjection(bson.D{
				primitive.E{Key: "_id", Value: "$_id"},
				primitive.E{Key: "fullname", Value: "$fullname"},
				primitive.E{Key: "username", Value: "$username"},
				primitive.E{Key: "language", Value: "$language"},
			})
			usersCollection.FindOne(ctx, bson.D{primitive.E{Key: "_id", Value: oID}}, notificationOpts).Decode(&commentator)
			usersCollection.FindOne(ctx, bson.D{primitive.E{Key: "_id", Value: post["author"]}}, notificationOpts).Decode(&commentee)

			notification.SendNotification(post["author"].(primitive.ObjectID), messaging.Notification{
				Title: config.Languages[commentee["language"].(string)].NewComment(),
				Body:  config.Languages[commentee["language"].(string)].PostComment(commentator["fullname"].(string) + " (@" + commentator["username"].(string) + ")"),
			}, db)

			response.Write([]byte(`{ "message": "OK" }`))
		}
	}
}

// DeleteComment delete comment from database
func DeleteComment(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")
		params := mux.Vars(request)

		collection := db.Database("justhink-dev").Collection("comments")
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
			filter := bson.D{primitive.E{Key: "_id", Value: id}}
			opts := options.FindOne().SetProjection(bson.D{primitive.E{Key: "author", Value: "$author"}})
			collection.FindOne(ctx, filter, opts).Decode(&result)

			if result["author"] == nil {
				response.WriteHeader(http.StatusNotFound)
				response.Write([]byte(`{ "message": "Comment Not Found" }`))
				return
			}

			if result["author"] != oID {
				response.WriteHeader(http.StatusUnauthorized)
				response.Write([]byte(`{ "message": "Unauthorized" }`))
				return
			}

			collection.FindOneAndDelete(ctx, filter)

			response.Write([]byte(`{ "message": "OK" }`))
		}

	}
}

// CommentAction is for upvoting and downvoting posts
func CommentAction(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")
		actionType := mux.Vars(request)["type"]

		authID, _, ok := request.BasicAuth()

		if ok {
			oID, _ := primitive.ObjectIDFromHex(authID)
			var action CommentActionModel
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
	commentID primitive.ObjectID,
) error {
	collection := db.Database("justhink-dev").Collection("comments")
	usersCollection := db.Database("justhink-dev").Collection("users")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	results := upvoted(db, response, request, upvoterID, commentID)

	if len(results) > 0 {
		if results[0]["upvoted"].(bool) {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "Comment already upvoted" }`))
			return nil
		}
		updateOpts := options.FindOneAndUpdate().SetUpsert(true)

		filter := bson.D{primitive.E{Key: "_id", Value: commentID}}
		update := bson.D{primitive.E{
			Key: "$push", Value: bson.D{primitive.E{Key: "upvotes", Value: upvoterID}},
		}}
		collection.FindOneAndUpdate(ctx, filter, update, updateOpts)

		// Send notification
		var upvoter, upvotee, post bson.M

		readOpts := options.FindOne().SetProjection(bson.D{primitive.E{Key: "author", Value: "$author"}})
		filter = bson.D{primitive.E{Key: "_id", Value: commentID}}
		collection.FindOne(ctx, filter, readOpts).Decode(&post)

		opts := options.FindOne().SetProjection(bson.D{
			primitive.E{Key: "_id", Value: "$_id"},
			primitive.E{Key: "fullname", Value: "$fullname"},
			primitive.E{Key: "username", Value: "$username"},
			primitive.E{Key: "language", Value: "$language"},
		})
		usersCollection.FindOne(ctx, bson.D{primitive.E{Key: "_id", Value: upvoterID}}, opts).Decode(&upvoter)
		usersCollection.FindOne(ctx, bson.D{primitive.E{Key: "_id", Value: post["author"]}}, opts).Decode(&upvotee)

		notification.SendNotification(post["author"].(primitive.ObjectID), messaging.Notification{
			Title: config.Languages[upvotee["language"].(string)].UpvoteTitle(),
			Body:  config.Languages[upvotee["language"].(string)].CommentUpvote(upvoter["fullname"].(string) + " (@" + upvoter["username"].(string) + ")"),
		}, db)

		response.Write([]byte(`{ "message": "OK" }`))
		return nil
	}

	response.WriteHeader(http.StatusNotFound)
	response.Write([]byte(`{ "message": "Comment Not Found" }`))
	return nil
}

func downvote(
	db *mongo.Client,
	response http.ResponseWriter,
	request *http.Request,
	upvoterID primitive.ObjectID,
	commentID primitive.ObjectID,
) error {
	collection := db.Database("justhink-dev").Collection("comments")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	results := upvoted(db, response, request, upvoterID, commentID)

	if len(results) > 0 {
		if results[0]["upvoted"].(bool) == false {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "Comment already downvoted" }`))
			return nil
		}
		updateOpts := options.FindOneAndUpdate().SetUpsert(true)

		filter := bson.D{primitive.E{Key: "_id", Value: commentID}}
		update := bson.D{primitive.E{
			Key: "$pull", Value: bson.D{primitive.E{Key: "upvotes", Value: upvoterID}},
		}}
		collection.FindOneAndUpdate(ctx, filter, update, updateOpts)

		response.Write([]byte(`{ "message": "OK" }`))
		return nil
	}

	response.WriteHeader(http.StatusNotFound)
	response.Write([]byte(`{ "message": "Comment Not Found" }`))
	return nil
}

func upvoted(
	db *mongo.Client,
	response http.ResponseWriter,
	request *http.Request,
	upvoterID primitive.ObjectID,
	commentID primitive.ObjectID,
) []bson.M {
	collection := db.Database("justhink-dev").Collection("comments")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	aggregateOpts := options.Aggregate().SetMaxTime(2 * time.Second)
	match := bson.D{
		primitive.E{Key: "$match", Value: bson.D{primitive.E{Key: "_id", Value: commentID}}},
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
