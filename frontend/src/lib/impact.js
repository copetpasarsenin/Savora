// Impact Tracking — Verified-Gram Redistribution Accounting
//
// Helper murni untuk menghitung ringkasan dampak personal customer berdasarkan
// snapshot berat per porsi (weight_per_portion_snapshot) pada pesanan yang lolos
// gerbang kelayakan pickup (COMPLETED, PAID, completed_at terisi, w > 0, q > 0).

/**
 * Hitung ringkasan dampak personal customer dari daftar order.
 * Fail-closed: order tanpa bukti serah-terima (completed_at) atau tanpa snapshot
 * berat valid (> 0 g) berkontribusi 0 gram.
 *
 * @param {Array<{
 *   status?: string,
 *   payment_status?: string,
 *   completed_at?: string | null,
 *   weight_per_portion_snapshot?: number,
 *   quantity?: number,
 *   original_price?: number,
 *   rescue_price?: number,
 * }>} orders daftar order customer.
 * @returns {{
 *   totalPortions: number,
 *   totalSaved: number,
 *   totalGrams: number,
 *   verifiedKg: number,
 *   estimatedKg: number,
 * }}
 */
export function computeImpactSummary(orders) {
  const list = Array.isArray(orders) ? orders : [];

  let totalPortions = 0;
  let totalSaved = 0;
  let totalGrams = 0;

  for (const order of list) {
    // C7: hanya order COMPLETED (case-insensitive)
    const status = String(order?.status ?? "").trim().toLowerCase();
    if (status !== "completed") continue;

    // C5: bila field payment_status dikirim, wajib PAID
    if (order?.payment_status !== undefined && order?.payment_status !== null && String(order.payment_status).trim() !== "") {
      if (String(order.payment_status).trim().toLowerCase() !== "paid") continue;
    }

    // C6: bila field completed_at eksplisit null/kosong, tolak (belum terverifikasi pickup)
    if (order?.completed_at === null || order?.completed_at === "") continue;

    // C2: quantity > 0
    const qty = Number(order.quantity);
    if (!Number.isFinite(qty) || qty <= 0) continue;

    // C1: weight_per_portion_snapshot > 0 (fail-closed -> 0 g jika hilang/tidak valid)
    const unitGrams = Number(order.weight_per_portion_snapshot);
    const orderGrams = Number.isFinite(unitGrams) && unitGrams > 0 ? unitGrams * qty : 0;

    const original = Number(order.original_price);
    const rescue = Number(order.rescue_price);
    let savedPerItem = 0;
    if (Number.isFinite(original) && Number.isFinite(rescue) && original > rescue) {
      savedPerItem = original - rescue;
    }

    totalPortions += qty;
    totalSaved += savedPerItem * qty;
    totalGrams += orderGrams;
  }

  const verifiedKg = +(totalGrams / 1000).toFixed(2);
  return {
    totalPortions,
    totalSaved,
    totalGrams: +totalGrams.toFixed(1),
    verifiedKg,
    estimatedKg: verifiedKg,
  };
}
