# CONTEXT_FIX.md — obat "context habis cepat" untuk agent di project turunan

Project ini di-clone dari template `go_starter`. Template & turunannya pernah
mengalami **context agent habis terlalu cepat** — satu sesi belum banyak bekerja,
window sudah penuh. Dokumen ini adalah OBATNYA: apa penyebabnya, proteksi apa yang
SUDAH terpasang (jangan bongkar), dan cara MEMPERLUAS-nya saat kamu menemukan sumber
baru. Baca sekali di awal; rujuk saat menyentuh dokumentasi besar / aset vendored.

Ini panduan operasional agent, BUKAN acuan arsitektur — untuk itu baca `CLAUDE.md`.

---

## Gejala

- Satu `grep`/pencarian saja langsung memuntahkan puluhan ribu token ke context.
- Baca ulang dokumen besar (CLAUDE.md, CHANGELOG, arsip) tiap sesi tanpa perlu.
- "Kok baru beberapa tool-call sudah mepet limit?"

## Akar penyebab (tiga, berbeda sifat)

1. **File berbaris raksasa ter-grep.** Aset vendored/minified & CSS generated punya
   baris tunggal ratusan ribu karakter (`static/app.css` = 1 baris ~164k char;
   `echarts/mermaid.min.js` sejenis). Ripgrep mengembalikan **seluruh baris** yang
   match → satu match = ledakan token. Nilai bacanya NOL (minified/generated).
2. **Dokumen auto-inject membengkak.** `CLAUDE.md` disuntik ke context **tiap sesi**.
   Tiap 1.000 char di sana ≈ ~250 token yang kamu bayar SETIAP KALI, dikerjakan atau
   tidak. Rasional bertele-tele yang sudah ada di `docs/decisions/` = pajak berulang.
3. **Arsip usang tercampur pencarian.** Dokumen historis (spec asli, CHANGELOG lama)
   yang sudah di-supersede tetap muncul di hasil grep → agent membacanya sebagai acuan
   dan menghabiskan token pada informasi yang justru MENYESATKAN.

---

## Obat yang SUDAH terpasang (jangan bongkar)

### 1. `.ignore` (ripgrep — BUKAN `.gitignore`)

File di root yang menyembunyikan sumber ledakan dari pencarian ripgrep/agent, **tanpa**
mengeluarkannya dari git. Yang tercakup: `static/app.css`, `static/*.min.js`,
`static/daisyui.js`, `static/datastar.js`, dan `docs/archive/`.

> **WAJIB paham bedanya `.ignore` vs `.gitignore`:** target `.ignore` TETAP di-commit &
> **ter-embed via `embed.FS`** ke binary. Menaruhnya di `.gitignore` akan membuat build
> single-binary KEHILANGAN aset (CSS/JS hilang saat runtime). `.ignore` hanya soal
> **visibilitas pencarian**, bukan distribusi. Jangan pernah pindahkan entri ini ke
> `.gitignore`.

Cakupan sengaja **sempit**: `charts.js`/`erd.js`/`health.js`/`sidebar.js`/`theme.js`/
`input.css` tetap tersearch (kecil & berguna), dan generated Go (`internal/db/*.go`)
sengaja dibiarkan (kamu perlu grep signature query). Jangan lebarkan tanpa alasan.

### 2. Arsip terpisah & grep-hidden (`docs/archive/`)

Dokumen historis yang di-supersede dipindah ke `docs/archive/` (mis. spec desain asli,
riwayat CHANGELOG fondasi awal) lalu di-hide lewat `.ignore`. Tetap di-commit sebagai
sejarah, tapi tak lagi mencemari pencarian. **Acuan mutakhir selalu**: `CLAUDE.md`,
`docs/decisions/`, `CHANGELOG.md` (root).

### 3. CLAUDE.md dedup konservatif

Rasional panjang yang sudah tercatat di `docs/decisions/000X` diciutkan menjadi
**pointer satu baris** ke decision-nya; SETIAP gotcha/aturan actionable dipertahankan
**verbatim**. Header tiap section menyebut nomor decision-nya sebagai jalan ke rasional
lengkap. Prinsipnya: yang dibaca tiap sesi = ringkas & actionable; yang panjang &
jarang-perlu = on-demand di `docs/decisions/`.

---

