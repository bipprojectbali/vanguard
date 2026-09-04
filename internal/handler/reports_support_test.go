package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_support_test.go — Support Report (Modul 8, wireframe 8.3, BL-46) di
// sisi handler. Dijaga:
//
//   - F2 (gerbang): tanpa izin crm:reports read → 403 + penjelasan.
//   - KPI & panel agregat: Total Tiket, Kepatuhan SLA (met/with_sla),
//     Terpenuhi/Terlanggar/Berisiko, kinerja agen + badge Status.
//   - F3 (ownership): Support (data_scope='none' + canWrite) mendapat override
//     ScopeAll — melihat SEMUA tiket, PERSIS TicketsListFilterFor; CSM
//     (own-scope) hanya tiket desa binaannya.
//   - Logika murni (tanpa DB): backlog kumulatif & ambang badge agen.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS di
// rls_test.go. Setup/helper reuse accounts_test.go & tickets_test.go.

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

// --- KPI & agregasi ------------------------------------------------------

// TestReportsSupport_KPITotal: kartu Total Tiket menghitung semua tiket dalam
// cakupan.
func TestReportsSupport_KPITotal(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Total", &uid, nil, nil)
	env.seedTicketRow(t, acc.ID, "T1")
	env.seedTicketRow(t, acc.ID, "T2")
	env.seedTicketRow(t, acc.ID, "T3")

	_, body := env.reportsSupportBody(t, uid, "owner", "admin")
	if got := dashboardKPIValue(t, body, "Total Tiket"); got != "3" {
		t.Errorf("Total Tiket = %q, want \"3\"", got)
	}
}

// TestReportsSupport_SLACompliance: satu tiket terpenuhi (deadline masa depan,
// diselesaikan), satu terlanggar (deadline lampau, belum selesai), satu
// berisiko (deadline < 4 jam lagi, belum selesai). Kartu SLA panel 2 & KPI
// Kepatuhan SLA konsisten (with_sla=3, met=1).
func TestReportsSupport_SLACompliance(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa SLA", &uid, nil, nil)

	future := pgtype.Timestamptz{Time: time.Now().Add(48 * time.Hour), Valid: true}
	met := env.seedTicketRowWithDeadline(t, acc.ID, "Terpenuhi", future)
	if _, err := env.q.UpdateTicketStatus(t.Context(), db.UpdateTicketStatusParams{
		Status: "selesai", ID: met.ID,
	}); err != nil {
		t.Fatalf("selesaikan tiket met: %v", err)
	}
	past := pgtype.Timestamptz{Time: time.Now().Add(-2 * time.Hour), Valid: true}
	env.seedTicketRowWithDeadline(t, acc.ID, "Terlanggar", past)
	soon := pgtype.Timestamptz{Time: time.Now().Add(2 * time.Hour), Valid: true}
	env.seedTicketRowWithDeadline(t, acc.ID, "Berisiko", soon)

	_, body := env.reportsSupportBody(t, uid, "owner", "admin")
	if got := dashboardKPIValue(t, body, "Kepatuhan SLA"); got != "33,3%" {
		t.Errorf("Kepatuhan SLA = %q, want \"33,3%%\" (1 dari 3 ber-SLA terpenuhi)", got)
	}
	if got := dashboardKPIValue(t, body, "Terpenuhi"); got != "33,3%" {
		t.Errorf("kartu Terpenuhi = %q, want \"33,3%%\"", got)
	}
	if got := dashboardKPIValue(t, body, "Terlanggar"); got != "33,3%" {
		t.Errorf("kartu Terlanggar = %q, want \"33,3%%\"", got)
	}
	if got := dashboardKPIValue(t, body, "Berisiko"); got != "1" {
		t.Errorf("kartu Berisiko = %q, want \"1\"", got)
	}
}

