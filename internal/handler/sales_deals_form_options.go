package handler

// sales_deals_form_options.go — opsi dropdown enum & guard sinkronisasi
// compile-time, dipisah dari sales_deals_form.go (ukuran file). Perilaku
// identik; hanya organisasi file yang berubah.

// Opsi enum untuk dropdown — slice BERURUT (map validasi tak berurutan). Nilai
// HARUS himpunan yang sama dengan map validasi & CHECK 00009; urutan untuk tampilan.
var (
	dealTypeOptions         = []string{"New Business", "Renewal", "Upsell", "Cross-sell"}
	dealStageOptions        = []string{"Prospecting", "Qualification", "Demo", "Proposal", "Negotiation", "Closed Won", "Closed Lost"}
	subscriptionTermOptions = []string{"Monthly", "Annual", "Multi-year"}
	// lossReasonCodeOptions = picklist alasan kalah (BL-44 3a; cermin CHECK 00038).
	// Hanya relevan saat Closed Lost → dipakai grouping bersih di Sales Report.
	lossReasonCodeOptions = []string{"Harga", "Fitur", "Kompetitor", "Anggaran", "Lainnya"}
)

// validLossReasonCodes = himpunan kode alasan kalah yang sah (gate handler saat
// Closed Lost). Sumber sama dengan lossReasonCodeOptions & CHECK 00038.
var validLossReasonCodes = map[string]struct{}{
	"Harga": {}, "Fitur": {}, "Kompetitor": {}, "Anggaran": {}, "Lainnya": {},
}

// compile-time: opsi & map validasi sepakat (panjang sama). Berbeda = dropdown
// menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(dealTypeOptions) != len(validDealTypes) ||
		len(dealStageOptions) != len(validDealStages) ||
		len(subscriptionTermOptions) != len(validSubscriptionTerms) ||
		len(lossReasonCodeOptions) != len(validLossReasonCodes) {
		panic("deals: opsi enum tak sinkron dengan map validasi")
	}
	return struct{}{}
}()
