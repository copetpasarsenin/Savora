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

import (
	"time"

	"github.com/saveplate/backend/models"
)

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

// OrderToInput mengonversi models.Order menjadi OrderInput untuk EligibilityGate.
func OrderToInput(o models.Order) OrderInput {
	return OrderInput{
		ID:                       o.ID,
		WeightPerPortionSnapshot: o.WeightPerPortionSnapshot,
		Quantity:                 o.Quantity,
		Status:                   o.Status,
		PaymentStatus:            o.PaymentStatus,
		PickupCode:               o.PickupCode,
		CompletedAt:              o.CompletedAt,
		PickupDeadline:           o.PickupDeadline,
	}
}

// RedistributionAuditItem menyimpan jejak audit kelayakan per order item.
type RedistributionAuditItem struct {
	OrderID                  uint       `json:"order_id"`
	ProductID                uint       `json:"product_id"`
	ProductName              string     `json:"product_name"`
	UMKMID                   uint       `json:"umkm_id"`
	CustomerID               uint       `json:"customer_id"`
	WeightPerPortionSnapshot float64    `json:"weight_per_portion_snapshot"`
	Quantity                 int        `json:"quantity"`
	Status                   string     `json:"status"`
	PaymentStatus            string     `json:"payment_status"`
	CompletedAt              *time.Time `json:"completed_at,omitempty"`
	PickupDeadline           *time.Time `json:"pickup_deadline,omitempty"`
	Eligible                 bool       `json:"eligible"`
	FailedConditions         []string   `json:"failed_conditions"`
	WeightContributionGrams  float64    `json:"weight_contribution_grams"`
}

// RedistributionReport merangkum metrik verified-gram (Wd, Wc, OBR, EC) dan audit per order.
type RedistributionReport struct {
	VerifiedWeightGrams float64                   `json:"verified_weight_grams"`
	VerifiedWeightKg    float64                   `json:"verified_weight_kg"`
	ClaimedWeightGrams  float64                   `json:"claimed_weight_grams"`
	ClaimedWeightKg     float64                   `json:"claimed_weight_kg"`
	PhantomRescueGrams  float64                   `json:"phantom_rescue_grams"`
	OBR                 *float64                  `json:"obr"`
	OBRPercent          *float64                  `json:"obr_percent"`
	EC                  *float64                  `json:"ec"`
	EligibleCount       int                       `json:"eligible_count"`
	ClaimedCount        int                       `json:"claimed_count"`
	TotalCount          int                       `json:"total_count"`
	EligiblePortions    int                       `json:"eligible_portions"`
	Items               []RedistributionAuditItem `json:"items"`
}

// BuildRedistributionReport menghitung Wd, Wc, OBR, EC, dan audit trail per order
// menggunakan EligibilityGate (Algoritma 1) sebagai satu sumber kebenaran.
func BuildRedistributionReport(orders []models.Order) RedistributionReport {
	report := RedistributionReport{
		TotalCount: len(orders),
		Items:      make([]RedistributionAuditItem, 0, len(orders)),
	}

	for _, o := range orders {
		in := OrderToInput(o)
		det := EligibilityGate(in)

		failed := det.FailedConditions
		if failed == nil {
			failed = []string{}
		}

		report.Items = append(report.Items, RedistributionAuditItem{
			OrderID:                  o.ID,
			ProductID:                o.ProductID,
			ProductName:              o.Product.Name,
			UMKMID:                   o.Product.UmkmID,
			CustomerID:               o.CustomerID,
			WeightPerPortionSnapshot: o.WeightPerPortionSnapshot,
			Quantity:                 o.Quantity,
			Status:                   o.Status,
			PaymentStatus:            o.PaymentStatus,
			CompletedAt:              o.CompletedAt,
			PickupDeadline:           o.PickupDeadline,
			Eligible:                 det.Eligible,
			FailedConditions:         failed,
			WeightContributionGrams:  det.WeightContributionGrams,
		})

		// C = {i ∈ I | PaymentStatus = PAID ∧ wi > 0 ∧ qi > 0}
		if o.PaymentStatus == models.PaymentPaid && o.WeightPerPortionSnapshot > 0 && o.Quantity > 0 {
			report.ClaimedWeightGrams += o.WeightPerPortionSnapshot * float64(o.Quantity)
			report.ClaimedCount++
		}

		if det.Eligible {
			report.VerifiedWeightGrams += det.WeightContributionGrams
			report.EligibleCount++
			report.EligiblePortions += o.Quantity
		}
	}

	report.VerifiedWeightKg = report.VerifiedWeightGrams / 1000.0
	report.ClaimedWeightKg = report.ClaimedWeightGrams / 1000.0
	if report.ClaimedWeightGrams > report.VerifiedWeightGrams {
		report.PhantomRescueGrams = report.ClaimedWeightGrams - report.VerifiedWeightGrams
	}

	if report.VerifiedWeightGrams > 0 {
		obr := (report.ClaimedWeightGrams - report.VerifiedWeightGrams) / report.VerifiedWeightGrams
		obrPct := obr * 100.0
		report.OBR = &obr
		report.OBRPercent = &obrPct
	}
	if report.ClaimedCount > 0 {
		ec := float64(report.EligibleCount) / float64(report.ClaimedCount)
		report.EC = &ec
	}

	return report
}
