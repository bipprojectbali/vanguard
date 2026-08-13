package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/ui/pages/panel"
)

// tickets_view.go — gerbang F2 (Casbin bisnis) modul Tickets / Cases (Modul 6
// Customer Success, slice B2, wireframe 6.9). Satu tempat objek Casbin
// "crm:tickets" disebut, agar menu sidebar & gate halaman selalu menyebut
// objek/aksi yang SAMA (nol menu hantu). Meniru kb_articles_view.go.
//
// Tickets punya F3 ownership lewat accounts: CSM/Sales lihat tiket desa binaan,
// Support lihat SEMUA meski data_scope='none' (override di TicketsListFilterFor,
// ownership.go). Ini F3 — bukan urusan Casbin; filter query, bukan policy.

// canViewTickets = gerbang READ daftar tiket. Sumber tunggal untuk menu &
// gate halaman TicketsList. Semua peran CRM kecuali platform/blank lolos.
func canViewTickets(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:tickets", "read")
}

// canWriteTicketsPerm = izin F2 mentah tulis tiket (tanpa cek arsip workspace).
// Dipakai gate aksi POST. write mencakup read di Casbin → Support/Manager/Admin
// lolos; Sales/CSM (read-only) ditolak.
func canWriteTicketsPerm(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:tickets", "write")
}

// canWriteTickets = tombol tulis di view: izin F2 write DAN workspace tak
// read-only (arsip). Dihitung di handler, dioper bool — view tak panggil authz.
func canWriteTickets(ctx context.Context) bool {
	return canWriteTicketsPerm(ctx) && !IsReadOnly(ctx)
}

// renderTicketsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderTicketsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Tickets / Cases", "/tickets", panel.SalesForbidden("Tickets / Cases"))
}

// ticketsMsg memetakan ?ok= → pesan sukses Tickets.
func ticketsMsg(code string) string {
	switch code {
	case "created":
		return "Tiket berhasil dibuat."
	case "updated":
		return "Status tiket diperbarui."
	default:
		return ""
	}
}

// ticketsErrMsg memetakan ?err= → pesan galat form Tickets.
func ticketsErrMsg(code string) string {
	switch code {
	case "required":
		return "Subjek dan desa wajib diisi."
	case "priority":
		return "Prioritas harus salah satu: Rendah, Sedang, atau Tinggi."
	case "status":
		return "Status tidak valid."
	case "account":
		return "Desa tidak ditemukan atau tidak dalam cakupan Anda."
	case "failed":
		return "Gagal menyimpan tiket. Coba lagi."
	default:
		return ""
	}
}
