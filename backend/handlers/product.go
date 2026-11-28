package handlers

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/saveplate/backend/models"
	"github.com/saveplate/backend/services"
)

func validateProductInput(name string, stock int, weightPerPortion float64) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("Nama produk wajib diisi")
	}
	if stock < 0 {
		return errors.New("Stok produk tidak boleh negatif")
	}
	if weightPerPortion <= 0 {
		return errors.New("Berat per porsi wajib diisi dan harus lebih dari 0 gram")
	}
	return nil
}

func canManageProduct(claims *JWTClaims, productUmkmID uint) bool {
	if claims == nil {
		return false
	}
	roleUpper := strings.ToUpper(claims.Role)
	if roleUpper == models.RoleAdmin {
		return true
	}
	return roleUpper == models.RoleUmkm && productUmkmID == claims.UserID
}

// GetProductsByUMKM returns all products for a UMKM excluding Expired/Limbah status
// (for active product management - expired products are archived automatically)
func GetProductsByUMKM(c *fiber.Ctx) error {
	umkmID := c.Params("umkm_id")
	var products []models.Product

	// Exclude Kadaluwarsa (expired) and Limbah (waste) products from main product list
	// These products are archived and should only appear in history/waste logs
	// Using Indonesian status values to match database data
	if err := services.GetDB().
		Where("umkm_id = ? AND status NOT IN (?)", umkmID, []string{"Kadaluwarsa", "Limbah"}).
		Order("created_at desc").
		Find(&products).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(products)
}

// GetActiveMarketplaceProducts
func GetActiveMarketplaceProducts(c *fiber.Ctx) error {
	var products []models.Product

	if err := services.GetDB().Where("status = ?", models.ProductStatusAktif).Order("created_at desc").Find(&products).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(products)
}

// CreateProduct
func CreateProduct(c *fiber.Ctx) error {
	userLocal := c.Locals("user")
	if userLocal == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized: token tidak valid"})
	}
	claims := userLocal.(*JWTClaims)
	roleUpper := strings.ToUpper(claims.Role)
	if roleUpper != models.RoleUmkm && roleUpper != models.RoleAdmin {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden: hanya UMKM atau Admin yang dapat membuat produk"})
	}

	product := new(models.Product)

	if err := c.BodyParser(product); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}

	if err := validateProductInput(product.Name, product.Stock, product.WeightPerPortion); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}

	// Paksa umkm_id dari JWT untuk role UMKM agar tidak bisa spoof milik UMKM lain
	if roleUpper == models.RoleUmkm {
		product.UmkmID = claims.UserID
	}

	if strings.TrimSpace(product.Status) == "" {
		product.Status = models.ProductStatusAktif
	}

	product.FoodTrustStatus = CalculateFoodTrustStatus(product)

	// Set status: "Limbah" jika tidak layak, "Aktif" untuk normal products
	if product.FoodTrustStatus == "Tidak Layak Konsumsi" {
		product.Status = models.ProductStatusLimbah
	}

	if err := services.GetDB().Create(&product).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if product.Status == models.ProductStatusLimbah {
		estimatedWeight := float64(product.Stock) * product.WeightPerPortion / 1000.0
		wasteLog := models.WasteLog{
			UmkmID:          product.UmkmID,
			FoodName:        product.Name,
			Category:        product.Category,
			EstimatedWeight: estimatedWeight,
			Reason:          "Otomatis oleh sistem: Tidak Layak Konsumsi (Berdasarkan Food Trust Index)",
			PhotoURL:        product.PhotoURL,
		}
		services.GetDB().Create(&wasteLog)

		notification := models.Notification{
			UserID:   product.UmkmID,
			UserRole: "umkm",
			Title:    "Produk Ditolak Sistem",
			Message:  "Produk " + product.Name + " terdeteksi Tidak Layak Konsumsi dan otomatis dialihkan ke Waste Log.",
		}
		services.GetDB().Create(&notification)
	}

	return c.Status(fiber.StatusCreated).JSON(product)
}

