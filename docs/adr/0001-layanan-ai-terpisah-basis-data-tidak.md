# ADR-0001 — Layanan AI terpisah, basis data tidak

**Status:** Diterima · 5 September 2026

## Konteks

Fitur rencana tanam punya tiga lapis: model agronomi (L1), perencana kombinatorial (L2), dan agen narasi (L3). L1 sudah ada di Go, teruji, dan terikat pada entitas serta cuaca di basis data. L2 diangkat dari greedy ke CP-SAT dan L3 membutuhkan klien LLM — keduanya jauh lebih murah di Python. Pekerjaan juga dibagi ke dua orang dengan bidang berbeda.

## Keputusan

L2 dan L3 pindah ke repo terpisah `Terrion_AI` (Python). L1 dan **seluruh kepemilikan data** tetap di Go. Layanan Python tidak punya koneksi basis data, kredensial Supabase, atau konsep pengguna. Pemisahan ini memindahkan *komputasi*, bukan *kepemilikan data*.

## Konsekuensi

Satu hop jaringan pada jalur `Propose`, dibayar dengan cache Redis dan solver fallback. Dua target deploy. Sebagai gantinya: dua orang bisa bekerja tanpa saling menunggu, dan radius ledakan kalau layanan AI jebol adalah 'saran tanam yang salah', bukan kebocoran data anggota.

## Alternatif yang ditolak

**Python memiliki L1 juga.** Perlu akses basis data atau payload cuaca raksasa, dan menduplikasi logika yang sudah lulus uji di Go — dua sumber kebenaran untuk angka yang sama.

**Python memiliki basis data sendiri.** Dua pemilik skema adalah kelas bug yang tidak akan selesai dalam sisa waktu.

**Tetap satu repo.** Masih benar untuk premis satu orang pengembang dan lingkup AI tanpa ekosistem Python; kedua premis itu berubah.
