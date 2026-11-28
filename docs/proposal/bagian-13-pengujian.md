# Rencana Pengujian SavePlate (Proyek 3 — Bab IV)

Ringkasan skenario pengujian fungsional, integrasi Xendit Invoice Sandbox, RBAC, dan fitur perluasan SavePlate sesuai Proposal Proyek 3 dan PRD v3.9.

| Fitur/Skenario | Data Uji | Hasil yang Diharapkan |
|---|---|---|
| Registrasi dan login | Registrasi valid, email duplikat, password salah, kredensial benar. | Akun dibuat sesuai aturan role; kredensial tidak valid ditolak; login valid memperoleh akses sesuai role. |
| RBAC | Customer memanggil endpoint UMKM/admin; UMKM memanggil endpoint admin; request tanpa token. | Backend menolak akses tidak sesuai dengan respons 401/403 dan tidak mengubah data. |
| Verifikasi UMKM | Profil pending, approve, reject, dan percobaan publish sebelum verifikasi. | Hanya UMKM berstatus `APPROVED` dapat mempublikasikan listing. |
| Listing produk | Data produk valid, stok nol, masa layak habis, kemasan rusak, penyimpanan tidak sesuai. | Validasi input berjalan; produk tidak layak/expired tidak dipasarkan. |
| Food Trust Index | Kombinasi fraksi masa layak, kondisi kemasan, dan metode penyimpanan sesuai PRD 12.4. | Status mengikuti aturan if-else PRD dan disclaimer ditampilkan. |
| Food Score Decay | Sisa waktu 8, 6, 4, 2, 1, dan 0 jam untuk masa layak 8 jam. | Skor mengikuti power decay `gamma = 0.65` (`100, 83, 64, 41, 26, 0`). |
| Dynamic discount | Produk Fresh, Layak Dijual, Segera Dijual, Tidak Disarankan Dijual, dan `min_price`. | Rentang diskon sesuai status; produk tidak layak dibatasi; harga tidak melewati guardrail minimum. |
| Marketplace dan filter | Pencarian kata kunci, kategori, rentang harga, lokasi, urutan, produk habis/expired. | Daftar yang tampil sesuai kriteria dan hanya memuat listing aktif. |
| Checkout dan pembayaran (Xendit Sandbox) | Order valid, stok berubah, webhook `x-callback-token` valid (`PAID`/`EXPIRED`), token salah/kosong, webhook duplikat. | Order dan status pembayaran berubah hanya setelah verifikasi `x-callback-token` secara constant-time; tercatat di `payment_logs`; stok dikembalikan saat `EXPIRED`. |
| Pickup code dan no-show | Pickup code valid, salah, dipakai ulang, serta pesanan melewati deadline. | Kode valid menyelesaikan order; kode salah/duplikat ditolak; status `NO_SHOW`/`EXPIRED` konsisten. |
| Review dan keyword | Review sebelum `COMPLETED`, review setelah `COMPLETED`, keyword Aman/Warning/Gawat, review berulang. | Review hanya diterima pada order `COMPLETED`; keyword tersimpan dan agregasi badge mengikuti threshold PRD 12.7. |
| Admin moderation | Approve/reject UMKM, suspend listing, warning atau suspend akun. | Aksi sesuai role admin dan tercatat di `audit_logs`. |
| Fitur perluasan | Help ticket, waste log, analitik, iklan, mitra, notifikasi, export. | Dinyatakan selesai hanya bila UI, API, data, RBAC, dan test berjalan konsisten. |
