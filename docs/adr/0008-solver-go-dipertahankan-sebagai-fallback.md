# ADR-0008 — Solver Go dipertahankan sebagai fallback, bukan dibuang

**Status:** Diterima · 5 September 2026

## Konteks

Setelah CP-SAT ada di Python, solver greedy di Go menjadi kode yang tidak dipakai pada jalur normal.

## Keputusan

`internal/planning` tetap dibangun, diuji, dan dipelihara. Kalau `AI_SERVICE_URL` kosong, layanan AI mati, timeout, atau circuit breaker terbuka, `PlanningUseCase` memakainya dan respons menyebut `engine: "fallback"`.

## Konsekuensi

Dua implementasi solver yang harus dipelihara. Keduanya kecil, dan satu adalah **jaminan ketersediaan**: layanan Python tidak pernah menjadi jalur kritis, sehingga menambahkannya satu hari sebelum tenggat tidak berisiko.

## Alternatif yang ditolak

**Membuang solver Go.** Menjadikan layanan Python dependensi keras: kalau ia gagal di-deploy, fiturnya hilang dari pengumpulan.

**Menampilkan galat saat AI mati.** Setiap baris tabel galat berakhir di fallback justru supaya tidak ada galat layanan AI yang sampai ke pengguna.
