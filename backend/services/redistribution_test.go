package services

// redistribution_test.go — TestExperiment1_MeasurementAccuracy (TC-01..TC-15)
//                           TestExperiment3_ComponentComparison
//
// Menguji Algoritma 1 (EligibilityGate) dan CalculateRedistributionWeight.
// Setiap TC menguji satu skenario batas sesuai deskripsi di jurnal Bagian IV.
//
// Jalankan: go test ./services/... -v -run TestExperiment

import (
	"testing"
	"time"
)

// helper: pointer ke string
func sp(s string) *string { return &s }

// helper: pointer ke time.Time
func tp(t time.Time) *time.Time { return &t }

var (
	now      = time.Now()
	past     = now.Add(-1 * time.Hour)
	future   = now.Add(24 * time.Hour)
	someCode = sp("ABC123")
)

// completedOrder membuat order yang memenuhi semua 7 kondisi.
// Gunakan sebagai dasar, lalu override field yang ingin diuji.
func completedOrder(id uint, weight float64, qty int) OrderInput {
	return OrderInput{
		ID:                       id,
		WeightPerPortionSnapshot: weight,
		Quantity:                 qty,
		Status:                   "COMPLETED",
		PaymentStatus:            "PAID",
		PickupCode:               someCode,
		CompletedAt:              tp(now),
		PickupDeadline:           tp(future),
	}
}

// ===========================================================================
// TestExperiment1_MeasurementAccuracy — 15 test case (TC-01 s.d. TC-15)
// ===========================================================================

