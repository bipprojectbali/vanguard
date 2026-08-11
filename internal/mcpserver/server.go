// Package mcpserver mengekspos runtime go_starter sebagai MCP server READ-ONLY,
// supaya agent AI bisa MELIHAT keadaan dev/staging/produksi tanpa akses tulis.
//
// Kenapa ada: aplikasi dijalankan di beberapa lingkungan, tapi agent hanya bisa
// menyentuh mesin lokal. Saat ada masalah di staging/prod, agent buta. Server
// ini jadi "mata"-nya — dan HANYA mata: tiap tool memanggil ulang jalur baca
// yang SUDAH aman, lewat db.WithSuper yang menurunkan hak ke app_rw (DDL ditolak
// database, bukan sekadar dijanjikan kode).
//
// Prinsip yang dijaga ketat: paket ini adalah ADAPTER TIPIS, bukan pintu baru.
// Ia tak boleh memuat query tulis, tak meng-embed maintenance/migrasi, dan tak
// pernah mengalirkan nilai rahasia (Rule 7/12). Menambah tool = memetakan
// fungsi read-only yang ada ke satu handler, bukan menulis logika baru.
package mcpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"go_starter/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// serverName & serverVersion tampil di klien MCP saat handshake `initialize`.
const (
	serverName    = "go_starter-readonly"
	serverVersion = "0.1.0"
)

// toolTimeout membatasi lama satu panggilan tool menyentuh DB.
//
// SDK meneruskan ctx request HTTP APA ADANYA — tanpa deadline. Tanpa batas ini,
// DB yang hang (koneksi staging lambat, lock panjang) membuat tool menggantung
// tak berujung — persis skenario "bug di staging" yang jadi alasan tool ini ada.
// Read-only jadi harus tetap cepat-gagal: lebih baik lapor "timeout" daripada
// diam selamanya. 10 dtk cukup lapang untuk introspeksi skema (yang terberat),
// masih jauh di bawah kesabaran manusia menunggu jawaban agent.
const toolTimeout = 10 * time.Second

// deps = ketergantungan yang dibagi seluruh tool. Dikumpulkan satu kali di build
// lalu ditangkap closure tiap handler (pola internal/maintenance) — jadi handler
// tak perlu menyimpan state sendiri, dan tak ada jalur untuk lupa mengoper pool.
type deps struct {
	pool *pgxpool.Pool
	cfg  *config.Config
	log  *slog.Logger
}

// ctxWith memberi ctx tool sebuah deadline. Tiap handler membungkus ctx-nya
// lewat ini SEBELUM menyentuh DB — satu jalur, jadi tak ada tool yang bisa lupa
// dan menggantung tanpa batas.
func (d *deps) ctxWith(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, toolTimeout)
}

// build merakit *mcp.Server lengkap dengan seluruh tool baca. Dipakai bersama
// oleh jalur HTTP (Handler) & stdio (ServeStdio) — satu sumber tool, dua
// transport, jadi keduanya mustahil menawarkan kemampuan yang berbeda.
func build(pool *pgxpool.Pool, cfg *config.Config, log *slog.Logger) *mcp.Server {
	d := &deps{pool: pool, cfg: cfg, log: log}
	s := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: serverVersion}, nil)
	registerHealthTools(s, d)
	registerActivityTools(s, d)
	registerSchemaTools(s, d)
	return s
}

// Handler mengembalikan http.Handler untuk dipasang di rute /mcp aplikasi.
//
// Ini inti keputusan arsitektur: MCP server BUKAN service/proses terpisah, tapi
// http.Handler biasa yang menumpang aplikasi yang sudah jalan — image sama,
// container sama, reverse proxy sama. Penjagaan (Bearer token) dipasang di
// routes.go sebagai middleware, bukan di sini, agar lapisan auth tetap satu.
//
// Stateless + JSONResponse (pola produksi): tool baca cuma request→response,
// tak perlu membuka stream SSE per POST. Konsekuensinya reverse proxy tak perlu
// setelan buffering/timeout SSE khusus, dan GET/DELETE otomatis 405 (hanya POST
// yang bermakna) — permukaan lebih kecil.
func Handler(pool *pgxpool.Pool, cfg *config.Config, log *slog.Logger) http.Handler {
	srv := build(pool, cfg, log)
	return mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)
}

// ServeStdio menjalankan server yang SAMA lewat stdin/stdout — untuk dev lokal
// (`./app mcp`), tanpa HTTP/token. BLOCKING sampai klien memutus atau ctx batal.
//
// stdout MILIK protokol (framing JSON-RPC): satu baris log ke sana merusak
// pesan. Pemanggil (main.go) WAJIB mengarahkan logger ke stderr.
func ServeStdio(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, log *slog.Logger) error {
	return build(pool, cfg, log).Run(ctx, &mcp.StdioTransport{})
}
