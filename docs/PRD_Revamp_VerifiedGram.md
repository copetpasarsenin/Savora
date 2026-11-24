# PRD Revamp Aplikasi SavePlate — Penyelarasan Mekanisme *Verified-Gram*

**Versi:** 4.0-revamp (suplemen terfokus dari `SavePlate_PRD_v3.9.md`)  
**Tanggal:** 22 November 2026  
**Acuan Naskah:** `Docs/Jurnal_SavePlate_JUTIF.tex`  
**Stack:** Next.js 16 + React 19 (Frontend), Go 1.26 + Fiber v2 + GORM + PostgreSQL (Backend)

---

## 1. Latar Belakang & Tujuan

Audit terhadap kode produksi SavePlate dibandingkan dengan naskah jurnal JUTIF (*Verified-Gram Redistribution Accounting with Pickup Verification*) menemukan enam celah implementasi: mekanisme `EligibilityGate` baru dipakai di unit test, verifikasi kode pickup dapat dilewati melalui endpoint status umum, input berat produk belum wajib di backend/frontend, terdapat bug SQL pada scheduler kedaluwarsa, endpoint mutasi inti belum terlindungi otorisasi, dan `umkm_id = 1` masih di-*hardcode* di halaman UMKM.

**Tujuan revamp ini:**
1. Menyelaraskan perilaku aplikasi produksi secara penuh dengan klaim mekanisme *verified-gram* di naskah jurnal (pencatatan berat saat listing, *snapshot* berat saat order, verifikasi kode pickup sebagai gerbang wajib, evaluasi 7 kondisi `EligibilityGate`, dan jejak audit per transaksi).
2. Menutup celah keamanan dan bug kritis yang dapat mematahkan demonstrasi maupun alur transaksi nyata.
3. Menyiapkan aplikasi agar siap dipakai pada **Eksperimen 3 (RQ3 — Evaluasi Berbasis Pengguna)** bersama partisipan peran UMKM dan pelanggan tersimulasi untuk mengukur TCR, TCT, dan DIA.

---

## 2. Ruang Lingkup

### Termasuk (*In Scope*)
- Integrasi `services.EligibilityGate` dan `services.CalculateRedistributionWeight` ke laporan produksi (Admin, UMKM, Customer).
- Penegakan gerbang verifikasi `pickup_code` di backend dan penyatuan manajemen pesanan UMKM pada satu halaman (`/pesanan`).
- Validasi wajib berat satuan (`weight_per_portion > 0`, dalam gram) di backend dan seluruh form produk frontend.
- Perbaikan bug SQL pada `SchedulerService.ExpireStalePayments` serta penegakan batas waktu `pickup_deadline`.
- Perlindungan `AuthMiddleware` + pemeriksaan kepemilikan (*ownership*) pada endpoint produk dan pesanan, serta penghapusan *hardcode* `umkm_id = 1`.
- Pembersihan registrasi route duplikat dan penyelarasan metrik analitik agar tidak menampilkan angka dummy/estimasi.

### Di Luar Ruang Lingkup (*Out of Scope*)
- Modul Iklan (`ads`), Mitra Donasi, Mitra Pengolah, Promosi, dan Review Keyword.
- Dukungan multi-produk per pesanan (*cart checkout* tetap 1 order per produk) dan *partial pickup* / pembatalan sebagian item.
- Perhitungan emisi CO₂, *Life Cycle Assessment* (LCA), integrasi timbangan IoT, maupun *blockchain* (secara eksplisit dikecualikan oleh naskah).
- Redesign tampilan visual di luar elemen yang dibutuhkan alur di atas.

---

## 3. Kesenjangan Saat Ini (*As-Is* vs *To-Be*)