// TestReportsSupport_SLATargetFromPolicy: Target SLA panel 2 diambil dari
// sla_policies via snapshot sla_policy_id; tanpa policy → "—".
func TestReportsSupport_SLATargetFromPolicy(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Policy", &uid, nil, nil)

	var policyID int64
	if err := env.h.Pool.QueryRow(t.Context(),
		`INSERT INTO sla_policies (tenant_id, sla_name, applies_to_priority, resolution_target_minutes)
		 VALUES ($1, 'Kebijakan Sedang', 'sedang', 1440) RETURNING id`,
		env.tenantID,
	).Scan(&policyID); err != nil {
		t.Fatalf("seed sla_policy: %v", err)
	}
	if _, err := env.q.CreateTicket(t.Context(), db.CreateTicketParams{
		TenantID:    env.tenantID,
		AccountID:   acc.ID,
		Subject:     "Tiket ber-policy",
		Priority:    "sedang",
		SlaPolicyID: &policyID,
	}); err != nil {
		t.Fatalf("seed ticket ber-policy: %v", err)
	}

	_, body := env.reportsSupportBody(t, uid, "owner", "admin")
	if !strings.Contains(body, "Sedang") {
		t.Errorf("panel SLA harus memuat prioritas Sedang, body:\n%s", body)
	}
	if !strings.Contains(body, "24 jam") {
		t.Errorf("Target SLA harus \"24 jam\" (dari resolution_target_minutes), body:\n%s", body)
	}
}

// TestReportsSupport_AgentPerformance: tiket ditugaskan ke agen tampil di panel
// 5 dengan nama & badge Status "Sangat baik" (kepatuhan SLA 100%).
func TestReportsSupport_AgentPerformance(t *testing.T) {
	env, uid := setupAccounts(t)
	agent := env.seedMember(t, "agen-support@local", "member", 0).ID
	acc := env.seedAccount(t, "Desa Agen", &uid, nil, nil)

	future := pgtype.Timestamptz{Time: time.Now().Add(48 * time.Hour), Valid: true}
	for _, subj := range []string{"Agen T1", "Agen T2"} {
		tk, err := env.q.CreateTicket(t.Context(), db.CreateTicketParams{
			TenantID:      env.tenantID,
			AccountID:     acc.ID,
			Subject:       subj,
			Priority:      "sedang",
			AssignedTo:    &agent,
			SlaDeadlineAt: future,
		})
		if err != nil {
			t.Fatalf("seed ticket agen %q: %v", subj, err)
		}
		if _, err := env.q.UpdateTicketStatus(t.Context(), db.UpdateTicketStatusParams{
			Status: "selesai", AssignedTo: &agent, ID: tk.ID,
		}); err != nil {
			t.Fatalf("selesaikan tiket agen: %v", err)
		}
	}

	_, body := env.reportsSupportBody(t, uid, "owner", "admin")
	if !strings.Contains(body, "agen-support@local") {
		t.Errorf("panel Agent Performance harus memuat agen, body:\n%s", body)
	}
	if !strings.Contains(body, "Sangat baik") {
		t.Errorf("badge Status agen harus \"Sangat baik\" (SLA 100%%), body:\n%s", body)
	}
}

// --- F3: ownership ---------------------------------------------------------

// TestReportsSupport_ScopedByOwnership: Support (data_scope='none' + canWrite)
// override ScopeAll — Total Tiket menghitung SEMUA tiket workspace (2); CSM
// (own-scope) hanya tiket desa binaannya (1).
func TestReportsSupport_ScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "sup-report@local", "member", 0).ID

	accMine := env.seedAccount(t, "Desa Milikku Support", &uid, nil, nil)
	env.seedTicketRow(t, accMine.ID, "Tiket Milikku")

	accOther := env.seedAccount(t, "Desa Orang Support", nil, &other, nil)
	env.seedTicketRow(t, accOther.ID, "Tiket Orang")

	const label = "Total Tiket"
	_, supportBody := env.reportsSupportBody(t, uid, "owner", "support")
	if got := dashboardKPIValue(t, supportBody, label); got != "2" {
		t.Errorf("support ScopeAll: Total Tiket = %q, want \"2\" (lintas-desa)", got)
	}

	_, csmBody := env.reportsSupportBody(t, uid, "member", "csm")
	if got := dashboardKPIValue(t, csmBody, label); got != "1" {
		t.Errorf("csm own-scope: Total Tiket = %q, want \"1\" (hanya desa Milikku, uid = account_owner)", got)
	}
}

