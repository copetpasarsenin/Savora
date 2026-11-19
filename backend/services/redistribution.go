package services

// redistribution.go — Mekanisme pengukuran berat redistribusi SavePlate
//
// Mengimplementasikan Algoritma 1 (tujuh kondisi eligibility) dan formula
// Wd = Σ_{i∈E}(wi × qi) sebagaimana dirancang dalam jurnal.
//
// Tujuh kondisi eligibility (C1–C7):
//   C1: weight_per_portion_snapshot > 0  (berat valid)
//   C2: quantity > 0                      (kuantitas valid)
//   C3: status != CANCELLED               (tidak dibatalkan)
//   C4: status ∉ {EXPIRED, NO_SHOW}       (tidak kedaluwarsa / no-show)
//   C5: payment_status == PAID            (pembayaran lunas)
//   C6: pickup_code terverifikasi (completed_at != nil) (pengambilan terkonfirmasi)
//   C7: status == COMPLETED               (pesanan selesai)
//
// Referensi model: backend/models/order.go
// Diuji di: backend/services/redistribution_test.go

import "time"

// OrderInput adalah representasi order yang sudah didenormalisasi untuk
// keperluan pengujian dan kalkulasi — tidak bergantung pada koneksi DB.
type OrderInput struct {
	ID                       uint
	WeightPerPortionSnapshot float64    // wi — disnap saat order dibuat
	Quantity                 int        // qi
	Status                   string     // CREATED, PAID, COMPLETED, CANCELLED, ...
	PaymentStatus            string     // UNPAID, PENDING, PAID, FAILED, EXPIRED
	PickupCode               *string    // nil = belum di-generate
	CompletedAt              *time.Time // nil = belum selesai
	PickupDeadline           *time.Time // nil = tidak ada batas
}

// EligibilityResult menyimpan hasil evaluasi eligibility satu order.
type EligibilityResult struct {
	OrderID  uint
	Eligible bool
	// Kondisi yang gagal (kosong bila eligible).
	FailedConditions []string
	// Kontribusi berat bila eligible: wi × qi (gram).
	WeightContributionGrams float64
}

// EligibilityGate mengevaluasi tujuh kondisi untuk satu order.
// Mengembalikan EligibilityResult dengan detail kondisi yang gagal.
func EligibilityGate(o OrderInput) EligibilityResult {
	var failed []string

	// C1 — berat valid
	if o.WeightPerPortionSnapshot <= 0 {
		failed = append(failed, "C1:weight_invalid")
	}

	// C2 — kuantitas valid
	if o.Quantity <= 0 {
		failed = append(failed, "C2:quantity_invalid")
	}

	// C3 — tidak dibatalkan
	if o.Status == "CANCELLED" {
		failed = append(failed, "C3:cancelled")
	}

	// C4 — tidak expired / no-show
	if o.Status == "EXPIRED" || o.Status == "NO_SHOW" {
		failed = append(failed, "C4:expired_or_no_show")
	}

	// C5 — pembayaran lunas
	if o.PaymentStatus != "PAID" {
		failed = append(failed, "C5:payment_not_paid")
	}

	// C6 — pickup_code terverifikasi (completed_at tidak nil)
	if o.CompletedAt == nil {
		failed = append(failed, "C6:pickup_not_verified")
	}

	// C7 — status COMPLETED
	if o.Status != "COMPLETED" {
		failed = append(failed, "C7:status_not_completed")
	}

	eligible := len(failed) == 0
	var contribution float64
	if eligible {
		contribution = o.WeightPerPortionSnapshot * float64(o.Quantity)
	}

	return EligibilityResult{
		OrderID:                 o.ID,
		Eligible:                eligible,
		FailedConditions:        failed,
		WeightContributionGrams: contribution,
	}
}

// RedistributionResult adalah hasil kalkulasi Wd untuk sekumpulan order.
type RedistributionResult struct {
	// Wd = Σ_{i∈E}(wi × qi) dalam gram
	TotalWeightGrams float64
	// Jumlah order yang lolos eligibility gate
	EligibleCount int
	// Jumlah order yang dievaluasi
	TotalCount int
	// Detail per order
	Details []EligibilityResult
}

// CalculateRedistributionWeight menghitung Wd dari sekumpulan OrderInput.
// Ini adalah fungsi utama yang diuji oleh TestExperiment1_MeasurementAccuracy
// dan TestExperiment3_ComponentComparison.
func CalculateRedistributionWeight(orders []OrderInput) RedistributionResult {
	result := RedistributionResult{
		TotalCount: len(orders),
		Details:    make([]EligibilityResult, 0, len(orders)),
	}

	for _, o := range orders {
		det := EligibilityGate(o)
		result.Details = append(result.Details, det)
		if det.Eligible {
			result.TotalWeightGrams += det.WeightContributionGrams
			result.EligibleCount++
		}
	}

	return result
}
