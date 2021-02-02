package main

import (
	"context"
	"fmt"
	"jt-api/middleware"
	"jt-api/service/auth"
	"jt-api/service/comments"
	"jt-api/service/notification"
	"jt-api/service/posts"
	"jt-api/service/search"
	"jt-api/service/upload"
	"jt-api/service/users"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	godotenv.Load()
	if os.Getenv("ENV") == "prod" {
		fun()
	}

	fmt.Println("Starting Justhink Backend...")
	fmt.Println("Environment variables are set")

	notification.InitFirebase()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(os.Getenv("DB_CONN_STR")))
	if err != nil {
		fmt.Println("Failed to connect to database")
		log.Fatal(err)
	}
	fmt.Println("Connected to database")

	router := mux.NewRouter()

	// Users route
	usersRoute := router.PathPrefix("/users").Subrouter()
	usersRoute.HandleFunc("/find/{id}", middleware.AuthMiddleware(users.GetUser(client))).Methods("GET")
	usersRoute.HandleFunc("/exists/{type}/{query}", users.UserExists(client)).Methods("GET")
	usersRoute.HandleFunc("/signup", users.CreateUser(client)).Methods("POST")
	usersRoute.HandleFunc("/edit", middleware.AuthMiddleware(users.EditUser(client))).Methods("POST")
	usersRoute.HandleFunc("/action/{type}", middleware.AuthMiddleware(users.UserAction(client))).Methods("POST")
	usersRoute.HandleFunc("/updateFCMToken", middleware.AuthMiddleware(users.UpdateFCMToken(client))).Methods("POST")

	// Posts route
	postsRoute := router.PathPrefix("/posts").Subrouter()
	postsRoute.HandleFunc("/delete/{id}", middleware.AuthMiddleware(posts.DeletePost(client))).Methods("GET")
	postsRoute.HandleFunc("/find/{id}", middleware.AuthMiddleware(posts.GetPost(client))).Methods("GET")
	postsRoute.HandleFunc("/personal/{page}", middleware.AuthMiddleware(posts.GetPersonal(client))).Methods("GET")
	postsRoute.HandleFunc("/new/{page}", middleware.AuthMiddleware(posts.GetNew(client))).Methods("GET")
	postsRoute.HandleFunc("/liked/{page}", middleware.AuthMiddleware(posts.GetLiked(client))).Methods("GET")
	postsRoute.HandleFunc("/create", middleware.AuthMiddleware(posts.CreatePost(client))).Methods("POST")
	postsRoute.HandleFunc("/action/{type}", middleware.AuthMiddleware(posts.PostAction(client))).Methods("POST")

	// Comments route
	commentsRoute := router.PathPrefix("/comments").Subrouter()
	commentsRoute.HandleFunc("/of/{id}/{page}", middleware.AuthMiddleware(comments.GetComments(client))).Methods("GET")
	commentsRoute.HandleFunc("/delete/{id}", middleware.AuthMiddleware(comments.DeleteComment(client))).Methods("GET")
	commentsRoute.HandleFunc("/create", middleware.AuthMiddleware(comments.CreateComment(client))).Methods("POST")
	commentsRoute.HandleFunc("/action/{type}", middleware.AuthMiddleware(comments.CommentAction(client))).Methods("POST")

	// Auth route
	authRoute := router.PathPrefix("/auth").Subrouter()
	authRoute.HandleFunc("/login", auth.Login(client)).Methods("POST")

	// Upload route
	uploadRoute := router.PathPrefix("/upload").Subrouter()
	uploadRoute.HandleFunc("/", middleware.AuthMiddleware(upload.Image(client, 512))).Methods("POST")

	// Search route
	searchRoute := router.PathPrefix("/search").Subrouter()
	searchRoute.HandleFunc("/content/{query}", search.Content(client)).Methods("GET")

	// Notification route
	notificationRoute := router.PathPrefix("/notification").Subrouter()
	notificationRoute.HandleFunc("/", middleware.AuthMiddleware(notification.GetNotifications(client))).Methods("GET")
	notificationRoute.HandleFunc("/send/u/{username}", notification.SendToUsername(client)).Methods("POST")
	notificationRoute.HandleFunc("/send/id/{id}", notification.SendToID(client)).Methods("POST")

	fmt.Println("Server is up and listening on port " + os.Getenv("PORT"))
	loggedRouter := handlers.LoggingHandler(os.Stdout, router)
	http.ListenAndServe(":"+os.Getenv("PORT"), loggedRouter)
}

func fun() {
	fmt.Println(`
	MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM
	MMMMNmhysMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMmdysMMMMMNdhsdMMMMMMMMMMNdhsdMMMMMMMMMMMMMMMMMNdysNMMMMMMMMMM
	MMMM////oMMMMMMMMMMMMMMMMMMMMMMMMMMMMMs////MMMMd////hMMMMMMMMMd////hMMMMMMMMMMMMMMMMy////NMMMMMMMMMM
	MMMMsssshMNssssdMMMssssdMMMdysossydNNs+////ssyMd////hdyooshNMMmssssmMdssssNdsoosdMMMy////NMNysssymMM
	MMMM////oMm////yMMN////yMm/////////ym/////////Md////////////mMh////dMh///////////+NMy////mh////omMMM
	MMMM////oMm////yMMN////yMy///+hddddMMdo////ddmMd////+hds////yMh////dMh////ohho////dMy////+///+dMMMMM
	MMMM////oMm////yMMN////yMNo///////odMMo////MMMMd////hMMN////yMh////dMh////mMMd////dMy////////oNMMMMM
	MMMM////oMm////oNNh////yMMdmdhyo////mMo////NMNMd////hMMN////yMh////dMh////mMMm////dMy/////////+mMMMM
	MMMM////oMN+///////////yMy///ooo////NMy///////Md////hMMN////yMh////dMh////mMMm////dMy////mNo////dMMM
	MMMN////oMMNy+//+sd++++yMmyo+///+oyNMMMho///+sMm++++dMMN++++hMd++++dMh++++NMMm++++dMh++++NMMs++++dMM
	MMo/////hMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM
	MMysssymMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM
	MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM
	`)
}
