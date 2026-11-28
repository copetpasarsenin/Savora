import test from "node:test";
import assert from "node:assert/strict";
import { computeImpactSummary } from "../src/lib/impact.js";

// ---------------------------------------------------------------------------
// computeImpactSummary — kasus dasar (verified-gram berbasis snapshot berat)
// ---------------------------------------------------------------------------

test("computeImpactSummary: order kosong → semua nol", () => {
  const result = computeImpactSummary([]);
  assert.equal(result.totalPortions, 0);
  assert.equal(result.totalSaved, 0);
  assert.equal(result.totalGrams, 0);
  assert.equal(result.verifiedKg, 0);
  assert.equal(result.estimatedKg, 0);
});

test("computeImpactSummary: input bukan array → semua nol", () => {
  const zero = { totalPortions: 0, totalSaved: 0, totalGrams: 0, verifiedKg: 0, estimatedKg: 0 };
  assert.deepEqual(computeImpactSummary(null), zero);
  assert.deepEqual(computeImpactSummary(undefined), zero);
  assert.deepEqual(computeImpactSummary("bukan array"), zero);
});

// ---------------------------------------------------------------------------
// Order campuran status — hanya Completed + PAID + completed_at yang dihitung
// ---------------------------------------------------------------------------

test("computeImpactSummary: hanya menghitung order Completed terverifikasi dengan snapshot berat", () => {
  const orders = [
    { status: "Completed", payment_status: "PAID", completed_at: "2026-07-21T13:15:00Z", quantity: 2, weight_per_portion_snapshot: 350, original_price: 25000, rescue_price: 12000 },
    { status: "Paid", payment_status: "PAID", completed_at: null, quantity: 1, weight_per_portion_snapshot: 400, original_price: 30000, rescue_price: 15000 },
    { status: "Cancelled", payment_status: "UNPAID", completed_at: null, quantity: 3, weight_per_portion_snapshot: 300, original_price: 20000, rescue_price: 10000 },
    { status: "Completed", payment_status: "PAID", completed_at: "2026-07-21T14:00:00Z", quantity: 1, weight_per_portion_snapshot: 500, original_price: 45000, rescue_price: 22000 },
  ];
  const result = computeImpactSummary(orders);
  // Completed: 2×350g = 700g + 1×500g = 500g => 1200g (1.2 kg), 3 porsi, hemat 49000
  assert.equal(result.totalPortions, 3);
  assert.equal(result.totalSaved, 49000);
  assert.equal(result.totalGrams, 1200);
  assert.equal(result.verifiedKg, 1.2);
  assert.equal(result.estimatedKg, 1.2);
});

test("computeImpactSummary: tolak Completed tanpa bukti pickup (completed_at = null) atau belum PAID", () => {
  const orders = [
    { status: "COMPLETED", payment_status: "PAID", completed_at: null, quantity: 2, weight_per_portion_snapshot: 350, original_price: 25000, rescue_price: 12000 },
    { status: "COMPLETED", payment_status: "PENDING", completed_at: "2026-07-21T13:15:00Z", quantity: 2, weight_per_portion_snapshot: 350, original_price: 25000, rescue_price: 12000 },
  ];
  const result = computeImpactSummary(orders);
  assert.equal(result.totalPortions, 0);
  assert.equal(result.totalGrams, 0);
  assert.equal(result.verifiedKg, 0);
});

test("computeImpactSummary: case-insensitive status matching", () => {
  const orders = [
    { status: "completed", quantity: 1, weight_per_portion_snapshot: 300, original_price: 20000, rescue_price: 10000 },
    { status: "COMPLETED", quantity: 1, weight_per_portion_snapshot: 300, original_price: 20000, rescue_price: 10000 },
    { status: " Completed ", quantity: 1, weight_per_portion_snapshot: 400, original_price: 20000, rescue_price: 10000 },
  ];
  const result = computeImpactSummary(orders);
  assert.equal(result.totalPortions, 3);
  assert.equal(result.totalSaved, 30000);
  assert.equal(result.totalGrams, 1000);
  assert.equal(result.verifiedKg, 1);
});

// ---------------------------------------------------------------------------
// qty > 1 — berat dihitung dari weight_per_portion_snapshot × quantity
// ---------------------------------------------------------------------------

test("computeImpactSummary: qty > 1 dihitung dari snapshot berat aktual", () => {
  const orders = [
    { status: "Completed", quantity: 5, weight_per_portion_snapshot: 480, original_price: 15000, rescue_price: 6000 },
  ];
  const result = computeImpactSummary(orders);
  assert.equal(result.totalPortions, 5);
  assert.equal(result.totalSaved, 45000);
  assert.equal(result.totalGrams, 2400);
  assert.equal(result.verifiedKg, 2.4);
});