| Area | Kondisi Saat Ini (*As-Is*) | Target (*To-Be*) |
|---|---|---|
| Perhitungan berat redistribusi | `EligibilityGate` hanya dipanggil di `redistribution_test.go`; `admin.go` hanya filter `status = COMPLETED`; `impact.js` pakai estimasi `0.4 kg/porsi`; Dashboard UMKM *hardcode* `234 kg` & `156 kg CO₂`. | Satu sumber kebenaran via `services.CalculateRedistributionWeight`; seluruh tampilan berat menampilkan data terverifikasi (gram/kg) beserta rincian audit; angka dummy & klaim CO₂ dihapus. |
| Verifikasi pickup | `PATCH /api/orders/:id/status` mengizinkan `READY_FOR_PICKUP → COMPLETED` tanpa `pickup_code` & tanpa `completed_at`; Sidebar ke `/pesanan` (bypass), sedangkan input kode terpisah di `/dashboard/orders` (tanpa token auth). | Transisi ke `COMPLETED` **hanya** melalui `POST /api/orders/:id/validate-pickup`; input kode pickup dipindah ke `/pesanan`; `/dashboard/orders` dihapus/dialihkan ke `/pesanan`. |
| Input berat produk | Backend terima `weight_per_portion <= 0`; `/produk/tambah` pakai teks bebas (`parseFloat("0.5 kg") -> 0.5`); Mystery Bag tidak mengirim berat; modal `/produk` tidak punya field berat. | Backend wajibkan `weight_per_portion > 0` (gram); semua form tambah/edit produk memakai input numerik wajib dalam satuan gram (`g`). |
| Scheduler & deadline | `scheduler.go:58` pakai kolom salah `order_id` pada tabel `orders` (gagal SQL); `ValidatePickupCode` belum menolak pickup lewat `pickup_deadline`. | Query pakai `id`; `ValidatePickupCode` menolak order yang melewati `pickup_deadline` (sesuai keputusan terbuka §9.1). |
| Keamanan & multi-UMKM | `POST/PUT/DELETE /api/products`, `GET /api/orders/:id`, `PATCH /api/orders/:id/status`, `POST /api/orders/:id/validate-pickup` tanpa `AuthMiddleware` & tanpa cek kepemilikan; frontend *hardcode* `umkm_id = 1`. | Seluruh endpoint mutasi & detail order dilindungi JWT + cek kepemilikan peran; frontend mengambil `umkm_id` dari sesi login (`getUser().id`) / backend menurunkannya dari JWT. |
| Route & analitik | `/products` dan `/waste-logs` didaftarkan dua kali (`main.go` & `routes/routes.go`); `GetUmkmInsight` menghitung `PAID`/`READY_FOR_PICKUP` sebagai terjual + *hardcode* rating `4.8`. | Registrasi route tunggal tanpa duplikasi; metrik penjualan/redistribusi hanya menghitung order terverifikasi atau memisahkan status secara jelas. |

---

## 4. Spesifikasi Kebutuhan Fungsional

### RF-01: Integrasi `EligibilityGate` & Laporan Redistribusi Ter-audit
- **Aturan Bisnis:**
  1. Seluruh perhitungan berat redistribusi ($W_d$) wajib melewati `services.CalculateRedistributionWeight` (Algoritma 1: kondisi C1–C7).
  2. Hitung metrik turunan secara konsisten dengan definisi naskah & kode:
     - $C = \{i \in I \mid \texttt{PaymentStatus} = \texttt{"PAID"} \land w_i > 0 \land q_i > 0\}$
     - $W_c = \sum_{i \in C} (w_i \times q_i)$
     - $W_d = \sum_{i \in E} (w_i \times q_i)$
     - $\text{OBR} = (W_c - W_d) / W_d$ (bila $W_d > 0$; jika $W_d = 0$, kembalikan `null` dan laporkan selisih absolut $W_c$)
     - $\text{EC} = |E| / |C|$ (bila $|C| > 0$; jika $|C| = 0$, kembalikan `null`)
  3. Setiap item pesanan pada laporan audit menyertakan `order_id`, `product_id`, `product_name`, `weight_per_portion_snapshot`, `quantity`, `status`, `payment_status`, `completed_at`, `eligible`, `failed_conditions`, dan `weight_contribution_grams`.
- **Backend:**
  - Tambah helper konversi `models.Order` → `services.OrderInput` dan fungsi agregasi laporan (termasuk $W_c$, OBR, EC) di `backend/services/redistribution.go`.
  - Tambah endpoint terautentikasi:
    - `GET /api/redistribution/report` — otomatis terfilter sesuai JWT: `ADMIN` (semua order, opsional filter `umkm_id`), `UMKM` (hanya order milik UMKM tersebut), `CUSTOMER` (hanya order milik customer tersebut).
  - Perbarui `GetAdminSummaryHandler` (`backend/handlers/admin.go`) agar `makanan_diselamatkan_kg`, `total_makanan_diselamatkan_kg`, dan `top_umkm[].food_rescued_kg` menggunakan filter kelayakan yang setara C1–C7 (atau langsung memanggil `CalculateRedistributionWeight`).
