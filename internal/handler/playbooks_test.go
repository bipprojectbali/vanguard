package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// playbooks_test.go — katalog Playbooks (Modul 6 slice A2) di sisi handler.
// Sama seperti SLA Policies (slice A1), master milik WORKSPACE → hanya DUA
// sumbu yang dijaga:
//
//   - F2 (Casbin bisnis): GET butuh crm:playbooks read — HANYA admin/manager/
//     csm lolos (support & sales TANPA baris crm:playbooks sama sekali,
//     BEDA dari SLA Policies tempat support punya baca) → keduanya ditolak
//     403 & tak menyentuh DB, begitu pula "". POST butuh crm:playbooks write
//     — admin/manager/csm (csm write DI SINI, beda dari SLA Policies tempat
//     csm read-saja); support & "" ditolak 403.
//   - Aturan tulis: playbook_name wajib; trigger_scenario &
//     recommended_owner enum opsional; description/steps teks bebas
//     opsional; is_active hanya lewat draft/activate (tak tersentuh saat
//     update profil). TANPA unique constraint.
//
// TANPA F3/F4/keyset (katalog bounded, tak ada kolom pemilik). Koneksi test =
// superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS diuji di
// rls_test.go. Setup & helper request memakai ulang
// setupAccounts/accountsReq/runAccount. Meniru sla_policies_test.go.

// --- helper ----------------------------------------------------------------

// playbookFormValues merakit form minimal yang valid untuk create/update.
func playbookFormValues(name string) url.Values {
	return url.Values{"playbook_name": {name}}
}

// seedPlaybookRow menaruh satu playbook langsung lewat pool (bypass
// handler), is_active=true — untuk menguji update/draft/activate tanpa
// merangkai create. Mengembalikan db.Playbook utuh.
func (e *testEnv) seedPlaybookRow(t *testing.T, name string) db.Playbook {
	t.Helper()
	p, err := e.q.CreatePlaybook(t.Context(), db.CreatePlaybookParams{
		TenantID:     e.tenantID,
		PlaybookName: name,
		IsActive:     true,
	})
	if err != nil {
		t.Fatalf("seed playbook %s: %v", name, err)
	}
	return p
}

// allPlaybooks mendaftar seluruh katalog langsung dari pool — untuk
// membuktikan sebuah aksi menyimpan / tak menyimpan baris. Superuser (bypass
// RLS) → satu tenant test.
func (e *testEnv) allPlaybooks(t *testing.T) []db.Playbook {
	t.Helper()
	cAt, cID := firstPageCursor()
	rows, err := e.q.ListPlaybooksAll(t.Context(), db.ListPlaybooksAllParams{
		CursorCreatedAt: cAt, CursorID: cID, PageSize: allCatalogPageSize,
	})
	if err != nil {
		t.Fatalf("list playbooks: %v", err)
	}
	return rows
}

// --- F2: gerbang read --------------------------------------------------

// TestPlaybooks_GateRead: siapa boleh MEMBUKA katalog (act read).
// admin/manager/csm lolos; support & sales (TANPA baris crm:playbooks sama
// sekali — beda dari SLA Policies tempat support punya baca) & "" ditolak
// 403 dengan penjelasan butuh peran CRM.
func TestPlaybooks_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"csm", true},
		{"support", false},
		{"sales", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/playbooks", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.PlaybooksList)
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

// TestPlaybooks_GateWrite: POST create butuh act write — admin/manager/csm
// (csm write DI SINI, beda dari SLA Policies tempat csm read-saja). support &
// "" ditolak 403 tanpa menyentuh DB.
func TestPlaybooks_GateWrite(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"csm", true},
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			form := playbookFormValues("Playbook Standar")
			req := accountsReq(http.MethodPost, "/w/test/playbooks", form, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.PlaybookCreate)

			rows := env.allPlaybooks(t)
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

// TestPlaybooks_CreateSuccess: csm create → 303 ok=created; baris tersimpan
// aktif, pembuat sebagai created_by; audit playbook.create tercatat.
func TestPlaybooks_CreateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	form := playbookFormValues("Playbook Health Drop")
	form.Set("trigger_scenario", "Health Drop")
	form.Set("recommended_owner", "CSM")
	form.Set("description", "Respons cepat saat skor kesehatan turun tajam.")
	form.Set("steps", "Hubungi PIC\nAudit pemakaian\nJadwalkan check-in")
	req := accountsReq(http.MethodPost, "/w/test/playbooks", form, "")
	rec := env.runAccount(uid, "owner", "csm", req, env.h.PlaybookCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("redirect harus ok=created, got %q", loc)
	}
	rows := env.allPlaybooks(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 baris, ada %d", len(rows))
	}
	p := rows[0]
	if p.PlaybookName != "Playbook Health Drop" {
		t.Errorf("nama salah: %q", p.PlaybookName)
	}
	if p.TriggerScenario == nil || *p.TriggerScenario != "Health Drop" {
		t.Errorf("trigger_scenario salah: %v", p.TriggerScenario)
	}
	if p.RecommendedOwner == nil || *p.RecommendedOwner != "CSM" {
		t.Errorf("recommended_owner salah: %v", p.RecommendedOwner)
	}
	if !p.IsActive {
		t.Error("playbook baru harus lahir aktif (is_active=true)")
	}
	if p.CreatedBy == nil || *p.CreatedBy != uid {
		t.Errorf("created_by harus pembuat, got %v", p.CreatedBy)
	}
	env.assertAudited(t, "playbook.create")
}

