package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/saveplate/backend/models"
	"github.com/saveplate/backend/services"
)

// GetRedistributionReport - GET /api/redistribution/report
// Mengembalikan ringkasan verified-gram (Wd, Wc, OBR, EC) dan audit trail per order
// terfilter otomatis berdasarkan peran JWT (ADMIN, UMKM, CUSTOMER).
func GetRedistributionReport(c *fiber.Ctx) error {
	userLocal := c.Locals("user")
	if userLocal == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(APIResponse{
			Success: false,
			Error:   &ErrorInfo{Code: "UNAUTHORIZED", Message: "Token tidak valid"},
		})
	}

	claims := userLocal.(*JWTClaims)
	roleUpper := strings.ToUpper(claims.Role)

	db := services.GetDB()
	query := db.Preload("Product").Order("orders.created_at desc")

	switch roleUpper {
	case models.RoleAdmin:
		if umkmIDStr := strings.TrimSpace(c.Query("umkm_id")); umkmIDStr != "" {
			umkmID, err := strconv.ParseUint(umkmIDStr, 10, 64)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
					Success: false,
					Error:   &ErrorInfo{Code: "INVALID_PARAM", Message: "umkm_id tidak valid"},
				})
			}
			query = query.Joins("JOIN products ON products.id = orders.product_id").
				Where("products.umkm_id = ?", uint(umkmID))
		}
	case models.RoleUmkm:
		query = query.Joins("JOIN products ON products.id = orders.product_id").
			Where("products.umkm_id = ?", claims.UserID)
	case models.RoleCustomer:
		query = query.Where("orders.customer_id = ?", claims.UserID)
	default:
		return c.Status(fiber.StatusForbidden).JSON(APIResponse{
			Success: false,
			Error:   &ErrorInfo{Code: "FORBIDDEN", Message: "Role tidak memiliki akses ke laporan redistribusi"},
		})
	}

	var orders []models.Order
	if err := query.Find(&orders).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Error:   &ErrorInfo{Code: "DB_ERROR", Message: err.Error()},
		})
	}

	report := services.BuildRedistributionReport(orders)
	return c.JSON(APIResponse{
		Success: true,
		Data:    report,
	})
}
