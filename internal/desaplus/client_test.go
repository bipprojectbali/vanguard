package desaplus

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// client_test.go — 5 skenario HTTP murni (httptest.Server), tanpa DB: package
// desaplus cuma bicara HTTP+JSON, jadi cocok unit test cepat.

func TestVillageSummary_Success_WithLastActivity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "secret-token" {
			t.Errorf("x-api-key = %q, want %q", got, "secret-token")
		}
		if got := r.URL.Query().Get("codeVanguard"); got != "DESA-001" {
			t.Errorf("codeVanguard = %q, want %q", got, "DESA-001")
		}
		if r.URL.Path != "/village-summary" {
			t.Errorf("path = %q, want /village-summary", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"success": true,
			"message": "Berhasil mendapatkan data",
			"data": {
				"village": {"id": "v1", "name": "Desa Contoh", "codeVanguard": "DESA-001"},
				"lastActivity": {
					"action": "CREATE", "feature": "surat", "desc": "Buat surat",
					"user": "Budi", "timestamp": "21 Sep 2026, 15.05", "since": "2 jam lalu"
				},
				"activeUsers": 7,
				"topFeatures": [{"feature": "presensi", "count": 12}, {"feature": "surat", "count": 8}],
				"todayActivityCount": 3
			}
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret-token")
	sum, err := c.VillageSummary(context.Background(), "DESA-001")
	if err != nil {
		t.Fatalf("VillageSummary() error = %v", err)
	}
	if sum.ActiveUsers != 7 {
		t.Errorf("ActiveUsers = %d, want 7", sum.ActiveUsers)
	}
	if sum.LastActivity == nil {
		t.Fatal("LastActivity = nil, want non-nil")
	}
	y, m, d, ok := sum.LastActivity.Date()
	if !ok || y != 2026 || m != time.September || d != 21 {
		t.Errorf("LastActivity.Date() = %d/%d/%d ok=%v, want 2026/September/21 ok=true", y, m, d, ok)
	}
	if len(sum.TopFeatures) != 2 || sum.TopFeatures[0].Feature != "presensi" {
		t.Errorf("TopFeatures = %+v", sum.TopFeatures)
	}
}

func TestVillageSummary_Success_NullLastActivity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"success": true,
			"message": "Berhasil mendapatkan data",
			"data": {
				"village": {"id": "v1", "name": "Desa Baru", "codeVanguard": "DESA-002"},
				"lastActivity": null,
				"activeUsers": 0,
				"topFeatures": [],
				"todayActivityCount": 0
			}
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret-token")
	sum, err := c.VillageSummary(context.Background(), "DESA-002")
	if err != nil {
		t.Fatalf("VillageSummary() error = %v", err)
	}
	if sum.LastActivity != nil {
		t.Errorf("LastActivity = %+v, want nil", sum.LastActivity)
	}
	if _, _, _, ok := sum.LastActivity.Date(); ok {
		t.Error("nil LastActivity.Date() ok = true, want false")
	}
}

func TestVillageSummary_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"success": false, "message": "Desa tidak ditemukan", "data": null}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret-token")
	_, err := c.VillageSummary(context.Background(), "DESA-GHOST")
	if !errors.Is(err, ErrVillageNotFound) {
		t.Errorf("err = %v, want ErrVillageNotFound", err)
	}
}

func TestVillageSummary_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"success": false, "message": "Unauthorized"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "wrong-token")
	_, err := c.VillageSummary(context.Background(), "DESA-001")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want ErrUnauthorized", err)
	}
}

func TestVillageSummary_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"success": false, "message": "Terjadi kesalahan pada server", "data": null}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret-token")
	_, err := c.VillageSummary(context.Background(), "DESA-001")
	if err == nil {
		t.Fatal("err = nil, want error")
	}
	if errors.Is(err, ErrVillageNotFound) || errors.Is(err, ErrUnauthorized) {
		t.Errorf("err = %v, want generic error (not NotFound/Unauthorized)", err)
	}
}

func TestLastActivity_Date_UnknownFormat(t *testing.T) {
	la := &LastActivity{Timestamp: "not-a-date"}
	if _, _, _, ok := la.Date(); ok {
		t.Error("Date() ok = true for garbage timestamp, want false")
	}
}
