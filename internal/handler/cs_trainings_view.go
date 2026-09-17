package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/ui/pages/panel"
)

// cs_trainings_view.go — gerbang F2 (Casbin bisnis) modul Training Schedule
// (Modul 6 Customer Success, sub-item Onboarding 6.2.1.2). Objek Casbin
// "crm:adoption" (BL-169 — sebelumnya baris matriks "Product Adoption" tanpa
// halaman nyata; direlabel jadi gerbang Training Schedule, satu-satunya
// pemakainya). Satu tempat objek disebut, agar menu sidebar & gate halaman
// selalu menyebut objek/aksi yang SAMA.
//
// Akses (default grant crm:adoption, tak diubah BL-169): Admin (crm:* write),
// Manager (write), CSM (write). Sales & Support TAK punya crm:adoption sama
// sekali → fail-closed F2 baik read maupun write (regresi disengaja: Sales
// sebelumnya dapat read via crm:journey).
// F3 ownership ditegakkan lewat CSTrainingsListFilterFor (ownership_cs_onboarding.go).

// canViewTrainings = gerbang READ daftar training. Sumber tunggal untuk menu &
// gate halaman CSTrainingsList. Admin/Manager/CSM lolos; Sales/Support tidak.
func canViewTrainings(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:adoption", "read")
}

// canWriteTrainingsPerm = izin F2 mentah tulis training (tanpa cek arsip
// workspace). write mencakup read; Admin/Manager/CSM lolos, Sales/Support tidak.
func canWriteTrainingsPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:adoption", "write")
}

// canWriteTrainings = tombol tulis di view: izin F2 write DAN workspace tak
// read-only (arsip). Dihitung di handler, dioper bool — view tak panggil authz.
func canWriteTrainings(ctx context.Context) bool {
	return canWriteTrainingsPerm(ctx) && !IsReadOnly(ctx)
}

// renderTrainingsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderTrainingsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Training Schedule", "/trainings", panel.SalesForbidden("Training Schedule"))
}

// csTrainingsMsg memetakan ?ok= → pesan sukses Training Schedule.
func csTrainingsMsg(code string) string {
	switch code {
	case "created":
		return "Jadwal training berhasil dibuat."
	case "updated":
		return "Status training diperbarui."
	default:
		return ""
	}
}

// csTrainingsErrMsg memetakan ?err= → pesan galat form Training Schedule.
func csTrainingsErrMsg(code string) string {
	switch code {
	case "required":
		return "Desa, topik, dan tanggal training wajib diisi."
	case "status":
		return "Status tidak valid."
	case "account":
		return "Desa tidak ditemukan atau tidak dalam cakupan Anda."
	case "attendance":
		return "Nilai attendance tidak valid."
	case "datetime":
		return "Format tanggal training tidak valid."
	case "failed":
		return "Gagal menyimpan jadwal training. Coba lagi."
	default:
		return ""
	}
}
