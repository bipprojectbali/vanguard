package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_deals_page.go — HALAMAN daftar Deal: pipeline (KPI + Kanban) & tampilan
// Tabel. Detail satu deal ada di sales_deals_detail.go — dipisah karena keduanya
// tumbuh dengan aturan sendiri (list berkeyset/bucket vs detail+resolusi label).
// Aksi ada di sales_deals.go dkk. Meniru sales_leads_page.go/accounts_page.go.

// dealPipelineLimit membatasi kartu yang dimuat papan Kanban (guardrail
// pagination: papan tak boleh memuat seluruh tabel). Deal di luar batas tetap
// terlihat lewat tampilan Tabel berkeyset.
const dealPipelineLimit = 200

// dealMineOnly menerjemahkan ?mine=1 → sumbu filter kepemilikan ORTOGONAL dari
// F3: paksa deal_owner = uid walau aktor ScopeAll (toggle "Deal Saya", BL-10).
// Cermin leadTab (sales_leads_page.go) tapi sumbu tunggal — Deal tak punya tab
// status/stage berbasis tab. Toggle hanya bermakna bagi ScopeAll (cakupan 'own'
// sudah otomatis milik sendiri → redundan; gate tampil di handler, lihat DealsList*).
func dealMineOnly(r *http.Request) bool {
	return r.URL.Query().Get("mine") == "1"
}

// DealsList — GET /w/{workspace}/deals. Default = pipeline (KPI + Kanban per-stage);
// ?view=table = tampilan Tabel berkeyset. Bukan pemegang peran CRM → 403 + penjelasan.
func (h *Handler) DealsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewDeals(ctx) {
		h.renderDealsForbidden(w, r)
		return
	}
	if r.URL.Query().Get("view") == "table" {
		h.dealsTable(w, r)
		return
	}
	h.dealsPipeline(w, r)
}

// dealsPipeline merender papan Kanban + KPI. Satu query KPI (DealPipelineStats) +
// satu query kartu (ListDealsForPipeline) di-bucket per-stage di Go (bukan N query
// per kolom). F4: nilai ARR (amount & pipeline value) disamarkan untuk Support.
func (h *Handler) dealsPipeline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filter := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)
	mineOnly := dealMineOnly(r)

	stats, err := h.q(ctx).DealPipelineStats(ctx, db.DealPipelineStatsParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid, MineOnly: mineOnly,
	})
	if err != nil {
		h.Log.Error("deals: stats", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	rows, err := h.q(ctx).ListDealsForPipeline(ctx, db.ListDealsForPipelineParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid, MineOnly: mineOnly,
		PageSize: dealPipelineLimit,
	})
	if err != nil {
		h.Log.Error("deals: pipeline", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("deals: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Bucket per-stage mengikuti urutan pipeline (dealStageOptions). Deal dengan
	// stage tak dikenal (mustahil lewat CHECK) diabaikan diam-diam agar papan tetap rapi.
	buckets := make(map[string][]panel.DealRow, len(dealStageOptions))
	for _, d := range rows {
		buckets[d.Stage] = append(buckets[d.Stage], dealRowView(d, names, br))
	}
	cols := make([]panel.DealStageColumn, 0, len(dealStageOptions))
	for _, s := range dealStageOptions {
		cols = append(cols, panel.DealStageColumn{Stage: s, Cards: buckets[s]})
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Deals", "/deals", panel.DealPipeline(panel.DealPipelineView{
		Base:           base,
		View:           "",
		CanWrite:       canWriteDeals(ctx),
		Mine:           mineOnly,
		ShowMineToggle: filter.ScopeAll, // BL-10: 'own' sudah milik sendiri → redundan
		Err:            wsErrMsg(r.URL.Query().Get("err")),
		Msg:            dealsMsg(r.URL.Query().Get("ok")),
		OpenCount:      strconv.FormatInt(stats.OpenCount, 10),
		PipelineValue:  maskARR(formatRupiah(stats.PipelineValue), br),
		WinRate:        winRate(stats.WonCount, stats.LostCount),
		Stages:         cols,
	}))
}
