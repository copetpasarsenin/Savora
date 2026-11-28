# SavePlate Frontend

Antarmuka web responsive SavePlate menggunakan Next.js 16 (App Router), React 19, JavaScript, dan Tailwind CSS 4.

## Konfigurasi Environment

Salin `.env.example` menjadi `.env.local`:

```bash
cp .env.example .env.local
```

Variabel utama:
- `NEXT_PUBLIC_API_BASE_URL=http://localhost:8000`
- `NEXT_PUBLIC_USE_MOCK=false`

## Perintah Utama

```bash
npm install      # Instalasi dependensi
npm run dev      # Server pengembangan di http://localhost:3000
npm test         # Menjalankan unit test (node --test tests/*.test.js)
npm run build    # Build produksi Next.js
npm run lint     # Pemeriksaan ESLint
```

## Referensi

- PRD SavePlate v3.9: `docs/PRD.md`
- Dokumentasi modul marketplace customer: `docs/customer-marketplace.md`
