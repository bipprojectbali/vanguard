package handler

import (
	"encoding/json"

	"go_starter/internal/claudeai"
)

// jena_ai_tools_registry.go — daftar skema tool Jena AI (nama+deskripsi+
// input schema), dipisah dari jena_ai_tools.go (file health, CLAUDE.md §8).
// Murni DATA (tak ada logic/keamanan di sini) — implementasi F2/F3/F4 tiap
// tool ada di jena_ai_tools.go (Account/Subscription) & jena_ai_tools_sales.go
// (Deal/Lead); lihat komentar file itu utk penjelasan pola dua-rasa
// list_my_*/search_*.

// jenaTools mendaftarkan allowlist eksplisit tool yang boleh dipanggil Jena AI
// — dikirim ke claudeai.AskWithTools, dibaca ulang oleh jenaDispatch di bawah.
func (h *Handler) jenaTools() []claudeai.Tool {
	return []claudeai.Tool{
		{
			Name: "search_accounts",
			Description: "Cari desa (Account) berdasarkan nama atau kode desa. " +
				"Kembalikan daftar id+nama saja (tanpa data finansial/kepemilikan) — " +
				"dipakai untuk menemukan account_id sebelum memanggil " +
				"get_account_summary atau get_subscription_status.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {"type": "string", "description": "nama atau kode desa yang dicari"}
				},
				"required": ["query"]
			}`),
		},
		{
			Name: "get_account_summary",
			Description: "Ambil ringkasan satu desa (Account) berdasarkan account_id " +
				"— profil pemerintahan, kepemilikan (Account Owner/CSM), dan anggaran " +
				"desa (bila penanya berhak melihat nilai komersial). Hanya berhasil " +
				"untuk desa yang boleh dilihat penanya.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"account_id": {"type": "integer", "description": "id account, hasil search_accounts"}
				},
				"required": ["account_id"]
			}`),
		},
		{
			Name: "get_subscription_status",
			Description: "Ambil status langganan (Subscription) TERBARU satu desa " +
				"berdasarkan account_id — paket, status, tanggal mulai/berakhir, " +
				"MRR/ARR (bila penanya berhak melihat nilai komersial).",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"account_id": {"type": "integer", "description": "id account, hasil search_accounts"}
				},
				"required": ["account_id"]
			}`),
		},
		{
			Name: "list_my_deals",
			Description: "Daftar Deal (proses penjualan) MILIK penanya sendiri — " +
				"selalu hanya deal dengan Deal Owner = penanya, apa pun cakupan data " +
				"perannya. Tanpa parameter. Untuk deal MILIK ORANG LAIN atau lintas-" +
				"pemilik (mis. 'deal yang dipegang user X'), pakai search_deals.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name: "list_my_leads",
			Description: "Daftar Lead (prospek belum jadi Deal) MILIK penanya sendiri — " +
				"selalu hanya lead dengan Lead Owner = penanya, apa pun cakupan data " +
				"perannya. Tanpa parameter. Untuk lead MILIK ORANG LAIN atau lintas-" +
				"pemilik (mis. 'lead yang dibuat user X'), pakai search_leads.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
		},
		{
			Name: "search_deals",
			Description: "Cari Deal (proses penjualan) berdasarkan nama/kode deal " +
				"dan/atau NAMA PEMILIK (Deal Owner) — dipakai untuk pertanyaan " +
				"lintas-pemilik (mis. 'deal yang dipegang oleh user jun'). Hasil " +
				"TETAP tunduk cakupan data (data_scope) peran penanya: bila " +
				"perannya hanya boleh melihat deal sendiri, deal milik orang lain " +
				"tak akan muncul walau namanya cocok. Semua parameter opsional; " +
				"kosongkan keduanya untuk daftar terbaru sesuai cakupan penanya.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {"type": "string", "description": "nama atau kode deal yang dicari (opsional)"},
					"owner_search": {"type": "string", "description": "nama atau email pemilik deal yang dicari (opsional)"}
				}
			}`),
		},
		{
			Name: "search_leads",
			Description: "Cari Lead (prospek belum jadi Deal) berdasarkan nama/kode " +
				"lead dan/atau NAMA PEMILIK (Lead Owner) — dipakai untuk pertanyaan " +
				"lintas-pemilik (mis. 'lead yang dibuat oleh user jun'). Hasil " +
				"TETAP tunduk cakupan data (data_scope) peran penanya: bila " +
				"perannya hanya boleh melihat lead sendiri, lead milik orang lain " +
				"tak akan muncul walau namanya cocok. Semua parameter opsional; " +
				"kosongkan keduanya untuk daftar terbaru sesuai cakupan penanya.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"query": {"type": "string", "description": "nama atau kode lead yang dicari (opsional)"},
					"owner_search": {"type": "string", "description": "nama atau email pemilik lead yang dicari (opsional)"}
				}
			}`),
		},
	}
}
