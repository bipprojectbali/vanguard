package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/ui/pages/panel"
)

// success_plans_view.go — gerbang F2 (Casbin bisnis) modul Success Plans
// (Modul 6 Customer Success, slice 6.3). Satu tempat objek Casbin
// "crm:success_plans" disebut, agar menu sidebar & gate halaman selalu menyebut
// objek/aksi yang SAMA (nol menu hantu). Meniru engagements_view.go.
//
// Akses: Admin (crm:* write), Manager (write), CSM (write). Sales tidak punya
// crm:success_plans di policy.csv → fail-closed F2. Support juga tidak → sama.
// F3 ownership ditegakkan lewat SuccessPlansListFilterFor (ownership.go).

// canViewSuccessPlans = gerbang READ daftar success plan. Sumber tunggal untuk
// menu & gate halaman SuccessPlansList. Admin/Manager/CSM lolos; Sales/Support tidak.
func canViewSuccessPlans(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:success_plans", "read")
}

// canWriteSuccessPlansPerm = izin F2 mentah tulis success plan (tanpa cek arsip
// workspace). write mencakup read; Admin/Manager/CSM lolos.
func canWriteSuccessPlansPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:success_plans", "write")
}

// canWriteSuccessPlans = tombol tulis di view: izin F2 write DAN workspace tak
// read-only (arsip). Dihitung di handler, dioper bool — view tak panggil authz.
func canWriteSuccessPlans(ctx context.Context) bool {
	return canWriteSuccessPlansPerm(ctx) && !IsReadOnly(ctx)
}

// renderSuccessPlansForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderSuccessPlansForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Success Plans", "/success-plans", panel.SalesForbidden("Success Plans"))
}

// successPlansMsg memetakan ?ok= → pesan sukses Success Plans.
func successPlansMsg(code string) string {
	switch code {
	case "created":
		return "Success plan berhasil dibuat."
	case "updated":
		return "Success plan berhasil diperbarui."
	case "deleted":
		return "Success plan berhasil dihapus."
	default:
		return ""
	}
}

// successPlansErrMsg memetakan ?err= → pesan galat form Success Plans.
func successPlansErrMsg(code string) string {
	switch code {
	case "name":
		return "Nama plan wajib diisi."
	case "status":
		return "Status plan tidak valid."
	case "progress":
		return "Progress harus berupa angka 0–100."
	case "account":
		return "Desa tidak ditemukan atau tidak dalam cakupan Anda."
	case "owner":
		return "Owner CS tidak valid."
	case "notfound":
		return "Success plan tidak ditemukan."
	case "failed":
		return "Gagal menyimpan success plan. Coba lagi."
	default:
		return ""
	}
}
