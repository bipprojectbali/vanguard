package dev

import (
	"net/url"
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// logs_trail.go — tabel "Jejak Aktivitas" di panel /dev/logs.
//
// Menggantikan tabel "Event autentikasi" yang hanya menampilkan login/logout,
// sementara 14 jenis aksi lain ikut tercatat dan tak pernah dilihat siapa pun.

// TrailRow = satu peristiwa siap tampil. Kalimatnya sudah dirakit handler
// (activity.Sentence) — view tak menyusun kalimat sendiri, sebab menyusunnya
// butuh nama orang, dan itu berarti nama harus dioper ke sini untuk dirangkai,
// bukan sekadar dicetak.
type TrailRow struct {
	Sentence string
	Family   string // "auth" | "workspace" | ... (untuk badge)
	Action   string // kode mentah, ditampilkan kecil sebagai rujukan
	When     string
}

// ActorOption = satu orang di dropdown filter, dengan jumlah jejaknya.
type ActorOption struct {
	ID     int64
	Label  string
	Events int64
}

// FamilyOption = satu jenis aksi di filter, dengan jumlahnya.
type FamilyOption struct {
	Key    string
	Label  string
	Events int64
}

// TrailView = seluruh keadaan tabel jejak: isi + filter aktif + paginasi.
//
// Satu struct, bukan daftar parameter: dengan tiga string dan dua slice
// bersebelahan, pemanggil yang tertukar urutannya tetap lolos compiler dan
// salahnya baru terlihat sebagai filter yang menyaring hal yang keliru.
type TrailView struct {
	Rows       []TrailRow
	Families   []FamilyOption
	Actors     []ActorOption
	Range      string // rentang aktif — WAJIB ikut di tiap tautan filter
	Family     string // filter jenis aksi yang sedang aktif
	ActorID    int64  // filter orang yang sedang aktif
	NextCursor string
	After      string // BL-7: cursor pembuka halaman ini
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
}

// trailURL merakit tautan panel dengan mempertahankan filter LAIN yang sedang
// aktif. Ini bukan kerapian: tanpa itu, memilih "Workspace" akan membuang
// rentang yang baru saja dipilih, dan halaman menjawab pertanyaan yang tak
// ditanyakan siapa pun.
//
// Cursor SENGAJA tak pernah dibawa: mengubah filter berarti daftar yang lain,
// dan posisi di daftar lama tak punya arti di daftar baru — membawanya akan
// membuka halaman kosong di tengah hasil yang sebenarnya ada.
func trailURL(v TrailView, family string, actorID int64, cursor string) string {
	q := url.Values{}
	if v.Range != "" && v.Range != "day" {
		q.Set("range", v.Range)
	}
	if family != "" {
		q.Set("act", family)
	}
	if actorID != 0 {
		q.Set("by", strconv.FormatInt(actorID, 10))
	}
	if cursor != "" {
		q.Set("after", cursor)
	}
	if len(q) == 0 {
		return "/dev/logs"
	}
	return "/dev/logs?" + q.Encode()
}

// TrailCard merender kartu jejak aktivitas lengkap dengan filternya.
func TrailCard(v TrailView) g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 mb-4 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold"), g.Text("Jejak Aktivitas")),
			h.P(h.Class("text-sm text-base-content/70 mb-3"),
				g.Text("Siapa melakukan apa, pada rentang waktu yang dipilih di atas.")),
			trailFilters(v),
			trailTable(v),
			trailPager(v),
		),
	)
}

// trailFilters = dua baris filter: jenis aksi (chip) & pelaku (dropdown).
//
// Chip untuk jenis aksi karena jumlahnya sedikit & tetap — terlihat sekaligus,
// dan angkanya memberi tahu mana yang berisi sebelum diklik. Dropdown untuk
// orang karena daftarnya tumbuh mengikuti jumlah user.
func trailFilters(v TrailView) g.Node {
	return h.Div(
		h.Class("flex flex-col gap-2 mb-3 min-w-0"),
		trailFamilyChips(v),
		g.If(len(v.Actors) > 0, trailActorSelect(v)),
	)
}

func trailFamilyChips(v TrailView) g.Node {
	chip := func(key, label string, count int64, active bool) g.Node {
		cls := "btn btn-sm min-h-11"
		if active {
			cls += " btn-primary"
		} else {
			cls += " btn-ghost"
		}
		text := label
		if count > 0 {
			text += " (" + itoa64(count) + ")"
		}
		return h.A(h.Href(trailURL(v, key, v.ActorID, "")), h.Class(cls), g.Text(text))
	}
	chips := []g.Node{chip("", "Semua", 0, v.Family == "")}
	for _, f := range v.Families {
		chips = append(chips, chip(f.Key, f.Label, f.Events, v.Family == f.Key))
	}
	// flex-wrap WAJIB: chip-nya bisa banyak dan tak muat satu baris di 375px;
	// tanpa membungkus ia mendorong lebar halaman (konvensi mobile-first).
	return h.Div(h.Class("flex flex-wrap items-center gap-1 min-w-0"), g.Group(chips))
}

