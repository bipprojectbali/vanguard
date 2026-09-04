package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// workspace_codes_update.go — aksi WorkspaceCodeFormatUpdate (simpan format kode
// per-entitas). Dipisah dari workspace_codes.go (halaman daftar format + helper
// label/pesan) agar keduanya di bawah ambang tipe Route/Handler (150).
// WorkspaceCodeFormatUpdate — POST /w/{workspace}/codes. Simpan format
// SATU entitas (dipilih lewat hidden field `entity`). Guard di handler, bukan
// route: canEditWorkspace menyaring simpan — admin di mode multi boleh membuka
// halaman tapi tak boleh menyimpan.
func (h *Handler) WorkspaceCodeFormatUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canEditWorkspace(ctx) {
		wsRedirect(w, r, "/codes", "forbidden")
		return
	}

	// Entity dari form, bukan URL — satu endpoint melayani semua entitas. Nilai
	// user-controlled → divalidasi terhadap daftar entitas yang dikenal sebelum
	// menyentuh DB (CHECK constraint adalah jaring terakhir, bukan yang pertama).
	entity := codes.Entity(r.FormValue("entity"))
	if !codes.Valid(entity) {
		wsRedirect(w, r, "/codes", "code_entity")
		return
	}

	// Prefix di-trim: spasi tepi di kode lookup nyaris pasti keliru dan bikin kode
	// sulit disebut/dicari. Separator TIDAK di-trim — spasi di dalam separator bisa
	// disengaja, dan panjangnya sudah dibatasi.
	prefix := strings.TrimSpace(r.FormValue("prefix"))
	separator := r.FormValue("separator")
	padding, err := strconv.Atoi(r.FormValue("padding"))
	if err != nil {
		wsRedirect(w, r, "/codes", "code_padding")
		return
	}

	f := codes.Format{Prefix: prefix, Separator: separator, Padding: padding}
	// Validasi di BACKEND, cermin ValidFormat/CHECK 00006 — atribut min/max di form
	// hanya jaring klien. Pesan dipisah agar operator tahu FIELD mana yang salah.
	if l := len(prefix); l < 1 || l > codes.MaxPrefixLen {
		wsRedirect(w, r, "/codes", "code_prefix")
		return
	}
	if !codes.ValidFormat(f) {
		wsRedirect(w, r, "/codes", "code_padding")
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).UpsertCodeFormat(ctx, db.UpsertCodeFormatParams{
		TenantID:  session.TenantID(ctx),
		Entity:    string(entity),
		Prefix:    prefix,
		Separator: separator,
		Padding:   int32(padding),
		CreatedBy: &uid,
	}); err != nil {
		h.Log.Error("codes: upsert format", "err", err)
		wsRedirect(w, r, "/codes", "failed")
		return
	}

	// Ter-audit: format kode memengaruhi setiap kode BARU yang terbit sesudahnya,
	// jadi perubahannya harus punya jawaban untuk "siapa & kapan". target = tenant
	// sendiri; metadata memuat entitas & bentuknya (bukan PII).
	h.auditWorkspace(ctx, uid, "workspace.code_format", session.TenantID(ctx), map[string]string{
		"entity": string(entity),
		"format": f.Render(1),
	})
	http.Redirect(w, r, wsPath(slugFromRequest(r), "/codes")+"?ok=saved", http.StatusSeeOther)
}
