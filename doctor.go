package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go_starter/internal/config"
	"go_starter/internal/preflight"
)

// doctor.go — `./app doctor` (lewat `make doctor`): periksa lingkungan tanpa
// menjalankan apa pun.
//
// Memakai preflight yang SAMA dengan jalur boot. Kalau doctor memeriksa hal
// yang berbeda dari yang menggagalkan boot, ia berubah dari alat bantu jadi
// jebakan — "doctor bilang aman" lalu aplikasi tetap menolak start, dan
// kepercayaan pada alatnya habis pada kejadian pertama.
//
// SENGAJA tak memanggil config.MustLoad(): MustLoad memanggil os.Exit saat env
// wajib kosong, dan doctor yang mati sebelum sempat melaporkan apa pun adalah
// kebalikan dari gunanya. Env dibaca langsung, lalu ketiadaannya dilaporkan
// sebagai temuan — bukan sebagai kematian.
func runDoctor() int {
	_ = config.LoadDotEnv(".env")

	rep := preflight.Run(context.Background(), preflight.Opts{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisAddr:   os.Getenv("REDIS_ADDR"),
		Module:      moduleName(),
		// TIDAK membuat database sendiri: doctor adalah PEMERIKSAAN. Alat
		// diagnosis yang diam-diam mengubah keadaan membuat orang tak bisa lagi
		// memakainya untuk menjawab "apa yang sebenarnya terjadi di sini?".
		AutoCreateDB: false,
		// doctor dijalankan saat SETUP dari direktori repo — .env memang harus
		// ada di sini, dan ketiadaannya justru masalah pertama yang dilaporkan.
		FromFile: true,
	})
	rep.Add(sessionKeyProblem()...)

	fmt.Println(rep.String())
	if rep.OK() {
		fmt.Println("\nJalankan: make dev")
		return 0
	}
	return 1
}

// sessionKeyProblem memeriksa SESSION_KEY — panjangnya, bukan sekadar
// keberadaannya (lihat § "gagal keras" di CLAUDE.md: kunci lemah lebih
// berbahaya daripada kosong, sebab kosong menggagalkan boot sedangkan lemah
// menciptakan rasa aman yang keliru).
//
// Mengembalikan slice supaya "tak ada masalah" tak perlu diwakili nilai kosong
// yang harus diperiksa pemanggil.
func sessionKeyProblem() []preflight.Problem {
	key := os.Getenv("SESSION_KEY")
	switch {
	case key == "":
		return []preflight.Problem{{
			What: "SESSION_KEY belum diisi.",
			Why:  "Kunci ini menandatangani cookie sesi; tanpanya aplikasi menolak start.",
			Fix:  []string{"openssl rand -hex 32   # salin hasilnya ke SESSION_KEY di .env"},
		}}
	case len(key) < config.MinSessionKeyLen:
		return []preflight.Problem{{
			What: fmt.Sprintf("SESSION_KEY terlalu pendek (%d karakter, minimal %d).",
				len(key), config.MinSessionKeyLen),
			Why: "Kunci lemah lebih berbahaya daripada kosong: yang kosong menggagalkan " +
				"boot, yang lemah menciptakan rasa aman yang keliru.",
			Fix: []string{"openssl rand -hex 32   # salin hasilnya ke SESSION_KEY di .env"},
		}}
	}
	return nil
}

// moduleName membaca nama module dari go.mod. "" bila tak terbaca — mis. binary
// produksi yang jalan di luar repo, tempat pemeriksaan kecocokan nama memang
// tak berlaku.
func moduleName() string {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(b), "\n")
	return strings.TrimSpace(strings.TrimPrefix(line, "module "))
}
