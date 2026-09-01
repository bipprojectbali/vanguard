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

	shown, nextCursor := splitPage(rows, func(l db.Lead) (pgtype.Timestamptz, int64) {
		return l.CreatedAt, l.ID
	})

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("leads: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	br := session.BusinessRole(ctx)
	items := make([]panel.LeadRow, 0, len(shown))
	for _, l := range shown {
		items = append(items, leadRowView(l, names, br))
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
	}))
}

// renderLeadsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM. Status
// ditulis SEBELUM body agar penolakan tak terkirim sebagai 200 palsu.
func (h *Handler) renderLeadsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Leads", "/leads", panel.SalesForbidden("Leads"))
}

// leadRowView memetakan satu baris daftar + F4 (estimated_value tersamar untuk
// Support). Nama pemilik diresolusi dari peta anggota (bukan id telanjang).
func leadRowView(l db.Lead, names map[int64]string, businessRole string) panel.LeadRow {
	return panel.LeadRow{
		ID:            l.ID,
		EntityCode:    deref(l.EntityCode),
		LeadName:      l.LeadName,
		ContactPerson: deref(l.ContactPerson),
		LeadSource:    deref(l.LeadSource),
		Status:        l.LeadStatus,
		Rating:        deref(l.Rating),
		EstValue:      maskARR(formatRupiah(l.EstimatedValue), businessRole),
		Owner:         ownerName(l.LeadOwner, names),
	}
}
