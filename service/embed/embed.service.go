package embed

import (
	"io/ioutil"
	"jt-api/service/posts"
	"net/http"
	"os"
	"path"
	"text/template"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Post returns an html post embed for websites
func Post(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "text/html; charset=utf-8")

		params := mux.Vars(request)
		id, err := primitive.ObjectIDFromHex(params["id"])

		post, err := posts.AnonymousPost(db, id)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		dir, err := os.Getwd()
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		data, err := ioutil.ReadFile(path.Join(dir, "service/embed/post.html"))
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		tmpl, err := template.New("test").Parse(string(data))
		if err != nil {
			panic(err)
		}

		date := post["date"].(primitive.DateTime).Time()
		post["date"] = date.Format("02-Jan-2006")

		tmpl.Execute(response, post)
	}
}
