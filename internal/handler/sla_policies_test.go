package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// sla_policies_test.go — katalog SLA Policies (Modul 6 slice A1) di sisi
// handler. Sama seperti Plans (Modul 5), master milik WORKSPACE → hanya DUA
// sumbu yang dijaga:
//
//   - F2 (Casbin bisnis): GET butuh crm:sla read (admin/manager/csm/support
//     lolos, sales TIDAK punya baris → ditolak), POST butuh crm:sla write —
//     hanya manager+admin (glob crm:*); csm/support yang read-saja ditolak
//     403 & tak menyentuh DB. "" ditolak keduanya.
//   - Aturan tulis: sla_name wajib; applies_to_priority & business_hours enum
//     opsional; target respon/selesai bulat ≥0 (menit, migrasi 00016);
//     is_active hanya lewat retire/activate (tak tersentuh saat update
//     profil). TANPA unique constraint (beda dari plans.plan_code).
//
// TANPA F3/F4/keyset (katalog bounded, tak ada kolom pemilik). Koneksi test =
// superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS diuji di
// rls_test.go. Setup & helper request memakai ulang
// setupAccounts/accountsReq/runAccount (generik: memuat kedua enforcer +
// sesi ber-scope). Meniru plans_test.go.

// --- helper ----------------------------------------------------------------

// slaPolicyFormValues merakit form minimal yang valid untuk create/update.
func slaPolicyFormValues(name string) url.Values {
	return url.Values{"sla_name": {name}}
}

// seedSLAPolicyRow menaruh satu kebijakan SLA langsung lewat pool (bypass
// handler), is_active=true — untuk menguji update/retire tanpa merangkai
// create. Mengembalikan db.SlaPolicy utuh.
func (e *testEnv) seedSLAPolicyRow(t *testing.T, name string) db.SlaPolicy {
	t.Helper()
	p, err := e.q.CreateSLAPolicy(t.Context(), db.CreateSLAPolicyParams{
		TenantID: e.tenantID,
		SlaName:  name,
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("seed sla policy %s: %v", name, err)
	}
	return p
}

// allSLAPolicies mendaftar seluruh katalog langsung dari pool — untuk
// membuktikan sebuah aksi menyimpan / tak menyimpan baris. Superuser (bypass
// RLS) → satu tenant test.
func (e *testEnv) allSLAPolicies(t *testing.T) []db.SlaPolicy {
	t.Helper()
	rows, err := e.q.ListSLAPoliciesAll(t.Context())
	if err != nil {
		t.Fatalf("list sla policies: %v", err)
	}
	return rows
}

// --- F2: gerbang read --------------------------------------------------

// TestSLAPolicies_GateRead: siapa boleh MEMBUKA katalog (act read).
// admin/manager/csm/support lolos; sales (tanpa baris crm:sla) & "" ditolak
// 403 dengan penjelasan butuh peran CRM.
func TestSLAPolicies_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"csm", true},
		{"support", true},
		{"sales", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/sla-policies", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.SLAPoliciesList)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate read, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if !strings.Contains(rec.Body.String(), "peran CRM") {
					t.Errorf("penolakan harus menjelaskan butuh peran CRM")
				}
			}
		})
	}
}

// TestSLAPolicies_GateWrite: POST create butuh act write — HANYA manager+
// admin. csm/support (read-saja) & "" ditolak 403 tanpa menyentuh DB.
func TestSLAPolicies_GateWrite(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"csm", false},
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			form := slaPolicyFormValues("Kebijakan Standar")
			req := accountsReq(http.MethodPost, "/w/test/sla-policies", form, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.SLAPolicyCreate)

			rows := env.allSLAPolicies(t)
			if c.allow {
				if rec.Code != http.StatusSeeOther {
					t.Errorf("role %q harus create (303), got %d\n%s", c.role, rec.Code, rec.Body.String())
				}
				if len(rows) != 1 {
					t.Errorf("role %q create harus menyimpan 1 baris, ada %d", c.role, len(rows))
				}
			} else {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if len(rows) != 0 {
					t.Errorf("role %q ditolak tak boleh menyimpan apa pun, ada %d baris", c.role, len(rows))
				}
			}
		})
	}
}

// --- create ------------------------------------------------------------

// TestSLAPolicies_CreateSuccess: manager create → 303 ok=created; baris
// tersimpan aktif, target respon/selesai satuan menit, pembuat sebagai
// created_by; audit sla_policy.create tercatat.
func TestSLAPolicies_CreateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	form := slaPolicyFormValues("SLA Kritis")
	form.Set("applies_to_priority", "Kritis")
	form.Set("business_hours", "24/7")
	form.Set("first_response_target_minutes", "15")
	form.Set("resolution_target_minutes", "120")
	req := accountsReq(http.MethodPost, "/w/test/sla-policies", form, "")
	rec := env.runAccount(uid, "owner", "manager", req, env.h.SLAPolicyCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("redirect harus ok=created, got %q", loc)
	}
	rows := env.allSLAPolicies(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 baris, ada %d", len(rows))
	}
	p := rows[0]
	if p.SlaName != "SLA Kritis" {
		t.Errorf("nama salah: %q", p.SlaName)
	}
	if p.FirstResponseTargetMinutes == nil || *p.FirstResponseTargetMinutes != 15 {
		t.Errorf("target respon harus 15 menit, got %v", p.FirstResponseTargetMinutes)
	}
	if p.ResolutionTargetMinutes == nil || *p.ResolutionTargetMinutes != 120 {
		t.Errorf("target selesai harus 120 menit, got %v", p.ResolutionTargetMinutes)
	}
	if !p.IsActive {
		t.Error("kebijakan baru harus lahir aktif (is_active=true)")
	}
	if p.CreatedBy == nil || *p.CreatedBy != uid {
		t.Errorf("created_by harus pembuat, got %v", p.CreatedBy)
	}
	env.assertAudited(t, "sla_policy.create")
}

