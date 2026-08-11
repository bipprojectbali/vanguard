package preflight

import (
	"context"
	"net"
	"os"
	"strings"
)

// check.go — pemeriksaan lengkap lingkungan, dipakai BERSAMA oleh jalur boot
// dan `make doctor`.
//
// Satu sumber, dua pemakai: kalau doctor memeriksa hal yang berbeda dari yang
// menggagalkan boot, ia berubah dari alat bantu jadi jebakan — "doctor bilang
// aman" lalu aplikasi tetap tak mau start.

// Opts = apa yang perlu diperiksa.
type Opts struct {
	DatabaseURL string
	RedisAddr   string
	// Module = nama module dari go.mod. Kosong = lewati pemeriksaan
	// kecocokan nama (tak semua pemanggil tahu, mis. binary produksi).
	Module string
	// AutoCreateDB = boleh membuat database yang belum ada. HANYA dev: di
	// production ini menyembunyikan DSN salah ketik sebagai database kosong
	// yang tampak sehat.
	AutoCreateDB bool
	// FromFile = env berasal dari file .env yang harus ADA di direktori kerja
	// (jalur `make doctor` saat setup). false = env datang dari lingkungan
	// runtime (container/Portainer), tempat .env memang tak ada & tak relevan.
	//
	// Dibutuhkan karena `os.Stat(".env")` menilai working directory: dari dalam
	// container distroless, file itu tak pernah ada, dan memeriksanya menghasilkan
	// diagnosis palsu ".env belum ada" untuk keadaan yang justru normal.
	FromFile bool
}

// Run memeriksa seluruh lingkungan dan mengembalikan laporannya.
//
// Mengumpulkan SEMUA masalah, tak berhenti di yang pertama — memperbaiki satu
// lalu menemukan berikutnya berarti menjalankan perintah yang sama berulang
// kali untuk pertanyaan yang sama: "apa saja yang kurang?".
func Run(ctx context.Context, o Opts) *Report {
	r := &Report{}

	// Pemeriksaan file .env HANYA saat setup dari direktori repo (make doctor).
	// Di runtime container env datang dari lingkungan, jadi ketiadaan .env itu
	// normal — memeriksanya di sana melapor masalah yang tak ada.
	if o.FromFile {
		if _, err := os.Stat(".env"); os.IsNotExist(err) {
			r.Add(EnvMissing())
			// Tanpa .env, DSN-nya pun tak akan terisi — pemeriksaan berikutnya cuma
			// akan menghasilkan kebisingan yang menutupi sebab sebenarnya.
			return r
		}
	}

	t, err := ParseDSN(o.DatabaseURL)
	if err != nil {
		r.Add(Problem{
			What: "DATABASE_URL tak bisa diurai.",
			Why:  firstLine(err.Error()),
			Fix:  []string{"# format: postgres://user@host:5432/namadb?sslmode=disable"},
		})
		return r
	}

	checkPostgres(ctx, r, t, o.AutoCreateDB)
	checkRedis(r, o.RedisAddr)
	checkNames(r, o.Module, t.DBName)
	return r
}

// checkPostgres memeriksa server + database, dan membuat database bila
// diizinkan.
func checkPostgres(ctx context.Context, r *Report, t DBTarget, autoCreate bool) {
	err := CheckDatabase(ctx, t)
	if err == nil {
		return
	}
	if !strings.Contains(err.Error(), ErrDatabaseMissing.Error()) {
		// Bukan "database tak ada" → server yang bermasalah. Dibedakan karena
		// tindakannya berbeda sama sekali.
		r.Add(PostgresDown(t, err))
		return
	}
	if autoCreate {
		if cerr := CreateDatabase(ctx, t); cerr == nil {
			return // dibuat; tak ada yang perlu dilaporkan
		}
		// Gagal membuat → laporkan sebagai "belum ada" beserta cara manualnya.
		// Sebabnya sering izin, dan menyodorkan perintah createdb membiarkan
		// orang menjalankannya sebagai user yang tepat.
	}
	r.Add(DatabaseMissing(t, SimilarDatabases(ctx, t)))
}

// checkRedis memeriksa Redis bisa dihubungi. Dial TCP telanjang, bukan klien
// penuh: yang ingin diketahui cuma "ada yang mendengarkan di sana?", dan klien
// rueidis melakukan retry yang menunda pesan justru saat jawabannya sudah jelas.
func checkRedis(r *Report, addr string) {
	if addr == "" {
		return
	}
	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		r.Add(RedisDown(addr, err))
		return
	}
	_ = conn.Close()
}

// checkNames memperingatkan bila nama module & nama database jauh berbeda.
//
// PERINGATAN, bukan kegagalan: nama yang berbeda sepenuhnya sah (satu database
// dipakai beberapa aplikasi, penamaan internal perusahaan). Yang dicari adalah
// pola khas `make rename` yang belum diikuti .env — dan itu terlihat sebagai
// nama yang MIRIP tapi tak sama, bukan nama yang berbeda sama sekali.
func checkNames(r *Report, module, dbName string) {
	if module == "" || dbName == "" || module == dbName {
		return
	}
	if similar(module, dbName) {
		r.Add(NameMismatch(module, dbName))
	}
}