// --- Export CSV --------------------------------------------------------------

// TestReportsSupport_Export: default panel (volume) → header Periode; panel=agent
// → header Agen + nama agen.
func TestReportsSupport_Export(t *testing.T) {
	env, uid := setupAccounts(t)
	agent := env.seedMember(t, "agen-export@local", "member", 0).ID
	acc := env.seedAccount(t, "Desa Export Support", &uid, nil, nil)
	if _, err := env.q.CreateTicket(t.Context(), db.CreateTicketParams{
		TenantID:   env.tenantID,
		AccountID:  acc.ID,
		Subject:    "Tiket Export",
		Priority:   "sedang",
		AssignedTo: &agent,
	}); err != nil {
		t.Fatalf("seed ticket export: %v", err)
	}

	t.Run("default volume", func(t *testing.T) {
		req := accountsReq(http.MethodGet, "/w/test/reports/support/export", nil, "")
		rec := env.runAccount(uid, "owner", "admin", req, env.h.ReportsSupportExport)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
			t.Errorf("Content-Type = %q, want text/csv prefix", ct)
		}
		if !strings.Contains(rec.Body.String(), "Periode") {
			t.Errorf("CSV volume harus memuat header Periode, got:\n%s", rec.Body.String())
		}
	})

	t.Run("panel=agent", func(t *testing.T) {
		req := accountsReq(http.MethodGet, "/w/test/reports/support/export?panel=agent", nil, "")
		rec := env.runAccount(uid, "owner", "admin", req, env.h.ReportsSupportExport)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Agen") || !strings.Contains(body, "agen-export@local") {
			t.Errorf("CSV agent harus memuat header & nama agen, got:\n%s", body)
		}
	})
}

// --- Logika murni (tanpa DB) -------------------------------------------------

// TestBuildVolumeRows_BacklogCumulative: backlog = masuk−selesai berjalan
// lintas bulan, dijaga ≥0.
func TestBuildVolumeRows_BacklogCumulative(t *testing.T) {
	rows := buildVolumeRows([]db.ReportTicketVolumeByMonthRow{
		{Period: "2026-01", Masuk: 5, Selesai: 2}, // backlog 3
		{Period: "2026-02", Masuk: 1, Selesai: 3}, // 3+1-3 = 1
		{Period: "2026-03", Masuk: 0, Selesai: 4}, // 1-4 = -3 → 0
	})
	want := []int64{3, 1, 0}
	if len(rows) != len(want) {
		t.Fatalf("len rows = %d, want %d", len(rows), len(want))
	}
	for i, w := range want {
		if rows[i].Backlog != w {
			t.Errorf("row %d Backlog = %d, want %d", i, rows[i].Backlog, w)
		}
	}
}

// TestAgentStatusBadge: ambang badge (const) menurunkan label yang benar.
func TestAgentStatusBadge(t *testing.T) {
	cases := []struct {
		name                            string
		withSLA, met, handled, resolved int64
		wantLabel                       string
	}{
		{"excellent", 10, 10, 10, 10, "Sangat baik"}, // 100% ≥90
		{"good-sla", 10, 8, 10, 8, "Baik"},           // 80% ≥70
		{"needs", 10, 5, 10, 5, "Perlu bimbingan"},   // 50% <70
		{"no-sla-good", 0, 0, 10, 9, "Baik"},         // 90% resolved ≥80
		{"no-sla-needs", 0, 0, 10, 3, "Perlu bimbingan"},
		{"insufficient", 0, 0, 0, 0, "Belum cukup data"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := agentStatusBadge(c.withSLA, c.met, c.handled, c.resolved)
			if got != c.wantLabel {
				t.Errorf("agentStatusBadge = %q, want %q", got, c.wantLabel)
			}
		})
	}
}
