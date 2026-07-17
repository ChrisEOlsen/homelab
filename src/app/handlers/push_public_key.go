package handlers

import (
	"net/http"
	"os"
)

func PushPublicKeyGET() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := os.Getenv("VAPID_PUBLIC_KEY")
		if key == "" {
			jsonError(w, "push notifications are not configured", 503)
			return
		}
		jsonOK(w, map[string]string{"key": key})
	}
}
