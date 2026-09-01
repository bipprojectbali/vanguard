package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// plans_test.go — katalog Plans & Pricing (Modul 5) di sisi handler. Beda dari
// Accounts, plan = master milik WORKSPACE → hanya DUA sumbu yang dijaga:
//
//   - F2 (Casbin bisnis): GET butuh crm:plans read (admin/manager/sales/csm lolos),
//     POST butuh crm:plans write — hanya admin (glob crm:*); manager/sales/csm yang
//     read-saja ditolak 403 & tak menyentuh DB. "" ditolak keduanya.
//   - Aturan tulis: plan_code unik per tenant (idx_plans_code) → tabrakan jadi
//     galat spesifik; kategori wajib enum; is_active hanya lewat retire/activate
//     (tak tersentuh saat update profil).
//
// TANPA F3/F4 (tak ada kolom pemilik). Keyset paginasi daftar kelola diuji
// terpisah di crm_master_pagination_test.go (BL-6, keempat katalog master).
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS diuji
// di rls_test.go.
// Setup & helper request memakai ulang setupAccounts/accountsReq/runAccount
// (generik: memuat kedua enforcer + session ber-scope).

// --- helper ----------------------------------------------------------------

// planFormValues merakit form minimal yang valid untuk create/update.
func planFormValues(name, code, category string) url.Values {
	return url.Values{
		"plan_name":     {name},
		"plan_code":     {code},
		"plan_category": {category},
	}
}

// seedPlanRow menaruh satu plan langsung lewat pool (bypass handler), is_active=
// true — untuk menguji update/retire tanpa merangkai create. Mengembalikan db.Plan
// utuh (seedPlan di sales_quotes_test.go mengembalikan int64 saja & fix kategori
// Core; di sini butuh kategori bervariasi + baris utuh).
func (e *testEnv) seedPlanRow(t *testing.T, name, code, category string) db.Plan {
	t.Helper()
	p, err := e.q.CreatePlan(t.Context(), db.CreatePlanParams{
		TenantID:     e.tenantID,
		PlanName:     name,
		PlanCode:     code,
		PlanCategory: category,
		IsActive:     true,
		Currency:     defaultPlanCurrency,
	})
	if err != nil {
		t.Fatalf("seed plan %s: %v", code, err)
	}
	return p
}

// allPlans mendaftar seluruh katalog langsung dari pool — untuk membuktikan sebuah
// aksi menyimpan / tak menyimpan baris. Superuser (bypass RLS) → satu tenant test.
func (e *testEnv) allPlans(t *testing.T) []db.Plan {
	t.Helper()
	cAt, cID := firstPageCursor()
	rows, err := e.q.ListPlansAll(t.Context(), db.ListPlansAllParams{
		CursorCreatedAt: cAt, CursorID: cID, PageSize: allCatalogPageSize,
	})
	if err != nil {
		t.Fatalf("list plans: %v", err)
	}
	return rows
}

// --- F2: gerbang read ------------------------------------------------------

// TestPlans_GateRead: siapa boleh MEMBUKA katalog (act read). admin/manager/sales/
// csm lolos; "" ditolak 403 dengan penjelasan butuh peran CRM.
func TestPlans_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"sales", true},
		{"csm", true},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/plans", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.PlansList)
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

// TestPlans_GateWrite: POST create butuh act write — HANYA admin (glob crm:*).
// manager/sales (read-saja) & "" ditolak 403 tanpa menyentuh DB.
func TestPlans_GateWrite(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", false},
		{"sales", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			form := planFormValues("Paket Dasar", "PLAN-A", "Core")
			req := accountsReq(http.MethodPost, "/w/test/plans", form, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.PlanCreate)

			rows := env.allPlans(t)
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

// --- create ----------------------------------------------------------------

// TestPlans_CreateSuccess: admin create → 303 ok=created; baris tersimpan aktif,
// pembuat sebagai created_by; audit plan.create tercatat.
func TestPlans_CreateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	form := planFormValues("Paket Premium", "PLAN-PREM", "Core")
	form.Set("base_price", "150000")
	form.Set("billing_frequency", "Monthly")
	req := accountsReq(http.MethodPost, "/w/test/plans", form, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.PlanCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("redirect harus ok=created, got %q", loc)
	}
	rows := env.allPlans(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 baris, ada %d", len(rows))
	}
	p := rows[0]
	if p.PlanName != "Paket Premium" || p.PlanCode != "PLAN-PREM" {
		t.Errorf("nama/kode salah: %q / %q", p.PlanName, p.PlanCode)
	}
	if !p.IsActive {
		t.Error("plan baru harus lahir aktif (is_active=true)")
	}
	if p.CreatedBy == nil || *p.CreatedBy != uid {
		t.Errorf("created_by harus pembuat, got %v", p.CreatedBy)
	}
	env.assertAudited(t, "plan.create")
}

// TestPlans_CreateRejectsInvalid: input yang melanggar validasi backend ditolak →
// redirect err + tak menyentuh DB.
func TestPlans_CreateRejectsInvalid(t *testing.T) {
	cases := []struct {
		name    string
		form    url.Values
		wantErr string
	}{
		{"nama kosong", planFormValues("", "PLAN-X", "Core"), "err=required"},
		{"kode kosong", planFormValues("Paket", "", "Core"), "err=required"},
		{"kategori asing", planFormValues("Paket", "PLAN-X", "Bukan"), "err=category"},
		{"billing asing", withField(planFormValues("Paket", "PLAN-X", "Core"), "billing_frequency", "Weekly"), "err=billing"},
		{"harga bukan angka", withField(planFormValues("Paket", "PLAN-X", "Core"), "base_price", "abc"), "err=base_price"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			req := accountsReq(http.MethodPost, "/w/test/plans", c.form, "")
			rec := env.runAccount(uid, "owner", "admin", req, env.h.PlanCreate)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, c.wantErr) {
				t.Errorf("harus %q, got %q", c.wantErr, loc)
			}
			if rows := env.allPlans(t); len(rows) != 0 {
				t.Errorf("input invalid tak boleh menyimpan, ada %d baris", len(rows))
			}
		})
	}
}