- **Frontend:**
  - **Dashboard UMKM** (`frontend/src/app/dashboard/page.js`): ganti blok dummy `234 kg` / `89 kg` / `156 kg Emisi CO₂` dengan data nyata dari `/api/redistribution/report`: total berat terverifikasi ($W_d$), jumlah pesanan layak ($|E|$), berat belum terverifikasi / *phantom rescue* ($W_c - W_d$), serta tabel ringkas audit per order. Hapus klaim emisi CO₂.
  - **Admin Dashboard** (`frontend/src/app/admin/dashboard/page.js`): tampilkan $W_d$, $W_c$, OBR, dan EC dari data nyata serta tautan/tabel audit order.
  - **Akun Customer** (`frontend/src/lib/impact.js` & `frontend/src/app/akun/page.js`): ganti perhitungan `KG_PER_PORTION = 0.4` dengan jumlah `weight_per_portion_snapshot * quantity` dari order yang lolos syarat (`status === "COMPLETED"`, `payment_status === "PAID"`, `completed_at != null`, `weight_per_portion_snapshot > 0`, `quantity > 0`). Sertakan `weight_per_portion_snapshot` dan `payment_status` dalam normalisasi order di `akun/page.js`.

### RF-02: Penegakan Gerbang Verifikasi Pickup & Penyatuan Halaman Pesanan UMKM
- **Aturan Bisnis:**
  1. Status `COMPLETED` **dilarang** ditetapkan lewat `PATCH /api/orders/:id/status`. Satu-satunya jalur menuju `COMPLETED` adalah `POST /api/orders/:id/validate-pickup`.
  2. `ValidatePickupCode` hanya berhasil bila: order milik UMKM yang login, `status == READY_FOR_PICKUP`, `payment_status == PAID`, `pickup_code` cocok (case-insensitive atau trim spasi), dan (bila §9.1 disetujui) `now <= pickup_deadline`.
  3. Verifikasi kedua atas order yang sama wajib ditolak (idempoten, sifat P3) dengan pesan error yang jelas.
- **Backend:**
  - `backend/handlers/order.go` (`UpdateOrderStatus`): tolak `req.Status == models.OrderCompleted` dengan HTTP `400` (`"Gunakan verifikasi kode pickup untuk menyelesaikan pesanan"`).
  - `backend/handlers/order.go` (`ValidatePickupCode`): tambahkan pengecekan kepemilikan UMKM, `payment_status == PAID`, dan guard `pickup_deadline`.
  - Pastikan `GetOrders` dan `GetOrderDetail` menyertakan `weight_per_portion_snapshot` dan data nama/telepon customer (perhatikan `normalizeUMKMOrder` di `frontend/src/lib/orders.js:142` saat ini membaca `base.user`, padahal backend mengirim field `customer`!).
