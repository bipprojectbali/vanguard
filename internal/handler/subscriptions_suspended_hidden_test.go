package handler

import (
	"net/http"
	"strings"
	"testing"
)

// subscriptions_suspended_hidden_test.go — regresi BL-22: status "Suspended"
// dicopot dari PERMUKAAN langganan (dropdown filter + seed), TAPI nilai enum DB
// SENGAJA dibiarkan sebagai cadangan. Tiga kontrak yang wajib benar bersama:
//   - subscriptionStatuses (opsi dropdown) TAK lagi memuat "Suspended", tapi
//     tetap memuat status yang bisa dicapai app.
//   - Daftar /subscriptions tak merender tab filter ?status=Suspended.
//   - CHECK subs_status_chk TETAP menerima "Suspended" (pintu masa depan;
//     membuangnya butuh migrasi + backfill — di luar scope BL-22).

// TestSubscriptionStatuses_NoSuspended mengunci isi slice opsi filter (unit,
// tanpa DB). Bila kelak ada aksi Tangguhkan/Unsuspend dan "Suspended" pantas
// kembali, test ini yang pertama mengingatkan untuk memperbaruinya bersama.
func TestSubscriptionStatuses_NoSuspended(t *testing.T) {
	for _, s := range subscriptionStatuses {
		if s == "Suspended" {
			t.Fatalf("subscriptionStatuses masih menawarkan %q — BL-22 mencopotnya dari dropdown filter", s)
		}
	}
	// Kontrol: status yang BISA dicapai app tetap ditawarkan (bukan slice kosong).
	want := []string{"Trial", "Active", "PendingApproval", "Expired", "Cancelled", "Churned"}
	if len(subscriptionStatuses) != len(want) {
		t.Fatalf("subscriptionStatuses = %v, ingin %v", subscriptionStatuses, want)
	}
	for i, s := range want {
		if subscriptionStatuses[i] != s {
			t.Fatalf("subscriptionStatuses[%d] = %q, ingin %q", i, subscriptionStatuses[i], s)
		}
	}
}

// TestSubscriptionsList_FilterOmitsSuspended merender daftar dan memastikan tab
// filter status TAK menawarkan ?status=Suspended (href unik ke tab; badge baris
// tak menghasilkan pola "status=Suspended"). Tab ?status=Active hadir sebagai
// kontrol bahwa baris tab memang dirender.
func TestSubscriptionsList_FilterOmitsSuspended(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Filter", &uid, nil, nil)
	plan := env.seedPlanRow(t, "Paket Filter", "PKT-FLT", "Core")
	env.seedSubscription(t, acc.ID, plan.ID, &uid, "Active", "100000", "1200000")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions?status=all", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SubscriptionsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "status=Suspended") {
		t.Error("daftar langganan masih merender tab filter ?status=Suspended — BL-22 mencopotnya")
	}
	if !strings.Contains(body, "status=Active") {
		t.Error("tab filter ?status=Active tak dirender — kontrol gagal, assertion Suspended tak bermakna")
	}
}

// TestSubscriptions_EnumStillAcceptsSuspended membuktikan CHECK subs_status_chk
// TETAP menerima "Suspended": bila constraint menolaknya, seedSubscription
// (CreateSubscription) akan t.Fatalf. Mencapai baris setelahnya = enum utuh.
func TestSubscriptions_EnumStillAcceptsSuspended(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Reserved", &uid, nil, nil)
	plan := env.seedPlanRow(t, "Paket Reserved", "PKT-RSV", "Core")

	sub := env.seedSubscription(t, acc.ID, plan.ID, &uid, "Suspended", "100000", "1200000")
	if sub.Status != "Suspended" {
		t.Fatalf("status tersimpan = %q, ingin %q (enum harus tetap menerima nilai cadangan)", sub.Status, "Suspended")
	}
}
