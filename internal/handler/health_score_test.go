package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// health_score_test.go — Health Score workspace-level listing (Modul 6 slice C1).
// Tiga sumbu yang dijaga:
//
//   - F2 (Casbin bisnis): GET /health-scores butuh crm:health read — semua peran
//     CRM (admin/manager/support/sales/csm) lolos; "" ditolak 403.
//   - F3 (ownership): CSM (data_scope='own') melihat HANYA desa binaannya;
//     Admin (data_scope='all') melihat semua.
//   - KPI sanity: CountHealthScoreKPIs mengembalikan angka non-negatif yang
//     konsisten dengan jumlah baris ListHealthScores.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS di
// rls_test.go. Helper seedAccount/setupAccounts/runAccount dari accounts_test.go.

// --- helper ----------------------------------------------------------------

// seedHealthScore menaruh satu baris customer_success untuk desa langsung lewat
// pool (bypass handler). score & status dioper eksplisit agar test KPI bisa
// memverifikasi bucket.
func (e *testEnv) seedHealthScore(t *testing.T, accountID int64, score int16, status string) {
	t.Helper()
	_, err := e.q.CreateCustomerSuccess(t.Context(), db.CreateCustomerSuccessParams{
		TenantID:           e.tenantID,
		AccountID:          accountID,
		OverallHealthScore: &score,
		HealthStatus:       &status,
	})
	if err != nil {
		t.Fatalf("seed health score for account %d: %v", accountID, err)
	}
}

// allHealthScoreRows mendaftar seluruh health score workspace (scope_all, tanpa
// filter) langsung dari pool — untuk membuktikan visibilitas ownership F3.
func (e *testEnv) allHealthScoreRows(t *testing.T) []db.ListHealthScoresRow {
	t.Helper()
	at, id := firstPageCursor()
	rows, err := e.q.ListHealthScores(t.Context(), db.ListHealthScoresParams{
		CursorCreatedAt: at,
		CursorID:        id,
		ScopeAll:        true,
		FilterStatus:    "", // interface{} nil → SQL NULL → NULL='' saring semua baris
		PageSize:        100,
	})
	if err != nil {
		t.Fatalf("list health scores: %v", err)
	}
	return rows
}

// --- F2: gerbang read --------------------------------------------------

// TestHealthScore_GateRead: siapa boleh MEMBUKA daftar health score (act read).
// Semua peran CRM lolos (admin/manager/support/sales/csm); "" ditolak 403.
func TestHealthScore_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	// Seed akun agar ListHealthScores tidak error walau list kosong.
	_ = env.seedAccount(t, "Desa Gate", &uid, nil, nil)

	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"support", true},
		{"sales", true},
		{"csm", true},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/health-scores", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.HealthScoreList)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate read, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if !strings.Contains(rec.Body.String(), "peran CRM") {
					t.Errorf("penolakan harus menyebut peran CRM, got %q", rec.Body.String())
				}
			}
		})
	}
}

// --- F3: ownership (health score) --------------------------------------

