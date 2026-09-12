package handler

// sales_leads_form_options.go — opsi dropdown (urutan tampilan) untuk enum Lead,
// dipisah dari sales_leads_form.go (parsing & map validasi) untuk file health.

// Opsi enum untuk dropdown — slice BERURUT (map validasi tak berurutan). Nilai
// HARUS himpunan yang sama dengan map validasi & CHECK 00009; urutan untuk tampilan.
var (
	leadStatusOptions = []string{"New", "Contacted", "Qualified", "Unqualified"}
	leadRatingOptions = []string{"Hot", "Warm", "Cold"}
	// leadSourceOptions = urutan tampilan dropdown "Sumber Lead" (BL-82). HARUS
	// himpunan yang sama dengan validLeadSources; urutan sesuai permintaan user.
	leadSourceOptions = []string{"Referral", "Event", "Website", "Cold Call", "Tender", "Dinas PMD", "Lainnya"}
)

// compile-time: opsi & map validasi sepakat (panjang sama). Berbeda = dropdown
// menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(leadStatusOptions) != len(validLeadStatuses) ||
		len(leadRatingOptions) != len(validLeadRatings) ||
		len(leadSourceOptions) != len(validLeadSources) {
		panic("leads: opsi enum tak sinkron dengan map validasi")
	}
	return struct{}{}
}()
