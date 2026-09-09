package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/fls"
	"go_starter/internal/session"
)

// workspace_field_security_update.go — aksi simpan matriks Field Security. Dipisah
// dari workspace_field_security.go (halaman + gate) agar keduanya di bawah ambang
// file-health.

// WorkspaceFieldSecurityUpdate — POST /w/{workspace}/field-security. Simpan SELURUH
// matriks (pola replace-all seperti RoleUpdate): hapus semua baris tenant lalu tulis
// satu baris per peran. Menulis baris untuk SETIAP peran (termasuk all-false) agar
// "admin uncheck semua" tersimpan nyata & beda dari "belum dikonfigurasi" (= nol baris).
//
// Gerbang di handler (canManageFieldSecurity), bukan route — pola /codes & /roles:
// admin di mode read-only boleh membuka halaman tapi tak boleh menyimpan. Section-nya
// dirender di halaman /roles (opsi B), jadi PRG (sukses & tolak) kembali ke /roles.
func (h *Handler) WorkspaceFieldSecurityUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageFieldSecurity(ctx) {
		wsRedirect(w, r, "/roles", "forbidden")
		return
	}
	if IsReadOnly(ctx) {
		wsRedirect(w, r, "/roles", "forbidden")
		return
	}
	tenantID := session.TenantID(ctx)

	// Closed set: peran DITENTUKAN dari DB, bukan dari nama field POST — field peran
	// liar/asing (mis. peran yang baru dihapus, atau nama yang dikarang) diabaikan
	// diam-diam. FK komposit di DB adalah jaring terakhir; ini yang pertama.
	roles, err := h.q(ctx).ListBusinessRoles(ctx, tenantID)
	if err != nil {
		h.Log.Error("field-security: list roles", "err", err)
		wsRedirect(w, r, "/roles", "failed")
		return
	}

	// Bangun kebijakan tiap peran dari form. coerce view = view||edit: config
	// "sunting tanpa lihat" nirmakna & ditolak CHECK DB — kita naikkan view alih-alih
	// menolak simpan, agar admin yang mencentang edit saja tetap mendapat hasil masuk
	// akal (cermin edit⇒view). Peta ini juga sumber reload cache (satu kebenaran).
	policies := make(map[string]fls.Policy, len(roles))
	for _, role := range roles {
		view := r.FormValue("view."+role.Name) == "1"
		edit := r.FormValue("edit."+role.Name) == "1"
		if edit {
			view = true
		}
		policies[role.Name] = fls.Policy{CanViewPhone: view, CanEditPhone: edit}
	}

	uid := session.UserID(ctx)
	// Replace-all dalam tx Scope (h.q): kosongkan lalu tulis set baru — sederhana &
	// bebas selisih (pola RoleUpdate). Gagal di tengah → tx rollback, cache tak
	// disentuh (ReloadTenant hanya setelah semua Upsert sukses).
	if err := h.q(ctx).DeleteFieldSecurityPoliciesForTenant(ctx, tenantID); err != nil {
		h.Log.Error("field-security: clear", "err", err)
		wsRedirect(w, r, "/roles", "failed")
		return
	}
	for role, p := range policies {
		if err := h.q(ctx).UpsertFieldSecurityPolicy(ctx, db.UpsertFieldSecurityPolicyParams{
			TenantID:     tenantID,
			BusinessRole: role,
			CanViewPhone: p.CanViewPhone,
			CanEditPhone: p.CanEditPhone,
			CreatedBy:    &uid,
		}); err != nil {
			h.Log.Error("field-security: upsert", "role", role, "err", err)
			wsRedirect(w, r, "/roles", "failed")
			return
		}
	}

	// Cache berlaku seketika (pola reloadBusinessTenant). ReloadTenant menandai tenant
	// TERKONFIGURASI walau semua all-false — beda dari default terkunci.
	fls.ReloadTenant(tenantID, policies)

	// Ter-audit: ini kontrol keamanan (siapa boleh melihat PII nomor), perubahannya
	// wajib punya jawaban "siapa & kapan". Metadata = jumlah peran, bukan PII.
	h.auditWorkspace(ctx, uid, "workspace.field_security", tenantID, map[string]string{
		"roles": strconv.Itoa(len(policies)),
	})
	// PRG kembali ke halaman /roles yang menampung section Field Security (opsi B):
	// kode `fsec_saved` (bukan `saved`) agar alert-nya spesifik, tak tertukar dengan
	// "Perubahan peran disimpan".
	http.Redirect(w, r, wsPath(slugFromRequest(r), "/roles")+"?ok=fsec_saved", http.StatusSeeOther)
}
