package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/ui/pages/panel"
)

// cs_renewals_view.go — gerbang F2 (Casbin bisnis) modul Renewal Management CS
// (Modul 6 slice 6.6). Objek Casbin "crm:renewal_mgmt" disebut SATU TEMPAT agar
// menu sidebar & gate halaman selalu setara (nol menu hantu).
//
// Akses: Admin (crm:* write), Manager (write+approve), CSM (write), Sales (read).
// Support tidak punya crm:renewal_mgmt → fail-closed. Pola identik
// engagements_view.go / kb_articles_view.go / tickets_view.go.

// canViewCSRenewals = gerbang READ Renewal Management CS. Admin/Manager/CSM/Sales
// lolos; Support tidak.
func canViewCSRenewals(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:renewal_mgmt", "read")
}

// canWriteCSRenewalsPerm = izin F2 mentah tulis renewal CS (tanpa cek arsip
// workspace). Admin/Manager/CSM lolos; Sales tidak.
func canWriteCSRenewalsPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:renewal_mgmt", "write")
}

// canWriteCSRenewals = tombol tulis di view: izin F2 write DAN workspace tak
// read-only (arsip). Dihitung di handler, dioper bool — view tak panggil authz.
func canWriteCSRenewals(ctx context.Context) bool {
	return canWriteCSRenewalsPerm(ctx) && !IsReadOnly(ctx)
}

// renderCSRenewalsForbidden — 403 + penjelasan bagi anggota tanpa hak.
func (h *Handler) renderCSRenewalsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Renewal Management", "/renewal-management",
		panel.SalesForbidden("Renewal Management"))
}

// csRenewalsMsg memetakan ?ok= → pesan sukses Renewal Management.
func csRenewalsMsg(code string) string {
	switch code {
	case "updated":
		return "Aksi renewal diperbarui."
	default:
		return ""
	}
}

// csRenewalsErrMsg memetakan ?err= → pesan galat form Renewal Management.
func csRenewalsErrMsg(code string) string {
	switch code {
	case "stage":
		return "Stage renewal tidak valid."
	case "risk":
		return "Tingkat risiko tidak valid."
	case "date":
		return "Format tanggal tindakan berikutnya tidak valid."
	case "owner":
		return "Anggota tidak ditemukan."
	case "notfound":
		return "Langganan tidak ditemukan atau di luar cakupan Anda."
	case "failed":
		return "Gagal menyimpan aksi renewal. Coba lagi."
	default:
		return ""
	}
}