- **Frontend:**
  - **Halaman `/pesanan`** (`frontend/src/app/pesanan/page.js`):
    - Saat `backend_status === "PAID"`: tombol **Siapkan Pesanan** (`PATCH /status` → `READY_FOR_PICKUP`).
    - Saat `backend_status === "READY_FOR_PICKUP"`: ganti tombol langsung "Selesaikan Pesanan" dengan form input **Kode Pickup** + tombol **Verifikasi & Selesaikan** yang memanggil `POST /api/orders/:id/validate-pickup` via `apiFetch` (dengan JWT). Tampilkan pesan sukses beserta berat terverifikasi ($w_i \times q_i$ gram) atau pesan error bila kode salah/kedaluwarsa.
    - Perbaiki modal Darurat (No-Show): kirim `NO_SHOW` (bukan `"Didonasikan"`); hapus atau nonaktifkan tombol `"Dibatalkan"` untuk order yang sudah `PAID` karena state machine backend tidak mengizinkan `PAID → CANCELLED`.
    - Hapus `useEffect` duplikat pada `pesanan/page.js:58-66` dan ganti `fetchUMKMOrders(1)` agar memakai sesi login.
    - Perbaiki pemetaan status `NO_SHOW` di `pesanan/page.js:53` dari `'Didonasikan'` menjadi `'No-Show'`.
  - **Halaman `/dashboard/orders`** (`frontend/src/app/dashboard/orders/page.js`): ganti menjadi redirect ke `/pesanan` agar tidak ada dua alur pesanan yang saling bertentangan.
  - **Helper `frontend/src/lib/orders.js`**: tambah fungsi `validatePickupCode(orderId, pickupCode)` memakai `apiFetch`; perbaiki `normalizeUMKMOrder` membaca `base.customer` (fallback `base.user`), `weight_per_portion_snapshot`, `completed_at`, dan `pickup_deadline`; perbarui `allowedUmkmTransitions` untuk mencerminkan bahwa `COMPLETED` melalui verifikasi pickup (bukan `updateOrderStatus`).

### RF-03: Wajib Input Berat Satuan (Gram) di Seluruh Alur Produk
- **Aturan Bisnis:**
  1. Setiap produk yang dibuat atau diperbarui wajib memiliki `weight_per_portion > 0` (dalam satuan gram, `float64`).
  2. `CreateOrder` wajib menolak pemesanan bila `product.WeightPerPortion <= 0` (*fail-closed* sejak pembuatan order).
  3. Perubahan `weight_per_portion` pada produk tidak pernah mengubah `weight_per_portion_snapshot` pada order yang sudah ada (sifat P4 — sudah terpenuhi di DB, dipertahankan dan diuji).
- **Backend (`backend/handlers/product.go` & `backend/services/order.go`):**
  - `CreateProduct`: tolak jika `weight_per_portion <= 0` (HTTP `422`), `stock < 0`, atau `name == ""`.
  - `UpdateProduct`: tolak jika payload mengirim `weight_per_portion <= 0` (HTTP `422`).
  - `CreateOrder` (`backend/services/order.go`): tambahkan guard `if product.WeightPerPortion <= 0 { return errors.New("berat produk belum valid") }`.
- **Frontend:**
  - `/produk/tambah` (`frontend/src/app/produk/tambah/page.js`):
    - Ubah field berat menjadi input angka positif (`type="number"`, `min="1"`, suffix `gram`) dan wajib diisi sebelum submit.
    - Pastikan mode Mystery Bag (jika tetap diaktifkan, lihat §9.2) juga mengisi `weight_per_portion` numerik dalam gram.
  - `/produk` (`frontend/src/app/produk/page.js`) & `frontend/src/lib/products.js`:
    - Tambahkan `weight_per_portion` di `normalizeProduct`, state form modal tambah/edit, tampilan kartu/tabel produk, dan payload `createProduct`/`updateProduct`.

### RF-04: Perbaikan Bug Scheduler & Penegakan `pickup_deadline`
- **Backend (`backend/services/scheduler.go`):**
  - Baris 58: perbaiki `Where("order_id = ? AND status = ?", ...)` menjadi `Where("id = ? AND status = ?", payment.OrderID, models.OrderPaymentPending)`.
  - `ProcessNoShow`: gunakan `Updates(map[string]interface{}{"status": models.OrderNoShow})` dengan kondisi `Where("id = ? AND status IN (?, ?)", ...)` alih-alih `db.Save(&order)` agar aman dari *race condition* dengan `ValidatePickupCode`.
- **Penyelarasan `pickup_deadline` (tergantung keputusan §9.1):**
  - Jika P2 dipertahankan: `ValidatePickupCode` menolak bila `order.PickupDeadline != nil && now.After(*order.PickupDeadline)`, dan opsi penambahan pengecekan di `EligibilityGate` bila dibutuhkan konsistensi formal.