// UpdateProduct
func UpdateProduct(c *fiber.Ctx) error {
	userLocal := c.Locals("user")
	if userLocal == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized: token tidak valid"})
	}
	claims := userLocal.(*JWTClaims)

	id := c.Params("id")
	var product models.Product

	if err := services.GetDB().First(&product, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	if !canManageProduct(claims, product.UmkmID) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden: Anda tidak berhak mengubah produk ini"})
	}

	var updateData models.Product
	if err := c.BodyParser(&updateData); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot parse JSON"})
	}

	// Validasi berat: wajib > 0 bila dikirim, atau produk lama belum punya berat valid
	var rawBody map[string]json.RawMessage
	_ = json.Unmarshal(c.Body(), &rawBody)
	if _, sent := rawBody["weight_per_portion"]; sent {
		if updateData.WeightPerPortion <= 0 {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "Berat per porsi wajib diisi dan harus lebih dari 0 gram"})
		}
	} else if product.WeightPerPortion <= 0 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "Berat per porsi wajib diisi dan harus lebih dari 0 gram"})
	}
	if updateData.Stock < 0 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": "Stok produk tidak boleh negatif"})
	}

	// Jaga kepemilikan UMKM tidak bisa diubah lewat payload
	updateData.UmkmID = product.UmkmID
	updateData.FoodTrustStatus = CalculateFoodTrustStatus(&updateData)

	if updateData.FoodTrustStatus == "Tidak Layak Konsumsi" && product.Status != models.ProductStatusLimbah {
		updateData.Status = models.ProductStatusLimbah
	}

	prevStatus := product.Status
	services.GetDB().Model(&product).Updates(updateData)

	if updateData.Status == models.ProductStatusLimbah && prevStatus != models.ProductStatusLimbah {
		estimatedWeight := float64(product.Stock) * product.WeightPerPortion / 1000.0
		wasteLog := models.WasteLog{
			UmkmID:          product.UmkmID,
			FoodName:        product.Name,
			Category:        product.Category,
			EstimatedWeight: estimatedWeight,
			Reason:          "Otomatis oleh sistem: Update produk menjadi Tidak Layak Konsumsi",
			PhotoURL:        product.PhotoURL,
		}
		services.GetDB().Create(&wasteLog)

		notification := models.Notification{
			UserID:   product.UmkmID,
			UserRole: "umkm",
			Title:    "Produk Menjadi Limbah",
			Message:  "Produk " + product.Name + " telah diubah menjadi Tidak Layak Konsumsi dan masuk ke Waste Log.",
		}
		services.GetDB().Create(&notification)
	}

	return c.JSON(product)
}

// DeleteProduct
func DeleteProduct(c *fiber.Ctx) error {
	userLocal := c.Locals("user")
	if userLocal == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized: token tidak valid"})
	}
	claims := userLocal.(*JWTClaims)

	id := c.Params("id")
	var product models.Product

	if err := services.GetDB().First(&product, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Product not found"})
	}

	if !canManageProduct(claims, product.UmkmID) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Forbidden: Anda tidak berhak menghapus produk ini"})
	}

	// Delete product - may fail if referenced by orders (foreign key constraint)
	if err := services.GetDB().Delete(&product).Error; err != nil {
		// Check if it's a foreign key constraint violation
		if strings.Contains(err.Error(), "foreign key constraint") || strings.Contains(err.Error(), "violates foreign key") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Produk tidak dapat dihapus karena sudah memiliki order. Silakan ubah status menjadi 'Habis' atau 'Draft' sebagai gantinya.",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "Product successfully deleted"})
}

// CalculateFoodTrustStatus menghitung FTI berdasarkan PRD 12.4
func CalculateFoodTrustStatus(p *models.Product) string {
	if p.ProductionTime == nil || p.ExpiresAt == nil {
		// Fallback jika tidak ada data waktu
		return p.FoodTrustStatus
	}

	now := time.Now()
	totalLifespan := p.ExpiresAt.Sub(*p.ProductionTime)
	remainingTime := p.ExpiresAt.Sub(now)

	var f float64 = 0
	if totalLifespan > 0 {
		f = float64(remainingTime) / float64(totalLifespan)
	}

	if f <= 0 || p.PackagingCondition == "Rusak" {
		return "Tidak Layak Konsumsi"
	}
	if f < 0.15 || p.StorageMethod == "Tidak Sesuai" {
		return "Tidak Disarankan Dijual"
	}
	if f < 0.40 {
		return "Segera Dijual"
	}
	if f < 0.75 || p.PackagingCondition == "Standar" {
		return "Layak Dijual"
	}
	if f >= 0.75 && p.PackagingCondition == "Baik" && p.StorageMethod == "Sesuai" {
		return "Fresh"
	}

	return "Layak Dijual" // Fallback
}