// TestHealthScore_OwnershipCSM: CSM melihat HANYA desa di mana ia menjadi
// assigned_csm atau backup_csm — desa yang dimiliki user lain tak tampil.
func TestHealthScore_OwnershipCSM(t *testing.T) {
	env, uid := setupAccounts(t)
	// other = member biasa dalam env yang SAMA — seedMember tak truncate DB.
	other := env.seedMember(t, "other-csm@local", "member", 0).ID

	// Desa A: uid sebagai assigned_csm → harus tampil ke uid sebagai CSM.
	own := env.seedAccount(t, "Desa Sendiri", nil, &uid, nil)
	env.seedHealthScore(t, own.ID, 85, "Healthy")

	// Desa B: other sebagai assigned_csm → TIDAK tampil ke uid sebagai CSM.
	otherAcc := env.seedAccount(t, "Desa Orang Lain", nil, &other, nil)
	env.seedHealthScore(t, otherAcc.ID, 30, "Critical")

	req := accountsReq(http.MethodGet, "/w/test/health-scores", nil, "")
	// Jalankan sebagai uid dengan business_role=csm (data_scope='own').
	rec := env.runAccount(uid, "member", "csm", req, env.h.HealthScoreList)

	if rec.Code != http.StatusOK {
		t.Fatalf("CSM harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Sendiri") {
		t.Errorf("CSM harus melihat desa binaannya (Desa Sendiri)")
	}
	if strings.Contains(body, "Desa Orang Lain") {
		t.Errorf("CSM tidak boleh melihat desa milik user lain (Desa Orang Lain)")
	}
}

// TestHealthScore_OwnershipAdmin: Admin (data_scope='all') melihat SEMUA desa
// dalam workspace, termasuk desa yang assigned_csm-nya user lain.
func TestHealthScore_OwnershipAdmin(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-admin@local", "member", 0).ID

	// Seed dua desa dengan pemilik berbeda.
	a1 := env.seedAccount(t, "Desa Admin A", &uid, nil, nil)
	env.seedHealthScore(t, a1.ID, 75, "At-Risk")

	a2 := env.seedAccount(t, "Desa Admin B", &other, nil, nil)
	env.seedHealthScore(t, a2.ID, 90, "Healthy")

	req := accountsReq(http.MethodGet, "/w/test/health-scores", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.HealthScoreList)

	if rec.Code != http.StatusOK {
		t.Fatalf("Admin harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Admin A") {
		t.Errorf("Admin harus melihat Desa Admin A")
	}
	if !strings.Contains(body, "Desa Admin B") {
		t.Errorf("Admin harus melihat Desa Admin B")
	}
}

// --- KPI sanity --------------------------------------------------------

// TestHealthScore_KPISanity: CountHealthScoreKPIs mengembalikan bucket yang
// konsisten dengan data seed — Healthy/AtRisk/Critical sesuai health_status
// masing-masing baris.
func TestHealthScore_KPISanity(t *testing.T) {
	env, uid := setupAccounts(t)

	// Seed 3 akun dengan health_status berbeda.
	a1 := env.seedAccount(t, "Desa Sehat", &uid, nil, nil)
	env.seedHealthScore(t, a1.ID, 85, "Healthy")

	a2 := env.seedAccount(t, "Desa Berisiko", &uid, nil, nil)
	env.seedHealthScore(t, a2.ID, 55, "At-Risk")

	a3 := env.seedAccount(t, "Desa Kritis", &uid, nil, nil)
	env.seedHealthScore(t, a3.ID, 20, "Critical")

	// Verifikasi langsung via DB (scope_all, admin view).
	kpis, err := env.q.CountHealthScoreKPIs(t.Context(), db.CountHealthScoreKPIsParams{
		ScopeAll: true,
	})
	if err != nil {
		t.Fatalf("CountHealthScoreKPIs: %v", err)
	}

	// Total harus ≥ 3 (mungkin ada akun lain dari setup truncate).
	if kpis.Total < 3 {
		t.Errorf("Total harus ≥ 3, got %d", kpis.Total)
	}
	if kpis.Healthy < 1 {
		t.Errorf("Healthy harus ≥ 1, got %d", kpis.Healthy)
	}
	if kpis.AtRisk < 1 {
		t.Errorf("AtRisk harus ≥ 1, got %d", kpis.AtRisk)
	}
	if kpis.Critical < 1 {
		t.Errorf("Critical harus ≥ 1, got %d", kpis.Critical)
	}
	// Sanity: jumlah bucket tak melebihi total.
	if kpis.Healthy+kpis.AtRisk+kpis.Critical > kpis.Total {
		t.Errorf("jumlah bucket (%d+%d+%d) melebihi total (%d)",
			kpis.Healthy, kpis.AtRisk, kpis.Critical, kpis.Total)
	}
}

// --- status filter --------------------------------------------------------

// TestHealthScore_StatusFilter: ?tab=sehat → hanya baris Healthy yang tampil di
// response HTML; baris Critical & At-Risk tidak.
func TestHealthScore_StatusFilter(t *testing.T) {
	env, uid := setupAccounts(t)

	a1 := env.seedAccount(t, "Desa Hijau", &uid, nil, nil)
	env.seedHealthScore(t, a1.ID, 82, "Healthy")

	a2 := env.seedAccount(t, "Desa Merah", &uid, nil, nil)
	env.seedHealthScore(t, a2.ID, 25, "Critical")

	req := accountsReq(http.MethodGet, "/w/test/health-scores?tab=sehat", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.HealthScoreList)

	if rec.Code != http.StatusOK {
		t.Fatalf("filter sehat harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Hijau") {
		t.Errorf("tab=sehat harus menampilkan desa Healthy (Desa Hijau)")
	}
	if strings.Contains(body, "Desa Merah") {
		t.Errorf("tab=sehat tidak boleh menampilkan desa Critical (Desa Merah)")
	}
}
