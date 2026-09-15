package handler

import (
	"sort"

	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// all_activities_feed.go — entri feed, paginasi (merge-sort → potong → majukan
// cursor komposit), dan pemetaan baris CS→ActivityRow untuk feed terpadu.
// Dipisah dari all_activities_unified.go (ukuran file); logika identik.

// feedEntry = satu baris terpadu + kunci sort mentah untuk merge-sort &
// memajukan sub-cursor sumbernya. source membedakan asal ("sales" activities /
// "cs" engagements). sortVal/sortNull = NILAI SUMBU AKTIF (bukan selalu
// created_at lagi sejak BL-157k) — juga dipakai LANGSUNG sebagai titik lanjut
// sub-cursor (genSubCursor{val: sortVal, isNull: sortNull, id: id}), jadi tak
// perlu field cursor terpisah seperti desain lama.
type feedEntry struct {
	id       int64
	source   string
	row      panel.ActivityRow
	sortVal  string
	sortNull bool
}

// lessEntry menentukan urutan tampil dua entri pada sumbu aktif, meniru
// PERSIS semantik ORDER BY dinamis kelima query SortBy*: NULLS LAST pada asc,
// NULLS FIRST pada desc (default Postgres), id sebagai tie-break SEARAH dir,
// dan source sebagai tie-break terakhir (deterministik bila id kebetulan
// bertumbukan lintas-tabel — dua PK independen).
func lessEntry(a, b feedEntry, dir string) bool {
	if a.sortNull != b.sortNull {
		if dir == "asc" {
			return !a.sortNull // non-NULL dulu (NULLS LAST)
		}
		return a.sortNull // NULL dulu (NULLS FIRST)
	}
	if !a.sortNull && a.sortVal != b.sortVal {
		if dir == "asc" {
			return a.sortVal < b.sortVal
		}
		return a.sortVal > b.sortVal
	}
	if a.id != b.id {
		if dir == "asc" {
			return a.id < b.id
		}
		return a.id > b.id
	}
	return a.source < b.source
}

// pageEntries mengurut entries pada sumbu aktif (lessEntry), memotong ke
// pageSize, memajukan tiap sub-cursor ke baris TERAKHIR yang ditampilkan dari
// sumbernya, lalu merakit cursor komposit halaman berikutnya ("" = halaman
// terakhir).
func pageEntries(entries []feedEntry, dc dualCursorGen, dir string) ([]panel.ActivityRow, string) {
	sort.SliceStable(entries, func(i, j int) bool {
		return lessEntry(entries[i], entries[j], dir)
	})

	// ── Potong ke pageSize; kelebihan = penanda "masih ada" (sama pola
	// splitPage). Kombinasi ≤ pageSize ⟹ tak ada sumber yang menyentuh +1 ⟹
	// keduanya habis ⟹ tak ada lagi (lihat rasional di BL-41 tasks). ──────────
	more := len(entries) > pageSize
	if more {
		entries = entries[:pageSize]
	}

	// ── Majukan tiap sub-cursor ke baris TERAKHIR yang DITAMPILKAN dari sumber
	// itu (entries sudah urut sesuai dir → kemunculan terakhir dalam loop =
	// titik lanjut benar). Sumber tanpa baris tampil → sub-cursor tak berubah
	// (baris yang di-fetch tapi tak tampil akan di-query ulang halaman berikut). ─
	next := dc
	for _, e := range entries {
		adv := genSubCursor{hasCursor: true, isNull: e.sortNull, val: e.sortVal, id: e.id}
		switch e.source {
		case "sales":
			next.act = adv
		case "cs":
			next.eng = adv
		}
	}
	nextCursor := ""
	if more {
		nextCursor = encodeDualCursorGen(next)
	}

	items := make([]panel.ActivityRow, 0, len(entries))
	for _, e := range entries {
		items = append(items, e.row)
	}
	return items, nextCursor
}

// engagementFeedRowCore memetakan satu baris engagement (field mentah, bukan
// tipe Row spesifik) → ActivityRow bertanda Source="cs". sqlc menghasilkan
// SATU TIPE ROW TERPISAH per query walau SELECT list identik (5 varian
// SortBy*), jadi mapper ambil field mentah alih-alih tipe Row bertipe agar
// dipakai lintas kelima varian TANPA 5 wrapper duplikat. Read-only:
// TargetType/ID = desa induk (view menaut ke /accounts/{id}, bukan
// /activities/{id}). Kolom Jenis pakai TypeLabel (engagement_type), Status
// pakai peta engagement (engagementStatusLabel → label + badge daisyUI).
// Konteks="cs" → chip "CS".
func engagementFeedRowCore(
	id, accountID int64,
	subject, engagementType, status string,
	createdAt pgtype.Timestamptz,
	ownerName *string,
) panel.ActivityRow {
	statusLabel, statusBadge := engagementStatusLabel(status)
	owner := ""
	if ownerName != nil {
		owner = *ownerName
	}
	return panel.ActivityRow{
		ID:               id,
		Subject:          subject,
		TargetType:       "account",
		TargetID:         accountID,
		Owner:            owner,
		Status:           statusLabel,
		StatusBadgeClass: statusBadge, // "badge-info"/…; view menambah prefiks "badge "
		Created:          fmtLocal(createdAt),
		Context:          "cs",
		Source:           "cs",
		TypeLabel:        engagementTypeLabel(engagementType),
	}
}
