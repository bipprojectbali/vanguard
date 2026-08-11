package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// role_desc_test.go — kolom wireframe 9.2 yang ditambahkan ke panel peran:
// DESKRIPSI (opsional, maks 200) dan JUMLAH USER (pemegang peran). Dipisah dari
// roles_test.go agar file itu tetap fokus pada gerbang & matriks, tak melewati
// batas file-health. Berbagi helper di package yang sama (setupRoles, rolesReq,
// roleFormValues, seedRole, runAccount, seedMember).

// TestRoles_CreateWithDescription: deskripsi yang dikirim saat create tersimpan
// apa adanya (setelah trim) — kolom 9.2 ikut jalur create, bukan sekadar tampilan.
func TestRoles_CreateWithDescription(t *testing.T) {
	env, uid := setupRoles(t)
	form := roleFormValues("Keuangan", "own")
	form.Set("name", "finance")
	form.Set("description", "  Kelola anggaran & tagihan desa binaan  ")
	req := rolesReq(http.MethodPost, "/w/test/roles", form, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleCreate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Fatalf("harus ok=created, got %q (status %d)", loc, rec.Code)
	}
	got, err := env.q.GetBusinessRole(t.Context(), db.GetBusinessRoleParams{
		TenantID: env.tenantID, Name: "finance",
	})
	if err != nil {
		t.Fatalf("peran tak tersimpan: %v", err)
	}
	if got.Description != "Kelola anggaran & tagihan desa binaan" {
		t.Errorf("deskripsi = %q, want trimmed value", got.Description)
	}
}

// TestRoles_CreateRejectsLongDescription: deskripsi > 200 karakter ditolak
// (err=role_desc) dan tak menyentuh DB — validasi backend, bukan cuma maxlength UI.
func TestRoles_CreateRejectsLongDescription(t *testing.T) {
	env, uid := setupRoles(t)
	form := roleFormValues("Keuangan", "own")
	form.Set("name", "finance")
	form.Set("description", strings.Repeat("x", roleDescMax+1))
	req := rolesReq(http.MethodPost, "/w/test/roles", form, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleCreate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=role_desc") {
		t.Errorf("deskripsi terlalu panjang harus err=role_desc, got %q", loc)
	}
	if env.hasRole(t, "finance") {
		t.Error("input invalid tak boleh menyimpan peran")
	}
}

// TestRoles_UpdateDescription: menyunting deskripsi peran kustom menyimpannya —
// kolom 9.2 ikut jalur update yang sama dengan label & cakupan.
func TestRoles_UpdateDescription(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "own", false)

	form := roleFormValues("Keuangan", "own")
	form.Set("description", "Deskripsi baru sesudah disunting")
	req := rolesReq(http.MethodPost, "/w/test/roles/finance", form, "finance")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved, got %q (status %d)", loc, rec.Code)
	}
	got, _ := env.q.GetBusinessRole(t.Context(), db.GetBusinessRoleParams{
		TenantID: env.tenantID, Name: "finance",
	})
	if got.Description != "Deskripsi baru sesudah disunting" {
		t.Errorf("deskripsi = %q, want updated value", got.Description)
	}
}

// TestRolesPage_ShowsDescriptionAndMemberCount: daftar peran (GET) merender
// deskripsi + jumlah pemegang. finance dengan satu anggota → "1 user"; peran
// tanpa deskripsi tetap terhitung (kolom Jumlah User selalu ada).
func TestRolesPage_ShowsDescriptionAndMemberCount(t *testing.T) {
	env, uid := setupRoles(t)
	// Peran kustom + deskripsi lewat create agar deskripsi terisi.
	form := roleFormValues("Keuangan", "own")
	form.Set("name", "finance")
	form.Set("description", "Kelola anggaran desa binaan")
	req := rolesReq(http.MethodPost, "/w/test/roles", form, "")
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleCreate); rec.Code != http.StatusSeeOther {
		t.Fatalf("create finance gagal, status %d", rec.Code)
	}
	// Satu anggota memegang finance → jumlah user = 1.
	member := env.seedMember(t, "fin@local", "member", 0).ID
	fin := "finance"
	if err := env.q.UpdateMemberBusinessRole(t.Context(), db.UpdateMemberBusinessRoleParams{
		UserID: member, TenantID: env.tenantID, BusinessRole: &fin,
	}); err != nil {
		t.Fatalf("assign role: %v", err)
	}

	getReq := rolesReq(http.MethodGet, "/w/test/roles", nil, "")
	rec := env.runAccount(uid, "owner", "admin", getReq, env.h.RolesPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Kelola anggaran desa binaan") {
		t.Error("daftar harus merender deskripsi peran")
	}
	if !strings.Contains(body, "1 user") {
		t.Error("daftar harus merender jumlah pemegang peran (1 user)")
	}
}

// TestRoleEdit_ShowsDescription: halaman detail peran kustom memuat deskripsi
// tersimpan di input (bisa disunting), bukan sekadar di daftar.
func TestRoleEdit_ShowsDescription(t *testing.T) {
	env, uid := setupRoles(t)
	form := roleFormValues("Keuangan", "own")
	form.Set("name", "finance")
	form.Set("description", "Kelola anggaran desa binaan")
	req := rolesReq(http.MethodPost, "/w/test/roles", form, "")
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleCreate); rec.Code != http.StatusSeeOther {
		t.Fatalf("create finance gagal, status %d", rec.Code)
	}

	getReq := rolesReq(http.MethodGet, "/w/test/roles/finance", url.Values{}, "finance")
	rec := env.runAccount(uid, "owner", "admin", getReq, env.h.RoleEditPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Kelola anggaran desa binaan") {
		t.Error("editor harus memuat deskripsi tersimpan di input")
	}
}
