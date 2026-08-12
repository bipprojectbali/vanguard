package handler

import (
	"context"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// accounts_list.go — helper khusus DAFTAR desa: tab cakupan (All/My/Belum-ada-
// Owner) + resolusi nama Owner/CSM. Dipisah dari accounts_page.go agar handler
// halaman tetap ramping (file-health) dan aturan "tab mana untuk siapa" berdiri
// di satu tempat, tegak lurus dari gerbang F2 & pemetaan view (accounts_view.go).

// normalizeAccountsView menormalkan ?view= mentah. showTabs=false (peran bukan
// ScopeAll) → "" : tab tak ada konsepnya, filter cakupan asli yang dipakai.
// Nilai liar → AccViewAll (fail-ke-default, bukan galat) agar URL yang diutak-atik
// tak pernah menampilkan halaman rusak.
func normalizeAccountsView(raw string, showTabs bool) string {
	if !showTabs {
		return ""
	}
	switch raw {
	case panel.AccViewMy, panel.AccViewUnowned:
		return raw
	default:
		return panel.AccViewAll
	}
}

// accountsListParams menurunkan flag ownership ListAccounts dari cakupan role +
// tab aktif. Untuk peran ScopeAll, tab MENGGANTIKAN cakupan default:
//   - all     → ScopeAll (semua desa di workspace)
//   - my      → IsOwn (union kepemilikan aktor) — override walau role ScopeAll
//   - unowned → ScopeAll + Unowned (account_owner IS NULL)
//
// Peran non-ScopeAll (view "") tak punya tab → filter cakupan aslinya
// (AccountsListFilterFor): ScopeOwn → IsOwn; ScopeNone → nol baris (fail-closed).
// IsOwn = union kepemilikan → KEDUA flag SQL (is_sales/is_csm) true (lihat
// ownership.go). uid tetap dioper walau diabaikan saat ScopeAll.
func accountsListParams(dataScope, view string, uid int64) db.ListAccountsParams {
	var scopeAll, isOwn, unowned bool
	switch view {
	case panel.AccViewMy:
		isOwn = true
	case panel.AccViewUnowned:
		scopeAll, unowned = true, true
	case panel.AccViewAll:
		scopeAll = true
	default: // "" → peran bukan-ScopeAll: pakai cakupan asli role
		f := db.AccountsListFilterFor(dataScope)
		scopeAll, isOwn = f.ScopeAll, f.IsOwn
	}
	return db.ListAccountsParams{
		ScopeAll: scopeAll,
		IsSales:  isOwn,
		IsCsm:    isOwn,
		Unowned:  unowned,
		Uid:      &uid,
	}
}

// accountMemberNames memetakan user_id → nama tampil (nama, jatuh ke email) untuk
// SELURUH anggota workspace, sekali per muat halaman (bukan N+1 per baris). Owner/
// CSM sebuah desa selalu anggota workspace saat ditugaskan; yang tak lagi anggota
// (mis. keluar) tak ketemu → handler menjatuhkannya ke "" → dirender "—".
func (h *Handler) accountMemberNames(ctx context.Context) (map[int64]string, error) {
	rows, err := h.q(ctx).ListMembersByTenant(ctx, session.TenantID(ctx))
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(rows))
	for _, m := range rows {
		label := m.Email
		if m.Name != nil && *m.Name != "" {
			label = *m.Name
		}
		names[m.UserID] = label
	}
	return names, nil
}

// memberName meresolusi id opsional → nama; nil / tak-dikenal → "" (view: "—").
func memberName(names map[int64]string, id *int64) string {
	if id == nil {
		return ""
	}
	return names[*id]
}