### RF-05: Keamanan Endpoint, Kepemilikan Data & Penghapusan Hardcode `umkm_id = 1`
- **Backend (`backend/main.go`, `backend/handlers/product.go`, `backend/handlers/order.go`):**
  - Pasang `middleware.AuthMiddleware` pada:
    - `POST /api/products` (hanya `UMKM`/`ADMIN`; paksa `product.UmkmID = claims.UserID` untuk role `UMKM`).
    - `PUT /api/products/:id` dan `DELETE /api/products/:id` (cek `product.UmkmID == claims.UserID` atau `ADMIN`).
    - `GET /api/orders/:id` (hanya customer pemilik order, UMKM pemilik produk, atau `ADMIN`; hanya tampilkan `pickup_code` ke customer pemilik & admin, **jangan** bocorkan `pickup_code` ke respons list/detail UMKM agar UMKM benar-benar menerima kode dari customer!).
    - `PATCH /api/orders/:id/status` (hanya UMKM pemilik produk atau `ADMIN`).
    - `POST /api/orders/:id/validate-pickup` (hanya UMKM pemilik produk atau `ADMIN`).
  - Tambahkan field `WeightPerPortionSnapshot` pada `OrderDetailResponse` di `GetOrderDetail`.
- **Frontend:**
  - Ganti seluruh hardcode `umkm_id: 1`, `UMKM_ID = 1`, `fetchUMKMProducts(1)`, dan `fetchUMKMOrders(1)` di `produk/tambah/page.js`, `produk/page.js`, `pesanan/page.js`, `dashboard/page.js`, `analitik/page.js` agar membaca ID dari `getUser()?.id`.
  - Perbarui `UmkmContext.js` agar memprioritaskan data user yang sedang login (`getUser()`) bukan hardcoded `"Ibu Lestari"`.

### RF-06: Pembersihan Route Duplikat & Sinkronisasi Analitik Minimal
- **Backend:**
  - Hapus duplikasi registrasi `/api/products*` dan `/api/waste-logs*` antara `backend/main.go` dan `backend/routes/routes.go` (tetapkan satu tempat registrasi).
  - Pada `GetUmkmInsight` (`backend/handlers/analytics.go:51-71`): filter order selesai hanya `status = COMPLETED AND payment_status = PAID AND completed_at IS NOT NULL`, bukan mencampur `PAID`/`READY_FOR_PICKUP`.

---

## 5. Perubahan Kontrak API & Skema Data

### Skema Basis Data
**Tidak ada perubahan skema/migrasi baru.** Seluruh kolom yang dibutuhkan (`products.weight_per_portion`, `orders.weight_per_portion_snapshot`, `orders.pickup_code`, `orders.pickup_deadline`, `orders.payment_status`, `orders.status`, `orders.completed_at`) sudah ada di `backend/models/`.

### Ringkasan Endpoint Baru / Berubah

| Method & Path | Perubahan | Auth & Role |
|---|---|---|
| `GET /api/redistribution/report` | **Baru.** Mengembalikan ringkasan ($W_d$, $W_c$, `phantom_rescue_grams`, `obr`, `ec`, `eligible_count`, `claimed_count`, `total_count`) + array `items[]` (audit per order beserta `failed_conditions`). | Wajib JWT (`ADMIN`, `UMKM`, `CUSTOMER` — terfilter otomatis) |
| `POST /api/products` | Tambah validasi `weight_per_portion > 0`; `umkm_id` diambil dari JWT. | Wajib JWT (`UMKM`, `ADMIN`) |
| `PUT /api/products/:id` | Tambah validasi `weight_per_portion > 0`; cek kepemilikan. | Wajib JWT (`UMKM` pemilik, `ADMIN`) |
| `DELETE /api/products/:id` | Cek kepemilikan. | Wajib JWT (`UMKM` pemilik, `ADMIN`) |
| `GET /api/orders/:id` | Tambah `weight_per_portion_snapshot`; sembunyikan `pickup_code` dari role `UMKM`; cek kepemilikan. | Wajib JWT (`CUSTOMER` pemilik, `UMKM` pemilik, `ADMIN`) |
| `PATCH /api/orders/:id/status` | Tolak target status `COMPLETED`; cek kepemilikan. | Wajib JWT (`UMKM` pemilik, `ADMIN`) |
| `POST /api/orders/:id/validate-pickup` | Cek kepemilikan, `payment_status == PAID`, dan `pickup_deadline`. | Wajib JWT (`UMKM` pemilik, `ADMIN`) |
| `GET /api/admin/reports/summary` | Hitung berat redistribusi memakai kriteria penuh C1–C7. | Wajib JWT (`ADMIN`) |

