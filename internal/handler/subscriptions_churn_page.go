package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_churn_page.go — dasbor Churn (Menu 5.2/5.4, READ-ONLY). Menyorot
// langganan yang telah berhenti (status Cancelled/Churned) + nilai MRR yang hilang.
// Aksi churn SENGAJA tidak di sini — ditandai dari detail langganan (subscriptions_
// churn.go); dasbor ini hanya baca (sejalan wireframe 5.2/5.4).
//
// Gerbang sama dengan daftar langganan: F2 crm:subscriptions read + F3 ownership
// (subscription_owner) di layer query. MRR Hilang = lost_value_mrr, nilai komersial
// → maskARR (F4, diperbaiki audit FLS M9-1). Keyset created_at DESC (reuse
// pageCursor/splitPage).
//
// BL-92: didesain ulang jadi dasbor (permintaan user 7 Sep, wireframe). Tambahan
// banner peringatan (token warning), 4 kartu KPI berjendela TETAP per label mockup
// (bukan tab periode) & SELALU global (tab Tipe hanya menyaring tabel + CSV), kolom
// Tenure (cancellation_date − start_date, bulan), serta ekspor CSV read-only.
// KPI REUSE fungsi baca agregat Report 8.4 (ReportRetention/ReportChurnAge) apa adanya.

// churnTypeTab = satu tab tipe churn (key untuk query + label tampilan). Sumber SATU
// untuk tab view & normalisasi handler; key ” = semua tipe (mencerminkan
// subs_churn_type_chk: Voluntary/Involuntary).
type churnTypeTab struct{ Key, Label string }

var churnTypeTabs = []churnTypeTab{
	{"", "Semua"},
	{"Voluntary", "Sukarela"},
	{"Involuntary", "Terpaksa"},
}

// SubscriptionChurnList — GET /w/{slug}/subscriptions/churn. Dasbor read-only
// langganan berhenti, ter-scope kepemilikan (F3) + filter tipe churn opsional.
func (h *Handler) SubscriptionChurnList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSubscriptions(ctx) {
		h.renderSubscriptionsForbidden(w, r)
		return
	}
	typeFilter := normalizeChurnTypeFilter(r.URL.Query().Get("type"))
	filter := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)

	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListChurned(ctx, db.ListChurnedParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		TypeFilter:      typeFilter,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("subscriptions: churn list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shown, nextCursor := splitPage(rows, func(s db.ListChurnedRow) (pgtype.Timestamptz, int64) {
		return s.CreatedAt, s.ID
	})
	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("subscriptions: churn members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items := make([]panel.ChurnRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, churnRowView(s, names, br))
	}

	// KPI global (semua tipe) — tab Tipe hanya menyaring tabel, bukan kartu (BL-92).
	kpis, err := h.churnKPIs(ctx, filter, uid, br)
	if err != nil {
		h.Log.Error("subscriptions: churn kpis", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Churn", "/subscriptions/churn",
		panel.ChurnList(panel.ChurnView{
			Base:       base,
			Type:       typeFilter,
			Types:      churnTypeFilterOptions(),
			KPIs:       kpis,
			Err:        wsErrMsg(r.URL.Query().Get("err")),
			Items:      items,
			NextCursor: nextCursor,
			After:      r.URL.Query().Get("after"),
			Trail:      pageTrail(r),
		}))
}