// TestSLAPolicies_CreateRejectsInvalid: input yang melanggar validasi
// backend ditolak → redirect err + tak menyentuh DB.
func TestSLAPolicies_CreateRejectsInvalid(t *testing.T) {
	cases := []struct {
		name    string
		form    url.Values
		wantErr string
	}{
		{"nama kosong", slaPolicyFormValues(""), "err=required"},
		{"prioritas asing", withField(slaPolicyFormValues("SLA X"), "applies_to_priority", "Ekstrim"), "err=priority"},
		{"jam berlaku asing", withField(slaPolicyFormValues("SLA X"), "business_hours", "Kapan Saja"), "err=business_hours"},
		{"target respon bukan angka", withField(slaPolicyFormValues("SLA X"), "first_response_target_minutes", "abc"), "err=number"},
		{"target selesai negatif", withField(slaPolicyFormValues("SLA X"), "resolution_target_minutes", "-1"), "err=number"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			req := accountsReq(http.MethodPost, "/w/test/sla-policies", c.form, "")
			rec := env.runAccount(uid, "owner", "admin", req, env.h.SLAPolicyCreate)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, c.wantErr) {
				t.Errorf("harus %q, got %q", c.wantErr, loc)
			}
			if rows := env.allSLAPolicies(t); len(rows) != 0 {
				t.Errorf("input invalid tak boleh menyimpan, ada %d baris", len(rows))
			}
		})
	}
}

// --- update --------------------------------------------------------------

// TestSLAPolicies_UpdateSuccess: admin update → ok=saved, profil tersimpan,
// is_active dipertahankan (bukan efek samping edit), audit
// sla_policy.update tercatat.
func TestSLAPolicies_UpdateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	p := env.seedSLAPolicyRow(t, "SLA Lama")

	form := slaPolicyFormValues("SLA Baru")
	form.Set("applies_to_priority", "Tinggi")
	form.Set("business_hours", "Jam kerja")
	form.Set("first_response_target_minutes", "60")
	form.Set("resolution_target_minutes", "480")
	req := accountsReq(http.MethodPost, "/w/test/sla-policies/"+itoa(p.ID), form, itoa(p.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SLAPolicyUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Errorf("harus ok=saved, got %q (status %d)", loc, rec.Code)
	}
	got, _ := env.q.GetSLAPolicy(t.Context(), p.ID)
	if got.SlaName != "SLA Baru" {
		t.Errorf("nama tak tersimpan: %q", got.SlaName)
	}
	if got.ResolutionTargetMinutes == nil || *got.ResolutionTargetMinutes != 480 {
		t.Errorf("target selesai tak tersimpan: %v", got.ResolutionTargetMinutes)
	}
	if !got.IsActive {
		t.Error("update profil tak boleh mengubah is_active")
	}
	env.assertAudited(t, "sla_policy.update")
}

// --- status: retire / activate ------------------------------------------

// TestSLAPolicies_RetireThenActivate: pensiun mengeset is_active=false
// (ok=retired); aktifkan kembali mengembalikannya true (ok=activated).
// Keduanya ter-audit.
func TestSLAPolicies_RetireThenActivate(t *testing.T) {
	env, uid := setupAccounts(t)
	p := env.seedSLAPolicyRow(t, "SLA Siklus")

	// Pensiun.
	req := accountsReq(http.MethodPost, "/w/test/sla-policies/"+itoa(p.ID)+"/retire", url.Values{}, itoa(p.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SLAPolicyRetire)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=retired") {
		t.Errorf("harus ok=retired, got %q (status %d)", loc, rec.Code)
	}
	if got, _ := env.q.GetSLAPolicy(t.Context(), p.ID); got.IsActive {
		t.Error("kebijakan dipensiunkan harus is_active=false")
	}
	env.assertAudited(t, "sla_policy.retire")

	// Aktifkan kembali.
	req2 := accountsReq(http.MethodPost, "/w/test/sla-policies/"+itoa(p.ID)+"/activate", url.Values{}, itoa(p.ID))
	rec2 := env.runAccount(uid, "owner", "admin", req2, env.h.SLAPolicyActivate)
	if loc := rec2.Header().Get("Location"); !strings.Contains(loc, "ok=activated") {
		t.Errorf("harus ok=activated, got %q (status %d)", loc, rec2.Code)
	}
	if got, _ := env.q.GetSLAPolicy(t.Context(), p.ID); !got.IsActive {
		t.Error("kebijakan diaktifkan kembali harus is_active=true")
	}
	env.assertAudited(t, "sla_policy.activate")
}

// TestSLAPolicies_StatusGateWrite: retire/activate butuh act write — csm
// (read-saja) ditolak 403, is_active tak berubah.
func TestSLAPolicies_StatusGateWrite(t *testing.T) {
	env, uid := setupAccounts(t)
	p := env.seedSLAPolicyRow(t, "SLA Jaga")

	req := accountsReq(http.MethodPost, "/w/test/sla-policies/"+itoa(p.ID)+"/retire", url.Values{}, itoa(p.ID))
	rec := env.runAccount(uid, "owner", "csm", req, env.h.SLAPolicyRetire)
	if rec.Code != http.StatusForbidden {
		t.Errorf("csm harus 403 di retire, got %d", rec.Code)
	}
	if got, _ := env.q.GetSLAPolicy(t.Context(), p.ID); !got.IsActive {
		t.Error("retire yang ditolak tak boleh mengubah is_active")
	}
}
