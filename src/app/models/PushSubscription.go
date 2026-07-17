package models

import (
	"database/sql"
)

type PushSubscription struct {
	ID        int64  `json:"id"`
	Endpoint  string `json:"endpoint"`
	P256dh    string `json:"p256dh"`
	Auth      string `json:"auth"`
	CreatedAt string `json:"created_at"`
}

type PushSubscriptionModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
}

func NewPushSubscriptionModel(readDB, writeDB *sql.DB) *PushSubscriptionModel {
	return &PushSubscriptionModel{readDB: readDB, writeDB: writeDB}
}

// Create is idempotent per endpoint — re-subscribing the same device
// (e.g. after reopening the app) must not create a duplicate row.
func (m *PushSubscriptionModel) Create(endpoint, p256dh, auth string) error {
	_, err := m.writeDB.Exec(
		"INSERT OR IGNORE INTO push_subscriptions (endpoint, p256dh, auth) VALUES (?, ?, ?)",
		endpoint, p256dh, auth,
	)
	return err
}

func (m *PushSubscriptionModel) GetAll() ([]PushSubscription, error) {
	rows, err := m.readDB.Query("SELECT id, endpoint, p256dh, auth, created_at FROM push_subscriptions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PushSubscription
	for rows.Next() {
		var item PushSubscription
		if err := rows.Scan(&item.ID, &item.Endpoint, &item.P256dh, &item.Auth, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// DeleteByEndpoint is called when webpush-go's SendNotification reports the
// subscription is gone (404/410) — the browser install was uninstalled or
// the subscription expired, so it self-cleans instead of erroring forever.
func (m *PushSubscriptionModel) DeleteByEndpoint(endpoint string) error {
	_, err := m.writeDB.Exec("DELETE FROM push_subscriptions WHERE endpoint = ?", endpoint)
	return err
}
