package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/ui/pages/panel"
)

// engagements_view.go — gerbang F2 (Casbin bisnis) modul Engagements / Check-ins
// (Modul 6 Customer Success, slice 6.5). Satu tempat objek Casbin "crm:engagements"
// disebut, agar menu sidebar & gate halaman selalu menyebut objek/aksi yang SAMA
// (nol menu hantu). Meniru kb_articles_view.go / tickets_view.go.
//
// Akses: Admin (crm:* write), Manager (write), CSM (write). Sales tidak punya
// crm:engagements di policy.csv → fail-closed F2. Support juga tidak → sama.
// F3 ownership ditegakkan lewat EngagementsListFilterFor (ownership.go).

// canViewEngagements = gerbang READ daftar engagement. Sumber tunggal untuk menu
// & gate halaman EngagementsList. Admin/Manager/CSM lolos; Sales/Support tidak.
func canViewEngagements(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:engagements", "read")
}

// canWriteEngagementsPerm = izin F2 mentah tulis engagement (tanpa cek arsip
// workspace). write mencakup read; Admin/Manager/CSM lolos.
func canWriteEngagementsPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:engagements", "write")
}

// canWriteEngagements = tombol tulis di view: izin F2 write DAN workspace tak
// read-only (arsip). Dihitung di handler, dioper bool — view tak panggil authz.
func canWriteEngagements(ctx context.Context) bool {
	return canWriteEngagementsPerm(ctx) && !IsReadOnly(ctx)
}

// renderEngagementsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderEngagementsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Engagements", "/engagements", panel.SalesForbidden("Engagements"))
}

// engagementsMsg memetakan ?ok= → pesan sukses Engagements.
func engagementsMsg(code string) string {
	switch code {
	case "created":
		return "Engagement berhasil dibuat."
	case "updated":
		return "Status engagement diperbarui."
	default:
		return ""
	}
}

// engagementsErrMsg memetakan ?err= → pesan galat form Engagements.
func engagementsErrMsg(code string) string {
	switch code {
	case "required":
		return "Subjek, desa, tipe, dan jadwal wajib diisi."
	case "type":
		return "Tipe engagement tidak valid."
	case "status":
		return "Status tidak valid."
	case "account":
		return "Desa tidak ditemukan atau tidak dalam cakupan Anda."
	case "failed":
		return "Gagal menyimpan engagement. Coba lagi."
	default:
		return ""
	}
}
