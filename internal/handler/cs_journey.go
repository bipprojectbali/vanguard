package handler

import (
	"net/http"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// cs_journey.go — HALAMAN baca Customer Journey / Lifecycle (Modul 6 Customer
// Success, 6.2 — BL-77). Dashboard portofolio posisi tiap desa di sepanjang
// fase Onboarding → Adoption → Retention → Renewal → Advocacy. Read-only:
// tak ada aksi tulis di sini (tombol "+ Mulai Onboarding" hanya menautkan ke
// daftar Accounts).
//
// Akses lewat tiga sumbu orthogonal (pola cs_impl_tasks.go):
//   - F2 (Casbin bisnis): canReadCSJourney — REUSE objek "crm:journey".
//     Support tak punya objek ini → fail-closed. Sales hanya read.
//   - F3 (ownership): CSJourneyListFilterFor(dataScope) — Admin/Manager
//     (ScopeAll) lihat semua desa; CSM (ScopeOwn) lihat desa binaannya.
//   - RLS: h.q ber-tenant → mengisolasi workspace di bawah semua ini.

// CSJourneyList — GET /w/{workspace}/journey. 4 KPI + funnel fase + panel
// onboarding aktif + tabel utama posisi lifecycle. canReadCSJourney=false → 403.
func (h *Handler) CSJourneyList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canReadCSJourney(ctx) {
		h.renderJourneyForbidden(w, r)
		return
	}

	dataScope := session.BusinessDataScope(ctx)
	filter := db.CSJourneyListFilterFor(dataScope)
	uid := session.UserID(ctx)
	today := time.Now().In(appTZ)

	filterStage := csJourneyFilterStage(r.URL.Query().Get("stage"))
	cursorAt, cursorID := pageCursorAsc(r)

	kpis, err := h.q(ctx).CountCSJourneyKPIs(ctx, db.CountCSJourneyKPIsParams{
		ScopeAll: filter.ScopeAll,
		IsOwn:    filter.IsOwn,
		Uid:      &uid,
	})
	if err != nil {
		h.Log.Error("cs_journey: count kpis", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	phases, err := h.q(ctx).ListCSJourneyPhases(ctx, db.ListCSJourneyPhasesParams{
		StalledDays: csJourneyStalledDays,
		ScopeAll:    filter.ScopeAll,
		IsOwn:       filter.IsOwn,
		Uid:         &uid,
	})
	if err != nil {
		h.Log.Error("cs_journey: list phases", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	onboarding, err := h.q(ctx).ListCSJourneyOnboarding(ctx, db.ListCSJourneyOnboardingParams{
		ScopeAll: filter.ScopeAll,
		IsOwn:    filter.IsOwn,
		Uid:      &uid,
		Lim:      csJourneyOnboardingLimit,
	})
	if err != nil {
		h.Log.Error("cs_journey: list onboarding", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows, err := h.q(ctx).ListCSJourneyAccounts(ctx, db.ListCSJourneyAccountsParams{
		CursorAt:    cursorAt,
		CursorID:    cursorID,
		ScopeAll:    filter.ScopeAll,
		IsOwn:       filter.IsOwn,
		Uid:         &uid,
		FilterStage: filterStage,
		PageSize:    pageSize + 1,
	})
	if err != nil {
		h.Log.Error("cs_journey: list accounts", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	shown, nextCursor := splitPage(rows, csJourneyKeyOf)

	slug := slugFromRequest(r)
	base := wsPath(slug, "")

	onboardRows := make([]panel.CSJourneyOnboardRow, 0, len(onboarding))
	for _, o := range onboarding {
		onboardRows = append(onboardRows, csJourneyOnboardRowView(o, slug, today))
	}
	items := make([]panel.CSJourneyAccountRow, 0, len(shown))
	for _, row := range shown {
		items = append(items, csJourneyAccountRowView(row, slug, today))
	}

	h.renderWorkspaceShell(w, r, "Customer Journey", "/journey", panel.CSJourneyList(panel.CSJourneyListView{
		Base:         base,
		KPIs:         csJourneyKPIView(kpis),
		Phases:       csJourneyPhaseViews(phases),
		Onboarding:   onboardRows,
		Items:        items,
		StageOptions: lifecycleStageOptions,
		FilterStage:  filterStage,
		CanWrite:     canWriteCSJourney(ctx) && !IsReadOnly(ctx),
		NextCursor:   nextCursor,
		After:        r.URL.Query().Get("after"),
		Trail:        pageTrail(r),
	}))
}

// renderJourneyForbidden — 403 + penjelasan bagi anggota tanpa peran CRM yang
// memuat crm:journey read (mis. Support). Meniru renderImplTasksForbidden.
func (h *Handler) renderJourneyForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Customer Journey", "/journey", panel.SalesForbidden("Customer Journey"))
}
