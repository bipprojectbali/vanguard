package handler

import (
	"net/http"

	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// members_page.go — HALAMAN daftar anggota (baca). Dipisah dari members.go yang
// berisi AKSI (ubah role, keluarkan): halaman ini punya aturan sendiri soal apa
// yang boleh DILIHAT, dan aturan itu tumbuh bersama kebijakan privasi — bukan
// bersama daftar aksinya.
//
// Perakitan opsi peran CRM, baris anggota, dan baris undangan (murni pemetaan
// data → panel.*Row/Option, rasional detail di masing-masing fungsi) dipisah
// ke members_page_view.go agar file ini di bawah ambang tipe Route/Handler
// (150).

// MembersPage — GET /w/{workspace}/members. Daftar anggota + undangan pending.
//
// PENGELOLA (owner/admin/platform) ATAU role kustom ber-akses "User Management"
// (crm:members, BL-171) — ini MEMBALIK 0004, yang membuka halaman ini untuk
// SEMUA anggota dengan alasan "tahu siapa yang punya akses adalah bagian dari
// mempercayai ruang bersama". Alasan itu benar untuk ruang kerja kecil yang
// saling kenal, tapi daftar anggota adalah DIREKTORI ORANG: ia mengumpulkan
// nama, wajah, dan keanggotaan setiap orang di satu tempat yang bisa disalin
// sekaligus. BL-171 MELEBARKAN (bukan mengganti) 0004: role kustom yang diberi
// Lihat/Kelola pada "User Management" kini juga dianggap "mengelola
// keanggotaan", tapi dibatasi actorKindScope — hanya melihat/mengelola anggota
// berjenis (internal/eksternal) yang cakupannya izinkan (member_scope_policies,
// diatur di /roles). Perubahan role TENANT (member/admin/owner) tetap TERKUNCI
// ke canManageMembers — lihat member_access.go.
//
// Email TAK disamarkan (maskEmail dicabut dari jalur ini): gerbang halaman
// (canViewMembers) sudah mempersempit penglihat ke pengelola tenant ATAU role
// bisnis ber-akses "User Management", jadi siapa pun yang sampai ke sini sudah
// dipercaya melihat direktori penuh — menyamarkannya lagi tak melindungi
// siapa pun, cuma menyembunyikan pembeda yang justru dicari.
//
// PENANDA ORANG = NAMA, lalu email sebagai baris pendamping: nama TIDAK unik
// (dua "Budi" itu lumrah, dan nilainya dikendalikan user di Google), jadi yang
// hendak mengeluarkan atau menaikkan seseorang butuh sesuatu yang benar-benar
// membedakan.
//
// Keputusan DILAKUKAN DI SINI, bukan di view: dengan begitu view tinggal
// merender apa yang dioper (view murni-data), tak perlu tahu aturan izinnya.
func (h *Handler) MembersPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Gerbang di HANDLER, bukan di route: sejak 0004 satu alamat melayani semua
	// role, dan yang membedakan adalah apa yang boleh dilakukan di dalamnya.
	// Menu yang disembunyikan bukan pengaman — URL-nya tetap bisa diketik.
	if !canViewMembers(ctx) {
		h.renderMembersForbidden(w, r)
		return
	}
	tenantID := session.TenantID(ctx)
	q := h.q(ctx)
	rows, err := q.ListMembersByTenant(ctx, tenantID)
	if err != nil {
		h.Log.Error("members: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Dihitung SEKALI di luar loop: nilainya sama untuk semua baris.
	// manage = boleh mengubah data (bukan cuma melihat) — sumbu tenant
	// (canManageMembers) ATAU sumbu bisnis "Kelola" (crmMemberAccess).
	manage := (canManageMembers(ctx) || crmMemberAccess(ctx) == memberAccessManage) && !IsReadOnly(ctx)
	// scope = Jenis Anggota yang boleh dilihat aktor ini (BL-171). Pengelola
	// tenant selalu dapat cakupan penuh (actorKindScope); aktor business-axis
	// dibatasi member_scope_policies perannya.
	scope := actorKindScope(ctx, q)
	// canEditKind: kolom Kind hanya bisa diubah aktor yang cakupannya mencakup
	// KEDUA jenis (keputusan desain #3 plan BL-171) — mencegah aktor bercakupan
	// sempit memindahkan anggota ke jenis yang tak bisa ia lihat sendiri.
	canEditKind := manage && scope.Both()

	crmDisplay, crmRolesInternal, crmRolesExternal := h.loadMemberCRMOptions(ctx, q, tenantID, manage, scope)

	selfID := session.UserID(ctx)
	members, selfIsAdmin := buildMemberRows(rows, crmDisplay, scope, selfID, canManageMembers(ctx))

	invites := h.buildInviteRows(ctx, q, tenantID, scope, crmDisplay, r)

	h.renderWorkspaceShell(w, r, "Anggota", "/members",
		panel.Members(wsPath(slugFromRequest(r), ""),
			crmRolesInternal, crmRolesExternal, members, invites, manage, canEditKind, selfIsAdmin, selfID,
			wsErrMsg(r.URL.Query().Get("err")), membersMsg(r.URL.Query().Get("ok"))))
}

// renderMembersForbidden menjawab anggota biasa yang membuka halaman ini lewat
// URL langsung: 403 + penjelasan, BUKAN 404.
//
// Penerimanya sudah terbukti anggota workspace ini (Scope memvalidasinya lebih
// dulu), jadi menyangkal keberadaan halaman itu tak melindungi apa pun — ia
// cuma membuat orang mengira ada yang rusak, lalu melaporkannya. Ini keadaan
// yang sama dengan workspace tersuspensi (0005 §3): kepada yang sah, katakan
// alasannya. 404 tetap untuk yang BUKAN anggota, dan itu ditangani Scope.
//
// Status ditulis SEBELUM merender: renderWorkspaceShell menulis body, dan
// WriteHeader setelah body tak berpengaruh — halaman penolakan yang terkirim
// sebagai 200 akan tampak seperti sukses bagi apa pun yang membaca statusnya.
func (h *Handler) renderMembersForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Anggota", "/members", panel.MembersForbidden())
}
