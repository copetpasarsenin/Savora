# SavePlate — Food Rescue Marketplace untuk UMKM

SavePlate adalah aplikasi web food rescue marketplace yang mempertemukan UMKM kuliner dengan konsumen untuk memanfaatkan makanan surplus yang masih layak konsumsi dengan harga terjangkau melalui mekanisme self-pickup.

Proyek ini dikembangkan untuk Mata Kuliah Proyek 3 Program Studi D4 Teknik Informatika, Universitas Logistik & Bisnis Internasional (ULBI) Bandung:
- Richard Firmansyah (714240047)
- Wa Ode Nur Alia (714240035)

## Fitur Utama

- Marketplace makanan surplus dengan pencarian, filter kategori/harga/lokasi, dan keranjang belanja.
- Indikator kelayakan berbasis aturan: Food Trust Index dan Food Score Decay (`gamma = 0.65`) disertai disclaimer non-laboratorium.
- Rekomendasi diskon dinamis berbasis aturan dengan guardrail `min_price`.
- Checkout cashless menggunakan **Xendit Invoice Sandbox** (`XENDIT_SANDBOX`, durasi pembayaran 15 menit, service fee 5%, verifikasi callback `x-callback-token` secara constant-time, dan audit `payment_logs`).
- Penerbitan dan validasi `pickup_code` oleh UMKM serta penanganan `EXPIRED` / `NO_SHOW` via scheduler.
- Ulasan pelanggan pasca-transaksi selesai dengan klasifikasi keyword (`AMAN`, `WARNING`, `GAWAT`) dan moderasi Admin.
- Modul perluasan: Dashboard Admin, Verifikasi UMKM & Mitra, Help Center, Waste Log, Iklan, dan Analitik UMKM.

## Teknologi

| Komponen | Teknologi |
|---|---|
| Frontend | Next.js 16, React 19, JavaScript, Tailwind CSS 4, Recharts, Lucide React |
| Backend | Go 1.26.2, Fiber v2.52.14, JWT v5.2.1, bcrypt |
| ORM & Database | GORM v1.31.2, PostgreSQL |
| Payment Gateway | Xendit Invoice API v2 (Sandbox) |
| Pengujian | Node Test Runner (`node --test`), Go Test (`go test`), Playwright E2E |

## Struktur Proyek

```text
SavePlate/
├── backend/                  # REST API Go + Fiber + GORM
├── frontend/                 # Aplikasi web Next.js 16 + React 19
├── tests/                    # Skenario E2E Playwright & dokumentasi cakupan uji
├── docs/
│   ├── SavePlate_PRD_v3.9.md # Product Requirements Document (sumber kebenaran)
│   └── CLAUDE.md             # Panduan arsitektur & aturan bisnis teknis
└── playwright.config.js
```

## Menjalankan Secara Lokal

### 1. Backend (`:8000` / sesuai `PORT`)

```bash
cd backend
cp .env.example .env
# Isi DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, JWT_SECRET,
# XENDIT_SECRET_KEY, XENDIT_CALLBACK_TOKEN
go run main.go
```

Seeder akun demo (opsional):

```bash
cd backend
go run cmd/seed/main.go
```

### 2. Frontend (`:3000`)

```bash
cd frontend
cp .env.example .env.local
npm install
npm run dev
```

## Pengujian

```bash
# Unit test frontend
cd frontend && npm test

# Build & test backend
cd backend && go build ./... && go test ./middleware/... ./services/...

# End-to-End Playwright (membutuhkan backend & frontend aktif + data uji)
npx playwright test
```

## Dokumentasi Terkait

- PRD v3.9: `docs/SavePlate_PRD_v3.9.md` dan `frontend/docs/PRD.md`
- Spesifikasi Admin: `docs/admin-dashboard-spec.md`, `docs/admin-endpoints.md`
- Panduan Pengujian E2E: `tests/README-PLAYWRIGHT.md`, `tests/TEST-COVERAGE-SUMMARY.md`
