-- 00032_crm_cs_score_trend_history.sql — BL-25: score_trend OTOMATIS dari
-- riwayat skor (arah Naik/Stabil/Turun dihitung sistem, bukan dropdown manual).
--
-- KENAPA: customer_success.score_trend (00019) dulu diisi MANUAL via dropdown —
-- penilaian subjektif operator, bukan arah pergerakan skor sebenarnya. Tabel
-- customer_success 1:1-dengan-desa & hanya menyimpan SNAPSHOT TERKINI (komentar
-- 00019: "kondisi TERKINI… bukan riwayat"); tanpa nilai skor lama, delta tak
-- terhitung. BL-25 menyediakan MEMORI skor sebelumnya lalu score_trend =
-- tanda(skor_sekarang − skor_lalu) dengan dead-band (deriveScoreTrend, handler).
--
-- STRATEGI (Opsi Ringan, keputusan BL-25 v1): DUA kolom di customer_success —
-- previous_health_score + previous_health_calculated_at. Saat section Health
-- ditulis, handler menggeser overall_health_score LAMA → previous_* SEBELUM
-- menulis nilai baru, lalu deriveScoreTrend(previous, sekarang) mengisi
-- score_trend. Tanpa tabel histori baru: cukup untuk trend, tapi hanya ingat
-- SATU nilai lama (tak untuk grafik histori — di luar scope v1).
--
-- previous_health_score        = skor keseluruhan tepat SEBELUM recompute terakhir.
-- previous_health_calculated_at = KAPAN skor lama itu dihitung (jejak audit ringan).
--
-- Kolom NULLABLE tanpa backfill (selaras BL-24b): baris lama tak punya skor lalu
-- → previous_* NULL → score_trend NULL ("—", belum ada dasar), BUKAN "Stable".
-- CHECK rentang mencerminkan cs_overall_health_score_chk (00019). enum
-- cs_score_trend_chk (00019) TAK disentuh — nilai turunan tetap dalam 3-set sah.
-- Idempotent (IF NOT EXISTS) — aman diulang.

-- +goose Up
-- +goose StatementBegin

ALTER TABLE customer_success
    ADD COLUMN IF NOT EXISTS previous_health_score SMALLINT
        CHECK (previous_health_score IS NULL OR previous_health_score BETWEEN 0 AND 100);

ALTER TABLE customer_success
    ADD COLUMN IF NOT EXISTS previous_health_calculated_at TIMESTAMPTZ;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE customer_success DROP COLUMN IF EXISTS previous_health_calculated_at;
ALTER TABLE customer_success DROP COLUMN IF EXISTS previous_health_score;

-- +goose StatementEnd
