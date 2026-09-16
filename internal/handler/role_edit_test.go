package handler

import (
	"net/http"
	"strings"
	"testing"
)

// role_edit_test.go — halaman DETAIL/EDIT satu peran (GET /w/{slug}/roles/{name}).
// Yang dijaga: gerbang crm:roles sama seperti daftar (URL detail pun bisa diketik
// langsung), peran tak ada → redirect role_notfound, peran kustom render editor,
// peran sistem render terkunci (tanpa tombol simpan). Sibling roles_test.go —
// berbagi helper (setupRoles, rolesReq, runAccount) di package yang sama.

// TestRoleEdit_Gate: hanya admin (pemegang crm:roles) boleh membuka detail;
// peran CRM lain & "" ditolak 403 dengan penjelasan — bukan diandalkan dari
// daftar, sebab URL detail bisa diketik langsung.
func TestRoleEdit_Gate(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "own", false)
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
			req := rolesReq(http.MethodGet, "/w/test/roles/finance", nil, "finance")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.RoleEditPage)
			if c.allow {
				if rec.Code != http.StatusOK {
					t.Errorf("role %q harus lolos gate, got %d\n%s", c.role, rec.Code, rec.Body.String())
				}
				return
			}
			if rec.Code != http.StatusForbidden {
				t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "admin CRM") {
				t.Errorf("penolakan harus menyebut admin CRM")
			}
		})
	}
}

// TestRoleEdit_NotFound: admin membuka peran yang tak ada → kembali ke daftar
// dengan role_notfound (bukan 404 telanjang), tanpa menyentuh render editor.
func TestRoleEdit_NotFound(t *testing.T) {
	env, uid := setupRoles(t)
	req := rolesReq(http.MethodGet, "/w/test/roles/ghost", nil, "ghost")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleEditPage)
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "err=role_notfound") {
		t.Errorf("peran tak ada harus redirect role_notfound, got %q (status %d)", loc, rec.Code)
	}
}

// TestRoleEdit_RendersCustom: peran kustom → editor tampil (nama tampilan +
// tombol simpan), bukti halaman memuat form sunting yang bisa di-POST.
func TestRoleEdit_RendersCustom(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "own", false)
	req := rolesReq(http.MethodGet, "/w/test/roles/finance", nil, "finance")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleEditPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Keuangan") {
		t.Error("body harus memuat nama tampilan peran")
	}
	if !strings.Contains(body, "Simpan Perubahan") {
		t.Error("peran kustom harus punya tombol simpan (form editor)")
	}
}

// TestRoleEdit_ApproveARRReactiveToLevel: kolom "Setujui" (baris yang
// CanApprove — Deals pada branch ini, LIHAT CATATAN) & "Lihat ARR" (Subscriptions,
// satu-satunya CanARR) ter-render reaktif Datastar (BL-145 subtask 4/5) — level
// select ter-bind signal per baris, checkbox terkait ter-bind signal SENDIRI +
// dinonaktifkan kondisional saat level baris itu "none", dan handler on:change
// level memaksa signal checkbox itu balik ke false saat level diganti ke
// "none". Reaktivitas ini murni UX klien — backend (readRoleMatrix guard
// hasLevel, BL-145 subtask 0) tetap penjaga sesungguhnya, tak diuji ulang di
// sini.
//
// CATATAN: dipakai "crm:deals" (bukan "crm:renewals") krn branch subtask 4/5
// ini dicabang dari main SEBELUM 145-2 (pemindahan approve renewal_mgmt→
// renewals) merge — CanApprove Deals masih true di base ini. Tak masalah:
// subtask ini menguji MEKANISME reaktivitas per-baris (generik, jalan untuk
// modul CanApprove manapun), bukan objek spesifik mana yang CanApprove.
func TestRoleEdit_ApproveARRReactiveToLevel(t *testing.T) {
	env, uid := setupRoles(t)
	req := rolesReq(http.MethodGet, "/w/test/roles/manager", nil, "manager")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleEditPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`data-bind="lvl_deals"`, // level select baris Deals ter-bind
		`data-bind="apv_deals"`, // checkbox Setujui ter-bind signal sendiri
		`data-attr="{disabled: $lvl_deals == &#39;none&#39;}"`,
		`data-on:change="evt.target.value===&#39;none&#39;&amp;&amp;($apv_deals=false)"`,
		`data-bind="lvl_subscriptions"`, // level select baris Subscriptions ter-bind
		`data-bind="arr_subscriptions"`, // checkbox Lihat ARR ter-bind signal sendiri
		`data-attr="{disabled: $lvl_subscriptions == &#39;none&#39;}"`,
		`data-on:change="evt.target.value===&#39;none&#39;&amp;&amp;($arr_subscriptions=false)"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("matriks harus memuat %q (reaktivitas approve/arr ↔ level):\n%s", want, body)
		}
	}
}

// TestRoleEdit_SystemLocked: peran sistem (admin) → keterangan terkunci, TANPA
// tombol simpan — admin diwakili glob crm:* yang tak terpetakan ke matriks.
func TestRoleEdit_SystemLocked(t *testing.T) {
	env, uid := setupRoles(t)
	req := rolesReq(http.MethodGet, "/w/test/roles/admin", nil, "admin")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleEditPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "akses penuh") {
		t.Error("peran sistem harus tampil keterangan akses penuh")
	}
	if strings.Contains(body, "Simpan Perubahan") {
		t.Error("peran sistem tak boleh punya tombol simpan")
	}
}
