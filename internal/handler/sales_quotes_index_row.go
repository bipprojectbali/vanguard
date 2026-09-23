package handler

import (
	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// sales_quotes_index_row.go — whitelist sort & row-mapping utk QuotesIndex,
// dipisah dari sales_quotes_index.go krn ambang File Health (Route/Handler
// 150 baris). Fungsi sort per-kolom di sales_quotes_index_sort_a.go &
// sales_quotes_index_sort_b.go.

// quoteSortableColumns = whitelist kolom yang boleh diminta lewat ?sort=
// (BL-157f: 5 kolom tabel Quotes global). ?sort= di luar daftar ini
// diperlakukan seolah absen (jatuh ke default created_at DESC), TAK error.
var quoteSortableColumns = map[string]bool{
	"code":   true,
	"name":   true,
	"deal":   true,
	"status": true,
	"total":  true,
}

// quoteIndexRowView memetakan satu baris ListQuotes (quote + deal_name) → baris
// daftar global. Angka SUDAH diformat; status mentah (badge diputuskan view).
// DealID dibawa agar view merakit tautan ke quote di bawah deal induknya.
func quoteIndexRowView(q db.ListQuotesRow) panel.QuoteIndexRow {
	// deal_id selalu terisi (ListQuotes INNER JOIN deals ON d.id = q.deal_id) —
	// guard nil hanya untuk ketahanan tipe (skema mengizinkan NULL).
	var dealID int64
	if q.DealID != nil {
		dealID = *q.DealID
	}
	return panel.QuoteIndexRow{
		QuoteID:    q.ID,
		DealID:     dealID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		DealName:   q.DealName,
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
	}
}

// quoteIndexRowViewSortByCode/…SortByName/…SortByDeal/…SortByStatus/…SortByTotal
// memetakan baris dari tiap query ListQuotesSortByX (kolom Row-nya identik dgn
// ListQuotesRow, generated per-query oleh sqlc) → panel.QuoteIndexRow yang sama.
func quoteIndexRowViewSortByCode(q db.ListQuotesSortByCodeRow) panel.QuoteIndexRow {
	var dealID int64
	if q.DealID != nil {
		dealID = *q.DealID
	}
	return panel.QuoteIndexRow{
		QuoteID:    q.ID,
		DealID:     dealID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		DealName:   q.DealName,
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
	}
}

func quoteIndexRowViewSortByName(q db.ListQuotesSortByNameRow) panel.QuoteIndexRow {
	var dealID int64
	if q.DealID != nil {
		dealID = *q.DealID
	}
	return panel.QuoteIndexRow{
		QuoteID:    q.ID,
		DealID:     dealID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		DealName:   q.DealName,
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
	}
}

func quoteIndexRowViewSortByDeal(q db.ListQuotesSortByDealRow) panel.QuoteIndexRow {
	var dealID int64
	if q.DealID != nil {
		dealID = *q.DealID
	}
	return panel.QuoteIndexRow{
		QuoteID:    q.ID,
		DealID:     dealID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		DealName:   q.DealName,
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
	}
}

func quoteIndexRowViewSortByStatus(q db.ListQuotesSortByStatusRow) panel.QuoteIndexRow {
	var dealID int64
	if q.DealID != nil {
		dealID = *q.DealID
	}
	return panel.QuoteIndexRow{
		QuoteID:    q.ID,
		DealID:     dealID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		DealName:   q.DealName,
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
	}
}

func quoteIndexRowViewSortByTotal(q db.ListQuotesSortByTotalRow) panel.QuoteIndexRow {
	var dealID int64
	if q.DealID != nil {
		dealID = *q.DealID
	}
	return panel.QuoteIndexRow{
		QuoteID:    q.ID,
		DealID:     dealID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		DealName:   q.DealName,
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
	}
}