> **Catatan keamanan penting:** Pada `GET /api/orders` (list untuk UMKM) dan `GET /api/orders/:id` (untuk UMKM), field `pickup_code` **harus dikosongkan (`nil`)** agar UMKM tidak bisa melihat kode di network response / UI lalu memvalidasi sendiri tanpa serah-terima fisik dari pelanggan.

---

## 6. Dampak ke File & Pengujian

### File Kode yang Akan Diubah
- **Backend:**
  - `backend/services/redistribution.go` (tambah mapper `Order -> OrderInput` + agregasi metrik $W_c$, OBR, EC)
  - `backend/services/order.go` (guard `weight_per_portion > 0` di `CreateOrder`)
  - `backend/services/scheduler.go` (perbaiki bug kolom `order_id` → `id`, update atomik `ProcessNoShow`)
  - `backend/handlers/redistribution.go` (**file baru**, handler `GET /api/redistribution/report`)
  - `backend/handlers/order.go` (blokir `COMPLETED` di `UpdateOrderStatus`, cek auth/ownership/deadline, mask `pickup_code` untuk UMKM, tambah snapshot di DTO)
  - `backend/handlers/product.go` (validasi `weight_per_portion > 0`, auth/ownership)
  - `backend/handlers/admin.go` (pakai aturan C1–C7 untuk berat terselamatkan)
  - `backend/handlers/analytics.go` (filter order selesai terverifikasi di `GetUmkmInsight`)
  - `backend/main.go` & `backend/routes/routes.go` (pasang auth middleware, daftarkan `/redistribution/report`, hapus route duplikat)
- **Frontend:**
  - `frontend/src/lib/orders.js`, `frontend/src/lib/products.js`, `frontend/src/lib/impact.js`
  - `frontend/src/context/UmkmContext.js`
  - `frontend/src/app/produk/tambah/page.js`, `frontend/src/app/produk/page.js`
  - `frontend/src/app/pesanan/page.js`, `frontend/src/app/dashboard/orders/page.js`
  - `frontend/src/app/dashboard/page.js`, `frontend/src/app/admin/dashboard/page.js`, `frontend/src/app/akun/page.js`, `frontend/src/app/analitik/page.js`

### Suite Pengujian yang Wajib Diperbarui / Ditambah
1. `backend/services/redistribution_test.go` — pertahankan 15 TC & perbandingan komponen (juga pertimbangkan sinkronisasi nama fungsi tes dengan naskah), tambah test metrik $W_c$, OBR, EC termasuk kasus batas $W_d = 0$ dan $|C| = 0$.
2. `backend/handlers/order_test.go` — tambah test:
   - `PATCH /orders/:id/status` dengan `COMPLETED` ditolak (HTTP 400).
   - `POST /orders/:id/validate-pickup` gagal tanpa token, gagal oleh UMKM lain, gagal jika kode salah, gagal pada pemanggilan kedua (idempoten P3), gagal jika lewat `pickup_deadline` (bila §9.1 disetujui).
   - `GET /orders` dan `GET /orders/:id` untuk role UMKM tidak membocorkan `pickup_code`.
3. `backend/services/order_test.go` — tambah test `CreateOrder` menolak produk dengan `weight_per_portion <= 0` dan `ExpireStalePayments` memperbarui `orders.id` dengan benar.
4. `frontend/tests/impact.test.js` — perbarui ekspektasi dari konstanta `0.4 kg` menjadi berbasis `weight_per_portion_snapshot * quantity` dengan gerbang `COMPLETED` + `PAID` + `completed_at`.
5. `frontend/tests/orders.test.js` & `frontend/tests/products.test.js` — sesuaikan dengan `validatePickupCode`, normalisasi `customer` + `weight_per_portion_snapshot`, dan `weight_per_portion` di `normalizeProduct`.

---

## 7. Urutan Eksekusi

- **Fase 1 — Backend Integrity & Security (RF-04, RF-05, RF-03 backend, RF-02 backend, RF-01 backend, RF-06):**  
  Perbaiki scheduler, pasang auth + ownership + masking `pickup_code`, wajibkan `weight_per_portion > 0`, blokir bypass `COMPLETED`, tambah `GET /api/redistribution/report`, perbarui `admin.go`, bersihkan duplikasi route. Verifikasi dengan `go test ./...`.
