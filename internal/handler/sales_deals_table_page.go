package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_deals_table_page.go — tampilan Tabel berkeyset daftar Deal (dealsTable),
// dipisah dari sales_deals_page.go (pipeline/Kanban + KPI) demi ambang tipe
// Route/Handler (150). Package sama; sumber ownership & F4 identik.

// dealsTable merender tampilan Tabel berkeyset (alternatif papan). Sama sumber
// ownership; menampilkan lebih banyak kolom & navigasi halaman berikutnya.
func (h *Handler) dealsTable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filter := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	canARR := canSeeARR(ctx)

	// q = pencarian bebas (BL-6): MEMPERSEMPIT di atas F3+stage, tak melebarkan.
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	mineOnly := dealMineOnly(r)
	stageFilter := r.URL.Query().Get("stage")

	// sort/dir (BL-157e): whitelist 7 kolom sortable (dealSortableColumns).
	// Kombinasi tak dikenal → jatuh ke jalur default (created_at DESC), TAK
	// error — URL lama/disunting tetap render halaman yang benar.
	sortCol := r.URL.Query().Get("sort")
	if !dealSortableColumns[sortCol] {
		sortCol = ""
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("deals: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var shown []db.Deal
	var nextCursor string
	switch sortCol {
	case "name":
		cursorVal, cursorSortID, hasCursor := pageCursorText(r)
		rows, err := h.q(ctx).ListDealsSortByName(ctx, db.ListDealsSortByNameParams{
			HasCursor:   hasCursor,
			Dir:         dir,
			CursorVal:   cursorVal,
			CursorID:    cursorSortID,
			ScopeAll:    filter.ScopeAll,
			IsOwn:       filter.IsOwn,
			Uid:         &uid,
			MineOnly:    mineOnly,
			StageFilter: stageFilter,
			Search:      query,
			PageSize:    pageSize + 1,
		})
		if err != nil {
			h.Log.Error("deals: list sort name", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPageText(rows, func(d db.Deal) (string, int64) {
			return d.DealName, d.ID
		})
	case "stage":
		// Tahap diurut RAW enum (Prospecting/Qualification/…, alfabetis) —
		// keputusan user, tak menduplikasi urutan pipeline ke SQL.
		cursorVal, cursorSortID, hasCursor := pageCursorText(r)
		rows, err := h.q(ctx).ListDealsSortByStage(ctx, db.ListDealsSortByStageParams{
			HasCursor:   hasCursor,
			Dir:         dir,
			CursorVal:   cursorVal,
			CursorID:    cursorSortID,
			ScopeAll:    filter.ScopeAll,
			IsOwn:       filter.IsOwn,
			Uid:         &uid,
			MineOnly:    mineOnly,
			StageFilter: stageFilter,
			Search:      query,
			PageSize:    pageSize + 1,
		})
		if err != nil {
			h.Log.Error("deals: list sort stage", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPageText(rows, func(d db.Deal) (string, int64) {
			return d.Stage, d.ID
		})
	case "code":
		cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		rows, err := h.q(ctx).ListDealsSortByCode(ctx, db.ListDealsSortByCodeParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			MineOnly:     mineOnly,
			StageFilter:  stageFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("deals: list sort code", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPageTextNullable(rows, func(d db.Deal) (string, int64, bool) {
			if d.EntityCode == nil {
				return "", d.ID, true
			}
			return *d.EntityCode, d.ID, false
		})
	case "amount":
		cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		// cursor Nilai kanonik = teks desimal apa adanya (optNumeric ada di
		// sales_format_number.go) — urutan SUNGGUHAN terjadi di SQL atas
		// cursor_val bertipe numeric asli, mirror Estimasi Leads.
		cursorVal, code := optNumeric(cursorRaw, "")
		if code != "" {
			hasCursor = false
		}
		rows, err := h.q(ctx).ListDealsSortByAmount(ctx, db.ListDealsSortByAmountParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			MineOnly:     mineOnly,
			StageFilter:  stageFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("deals: list sort amount", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPageTextNullable(rows, func(d db.Deal) (string, int64, bool) {
			if !d.Amount.Valid {
				return "", d.ID, true
			}
			return numericStr(d.Amount), d.ID, false
		})
	case "probability":
		cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		// cursor Peluang kanonik = teks integer apa adanya; urutan SUNGGUHAN
		// terjadi di SQL atas cursor_val bertipe smallint asli.
		var cursorVal int16
		if hasCursor && !isNull {
			n, perr := strconv.ParseInt(cursorRaw, 10, 16)
			if perr != nil {
				hasCursor = false
			} else {
				cursorVal = int16(n)
			}
		}
		rows, err := h.q(ctx).ListDealsSortByProbability(ctx, db.ListDealsSortByProbabilityParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			MineOnly:     mineOnly,
			StageFilter:  stageFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("deals: list sort probability", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPageTextNullable(rows, func(d db.Deal) (string, int64, bool) {
			if d.Probability == nil {
				return "", d.ID, true
			}
			return strconv.FormatInt(int64(*d.Probability), 10), d.ID, false
		})
	case "close":
		cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		// cursor Perkiraan Tutup kanonik = ISO tanggal apa adanya (optDate ada
		// di sales_format.go), mirror Renewal Date Subscriptions.
		cursorVal, code := optDate(cursorRaw)
		if code != "" {
			hasCursor = false
		}
		rows, err := h.q(ctx).ListDealsSortByCloseDate(ctx, db.ListDealsSortByCloseDateParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			MineOnly:     mineOnly,
			StageFilter:  stageFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("deals: list sort close date", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPageTextNullable(rows, func(d db.Deal) (string, int64, bool) {
			if !d.ExpectedCloseDate.Valid {
				return "", d.ID, true
			}
			return dateStr(d.ExpectedCloseDate), d.ID, false
		})
	case "owner":
		cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		rows, err := h.q(ctx).ListDealsSortByOwner(ctx, db.ListDealsSortByOwnerParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			MineOnly:     mineOnly,
			StageFilter:  stageFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("deals: list sort owner", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// Kunci cursor Pemilik = nama/email resolusi peta anggota (ownerName),
		// PERSIS yang ditampilkan — bukan owner mentah. NULL-nya mengikuti
		// deal_owner asli (bukan string kosong hasil ownerName), sama dengan
		// kondisi NULL di kunci sort SQL (LEFT JOIN users).
		shown, nextCursor = splitPageTextNullable(rows, func(d db.Deal) (string, int64, bool) {
			if d.DealOwner == nil {
				return "", d.ID, true
			}
			return ownerName(d.DealOwner, names), d.ID, false
		})
	default:
		cursorAt, cursorID := pageCursor(r)
		rows, err := h.q(ctx).ListDeals(ctx, db.ListDealsParams{
			CursorCreatedAt: cursorAt,
			CursorID:        cursorID,
			ScopeAll:        filter.ScopeAll,
			IsOwn:           filter.IsOwn,
			Uid:             &uid,
			MineOnly:        mineOnly,
			StageFilter:     stageFilter,
			Search:          query,
			PageSize:        pageSize + 1,
		})
		if err != nil {
			h.Log.Error("deals: table", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPage(rows, func(d db.Deal) (pgtype.Timestamptz, int64) {
			return d.CreatedAt, d.ID
		})
	}

	items := make([]panel.DealRow, 0, len(shown))
	for _, d := range shown {
		items = append(items, dealRowView(d, names, canARR))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Deals", "/deals", panel.DealPipeline(panel.DealPipelineView{
		Base:           base,
		View:           "table",
		CanWrite:       canWriteDeals(ctx),
		Mine:           mineOnly,
		ShowMineToggle: filter.ScopeAll, // BL-10: 'own' sudah milik sendiri → redundan
		Err:            wsErrMsg(r.URL.Query().Get("err")),
		Msg:            dealsMsg(r.URL.Query().Get("ok")),
		StageFilter:    stageFilter,
		Query:          query,
		Items:          items,
		NextCursor:     nextCursor,
		After:          r.URL.Query().Get("after"),
		Trail:          pageTrail(r),
		Sort:           sortCol,
		Dir:            dir,
	}))
}

// dealSortableColumns = whitelist kolom yang boleh diminta lewat ?sort= (BL-157e:
// 7 kolom tabel Deals). ?sort= di luar daftar ini diperlakukan seolah absen
// (jatuh ke default created_at DESC), TAK error.
var dealSortableColumns = map[string]bool{
	"code":        true,
	"name":        true,
	"stage":       true,
	"amount":      true,
	"probability": true,
	"close":       true,
	"owner":       true,
}
