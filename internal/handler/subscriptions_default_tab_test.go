package handler

import (
	"net/http"
	"strings"
	"testing"
)

// subscriptions_default_tab_test.go — regresi BL-20: menu "Subscription Lists"
// mendarat pada langganan Active secara default (bukan SEMUA status), namun tab
// "Semua" tetap terjangkau lewat penanda eksplisit ?status=all. Tiga sifat yang
// WAJIB benar bersama di sisi handler:
//   - LANDING MURNI (tanpa ?status=) → default menyaring status=Active; baris
//     Cancelled/Expired/Churned dst tak ikut tampil di menu bernama "Active".
//   - ?status=all → penanda "Semua" eksplisit, DIBEDAKAN dari param absen →
//     handler tak menyaring status (lintas-status tampil).
//   - ?status=<legal> → menyaring status itu saja.
// Default hanya menyaring status DI ATAS cakupan F3/RLS (tak melebarkan) —
// dijaga oleh test F3 terpisah; di sini seed satu pemilik agar fokus ke status.

// subsDefaultTabEnv menyeed dua langganan pemilik sama, status berbeda, di
// account+plan berbeda (idx_subs_one_active hanya mengikat baris Active).
func subsDefaultTabEnv(t *testing.T) (*testEnv, int64) {
	t.Helper()
	env, uid := setupAccounts(t)
	accAktif := env.seedAccount(t, "Desa Aktif", &uid, nil, nil)
	planAktif := env.seedPlanRow(t, "Paket Aktif", "PKT-AKT", "Core")
	env.seedSubscription(t, accAktif.ID, planAktif.ID, &uid, "Active", "100000", "1200000")

	accBatal := env.seedAccount(t, "Desa Batal", &uid, nil, nil)
	planBatal := env.seedPlanRow(t, "Paket Batal", "PKT-BTL", "Add-on")
	env.seedSubscription(t, accBatal.ID, planBatal.ID, &uid, "Cancelled", "200000", "2400000")
	return env, uid
}

func TestSubscriptionsList_DefaultTabActive(t *testing.T) {
	env, uid := subsDefaultTabEnv(t)

	// Landing murni: tanpa ?status= → default Active.
	req := accountsReq(http.MethodGet, "/w/test/subscriptions", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Aktif") {
		t.Error("landing default (Active) harus memuat langganan Active")
	}
	if strings.Contains(body, "Desa Batal") {
		t.Error("landing default TAK boleh memuat langganan Cancelled (menu mendarat di Active)")
	}
}

func TestSubscriptionsList_AllTabCrossStatus(t *testing.T) {
	env, uid := subsDefaultTabEnv(t)

	// ?status=all → penanda Semua eksplisit → tak menyaring status.
	req := accountsReq(http.MethodGet, "/w/test/subscriptions?status=all", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Aktif") || !strings.Contains(body, "Desa Batal") {
		t.Error("tab Semua (?status=all) harus memuat langganan lintas-status")
	}
}

func TestSubscriptionsList_StatusTabFilters(t *testing.T) {
	env, uid := subsDefaultTabEnv(t)

	// ?status=Cancelled → hanya baris Cancelled.
	req := accountsReq(http.MethodGet, "/w/test/subscriptions?status=Cancelled", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Batal") {
		t.Error("?status=Cancelled harus memuat langganan Cancelled")
	}
	if strings.Contains(body, "Desa Aktif") {
		t.Error("?status=Cancelled TAK boleh memuat langganan Active")
	}
}
