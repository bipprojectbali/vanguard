package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/ui/pages/panel"
)

// all_activities_page.go — HALAMAN daftar Activities lintas-context (M7).
// Berbeda dari sales_activities_page.go (context='sales' saja), halaman ini
// menampilkan SEMUA aktivitas (sales+cs+general) — cermin menu "Activities"
// top-level di sidebar (di luar grup Sales).
//
// Gate: canViewAllActivities (crm:activities read — objek global lintas-context,
// BUKAN crm:sales_activity). csm/support yang memegang crm:activities kini bisa
// membaca tampilan ini; Sales Activities tetap di gate crm:sales_activity (BL-39).

// AllActivitiesList — GET /w/{workspace}/activity-log. Linimasa TERPADU (BL-41):
// activities (Sales/umum) + engagements (CS) dalam satu daftar berkeyset lintas-
// tabel + F3 ownership per-sumber. Gate halaman: canViewAllActivities (CRM role
// ATAU platform role). Lengan CS di-gate LAGI di dalam feed (canViewEngagements)
// agar Sales/Support tak melihat baris CS — lihat buildUnifiedActivityFeed.
func (h *Handler) AllActivitiesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewAllActivities(ctx) {
		h.renderAllActivitiesForbidden(w, r)
		return
	}

	// q = pencarian bebas (BL-6): MEMPERSEMPIT subject di atas F3, tak melebarkan.
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("all-activities: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items, nextCursor, err := h.buildUnifiedActivityFeed(ctx, r, names)
	if err != nil {
		h.Log.Error("all-activities: feed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Aktivitas", "/activity-log",
		panel.AllActivitiesList(panel.AllActivitiesListView{
			Base:       base,
			CanWrite:   canWriteSalesActivity(ctx),
			Err:        wsErrMsg(r.URL.Query().Get("err")),
			Msg:        activitiesMsg(r.URL.Query().Get("ok")),
			Items:      items,
			NextCursor: nextCursor,
			After:      r.URL.Query().Get("after"),
			Trail:      pageTrail(r),
			Query:      query,
		}))
}

// renderAllActivitiesForbidden — 403 + penjelasan bagi anggota tanpa izin
// (bukan platform role, dan tidak punya crm:activities read).
func (h *Handler) renderAllActivitiesForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Aktivitas", "/activity-log",
		panel.SalesForbidden("Aktivitas"))
}
