package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// sla_policies_page.go — HALAMAN baca katalog SLA Policies (Modul 6 slice A1).
// Aksi ada di sla_policies.go / sla_policies_status.go. Dipisah karena halaman
// tumbuh dengan aturan LIHAT (F2 read), aksi dengan aturan TULIS (F2 write).
// Meniru plans_page.go.
//
// Katalog master bounded per-workspace (ListSLAPoliciesAll, TERMASUK pensiun)
// → TANPA keyset & TANPA F3: RLS satu-satunya pengurung.
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
	rows, err := h.q(ctx).ListSLAPoliciesAll(ctx)
	if err != nil {
		h.Log.Error("sla_policies: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items := make([]panel.SLAPolicyRow, 0, len(rows))
	for _, p := range rows {
		items = append(items, slaPolicyRowView(p))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "SLA Management", "/sla-policies", panel.SLAPolicyList(panel.SLAPolicyListView{
		Base:     base,
		CanWrite: canWriteSLAPolicies(ctx),
		Err:      slaPoliciesErrMsg(r.URL.Query().Get("err")),
		Msg:      slaPoliciesMsg(r.URL.Query().Get("ok")),
		Items:    items,
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