// churnKPIs merakit 4 kartu KPI dasbor Churn. Jendela waktu TETAP per label mockup
// (bukan tab periode, keputusan user 7 Sep): Churn Rate & Desa Churn = 30 hari
// terakhir, Churned MRR = bulan berjalan, Avg Tenure = seluruh riwayat (period NULL).
// SELALU global (semua tipe churn) — tab Tipe hanya menyaring tabel + CSV. F3
// ownership diwariskan dari filter/uid. REUSE ReportRetention/ReportChurnAge apa adanya.
func (h *Handler) churnKPIs(ctx context.Context, filter db.SubscriptionsListFilter, uid int64, br string) (panel.ChurnKPIs, error) {
	now := time.Now().In(appTZ)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, appTZ)
	start30 := salesTS(today.AddDate(0, 0, -30))
	endTom := salesTS(today.AddDate(0, 0, 1)) // eksklusif; sertakan churn hari ini
	monStart, monEnd := salesPresetRange(salesPeriodMonth, now)

	// Churn Rate & Desa Churn (30 hari): active = snapshot (tak dibatasi waktu),
	// churned = cancellation_date dalam [start30, endTom).
	ret, err := h.q(ctx).ReportRetention(ctx, db.ReportRetentionParams{
		PeriodStart: start30, PeriodEnd: endTom,
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ChurnKPIs{}, err
	}
	// Churned MRR (bulan berjalan): SUM lost_value_mrr atas kohort churn bulan ini.
	mon, err := h.q(ctx).ReportChurnAge(ctx, db.ReportChurnAgeParams{
		PeriodStart: salesTS(monStart), PeriodEnd: salesTS(monEnd),
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ChurnKPIs{}, err
	}
	// Avg Tenure (seluruh riwayat): period NULL (pgtype zero → sqlc narg NULL).
	life, err := h.q(ctx).ReportChurnAge(ctx, db.ReportChurnAgeParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ChurnKPIs{}, err
	}

	return panel.ChurnKPIs{
		ChurnRate:       ratePct(ret.Churned, ret.Active+ret.Churned),
		ChurnedMRR:      maskARR(formatRupiah(mon.LostValue), br),
		VillagesChurned: strconv.FormatInt(ret.Churned, 10),
		VillagesActive:  strconv.FormatInt(ret.Active, 10),
		AvgTenure:       tenureMonthsStr(life.AvgAgeDays, life.AgedCount),
	}, nil
}

// tenureMonthsStr memformat rata umur (hari, dari ReportChurnAge) → "N bln".
// count ≤ 0 (tak ada kohort ber-tanggal lengkap) → "—". 30.44 = rata hari/bulan.
func tenureMonthsStr(days pgtype.Numeric, count int64) string {
	if count <= 0 {
		return "—"
	}
	f, err := days.Float64Value()
	if err != nil || !f.Valid {
		return "—"
	}
	return strconv.FormatInt(int64(f.Float64/30.44+0.5), 10) + " bln"
}

// subTenureMonthsStr = masa langganan satu baris (cancellation_date − start_date)
// dalam bulan, "N bln". Salah satu tanggal kosong / negatif → "—". Dihitung di
// handler agar view tetap murni-data (BL-92 kolom Tenure).
func subTenureMonthsStr(start, end pgtype.Date) string {
	if !start.Valid || !end.Valid {
		return "—"
	}
	days := end.Time.Sub(start.Time).Hours() / 24
	if days < 0 {
		return "—"
	}
	return strconv.FormatInt(int64(days/30.44+0.5), 10) + " bln"
}

// normalizeChurnTypeFilter memetakan ?type= ke salah satu key sah; nilai asing →
// ” (semua tipe). Key ” sendiri sah (tab "Semua").
func normalizeChurnTypeFilter(v string) string {
	for _, t := range churnTypeTabs {
		if t.Key == v {
			return v
		}
	}
	return ""
}

// churnTypeFilterOptions menyalin daftar tab ke tipe view (handler pemilik enum).
func churnTypeFilterOptions() []panel.ChurnType {
	out := make([]panel.ChurnType, 0, len(churnTypeTabs))
	for _, t := range churnTypeTabs {
		out = append(out, panel.ChurnType{Key: t.Key, Label: t.Label})
	}
	return out
}

// churnRowView memetakan satu baris → baris tabel Churn. MRR Hilang = lost_value_
// mrr, nilai komersial → maskARR (F4). CSM = nama pemilik langganan dari peta
// anggota. Tenure = cancellation_date − start_date (BL-92). Kolom mengikuti
// wireframe 5.2/5.4.
func churnRowView(s db.ListChurnedRow, names map[int64]string, businessRole string) panel.ChurnRow {
	return panel.ChurnRow{
		ID:        s.ID,
		Village:   s.VillageName,
		Plan:      s.PlanName,
		LostMRR:   maskARR(formatRupiah(s.LostValueMrr), businessRole),
		Reason:    deref(s.ChurnReason),
		Type:      deref(s.ChurnType),
		ChurnDate: dateStr(s.CancellationDate),
		Tenure:    subTenureMonthsStr(s.StartDate, s.CancellationDate),
		CSM:       ownerName(s.SubscriptionOwner, names),
	}
}
