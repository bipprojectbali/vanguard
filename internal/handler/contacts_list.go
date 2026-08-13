package handler

import (
	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// contacts_list.go — helper khusus DAFTAR kontak global: tab cakupan (Semua/Kontak
// Saya) + penurunan flag ownership. Dipisah dari contacts_page.go agar handler
// halaman tetap ramping (file-health) dan aturan "tab mana untuk siapa" berdiri di
// satu tempat. Kembaran accounts_list.go — kontak MEWARISI kepemilikan desa induk,
// jadi flag & cakupannya diturunkan dari sumber yang SAMA (AccountsListFilter),
// tanpa tab "Belum ada Owner" (kontak tak punya kolom owner sendiri).

// normalizeContactsView menormalkan ?view= mentah. showTabs=false (peran bukan
// ScopeAll) → "" : tab tak ada konsepnya, filter cakupan asli yang dipakai. Nilai
// liar → ContactViewAll (fail-ke-default, bukan galat) agar URL yang diutak-atik
// tak menampilkan halaman rusak.
func normalizeContactsView(raw string, showTabs bool) string {
	if !showTabs {
		return ""
	}
	if raw == panel.ContactViewMy {
		return raw
	}
	return panel.ContactViewAll
}

// contactsListParams menurunkan flag ownership ListContacts dari cakupan role + tab
// aktif. Untuk peran ScopeAll, tab MENGGANTIKAN cakupan default:
//   - all → ScopeAll (semua kontak di workspace)
//   - my  → IsOwn (union kepemilikan desa induk aktor) — override walau role ScopeAll
//
// Peran non-ScopeAll (view "") tak punya tab → filter cakupan aslinya
// (AccountsListFilterFor): ScopeOwn → IsOwn; ScopeNone → nol baris (fail-closed).
// IsOwn = union kepemilikan → KEDUA flag SQL (is_sales/is_csm) true.
func contactsListParams(dataScope, view string, uid int64) db.ListContactsParams {
	var scopeAll, isOwn bool
	switch view {
	case panel.ContactViewMy:
		isOwn = true
	case panel.ContactViewAll:
		scopeAll = true
	default: // "" → peran bukan-ScopeAll: pakai cakupan asli role
		f := db.AccountsListFilterFor(dataScope)
		scopeAll, isOwn = f.ScopeAll, f.IsOwn
	}
	return db.ListContactsParams{
		ScopeAll: scopeAll,
		IsSales:  isOwn,
		IsCsm:    isOwn,
		Uid:      &uid,
	}
}
