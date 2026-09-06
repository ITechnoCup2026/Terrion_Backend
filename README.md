# Terrion Backend

[![Go Report Card](https://goreportcard.com/badge/github.com/ITechnoCup2026/Terrion_Backend)](https://goreportcard.com/report/github.com/ITechnoCup2026/Terrion_Backend)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.25.6-00ADD8?logo=go)](go.mod)
[![Architecture](https://img.shields.io/badge/Architecture-Clean%20Architecture-blue)](#arsitektur-sistem--rekayasa-perangkat-lunak)
[![Platform](https://img.shields.io/badge/Deploy-Railway%20%7C%20Docker-blueviolet)](Dockerfile)

API HTTP backend untuk **Terrion**, sistem pelacakan lahan cerdas, proyeksi panen, dan perencanaan tanam musiman terpadu bagi koperasi tani Indonesia (KUD/kelompok tani). Dibangun menggunakan bahasa pemrograman **Go** dengan kerangka HTTP **Fiber v2**, ORM **GORM**, penyimpanan relasional **Postgres (Supabase)**, caching dan manajemen sesi di **Redis (Upstash)**, serta integrasi identitas bersama **Supabase Auth**.

Repo dalam ekosistem Terrion:
* **`Terrion_Backend`** (Repo ini) — *Core business engine*, manajemen transaksi, isolasi *tenancy*, mesin agronomi (GDD, ridge regression, empirical Bayes), solver optimasi tanam bawaan (*fallback*), dan integrasi API publik.
* **`Terrion_Frontend`** — Aplikasi web antarmuka pengguna berbasis **Next.js 16** (App Router, Tailwind CSS, TypeScript).
* **`Terrion_AI`** (Layanan Opsional) — Layanan pendukung berbasis **Python (FastAPI)** yang menyediakan optimasi kombinatorial lanjutan menggunakan Google OR-Tools (CP-SAT), simulasi Monte Carlo (NumPy), serta penalaran narasi bahasa alami.

---

## Daftar Isi

1. [Penjelasan Aplikasi & Latar Belakang Masalah](#1-penjelasan-aplikasi--latar-belakang-masalah)
2. [Fitur-Fitur Utama](#2-fitur-fitur-utama)
3. [Arsitektur Sistem & Rekayasa Perangkat Lunak](#3-arsitektur-sistem--rekayasa-perangkat-lunak)
4. [Teknologi yang Digunakan](#4-teknologi-yang-digunakan)
5. [Struktur Direktori Repositori](#5-struktur-direktori-repositori)
6. [Variabel Lingkungan (.env)](#6-variabel-lingkungan-env)
7. [Panduan Instalasi & Menjalankan Aplikasi](#7-panduan-instalasi--menjalankan-aplikasi)
8. [CLI Tools & Eksekusi Perintah Terminal](#8-cli-tools--eksekusi-perintah-terminal)
9. [Dokumentasi & Kontrak API HTTP](#9-dokumentasi--kontrak-api-http)
10. [Logika Domain & Mesin Agronomi Inti](#10-logika-domain--mesin-agronomi-inti)
11. [Keamanan, Privasi, & AI Responsif](#11-keamanan-privasi--ai-responsif)
12. [Pengujian & Jaminan Mutu (Testing)](#12-pengujian--jaminan-mutu-testing)
13. [Panduan Deployment Produksi](#13-panduan-deployment-produksi)
14. [Pedoman Kontribusi & Konvensi Kode](#14-pedoman-kontribusi--konvensi-kode)

---

## 1. Penjelasan Aplikasi & Latar Belakang Masalah

### Masalah Nyata di Lapangan
Koperasi tani Indonesia kerap mengalami masalah struktural klasik: **panen raya serentak yang memicu jatuhnya harga komoditas**. Puluhan hingga ratusan petani anggota menanam komoditas yang sama (seperti padi atau jagung) pada minggu kalender yang hampir bersamaan. Ketika waktu panen tiba secara serempak:
1. Volume panen melonjak drastis melampaui kapasitas serap gudang atau kapasitas logistik koperasi.
2. Posisi tawar koperasi melemah terhadap pedagang besar dan tengkulak, sehingga harga komoditas jatuh bebas di saat volume produksi petani sedang di puncak tertinggi.
3. Kebutuhan pupuk dan sarana produksi melonjak pada waktu bersamaan, memicu kelangkaan lokal dan ketidaksesuaian alokasi kuota subsidi pemerintah.

Keputusan kritis yang memicu masalah ini sebenarnya terjadi **jauh sebelum benih ditanam**. Namun, selama ini pengurus koperasi dan petugas lapangan (kader) tidak memiliki sistem analitik yang mampu memproyeksikan dinamika kalender tanam, akumulasi cuaca mikro, dan kapasitas serap pasar.

### Solusi Terrion
Terrion hadir sebagai sistem analitik operasional tingkat koperasi:
* **Pencatatan Presisi:** Mencatat kepemilikan lahan anggota, membagi petak menjadi blok-blok tanam multi-musim, serta mendokumentasikan varietas dan tanggal tanam aktual.
* **Proyeksi Berbasis Dinamika Iklim Mikro:** Menghitung perkiraan jendela panen menggunakan akumulasi unit termal biologis (*Growing Degree Days* / GDD) berdasarkan data cuaca historis, prakiraan harian riil (Open-Meteo), dan iklim rata-rata sepuluh tahun.
* **Deteksi Tabrakan Panen (*Collision Detection*):** Menandai minggu-minggu di mana estimasi akumulasi panen melebihi ambang batas kapasitas gudang koperasi, dan memberikan saran penggeseran (*staggering*) tanggal tanam.
* **Rencana Tanam Musim Depan (*Seasonal Planning*):** Memberikan tiga alternatif strategi tanam komprehensif bagi seluruh lahan anggota koperasi sebelum musim tanam (MT I / MT II) dimulai.
* **Transparansi Berbasis Probabilitas:** Angka estimasi selalu disajikan sebagai **rentang probabilitas (P10–P90)** dengan indikasi dasar data (*observed*, *forecast*, atau *climatology*). **Ketiadaan data ditampilkan sebagai `null` (kosong), bukan angka `0` fiktif**, guna mencegah kesimpulan yang menyesatkan para pengambil keputusan.

---

## 2. Fitur-Fitur Utama

| Fitur | Deskripsi Fungsional |
|---|---|
| **Rencana Tanam Musim Depan** | Menghitung dan mengajukan **tiga skenario rencana tanam** (*Aman/Pencegahan Tabrakan*, *Pendapatan Maksimal*, *Terikat Permintaan Pasar*) untuk seluruh lahan koperasi pada musim tanam tertentu. Rencana yang dipilih dapat langsung diterapkan menjadi blok-blok tanam baru secara otomatis, dan dapat dibatalkan secara massal tanpa mengorbankan catatan historis kader. |
| **Pembagian Rencana via Token (Plan Share Token)** | Setiap anggota (petani) otomatis menerima tautan unik per musim (*read-only* tanpa perlu akun login). Tautan ini dapat dikirimkan langsung melalui WhatsApp untuk melihat detail rencana tanam masing-masing, lengkap dengan pelacak status pembacaan (*tracker* sudah dibuka atau belum). |
| **Proyeksi Panen & Fenologi GDD** | Menghitung akumulasi panas spesifik tanaman (*base temperature*) dan memetakan pertumbuhan ke dalam 5 stadium visual (Gundul, Berdiri, Vegetatif, Pematangan, Siap Panen). Jendela panen disimulasikan melalui anomali iklim P10–P90 ($Z = \pm 1{,}2816$). |
| **Model Hasil Panen & Kalibrasi Berkelanjutan** | Model regresi ridge matematis yang belajar dari data panen koperasi itu sendiri. Menggunakan penyusutan empiris Bayesian (*Empirical Bayes Shrinkage*, $k=3$) untuk menarik prediksi kembali ke acuan dasar varietas nasional apabila data riwayat lokal masih sedikit. |
| **Deteksi Tabrakan & Penggeseran Tanam (Staggering)** | Memetakan distribusi panen mingguan sepanjang horizon 12 minggu, mengidentifikasi minggu terberat (*lead collision week*), dan menghasilkan rekomendasi pergeseran tanggal tanam mundur/maju yang dapat langsung disetujui pengurus dalam satu transaksi basis data. |
| **RDKK & Siklus Hidup Pesanan Sarana Produksi** | Agregasi kebutuhan pupuk bersubsidi (Urea, NPK, Organik) per anggota berdasarkan formula resmi Permentan. Penegakan batas alokasi subsidi 2 hektare per anggota (ditandai dengan jelas, tidak dipotong diam-diam). Pengelolaan status pesanan kelompok (*draft* $\to$ *submitted* $\to$ *completed* / *cancelled*) lengkap dengan pencatatan penyesuaian kuantitas nyata. |
| **Katalog Terbuka & Permintaan Pasokan B2B** | Listing panen dinamis yang diturunkan langsung dari proyeksi aktif tanpa duplikasi data. Pembeli terverifikasi (*buyer*) dapat melihat jendela ketersediaan panen dan mengajukan permintaan pasokan kuantitas tertentu ke koperasi. |
| **Visualisasi Lahan Publik & Atlas Komoditas** | Halaman publik per petak lahan yang dapat dibagikan kepada mitra/investor, aman dari kebocoran privasi karena membaca SQL View tanpa kolom koordinat GPS maupun identitas sensitif (NIK/telepon). Atlas publik direktori pertanian tingkat kecamatan dan desa. |

---

## 3. Arsitektur Sistem & Rekayasa Perangkat Lunak

Terrion memisahkan tanggung jawab antara API transaksional yang memegang kredensial basis data dengan layanan optimasi komputasi yang beroperasi tanpa data pribadi (*Zero-PII*).

### Diagram Arsitektur Komponen

```mermaid
graph TB
    subgraph Klien["Lapisan Klien"]
        fe["<b>Terrion_Frontend</b><br/>Next.js 16 · Vercel<br/><i>Role-based UI, Garden Canvas</i>"]
        wa["<b>Petani Anggota</b><br/>WhatsApp / Peramban Ponsel<br/><i>Melihat Rencana via Token Share</i>"]
        buyer["<b>Pembeli Komoditas</b><br/>B2B Procurement Portal"]
    end

    subgraph CoreBackend["Zona Tepercaya — Memegang Kredensial & Tenancy"]
        api["<b>Terrion_Backend</b> (Repo Ini)<br/>Go 1.25 · Fiber v2 · GORM<br/><i>Auth, Tenancy, Transaksi, Mesin Agronomi GDD,<br/>Ridge Yield Model, Fallback Solver</i>"]
    end

    subgraph AIService["Zona Nir-Kredensial — Komputasi Lepas (Opsional)"]
        aisvc["<b>Terrion_AI</b><br/>Python 3.12 · FastAPI<br/><i>Solver CP-SAT (OR-Tools),<br/>Monte Carlo Vectorized, Narasi LLM</i>"]
    end

    subgraph DataStorage["Lapisan Penyimpanan & Layanan Eksternal"]
        db[("<b>Supabase Postgres</b><br/>Relasional, RLS, View Lahan Publik")]
        auth["<b>Supabase Auth (GoTrue)</b><br/>Penerbit Identitas Pengguna"]
        redis[("<b>Redis (Upstash)</b><br/>Sesi Cookie, Cache Katalog, Cache Rencana")]
        meteo["<b>Open-Meteo API</b><br/>Data Historis & Prakiraan Cuaca Mikro"]
    end

    fe -->|HTTP Cookie / API Calls| api
    wa -->|GET /api/public/plan-share/:token| api
    buyer -->|Katalog & Supply Requests| api

    api -->|Postgres Wire (Session Pooler)| db
    api -->|GoTrue Admin API| auth
    api -->|TLS TCP (rediss://)| redis
    api -->|HTTP REST Client| meteo
    api -.->|HTTP REST (Kontrak v1.0, Timeout 3.5s)| aisvc
```

### Pola Arsitektur Berlapis (Clean Architecture)
Struktur kode mengadopsi prinsip *Clean Architecture* dengan aliran dependensi satu arah:
```
Delivery/HTTP (Controller & Middleware)
       ↓
    UseCase (Mengendalikan Transaksi & Validasi)
       ↓
   Repository (Akses Data Bersih, Menampung *gorm.DB)
       ↓
     Entity (Pemetaan Skema GORM & Penamaan Tabel)
```
Tanggapan dari lapisan UseCase dikonversikan kembali melalui `internal/model/converter` menjadi DTO standar sebelum dikembalikan ke klien.

### Paket Domain Murni Bebas Basis Data
Untuk menjamin pengujian unit yang cepat dan deterministik tanpa ketergantungan pada koneksi jaringan atau database, logika agronomi dan bisnis diisolasi ke dalam paket murni independen:
* **`internal/agronomy`**: Kalender UTC, normalisasi minggu ISO, perhitungan akumulasi GDD, interpolasi anomali P10–P90, dan regresi ridge.
* **`internal/planning`**: Representasi kandidat tanam, penghitungan tabrakan tonase, dan solver heuristik lokal (*fallback optimizer*).
* **`internal/weather`**: Pemrosesan serial waktu suhu harian, integrasi observasi dan prakiraan cuaca, serta deteksi kebutuhan *backfill*.
* **`internal/plots`**: Geometri sel grid koordinat, algoritma pembagian blok (*split*), validasi ambang minimum lahan, dan pembuatan benih visualisasi lanskap (*terrain seed*).
* **`internal/rdkk`**: Agregasi kebutuhan kuota pupuk musiman per komoditas menurut Permentan dan pemisahan batas subsidi 2 Ha.
* **`internal/dashboard`**: Agregasi 12 minggu proyeksi, penentuan minggu utama (*lead week*), dan kalkulasi 4 metrik dampak.
* **`internal/catalog`**: Pengelompokan panen terbuka, pembuatan listing ID deterministik (`<coop_id>--<commodity_id>--<iso_week>`), dan seleksi filter.

### Kepemilikan Transaksi oleh UseCase
Berbeda dengan arsitektur umum di mana transaksi dikelola oleh repositori, di Terrion **lapisan UseCase adalah pemilik tunggal transaksi basis data**:
1. UseCase membuka transaksi: `tx := u.DB.WithContext(ctx).Begin()`.
2. Menjamin keamanan eksekusi dengan `defer tx.Rollback()`.
3. Menjalankan validasi input menggunakan `validator.Validate`.
4. Memanggil method-method repositori dengan mengoper instans `tx` (`*gorm.DB`).
5. Melakukan `tx.Commit()` setelah seluruh mutasi data berhasil.

Struktur generik `repository.Repository[T]` menerima `*gorm.DB` sebagai parameter di setiap pemanggilan metodenya (`Create`, `Update`, `Delete`, `FindById`, `CountById`), sehingga repositori murni berfungsi sebagai penyedia kueri tanpa mengetahui batasan transaksi.

### Strategi Dual-Engine AI & Graceful Degradation
Integrasi dengan layanan AI (`Terrion_AI`) dirancang dengan prinsip **ketersediaan tanpa syarat (zero-downtime resilience)**:
* Layanan Python bersifat **opsional**. Jika `AI_SERVICE_URL` tidak dikonfigurasi, sistem langsung menggunakan solver Go internal (*fallback*).
* Komunikasi dilindungi dengan batas waktu ketat (*timeout* 3.500 ms) dan circuit breaker otomatis (1 kali percobaan ulang).
* Apabila layanan AI gagal, timeout, atau mengembalikan respons tidak valid, sistem secara otomatis beralih ke solver Go internal tanpa melempar galat ke pengguna. Respons JSON secara transparan mengindikasikan status mesin: `"engine": "ai-service"` atau `"engine": "fallback"`.
* **Integritas Angka:** Angka tonase, tanggal jendela panen, dan kebutuhan sarana produksi **selalu dihitung ulang di sisi Go**. Sistem tidak pernah memercayai angka mentah dari layanan luar; hanya keputusan penugasan petak dan narasi teks yang diadopsi ([ADR-0006](docs/adr/0006-angka-selalu-dihitung-ulang.md)).

---

## 4. Teknologi yang Digunakan

Seluruh dependensi eksternal tercantum di [go.mod](go.mod). Tidak ada dependensi tersembunyi.

| Teknologi / Pustaka | Versi | Peran & Alasan Pemilihan |
|---|---|---|
| **Go** | `1.25.6` | Bahasa kompilasi utama. Dipilih karena eksekusi model agronomi dan simulasi cuaca membutuhkan jutaan kalkulasi titik apung per permintaan dengan latensi mikrodetik dalam satu proses. |
| **Fiber v2** (`gofiber/fiber/v2`) | `2.52.15` | Kerangka kerja HTTP berbasis Fasthttp. Ringan, memiliki alokasi memori minimal, tanpa refleksi berat, serta mendukung arsitektur RESTful murni. |
| **GORM** (`gorm.io/gorm`) | `1.31.2` | Object-Relational Mapper untuk Go. Digunakan untuk pemetaan relasi entitas dan pengelolaan transaksi lintas repositori. |
| **GORM Postgres Driver** | `1.6.2` | Driver koneksi PostgreSQL berbasis pgx untuk interaksi langsung dengan basis data Supabase. |
| **golang-migrate** (`golang-migrate/migrate/v4`) | `4.19.1` | Pengelola migrasi skema basis data versi sekuensial (`cmd/migrate`), mencakup definisi tabel, indeks, RLS, dan data acuan. |
| **go-redis** (`github.com/redis/go-redis/v9`) | `9.22.0` | Klien Redis berperforma tinggi untuk penyimpanan token sesi cookie, cache proyeksi katalog publik, dan cache rencana AI. |
| **go-playground/validator** | `10.30.3` | Validasi struct request DTO pada lapisan UseCase berbasis anotasi tag. |
| **golang-jwt** (`golang-jwt/jwt/v5`) | `5.3.1` | Pustaka kriptografi untuk validasi rahasia Supabase JWT (HS256) saat inisialisasi aplikasi. |
| **Logrus** (`github.com/sirupsen/logrus`) | `1.10.2` | Logger terstruktur berformat JSON dengan penanda level dan konteks request ID. |
| **Google UUID** (`github.com/google/uuid`) | `1.6.0` | Pembangkitan identifier unik RFC 4122 untuk entitas basis data dan penanda pelacakan HTTP. |
| **godotenv** (`github.com/joho/godotenv`) | `1.5.1` | Pemuat variabel lingkungan `.env` pada lingkungan lokal (produksi menggunakan *native system environment*). |
| **glebarez/sqlite** | `1.11.0` | **Hanya Pengujian.** Driver SQLite berbasis Go murni (tanpa CGO) untuk pengujian unit database in-memory super cepat. |
| **alicebob/miniredis** | `2.38.0` | **Hanya Pengujian.** Server Redis in-memory untuk pengujian alur sesi dan cache tanpa membutuhkan instance Redis eksternal. |

> [!NOTE]
> **Nir-Pustaka Machine Learning Berat:** Algoritma regresi ridge, penyusutan Bayesian, simulasi GDD, dan pencarian heuristik kombinatorial diimplementasikan sepenuhnya menggunakan pustaka standar Go. Seluruh proses berjalan di CPU server standar tanpa memerlukan akselerator GPU atau biaya lisensi pihak ketiga.

---

## 5. Struktur Direktori Repositori

```
Terrion_Backend/
├── .air.toml                    # Konfigurasi live-reload Air untuk pengembangan lokal
├── .env.example                 # Templat variabel lingkungan lengkap
├── Dockerfile                   # Konfigurasi container multi-stage Alpine Linux
├── railway.json                 # Konfigurasi deployment platform Railway
├── go.mod / go.sum              # Definisi modul dan checksum dependensi Go
├── cmd/                         # Entrypoint CLI dan daemon aplikasi
│   ├── web/                     # Daemon server HTTP Fiber utama (main.go)
│   ├── migrate/                 # CLI pelari migrasi basis data (up, down, force, version)
│   ├── seed/                    # CLI pembuat data percontohan multi-provinsi & cuaca
│   ├── register/                # CLI pendaftaran akun pengurus/kader/buyer & koperasi baru
│   └── plan/                    # CLI komputasi simulasi rencana tanam langsung dari terminal
├── db/
│   └── migrations/              # Berkas migrasi SQL berurutan (up/down)
├── internal/                    # Kode sumber internal (terenkapsulasi)
│   ├── agronomy/                # Logika matematika agronomi, fenologi GDD, & model panen (DB-free)
│   ├── aiclient/                # Klien HTTP terisolasi untuk komunikasi ke Terrion_AI (Zero-PII)
│   ├── catalog/                 # Domain penyusunan katalog panen terbuka & listing ID (DB-free)
│   ├── config/                  # Komposisi root, pemuat konfigurasi .env, & singleton infra
│   ├── constants/               # Konstanta global, enum status, durasi sesi, dan peran pengguna
│   ├── dashboard/               # Agregasi data dasbor 12 minggu & metrik dampak (DB-free)
│   ├── delivery/http/           # Lapisan HTTP Fiber (Controller, Middleware, Routing)
│   │   ├── middleware/          # Autentikasi sesi cookie, penegakan peran, dan token cron
│   │   └── route/               # Pendaftaran seluruh rute endpoint API
│   ├── entity/                  # Struktur entitas GORM pemeta tabel PostgreSQL
│   ├── model/                   # DTO permintaan (Request) dan tanggapan (Response)
│   │   └── converter/           # Pemeta murni antara Entity dan Model DTO
│   ├── planning/                # Struktur domain rencana tanam & solver fallback internal (DB-free)
│   ├── plots/                   # Kalkulasi spasial petak lahan, pembagian blok, & terrain (DB-free)
│   ├── rdkk/                    # Agregasi pupuk bersubsidi menurut Permentan (DB-free)
│   ├── repository/              # Lapisan akses data GORM generic & kueri spesifik domain
│   ├── supabase/                # Klien API GoTrue untuk orkestrasi identitas pengguna
│   ├── usecase/                 # Lapisan logika bisnis & kepemilikan transaksi basis data
│   └── weather/                 # Klien Open-Meteo & agregator cuaca harian (DB-free)
└── docs/                        # Dokumentasi teknis mendalam dan catatan arsitektur (ADR)
```

---

## 6. Variabel Lingkungan (.env)

Konfigurasi aplikasi dikelola sepenuhnya melalui variabel lingkungan yang dimuat oleh `internal/config/env.go`. Salin `.env.example` ke `.env` sebelum menjalankan server:

```bash
cp .env.example .env
```

### Tabel Rincian Konfigurasi

| Variabel | Tipe | Default | Keterangan & Sumber Nilai |
|---|---|---|---|
| `APP_NAME` | String | `terrion-backend` | Nama identitas layanan pada header log dan respons health check. |
| `APP_ENV` | String | `development` | Lingkungan aplikasi (`development` atau `production`). Pada mode produksi, cookie sesi mewajibkan flag `Secure` dan `SameSite=None`. |
| `WEB_PORT` | Integer | `8080` | Port TCP tempat server HTTP Fiber mendengarkan koneksi masuk. |
| `WEB_PREFORK` | Boolean | `false` | Menyalakan fitur master-worker Fiber fork (rekomendasi: `false` untuk kemudahan debugging). |
| `WEB_CORS_ORIGINS` | String | `http://localhost:3000` | Daftar domain asal yang diizinkan untuk akses CORS (pisahkan dengan koma jika jamak). |
| `LOG_LEVEL` | Integer | `4` | Level pencatatan log Logrus (`0=Panic`, `1=Fatal`, `2=Error`, `3=Warn`, `4=Info`, `5=Debug`, `6=Trace`). |
| `DB_HOST` | String | *Wajib* | Hostname PostgreSQL Supabase (*Project Settings* $\to$ *Database* $\to$ *Host*). Gunakan direct port `5432` (Session Pooler). |
| `DB_PORT` | Integer | `5432` | Port PostgreSQL (Gunakan `5432`, jangan gunakan port transaction pooler `6543`). |
| `DB_USER` | String | *Wajib* | Username basis data (biasanya `postgres.<project-ref>`). |
| `DB_PASSWORD` | String | *Wajib* | Password database Supabase yang dikonfigurasi saat pembuatan proyek. |
| `DB_NAME` | String | `postgres` | Nama database relasional. |
| `DB_SSLMODE` | String | `require` | Mode enkripsi SSL PostgreSQL (wajib `require` untuk Supabase). |
| `DB_POOL_IDLE` | Integer | `10` | Jumlah koneksi idle minimum dalam connection pool GORM. |
| `DB_POOL_MAX` | Integer | `100` | Jumlah koneksi maksimum dalam connection pool GORM. |
| `DB_POOL_LIFETIME` | Integer | `300` | Masa hidup koneksi dalam pool sebelum di-refresh (detik). |
| `REDIS_URL` | String | *Wajib* | Alamat URI koneksi Redis Upstash. Wajib menggunakan skema `rediss://` (dengan TLS) agar koneksi terenkripsi. |
| `CRON_SECRET` | String | *Wajib* | Token rahasia bersama untuk memicu worker cron cuaca. Buat dengan: `openssl rand -hex 32`. |
| `SUPABASE_URL` | String | *Wajib* | URL proyek Supabase (*Project Settings* $\to$ *API* $\to$ *Project URL*). |
| `SUPABASE_ANON_KEY` | String | *Wajib* | Kunci anon/public Supabase, digunakan untuk registrasi mandiri akun buyer. |
| `SUPABASE_SERVICE_ROLE_KEY`| String | *Wajib* | Kunci *service_role* Supabase berprivilese tinggi, hanya dipakai untuk rollback akun auth jika profil GORM gagal dibuat. |
| `SUPABASE_JWT_SECRET` | String | *Wajib* | Secret kunci penandatangan JWT HS256 Supabase (dipakai sebagai verifikasi integritas saat boot aplikasi). |
| `AI_SERVICE_URL` | String | *Opsional* | URL endpoint layanan optimasi Python `Terrion_AI` (misal: `http://localhost:8000`). Kosongkan untuk menggunakan solver Go internal (*fallback*). |
| `AI_SERVICE_TOKEN` | String | *Opsional* | Bearer token bersama untuk autentikasi ke layanan Python. Buat dengan: `openssl rand -hex 32`. |
| `AI_SERVICE_TIMEOUT_MS` | Integer | `3500` | Batas waktu maksimum pemanggilan layanan AI dalam milidetik (mencakup 1 kali retry). |
| `AI_WARMUP_INTERVAL` | Integer | `0` | Interval ping berkala ke endpoint `/health` AI service untuk mencegah cold-start hosting gratis (`0` = nonaktif). |

---

## 7. Panduan Instalasi & Menjalankan Aplikasi

### Prasyarat Sistem
* **Go** versi `1.25.6` atau yang lebih baru ([Unduh Go](https://go.dev/dl/)).
* **PostgreSQL** instance aktif (disarankan menggunakan [Supabase](https://supabase.com)).
* **Redis** instance aktif dengan dukungan TLS (disarankan menggunakan [Upstash](https://upstash.com)).
* Akses internet untuk mengunduh modul Go dan data cuaca [Open-Meteo](https://open-meteo.com).

### 1. Kloning & Pemasangan Dependensi
```bash
git clone https://github.com/ITechnoCup2026/Terrion_Backend.git
cd Terrion_Backend

# Unduh seluruh dependensi Go
go mod download
```

### 2. Inisialisasi Konfigurasi
Salin templat konfigurasi dan sesuaikan nilai kredensial Anda:
```bash
cp .env.example .env
# Edit berkas .env menggunakan editor teks favorit Anda
```

### 3. Eksekusi Migrasi Basis Data
Terapkan seluruh skema migrasi ke database PostgreSQL:
```bash
go run cmd/migrate/main.go up
```
Periksa versi migrasi yang telah diterapkan:
```bash
go run cmd/migrate/main.go version
```

### 4. Menjalankan Server API
Jalankan server dalam mode standar:
```bash
go run ./cmd/web
```
Server akan aktif dan mendengarkan permintaan di `http://localhost:8080` (sesuai `WEB_PORT`).

Untuk pengembangan dengan fitur *live reload* otomatis setiap kali ada berkas kode yang diubah, gunakan [Air](https://github.com/air-verse/air):
```bash
air
```

---

## 8. CLI Tools & Eksekusi Perintah Terminal

Terrion Backend dilengkapi perkakas baris perintah mandiri di folder `cmd/` untuk memudahkan operasi dan pengujian sistem.

### 1. Migrator Database (`cmd/migrate`)
Mengelola siklus hidup skema database PostgreSQL menggunakan migrasi berurutan:
```bash
# Menjalankan seluruh migrasi yang belum terpasang
go run cmd/migrate/main.go up

# Membatalkan satu langkah migrasi terakhir
go run cmd/migrate/main.go down

# Menetapkan versi skema secara paksa (misal jika skema sudah ada sebelumnya)
go run cmd/migrate/main.go force 20260908000015

# Melihat versi skema aktif saat ini
go run cmd/migrate/main.go version
```

### 2. Penebar Data Percontohan Multi-Provinsi (`cmd/seed`)
Mengisi basis data dengan profil koperasi contoh yang realistis di berbagai provinsi (Subang - Jawa Barat, Brebes - Jawa Tengah, dll.), riwayat panen masa lampau untuk melatih model kalibrasi, petak lahan aktif, akun uji coba, dan data cuaca historis Open-Meteo:
```bash
# Penebaran data standar (membuat akun demo & mengambil data cuaca riil)
go run cmd/seed/main.go

# Reset: Hapus seluruh data percontohan lama, lalu tebarkan ulang data baru
go run cmd/seed/main.go -reset

# Hanya hapus seluruh data percontohan tanpa mengisi ulang
go run cmd/seed/main.go -reset-only

# Penebaran tanpa membuat akun auth di Supabase (hanya data lahan & acuan)
go run cmd/seed/main.go -accounts=false

# Menentukan password seragam untuk semua akun demo (default: terrion-demo-2026)
go run cmd/seed/main.go -password "KataSandiDemo123!"
```

Akun percontohan standar yang dihasilkan:
* **Pengurus Subang:** `pengurus.subang@terrion.test` (Kata sandi: `terrion-demo-2026`)
* **Kader Subang:** `kader.subang@terrion.test` (Kata sandi: `terrion-demo-2026`)
* **Pengurus Brebes:** `pengurus.brebes@terrion.test` (Kata sandi: `terrion-demo-2026`)
* **Kader Brebes:** `kader.brebes@terrion.test` (Kata sandi: `terrion-demo-2026`)
* **Pembeli B2B:** `buyer.budi@terrion.test` (Kata sandi: `terrion-demo-2026`)

### 3. Pendaftaran Akun & Koperasi Baru (`cmd/register`)
Membuat akun petugas atau pembeli secara administratif melalui CLI tanpa melalui web:
```bash
# Mendaftarkan pengurus sekaligus mendirikan koperasi baru:
go run cmd/register/main.go -role pengurus \
  -email "ketua@kud-makmur.id" \
  -name "Haji Slamet" \
  -create-cooperative \
  -coop-name "KUD Makmur Bersama" \
  -village "Jalancagak" \
  -district "Subang" \
  -province "Jawa Barat" \
  -lat -6.68 \
  -lng 107.68

# Mendaftarkan kader lapangan ke koperasi yang sudah ada:
go run cmd/register/main.go -role kader \
  -email "kader.agus@kud-makmur.id" \
  -name "Agus Suryanto" \
  -cooperative "<UUID_KOPERASI>"

# Mendaftarkan entitas pembeli komoditas (Buyer):
go run cmd/register/main.go -role buyer \
  -email "procurement@indofood.test" \
  -name "Bambang Wijaya" \
  -organisation "PT Pangan Nusantara"
```

### 4. Eksekusi Perencanaan Tanam Terminal (`cmd/plan`)
Menguji kalkulasi optimasi rencana tanam musim depan langsung dari terminal tanpa perlu menyalakan peramban atau frontend:
```bash
go run cmd/plan/main.go \
  -cooperative "<UUID_KOPERASI>" \
  -season "MT I 2026/2027" \
  -goal "Ratakan beban panen agar tidak menumpuk di minggu 48"
```

---

## 9. Dokumentasi & Kontrak API HTTP

Seluruh rute terdaftar di [internal/delivery/http/route/route.go](file:///C:/Code/Terrion/Terrion_Backend/internal/delivery/http/route/route.go). Terdapat **38 endpoint** yang mencakup seluruh operasi bisnis sistem.

### Format Amplop Respons Standar
Semua respons API dibungkus dalam format seragam `model.WebResponse[T]`:

**Respons Berhasil (HTTP 200 / 201):**
```json
{
  "data": { ... }
}
```

**Respons Galat (HTTP 400, 401, 403, 404, 422, 500):**
```json
{
  "errors": "error_code_identifier"
}
```
> [!IMPORTANT]
> Properti `errors` berisi **kode mesin standar**, bukan kalimat teks bebas. Penerjemahan ke dalam antarmuka bahasa manusia dilakukan di lapisan antarmuka pengguna (*Frontend*).

Pada penolakan validasi domain tertentu (seperti pembagian blok atau penggeseran tanam), backend mengembalikan kode error sekaligus objek data pendukung agar UI dapat menyusun pesan yang kontekstual:
```json
{
  "errors": "split_leaves_too_little",
  "data": {
    "min_ha": 0.01,
    "block_area_ha": 0.5,
    "max_takeable_ha": 0.49
  }
}
```

### Mekanisme Autentikasi Sesi
Sistem menggunakan autentikasi **Cookie HttpOnly Server-Side** yang tersimpan di Redis:
1. Pengguna memanggil `POST /api/auth/login` dengan `email` dan `password`.
2. Backend menukar kredensial tersebut dengan pasangan access/refresh token GoTrue (Supabase Auth).
3. Backend membangkitkan string ID sesi kriptografis acak 32-byte, menyimpan token GoTrue di Redis (`terrion:session:<id>`, TTL 30 hari), dan menyetel cookie:
   ```http
   Set-Cookie: terrion_session=<session_id>; Path=/; HttpOnly; SameSite=Lax; Max-Age=2592000
   ```
4. Setiap permintaan berikutnya secara otomatis membawa cookie ini. Middleware `middleware.Auth` membaca ID sesi dari cookie, mengambil data akun dari basis data, dan menempelkannya ke konteks Fiber (`ctx.Locals("auth_user")`).
5. **Rotasi Token:** Frontend memanggil `POST /api/auth/refresh` secara berkala untuk memperbarui refresh token GoTrue di Redis tanpa mengubah ID sesi atau nilai cookie pengguna.

### Matriks Peran Pengguna & Hak Akses

| Peran (`UserRole`) | Deskripsi & Hak Akses |
|---|---|
| `public` | Akses tanpa login. Menjelajahi katalog panen, melihat visualisasi lahan publik, profil atlas daerah, dan membuka tautan rencana anggota (`plan-share`). |
| `buyer` | Pembeli komoditas terverifikasi. Dapat menelusuri katalog panen teragregasi dan mengirimkan permintaan pasokan (*supply requests*). |
| `kader` | Petugas lapangan koperasi. Dapat mencatat pendaftaran lahan, membagi petak lahan menjadi blok tanaman, menyunting blok, dan mencatat realisasi panen anggota. |
| `pengurus` | Pengelola utama koperasi. Memiliki semua hak akses kader, ditambah wewenang: mengatur kapasitas gudang mingguan, memicu penggeseran tanam (*stagger*), menyusun & menerapkan rencana tanam musiman, menerbitkan pesanan pupuk RDKK, dan menyetujui permintaan pasokan pembeli. |
| `cron` | Pekerja sistem latar belakang. Dilindungi oleh otentikasi header `Authorization: Bearer $CRON_SECRET`. |

---

### Katalog Lengkap 38 Endpoint API

#### 1. Autentikasi & Akun Pengguna
| Metode | Endpoint | Akses | Deskripsi |
|---|---|---|---|
| `GET` | `/api/health` | Publik | Status kesehatan sistem (`{"data":{"status":"ok","service":"terrion-backend"}}`). |
| `POST` | `/api/auth/signup` | Publik | Registrasi mandiri akun baru khusus peran `buyer`. |
| `POST` | `/api/auth/login` | Publik | Otentikasi email/password; menerbitkan cookie sesi `terrion_session`. |
| `POST` | `/api/auth/refresh` | Publik | Rotasi refresh token GoTrue pada sesi aktif di Redis. |
| `POST` | `/api/auth/logout` | Publik | Mencabut sesi di Supabase dan menghapus data sesi dari Redis. |
| `GET` | `/api/me` | Terautentikasi | Mengambil profil pengguna aktif, peran, dan detail koperasi yang dinaungi. |

#### 2. Lahan & Blok Tanaman (Plots & Blocks)
| Metode | Endpoint | Akses | Deskripsi |
|---|---|---|---|
| `GET` | `/api/commodities` | Publik | Referensi komoditas (padi, jagung, dll.) beserta varietas unggul dan konstanta GDD-nya. |
| `GET` | `/api/plots` | Terautentikasi (Koperasi) | Daftar seluruh lahan anggota, terurut berdasarkan tanggal panen terdekat. |
| `GET` | `/api/plots/:id` | Terautentikasi (Koperasi) | Detail spesifik suatu lahan, riwayat blok, kurva GDD kumulatif, dan status panen. |
| `POST` | `/api/plots` | `kader`, `pengurus` | Mendaftarkan lahan baru beserta blok tanaman perdana dan kontak telepon anggota (`member_phone`). Luas total lahan diturunkan secara otomatis dari jumlah petak. |
| `POST` | `/api/blocks/:id/split` | `kader`, `pengurus` | Memecah satu blok tanaman yang berdiri menjadi dua sub-blok (misal: diversifikasi tanaman). |
| `PATCH` | `/api/blocks/:id` | `kader`, `pengurus` | Memperbarui komoditas, varietas, atau tanggal tanam pada blok yang belum dipanen. |
| `DELETE`| `/api/plots/:id` | `pengurus` | Menghapus pendaftaran lahan dari inventaris koperasi. |

#### 3. Realisasi Panen & Model Pembelajaran (Harvests)
| Metode | Endpoint | Akses | Deskripsi |
|---|---|---|---|
| `PATCH` | `/api/blocks/:id/harvest` | `kader`, `pengurus` | Mencatat realisasi hasil panen aktual (tanggal panen riil, total tonase, harga jual per kg, dan waktu pembayaran). Data ini langsung mengalibrasi model prediksi masa depan koperasi bersangkutan. |
| `GET` | `/api/harvests` | Terautentikasi (Koperasi) | Riwayat pencatatan panen masa lampau yang telah diselesaikan. |

#### 4. Kapasitas Gudang & Operasional (Capacity)
| Metode | Endpoint | Akses | Deskripsi |
|---|---|---|---|
| `GET` | `/api/capacity` | Terautentikasi (Koperasi) | Mengambil batas kapasitas serap gudang/logistik mingguan koperasi (ton/minggu). |
| `PUT` | `/api/capacity` | `pengurus` | Menyetel atau memperbarui ambang batas kapasitas mingguan koperasi. |

#### 5. Dasbor Analitik & Deteksi Tabrakan (Dashboard & Staggering)
| Metode | Endpoint | Akses | Deskripsi |
|---|---|---|---|
| `GET` | `/api/dashboard` | Terautentikasi (Koperasi) | Mengembalikan proyeksi panen 12 minggu, deteksi minggu tabrakan (*flagged*), penentuan minggu terberat (*lead week*), saran pergeseran tanam, jadwal panen 7 hari ke depan, dan 4 metrik dampak kumulatif. |
| `POST` | `/api/stagger` | `pengurus` | Menerapkan rekomendasi pergeseran tanggal tanam untuk meratakan akumulasi panen mingguan. Parameter cukup berupa `iso_week` dan `commodity_id`. |

#### 6. Rencana Tanam Musim Depan (Planning & Member Share)
| Metode | Endpoint | Akses | Deskripsi |
|---|---|---|---|
| `GET` | `/api/plans/propose` | `pengurus` | Menghitung dan menyajikan 3 skenario rencana tanam musim depan (*Aman*, *Pendapatan*, *Pasar*) berbasis optimasi AI atau fallback Go. |
| `GET` | `/api/plans` | Terautentikasi (Koperasi) | Daftar seluruh rencana musim yang pernah diajukan atau diterapkan di koperasi. |
| `GET` | `/api/plans/:id` | Terautentikasi (Koperasi) | Detail rencana musim: daftar penugasan lahan, varietas, jadwal tanam, ringkasan pupuk RDKK, dan token tautan per anggota. |
| `POST` | `/api/plans` | `pengurus` | Menerapkan (*apply*) salah satu skenario rencana menjadi blok-blok tanam aktif di lahan anggota. |
| `POST` | `/api/plans/:id/cancel`| `pengurus` | Membatalkan rencana yang sudah diterapkan secara massal dan aman tanpa menyentuh blok historis lainnya. |
| `GET` | `/api/public/plan-share/:token` | Publik | Halaman periksa rencana tanam khusus anggota (tanpa login). Mengembalikan detail rencana spesifik anggota dan menandai status bahwa tautan telah dibuka (*tracker*). |

#### 7. RDKK & Siklus Hidup Pesanan Pupuk (Input Orders)
| Metode | Endpoint | Akses | Deskripsi |
|---|---|---|---|
| `GET` | `/api/rdkk` | Terautentikasi (Koperasi) | Formulir Rencana Definitif Kebutuhan Kelompok (RDKK) pupuk bersubsidi sesuai rumus Permentan; batas subsidi 2 Ha per anggota ditandai secara transparan. |
| `POST` | `/api/input-orders` | `pengurus` | Menerbitkan draf pesanan kelompok pengadaan pupuk musiman bersubsidi berdasarkan hasil rekapitulasi RDKK. |
| `GET` | `/api/input-orders` | Terautentikasi (Koperasi) | Daftar seluruh pesanan sarana produksi kelompok yang tercatat di koperasi. |
| `PATCH` | `/api/input-orders/:id` | `pengurus` | Mengelola status pesanan pupuk (*draft* $\to$ *submitted* $\to$ *completed* / *cancelled*), mencatat identitas penanggung jawab, serta mendokumentasikan penyesuaian kuantitas karung pupuk aktual. |

#### 8. Katalog Terbuka & Permintaan Pasokan B2B (Catalog & Procurement)
| Metode | Endpoint | Akses | Deskripsi |
|---|---|---|---|
| `GET` | `/api/catalog` | Publik | Menampilkan katalog estimasi panen terbuka lintas koperasi (dilengkapi cache Redis 1 jam). |
| `GET` | `/api/catalog/cooperatives/:id`| Publik | Menampilkan katalog estimasi panen yang tersedia khusus untuk satu koperasi tertentu. |
| `GET` | `/api/supply-requests` | Terautentikasi | Daftar permintaan pasokan komoditas. Jika dipanggil oleh `buyer`, menampilkan permintaan milik dirinya; jika dipanggil oleh `pengurus`, menampilkan permintaan yang masuk ke koperasinya. |
| `POST` | `/api/supply-requests` | `buyer` | Mengajukan permintaan pembelian pasokan komoditas untuk minggu panen tertentu. |
| `PATCH` | `/api/supply-requests/:id`| `pengurus` | Menanggapi permintaan pasokan dari pembeli (status: `accepted` atau `declined`). |

#### 9. Halaman Lahan Publik & Atlas Pertanian
| Metode | Endpoint | Akses | Deskripsi |
|---|---|---|---|
| `GET` | `/api/public/plots/:publicId` | Publik | Visualisasi profil lahan dan tahapan tanam untuk publik tanpa mengekspos koordinat GPS maupun NIK pemilik. |
| `GET` | `/api/atlas/cooperatives` | Publik | Peta direktori organisasi koperasi tani yang terdaftar di platform Terrion. |
| `GET` | `/api/atlas/farms/:id` | Publik | Profil pertanian publik tingkat desa/kecamatan beserta komoditas yang sedang aktif ditanam. |

#### 10. Worker Latar Belakang (Cron)
| Metode | Endpoint | Akses | Deskripsi |
|---|---|---|---|
| `POST` | `/api/cron/weather` | Token `CRON_SECRET` | Mengambil pembaruan cuaca harian dan prakiraan iklim dari Open-Meteo untuk seluruh koordinat grid sel lahan koperasi. |

---

### Daftar Kode Galat Domain (HTTP 422 Unprocessable Entity)

Saat permintaan secara sintaksis valid (HTTP 200/400) namun melanggar batasan bisnis domain, API mengembalikan kode status `422` dengan kode spesifik berikut:

| Kode Galat | Area Domain | Penjelasan Masalah |
|---|---|---|
| `split_block_already_gone` | Lahan | Blok yang hendak dipecah sudah tidak ditemukan atau bukan milik koperasi pengguna. |
| `split_block_harvested` | Lahan | Blok sudah berstatus dipanen; tidak dapat dipecah lagi (daftarkan sebagai tanam baru). |
| `split_below_minimum` | Lahan | Luas petak hasil pemecahan kurang dari ambang minimum yang diizinkan (0,01 Ha). |
| `split_leaves_too_little` | Lahan | Sisa luas pada blok tanaman lama menjadi kurang dari batas minimum (0,01 Ha). |
| `stagger_suggestion_stale` | Dasbor | Rekomendasi penggeseran tanam sudah basi karena proyeksi cuaca telah bergeser; pengguna perlu memuat ulang dasbor. |
| `stagger_nothing_to_shift` | Dasbor | Rekomendasi penggeseran tanam ada, namun seluruh tanaman penyumbang sudah ditanam di masa lampau sehingga tanggal tanam tidak dapat digeser. |
| `rdkk_nothing_to_order` | RDKK | Tidak ada kebutuhan pupuk yang dapat dipesan untuk rentang musim yang dipilih. |
| `order_not_found` | Pesanan Pupuk | Berkas pesanan kelompok pupuk tidak ditemukan. |
| `order_transition_invalid` | Pesanan Pupuk | Transisi status tidak diizinkan oleh state machine (misal: melompat dari `draft` langsung ke `completed`, atau berupaya mundur). |
| `order_already_final` | Pesanan Pupuk | Berkas pesanan sudah berstatus final (`completed` atau `cancelled`) sehingga tidak dapat diubah lagi. |
| `order_season_already_open` | Pesanan Pupuk | Sudah terdapat pesanan kelompok lain yang sedang aktif untuk musim tanam yang sama. |
| `plan_cooperative_empty` | Perencanaan | Koperasi belum memiliki petak lahan terdaftar untuk disusun rencana tanamnya. |

---

### Contoh Alur Penggunaan cURL

#### 1. Melakukan Otentikasi dan Mendapatkan Cookie Sesi
```bash
curl -i -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"pengurus.subang@terrion.test","password":"terrion-demo-2026"}'
```
*Simpan cookie `terrion_session` yang dikembalikan pada header `Set-Cookie`.*

#### 2. Mendaftarkan Lahan Baru dengan Kontak Telepon Petani
```bash
curl -X POST http://localhost:8080/api/plots \
  -b "terrion_session=$SESSION_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "member_name": "Pak Dadang",
    "member_phone": "081234567890",
    "plot_name": "Petak Blok Legok C1",
    "lat": -6.6823,
    "lng": 107.6812,
    "plantings": [
      {
        "commodity_id": "3f4b1a2c-1111-4111-8111-111111111111",
        "variety_id": "3f4b1a2c-2222-4222-8222-222222222222",
        "planting_date": "2026-09-10",
        "area_ha": 0.75
      }
    ]
  }'
```

#### 3. Mengajukan Proposal Rencana Tanam Musim Depan
```bash
curl -s -X GET "http://localhost:8080/api/plans/propose?season=MT%20I%202026/2027" \
  -b "terrion_session=$SESSION_ID" | jq '.data.engine, .data.plans[].objective'
```

#### 4. Menerapkan Skenario Rencana Tanam
```bash
curl -X POST http://localhost:8080/api/plans \
  -b "terrion_session=$SESSION_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "season_label": "MT I 2026/2027",
    "objective": "aman",
    "assignments": [
      {
        "plot_id": "<UUID_PLOT>",
        "variety_id": "<UUID_VARIETAS>",
        "planting_date": "2026-10-15"
      }
    ]
  }'
```

#### 5. Memperbarui Status Siklus Hidup Pesanan Pupuk Kelompok
```bash
# Mengajukan draf pesanan ke distributor/kios resmi:
curl -X PATCH http://localhost:8080/api/input-orders/<ORDER_ID> \
  -b "terrion_session=$SESSION_ID" \
  -H "Content-Type: application/json" \
  -d '{"status": "submitted"}'

# Menyelesaikan pesanan saat pupuk fisik diterima di gudang koperasi:
curl -X PATCH http://localhost:8080/api/input-orders/<ORDER_ID> \
  -b "terrion_session=$SESSION_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "status": "completed",
    "notes": "Pupuk diterima lengkap di gudang KUD Subang",
    "lines": [
      {
        "fertiliser_type": "urea",
        "quantity_actual_bags": 120
      }
    ]
  }'
```

---

## 10. Logika Domain & Mesin Agronomi Inti

Seluruh kalkulasi agronomi diimplementasikan secara mandiri di dalam [internal/agronomy](file:///C:/Code/Terrion/Terrion_Backend/internal/agronomy):

### 1. Normalisasi Kalender & Zona Waktu UTC
Untuk mencegah pergeseran penanggalan akibat perbedaan zona waktu (misalnya eksekusi cron pada pukul 03.00 WIB yang dapat mendarat di tanggal kemarin dalam UTC), **seluruh sistem beroperasi dalam basis UTC tengah malam**.
* Perhitungan minggu kalender menggunakan standar **ISO-8601** (`time.ISOWeek()`). Aturan ISO menetapkan tahun dari suatu minggu berdasarkan hari Kamisnya. Contoh: 1 Januari 2027 jatuh pada hari Jumat, sehingga minggu tersebut dicatat sebagai `2026-W53`.

### 2. Akumulasi Suhu (*Growing Degree Days* / GDD)
GDD mengukur akumulasi energi panas efektif harian yang mendorong pertumbuhan biologis tanaman di atas suhu ambang dasar (*base temperature*, $T_{base}$):
$$\text{GDD} = \max\left(0, \frac{T_{max} + T_{min}}{2} - T_{base}\right)$$
Hari dengan suhu rata-rata di bawah $T_{base}$ menyumbang nilai 0 (tanaman tidak mengalami kemunduran pertumbuhan).

Tingkat pertumbuhan diklasifikasikan ke dalam **5 Stadium Fenologi**:
* `StageBare` (0): Fraksi akumulasi GDD $< 15\%$ (lahan baru diolah/ditanami).
* `StageEstablished` (1): Fraksi GDD $\ge 15\%$ (tanaman mulai bertunas kokoh).
* `StageVegetative` (2): Fraksi GDD $\ge 50\%$ (perkembangan daun dan anakan aktif).
* `StageRipening` (3): Fraksi GDD $\ge 85\%$ (fase pengisian bulir dan pematangan).
* `StageReady` (4): Fraksi GDD $\ge 100\%$ (tanaman mencapai kematangan fisiologis penuh & siap dipanen).

### 3. Simulasi Probabilistik Jendela Panen (P10–P90)
Alih-alih memberikan satu tanggal panen deterministik yang semu, sistem mensimulasikan dua lintasan iklim ekstrem menggunakan nilai z-score standar normal:
$$Z = \pm 1{,}2816 \quad (\text{Mencakup interval kepercayaan 80\% / persentil 10 hingga 90})$$
* Lintasan anomali hangat ($Z = +1{,}2816$) mematangkan tanaman lebih cepat dan menentukan tanggal awal jendela panen.
* Lintasan anomali dingin ($Z = -1{,}2816$) mematangkan tanaman lebih lambat dan menentukan tanggal akhir jendela panen.
* **Hierarki Data Cuaca:** Data historis teramati (*observed*) selalu menimpa data ramalan (*forecast*) untuk hari yang sama. Jika simulasi melampaui horizon prakiraan, sistem menggunakan iklim rata-rata sepuluh tahun (*climatology*).

### 4. Estimasi Hasil Panen & Penyusutan Empiris Bayesian
Prediksi produktivitas (ton/hektare) dimodelkan menggunakan regresi ridge matematis ($\lambda = 1$) terhadap indeks hasil panen (rasio hasil panen aktual terhadap potensi varietas nasional).
Untuk mengatasi bias pada koperasi baru yang baru mencatatkan segelintir panen, diterapkan teknik **Empirical Bayes Shrinkage** ($k=3$):
$$\text{Offset Terkoreksi} = \text{Offset Teramati} \times \frac{n}{n + 3}$$
* Jika sebuah koperasi baru memiliki $n=1$ data panen dengan selisih 8 hari dari teori, offset yang diterapkan hanya sebesar $8 \times \frac{1}{4} = 2$ hari.
* Seiring bertambahnya data historis (misal $n=97$), model akan mempercayai data lokal koperasi sepenuhnya ($8 \times \frac{97}{100} = 7{,}76$ hari).

### 5. Siklus Hidup Pesanan Sarana Produksi (Input Order Lifecycle)
Pengadaan pupuk bersubsidi merupakan alur hukum yang ketat. Transisi status dikendalikan oleh mesin status terbatas (*finite state machine*):

```
     ┌───────────▶ SUDAH DIAJUKAN (submitted) ───▶ SELESAI (completed)
     │                     │
  DRAF (draft)             │
     │                     ▼
     └───────────▶ DIBATALKAN (cancelled)
```
* **Ketat:** Tidak ada lompatan status (misal: draf langsung menjadi selesai), tidak dapat bergerak mundur, dan tidak dapat memproses pesanan yang sudah berstatus final (`completed` atau `cancelled`).
* **Satu Pesanan Aktif:** Mencegah duplikasi pengajuan pupuk ke distributor untuk musim tanam yang sama dalam satu koperasi.

---

## 11. Keamanan, Privasi, & AI Responsif

### 1. Jaminan Keamanan Data Pribadi (Zero-PII Export)
Terrion mematuhi prinsip perlindungan privasi data petani secara struktural:
* **Tidak Ada Data Pribadi yang Keluar:** Nama petani, Nomor Induk Kependudukan (NIK), nomor telepon, dan koordinat lintang/bujur lahan **tidak pernah dikirimkan ke layanan luar atau AI**.
* **Keamanan Ditegakkan Melalui Tipe Kompilasi:** Struct payload permintaan `aiclient.Candidate` tidak memiliki field untuk menampung nama, NIK, koordinat, maupun nama koperasi. Kebocoran data pribadi akan langsung memicu kegagalan kompilasi kode (*compile-time error*).
* **Pengujian Kebocoran Otomatis:** Terdapat tes unit otomatis (`TestRequestCarriesNoPersonalData` dan `TestProposeSendsNoPersonalDataToTheAIService`) yang sengaja menginjeksi nama dan koordinat mencolok ke dalam usecase, dan akan langsung gagal (*FAIL*) jika teks tersebut ditemukan dalam payload JSON keluar.

### 2. View Publik Bebas Data Sensitif
Endpoint publik (`/api/public/plots/:publicId`) dan atlas direktori membaca view SQL khusus `public_plot`, bukan tabel dasar `plot`:
* View `public_plot` secara arsitektural tidak memiliki kolom `lat`, `lng`, `grid_lat`, `grid_lng`, maupun `nik_hash`.
* Unit test menginspeksi metadata skema `pragma_table_info('public_plot')` untuk memastikan kolom-kolom sensitif tersebut tidak pernah hadir di view publik.

### 3. Isolasi Multi-Tenant pada Lapisan Bisnis
Seluruh kueri basis data pada lapisan UseCase selalu menyertakan filter identitas penyewa (`cooperative_id = ?`). Upaya mengakses atau memanipulasi sumber daya milik koperasi lain akan mengembalikan respons **HTTP 404 (Not Found)**, bukan HTTP 403. Penyatuan respons "tidak ada" dan "bukan milikmu" bertujuan untuk mencegah penyerang melakukan pemindaian (*enumeration attack*) terhadap keberadaan ID koperasi lain.

---

## 12. Pengujian & Jaminan Mutu (Testing)

Terrion mengedepankan pengujian yang dapat dijalankan secara instan dan mandiri tanpa konfigurasi eksternal yang rumit:
* **Tanpa Mocking Rumit:** Pengujian tidak menggunakan mocking tiruan `*gorm.DB` yang rapuh. Sebagai gantinya, pengujian repositori dan usecase memanfaatkan basis data **in-memory SQLite murni** (`glebarez/sqlite`) yang dikonfigurasi dengan batas koneksi tunggal (`SetMaxOpenConns(1)`).
* **In-Memory Redis:** Pengujian alur sesi dan cache menggunakan server **in-memory Miniredis** (`alicebob/miniredis/v2`).
* **Pustaka Standar:** Seluruh pengujian menggunakan pustaka bawaan Go `testing` dengan pemeriksaan eksplisit (`t.Errorf` / `t.Fatalf`).

### Menjalankan Pengujian

```bash
# Menjalankan seluruh rangkaian tes di seluruh paket:
go test ./...

# Menjalankan tes dengan log terperinci (verbose):
go test -v ./...

# Menjalankan tes khusus pada paket agronomi:
go test -v ./internal/agronomy/...

# Menjalankan satu fungsi tes tertentu berdasarkan nama:
go test -run TestPredictHarvest ./...

# Memeriksa format dan konvensi kode:
go vet ./...
```

---

## 13. Panduan Deployment Produksi

### 1. Build Kontainer Docker
Aplikasi menyertakan [Dockerfile](file:///C:/Code/Terrion/Terrion_Backend/Dockerfile) multi-stage berbasis Alpine Linux yang menghasilkan binary statis berukuran sangat ramping (< 30 MB):

```bash
# Membangun image Docker
docker build -t terrion-backend:latest .

# Menjalankan container dengan membaca variabel lingkungan dari berkas .env
docker run -d --name terrion-api -p 8080:8080 --env-file .env terrion-backend:latest
```

### 2. Deployment ke Platform Railway
Proyek ini sudah dilengkapi konfigurasi [railway.json](file:///C:/Code/Terrion/Terrion_Backend/railway.json):
* **Start Command:** `/app/migrate up && /app/terrion` — otomatis menjalankan seluruh migrasi skema database yang tertunda sebelum menyalakan daemon server Fiber.
* **Healthcheck Path:** `/api/health` dengan toleransi timeout 30 detik.
* **Restart Policy:** Otomatis melakukan restart hingga 3 kali jika terjadi kegagalan tak terduga.

### 3. Pengaturan Cron Cuaca Harian
Untuk menjaga agar data cuaca Open-Meteo selalu terbarui, atur cron job eksternal (menggunakan *crontab*, *GitHub Actions*, atau *Upstash QStash*) yang memanggil endpoint berikut setiap hari pukul 03.00 WIB (20.00 UTC):

```bash
curl -X POST https://api.terrion.id/api/cron/weather \
  -H "Authorization: Bearer $CRON_SECRET"
```

---

## 14. Pedoman Kontribusi & Konvensi Kode

### 1. Konvensi "No Comments in Code"
Repositori ini menerapkan aturan ketat: **tidak ada komentar naratif di dalam berkas kode Go maupun berkas migrasi SQL**.
* Kode harus ditulis secara deklaratif, dengan nama fungsi dan variabel yang jelas (*self-documenting*).
* Penjelasan konteks, alasan teknis, landasan formula agronomi, dan justifikasi keputusan arsitektur didokumentasikan di berkas markdown pada direktori `docs/` (misal: [AGRONOMY.md](docs/AGRONOMY.md), [ARCHITECTURE.md](docs/ARCHITECTURE.md), dan [docs/adr/](docs/adr/)).

### 2. Alur Penambahan Domain Baru
Jika Anda ingin menambahkan modul domain baru (contoh: `Warehouse`):
1. `internal/entity/warehouse_entity.go` — Definisikan struct GORM beserta method `TableName()`.
2. `internal/model/warehouse_model.go` — Definisikan struct DTO Request/Response dengan tag `validate:"..."`.
3. `internal/model/converter/warehouse_converter.go` — Buat fungsi murni konversi antara Entity dan Model DTO.
4. `internal/repository/warehouse_repository.go` — Embed `repository.Repository[entity.Warehouse]` dan tambahkan metode kueri khusus.
5. `internal/usecase/warehouse_usecase.go` — Tulis logika bisnis; pastikan UseCase mengelola transaksi `u.DB.WithContext(ctx).Begin()`.
6. `internal/delivery/http/warehouse_controller.go` — Buat controller handler untuk parsing request dan pengemasan respon.
7. `internal/delivery/http/route/route.go` — Daftarkan rute endpoint controller pada method `RouteConfig.Setup()`.
8. `internal/config/app.go` — Hubungkan seluruh instans dependensi di dalam fungsi `Bootstrap()`.
9. `db/migrations/<timestamp>_create_warehouses.up.sql` — Tambahkan skrip migrasi tabel PostgreSQL.

---

## Lisensi

Proyek ini dilisensikan di bawah naungan lisensi **MIT License** — lihat berkas [LICENSE](LICENSE) untuk informasi selengkapnya.

---
**Terrion** — *Memberdayakan Koperasi Tani Indonesia Menuju Pertanian Presisi Berkelanjutan.*
