package upload

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"go.mongodb.org/mongo-driver/mongo"
)

// ImageUpload uploaded image model response
type ImageUpload struct {
	Path string `json:"path" bson:"path"`
}

// Image upload file to aws s3 bucket
func Image(db *mongo.Client) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")

		file, header, err := request.FormFile("image")
		if err != nil {
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		defer file.Close()

		sess := session.Must(session.NewSession(&aws.Config{
			Region: aws.String("eu-central-1"),
		}))

		uploader := s3manager.NewUploader(sess)
		extension := strings.Split(header.Header.Get("Content-Type"), "/")[1]
		r := regexp.MustCompile(`[\s+=.:-]`)
		name := r.ReplaceAllString(header.Filename+time.Now().String(), "") + "." + extension

		result, err := uploader.Upload(&s3manager.UploadInput{
			ACL:    aws.String("public-read"),
			Bucket: aws.String(os.Getenv("AWS_BUCKET")),
			Key:    aws.String(name),
			Body:   file,
		})
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `" }`))
			return
		}

		data := ImageUpload{Path: result.Location}
		json.NewEncoder(response).Encode(data)
	}
}
