# ADR-0002 — HTTP+JSON, bukan gRPC atau antrean

**Status:** Diterima · 5 September 2026

## Konteks

Dua layanan harus berbicara. Pilihannya HTTP+JSON, gRPC, atau antrean pesan.

## Keputusan

Satu `POST` sinkron dengan badan JSON, di belakang bearer token.

## Konsekuensi

Payload lebih besar daripada protobuf — tidak relevan pada ~200 KB sekali per permintaan di belakang cache. Tidak ada generator kode dan tidak ada build step tambahan di dua repo.

## Alternatif yang ditolak

**gRPC.** Menambah `.proto`, generator kode, dan build step di dua repo. Keuntungannya nol pada volume ini; biayanya satu hari.

**Antrean (Kafka/RabbitMQ).** `Propose` adalah operasi sinkron yang menunggu jawaban pengguna dalam hitungan detik. Antrean menambah komponen yang harus di-host, dipantau, dan dijelaskan, untuk masalah yang tidak ada.