// TestPlans_CodeDuplicate: plan_code UNIQUE per tenant — tabrakan dikenali sebagai
// galat spesifik (plan_code_dup), bukan "internal error".
func TestPlans_CodeDuplicate(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedPlanRow(t, "Paket Satu", "DUP-CODE", "Core")

	dup := planFormValues("Paket Dua", "DUP-CODE", "Add-on")
	req := accountsReq(http.MethodPost, "/w/test/plans", dup, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.PlanCreate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=plan_code_dup") {
		t.Errorf("tabrakan plan_code harus err=plan_code_dup, got %q", loc)
	}
	if rows := env.allPlans(t); len(rows) != 1 {
		t.Errorf("duplikat tak boleh menambah baris, ada %d", len(rows))
	}
}

// --- update ----------------------------------------------------------------

// TestPlans_UpdateSuccess: admin update → ok=saved, profil tersimpan, is_active
// dipertahankan (bukan efek samping edit), audit plan.update tercatat.
func TestPlans_UpdateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	p := env.seedPlanRow(t, "Paket Lama", "PLAN-OLD", "Core")

	form := planFormValues("Paket Baru", "PLAN-OLD", "Module")
	req := accountsReq(http.MethodPost, "/w/test/plans/"+itoa(p.ID), form, itoa(p.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.PlanUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Errorf("harus ok=saved, got %q (status %d)", loc, rec.Code)
	}
	got, _ := env.q.GetPlan(t.Context(), p.ID)
	if got.PlanName != "Paket Baru" || got.PlanCategory != "Module" {
		t.Errorf("update tak tersimpan: %q / %q", got.PlanName, got.PlanCategory)
	}
	if !got.IsActive {
		t.Error("update profil tak boleh mengubah is_active")
	}
	env.assertAudited(t, "plan.update")
}

// --- status: retire / activate ---------------------------------------------

// TestPlans_RetireThenActivate: pensiun mengeset is_active=false (ok=retired);
// aktifkan kembali mengembalikannya true (ok=activated). Keduanya ter-audit.
func TestPlans_RetireThenActivate(t *testing.T) {
	env, uid := setupAccounts(t)
	p := env.seedPlanRow(t, "Paket Siklus", "PLAN-CYC", "Core")

	// Pensiun.
	req := accountsReq(http.MethodPost, "/w/test/plans/"+itoa(p.ID)+"/retire", url.Values{}, itoa(p.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.PlanRetire)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=retired") {
		t.Errorf("harus ok=retired, got %q (status %d)", loc, rec.Code)
	}
	if got, _ := env.q.GetPlan(t.Context(), p.ID); got.IsActive {
		t.Error("plan dipensiunkan harus is_active=false")
	}
	env.assertAudited(t, "plan.retire")

	// Aktifkan kembali.
	req2 := accountsReq(http.MethodPost, "/w/test/plans/"+itoa(p.ID)+"/activate", url.Values{}, itoa(p.ID))
	rec2 := env.runAccount(uid, "owner", "admin", req2, env.h.PlanActivate)
	if loc := rec2.Header().Get("Location"); !strings.Contains(loc, "ok=activated") {
		t.Errorf("harus ok=activated, got %q (status %d)", loc, rec2.Code)
	}
	if got, _ := env.q.GetPlan(t.Context(), p.ID); !got.IsActive {
		t.Error("plan diaktifkan kembali harus is_active=true")
	}
	env.assertAudited(t, "plan.activate")
}

// TestPlans_StatusGateWrite: retire/activate butuh act write — sales (read-saja)
// ditolak 403, is_active tak berubah.
func TestPlans_StatusGateWrite(t *testing.T) {
	env, uid := setupAccounts(t)
	p := env.seedPlanRow(t, "Paket Jaga", "PLAN-GRD", "Core")

	req := accountsReq(http.MethodPost, "/w/test/plans/"+itoa(p.ID)+"/retire", url.Values{}, itoa(p.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.PlanRetire)
	if rec.Code != http.StatusForbidden {
		t.Errorf("sales harus 403 di retire, got %d", rec.Code)
	}
	if got, _ := env.q.GetPlan(t.Context(), p.ID); !got.IsActive {
		t.Error("retire yang ditolak tak boleh mengubah is_active")
	}
}
