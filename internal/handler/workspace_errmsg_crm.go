package handler

// workspace_errmsg_crm.go — lanjutan wsErrMsg untuk kode PRG modul CRM (Sales:
// Leads/Deals/Quote/Activity, Subscriptions) plus fallback umum `failed`.
// Dipisah dari workspace_errmsg.go (kode inti workspace/settings/members/village/
// contact/role) agar tiap file di bawah ambang tipe Route/Handler (150). wsErrMsg
// mendelegasikan ke sini untuk kode yang bukan miliknya; SETIAP kode unik lintas
// kedua peta, jadi hasil untuk tiap input identik dengan switch tunggal semula.
func wsErrMsgCRM(code string) string {
	switch code {
	// ── Sales: Leads & Deals (Modul 4) ──────────────────────────────────────
	case "lead_name":
		return "Nama lead wajib diisi (maks 200 karakter)."
	case "lead_status":
		return "Status lead tidak valid."
	case "lead_rating":
		return "Rating lead tidak valid."
	case "estimated":
		return "Nilai estimasi harus berupa angka."
	case "mobile_phone":
		return "Nomor HP tidak valid (6–20 digit; boleh diawali + dan pemisah spasi/-)."
	case "whatsapp":
		return "Nomor WhatsApp tidak valid (6–20 digit; boleh diawali + dan pemisah spasi/-)."
	case "date":
		return "Tanggal tidak valid (format YYYY-MM-DD)."
	case "probability":
		return "Probabilitas harus berupa angka 0–100."
	case "amount":
		return "Nilai deal harus berupa angka."
	case "deal_name":
		return "Nama deal wajib diisi (maks 200 karakter)."
	case "stage":
		return "Tahap deal tidak valid."
	case "deal_type":
		return "Tipe deal tidak valid."
	case "deal_term":
		return "Termin langganan tidak valid."
	case "forecast":
		return "Kategori forecast tidak valid."
	case "win_loss":
		return "Alasan menang/kalah wajib diisi saat deal ditutup (Closed Won/Lost)."
	case "account_req":
		return "Desa (account) wajib dipilih untuk deal ini."
	case "convert_guard":
		return "Lead ini tak bisa dikonversi — harus berstatus Qualified dan belum pernah dikonversi."
	// ── Sales: Quote (Modul 4) ──────────────────────────────────────────────
	case "quote_name":
		return "Nama quote terlalu panjang (maks 200 karakter)."
	case "terms":
		return "Termin/catatan terlalu panjang (maks 2000 karakter)."
	case "tax":
		return "Pajak harus berupa angka ≥ 0."
	case "tax_mode":
		return "Jenis pajak tidak valid (pilih Persentase atau Nominal)."
	case "tax_rate":
		return "Tarif pajak harus berupa angka 0–100."
	case "prepared_by":
		return "Penyusun yang dipilih tidak valid."
	case "expiration_past":
		return "Tanggal kedaluwarsa tidak boleh tanggal yang sudah lewat."
	case "plan_req":
		return "Plan wajib dipilih untuk item ini."
	case "qty":
		return "Kuantitas harus berupa bilangan bulat lebih dari 0."
	case "discount":
		return "Diskon harus berupa angka 0–100."
	case "status":
		return "Status quote tidak valid."
	case "quote_stage":
		return "Quote hanya dapat dibuat/diubah saat deal di tahap Qualification–Negotiation."
	// ── Sales: Activity Log (4.4) ───────────────────────────────────────────
	case "activity_kind":
		return "Jenis aktivitas tidak valid."
	case "activity_subject":
		return "Subjek aktivitas wajib diisi (maks 200 karakter)."
	case "activity_target":
		return "Target aktivitas wajib dipilih dan berada dalam cakupan Anda."
	case "activity_priority":
		return "Prioritas tugas tidak valid."
	case "activity_status":
		return "Status tugas tidak valid."
	case "activity_direction":
		return "Arah panggilan/chat tidak valid."
	case "activity_call_result":
		return "Hasil panggilan tidak valid."
	case "activity_meeting_type":
		return "Tipe pertemuan tidak valid."
	case "activity_channel":
		return "Kanal chat tidak valid."
	case "activity_contact":
		return "Kontak yang dipilih tidak valid."
	case "activity_duration":
		return "Durasi panggilan harus berupa bilangan bulat ≥ 0."
	case "datetime":
		return "Waktu tidak valid (format YYYY-MM-DDTHH:MM)."
	// ── Subscriptions: Renewal & Churn (Modul 5) ────────────────────────────
	case "sub_not_active":
		return "Langganan harus berstatus Active untuk diperpanjang."
	case "new_mrr":
		return "MRR baru harus berupa angka ≥ 0."
	case "not_pending":
		return "Renewal ini tidak (lagi) menunggu persetujuan."
	case "churn_reason":
		return "Alasan churn tidak valid."
	case "churn_type":
		return "Tipe churn tidak valid."
	case "failed":
		return "Tindakan gagal. Coba lagi."
	default:
		return ""
	}
}