// TestPlaybooks_RecommendedOwnerAligned (BL-33): opsi "Pemilik Rekomendasi"
// selaras peran nyata — Manager kini tersedia (di samping CSM/Sales/Support),
// slice opsi & map validasi sepakat, dan create ber-owner Manager tersimpan
// utuh sebagai teks.
func TestPlaybooks_RecommendedOwnerAligned(t *testing.T) {
	// Slice opsi & map validasi harus memuat set peran yang sama.
	want := map[string]struct{}{"Manager": {}, "CSM": {}, "Sales": {}, "Support": {}}
	if len(playbookRecommendedOwnerOptions) != len(want) {
		t.Fatalf("opsi = %v, want set %v", playbookRecommendedOwnerOptions, want)
	}
	for _, o := range playbookRecommendedOwnerOptions {
		if _, ok := want[o]; !ok {
			t.Errorf("opsi tak terduga: %q", o)
		}
		if _, ok := validPlaybookRecommendedOwners[o]; !ok {
			t.Errorf("opsi %q tak ada di map validasi (dropdown menawarkan nilai ditolak backend)", o)
		}
	}

	// Manager (peran baru) diterima end-to-end & tersimpan apa adanya.
	env, uid := setupAccounts(t)
	form := playbookFormValues("Playbook Manajerial")
	form.Set("recommended_owner", "Manager")
	req := accountsReq(http.MethodPost, "/w/test/playbooks", form, "")
	rec := env.runAccount(uid, "owner", "csm", req, env.h.PlaybookCreate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Fatalf("redirect harus ok=created, got %q (status %d)", loc, rec.Code)
	}
	rows := env.allPlaybooks(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 baris, ada %d", len(rows))
	}
	if p := rows[0]; p.RecommendedOwner == nil || *p.RecommendedOwner != "Manager" {
		t.Errorf("recommended_owner harus Manager, got %v", p.RecommendedOwner)
	}
}

// TestPlaybooks_CreateRejectsInvalid: input yang melanggar validasi backend
// ditolak → redirect err + tak menyentuh DB.
func TestPlaybooks_CreateRejectsInvalid(t *testing.T) {
	cases := []struct {
		name    string
		form    url.Values
		wantErr string
	}{
		{"nama kosong", playbookFormValues(""), "err=required"},
		{"skenario pemicu asing", withField(playbookFormValues("PB X"), "trigger_scenario", "Bencana Alam"), "err=trigger_scenario"},
		{"pemilik rekomendasi asing", withField(playbookFormValues("PB X"), "recommended_owner", "Marketing"), "err=recommended_owner"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			req := accountsReq(http.MethodPost, "/w/test/playbooks", c.form, "")
			rec := env.runAccount(uid, "owner", "admin", req, env.h.PlaybookCreate)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, c.wantErr) {
				t.Errorf("harus %q, got %q", c.wantErr, loc)
			}
			if rows := env.allPlaybooks(t); len(rows) != 0 {
				t.Errorf("input invalid tak boleh menyimpan, ada %d baris", len(rows))
			}
		})
	}
}

