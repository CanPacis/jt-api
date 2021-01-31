package config

// Language is model for language data
type Language struct {
	FollowRequest  func(data string) string
	FollowStart    func(data string) string
	NewFollow      func() string
	NewComment     func() string
	UpvoteTitle    func() string
	PostUpvote     func(data string) string
	PostMention    func(data interface{}) string
	PostComment    func(data string) string
	CommentUpvote  func(data string) string
	CommentMention func(data interface{}) string
}

// Languages is language data
var Languages = map[string]Language{
	"en": {
		FollowRequest: func(data string) string {
			return data + " wants to follow you"
		},
		FollowStart: func(data string) string {
			return data + " started following you"
		},
		NewFollow: func() string {
			return "A New Follower!"
		},
		NewComment: func() string {
			return "A Comment!"
		},
		UpvoteTitle: func() string {
			return "An Upvote!"
		},
		PostUpvote: func(data string) string {
			return data + " upvoted your post"
		},
		PostMention: func(data interface{}) string {
			return ""
		},
		PostComment: func(data string) string {
			return data + " commented on your post"
		},
		CommentUpvote: func(data string) string {
			return data + " upvoted your comment"
		},
		CommentMention: func(data interface{}) string {
			return ""
		},
	},
	"tr": {
		FollowRequest: func(data string) string {
			return data + " seni takip etmek istiyor"
		},
		FollowStart: func(data string) string {
			return data + " seni takip etmeye başladı"
		},
		NewFollow: func() string {
			return "Yeni Bir Takipçi!"
		},
		NewComment: func() string {
			return "Bir Yorum!"
		},
		UpvoteTitle: func() string {
			return "Bir Oylama!"
		},
		PostUpvote: func(data string) string {
			return data + " paylaşımını oyladı"
		},
		PostMention: func(data interface{}) string {
			return ""
		},
		PostComment: func(data string) string {
			return data + " paylaşımına yorum yaptı"
		},
		CommentUpvote: func(data string) string {
			return data + " yorumunu oyladı"
		},
		CommentMention: func(data interface{}) string {
			return ""
		},
	},
}
