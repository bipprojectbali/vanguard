package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// sales_deals_page.go — HALAMAN baca Deal: pipeline (KPI + Kanban) / tampilan
// Tabel, & detail. Aksi ada di sales_deals.go. Dipisah karena halaman tumbuh
// dengan aturan LIHAT (F2 read + F3 ownership + F4 masking) & aksi dengan aturan
// TULIS. Meniru sales_leads_page.go/accounts_page.go.

// dealPipelineLimit membatasi kartu yang dimuat papan Kanban (guardrail
// pagination: papan tak boleh memuat seluruh tabel). Deal di luar batas tetap
// terlihat lewat tampilan Tabel berkeyset. Named-const, bukan angka telanjang.
const dealPipelineLimit = 200

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

	stats, err := h.q(ctx).DealPipelineStats(ctx, db.DealPipelineStatsParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
	})
	if err != nil {
		h.Log.Error("deals: stats", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	rows, err := h.q(ctx).ListDealsForPipeline(ctx, db.ListDealsForPipelineParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
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
		Base:          base,
		View:          "",
		CanWrite:      canWriteDeals(ctx),
		Err:           wsErrMsg(r.URL.Query().Get("err")),
		Msg:           dealsMsg(r.URL.Query().Get("ok")),
		OpenCount:     strconv.FormatInt(stats.OpenCount, 10),
		PipelineValue: maskARR(formatRupiah(stats.PipelineValue), br),
		WinRate:       winRate(stats.WonCount, stats.LostCount),
		Stages:        cols,
	}))
}

// dealsTable merender tampilan Tabel berkeyset (alternatif papan). Sama sumber
// ownership; menampilkan lebih banyak kolom & navigasi halaman berikutnya.
func (h *Handler) dealsTable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filter := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)

	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListDeals(ctx, db.ListDealsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		StageFilter:     r.URL.Query().Get("stage"),
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("deals: table", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shown, nextCursor := splitPage(rows, func(d db.Deal) (pgtype.Timestamptz, int64) {
		return d.CreatedAt, d.ID
	})
	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("deals: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items := make([]panel.DealRow, 0, len(shown))
	for _, d := range shown {
		items = append(items, dealRowView(d, names, br))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Deals", "/deals", panel.DealPipeline(panel.DealPipelineView{
		Base:        base,
		View:        "table",
		CanWrite:    canWriteDeals(ctx),
		Err:         wsErrMsg(r.URL.Query().Get("err")),
		Msg:         dealsMsg(r.URL.Query().Get("ok")),
		StageFilter: r.URL.Query().Get("stage"),
		Items:       items,
		NextCursor:  nextCursor,
	}))
}

// DealDetail — GET /w/{workspace}/deals/{id}. Satu deal. Ownership diputuskan di
// sini (filter.Allows) — kembaran per-baris dari list. Di luar cakupan → 404.
// Nama account & kontak utama diresolusi best-effort (satu query masing-masing,
// bukan N+1); gagal → tautan berlabel kode/id, bukan 500.
func (h *Handler) DealDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewDeals(ctx) {
		h.renderDealsForbidden(w, r)
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	d, err := h.q(ctx).GetDeal(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.Log.Error("deals: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	filter := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), d.DealOwner) {
		http.NotFound(w, r)
		return
	}

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("deals: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, d.DealName, "/deals",
		panel.DealDetail(h.dealDetailView(ctx, base, d, names)))
}

// renderDealsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM.
func (h *Handler) renderDealsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Deals", "/deals", panel.SalesForbidden("Deals"))
}

// dealRowView memetakan satu deal → baris/kartu + F4 (amount tersamar untuk
// Support). Owner diresolusi dari peta anggota.
func dealRowView(d db.Deal, names map[int64]string, businessRole string) panel.DealRow {
	return panel.DealRow{
		ID:            d.ID,
		EntityCode:    deref(d.EntityCode),
		DealName:      d.DealName,
		Stage:         d.Stage,
		Amount:        maskARR(formatRupiah(d.Amount), businessRole),
		Probability:   probabilityStr(d.Probability),
		ExpectedClose: dateStr(d.ExpectedCloseDate),
		Owner:         ownerName(d.DealOwner, names),
	}
}

// dealDetailView merakit detail lengkap + F4 (amount tersamar). Nama account &
// kontak utama diresolusi best-effort (baris di luar tenant/terhapus → label
// cadangan, tak menggagalkan halaman).
func (h *Handler) dealDetailView(ctx context.Context, base string, d db.Deal, names map[int64]string) panel.DealDetailView {
	br := session.BusinessRole(ctx)
	accountLabel := h.accountLabel(ctx, d.AccountID)
	contactLabel := ""
	if d.PrimaryContactID != nil {
		contactLabel = h.contactLabel(ctx, *d.PrimaryContactID)
	}
	return panel.DealDetailView{
		Base:             base,
		ID:               d.ID,
		EntityCode:       deref(d.EntityCode),
		DealName:         d.DealName,
		AccountID:        d.AccountID,
		AccountLabel:     accountLabel,
		PrimaryContact:   contactLabel,
		Stage:            d.Stage,
		Stages:           dealStageOptions,
		DealType:         deref(d.DealType),
		Amount:           maskARR(formatRupiah(d.Amount), br),
		Probability:      probabilityStr(d.Probability),
		ExpectedClose:    dateStr(d.ExpectedCloseDate),
		ForecastCategory: deref(d.ForecastCategory),
		NextStep:         deref(d.NextStep),
		SubscriptionTerm: deref(d.SubscriptionTerm),
		Competitor:       deref(d.Competitor),
		WinLossReason:    deref(d.WinLossReason),
		ClosedDate:       dateStr(d.ClosedDate),
		LossNotes:        deref(d.LossNotes),
		Owner:            ownerName(d.DealOwner, names),
		CanWrite:         canWriteDeals(ctx),
	}
}

// accountLabel meresolusi nama desa untuk tautan (kode — nama). Gagal → "Desa
// #<id>" sebagai cadangan (bukan 500): detail deal tetap terbaca.
func (h *Handler) accountLabel(ctx context.Context, id int64) string {
	a, err := h.q(ctx).GetAccount(ctx, id)
	if err != nil {
		return "Desa #" + strconv.FormatInt(id, 10)
	}
	if a.EntityCode != nil && *a.EntityCode != "" {
		return *a.EntityCode + " — " + a.VillageName
	}
	return a.VillageName
}

// contactLabel meresolusi nama kontak utama. Gagal → "Kontak #<id>" cadangan.
func (h *Handler) contactLabel(ctx context.Context, id int64) string {
	c, err := h.q(ctx).GetContact(ctx, id)
	if err != nil {
		return "Kontak #" + strconv.FormatInt(id, 10)
	}
	return contactFullName(c)
}

// winRate = won/(won+lost) sebagai persen bulat. Nol deal tertutup → "—" (tak ada
// dasar hitung; menyajikan 0% akan menyesatkan).
func winRate(won, lost int64) string {
	total := won + lost
	if total == 0 {
		return "—"
	}
	return strconv.FormatInt(won*100/total, 10) + "%"
}
