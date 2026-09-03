package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// accounts_timeline_test.go — linimasa TERPADU detail Account (BL-31): gabungan
// read-only activities (Sales) + engagements (CS). Yang dijaga:
//   - Union: kartu memuat baris DARI KEDUA sumber dengan penanda benar.
//   - F2 fail-closed: peran tanpa crm:engagements (sales) TAK melihat baris CS
//     walau boleh membuka desanya (tak ada kebocoran lintas-modul).
//   - Urutan waktu turun konsisten lintas-sumber.
//
// F3 ownership & 404 desa luar-cakupan sudah diuji accounts_fls_test.go /
// accounts_detail_test.go; di sini fokus pada builder gabungan baru.

// seedAccountActivity menaruh satu activity (context=sales) yang menargetkan
// desa TERTENTU — beda dari seedAllActivity yang membuat desanya sendiri.
func (e *testEnv) seedAccountActivity(
	t *testing.T, accountID int64, kind, subject string, owner *int64,
) db.Activity {
	t.Helper()
	a, err := e.q.CreateActivity(t.Context(), db.CreateActivityParams{
		TenantID:        e.tenantID,
		Kind:            kind,
		Subject:         subject,
		TargetType:      "account",
		TargetID:        accountID,
		OwnerID:         owner,
		ActivityContext: ptr("sales"),
		CreatedBy:       owner,
	})
	if err != nil {
		t.Fatalf("seed account activity %q: %v", subject, err)
	}
	return a
}

// accountDetailBody menjalankan AccountDetail → HTML shell, gagal bila bukan 200.
func (e *testEnv) accountDetailBody(t *testing.T, uid, accID int64, wsRole, bizRole string) string {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(accID), nil, itoa(accID))
	rec := e.runAccount(uid, wsRole, bizRole, req, e.h.AccountDetail)
	if rec.Code != http.StatusOK {
		t.Fatalf("AccountDetail status %d\n%s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// TestAccountTimeline_UnifiedShowsBothSources: Admin (crm:*, all-scope) membuka
// desa dengan satu activity Sales + satu engagement CS → keduanya muncul di
// kartu linimasa, dengan chip sumber (Sales/CS) dan label jenis engagement.
func TestAccountTimeline_UnifiedShowsBothSources(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Linimasa", &uid, &uid, nil)

	env.seedAccountActivity(t, acc.ID, "note", "Aktivitas-Sales-Uji", &uid)
	env.seedEngagementRow(t, acc.ID, "Engagement-CS-Uji") // type=touch_point

	body := env.accountDetailBody(t, uid, acc.ID, "owner", "admin")

	// Kedua sumber hadir (sinyal union utama).
	for _, want := range []string{
		"Aktivitas-Sales-Uji", // baris activity
		"Engagement-CS-Uji",   // baris engagement
		"Touch Point",         // label jenis engagement → baris melewati proyeksi CS
	} {
		if !strings.Contains(body, want) {
			t.Errorf("linimasa terpadu harus memuat %q, tak ditemukan", want)
		}
	}
	// Chip sumber CS hadir (badge-secondary outline dari timelineSourceChip).
	if !strings.Contains(body, "badge-secondary badge-outline") {
		t.Errorf("chip sumber CS (badge-secondary badge-outline) harus dirender")
	}
}

// TestAccountTimeline_EngagementsHiddenWithoutPerm: peran sales (punya
// crm:accounts + crm:sales_activity, TAPI bukan crm:engagements) membuka desa
// miliknya → baris activity tampil, baris engagement TIDAK (gated union
// fail-closed dari sisi CS). Membuktikan tak ada kebocoran data CS ke Sales.
func TestAccountTimeline_EngagementsHiddenWithoutPerm(t *testing.T) {
	env, uid := setupAccounts(t)
	// owner=uid → data_scope='own' (sales) meloloskan F3 buka desa.
	acc := env.seedAccount(t, "Desa Sales", &uid, nil, nil)

	env.seedAccountActivity(t, acc.ID, "note", "Aktivitas-Sales-Lihat", &uid)
	env.seedEngagementRow(t, acc.ID, "Engagement-CS-Rahasia")

	body := env.accountDetailBody(t, uid, acc.ID, "member", "sales")

	if !strings.Contains(body, "Aktivitas-Sales-Lihat") {
		t.Errorf("sales harus melihat baris activity desanya")
	}
	if strings.Contains(body, "Engagement-CS-Rahasia") {
		t.Errorf("sales TAK boleh melihat baris engagement (F2 crm:engagements bocor)")
	}
	// Tak ada chip CS sama sekali (sumber CS tak pernah dimuat).
	if strings.Contains(body, "badge-secondary badge-outline") {
		t.Errorf("chip sumber CS tak boleh muncul untuk sales (sumber CS ter-gate)")
	}
}

// TestAccountTimeline_OrderingAcrossSources: activity baru (created_at ~ now)
// harus tampil DI ATAS engagement lama (scheduled_at 15 Jun 2026) — urutan
// waktu turun konsisten lintas dua tabel dengan kunci waktu berbeda.
func TestAccountTimeline_OrderingAcrossSources(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Urutan", &uid, &uid, nil)

	// Engagement scheduled_at = 15 Jun 2026 (seedEngagementRow); activity
	// created_at default now() (jauh lebih baru) → activity mendahului.
	env.seedEngagementRow(t, acc.ID, "Engagement-Lama-Juni")
	env.seedAccountActivity(t, acc.ID, "note", "Aktivitas-Baru-Kini", &uid)

	body := env.accountDetailBody(t, uid, acc.ID, "owner", "admin")

	iAct := strings.Index(body, "Aktivitas-Baru-Kini")
	iEng := strings.Index(body, "Engagement-Lama-Juni")
	if iAct < 0 || iEng < 0 {
		t.Fatalf("kedua baris harus hadir (act=%d eng=%d)", iAct, iEng)
	}
	if iAct > iEng {
		t.Errorf("activity terbaru harus tampil sebelum engagement lama (act=%d, eng=%d)", iAct, iEng)
	}
}
