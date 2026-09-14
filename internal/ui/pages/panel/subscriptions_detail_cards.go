package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// subscriptions_detail_cards.go — 3 kartu baru BL-154 (Status & Lifecycle,
// Renewal, System & Audit), dipisah dari subscriptions_detail.go demi ukuran
// file. Murni-data: semua label/badge-class/turunan SUDAH diputuskan handler
// (subscriptions_detail_view.go); di sini hanya merender.

// subStatusLifecycleCard = kartu "Status & Lifecycle" (BL-154). Health &
// Onboarding lintas-modul (customer_success, BL-114) — kosong bila desa belum
// punya baris customer_success, ditampilkan "—" via orDash (bukan error).
func subStatusLifecycleCard(v SubDetailView) g.Node {
	healthBadge := g.Node(g.Text(orDash(v.HealthLabel)))
	if v.HealthLabel != "" {
		healthBadge = h.Span(h.Class("badge "+v.HealthBadgeClass), g.Text(v.HealthLabel))
	}
	return cardRows("Status & Lifecycle", "",
		detailRow("Status", subStatusBadge(v.Status)),
		detailRow("Ditangguhkan?", g.Text(suspendedLabel(v.Status))),
		detailRow("Kesehatan (Health)", healthBadge),
		detailRow("Status Onboarding", g.Text(orDash(v.OnboardingStatus))),
		detailRow("Progres Onboarding", g.Text(orDash(v.OnboardingProgress))),
		detailRow("Tanggal Aktivasi", g.Text(orDash(v.ActivatedAt))),
	)
}

// suspendedLabel = turunan boolean dari Status (bukan kolom tersendiri di DB) —
// "Ya" hanya saat status persis "Suspended" (enum subs_status_chk).
func suspendedLabel(status string) string {
	if status == "Suspended" {
		return "Ya"
	}
	return "Tidak"
}

// subRenewalCard = kartu "Renewal" (BL-154). RenewalStatusLabel/Class & sisa
// hari REUSE persis logika tab Renewals (subscriptions_renewals_row.go) agar
// badge selaras lintas halaman. PrevToCurrent sudah diformat & disamarkan (F4)
// oleh handler.
func subRenewalCard(v SubDetailView) g.Node {
	cls := v.RenewalStatusClass
	if cls == "" {
		cls = "badge badge-ghost"
	}
	statusBadge := h.Span(h.Class(cls), g.Text(orDash(v.RenewalStatusLabel)))
	return cardRows("Renewal", "",
		detailRow("Tanggal Renewal", g.Text(orDash(v.End))),
		detailRow("Sisa Hari", g.Text(orDash(v.DaysToRenewal))),
		detailRow("Tipe Renewal", g.Text(orDash(v.RenewalTypeLabel))),
		detailRow("Status Renewal", statusBadge),
		detailRow("Sebelum → Sekarang", g.Text(orDash(v.PrevToCurrent))),
	)
}

// subSystemAuditCard = kartu "System & Audit" (BL-154), sejajar
// dealSystemAuditCard. Source = deal asal (bila langganan dikonversi dari
// deal) — SourceDealHref kosong → teks polos "—" tanpa tautan.
func subSystemAuditCard(v SubDetailView) g.Node {
	source := g.Node(g.Text(orDash(v.SourceDealLabel)))
	if v.SourceDealHref != "" {
		source = h.A(h.Href(v.SourceDealHref), h.Class("link link-hover"),
			g.Text(v.SourceDealLabel))
	}
	return cardRows("System & Audit", "",
		detailRow("ID Langganan", g.Text(orDash(v.EntityCode))),
		detailRow("Dibuat Oleh", g.Text(orDash(v.CreatedByName))),
		detailRow("Tanggal Dibuat", g.Text(orDash(v.CreatedAt))),
		detailRow("Diubah Oleh", g.Text(orDash(v.UpdatedByName))),
		detailRow("Terakhir Diubah", g.Text(orDash(v.UpdatedAt))),
		detailRow("Sumber", source),
	)
}
