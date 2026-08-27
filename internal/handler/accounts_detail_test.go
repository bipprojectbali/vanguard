package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// accounts_detail_test.go — kartu ringkasan lintas-modul di detail Account
// (M2-7..M2-9): Ringkasan Langganan, Ringkasan Customer Success, Sistem
// (audit), dan baris "Terkait". F3 ownership & F4 phone/budget dasar sudah
// diuji di accounts_fls_test.go; file ini fokus pada builder rollup baru
// (accounts_rollups.go) — semua BEST-EFFORT, tak pernah 500 walau modul lain
// kosong/rusak.

// setAccountParent menaruh parent_account_id lewat SQL mentah. sqlc
// CreateAccountParams/UpdateAccountParams TAK memiliki field ini — gap di
// luar cakupan M2-7 (yang hanya diminta MENAMPILKAN kolom lama, bukan
// membuka jalur TULIS baru); dipakai HANYA di test.
func (e *testEnv) setAccountParent(t *testing.T, childID, parentID int64) {
	t.Helper()
	if _, err := e.h.Pool.Exec(t.Context(),
		`UPDATE accounts SET parent_account_id=$1 WHERE id=$2`, parentID, childID); err != nil {
		t.Fatalf("set parent_account_id: %v", err)
	}
}

// setSubscriptionEndDate: seedSubscription (subscriptions_test.go) tak
// menerima end_date — dipakai di sini utk mengisi baris "Berakhir/Perpanjangan".
func (e *testEnv) setSubscriptionEndDate(t *testing.T, subID int64, end time.Time) {
	t.Helper()
	if _, err := e.h.Pool.Exec(t.Context(),
		`UPDATE subscriptions SET end_date=$1 WHERE id=$2`, end, subID); err != nil {
		t.Fatalf("set end_date: %v", err)
	}
}

// TestAccountDetail_RollupCardsRender: satu baris tiap modul terkait → semua
// kartu ringkasan + baris "Terkait" merender datanya (Admin, all-scope).
func TestAccountDetail_RollupCardsRender(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Rollup", &uid, &uid, nil)
	planID := env.seedPlan(t, "Paket Inti", "PLAN-ROLLUP", "1000000")
	sub := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "1000000", "12000000")
	env.setSubscriptionEndDate(t, sub.ID, time.Now().AddDate(0, 0, 30))
	env.seedCustomerSuccess(t, acc.ID)
	env.seedEngagementRow(t, acc.ID, "Check-in Rollup")
	env.seedDeal(t, acc.ID, &uid)
	env.seedContact(t, acc.ID, "Kontak Rollup", &uid, true)
	env.seedTicketRowWithDeadline(t, acc.ID, "Tiket Breach",
		pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true})

	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(acc.ID), nil, itoa(acc.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Paket Inti", // Ringkasan Langganan: nama paket via JOIN
		"Active",     // subStatusBadge merender status mentah
		"Sehat",      // Ringkasan Customer Success: healthScoreStatus("Healthy")
		"Onboarding", // tahap siklus hidup
		"test@local", // CSM/pemilik akun/audit — satu-satunya anggota di testEnv
		"1 deal",     // chip Deal
		"1 kontak",   // chip Kontak
		"breach",     // chip Tiket (breached_count > 0)
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body harus memuat %q, tak ditemukan", want)
		}
	}
}

// TestAccountDetail_EmptyRollups: desa tanpa langganan/CS/engagement/deal/
// kontak/tiket tetap 200 dengan teks empty-state — tak boleh panik di cabang
// pgx.ErrNoRows manapun.
func TestAccountDetail_EmptyRollups(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Kosong", &uid, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(acc.ID), nil, itoa(acc.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (rollup kosong harus best-effort, bukan 500)\n%s",
			rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Belum ada langganan", "Belum ada kontak", "Belum ada deal"} {
		if !strings.Contains(body, want) {
			t.Errorf("body harus memuat %q (empty-state), tak ditemukan", want)
		}
	}
}

// TestAccountDetail_ParentAccountBestEffort: induk yang sudah di-soft-delete
// (tetap ada baris-nya krn FK, hanya tersembunyi filter deleted_at) harus
// menghasilkan label+href KOSONG — bukan tautan ke id yang tak lagi valid,
// dan bukan 500.
func TestAccountDetail_ParentAccountBestEffort(t *testing.T) {
	env, uid := setupAccounts(t)
	parent := env.seedAccount(t, "Desa Induk Terhapus", &uid, nil, nil)
	acc := env.seedAccount(t, "Desa Anak", &uid, nil, nil)
	env.setAccountParent(t, acc.ID, parent.ID)
	if err := env.q.SoftDeleteAccount(t.Context(), db.SoftDeleteAccountParams{
		UpdatedBy: &uid, ID: parent.ID,
	}); err != nil {
		t.Fatalf("soft-delete induk: %v", err)
	}
	fresh, err := env.q.GetAccount(t.Context(), acc.ID)
	if err != nil {
		t.Fatalf("ambil ulang anak: %v", err)
	}

	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(acc.ID), nil, itoa(acc.ID))
	var view panel.AccountDetailView
	rec := env.runAccount(uid, "owner", "admin", req, func(w http.ResponseWriter, r *http.Request) {
		view = env.h.accountDetailView(r.Context(), "", fresh)
		w.WriteHeader(http.StatusOK)
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if view.ParentAccountLabel != "" || view.ParentAccountHref != "" {
		t.Errorf("induk terhapus harus blank (best-effort), got label=%q href=%q",
			view.ParentAccountLabel, view.ParentAccountHref)
	}
}

// TestAccountDetail_F3_StillEnforcedWithRollups: query rollup baru (deal,
// dsb.) tak boleh membuat F3 bocor — desa di luar cakupan Support tetap 404,
// gerbang ownership berjalan SEBELUM accountDetailView (dan builder rollup-nya)
// pernah dipanggil.
func TestAccountDetail_F3_StillEnforcedWithRollups(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other@local", "member", 0).ID
	acc := env.seedAccount(t, "Desa Bukan Milik Support", &other, nil, nil)
	env.seedDeal(t, acc.ID, &other)

	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(acc.ID), nil, itoa(acc.ID))
	rec := env.runAccount(uid, "owner", "support", req, env.h.AccountDetail)
	if rec.Code != http.StatusNotFound {
		t.Errorf("support di luar cakupan harus 404 (F3 tetap berjalan), got %d", rec.Code)
	}
}
