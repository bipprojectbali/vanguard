package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_leads_page.go — HALAMAN daftar Lead (dengan tab) + gerbang forbidden
// bersama. Detail ada di sales_leads_detail.go (dipecah lebih lanjut untuk file
// health, package sama). Aksi ada di sales_leads.go/sales_leads_update.go/
// sales_leads_helpers.go. Dipisah karena keduanya tumbuh dengan aturan berbeda:
// halaman soal APA yang boleh DILIHAT & oleh SIAPA (F2 read + F3 ownership + F4
// masking), aksi soal apa yang boleh DIUBAH (F2 write). Meniru accounts_page.go.

// leadTab menerjemahkan ?tab= → dua sumbu filter ListLeads yang ORTOGONAL dari
// ownership: mine_only (paksa lead_owner=uid walau aktor ScopeAll) & status_filter.
// Tab default "" = semua yang boleh dilihat aktor dalam cakupan F3-nya.
func leadTab(tab string) (mineOnly bool, statusFilter string) {
	switch tab {
	case "my":
		return true, ""
	case "unqualified":
		return false, "Unqualified"
	default: // "all" / ""
		return false, ""
	}
}

// LeadsList — GET /w/{workspace}/leads. Daftar lead, keyset + filter kepemilikan
// F3 + tab. Bukan pemegang peran CRM → 403 + penjelasan (BUKAN 404: penerimanya
// terbukti anggota workspace).
func (h *Handler) LeadsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewLeads(ctx) {
		h.renderLeadsForbidden(w, r)
		return
	}

	filter := db.LeadsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	tab := r.URL.Query().Get("tab")
	mineOnly, statusFilter := leadTab(tab)
	// q = pencarian bebas (BL-6): MEMPERSEMPIT di atas F3+tab, tak melebarkan.
	// Trim agar spasi belaka ≡ tak mencari (query kosong lolos predikat SQL).
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	// sort/dir (BL-157b): whitelist 7 kolom sortable (leadSortableColumns).
	// Kombinasi tak dikenal → jatuh ke jalur default (created_at DESC), TAK
	// error — URL lama/disunting tetap render halaman yang benar.
	sortCol := r.URL.Query().Get("sort")
	if !leadSortableColumns[sortCol] {
		sortCol = ""
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("leads: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	canARR := canSeeARR(ctx)

	var shown []db.Lead
	var nextCursor string
	switch sortCol {
	case "name":
		cursorVal, cursorSortID, hasCursor := pageCursorText(r)
		rows, err := h.q(ctx).ListLeadsSortByName(ctx, db.ListLeadsSortByNameParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			MineOnly:     mineOnly,
			StatusFilter: statusFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("leads: list sort name", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPageText(rows, func(l db.Lead) (string, int64) {
			return l.LeadName, l.ID
		})
	case "status":
		// Status diurut RAW enum (New/Contacted/…, alfabetis) — keputusan user,
		// tak menduplikasi urutan tingkat ke SQL.
		cursorVal, cursorSortID, hasCursor := pageCursorText(r)
		rows, err := h.q(ctx).ListLeadsSortByStatus(ctx, db.ListLeadsSortByStatusParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			MineOnly:     mineOnly,
			StatusFilter: statusFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("leads: list sort status", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPageText(rows, func(l db.Lead) (string, int64) {
			return l.LeadStatus, l.ID
		})
	case "code":
		cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		rows, err := h.q(ctx).ListLeadsSortByCode(ctx, db.ListLeadsSortByCodeParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			MineOnly:     mineOnly,
			StatusFilter: statusFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("leads: list sort code", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPageTextNullable(rows, func(l db.Lead) (string, int64, bool) {
			if l.EntityCode == nil {
				return "", l.ID, true
			}
			return *l.EntityCode, l.ID, false
		})
	case "source":
		cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		rows, err := h.q(ctx).ListLeadsSortBySource(ctx, db.ListLeadsSortBySourceParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			MineOnly:     mineOnly,
			StatusFilter: statusFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("leads: list sort source", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPageTextNullable(rows, func(l db.Lead) (string, int64, bool) {
			if l.LeadSource == nil {
				return "", l.ID, true
			}
			return *l.LeadSource, l.ID, false
		})
	case "rating":
		cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		rows, err := h.q(ctx).ListLeadsSortByRating(ctx, db.ListLeadsSortByRatingParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			MineOnly:     mineOnly,
			StatusFilter: statusFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("leads: list sort rating", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPageTextNullable(rows, func(l db.Lead) (string, int64, bool) {
			if l.Rating == nil {
				return "", l.ID, true
			}
			return *l.Rating, l.ID, false
		})
	case "value":
		cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		// cursor Estimasi kanonik = teks desimal apa adanya (optNumeric ada di
		// sales_format_number.go) — urutan SUNGGUHAN terjadi di SQL atas
		// cursor_val bertipe numeric asli, mirror MRR Subscriptions.
		cursorVal, code := optNumeric(cursorRaw, "")
		if code != "" {
			hasCursor = false
		}
		rows, err := h.q(ctx).ListLeadsSortByValue(ctx, db.ListLeadsSortByValueParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			MineOnly:     mineOnly,
			StatusFilter: statusFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("leads: list sort value", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPageTextNullable(rows, func(l db.Lead) (string, int64, bool) {
			if !l.EstimatedValue.Valid {
				return "", l.ID, true
			}
			return numericStr(l.EstimatedValue), l.ID, false
		})
	case "owner":
		cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		rows, err := h.q(ctx).ListLeadsSortByOwner(ctx, db.ListLeadsSortByOwnerParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     filter.ScopeAll,
			IsOwn:        filter.IsOwn,
			Uid:          &uid,
			MineOnly:     mineOnly,
			StatusFilter: statusFilter,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("leads: list sort owner", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// Kunci cursor Pemilik = nama/email resolusi peta anggota (ownerName),
		// PERSIS yang ditampilkan — bukan owner mentah. NULL-nya mengikuti
		// lead_owner asli (bukan string kosong hasil ownerName), sama dengan
		// kondisi NULL di kunci sort SQL (LEFT JOIN users).
		shown, nextCursor = splitPageTextNullable(rows, func(l db.Lead) (string, int64, bool) {
			if l.LeadOwner == nil {
				return "", l.ID, true
			}
			return ownerName(l.LeadOwner, names), l.ID, false
		})
	default:
		cursorAt, cursorID := pageCursor(r)
		rows, err := h.q(ctx).ListLeads(ctx, db.ListLeadsParams{
			CursorCreatedAt: cursorAt,
			CursorID:        cursorID,
			ScopeAll:        filter.ScopeAll,
			IsOwn:           filter.IsOwn,
			Uid:             &uid,
			MineOnly:        mineOnly,
			StatusFilter:    statusFilter,
			Search:          query,
			PageSize:        pageSize + 1,
		})
		if err != nil {
			h.Log.Error("leads: list", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nextCursor = splitPage(rows, func(l db.Lead) (pgtype.Timestamptz, int64) {
			return l.CreatedAt, l.ID
		})
	}

	items := make([]panel.LeadRow, 0, len(shown))
	for _, l := range shown {
		items = append(items, leadRowView(l, names, canARR))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Leads", "/leads", panel.LeadsList(panel.LeadsListView{
		Base:       base,
		Items:      items,
		Tab:        tab,
		Query:      query,
		CanWrite:   canWriteLeads(ctx),
		HideMyTab:  filter.IsOwn, // BL-1: cakupan 'own' → "Semua" ≡ "Lead Saya"
		NextCursor: nextCursor,
		After:      r.URL.Query().Get("after"),
		Trail:      pageTrail(r),
		Err:        wsErrMsg(r.URL.Query().Get("err")),
		Msg:        leadsMsg(r.URL.Query().Get("ok")),
		Sort:       sortCol,
		Dir:        dir,
	}))
}

// leadSortableColumns = whitelist kolom yang boleh diminta lewat ?sort= (BL-157b:
// 7 kolom tabel Leads). ?sort= di luar daftar ini diperlakukan seolah absen
// (jatuh ke default created_at DESC), TAK error.
var leadSortableColumns = map[string]bool{
	"code":   true,
	"name":   true,
	"source": true,
	"status": true,
	"rating": true,
	"value":  true,
	"owner":  true,
}

// renderLeadsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM. Status
// ditulis SEBELUM body agar penolakan tak terkirim sebagai 200 palsu.
func (h *Handler) renderLeadsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Leads", "/leads", panel.SalesForbidden("Leads"))
}

// leadRowView memetakan satu baris daftar + F4 (estimated_value tersamar bagi
// pemanggil tanpa kapabilitas crm:subscriptions/arr, BL-169). Nama pemilik
// diresolusi dari peta anggota (bukan id telanjang). canARR dihitung SEKALI
// oleh pemanggil (canSeeARR(ctx)).
func leadRowView(l db.Lead, names map[int64]string, canARR bool) panel.LeadRow {
	return panel.LeadRow{
		ID:            l.ID,
		EntityCode:    deref(l.EntityCode),
		LeadName:      l.LeadName,
		ContactPerson: deref(l.ContactPerson),
		LeadSource:    deref(l.LeadSource),
		Status:        l.LeadStatus,
		Rating:        deref(l.Rating),
		EstValue:      maskARR(formatRupiah(l.EstimatedValue), canARR),
		Owner:         ownerName(l.LeadOwner, names),
	}
}
