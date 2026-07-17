package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func PushSubscribePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Endpoint string `json:"endpoint"`
			Keys     struct {
				P256dh string `json:"p256dh"`
				Auth   string `json:"auth"`
			} `json:"keys"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
			body.Endpoint == "" || body.Keys.P256dh == "" || body.Keys.Auth == "" {
			jsonError(w, "endpoint and keys are required", 400)
			return
		}
		model := models.NewPushSubscriptionModel(readDB, writeDB)
		if err := model.Create(body.Endpoint, body.Keys.P256dh, body.Keys.Auth); err != nil {
			jsonError(w, "failed to save subscription", 500)
			return
		}
		jsonOK(w, nil)
	}
}
