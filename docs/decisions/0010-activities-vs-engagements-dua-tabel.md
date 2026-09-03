# 0010 — Activities vs Engagements: dua tabel, bukan satu polimorfik

Status: **Diterima** (2026-09-04) — mendokumentasikan (post-hoc) divergensi
antara rencana di `docs/crm/skema.md` §7 / komentar migrasi `00011` dan realitas
kode migrasi `00022`. Keputusan user 2026-09-03 (opsi a: bela divergensi, jangan
satukan). Konteks backlog: BL-40.

## Konteks

Spec CRM §7 (Activities) merancang SATU tabel polimorfik `activities` untuk
SEMUA jejak interaksi lintas modul, dibedakan lewat kolom diskriminator
`activity_context`:

- Sales Activity Log (4.4) → `activity_context='sales'`
- Customer Success Engagement (6.5) → `activity_context='cs'`
- Activities global (Modul 7) → `activity_context='general'`

Tujuannya: timeline lintas-modul (semua aktivitas satu account/deal/contact)
cukup satu query, penambahan jenis baru tak menuntut tabel + join baru. Komentar
header migrasi `00011` menegaskan ini secara eksplisit — "Dibedakan lewat
`activity_context` … **bukan tabel terpisah**".

Realitas yang terbangun berbeda:

1. **Sales Activity MENGIKUTI rencana** — dibangun di atas tabel `activities`
   dengan `activity_context='sales'` (migrasi `00011`).
2. **CS Engagement MENYIMPANG** — migrasi `00022` membangunnya sebagai tabel
   SENDIRI `engagements`, bukan baris di `activities`.

Divergensi ini dilakukan **diam-diam**: tak ada catatan yang menjelaskan kenapa
`00022` membelok dari rencana, sementara komentar `00011` tetap mengklaim CS
Engagement hidup di tabel yang sama. Dua masalah konkret muncul:

- **Komentar `00011` menyesatkan agen berikutnya** — membaca "bukan tabel
  terpisah", lalu kaget menemukan `engagements` di `00022` (utang doc, CLAUDE.md
  §16/§17).
- **`activity_context='cs'` jadi slot mati** — nilai enum sah tapi **nol query**
  mengisinya; semua engagement CS ada di tabel `engagements`.

## Keputusan

### 1. Pertahankan dua tabel — JANGAN satukan

`activities` (Sales + general) dan `engagements` (CS) tetap tabel terpisah.
Penyatuan ke satu tabel polimorfik **ditolak**: ROI kecil (hanya timeline
gabungan yang diuntungkan, dan itu sudah beres via UNION — lihat §4), sementara
biayanya membuang properti struktural yang membuat `engagements` bernilai.

### 2. Divergensi dibela oleh SEMANTIK entity yang beda

`engagements` **bukan** activity generik dengan konteks berbeda; ia entity
dengan bentuk sendiri yang tak muat di `activities`:

| Aspek | `activities` (Sales/general) | `engagements` (CS) |
|---|---|---|
| Target | Polimorfik (`target_type`+`target_id`, BUKAN FK) | `account_id` FK sungguhan `ON DELETE RESTRICT` |
| Kosakata | `kind` (task/call/note/…) | `engagement_type` (touch_point/qbr/onboarding_call/escalation/check_in), `frequency`, `channel` |
| Siklus hidup | `status` longgar (jejak, tahan hard-delete target) | `planned/done/skipped/rescheduled` (siklus check-in) |
| Jadwal | `due_date DATE` opsional (task) | `scheduled_at TIMESTAMPTZ NOT NULL` + `next_due_date` berulang |
| Sifat | JEJAK (harus bertahan walau target hilang → FK cascade justru merusak) | KOMITMEN account-scoped (integritas account ditegakkan DB via RESTRICT) |

`activities` sengaja **tanpa FK target** justru agar jejak bertahan setelah
target di-hard-delete (lihat header `00011`). `engagements` menuntut kebalikannya
— `account_id` FK dengan `ON DELETE RESTRICT` agar account yang punya engagement
tak bisa dihapus sembarangan. Dua kebutuhan integritas ini **tak bisa dilayani
satu tabel** tanpa kolom bersyarat yang rapuh.

### 3. `activity_context='cs'` = slot mati yang dibiarkan

Nilai `'cs'` tetap di CHECK constraint `00011`
(`activity_context IN ('sales','cs','general')`) — **tidak** di-drop. Alasan:
CHECK adalah DDL di migrasi yang sudah ter-clone/produksi; menyuntingnya melanggar
prinsip "skema di satu migrasi, perubahan berikutnya inkremental" (CLAUDE.md).
Biaya membiarkan `'cs'` = nol (nilai enum sah tapi tak dipakai); biaya mencabut =
migrasi baru + risiko. Slot ini **ditandai sebagai mati** di komentar kolom dan
header `00011` agar tak menjebak — bukan dibersihkan.

### 4. Timeline lintas-modul = UNION dua tabel, bukan satu query

Konsekuensi langsung dua tabel: timeline gabungan Sales + CS di detail Account
memakai `UNION` read-only dua sumber (`activities` + `engagements`),
di-normalisasi ke bentuk baris seragam di layer query — bukan satu `SELECT`.
Ini sudah diimplementasikan (BL-31, linimasa terpadu di detail Account). Perluasan
ke halaman Activities global (`/activity-log`) di-"paper" oleh ADR ini dan
dikerjakan terpisah (BL-41).

## Konsekuensi

- **Komentar `00011` dikoreksi** (BL-40): menyatakan CS Engagement AKHIRNYA
  dipisah di `00022` + alasannya, hapus klaim "bukan tabel terpisah", tandai
  slot mati `activity_context='cs'`. Murni komentar — DDL tak disentuh.
- **Timeline lintas-modul selamanya UNION**, bukan single-table scan. Biaya
  query lebih tinggi diterima sadar sebagai harga semantik entity yang benar.
  Setiap fitur baru yang butuh "semua interaksi satu account" wajib menggabung
  DUA sumber (lihat pola BL-31), bukan mengira satu tabel `activities` cukup.
- **Penambahan jenis interaksi CS** (mis. tipe engagement baru) menyentuh
  `engagements` + `queries/engagements.sql`, TERPISAH dari `activities`. Jangan
  mencoba menambahkan `kind`/`activity_context` baru di `activities` untuk CS.
- **Nol perubahan runtime/DDL/query dari ADR ini sendiri** — ini dokumentasi
  keputusan yang SUDAH terbentuk di kode, bukan perubahan perilaku.

## Di luar cakupan (ditunda / ditolak eksplisit)

- **Menyatukan `activities` + `engagements`** — ditolak (§1); ROI kecil, buang
  FK dan semantik.
- **Mencabut nilai `'cs'` dari CHECK `activity_context`** — ditunda; slot mati
  dibiarkan + ditandai, bukan dimigrasikan keluar (§3).
- **Timeline terpadu di `/activity-log` global** — dikerjakan terpisah (BL-41);
  ADR ini jadi tempat resmi keputusan yang di-"paper" task itu.
