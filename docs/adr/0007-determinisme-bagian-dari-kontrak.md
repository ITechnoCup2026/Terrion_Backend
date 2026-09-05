# ADR-0007 — Determinisme adalah bagian kontrak

**Status:** Diterima · 5 September 2026

## Konteks

Optimizer boleh saja mengembalikan jawaban yang sedikit berbeda tiap kali dijalankan. Itu lazim, dan biasanya tidak dianggap masalah.

## Keputusan

Permintaan yang sama dengan `seed` yang sama wajib menghasilkan rencana yang identik. `seed` wajib ada di kontrak. CP-SAT dijalankan dengan `num_search_workers=1` dan `random_seed`; Monte Carlo memakai RNG ber-seed per permintaan, bukan RNG global; setiap koleksi di solver adalah tuple terurut; setiap perbandingan floating point memakai toleransi `1e-12`.

## Konsekuensi

Kehilangan paralelisme solver — tidak terasa, karena masalahnya kecil. Sebagai gantinya, cache di Go bekerja, sistem bisa diaudit, dan pengurus koperasi yang membuka layar yang sama dua kali melihat rencana yang sama. Harga lain: eksplorasi acak (ε-greedy, Thompson sampling) untuk varietas yang jarang ditanam menjadi terlarang; ini dinyatakan di model card sebagai batasan yang diketahui.

## Alternatif yang ditolak

**Menerima non-determinisme.** Sistem yang tidak bisa mengulangi jawabannya tidak bisa diaudit, tidak bisa didebug, dan tidak bisa dipertanggungjawabkan ketika panennya meleset.