- **Fase 2 — Alur Inti Frontend UMKM & Customer (RF-03 frontend, RF-02 frontend, RF-05 frontend):**  
  Perbaiki input berat wajib (gram) di `/produk/tambah` & `/produk`, satukan verifikasi `pickup_code` di `/pesanan`, redirect `/dashboard/orders`, hapus hardcode `umkm_id = 1`.
- **Fase 3 — Laporan Redistribusi & Verifikasi End-to-End (RF-01 frontend + seluruh test):**  
  Sambungkan Dashboard UMKM, Admin Dashboard, dan `/akun` (`impact.js`) ke data nyata *verified-gram*, hapus dummy CO₂, perbarui seluruh unit test frontend (`node --test`), dan jalankan uji coba alur lengkap Eksperimen 3.

---

## 8. Kriteria Penerimaan (*Acceptance Criteria*)

1. **AC-01 (Wajib berat):** Membuat/mengedit produk dengan berat `0`, negatif, atau kosong ditolak di UI dan API (HTTP 422).
2. **AC-02 (Snapshot invarian):** Mengubah `weight_per_portion` produk setelah pesanan dibuat tidak mengubah `weight_per_portion_snapshot` maupun $W_d$ pada laporan.
3. **AC-03 (Tanpa bypass pickup):** Mengirim `PATCH /api/orders/:id/status` dengan `{"status":"COMPLETED"}` selalu ditolak (HTTP 400); status `COMPLETED` dan `completed_at` hanya terisi setelah `POST /api/orders/:id/validate-pickup` dengan kode yang benar.
4. **AC-04 (Kerahasiaan kode pickup):** Respons API untuk role `UMKM` tidak pernah mengandung `pickup_code`; hanya pelanggan pemilik order dan admin yang melihat kode tersebut.
5. **AC-05 (Konsistensi angka lintas peran):** Total berat redistribusi di Admin Dashboard, Dashboard UMKM, dan `/akun` seluruhnya bersumber dari `weight_per_portion_snapshot * quantity` untuk order yang lolos C1–C7 — tanpa konstanta `0.4 kg`, tanpa angka dummy, dan tanpa klaim CO₂.
6. **AC-06 (Jejak audit):** Dari laporan redistribusi, setiap pesanan dapat ditelusuri ke `order_id`, `product_id`, `weight_per_portion_snapshot`, `quantity`, `completed_at`, dan alasan penolakan bila tidak layak.
7. **AC-07 (Multi-UMKM siap Eksperimen 3):** Dua akun UMKM berbeda (bukan ID 1) dapat membuat produk, menerima pesanan, memverifikasi kode pickup, dan melihat laporan masing-masing secara terisolasi.
8. **AC-08 (Regresi lulus):** `go test ./services/... ./handlers/...` dan `node --test frontend/tests/*.test.js` seluruhnya lulus tanpa skip baru.

---

## 9. Keputusan Terbuka (Konfirmasi Sebelum Implementasi)

1. **Penegakan `pickup_deadline` (Sifat P2):** Apakah `ValidatePickupCode` (dan opsional `EligibilityGate`) harus menolak pickup jika `now > pickup_deadline` agar cocok dengan pernyataan P2 di naskah, atau batas waktu cukup ditangani scheduler `ProcessNoShow` dan P2 di naskah yang disesuaikan?
2. **Mode Mystery Bag di `/produk/tambah`:** Apakah Mystery Bag juga diwajibkan mengisi berat numerik dalam gram (`weight_per_portion`), atau mode Mystery Bag disembunyikan sementara untuk ruang lingkup penelitian ini?
3. **Alur pembayaran Xendit saat Eksperimen 3:** Pembayaran memakai Xendit Sandbox nyata (webhook masuk lewat tunnel/URL publik), atau perlu tombol/endpoint simulasi pembayaran khusus mode `DEV`/`SANDBOX` agar partisipan Eksperimen 3 bisa menyelesaikan skenario di jaringan lokal tanpa setup webhook eksternal?
