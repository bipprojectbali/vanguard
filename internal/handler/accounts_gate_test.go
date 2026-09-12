package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// accounts_gate_test.go — F2 gerbang read/write, penugasan CSM, dan keyset
// pagination daftar Desa. Setup & helper (setupAccounts, accountsReq,
// runAccount, seedAccount, dst.) di accounts_test.go.

// --- F2: gerbang read/write ------------------------------------------------

// TestAccounts_GateRead: siapa boleh MEMBUKA daftar (act read). write mencakup
// read (matcher) → admin/manager/sales/csm/support lolos; "" ditolak 403.
func TestAccounts_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"sales", true},
		{"csm", true},
		{"support", true}, // lolos gate read; F3 ScopeNone → nol baris (diuji terpisah)
		{"", false},       // tanpa peran CRM → 403 (deny-default, tak ada root)
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.AccountsList)
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

// TestAccounts_GateReadTegakLurusPlatform: owner/super_admin TANPA business_role
// tetap DITOLAK — sumbu bisnis tak mewarisi otoritas platform (§3). Tenant role
// setinggi apa pun tak membuka modul Desa tanpa peran CRM.
func TestAccounts_GateReadTegakLurusPlatform(t *testing.T) {
	env, uid := setupAccounts(t)
	for _, tenantRole := range []string{"owner", "admin", "super_admin"} {
		req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
		rec := env.runAccount(uid, tenantRole, "", req, env.h.AccountsList)
		if rec.Code != http.StatusForbidden {
			t.Errorf("tenant role %q tanpa peran CRM harus 403, got %d", tenantRole, rec.Code)
		}
	}
}

// TestAccounts_GateWrite: POST create butuh act write. support (read saja) &
// "" ditolak; admin/sales lolos. Ditolak → 403 (renderAccountsForbidden), tak
// pernah menyentuh DB.
func TestAccounts_GateWrite(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"sales", true},
		{"support", false}, // read-only di sumbu bisnis
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			form := accountFormValues("prospect")
			// Desa WAJIB di create (village_code+nama diturunkan darinya, BL-66) —
			// tanpa ini jalur "allow" ditolak village_required sebelum menyentuh gate.
			withVillage(form, firstVillage(t, env))
			req := accountsReq(http.MethodPost, "/w/test/accounts", form, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.AccountCreate)

			rows := env.allAccounts(t)
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

// --- Assign CSM ------------------------------------------------------------

// TestAccounts_AssignSuccess: menetapkan CSM anggota workspace → tersimpan,
// ok=assigned, audit tercatat.
func TestAccounts_AssignSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	csm := env.seedMember(t, "csm@local", "member", 0).ID
	a := env.seedAccount(t, "Desa Tugas", &uid, nil, nil)

	form := url.Values{"assigned_csm": {itoa(csm)}}
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/assign", form, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountAssign)

	// BL-108: PRG kembali ke halaman detail Customer Success desa (form assign
	// kini di sana, bukan form edit) — bukan lagi /accounts/{id}.
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "ok=assigned") {
		t.Errorf("harus ok=assigned, got %q (status %d)", loc, rec.Code)
	}
	if !strings.Contains(loc, "/accounts/"+itoa(a.ID)+"/customer-success") {
		t.Errorf("BL-108: redirect harus ke halaman customer-success, got %q", loc)
	}
	got, _ := env.q.GetAccount(t.Context(), a.ID)
	if got.AssignedCsm == nil || *got.AssignedCsm != csm {
		t.Errorf("assigned_csm tak tersimpan, got %v", got.AssignedCsm)
	}
	env.assertAudited(t, "account.assign")
}

// TestAccounts_AssignNonMemberDitolak: kandidat CSM yang BUKAN anggota workspace
// ditolak (err=csm) — memasang non-anggota melahirkan baris yatim tak terlihat F3.
func TestAccounts_AssignNonMemberDitolak(t *testing.T) {
	env, uid := setupAccounts(t)
	outsider := env.seedUserOnly(t, "outsider@local").ID // user global tanpa membership
	a := env.seedAccount(t, "Desa Tugas", &uid, nil, nil)

	form := url.Values{"assigned_csm": {itoa(outsider)}}
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/assign", form, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountAssign)

	// BL-108: gagal validasi juga kembali ke halaman customer-success (tempat
	// form assign berada), bukan /accounts/{id}/edit.
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "err=csm") {
		t.Errorf("non-anggota harus ditolak err=csm, got %q", loc)
	}
	if !strings.Contains(loc, "/accounts/"+itoa(a.ID)+"/customer-success") {
		t.Errorf("BL-108: redirect gagal harus ke halaman customer-success, got %q", loc)
	}
	got, _ := env.q.GetAccount(t.Context(), a.ID)
	if got.AssignedCsm != nil {
		t.Error("penugasan non-anggota tak boleh tersimpan")
	}
}

// --- Keyset pagination -----------------------------------------------------

// TestAccounts_KeysetPagination: dengan pageSize+1 desa, halaman pertama
// menampilkan tautan "Berikutnya"; mengikuti cursor menampilkan sisanya +
// "Ujung daftar." (baris ke-(pageSize+1) tetap terjangkau, tak hilang senyap).
func TestAccounts_KeysetPagination(t *testing.T) {
	env, uid := setupAccounts(t)
	total := pageSize + 1
	for i := 0; i < total; i++ {
		env.seedAccount(t, "Desa "+itoa(int64(i)), &uid, nil, nil)
	}

	// Halaman 1: ada tautan Berikutnya, belum ujung daftar.
	req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
	body := env.runAccount(uid, "owner", "admin", req, env.h.AccountsList).Body.String()
	if !strings.Contains(body, "Berikutnya") {
		t.Fatal("halaman pertama dengan >pageSize desa harus punya tautan Berikutnya")
	}
	if strings.Contains(body, "Ujung daftar.") {
		t.Error("halaman pertama (masih ada lanjutan) tak boleh menyatakan ujung daftar")
	}

	// Ikuti cursor ke halaman 2.
	after := extractAfter(t, body)
	req2 := accountsReq(http.MethodGet, "/w/test/accounts?after="+after, nil, "")
	body2 := env.runAccount(uid, "owner", "admin", req2, env.h.AccountsList).Body.String()
	if !strings.Contains(body2, "Ujung daftar.") {
		t.Error("halaman terakhir harus menyatakan ujung daftar")
	}
	if strings.Contains(body2, "Berikutnya") {
		t.Error("halaman terakhir tak boleh menawarkan Berikutnya (berujung kosong)")
	}
}
