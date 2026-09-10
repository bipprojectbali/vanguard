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

// MembersPage — GET /w/{workspace}/members. Daftar anggota + undangan pending.
//
// HANYA PENGELOLA (owner/admin/platform), di mode single MAUPUN multi. Ini
// MEMBALIK 0004, yang membuka halaman ini untuk semua anggota dengan alasan
// "tahu siapa yang punya akses adalah bagian dari mempercayai ruang bersama".
// Alasan itu benar untuk ruang kerja kecil yang saling kenal, tapi daftar
// anggota adalah DIREKTORI ORANG: ia mengumpulkan nama, wajah, dan keanggotaan
// setiap orang di satu tempat yang bisa disalin sekaligus. Yang membutuhkannya
// untuk bekerja hanyalah yang mengelola keanggotaan; bagi yang lain ia
// pengetahuan tanpa kegunaan, dan kumpulan PII selalu punya biaya.
//
// Penyamaran email (maskEmail) TETAP ADA di jalur ini walau kini semua penglihat
// adalah pengelola: aturan tampilan dan aturan akses adalah dua hal berbeda, dan
// yang satu tak boleh diam-diam bergantung pada yang lain. Kalau kelak halaman
// ini dibuka lagi untuk sebagian orang, perlindungannya sudah di tempatnya —
// bukan sesuatu yang harus diingat untuk dipasang kembali.
//
// PENANDA ORANG = NAMA, bukan email. Email adalah PII yang bisa dipakai
// menghubungi & mencoba masuk sebagai orangnya. Bagi pengelola email tetap
// tampil sebagai baris pendamping: nama TIDAK unik (dua "Budi" itu lumrah, dan
// nilainya dikendalikan user di Google), jadi yang hendak mengeluarkan atau
// menaikkan seseorang butuh sesuatu yang benar-benar membedakan.
//
// Penyamaran & pemilihan dilakukan DI SINI, bukan di view: dengan begitu email
// asli tak pernah sampai ke browser yang tak berhak, tempat ia terbaca di
// view-source meski tak tampak di layar.
func (h *Handler) MembersPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Gerbang di HANDLER, bukan di route: sejak 0004 satu alamat melayani semua
	// role, dan yang membedakan adalah apa yang boleh dilakukan di dalamnya.
	// Menu yang disembunyikan bukan pengaman — URL-nya tetap bisa diketik.
	if !canManageMembers(ctx) {
		h.renderMembersForbidden(w, r)
		return
	}
	tenantID := session.TenantID(ctx)
	rows, err := h.q(ctx).ListMembersByTenant(ctx, tenantID)
	if err != nil {
		h.Log.Error("members: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Dihitung SEKALI di luar loop: nilainya sama untuk semua baris, dan
	// canManageMembers membaca session tiap kali dipanggil.
	canManage := canManageMembers(ctx) && !IsReadOnly(ctx)

	members := make([]panel.MemberRow, 0, len(rows))
	for _, m := range rows {
		avatar := ""
		if m.AvatarUrl != nil {
			avatar = *m.AvatarUrl
		}
		name := ""
		if m.Name != nil {
			name = *m.Name
		}
		// Diri sendiri SELALU utuh: menyamarkan email orang dari dirinya sendiri
		// hanya membuat ia mengira melihat baris orang lain.
		email := m.Email
		if !canManage && m.UserID != session.UserID(ctx) {
			// Tanpa nama, samaran email adalah SATU-SATUNYA pembeda baris ini —
			// dikirim. Dengan nama, email tak dikirim sama sekali: yang tak pernah
			// sampai ke browser tak bisa bocor dari sana.
			if name != "" {
				email = ""
			} else {
				email = maskEmail(email)
			}
		}
		// business_role (sumbu CRM) ikut dari ListMembersByTenant — NULL = belum
		// diberi peran CRM. Dropdown "Peran CRM" memilih nilai ini.
		crm := ""
		if m.BusinessRole != nil {
			crm = *m.BusinessRole
		}
		members = append(members, panel.MemberRow{
			UserID: m.UserID, Email: email, Name: name, Role: m.Role,
			BusinessRole: crm, AvatarURL: avatar, Status: m.Status,
		})
	}
	// Peran CRM yang boleh ditugaskan (sumber sama dgn panel /roles). Fail-soft:
	// gagal → daftar kosong, dropdown hanya menyisakan "(tak ada)".
	crmRoles := []panel.CRMRoleOption{}
	if canManage {
		if brs, e := h.q(ctx).ListBusinessRoles(ctx, tenantID); e == nil {
			for _, br := range brs {
				crmRoles = append(crmRoles, panel.CRMRoleOption{Name: br.Name, Display: br.DisplayName})
			}
		} else {
			h.Log.Error("members: list business roles", "err", e)
		}
	}
	// Undangan pending (fail-soft: gagal → daftar kosong, halaman tetap tampil).
	invites := []panel.InviteRow{}
	if inv, e := h.q(ctx).ListInvitesByTenant(ctx, tenantID); e == nil {
		for _, i := range inv {
			invites = append(invites, panel.InviteRow{
				ID: i.ID, Email: i.Email, Role: i.Role,
				Link: inviteLink(r, i.Token), Expires: fmtLocal(i.ExpiresAt),
			})
		}
	} else {
		h.Log.Error("members: list invites", "err", e)
	}

	h.renderWorkspaceShell(w, r, "Anggota", "/members",
		panel.Members(wsPath(slugFromRequest(r), ""),
			crmRoles, members, invites, canManage, session.UserID(ctx),
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