// trailActorSelect = dropdown pelaku. Form NATIVE GET (bukan Datastar): memilih
// orang mengubah ALAMAT halaman, jadi hasilnya harus bisa di-bookmark & dimuat
// ulang — sekaligus lolos gotcha #16 tanpa menyentuhnya.
//
// Rentang & jenis aksi dibawa sebagai hidden input: form GET mengganti SELURUH
// query string, jadi yang tak disertakan akan hilang diam-diam.
func trailActorSelect(v TrailView) g.Node {
	opts := []g.Node{
		h.Option(h.Value("0"), g.If(v.ActorID == 0, h.Selected()), g.Text("Semua orang")),
	}
	for _, a := range v.Actors {
		opts = append(opts, h.Option(
			h.Value(strconv.FormatInt(a.ID, 10)),
			g.If(v.ActorID == a.ID, h.Selected()),
			g.Text(a.Label+" ("+itoa64(a.Events)+")"),
		))
	}
	hidden := []g.Node{}
	if v.Range != "" && v.Range != "day" {
		hidden = append(hidden, h.Input(h.Type("hidden"), h.Name("range"), h.Value(v.Range)))
	}
	if v.Family != "" {
		hidden = append(hidden, h.Input(h.Type("hidden"), h.Name("act"), h.Value(v.Family)))
	}
	return h.FormEl(
		h.Method("get"), h.Action("/dev/logs"),
		h.Class("flex flex-wrap items-center gap-2 min-w-0"),
		g.Group(hidden),
		h.Label(h.Class("text-sm text-base-content/70"), h.For("trail-actor"), g.Text("Oleh:")),
		// text-base (≥16px) agar iOS tak auto-zoom saat select difokus.
		h.Select(h.ID("trail-actor"), h.Name("by"),
			h.Class("select select-sm text-base min-h-11 max-w-full"), g.Group(opts)),
		h.Button(h.Type("submit"), h.Class("btn btn-sm min-h-11"), g.Text("Terapkan")),
	)
}

func trailTable(v TrailView) g.Node {
	if len(v.Rows) == 0 {
		// Pesan kosong menyebut FILTER-nya, bukan cuma "tak ada data": halaman
		// kosong karena filter dan halaman kosong karena memang sepi terlihat
		// persis sama, dan yang pertama membuat orang mengira ada yang rusak.
		msg := "Belum ada aktivitas pada rentang ini."
		if v.Family != "" || v.ActorID != 0 {
			msg = "Tak ada aktivitas yang cocok dengan filter ini. Coba pilih \"Semua\"."
		}
		return h.P(h.Class("text-sm text-base-content/70 py-2"), g.Text(msg))
	}
	rows := make([]g.Node, 0, len(v.Rows))
	for _, t := range v.Rows {
		rows = append(rows, h.Tr(
			h.Class("border-b border-base-300"),
			h.Td(h.Class("py-2 pr-4 min-w-0"), h.Div(
				h.Class("min-w-0"),
				h.Div(h.Class("break-words"), g.Text(t.Sentence)),
				// Kode mentah sebagai rujukan kecil: kalimatnya untuk dibaca, kodenya
				// untuk dicari di kode sumber saat menyelidiki lebih jauh.
				h.Div(h.Class("text-xs text-base-content/50 font-mono break-words"), g.Text(t.Action)),
			)),
			h.Td(h.Class("py-2 pr-4"), h.Span(h.Class(familyBadge(t.Family)), g.Text(t.Family))),
			h.Td(h.Class("py-2 whitespace-nowrap"), g.Text(t.When)),
		))
	}
	return ui.TableScroll(h.Table(
		h.Class("w-full text-sm"),
		h.THead(h.Tr(
			h.Class("border-b border-base-300 text-left text-base-content/70"),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Peristiwa")),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Jenis")),
			h.Th(h.Class("py-2 font-medium"), g.Text("Waktu")),
		)),
		h.TBody(g.Group(rows)),
	))
}

// familyBadge memberi warna per keluarga aksi. Token SEMANTIK daisyUI, bukan
// warna absolut: token didefinisikan ulang tiap tema (gotcha #11).
//
// Yang menghapus/menangguhkan diberi `error`, yang mengubah wewenang diberi
// `warning` — warna di sini bukan hiasan melainkan penanda seberapa besar
// akibatnya, dan itulah yang dicari mata saat menyapu daftar panjang.
func familyBadge(fam string) string {
	switch fam {
	case "auth":
		return "badge badge-ghost"
	case "member":
		return "badge badge-warning"
	case "user":
		return "badge badge-error"
	case "workspace":
		return "badge badge-info"
	case "settings", "platform":
		return "badge badge-secondary"
	default:
		return "badge badge-neutral"
	}
}

// trailPager = jalan ke peristiwa yang lebih lama. Link biasa (navigasi, harus
// bisa di-bookmark). Tap target 44px: ini kontrol utama, bukan aksi baris.
func trailPager(v TrailView) g.Node {
	// baseHref = tautan kanonik yang mempertahankan filter range/act/by TANPA
	// after (ditambah oleh helper pager). Trail memberi "Sebelumnya" sungguhan.
	return ui.KeysetPager(trailURL(v, v.Family, v.ActorID, ""), v.After, v.Trail, v.NextCursor)
}
