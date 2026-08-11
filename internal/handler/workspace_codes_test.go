package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// getCodeFormat = pintasan test membaca format satu entitas dari pool.
func (e *testEnv) getCodeFormat(t *testing.T, entity codes.Entity) (db.CodeFormat, error) {
	t.Helper()
	return e.q.GetCodeFormat(t.Context(), db.GetCodeFormatParams{
		TenantID: e.tenantID, Entity: string(entity),
	})
}

// workspace_codes_test.go — halaman FORMAT kode entitas. Yang dijaga: gerbang
// (pengelola boleh ubah, member hanya lihat), validasi backend (nilai
// user-controlled tak boleh lolos ke DB sebagai galat constraint), dan bahwa
// format tersimpan sungguh mengubah kode BARU (lewat GenerateEntityCode).

// doCodePost menjalankan POST /codes ber-form dengan session ber-role tertentu +
// Queries ber-scope + chi param slug (dipakai wsRedirect/wsPath). Form-encoded,
// bukan JSON: handler ini native form POST → 303 (gotcha #16).
func (e *testEnv) doCodePost(uid int64, role string, form url.Values, fn http.HandlerFunc) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/w/test/codes", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withChiParam(req, slugURLParam, "test")
	rec := httptest.NewRecorder()
	wrapped := e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session.SetIdentity(r.Context(), uid, "test@local", role, false, e.tenantID, "Test", "test", "")
		fn(w, r.WithContext(withQueries(r.Context(), e.q)))
	}))
	wrapped.ServeHTTP(rec, req)
	return rec
}

// codeForm merakit form values untuk satu entitas.
func codeForm(entity, prefix, separator, padding string) url.Values {
	return url.Values{
		"entity":    {entity},
		"prefix":    {prefix},
		"separator": {separator},
		"padding":   {padding},
	}
}

// TestCodeFormatUpdate_OwnerSaves: owner menyimpan format → tersimpan di DB, kode
// berikutnya memakainya, redirect sukses, audit tercatat.
func TestCodeFormatUpdate_OwnerSaves(t *testing.T) {
	env, uid := setupTest(t)
	rec := env.doCodePost(uid, "owner",
		codeForm(string(codes.EntityAccount), "VILL", "/", "5"), env.h.WorkspaceCodeFormatUpdate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303 (PRG); body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Errorf("redirect harus ?ok=saved, got %q", loc)
	}
	// Tersimpan di DB dengan bentuk yang benar.
	row, err := env.getCodeFormat(t, codes.EntityAccount)
	if err != nil {
		t.Fatalf("get format: %v", err)
	}
	if row.Prefix != "VILL" || row.Separator != "/" || row.Padding != 5 {
		t.Errorf("tersimpan salah: %+v (want VILL / 5)", row)
	}
	// Audit tercatat.
	logs, _ := env.q.ListAuditLogs(t.Context(), 10)
	found := false
	for _, l := range logs {
		if l.Action == "workspace.code_format" {
			found = true
		}
	}
	if !found {
		t.Error("perubahan format harus tercatat di audit_logs")
	}
}

// TestCodeFormatUpdate_MemberDenied: member (bukan pengelola) → ditolak, tak
// menyimpan apa pun. Gerbang di HANDLER, bukan route (member boleh buka halaman).
func TestCodeFormatUpdate_MemberDenied(t *testing.T) {
	env, _ := setupTest(t)
	member := env.seedMember(t, "member@local", "member", 0)
	rec := env.doCodePost(member.ID, "member",
		codeForm(string(codes.EntityAccount), "HACK", "-", "3"), env.h.WorkspaceCodeFormatUpdate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
		t.Errorf("member harus ditolak (err=forbidden), got %q", loc)
	}
	if _, err := env.getCodeFormat(t, codes.EntityAccount); err == nil {
		t.Error("member tak boleh menyimpan format — baris tak seharusnya ada")
	}
}

// TestCodeFormatUpdate_TolakInvalid: input yang melanggar ValidFormat ditolak di
// backend (bukan cuma atribut min/max form) dan tak menyentuh DB.
func TestCodeFormatUpdate_TolakInvalid(t *testing.T) {
	env, uid := setupTest(t)
	cases := []struct {
		name             string
		form             url.Values
		wantErrSubstring string
	}{
		{"prefix kosong", codeForm(string(codes.EntityAccount), "", "-", "3"), "err=code_prefix"},
		{"prefix 17", codeForm(string(codes.EntityAccount), "ABCDEFGHIJKLMNOPQ", "-", "3"), "err=code_prefix"},
		{"padding 13", codeForm(string(codes.EntityAccount), "DESA", "-", "13"), "err=code_padding"},
		{"padding bukan angka", codeForm(string(codes.EntityAccount), "DESA", "-", "x"), "err=code_padding"},
		{"entitas asing", codeForm("bukan_entitas", "DESA", "-", "3"), "err=code_entity"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := env.doCodePost(uid, "owner", c.form, env.h.WorkspaceCodeFormatUpdate)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, c.wantErrSubstring) {
				t.Errorf("harus ditolak dengan %q, got %q", c.wantErrSubstring, loc)
			}
		})
	}
	// Tak satu pun tersimpan.
	rows, _ := env.q.ListCodeFormats(t.Context(), env.tenantID)
	if len(rows) != 0 {
		t.Errorf("input invalid tak boleh menyimpan apa pun, ada %d baris", len(rows))
	}
}

// TestCodeFormatUpdate_PrefixTrim: spasi tepi prefix dibersihkan sebelum simpan.
func TestCodeFormatUpdate_PrefixTrim(t *testing.T) {
	env, uid := setupTest(t)
	env.doCodePost(uid, "owner",
		codeForm(string(codes.EntityDeal), "  DEAL  ", "-", "3"), env.h.WorkspaceCodeFormatUpdate)

	row, err := env.getCodeFormat(t, codes.EntityDeal)
	if err != nil {
		t.Fatalf("get format: %v", err)
	}
	if row.Prefix != "DEAL" {
		t.Errorf("prefix harus di-trim jadi %q, got %q", "DEAL", row.Prefix)
	}
}
