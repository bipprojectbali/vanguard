package handler

import (
	"net/http"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// cs_journey_export.go — CSV tabel utama Customer Journey (BL-77). Route
// /w/{workspace}/journey/export baca ?stage= (filter fase, sama halaman HTML).
// Serial dari query yang SAMA (ListCSJourneyAccounts) → angka & masking scope
// CSV identik HTML, bukan jalur hitung kedua. F2 & F3 identik jalur HTML.
//
// Berbeda halaman HTML, ekspor mengambil sekaligus sampai csJourneyExportLimit
// (bukan keyset per-halaman): cursor halaman-pertama (firstPageCursorAsc) +
// LIMIT tinggi. Pagar atas mencegah pindai tanpa batas.

// CSJourneyExport — GET /w/{workspace}/journey/export?stage=…
func (h *Handler) CSJourneyExport(w http.ResponseWriter, r *http.Request) {
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

	cursorAt, cursorID := firstPageCursorAsc()
	rows, err := h.q(ctx).ListCSJourneyAccounts(ctx, db.ListCSJourneyAccountsParams{
		CursorAt:    cursorAt,
		CursorID:    cursorID,
		ScopeAll:    filter.ScopeAll,
		IsOwn:       filter.IsOwn,
		Uid:         &uid,
		FilterStage: filterStage,
		PageSize:    csJourneyExportLimit,
	})
	if err != nil {
		h.Log.Error("cs_journey: export list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	headers := []string{
		"Desa", "Fase", "Masuk Fase", "Lama di Fase (hari)",
		"Health", "Onboarding %", "Status Onboarding", "CSM",
	}
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		stageLabel, _ := csJourneyStageBadge(row.LifecycleStage)
		days, known := daysInStage(row.StageEntryDate, today)
		daysStr := "—"
		if known {
			daysStr = strconv.Itoa(days)
		}
		healthLabel := "—"
		if row.HealthStatus != nil && *row.HealthStatus != "" {
			healthLabel, _ = healthScoreStatus(row.HealthStatus)
		}
		progressStr := "—"
		if row.OnboardingProgress != nil {
			progressStr = strconv.Itoa(int(*row.OnboardingProgress))
		}
		out = append(out, []string{
			row.AccountName,
			stageLabel,
			csJourneyDate(row.StageEntryDate),
			daysStr,
			healthLabel,
			progressStr,
			strFromPtr(row.OnboardingStatus),
			strFromPtr(row.CsmName),
		})
	}

	if err := writeCSV(w, "customer-journey", headers, out); err != nil {
		h.Log.Error("cs_journey: export write", "err", err)
	}
}
