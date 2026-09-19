package auth

import (
	"context"
	"math/rand"
	"net/http"

	"github.com/topi314/campfire-event-manager/server/database"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

type sessionKey struct{}

var sessionContextKey = &sessionKey{}

func SetSession(ctx context.Context, session database.SessionWithUser) context.Context {
	return context.WithValue(ctx, sessionContextKey, session)
}

func GetSession(r *http.Request) (database.SessionWithUser, bool) {
	v := r.Context().Value(sessionContextKey)
	if v == nil {
		return database.SessionWithUser{}, false
	}
	return v.(database.SessionWithUser), true
}

func RandomStr(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
