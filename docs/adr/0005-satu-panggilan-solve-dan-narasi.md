# ADR-0005 — Satu panggilan (solve + narasi), bukan dua

**Status:** Diterima · 5 September 2026

## Konteks

Menyelesaikan soal dan menuliskan narasinya bisa dipisah menjadi dua endpoint, sehingga kegagalan LLM tidak menyentuh solver.

## Keputusan

Satu panggilan `POST /v1/plan/propose` mengembalikan rencana **dan** narasi. Python mengelola degradasi internalnya sendiri.

## Konsekuensi

Kegagalan LLM tidak boleh menjatuhkan hasil solve — ditegakkan di dalam Python oleh anggaran waktu terpisah (`asyncio.wait_for`) dan oleh urutan yang membangun narasi templat lebih dulu. Tidak ada jalur yang berakhir tanpa narasi.

## Alternatif yang ditolak

**Dua panggilan.** Menggandakan latensi jaringan dan memaksa Go mengelola state antar keduanya, untuk jaminan yang sudah diberikan oleh anggaran waktu di dalam Python.
