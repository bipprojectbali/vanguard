package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// regionselect.go — komponen bersama dropdown wilayah administratif berjenjang
// (Provinsi → Kabupaten/Kota → Kecamatan), dipakai AccountForm, LeadForm, dan
// LeadConvert (ADR 0009). Dataset PENUH (~7.817 baris master `regions`) di-embed
// SEKALI per render sbg <script type="application/json"> (pola sama ECharts,
// gotcha #12 — CSP-safe: g.Raw aman karena isinya json.Marshal handler, bukan
// input user), dibaca static/regions.js yang mem-populate 3 <select> murni di
// browser (tanpa round-trip server per pilihan — keputusan desain plan 0009 #5).
// HANYA Kecamatan yang benar-benar ter-submit (name="district_id"); Provinsi &
// Kabupaten/Kota TANPA `name`, cuma alat bantu filter (satu sumber kebenaran =
// district_id, dua level di atasnya diturunkan lewat parent_region_id di JS).

// regionSelect merender embed JSON + 3 <select> berjenjang. embedID membedakan
// beberapa instance komponen dalam satu halaman (jarang >1 di halaman yang sama,
// tapi aman kalau suatu saat ada). regionsJSON = output handler regionsJSON
// (sudah json.Marshal, "[]" bila galat). selectedDistrictID = prefill (kosong =
// belum pilih; form baru selalu kosong, form edit diisi int64PtrStr(district_id)).
// required menandai Kecamatan wajib (jaring klien; pemanggil kirim true HANYA di
// create — village_code diturunkan darinya, jadi tak boleh kosong saat tambah —
// dan false di edit agar baris lama tanpa Kecamatan tetap bisa disimpan).
func regionSelect(embedID, regionsJSON, selectedDistrictID string, required bool) g.Node {
	return g.Group([]g.Node{
		h.Script(
			h.Type("application/json"),
			g.Attr("data-region-json", embedID),
			g.Raw(regionsJSON),
		),
		regionFieldSelect("Provinsi", embedID, "1"),
		regionFieldSelect("Kabupaten/Kota", embedID, "2"),
		regionDistrictSelect(embedID, selectedDistrictID, required),
	})
}

// regionFieldSelect = level 1 (Provinsi) atau 2 (Kabupaten/Kota) — TANPA `name`
// (bukan field form sungguhan, cuma filter berjenjang; lihat komentar file di
// atas). Disabled() awal; static/regions.js mengaktifkan level 1 saat init &
// level berikutnya begitu level di atasnya dipilih.
func regionFieldSelect(label, embedID, level string) g.Node {
	id := "f-region-l" + level + "-" + embedID
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor(label, id, false),
		h.Select(
			h.ID(id), h.Class("select text-base w-full"), h.Disabled(),
			g.Attr("data-region-level", level),
			g.Attr("data-region-group", embedID),
			h.Option(h.Value(""), g.Text("— Pilih "+label+" —")),
		),
	)
}

// regionDistrictSelect = level 3 (Kecamatan) — SATU-SATUNYA select yang benar-
// benar ter-submit (name="district_id"). data-selected-value dibaca
// static/regions.js saat init utk preselect 3 level sekaligus (form edit) dgn
// menelusuri parent_region_id mundur dari district terpilih. required menambah
// h.Required() + penanda "*": aman berdampingan dgn Disabled() awal karena
// static/regions.js melepas disabled saat init (kontrol disabled dikecualikan
// validasi constraint HTML) — jadi wajib baru berlaku setelah select aktif, tepat
// saat user bisa memilih. Penegakan sebenarnya tetap di backend (AccountCreate).
func regionDistrictSelect(embedID, selectedDistrictID string, required bool) g.Node {
	id := "f-district_id-" + embedID
	sel := []g.Node{
		h.ID(id), h.Name("district_id"), h.Class("select text-base w-full"), h.Disabled(),
		g.Attr("data-region-level", "3"),
		g.Attr("data-region-group", embedID),
		g.Attr("data-selected-value", selectedDistrictID),
		h.Option(h.Value(""), g.Text("— Pilih Kecamatan —")),
	}
	if required {
		sel = append(sel, h.Required())
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor("Kecamatan", id, required),
		h.Select(sel...),
	)
}
