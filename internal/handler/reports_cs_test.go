package handler

import (
	"net/http"
	"strings"
	"testing"
)

// reports_cs_test.go — Customer Success Report (Modul 8, wireframe 8.2) di
// sisi handler. Tiga sumbu dijaga:
//
//   - F2 (gerbang): tanpa izin crm:reports read → 403 + penjelasan (pola sama
//     dgn reports_sales_test.go).
//   - Agregasi: breakdown per health_status (Sehat/Berisiko/Kritis/Belum
//     Dinilai) konsisten dengan CountHealthScoreKPIs.Total.
//   - F3 (ownership): CSM (data_scope='own') hanya melihat breakdown desa
//     binaannya; Admin (data_scope='all') lintas-CSM. Scope 8.2 di sini HANYA
//     Health/Adoption — NPS/CSAT ditunda (lihat doc comment reports_cs.go).
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS di
// rls_test.go. Setup/helper reuse accounts_test.go & health_score_test.go
// (seedHealthScore).

// reportsCSBody menjalankan ReportsCS sebagai (uid, tenantRole, businessRole)
// & mengembalikan (status, body).
func (e *testEnv) reportsCSBody(t *testing.T, uid int64, tenantRole, businessRole string) (int, string) {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/reports/customer-success", nil, "")
	rec := e.runAccount(uid, tenantRole, businessRole, req, e.h.ReportsCS)
	return rec.Code, rec.Body.String()
}

// --- F2: gerbang -------------------------------------------------------

// TestReportsCS_GateRead: anggota tanpa business_role → 403 + penjelasan.
func TestReportsCS_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)

	code, body := env.reportsCSBody(t, uid, "member", "")
	if code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
	if !strings.Contains(body, "Customer Success Report") {
		t.Errorf("body 403 harus menyebut judul halaman, got:\n%s", body)
	}
}

// TestReportsCS_GateRead_AllowedRoles: csm/manager/admin (pemegang
// crm:reports read) → 200.
func TestReportsCS_GateRead_AllowedRoles(t *testing.T) {
	env, uid := setupAccounts(t)
	for _, role := range []string{"csm", "manager", "admin"} {
		t.Run("role="+role, func(t *testing.T) {
			code, _ := env.reportsCSBody(t, uid, "owner", role)
			if code != http.StatusOK {
				t.Errorf("role %q: status = %d, want 200", role, code)
			}
		})
	}
}

// --- agregasi ------------------------------------------------------------

// TestReportsCS_BreakdownLabels: tabel breakdown memuat label Indonesia per
// status (Sehat/Berisiko/Kritis) dan totalnya cocok dengan kartu KPI Desa
// Total.
func TestReportsCS_BreakdownLabels(t *testing.T) {
	env, uid := setupAccounts(t)
	a1 := env.seedAccount(t, "Desa Sehat", &uid, nil, nil)
	env.seedHealthScore(t, a1.ID, 90, "Healthy")
	a2 := env.seedAccount(t, "Desa Berisiko", &uid, nil, nil)
	env.seedHealthScore(t, a2.ID, 60, "At-Risk")
	a3 := env.seedAccount(t, "Desa Kritis", &uid, nil, nil)
	env.seedHealthScore(t, a3.ID, 20, "Critical")

	_, body := env.reportsCSBody(t, uid, "owner", "admin")
	for _, label := range []string{"Sehat", "Berisiko", "Kritis"} {
		if !strings.Contains(body, ">"+label+"<") {
			t.Errorf("tabel harus memuat status %q, body:\n%s", label, body)
		}
	}
	if got := dashboardKPIValue(t, body, "Desa Total"); got != "3" {
		t.Errorf("Desa Total = %q, want \"3\"", got)
	}
}

// --- F3: ownership ---------------------------------------------------------

// TestReportsCS_ScopedByOwnership: CSM (own-scope) hanya menghitung desa
// binaannya di kartu Desa Total; Admin (all-scope) lintas-CSM.
func TestReportsCS_ScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-cs-report@local", "member", 0).ID

	own := env.seedAccount(t, "Desa Binaanku Report", nil, &uid, nil)
	env.seedHealthScore(t, own.ID, 85, "Healthy")

	otherAcc := env.seedAccount(t, "Desa Binaan Lain Report", nil, &other, nil)
	env.seedHealthScore(t, otherAcc.ID, 40, "Critical")

	const label = "Desa Total"
	_, ownBody := env.reportsCSBody(t, uid, "member", "csm")
	if got := dashboardKPIValue(t, ownBody, label); got != "1" {
		t.Errorf("csm own-scope: Desa Total = %q, want \"1\" (hanya binaannya)", got)
	}

	_, allBody := env.reportsCSBody(t, uid, "owner", "admin")
	if got := dashboardKPIValue(t, allBody, label); got != "2" {
		t.Errorf("admin all-scope: Desa Total = %q, want \"2\" (lintas-CSM)", got)
	}
}

// --- Export CSV --------------------------------------------------------------

// TestReportsCS_Export: status 200, Content-Type text/csv, baris CSV memuat
// status yang diseed.
func TestReportsCS_Export(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Export CS", &uid, nil, nil)
	env.seedHealthScore(t, acc.ID, 95, "Healthy")

	req := accountsReq(http.MethodGet, "/w/test/reports/customer-success/export", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.ReportsCSExport)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv prefix", ct)
	}
	if !strings.Contains(rec.Body.String(), "Sehat") {
		t.Errorf("CSV harus memuat status Sehat, got:\n%s", rec.Body.String())
	}
}