// --- update --------------------------------------------------------------

// TestPlaybooks_UpdateSuccess: admin update → ok=saved, profil tersimpan,
// is_active dipertahankan (bukan efek samping edit), audit playbook.update
// tercatat.
func TestPlaybooks_UpdateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	p := env.seedPlaybookRow(t, "Playbook Lama")

	form := playbookFormValues("Playbook Baru")
	form.Set("trigger_scenario", "Renewal Approaching")
	form.Set("recommended_owner", "Sales")
	req := accountsReq(http.MethodPost, "/w/test/playbooks/"+itoa(p.ID), form, itoa(p.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.PlaybookUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Errorf("harus ok=saved, got %q (status %d)", loc, rec.Code)
	}
	got, _ := env.q.GetPlaybook(t.Context(), p.ID)
	if got.PlaybookName != "Playbook Baru" {
		t.Errorf("nama tak tersimpan: %q", got.PlaybookName)
	}
	if got.TriggerScenario == nil || *got.TriggerScenario != "Renewal Approaching" {
		t.Errorf("trigger_scenario tak tersimpan: %v", got.TriggerScenario)
	}
	if !got.IsActive {
		t.Error("update profil tak boleh mengubah is_active")
	}
	env.assertAudited(t, "playbook.update")
}

// --- status: draft / activate ------------------------------------------

// TestPlaybooks_DraftThenActivate: jadikan-draf mengeset is_active=false
// (ok=drafted); aktifkan kembali mengembalikannya true (ok=activated).
// Keduanya ter-audit.
func TestPlaybooks_DraftThenActivate(t *testing.T) {
	env, uid := setupAccounts(t)
	p := env.seedPlaybookRow(t, "Playbook Siklus")

	// Jadikan draf.
	req := accountsReq(http.MethodPost, "/w/test/playbooks/"+itoa(p.ID)+"/draft", url.Values{}, itoa(p.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.PlaybookDraft)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=drafted") {
		t.Errorf("harus ok=drafted, got %q (status %d)", loc, rec.Code)
	}
	if got, _ := env.q.GetPlaybook(t.Context(), p.ID); got.IsActive {
		t.Error("playbook dijadikan draf harus is_active=false")
	}
	env.assertAudited(t, "playbook.draft")

	// Aktifkan kembali.
	req2 := accountsReq(http.MethodPost, "/w/test/playbooks/"+itoa(p.ID)+"/activate", url.Values{}, itoa(p.ID))
	rec2 := env.runAccount(uid, "owner", "admin", req2, env.h.PlaybookActivate)
	if loc := rec2.Header().Get("Location"); !strings.Contains(loc, "ok=activated") {
		t.Errorf("harus ok=activated, got %q (status %d)", loc, rec2.Code)
	}
	if got, _ := env.q.GetPlaybook(t.Context(), p.ID); !got.IsActive {
		t.Error("playbook diaktifkan kembali harus is_active=true")
	}
	env.assertAudited(t, "playbook.activate")
}

// TestPlaybooks_StatusGateWrite: draft/activate butuh act write — support
// (tanpa baris crm:playbooks sama sekali, jadi tanpa read MAUPUN write)
// ditolak 403, is_active tak berubah. Beda dari SLA Policies (yang punya
// peran read-saja khusus buat kasus ini, csm) — di Playbooks semua pemegang
// akses (manager/csm/admin) juga punya write, jadi support jadi contoh
// terdekat "berhak apa pun" yang tetap ditolak di gerbang tulis.
func TestPlaybooks_StatusGateWrite(t *testing.T) {
	env, uid := setupAccounts(t)
	p := env.seedPlaybookRow(t, "Playbook Jaga")

	req := accountsReq(http.MethodPost, "/w/test/playbooks/"+itoa(p.ID)+"/draft", url.Values{}, itoa(p.ID))
	rec := env.runAccount(uid, "owner", "support", req, env.h.PlaybookDraft)
	if rec.Code != http.StatusForbidden {
		t.Errorf("support harus 403 di draft, got %d", rec.Code)
	}
	if got, _ := env.q.GetPlaybook(t.Context(), p.ID); !got.IsActive {
		t.Error("draft yang ditolak tak boleh mengubah is_active")
	}
}
