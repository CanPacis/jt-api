package search

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

// ContentResult result model for search calls
type ContentResult struct {
	Length      int      `json:"length" bson:"length"`
	Users       []bson.M `json:"users" bson:"users"`
	Posts       []bson.M `json:"posts" bson:"posts"`
	Communities []bson.M `json:"communities" bson:"communities"`
}

// Content is for searching general content
func Content(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")
		params := mux.Vars(request)

		usersCollection := db.Database(os.Getenv("DATABASE_NAME")).Collection("users")
		postsCollection := db.Database(os.Getenv("DATABASE_NAME")).Collection("posts")
		communitiesCollection := db.Database(os.Getenv("DATABASE_NAME")).Collection("communities")

		usersChan := make(chan []bson.M, 1)
		postsChan := make(chan []bson.M, 1)
		communitiesChan := make(chan []bson.M, 1)

		go getUserResults(usersChan, usersCollection, params)
		go getPostResults(postsChan, postsCollection, params)
		go getCommunityResults(communitiesChan, communitiesCollection, params)

		userResults := <-usersChan
		postResults := <-postsChan
		communityResults := <-communitiesChan

		result := ContentResult{
			Users:       userResults,
			Posts:       postResults,
			Communities: communityResults,
			Length:      len(userResults) + len(postResults) + len(communityResults),
		}

		json.NewEncoder(response).Encode(result)
	}
}

func getUserResults(channel chan []bson.M, collection *mongo.Collection, params map[string]string) {
	opts := options.Aggregate().SetMaxTime(2 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	match := bson.D{
		primitive.E{Key: "$match", Value: bson.D{
			primitive.E{
				Key: "$or",
				Value: []interface{}{
					bson.D{
						primitive.E{
							Key: "username",
							Value: bson.D{
								primitive.E{
									Key:   "$regex",
									Value: params["query"],
								},
								primitive.E{Key: "$options", Value: "i"},
							},
						},
					},
					bson.D{
						primitive.E{
							Key: "fullname",
							Value: bson.D{
								primitive.E{
									Key:   "$regex",
									Value: params["query"],
								},
								primitive.E{Key: "$options", Value: "i"},
							},
						},
					},
				},
			},
		}},
	}

	project := bson.D{
		primitive.E{
			Key: "$project",
			Value: bson.D{
				primitive.E{Key: "_id", Value: "$_id"},
				primitive.E{Key: "username", Value: "$username"},
				primitive.E{Key: "fullname", Value: "$fullname"},
				primitive.E{Key: "image", Value: "$image"},
				primitive.E{Key: "verified", Value: "$verified"},
				primitive.E{Key: "followers", Value: bson.D{
					primitive.E{Key: "$size", Value: "$followers"},
				}},
			},
		},
	}

	// userSort := bson.D{
	// 	primitive.E{
	// 		Key: "$sort",
	// 		Value: primitive.E{
	// 			Key:   "",
	// 			Value: 1,
	// 		},
	// 	},
	// }

	cursor, _ := collection.Aggregate(ctx, mongo.Pipeline{match, project}, opts)
	results := []bson.M{}
	if err := cursor.All(context.TODO(), &results); err != nil {
		log.Fatal(err)
	}
	channel <- results
}

func getPostResults(channel chan []bson.M, collection *mongo.Collection, params map[string]string) {
	opts := options.Aggregate().SetMaxTime(2 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	match := bson.D{
		primitive.E{Key: "$match", Value: bson.D{
			primitive.E{
				Key: "$or",
				Value: []interface{}{
					bson.D{
						primitive.E{
							Key: "title",
							Value: bson.D{
								primitive.E{
									Key:   "$regex",
									Value: params["query"],
								},
								primitive.E{Key: "$options", Value: "i"},
							},
						},
					},
					bson.D{
						primitive.E{
							Key: "tags",
							Value: bson.D{
								primitive.E{
									Key:   "$in",
									Value: []interface{}{params["query"]},
								},
							},
						},
					},
				},
			},
		}},
	}

	project := bson.D{
		primitive.E{
			Key: "$project",
			Value: bson.D{
				primitive.E{Key: "_id", Value: "$_id"},
				primitive.E{Key: "title", Value: "$title"},
				primitive.E{Key: "content", Value: "$content"},
				primitive.E{Key: "upvotes", Value: bson.D{
					primitive.E{Key: "$size", Value: "$upvotes"},
				}},
				primitive.E{Key: "answers", Value: bson.D{
					primitive.E{Key: "$size", Value: "$answers"},
				}},
			},
		},
	}

	// postSort := bson.D{
	// 	primitive.E{
	// 		Key: "$sort",
	// 		Value: primitive.E{
	// 			Key:   "",
	// 			Value: 1,
	// 		},
	// 	},
	// }

	cursor, _ := collection.Aggregate(ctx, mongo.Pipeline{match, project}, opts)
	results := []bson.M{}
	if err := cursor.All(context.TODO(), &results); err != nil {
		log.Fatal(err)
	}
	channel <- results
}

func getCommunityResults(channel chan []bson.M, collection *mongo.Collection, params map[string]string) {
	opts := options.Aggregate().SetMaxTime(2 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	match := bson.D{
		primitive.E{Key: "$match", Value: bson.D{
			primitive.E{
				Key: "$or",
				Value: []interface{}{
					bson.D{
						primitive.E{
							Key: "title",
							Value: bson.D{
								primitive.E{
									Key:   "$regex",
									Value: params["query"],
								},
								primitive.E{Key: "$options", Value: "i"},
							},
						},
					},
				},
			},
		}},
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
			},
		},
	}

	// communitySort := bson.D{
	// 	primitive.E{
	// 		Key: "$sort",
	// 		Value: primitive.E{
	// 			Key:   "",
	// 			Value: 1,
	// 		},
	// 	},
	// }

	cursor, _ := collection.Aggregate(ctx, mongo.Pipeline{match, project}, opts)

	results := []bson.M{}
	if err := cursor.All(context.TODO(), &results); err != nil {
		log.Fatal(err)
	}

	channel <- results
}