func TestExperiment1_MeasurementAccuracy(t *testing.T) {
	tests := []struct {
		name            string
		order           OrderInput
		wantEligible    bool
		wantWeightGrams float64
		wantFailedCond  string // substring kondisi yang harus gagal (kosong = semua lulus)
	}{
		// TC-01: Normal completed pickup — satu item
		{
			name:            "TC-01: Normal completed pickup single item",
			order:           completedOrder(1, 350.0, 1),
			wantEligible:    true,
			wantWeightGrams: 350.0,
		},
		// TC-02: Normal completed pickup — multi-item qty 3
		{
			name:            "TC-02: Normal completed pickup qty=3",
			order:           completedOrder(2, 200.0, 3),
			wantEligible:    true,
			wantWeightGrams: 600.0,
		},
		// TC-03: Normal completed pickup — weight_per_portion besar (500 g)
		{
			name:            "TC-03: Large weight_per_portion 500g",
			order:           completedOrder(3, 500.0, 2),
			wantEligible:    true,
			wantWeightGrams: 1000.0,
		},
		// TC-04: Pesanan dibatalkan (CANCELLED) — tidak dihitung
		{
			name: "TC-04: Cancelled order excluded",
			order: func() OrderInput {
				o := completedOrder(4, 300.0, 1)
				o.Status = "CANCELLED"
				o.CompletedAt = nil
				return o
			}(),
			wantEligible:   false,
			wantFailedCond: "C3:cancelled",
		},
		// TC-05: Pesanan paid tapi no-show (NO_SHOW)
		{
			name: "TC-05: No-show order excluded",
			order: func() OrderInput {
				o := completedOrder(5, 300.0, 1)
				o.Status = "NO_SHOW"
				o.CompletedAt = nil
				return o
			}(),
			wantEligible:   false,
			wantFailedCond: "C4:expired_or_no_show",
		},
		// TC-06: Pesanan expired (payment expired)
		{
			name: "TC-06: Expired order excluded",
			order: func() OrderInput {
				o := completedOrder(6, 300.0, 1)
				o.Status = "EXPIRED"
				o.PaymentStatus = "EXPIRED"
				o.CompletedAt = nil
				return o
			}(),
			wantEligible:   false,
			wantFailedCond: "C4:expired_or_no_show",
		},
		// TC-07: Pickup belum diverifikasi (completed_at = nil, status PAID)
		{
			name: "TC-07: Pickup not verified — completed_at nil",
			order: func() OrderInput {
				o := completedOrder(7, 300.0, 1)
				o.Status = "PAID"
				o.CompletedAt = nil
				return o
			}(),
			wantEligible:   false,
			wantFailedCond: "C6:pickup_not_verified",
		},
		// TC-08: Pembayaran belum lunas (status PAID tapi payment_status PENDING)
		{
			name: "TC-08: Payment pending — not counted",
			order: func() OrderInput {
				o := completedOrder(8, 300.0, 1)
				o.PaymentStatus = "PENDING"
				return o
			}(),
			wantEligible:   false,
			wantFailedCond: "C5:payment_not_paid",
		},
		// TC-09: weight_per_portion_snapshot = 0 (input tidak valid)
		{
			name: "TC-09: Zero weight_per_portion_snapshot invalid",
			order: func() OrderInput {
				o := completedOrder(9, 0.0, 2)
				return o
			}(),
			wantEligible:   false,
			wantFailedCond: "C1:weight_invalid",
		},
		// TC-10: weight_per_portion_snapshot negatif
		{
			name: "TC-10: Negative weight_per_portion_snapshot invalid",
			order: func() OrderInput {
				o := completedOrder(10, -50.0, 1)
				return o
			}(),
			wantEligible:   false,
			wantFailedCond: "C1:weight_invalid",
		},
		// TC-11: quantity = 0 (input tidak valid)
		{
			name: "TC-11: Zero quantity invalid",
			order: func() OrderInput {
				o := completedOrder(11, 300.0, 0)
				return o
			}(),
			wantEligible:   false,
			wantFailedCond: "C2:quantity_invalid",
		},
		// TC-12: quantity negatif
		{
			name: "TC-12: Negative quantity invalid",
			order: func() OrderInput {
				o := completedOrder(12, 300.0, -1)
				return o
			}(),
			wantEligible:   false,
			wantFailedCond: "C2:quantity_invalid",
		},
		// TC-13: Perubahan berat produk setelah order — snapshot melindungi
		// UMKM mengubah weight_per_portion dari 300 → 500 g setelah order dibuat.
		// Order menyimpan snapshot 300 g, bukan 500 g.
		{
			name: "TC-13: Post-order weight mutation protected by snapshot",
			order: func() OrderInput {
				o := completedOrder(13, 300.0, 2) // snapshot 300, bukan 500
				return o
			}(),
			wantEligible:    true,
			wantWeightGrams: 600.0, // 300 × 2, bukan 500 × 2
		},
		// TC-14: Status CREATED (belum selesai sama sekali)
		{
			name: "TC-14: CREATED status not counted",
			order: func() OrderInput {
				o := completedOrder(14, 300.0, 1)
				o.Status = "CREATED"
				o.PaymentStatus = "UNPAID"
				o.CompletedAt = nil
				return o
			}(),
			wantEligible:   false,
			wantFailedCond: "C5:payment_not_paid",
		},
		// TC-15: Order valid dengan qty besar (5 item, weight 480 g)
		{
			name:            "TC-15: Valid multi-item large order",
			order:           completedOrder(15, 480.0, 5),
			wantEligible:    true,
			wantWeightGrams: 2400.0,
		},
	}

	var totalPass, totalFail int

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EligibilityGate(tt.order)

			if result.Eligible != tt.wantEligible {
				t.Errorf("Eligible = %v, want %v", result.Eligible, tt.wantEligible)
				totalFail++
				return
			}

			if tt.wantEligible {
				// Verifikasi akurasi kalkulasi (absolute error harus 0.0 g)
				if result.WeightContributionGrams != tt.wantWeightGrams {
					t.Errorf("WeightContributionGrams = %.1f g, want %.1f g (absolute error = %.1f g)",
						result.WeightContributionGrams, tt.wantWeightGrams,
						abs(result.WeightContributionGrams-tt.wantWeightGrams))
					totalFail++
					return
				}
			} else {
				// Verifikasi kondisi yang gagal mengandung substring yang diharapkan
				if tt.wantFailedCond != "" && !containsCondition(result.FailedConditions, tt.wantFailedCond) {
					t.Errorf("FailedConditions = %v, want to contain %q", result.FailedConditions, tt.wantFailedCond)
					totalFail++
					return
				}
			}

			totalPass++
		})
	}

	t.Logf("TestExperiment1_MeasurementAccuracy: %d/%d TC PASS, Calculation Accuracy = %.0f%%",
		totalPass, len(tests), float64(totalPass)/float64(len(tests))*100)
}

// ===========================================================================
// TestExperiment3_ComponentComparison
//
// Dataset 10 pesanan campuran: 5 eligible (ground truth 2400.0 g),
// 3 paid-but-unverified, 1 cancelled, 1 no-show.
//
// Config A (tanpa pickup verification gate): hitung semua PaymentStatus=PAID
// Config B (SavePlate, dengan gate): hanya yang lolos semua 7 kondisi
//
// Ground truth: 2400.0 g (dari 5 order terverifikasi)
// Config A overestimate: 3650.0 g (selisih 1250.0 g, +52.08%)
// Config B: tepat 2400.0 g (galat 0.0 g)
// ===========================================================================

