package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/ui/pages/panel"
)

// cs_impl_tasks_view.go — gerbang F2 (Casbin bisnis) modul Implementation
// Tracker (Modul 6 Customer Success, sub-item Onboarding 6.2.1.1). REUSE
// objek Casbin "crm:journey" (Journey/Onboarding) — BUKAN objek baru, task
// ini sub-fitur onboarding yang sama dgn kolom onboarding_* di
// customer_success (docs/crm/skema.md §6.2.1). Satu tempat objek disebut,
// agar menu sidebar & gate halaman selalu menyebut objek/aksi yang SAMA (nol
// menu hantu). Meniru engagements_view.go.
//
// Akses: Admin (crm:* write), Manager (write), CSM (write), Sales HANYA read
// (business_policy.csv: "p, sales, crm:journey, read" — tanpa write, jadi
// Sales bisa lihat tapi tak bisa buat/ubah task). Support tak punya
// crm:journey sama sekali → fail-closed F2 baik read maupun write.
// F3 ownership ditegakkan lewat CSImplTasksListFilterFor (ownership_cs_onboarding.go).

// canViewImplTasks = gerbang READ daftar task. Sumber tunggal untuk menu &
// gate halaman CSImplTasksList. Admin/Manager/CSM lolos; Sales/Support tidak.
func canViewImplTasks(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:journey", "read")
}

// canWriteImplTasksPerm = izin F2 mentah tulis task (tanpa cek arsip
// workspace). write mencakup read; Admin/Manager/CSM lolos.
func canWriteImplTasksPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:journey", "write")
}

// canWriteImplTasks = tombol tulis di view: izin F2 write DAN workspace tak
// read-only (arsip). Dihitung di handler, dioper bool — view tak panggil authz.
func canWriteImplTasks(ctx context.Context) bool {
	return canWriteImplTasksPerm(ctx) && !IsReadOnly(ctx)
}

// renderImplTasksForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderImplTasksForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Implementation Tracker", "/impl-tasks", panel.SalesForbidden("Implementation Tracker"))
}

// csImplTasksMsg memetakan ?ok= → pesan sukses Implementation Tracker.
func csImplTasksMsg(code string) string {
	switch code {
	case "created":
		return "Task berhasil dibuat."
	case "updated":
		return "Status task diperbarui."
	default:
		return ""
	}
}

// csImplTasksErrMsg memetakan ?err= → pesan galat form Implementation Tracker.
func csImplTasksErrMsg(code string) string {
	switch code {
	case "required":
		return "Desa dan nama task wajib diisi."
	case "status":
		return "Status tidak valid."
	case "account":
		return "Desa tidak ditemukan atau tidak dalam cakupan Anda."
	case "failed":
		return "Gagal menyimpan task. Coba lagi."
	default:
		return ""
	}
}
