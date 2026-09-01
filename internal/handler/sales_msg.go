package handler

// sales_msg.go — pemetaan kode ?ok=/?err= → pesan untuk modul Sales
// (leads/deals/quotes/activities). Dipisah dari sales_view.go (gerbang izin
// canView*/canWrite*) agar tiap file di bawah ambang Route/Handler (150).
// Murni string→string; tak sentuh ctx/authz.
// leadsMsg memetakan ?ok= → pesan sukses Leads (dipisah dari wsErrMsg agar alert
// sukses & galat tak pernah tertukar variannya).
func leadsMsg(code string) string {
	switch code {
	case "created":
		return "Lead ditambahkan."
	case "saved":
		return "Perubahan lead disimpan."
	case "deleted":
		return "Lead dihapus."
	case "converted":
		return "Lead dikonversi menjadi Desa, Kontak, dan Deal."
	default:
		return ""
	}
}

// dealsMsg memetakan ?ok= → pesan sukses Deals.
func dealsMsg(code string) string {
	switch code {
	case "created":
		return "Deal ditambahkan."
	case "saved":
		return "Perubahan deal disimpan."
	case "staged":
		return "Tahap deal diperbarui."
	case "deleted":
		return "Deal dihapus."
	default:
		return ""
	}
}

// quotesMsg memetakan ?ok= → pesan sukses Quote (header & item). Dipisah agar
// pesan quote tak tertukar dengan deals.
func quotesMsg(code string) string {
	switch code {
	case "created":
		return "Quote dibuat."
	case "saved":
		return "Perubahan quote disimpan."
	case "status":
		return "Status quote diperbarui."
	case "quote_deleted":
		return "Quote dihapus."
	case "item_added":
		return "Item ditambahkan ke quote."
	case "item_saved":
		return "Perubahan item disimpan."
	case "item_deleted":
		return "Item dihapus dari quote."
	case "tax_saved":
		return "Pajak quote diperbarui."
	default:
		return ""
	}
}

// activitiesMsg memetakan ?ok= → pesan sukses Sales Activity (4.4). Dipisah agar
// pesan aktivitas tak tertukar dengan deals/quotes.
func activitiesMsg(code string) string {
	switch code {
	case "created":
		return "Aktivitas dicatat."
	case "saved":
		return "Perubahan aktivitas disimpan."
	case "status":
		return "Status aktivitas diperbarui."
	case "deleted":
		return "Aktivitas dihapus."
	default:
		return ""
	}
}
