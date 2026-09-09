package handler

import (
	"context"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// accounts_view.go — pemetaan model DB → data siap-render view (murni-data).
// Penyamaran Field-Level Security (F4) dilakukan DI SINI, di handler, sebelum
// nilai menyentuh view: yang tak berhak TAK PERNAH menerima nilai aslinya (lihat
// fls.go). View accounts hanya menerima string yang sudah diputuskan.

// accountRowView memetakan satu baris daftar. Nomor HP TIDAK ikut di baris
// daftar (PII; hanya relevan di detail), jadi tak ada yang perlu disamarkan di
// sini — masking F4 berlaku di detail. names = peta user_id→nama (dirakit sekali
// di handler) untuk kolom Owner/CSM; id tak-tertugas / tak-dikenal → "" ("—").
// regions = peta ancestry wilayah (h.regionAncestryMap, dimuat SEKALI oleh
// pemanggil sebelum loop baris — rule #13, ADR 0009) dipakai resolve
// Regency/Province dari district_id TANPA query tambahan per baris.
func accountRowView(a db.Account, names map[int64]string, regions map[int64]regionNode) panel.AccountRow {
	_, regency, province := regionNames(regions, a.DistrictID)
	return panel.AccountRow{
		ID:          a.ID,
		VillageName: a.VillageName,
		VillageCode: deref(a.VillageCode),
		AccountType: accountTypeLabel(a.AccountType),
		Regency:     regency,
		Province:    province,
		OwnerName:   memberName(names, a.AccountOwner),
		CSMName:     memberName(names, a.AssignedCsm),
	}
}

// accountDetailView merakit data detail lengkap + terapkan F4. Menerima ctx untuk
// membaca business_role aktor (dasar masking). base = prefix URL workspace.
func (h *Handler) accountDetailView(ctx context.Context, base string, a db.Account) panel.AccountDetailView {
	br := session.BusinessRole(ctx)
	// Satu baris → 1 query GetRegionAncestry (bukan regionAncestryMap, yang
	// scan ~7.817 baris demi 1 hasil — cocok utk daftar, bukan detail).
	var province, regency, district string
	if a.DistrictID != nil {
		if anc, err := h.q(ctx).GetRegionAncestry(ctx, *a.DistrictID); err == nil {
			province, regency, district = anc.ProvinceName, anc.RegencyName, anc.DistrictName
		}
	}

	// names = peta user_id→nama, dirakit SEKALI untuk semua kartu rollup
	// (Pemilik Akun, CSM, audit Dibuat/Diperbarui oleh) — bukan query per kartu.
	names, err := h.accountMemberNames(ctx)
	if err != nil {
		h.Log.Error("accounts: member names", "err", err)
	}
	parentLabel, parentHref := h.parentAccountFor(ctx, base, a.ParentAccountID)
	sub := h.subscriptionSummaryFor(ctx, base, a.ID, br)

	return panel.AccountDetailView{
		Base:        base,
		ID:          a.ID,
		VillageName: a.VillageName,
		// BL-61: entity_code tak lagi dipetakan ke view detail (badge dihapus);
		// yang tampil = VillageCode (Kemendagri). entity_code tetap ada di DB.
		VillageCode: deref(a.VillageCode),
		AccountType: accountTypeLabel(a.AccountType),
		Website:     deref(a.Website),
		Description: deref(a.Description),

		// M2-7: kolom identitas tambahan — akun sudah punya kolomnya sejak awal,
		// sebelumnya tak pernah disurfacekan ke view.
		AccountOwnerName:   memberName(names, a.AccountOwner),
		ParentAccountLabel: parentLabel,
		ParentAccountHref:  parentHref,

		Province:       province,
		Regency:        regency,
		District:       district,
		VillageAddress: deref(a.VillageAddress),
		PostalCode:     deref(a.PostalCode),
		Territory:      deref(a.Territory),
		Latitude:       numericStr(a.Latitude),
		Longitude:      numericStr(a.Longitude),

		VillageStatus:         deref(a.VillageStatus),
		VillageClassification: deref(a.VillageClassification),
		Population:            int32Str(a.Population),
		HamletsCount:          int32Str(a.HamletsCount),
		// F4: anggaran desa = nilai komersial/sensitif setingkat ARR → maskARR
		// (kebijakan umum kecuali Support, skema.md §9). Diperbaiki audit FLS M9-1
		// (sebelumnya mentah; laten krn F3 sudah nol-baris utk Support — defense
		// in depth bila F3 pernah berubah).
		// BL-112: tampilkan terformat "Rp 7.500.000" (formatRupiah), bukan angka
		// mentah "7500000.00" (numericStr). Masking F4 (maskARR) tetap memutuskan
		// tampil/sembunyi TERPISAH dari format.
		VillageBudget: maskARR(formatRupiah(a.VillageBudget), br),

		// BL-106: HP Kontak account TAK lagi ber-FLS — siapa pun yang boleh melihat
		// desa ini melihat nomor penuh (beda dari Kontak/Lead yang tetap ber-mask).
		// Office phone/email = data kelembagaan (bukan PII pribadi) → juga tak disamar.
		ContactPhone: deref(a.ContactPhone),
		OfficePhone:  deref(a.OfficePhone),
		OfficeEmail:  deref(a.OfficeEmail),

		CanWrite: canWriteAccounts(ctx),

		// M2-8/M2-9: kartu ringkasan lintas-modul + baris "Terkait" — satu
		// layout utk semua role (F2/F3/F4 di builder masing-masing yang
		// memutuskan isinya, bukan tampilan per-POV terpisah).
		Subscription:    sub,
		CustomerSuccess: h.customerSuccessSummaryFor(ctx, base, a, names),
		Audit:           auditViewFor(a, names),
		Related:         h.relatedRecordsFor(ctx, base, a.ID, sub),

		// BL-31: linimasa TERPADU (Sales activities + CS engagements), read-only.
		// names sudah dirakit di atas → dioper agar tak query anggota dua kali.
		// canViewEngagements di dalam builder yang menentukan apakah sumber CS ikut.
		Activities: h.accountUnifiedTimelineFor(ctx, base, a, names, canWriteSalesActivityPerm(ctx)),
	}
}
