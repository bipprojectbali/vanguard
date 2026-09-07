package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// account_typeahead.go — SATU sumber markup pemilih cari-ketik (typeahead) yang
// dipandu static/accountpicker.js (kontrak [data-account-picker], CSP-safe).
//
// BL-76: sebelumnya tiap form merakit markup picker sendiri (deal/kontak/success
// plan) atau memakai <select> polos (tiket/engagement/training/impl task/target
// aktivitas). Kini semua lewat helper ini agar label + perilaku seragam.
//
// Nilai yang ter-submit bersifat OPAQUE: id akun numerik untuk pemilih desa,
// atau "type:id" untuk target aktivitas polimorfik. accountpicker.js hanya
// memetakan label terketik → atribut data-account-id (string apa pun) ke input
// hidden bernama Name. Backend tetap penjaga terakhir.

// TypeaheadOption = satu opsi picker. Value = string ter-submit (opaque);
// Label = teks tampil & yang diketik untuk mencari.
type TypeaheadOption struct {
	Value string
	Label string
}

// typeaheadPickerConfig = konfigurasi satu picker cari-ketik.
type typeaheadPickerConfig struct {
	Name         string            // nama field hidden ter-submit (mis. "account_id", "target")
	Label        string            // label field
	ListID       string            // id <datalist> (unik per halaman)
	Options      []TypeaheadOption // opsi
	Current      string            // Value terpilih (prefill); "" = tak ada
	CurrentLabel string            // label terpilih; "" → dicari dari Options
	Required     bool
	FullWidth    bool   // tambah sm:col-span-2
	Placeholder  string // placeholder input
	Help         string // teks bantuan di bawah field
	InvalidMsg   string // pesan validity kustom (kosong = default JS)
}

// typeaheadPickerField merender markup picker sesuai kontrak accountpicker.js.
func typeaheadPickerField(cfg typeaheadPickerConfig) g.Node {
	searchID := "f-" + cfg.Name + "_search"
	hiddenID := "f-" + cfg.Name

	// Label prefill: bila Current diberi tanpa CurrentLabel, cari di Options.
	resolvedLabel := cfg.CurrentLabel
	if cfg.Current != "" && resolvedLabel == "" {
		for _, o := range cfg.Options {
			if o.Value == cfg.Current {
				resolvedLabel = o.Label
				break
			}
		}
	}

	opts := make([]g.Node, 0, len(cfg.Options)+1)
	inList := false
	for _, o := range cfg.Options {
		if o.Value == cfg.Current {
			inList = true
		}
		opts = append(opts, h.Option(
			h.Value(o.Label),
			g.Attr("data-account-id", o.Value),
		))
	}
	// Terpilih di luar daftar (prefill edit) → sisipkan agar preselect cocok.
	if cfg.Current != "" && resolvedLabel != "" && !inList {
		opts = append(opts, h.Option(
			h.Value(resolvedLabel),
			g.Attr("data-account-id", cfg.Current),
		))
	}

	search := []g.Node{
		h.ID(searchID), h.Type("text"),
		h.List(cfg.ListID), h.Placeholder(cfg.Placeholder),
		g.Attr("autocomplete", "off"), g.Attr("data-account-search", ""),
		h.Class("input text-base w-full min-h-11"),
	}
	if cfg.Required {
		search = append(search, h.Required())
	}
	if resolvedLabel != "" {
		search = append(search, h.Value(resolvedLabel))
	}

	hidden := []g.Node{
		h.Type("hidden"), h.ID(hiddenID), h.Name(cfg.Name),
		g.Attr("data-account-value", ""),
	}
	if cfg.Current != "" {
		hidden = append(hidden, h.Value(cfg.Current))
	}

	wrapClass := "grid gap-1 min-w-0"
	if cfg.FullWidth {
		wrapClass += " sm:col-span-2"
	}
	wrap := []g.Node{
		h.Class(wrapClass),
		g.Attr("data-account-picker", ""),
	}
	if cfg.InvalidMsg != "" {
		wrap = append(wrap, g.Attr("data-invalid-msg", cfg.InvalidMsg))
	}
	help := cfg.Help
	if help == "" {
		help = "Ketik untuk mencari, lalu pilih desa dari daftar yang muncul."
	}
	wrap = append(wrap,
		labelFor(cfg.Label, searchID, cfg.Required),
		ui.Input(search...),
		h.Input(hidden...),
		h.DataList(append([]g.Node{h.ID(cfg.ListID)}, opts...)...),
		h.P(h.Class("text-xs text-base-content/60"), g.Text(help)),
	)
	return h.Div(wrap...)
}

// accountTypeaheadField = pemilih DESA (value = id akun numerik) — kasus paling
// umum. Menggunakan datalist id "account-options" (satu picker per halaman).
func accountTypeaheadField(name, label string, accounts []TypeaheadOption, required, fullWidth bool) g.Node {
	return typeaheadPickerField(typeaheadPickerConfig{
		Name:        name,
		Label:       label,
		ListID:      "account-options",
		Options:     accounts,
		Required:    required,
		FullWidth:   fullWidth,
		Placeholder: "Ketik kode Kemendagri atau nama desa…",
	})
}
