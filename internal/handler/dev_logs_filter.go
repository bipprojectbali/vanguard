package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/ui/pages/dev"
)

// dev_logs_filter.go — penyaringan jejak aktivitas dari query string, plus
// pemetaan opsi filternya. Dipisah dari dev_logs_trail.go (yang membaca DB):
// yang di sini murni menafsirkan MASUKAN, dan aturannya tumbuh mengikuti apa
// yang boleh diketik orang di URL — bukan mengikuti bentuk datanya.

// trailFilter = penyaringan yang diminta lewat query string.
type trailFilter struct {
	family  string // keluarga aksi ("workspace", "auth", ...); "" = semua
	actorID int64  // 0 = semua orang
}

// parseTrailFilter membaca ?act= & ?by= dari request.
//
// Nilai tak dikenal diabaikan (jadi "tanpa filter"), BUKAN ditolak: keduanya
// datang dari URL yang bisa disunting atau di-bookmark sebelum daftar aksinya
// berubah, dan halaman penuh selalu jawaban yang benar untuk filter yang tak
// bisa dipenuhi. Pola yang sama dengan cursor rusak (pagecursor.go).
func parseTrailFilter(r *http.Request) trailFilter {
	q := r.URL.Query()
	var f trailFilter
	if fam := q.Get("act"); isPlainWord(fam) {
		f.family = fam
	}
	if by := q.Get("by"); by != "" {
		if id, err := strconv.ParseInt(by, 10, 64); err == nil && id > 0 {
			f.actorID = id
		}
	}
	return f
}

// isPlainWord membatasi nilai filter ke huruf kecil & underscore.
//
// Bukan soal injeksi (sqlc memarameterkan semuanya), melainkan WILDCARD: nilai
// ini masuk ke `LIKE '<nilai>%'`, jadi `%` atau `_` dari user akan berperan
// sebagai pola dan diam-diam mengubah arti filternya — `_` mencocoki satu
// karakter apa pun, sehingga "a_th" ikut menyaring "auth" tanpa ada yang
// memintanya.
func isPlainWord(s string) bool {
	if s == "" || len(s) > 32 {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && r != '_' {
			return false
		}
	}
	return true
}

// identOf memilih penanda orang: nama bila ada, selainnya email.
//
// Email TIDAK disamarkan di sini — berbeda dari halaman anggota (0008). Yang
// membaca panel ini adalah operator platform, dan pekerjaannya justru
// mencocokkan orang; nama tak unik dan nilainya dikendalikan usernya sendiri di
// Google, jadi tanpa email jejak audit kehilangan gunanya sebagai bukti.
func identOf(name, email *string) string {
	if name != nil && strings.TrimSpace(*name) != "" {
		return *name
	}
	return deref(email)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// familyLabel menerjemahkan keluarga aksi ke label Indonesia. Yang tak dikenal
// dipakai apa adanya — keluarga baru muncul begitu ada aksi baru ditulis, dan
// label mentah lebih baik daripada opsi yang hilang dari filter.
func familyLabel(fam string) string {
	switch fam {
	case "auth":
		return "Masuk & keluar"
	case "member":
		return "Keanggotaan"
	case "invite":
		return "Undangan"
	case "workspace":
		return "Workspace"
	case "user":
		return "Akun user"
	case "settings":
		return "Pengaturan"
	case "platform":
		return "Platform"
	default:
		return fam
	}
}

// trailViewOf merakit TrailView dari potongan-potongan yang sudah dihitung.
// Dikumpulkan di satu tempat agar handler tak merakit struct berisi delapan
// field di tengah alurnya.
func trailViewOf(r *http.Request, rng string, rows []dev.TrailRow, next string,
	fams []dev.FamilyOption, actors []dev.ActorOption) dev.TrailView {
	f := parseTrailFilter(r)
	return dev.TrailView{
		Rows:       rows,
		Families:   fams,
		Actors:     actors,
		Range:      rng,
		Family:     f.family,
		ActorID:    f.actorID,
		NextCursor: next,
		After:      r.URL.Query().Get("after"),
		Trail:      pageTrail(r),
	}
}
