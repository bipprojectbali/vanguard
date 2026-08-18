package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_support_test.go — Support Report (Modul 8, wireframe 8.3) di sisi
// handler. Tiga sumbu dijaga:
//
//   - F2 (gerbang): tanpa izin crm:reports read → 403 + penjelasan.
//   - Agregasi: breakdown per status tiket, breach count konsisten dengan
//     CountTicketKPIs.BreachedCount; resolved_at diisi otomatis via
//     UpdateTicketStatus (status → 'selesai') sehingga avg_resolution_hours
//     terisi.
//   - F3 (ownership): Support (data_scope='none' + canWrite) mendapat override
//     ScopeAll — melihat SEMUA tiket walau bukan desa binaannya, PERSIS
//     TicketsListFilterFor (dibuktikan lewat kartu Tiket Terbuka). CSM
//     (own-scope) hanya menghitung tiket desa binaannya.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS di
// rls_test.go. Setup/helper reuse accounts_test.go & tickets_test.go
// (seedTicketRow, seedTicketRowWithDeadline).

// reportsSupportBody menjalankan ReportsSupport sebagai (uid, tenantRole,
// businessRole) & mengembalikan (status, body).
func (e *testEnv) reportsSupportBody(t *testing.T, uid int64, tenantRole, businessRole string) (int, string) {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/reports/support", nil, "")
	rec := e.runAccount(uid, tenantRole, businessRole, req, e.h.ReportsSupport)
	return rec.Code, rec.Body.String()
}

// --- F2: gerbang -------------------------------------------------------

// TestReportsSupport_GateRead: anggota tanpa business_role → 403 + penjelasan.
func TestReportsSupport_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)

	code, body := env.reportsSupportBody(t, uid, "member", "")
	if code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
	if !strings.Contains(body, "Support Report") {
		t.Errorf("body 403 harus menyebut judul halaman, got:\n%s", body)
	}
}

// TestReportsSupport_GateRead_AllowedRoles: support/manager/admin (pemegang
// crm:reports read) → 200.
func TestReportsSupport_GateRead_AllowedRoles(t *testing.T) {
	env, uid := setupAccounts(t)
	for _, role := range []string{"support", "manager", "admin"} {
		t.Run("role="+role, func(t *testing.T) {
			code, _ := env.reportsSupportBody(t, uid, "owner", role)
			if code != http.StatusOK {
				t.Errorf("role %q: status = %d, want 200", role, code)
			}
		})
	}
}

// --- agregasi ------------------------------------------------------------

// TestReportsSupport_BreakdownLabels: tabel breakdown memuat label Indonesia
// per status (Baru/Selesai) dan totalnya cocok dengan kartu KPI Tiket Terbuka.
func TestReportsSupport_BreakdownLabels(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Breakdown", &uid, nil, nil)
	tkA := env.seedTicketRow(t, acc.ID, "Tiket Baru A")
	tkB := env.seedTicketRow(t, acc.ID, "Tiket Baru B")
	_, err := env.q.UpdateTicketStatus(t.Context(), db.UpdateTicketStatusParams{
		Status: "selesai",
		ID:     tkB.ID,
	})
	if err != nil {
		t.Fatalf("selesaikan tiket B: %v", err)
	}
	_ = tkA

	_, body := env.reportsSupportBody(t, uid, "owner", "admin")
	for _, label := range []string{"Baru", "Selesai"} {
		if !strings.Contains(body, ">"+label+"<") {
			t.Errorf("tabel harus memuat status %q, body:\n%s", label, body)
		}
	}
	if got := dashboardKPIValue(t, body, "Tiket Terbuka"); got != "1" {
		t.Errorf("Tiket Terbuka = %q, want \"1\" (satu masih 'baru')", got)
	}
}

// TestReportsSupport_BreachedMatchesKPI: breakdown SLA Terlanggar (kolom
// tabel) SAMA dengan CountTicketKPIs.BreachedCount — satu sumber angka.
func TestReportsSupport_BreachedMatchesKPI(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Breach", &uid, nil, nil)
	past := pgtype.Timestamptz{Time: time.Now().Add(-2 * time.Hour), Valid: true}
	env.seedTicketRowWithDeadline(t, acc.ID, "Tiket Terlambat", past)
	future := pgtype.Timestamptz{Time: time.Now().Add(2 * time.Hour), Valid: true}
	env.seedTicketRowWithDeadline(t, acc.ID, "Tiket Aman", future)

	kpis, err := env.q.CountTicketKPIs(t.Context(), db.CountTicketKPIsParams{
		ScopeAll: true,
		Uid:      &uid,
	})
	if err != nil {
		t.Fatalf("CountTicketKPIs: %v", err)
	}
	if kpis.BreachedCount != 1 {
		t.Fatalf("KPI breached_count = %d, want 1", kpis.BreachedCount)
	}

	_, body := env.reportsSupportBody(t, uid, "owner", "admin")
	if got := dashboardKPIValue(t, body, "SLA Terlanggar"); got != "1" {
		t.Errorf("kartu SLA Terlanggar = %q, want \"1\" (konsisten dgn CountTicketKPIs)", got)
	}
}

// --- F3: ownership ---------------------------------------------------------

// TestReportsSupport_ScopedByOwnership: Support (data_scope='none' + canWrite)
// mendapat override ScopeAll — kartu Tiket Terbuka menghitung SEMUA tiket
// workspace (2), bukan cuma desa binaannya. CSM (own-scope) hanya menghitung
// tiket desa di mana uid = account_owner/assigned_csm/backup_csm (1 — desa
// "Milikku", bukan desa "Orang" milik user lain).
func TestReportsSupport_ScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "sup-report@local", "member", 0).ID

	accMine := env.seedAccount(t, "Desa Milikku Support", &uid, nil, nil)
	env.seedTicketRow(t, accMine.ID, "Tiket Milikku")

	accOther := env.seedAccount(t, "Desa Orang Support", nil, &other, nil)
	env.seedTicketRow(t, accOther.ID, "Tiket Orang")

	const label = "Tiket Terbuka"
	_, supportBody := env.reportsSupportBody(t, uid, "owner", "support")
	if got := dashboardKPIValue(t, supportBody, label); got != "2" {
		t.Errorf("support ScopeAll: Tiket Terbuka = %q, want \"2\" (lintas-desa)", got)
	}

	_, csmBody := env.reportsSupportBody(t, uid, "member", "csm")
	if got := dashboardKPIValue(t, csmBody, label); got != "1" {
		t.Errorf("csm own-scope: Tiket Terbuka = %q, want \"1\" (hanya desa Milikku Support, uid = account_owner)", got)
	}
}

// --- Export CSV --------------------------------------------------------------

// TestReportsSupport_Export: status 200, Content-Type text/csv, baris CSV
// memuat status yang diseed.
func TestReportsSupport_Export(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Export Support", &uid, nil, nil)
	env.seedTicketRow(t, acc.ID, "Tiket Export")

	req := accountsReq(http.MethodGet, "/w/test/reports/support/export", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.ReportsSupportExport)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv prefix", ct)
	}
	if !strings.Contains(rec.Body.String(), "Baru") {
		t.Errorf("CSV harus memuat status Baru, got:\n%s", rec.Body.String())
	}
}
