package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sla_policies_page.go — HALAMAN baca katalog SLA Policies (Modul 6 slice A1).
// Aksi ada di sla_policies.go / sla_policies_status.go. Dipisah karena halaman
// tumbuh dengan aturan LIHAT (F2 read), aksi dengan aturan TULIS (F2 write).
// Meniru plans_page.go.
//
// Katalog master per-workspace (ListSLAPoliciesAll, TERMASUK pensiun) → keyset
// (created_at DESC, id DESC) lewat ?after=, TANPA F3: RLS satu-satunya pengurung.
//
// KPI kepatuhan SLA (30 hari) & kolom "Kepatuhan" per baris di wireframe
// 6.11 DIHILANGKAN dari slice ini SENGAJA — keduanya dihitung dari data tiket
// yang belum ada (tickets baru mendarat di slice D1/D2). Menyusul begitu
// tiket & kalkulasi live-nya ada (lihat rencana slice D2).

// SLAPoliciesList — GET /w/{workspace}/sla-policies. Seluruh katalog (aktif
// dulu lalu pensiun). Bukan pemegang peran CRM (read) → 403 + penjelasan.
func (h *Handler) SLAPoliciesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSLAPolicies(ctx) {
		h.renderSLAPoliciesForbidden(w, r)
		return
	}
	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListSLAPoliciesAll(ctx, db.ListSLAPoliciesAllParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("sla_policies: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shown, nextCursor := splitPage(rows, func(p db.SlaPolicy) (pgtype.Timestamptz, int64) {
		return p.CreatedAt, p.ID
	})
	items := make([]panel.SLAPolicyRow, 0, len(shown))
	for _, p := range shown {
		items = append(items, slaPolicyRowView(p))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "SLA Management", "/sla-policies", panel.SLAPolicyList(panel.SLAPolicyListView{
		Base:       base,
		CanWrite:   canWriteSLAPolicies(ctx),
		Err:        slaPoliciesErrMsg(r.URL.Query().Get("err")),
		Msg:        slaPoliciesMsg(r.URL.Query().Get("ok")),
		Items:      items,
		NextCursor: nextCursor,
	}))
}

// renderSLAPoliciesForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderSLAPoliciesForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "SLA Management", "/sla-policies", panel.SalesForbidden("SLA Management"))
}

// slaPolicyRowView memetakan satu kebijakan → baris tabel katalog. Target
// respon/selesai diformat "N menit" (nil → "—", ditangani view via orDash).
func slaPolicyRowView(p db.SlaPolicy) panel.SLAPolicyRow {
	return panel.SLAPolicyRow{
		ID:               p.ID,
		SLAName:          p.SlaName,
		Priority:         deref(p.AppliesToPriority),
		FirstResponseMin: int32Str(p.FirstResponseTargetMinutes),
		ResolutionMin:    int32Str(p.ResolutionTargetMinutes),
		BusinessHours:    deref(p.BusinessHours),
		Active:           p.IsActive,
	}
}