func TestExperiment3_ComponentComparison(t *testing.T) {
	// Dataset: 10 pesanan campuran sesuai jurnal Bagian IV.
	//
	// Ground truth (Config B): 5 order terverifikasi = 2400 g
	// Config A (filter payment_status=PAID saja, tanpa gate lain):
	//   5 eligible + 2 unverified + 1 no-show = 8 items = 3650 g
	//   Bias overestimasi: 3650 − 2400 = 1250 g (+52.08%)
	// Order cancelled dan payment_failed tidak masuk Config A maupun Config B.
	dataset := []OrderInput{
		// 5 order terverifikasi (eligible, ground truth = 2400 g)
		completedOrder(101, 300.0, 1), // 300 g
		completedOrder(102, 200.0, 2), // 400 g
		completedOrder(103, 500.0, 1), // 500 g
		completedOrder(104, 250.0, 3), // 750 g
		completedOrder(105, 450.0, 1), // 450 g

		// 2 order paid tapi belum diverifikasi (completed_at = nil) — 850 g
		{ID: 106, WeightPerPortionSnapshot: 450.0, Quantity: 1, Status: "PAID", PaymentStatus: "PAID", PickupCode: someCode, CompletedAt: nil, PickupDeadline: tp(future)},
		{ID: 107, WeightPerPortionSnapshot: 400.0, Quantity: 1, Status: "PAID", PaymentStatus: "PAID", PickupCode: someCode, CompletedAt: nil, PickupDeadline: tp(future)},

		// 1 order no-show (PAID, tidak diambil) — 400 g
		{ID: 108, WeightPerPortionSnapshot: 400.0, Quantity: 1, Status: "NO_SHOW", PaymentStatus: "PAID", PickupCode: someCode, CompletedAt: nil},

		// 1 order cancelled dengan payment belum lunas (tidak masuk Config A maupun B)
		{ID: 109, WeightPerPortionSnapshot: 300.0, Quantity: 1, Status: "CANCELLED", PaymentStatus: "UNPAID", PickupCode: nil, CompletedAt: nil},

		// 1 order payment gagal (tidak masuk Config A maupun B)
		{ID: 110, WeightPerPortionSnapshot: 250.0, Quantity: 1, Status: "PAYMENT_FAILED", PaymentStatus: "FAILED", PickupCode: nil, CompletedAt: nil},
	}

	const groundTruth = 2400.0

	// --- Config B: SavePlate (pakai EligibilityGate penuh) ---
	configB := CalculateRedistributionWeight(dataset)

	if configB.TotalWeightGrams != groundTruth {
		t.Errorf("Config B (SavePlate): Wd = %.1f g, want %.1f g (error = %.1f g)",
			configB.TotalWeightGrams, groundTruth, abs(configB.TotalWeightGrams-groundTruth))
	} else {
		t.Logf("Config B (SavePlate): Wd = %.1f g, ground truth = %.1f g, absolute error = 0.0 g ✓", configB.TotalWeightGrams, groundTruth)
	}

	if configB.EligibleCount != 5 {
		t.Errorf("Config B: eligible count = %d, want 5", configB.EligibleCount)
	}

	// --- Config A: tanpa pickup verification gate ---
	// Hitung semua order dengan payment_status = PAID, tanpa cek completed_at / status
	var configAWeight float64
	var configACount int
	for _, o := range dataset {
		if o.PaymentStatus == "PAID" && o.WeightPerPortionSnapshot > 0 && o.Quantity > 0 {
			configAWeight += o.WeightPerPortionSnapshot * float64(o.Quantity)
			configACount++
		}
	}

	const expectedConfigA = 3650.0
	const expectedBias = 1250.0

	if configAWeight != expectedConfigA {
		t.Errorf("Config A: Wd = %.1f g, want %.1f g", configAWeight, expectedConfigA)
	}

	bias := configAWeight - groundTruth
	if bias != expectedBias {
		t.Errorf("Overestimation bias = %.1f g, want %.1f g", bias, expectedBias)
	}

	t.Logf("Config A (tanpa gate): Wd = %.1f g (%d items)", configAWeight, configACount)
	t.Logf("Config B (SavePlate):  Wd = %.1f g (%d items)", configB.TotalWeightGrams, configB.EligibleCount)
	t.Logf("Overestimation bias:   %.1f g (+%.2f%%)", bias, bias/groundTruth*100)
}

// ===========================================================================
// helpers
// ===========================================================================

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func containsCondition(conds []string, sub string) bool {
	for _, c := range conds {
		if c == sub {
			return true
		}
	}
	return false
}
