package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// roles_matrix_guard_test.go — dipisah dari roles_test.go agar tiap file di
// bawah ambang tipe Test (400). Isinya guard floor approve/arr matriks peran
// (BL-145 subtask 0 & BL-166): approve/arr tak boleh tersimpan tanpa level
// akses memadai pada modul yang relevan. Setup/helper bersama (setupRoles,
// rolesReq, roleFormValues, dll) tetap di roles_test.go.

// matrixFormValues = form update peran minimal + satu sel matriks (level/approve/
// arr) untuk satu modul. Dipakai tes guard hasLevel (BL-145 subtask 0).
func matrixFormValues(display, scope, obj, level string, approve, arr bool) url.Values {
	f := roleFormValues(display, scope)
	f.Set("level."+obj, level)
	if approve {
		f.Set("approve."+obj, "1")
	}
	if arr {
		f.Set("arr."+obj, "1")
	}
	return f
}

// TestRoleUpdate_BlocksApproveWithoutLevel: form kirim level=none + approve=1
// utk modul ber-CanApprove (renewal_mgmt) — readRoleMatrix HARUS menolak baris
// approve krn modulnya sendiri nol akses baca/tulis (celah kelas BL-166, sisi
// tulis kebijakan: tanpa guard ini, approve/arr bisa tersimpan tanpa read/write).
func TestRoleUpdate_BlocksApproveWithoutLevel(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "all", false)

	form := matrixFormValues("Keuangan", "all", "crm:renewal_mgmt", "none", true, false)
	req := rolesReq(http.MethodPost, "/w/test/roles/finance", form, "finance")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	if env.hasPerm(t, "finance", "crm:renewal_mgmt", "approve") {
		t.Error("approve tak boleh tersimpan saat level=none")
	}
	if env.hasPerm(t, "finance", "crm:renewal_mgmt", "read") || env.hasPerm(t, "finance", "crm:renewal_mgmt", "write") {
		t.Error("level none tak boleh menyimpan read/write apa pun")
	}
}

// TestRoleUpdate_BlocksARRWithoutLevel: sama seperti approve, utk modul
// ber-CanARR (subscriptions) — arr=1 dgn level=none harus ditolak.
func TestRoleUpdate_BlocksARRWithoutLevel(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "all", false)

	form := matrixFormValues("Keuangan", "all", "crm:subscriptions", "none", false, true)
	req := rolesReq(http.MethodPost, "/w/test/roles/finance", form, "finance")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	if env.hasPerm(t, "finance", "crm:subscriptions", "arr") {
		t.Error("arr tak boleh tersimpan saat level=none")
	}
}

// TestRoleUpdate_AllowsARRWhenEligibleModuleHasLevel: skenario persis laporan
// bug — Subscriptions level=none TAPI Leads="Kelola" (write). arr.crm:
// subscriptions=1 harus TETAP tersimpan krn crm:leads ada di
// authz.ARRGateObjects (fix checkbox "Lihat Nilai Kontrak" tak terbuka walau
// modul lain sudah "Kelola"). Pembanding positif dari TestRoleUpdate_
// BlocksARRWithoutLevel (yang membuktikan floor masih berlaku saat BENAR-BENAR
// nol akses di semua modul gate).
func TestRoleUpdate_AllowsARRWhenEligibleModuleHasLevel(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "all", false)

	form := roleFormValues("Keuangan", "all")
	form.Set("level.crm:subscriptions", "none")
	form.Set("level.crm:leads", "write")
	form.Set("arr.crm:subscriptions", "1")
	req := rolesReq(http.MethodPost, "/w/test/roles/finance", form, "finance")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	if !env.hasPerm(t, "finance", "crm:subscriptions", "arr") {
		t.Error("arr harus tersimpan: crm:leads='write' membuka gate lintas modul (authz.ARRGateObjects) walau Subscriptions sendiri level=none")
	}
	if env.hasPerm(t, "finance", "crm:subscriptions", "read") || env.hasPerm(t, "finance", "crm:subscriptions", "write") {
		t.Error("level.crm:subscriptions=none tak boleh menyimpan read/write Subscriptions apa pun")
	}
}

// TestRoleUpdate_AllowsApproveWithReadOnly: floor approve/arr adalah "read"
// (BUKAN "write") — pola maker-checker sengaja tak butuh hak sunting utk
// menyetujui (business.conf). level="read" + approve=1 harus TERSIMPAN.
func TestRoleUpdate_AllowsApproveWithReadOnly(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "all", false)

	form := matrixFormValues("Keuangan", "all", "crm:renewals", "read", true, false)
	req := rolesReq(http.MethodPost, "/w/test/roles/finance", form, "finance")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	if !env.hasPerm(t, "finance", "crm:renewals", "read") {
		t.Error("read harus tersimpan")
	}
	if !env.hasPerm(t, "finance", "crm:renewals", "approve") {
		t.Error("approve dgn level=read harus tersimpan (floor bukan write)")
	}
}