// ---------------------------------------------------------------------------
// Fail-closed: snapshot berat hilang/0/negatif -> 0 g (bukan estimasi 0.4 kg)
// ---------------------------------------------------------------------------

test("computeImpactSummary: fail-closed jika weight_per_portion_snapshot hilang atau <= 0", () => {
  const orders = [
    { status: "Completed", quantity: 3, original_price: 20000, rescue_price: 10000 },
    { status: "Completed", quantity: 2, weight_per_portion_snapshot: 0, original_price: 20000, rescue_price: 10000 },
    { status: "Completed", quantity: 2, weight_per_portion_snapshot: -100, original_price: 20000, rescue_price: 10000 },
  ];
  const result = computeImpactSummary(orders);
  assert.equal(result.totalPortions, 7);
  assert.equal(result.totalGrams, 0);
  assert.equal(result.verifiedKg, 0);
});

// ---------------------------------------------------------------------------
// Field harga hilang/tidak valid
// ---------------------------------------------------------------------------

test("computeImpactSummary: original_price hilang → hemat 0 tapi porsi & berat tetap dihitung", () => {
  const orders = [
    { status: "Completed", quantity: 2, weight_per_portion_snapshot: 350, rescue_price: 12000 },
  ];
  const result = computeImpactSummary(orders);
  assert.equal(result.totalPortions, 2);
  assert.equal(result.totalSaved, 0);
  assert.equal(result.totalGrams, 700);
  assert.equal(result.verifiedKg, 0.7);
});

test("computeImpactSummary: rescue_price hilang → hemat 0", () => {
  const orders = [
    { status: "Completed", quantity: 1, weight_per_portion_snapshot: 300, original_price: 25000 },
  ];
  const result = computeImpactSummary(orders);
  assert.equal(result.totalPortions, 1);
  assert.equal(result.totalSaved, 0);
  assert.equal(result.totalGrams, 300);
});

test("computeImpactSummary: original_price <= rescue_price → hemat 0 (bukan negatif)", () => {
  const orders = [
    { status: "Completed", quantity: 1, weight_per_portion_snapshot: 250, original_price: 10000, rescue_price: 15000 },
  ];
  const result = computeImpactSummary(orders);
  assert.equal(result.totalPortions, 1);
  assert.equal(result.totalSaved, 0);
});

test("computeImpactSummary: harga NaN atau string → hemat 0", () => {
  const orders = [
    { status: "Completed", quantity: 1, weight_per_portion_snapshot: 250, original_price: "invalid", rescue_price: 5000 },
  ];
  const result = computeImpactSummary(orders);
  assert.equal(result.totalPortions, 1);
  assert.equal(result.totalSaved, 0);
});

// ---------------------------------------------------------------------------
// quantity edge cases
// ---------------------------------------------------------------------------

test("computeImpactSummary: quantity hilang atau <= 0 → diabaikan (fail-closed)", () => {
  const orders = [
    { status: "Completed", weight_per_portion_snapshot: 300, original_price: 20000, rescue_price: 10000 },
    { status: "Completed", quantity: 0, weight_per_portion_snapshot: 300, original_price: 20000, rescue_price: 10000 },
    { status: "Completed", quantity: -1, weight_per_portion_snapshot: 300, original_price: 20000, rescue_price: 10000 },
  ];
  const result = computeImpactSummary(orders);
  assert.equal(result.totalPortions, 0);
  assert.equal(result.totalSaved, 0);
  assert.equal(result.totalGrams, 0);
});

// ---------------------------------------------------------------------------
// Semua order Completed — skenario realistis
// ---------------------------------------------------------------------------

test("computeImpactSummary: semua Completed — akumulasi berat dan hemat benar", () => {
  const orders = [
    { status: "Completed", payment_status: "PAID", completed_at: "2026-07-21T13:15:00Z", quantity: 1, weight_per_portion_snapshot: 350, original_price: 25000, rescue_price: 12000 },
    { status: "Completed", payment_status: "PAID", completed_at: "2026-07-21T14:15:00Z", quantity: 2, weight_per_portion_snapshot: 250, original_price: 15000, rescue_price: 6000 },
    { status: "Completed", payment_status: "PAID", completed_at: "2026-07-21T15:15:00Z", quantity: 1, weight_per_portion_snapshot: 450, original_price: 45000, rescue_price: 22000 },
  ];
  const result = computeImpactSummary(orders);
  assert.equal(result.totalPortions, 4);
  assert.equal(result.totalSaved, 54000);
  assert.equal(result.totalGrams, 1300);
  assert.equal(result.verifiedKg, 1.3);
});
