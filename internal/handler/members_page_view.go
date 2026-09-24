package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// members_page_view.go — perakitan data BACA untuk MembersPage
// (members_page.go): opsi peran CRM (loadMemberCRMOptions), baris anggota
// (buildMemberRows), dan baris undangan (buildInviteRows). Dipisah agar
// members_page.go di bawah ambang tipe Route/Handler (150, CLAUDE.md §8).
// Perilaku identik dgn sebelum pemisahan — murni pemindahan blok kode, bukan
// perubahan logika.

// loadMemberCRMOptions memuat peran CRM workspace ini, DIMUAT SEKALI, dipakai
// dua cara: crmDisplay (nama mesin → label tampilan, UNTUK SEMUA peran —
// menerjemahkan BusinessRole tiap baris anggota & undangan ke bentuk yang
// dibaca manusia, terlepas dari apakah penglihatnya "Kelola" atau cuma
// "Lihat") dan crmRolesInternal/External (opsi form Undang/edit, DIPECAH per
// Jenis Anggota, hanya diisi bila manage — penglihat "Lihat" tak pernah
// menyunting apa pun jadi tak butuh opsinya). Fail-soft: gagal → crmDisplay
// kosong (baris jatuh ke kode mesin apa adanya), dua daftar opsi kosong
// (dropdown hanya "(tak ada)").
func (h *Handler) loadMemberCRMOptions(ctx context.Context, q *db.Queries, tenantID int64, manage bool, scope kindScope) (crmDisplay map[string]string, crmRolesInternal, crmRolesExternal []panel.CRMRoleOption) {
	crmDisplay = map[string]string{}
	crmRolesInternal = []panel.CRMRoleOption{}
	crmRolesExternal = []panel.CRMRoleOption{}
	if brs, e := q.ListBusinessRoles(ctx, tenantID); e == nil {
		for _, br := range brs {
			crmDisplay[br.Name] = br.DisplayName
			if !manage {
				continue
			}
			opt := panel.CRMRoleOption{Name: br.Name, Display: br.DisplayName}
			if br.Kind == authz.KindExternal {
				if scope.Allows(authz.KindExternal) {
					crmRolesExternal = append(crmRolesExternal, opt)
				}
			} else {
				if scope.Allows(authz.KindInternal) {
					crmRolesInternal = append(crmRolesInternal, opt)
				}
			}
		}
	} else {
		h.Log.Error("members: list business roles", "err", e)
	}
	return crmDisplay, crmRolesInternal, crmRolesExternal
}

// buildMemberRows memetakan baris ListMembersByTenant → panel.MemberRow siap
// render, menyaring Jenis Anggota di luar cakupan aktor (scope.Allows).
// selfIsAdminInit = nilai awal (canManageMembers(ctx) dihitung pemanggil);
// dikembalikan setelah kemungkinan di-flip true di dalam loop bila aktor
// menemukan dirinya sendiri berperan CRM admin.
func buildMemberRows(rows []db.ListMembersByTenantRow, crmDisplay map[string]string, scope kindScope, selfID int64, selfIsAdminInit bool) (members []panel.MemberRow, selfIsAdmin bool) {
	// selfIsAdmin (BL-171): aktor boleh menyunting PERAN CRM dirinya sendiri di
	// baris anggota bila (a) pengelola TENANT (owner/admin/platform — otoritasnya
	// sudah lengkap dari sumbu tenant, cermin pengecualian yang sama di
	// MemberSetRole/MemberSetKind, TestMemberBiz_SelfOptIn) ATAU (b) peran CRM-nya
	// SAAT INI "admin" — mencegah aktor business-axis MURNI non-admin (mis. Field
	// Officer) tanpa sengaja mengubah/mengunci perannya sendiri lewat dropdown yang
	// sama dipakai mengelola anggota lain. Ditentukan di sini (bukan view) dari
	// baris anggota SENDIRI di loop di bawah — data yang sama yang sudah dimuat,
	// tak perlu query terpisah.
	selfIsAdmin = selfIsAdminInit
	members = make([]panel.MemberRow, 0, len(rows))
	for _, m := range rows {
		if !scope.Allows(m.Kind) {
			continue
		}
		avatar := ""
		if m.AvatarUrl != nil {
			avatar = *m.AvatarUrl
		}
		name := ""
		if m.Name != nil {
			name = *m.Name
		}
		// Email TAMPIL PENUH bagi siapa pun yang sampai ke baris ini: gerbang
		// halaman (canViewMembers, di MembersPage) sudah mempersempit penglihat ke
		// pengelola tenant ATAU role bisnis ber-akses "User Management" —
		// populasi yang dulu disamarkan (anggota biasa tanpa akses apa pun)
		// sudah ditolak sebelum mencapai baris ini, jadi menyamarkannya lagi
		// tak melindungi siapa pun.
		email := m.Email
		// business_role (sumbu CRM) ikut dari ListMembersByTenant — NULL = belum
		// diberi peran CRM. BusinessRole (kode mesin) tetap dipakai form edit
		// (memberKindRoleForm mencocokkannya ke value opsi select);
		// BusinessRoleDisplay (label manusia, via crmDisplay) dipakai tampilan
		// baca-saja (roleBadges) — dua field dari nilai yang sama, dipakai dua
		// jalur berbeda.
		crm := ""
		if m.BusinessRole != nil {
			crm = *m.BusinessRole
		}
		crmLabel := crm
		if d, ok := crmDisplay[crm]; ok {
			crmLabel = d
		}
		if m.UserID == selfID && crm == authz.BusinessRoleAdmin {
			selfIsAdmin = true
		}
		members = append(members, panel.MemberRow{
			UserID: m.UserID, Email: email, Name: name, Role: m.Role,
			BusinessRole: crm, BusinessRoleDisplay: crmLabel, Kind: m.Kind,
			AvatarURL: avatar, Status: m.Status,
		})
	}
	return members, selfIsAdmin
}

// buildInviteRows memetakan undangan pending → panel.InviteRow siap render,
// menyaring Jenis Anggota di luar cakupan aktor sama seperti buildMemberRows.
// Fail-soft: gagal → daftar kosong, halaman tetap tampil.
func (h *Handler) buildInviteRows(ctx context.Context, q *db.Queries, tenantID int64, scope kindScope, crmDisplay map[string]string, r *http.Request) []panel.InviteRow {
	invites := []panel.InviteRow{}
	if inv, e := q.ListInvitesByTenant(ctx, tenantID); e == nil {
		for _, i := range inv {
			if !scope.Allows(i.Kind) {
				continue
			}
			br := ""
			if i.BusinessRole != nil {
				br = *i.BusinessRole
				if d, ok := crmDisplay[br]; ok {
					br = d
				}
			}
			invites = append(invites, panel.InviteRow{
				ID: i.ID, Email: i.Email, BusinessRole: br, Kind: i.Kind,
				Link: inviteLink(r, i.Token), Expires: fmtLocal(i.ExpiresAt),
			})
		}
	} else {
		h.Log.Error("members: list invites", "err", e)
	}
	return invites
}