## Cara VERIFIKASI `.ignore` benar-benar bekerja

**Gotcha yang menipu:** `rg --files <path-eksplisit>` **MEM-BYPASS `.ignore`** — path
CLI eksplisit selalu di-list apa pun aturannya. Jadi `rg --files docs/archive/` yang
menampilkan file arsip BUKAN bukti `.ignore` gagal.

Uji dengan **content search dari root** (persis cara tool Grep agent berjalan): cari
string yang HANYA ada di file ter-hide. Kalau nihil, `.ignore` bekerja.

```sh
# app.css token — harus NIHIL (ter-hide)
rg --color=never 'color-base-100' static/
# string khas arsip — harus NIHIL (ter-hide)
rg --color=never 'STARTER asli' .
# kontrol: input.css TIDAK di-hide — harus MATCH
rg --color=never '@plugin' static/
```

> Catatan: `rg` telanjang di macOS bisa me-resolve ke BSD grep. Pakai path absolut
> ripgrep bila ragu (`/opt/homebrew/bin/rg`).

---

## Playbook: menemukan sumber ledakan BARU

Saat sebuah pencarian tiba-tiba membanjiri context, atau kamu menambah aset baru:

1. **Identifikasi.** File apa yang match & sepanjang apa barisnya?
   ```sh
   # cari baris terpanjang per file (kandidat generated/minified)
   awk '{ if (length > max[FILENAME]) max[FILENAME]=length } END { for (f in max) print max[f], f }' \
     $(git ls-files 'static/*' 'assets/*') | sort -rn | head
   ```
2. **Putuskan layak-baca atau tidak.** Generated/minified/vendored/arsip = NOL nilai
   baca → hide. Sumber yang kamu memang perlu grep (kode, config kecil) = JANGAN hide.
3. **Tambah ke `.ignore`** (bukan `.gitignore`) dengan komentar `#` alasannya.
4. **Verifikasi** dengan content search dari root (lihat atas), bukan `--files`.
5. **Jangan** commit/push kecuali diminta eksplisit (lihat aturan project).

---

## Prinsip menulis dokumen agar hemat context

Berlaku saat kamu menyunting `CLAUDE.md` / dokumen yang ikut ter-clone:

- **Gotcha actionable = verbatim.** "Kenapa" ringkas anti-rediscovery WAJIB tetap ada —
  itu justru yang mencegah agent berikutnya menemukan-ulang bug mahal. JANGAN pangkas.
- **Rasional panjang → pointer.** Sejarah "dulu X kini Y", anekdot "pernah terjadi",
  angka terukur, motivasi bertele-tele: ciutkan jadi rujukan ke `docs/decisions/000X`.
  Menghapus rasional TANPA meninggalkan pointer = melanggar; selalu tinggalkan jalan.
- **Arsip, jangan hapus.** Dokumen usang → `docs/archive/` + hide via `.ignore`, bukan
  `rm`. Sejarah tetap ada di git; pencarian tetap bersih.
- **Satu fakta satu tempat.** Bila hal sama muncul di CLAUDE.md dan decision, CLAUDE.md
  memuat versi operasional ringkas + pointer; decision memuat versi lengkap.

---

## JANGAN (ringkas)

- ❌ Memindahkan entri `.ignore` ke `.gitignore` (aset embed hilang dari binary).
- ❌ Memakai `rg --files <path>` sebagai bukti `.ignore` bekerja (bypass menyesatkan).
- ❌ Membuang gotcha/aturan actionable demi "menghemat baris".
- ❌ Menghapus rasional dari CLAUDE.md tanpa pointer ke decision-nya.
- ❌ Menghide file yang memang perlu kamu grep (kode, config kecil, `internal/db/*.go`).
- ❌ `rm` dokumen usang (arsipkan + hide).

## Checklist cepat

- [ ] Pencarian membanjiri context? → cari baris terpanjang → hide di `.ignore` bila
      generated/vendored/arsip.
- [ ] Verifikasi via **content search dari root**, bukan `--files`.
- [ ] Nyunting CLAUDE.md? → gotcha verbatim, rasional panjang jadi pointer decision.
- [ ] Dokumen usang? → `docs/archive/` + entri `.ignore`, bukan dihapus.
- [ ] Target `.ignore` tetap di-commit (cek `git status` — JANGAN sampai ter-gitignore).
