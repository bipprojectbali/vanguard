package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/authz"
)

// roles_merged_fsec_test.go — form GABUNGAN /roles/{name} (BL-145 subtask 7):
// satu POST, satu submit, mencakup identitas peran + matriks Casbin (F2) DAN
// Field Security (F4, HP/WhatsApp) untuk peran KUSTOM (peran sistem tetap pakai
// form terpisah, role_field_security_test.go). Sentinel fsec_present=1 menandai
// "bagian FLS memang dirender" — lihat komentar RoleUpdate (roles.go). Tulis FLS
// lewat db.WithSavepoint (SAVEPOINT bersarang, tx yang SAMA dgn tulis matriks),
// bukan transaksi kedua — lihat ADR 0012 (revisi BL-145 subtask 7).

// TestRoleUpdate_MergedFsec_SavesEndToEnd: submit satu form dgn field matriks +
// fsec_present=1 + view/edit → KEDUANYA tersimpan dari satu request.
func TestRoleUpdate_MergedFsec_SavesEndToEnd(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "own", false)

	form := roleFormValues("Keuangan", "own")
	form.Set("level.crm:contacts", "write")
	form.Set("fsec_present", "1")
	form.Set("view", "1")
	form.Set("edit", "1")
	req := rolesReq(http.MethodPost, "/w/test/roles/finance", form, "finance")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	if !env.hasPerm(t, "finance", "crm:contacts", "write") {
		t.Error("matriks: crm:contacts write harus tersimpan")
	}
	if p := env.fsecPolicies(t)["finance"]; !p.view || !p.edit {
		t.Errorf("FLS: finance harus tersimpan lihat+sunting dari form gabungan, got %+v", p)
	}
	env.assertAudited(t, "workspace.field_security")
}

// TestRoleUpdate_MergedFsec_AbsentSentinelLeavesUntouched: submit ala-lama (tanpa
// fsec_present, mis. panggilan/test yang tak tahu-menahu FLS) → matriks tetap
// tersimpan, TAK ada baris FLS tertulis sama sekali.
func TestRoleUpdate_MergedFsec_AbsentSentinelLeavesUntouched(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "own", false)

	form := roleFormValues("Keuangan", "own")
	form.Set("level.crm:contacts", "write")
	// TANPA fsec_present, TANPA view/edit.
	req := rolesReq(http.MethodPost, "/w/test/roles/finance", form, "finance")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	if !env.hasPerm(t, "finance", "crm:contacts", "write") {
		t.Error("matriks tetap harus tersimpan walau bagian FLS absen")
	}
	if len(env.fsecPolicies(t)) != 0 {
		t.Error("fsec_present absen: tak boleh ada baris FLS yang ditulis sama sekali")
	}
}

// TestRoleUpdate_MergedFsec_GateRecheckedIndependently: peran yang punya
// crm:roles (bisa sunting matriks) TAPI TAK punya crm:field_security (ditanam
// manual, di luar jalur panel biasa — crm:field_security sengaja tak jadi
// kolom editor) menyelipkan fsec_present+view/edit di form gabungan → matriks
// tetap tersimpan, TAPI FLS TAK ditulis: canManageFieldSecurity diperiksa ULANG
// di RoleUpdate, bukan cuma dipercaya dari kondisi render GET.
func TestRoleUpdate_MergedFsec_GateRecheckedIndependently(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "own", false)

	// Beri "finance" crm:roles LANGSUNG di enforcer (di luar CRMModules(), tak
	// bisa lewat panel) tanpa crm:field_security — loadBusinessRolesWith
	// mengganti enforcer tenant test set-penuh (peran bawaan + extra ini).
	env.loadBusinessRolesWith(t, authz.BusinessPerm{Role: "finance", Obj: "crm:roles", Act: "write"})

	form := roleFormValues("Keuangan", "own")
	form.Set("level.crm:contacts", "write")
	form.Set("fsec_present", "1")
	form.Set("view", "1")
	form.Set("edit", "1")
	req := rolesReq(http.MethodPost, "/w/test/roles/finance", form, "finance")
	rec := env.runAccount(uid, "owner", "finance", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved (matriks tetap diizinkan), got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	if !env.hasPerm(t, "finance", "crm:contacts", "write") {
		t.Error("matriks harus tetap tersimpan (crm:roles dipegang)")
	}
	if len(env.fsecPolicies(t)) != 0 {
		t.Error("tanpa crm:field_security: FLS TAK boleh ditulis walau fsec_present+view/edit diselipkan")
	}
}

// TestRoleUpdate_MergedFsec_ReadYourWritesSameRequest: submit
// level.crm:contacts=write BERSAMA edit=1 dalam SATU request → FLS edit harus
// tetap true. Ini bukti savepoint (db.WithSavepoint) membaca matriks yang BARU
// SAJA ditulis pada request yang SAMA (satu tx) — bila FLS dibaca dari tx/koneksi
// terpisah SEBELUM commit matriks, coercion "tak ada write di Contacts/Leads"
// akan keliru memaksa edit=false karena matriks lama (kosong) yang terlihat.
func TestRoleUpdate_MergedFsec_ReadYourWritesSameRequest(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "own", false)
	// Prakondisi: finance belum punya level apa pun di Contacts/Leads.
	if env.hasPerm(t, "finance", "crm:contacts", "write") {
		t.Fatal("prasyarat: finance belum boleh apa-apa di crm:contacts")
	}

	form := roleFormValues("Keuangan", "own")
	form.Set("level.crm:contacts", "write") // matriks BARU dlm request yang sama
	form.Set("fsec_present", "1")
	form.Set("view", "1")
	form.Set("edit", "1")
	req := rolesReq(http.MethodPost, "/w/test/roles/finance", form, "finance")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	if !env.hasPerm(t, "finance", "crm:contacts", "write") {
		t.Fatal("matriks harus tersimpan (prasyarat baca-tulis)")
	}
	p := env.fsecPolicies(t)["finance"]
	if !p.edit {
		t.Error("read-your-writes gagal: edit harus tetap true, coercion seharusnya melihat crm:contacts=write pada tx yang sama")
	}
	if !p.view {
		t.Error("edit=true harus meng-coerce view=true juga")
	}
}
