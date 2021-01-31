package middleware

import (
	"encoding/json"
	"fmt"
	"jt-api/service/auth"
	"net/http"
	"os"
	"strings"

	"github.com/dgrijalva/jwt-go"
)

// AuthMiddleware authentication middleware
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		response.Header().Add("content-type", "application/json")
		header := request.Header.Get("Authorization")

		if header == "" {
			response.WriteHeader(http.StatusUnauthorized)
			response.Write([]byte(`{ "message": "Unauthorized" }`))
			return
		}

		list := strings.Split(header, "Bearer ")

		if len(list) < 2 {
			response.WriteHeader(http.StatusUnauthorized)
			response.Write([]byte(`{ "message": "Unauthorized" }`))
			return
		}

		token, err := jwt.Parse(list[1], func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
			}

			return []byte(os.Getenv("JWT_SECRET")), nil
		})
		if err != nil {
			response.WriteHeader(http.StatusUnauthorized)
			response.Write([]byte(`{ "message": "Unauthorized" }`))
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			var user auth.LoginUser
			data, _ := json.Marshal(claims["user"])
			json.Unmarshal(data, &user)
			request.SetBasicAuth(user.ID.Hex(), "")
			next(response, request)
		} else {
			response.WriteHeader(http.StatusUnauthorized)
			response.Write([]byte(`{ "message": "Unauthorized" }`))
			return
		}
	}
}
