# ADR-0004 — Referensi buram per-permintaan, bukan UUID

**Status:** Diterima · 5 September 2026

## Konteks

Payload ke layanan AI memuat lahan, komoditas, dan varietas. Memakai UUID asli akan gratis secara implementasi.

## Keputusan

Go membangkitkan referensi buram `p1`, `k1`, `v3` **ulang pada setiap permintaan**. Pemetaan `p1 → uuid` hidup di memori proses selama satu permintaan dan tidak pernah ditulis ke mana pun. Sisi Python memvalidasi `^[pkv][0-9]+$` dan menolak UUID dengan `422`.

## Konsekuensi

Go harus memegang pemetaan dan memetakan balik jawaban — dan itu justru yang memaksa validasi ulang di ADR-0006. Layanan AI, bahkan kalau seluruh lognya disimpan selamanya, tidak bisa merakit profil lintas waktu untuk satu lahan tertentu.

## Alternatif yang ditolak

**UUID asli.** Stabil lintas permintaan, sehingga log lama bisa dirakit menjadi riwayat satu lahan. Gratis untuk diimplementasikan, dan itulah persisnya kenapa ia salah.

**Referensi stabil tapi di-hash.** Tetap stabil lintas permintaan; hash tidak mengubah sifat yang menjadi masalah.
