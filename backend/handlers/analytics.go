package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/saveplate/backend/models"
	"github.com/saveplate/backend/services"
)

// Analytics handlers - TODO: Update by Rifaidi after order/review model integration
// Stub untuk compatibility dengan existing routes

func GetUmkmAnalytics(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "Analytics endpoint - under development"})
}

func GetProductSalesAnalytics(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "Product sales analytics - under development"})
}

func GetSalesTrend(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "Sales trend - under development"})
}

func GetAnalyticsDashboard(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "Analytics dashboard - under development"})
}

func GetAnalyticsSales(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "Analytics sales - under development"})
}

func GetProductAnalytics(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "Product analytics - under development"})
}

func GetTopProducts(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "Top products - under development"})
}

func GetUmkmInsight(c *fiber.Ctx) error {
	db := services.GetDB()
	umkmID := c.Params("umkm_id")

	// Minimal queries for analytics
	var totalRevenue float64
	var totalUnits int64

	// Hitung hanya dari pesanan yang lolos verifikasi pickup (C1–C7: COMPLETED, PAID, completed_at terisi, berat & qty > 0)
	verifiedWhere := `products.umkm_id = ? AND orders.status = ? AND orders.payment_status = ?
		AND orders.completed_at IS NOT NULL AND orders.weight_per_portion_snapshot > 0 AND orders.quantity > 0`

	db.Model(&models.Order{}).
		Joins("JOIN products ON products.id = orders.product_id").
		Where(verifiedWhere, umkmID, models.OrderCompleted, models.PaymentPaid).
		Select("COALESCE(SUM(orders.total_price), 0)").Scan(&totalRevenue)

	db.Model(&models.Order{}).
		Joins("JOIN products ON products.id = orders.product_id").
		Where(verifiedWhere, umkmID, models.OrderCompleted, models.PaymentPaid).
		Select("COALESCE(SUM(orders.quantity), 0)").Scan(&totalUnits)

	type TopProduct struct {
		ProductID   uint    `json:"product_id"`
		Name        string  `json:"name"`
		Category    string  `json:"category"`
		UnitsSold   int     `json:"units_sold"`
		Revenue     float64 `json:"revenue"`
		OrdersCount int     `json:"orders_count"`
	}

	var topProducts []TopProduct
	db.Model(&models.Order{}).
		Joins("JOIN products ON products.id = orders.product_id").
		Where(verifiedWhere, umkmID, models.OrderCompleted, models.PaymentPaid).
		Select("products.id as product_id, products.name, products.category, SUM(orders.quantity) as units_sold, SUM(orders.total_price) as revenue, COUNT(orders.id) as orders_count").
		Group("products.id, products.name, products.category").
		Order("units_sold DESC").
		Limit(3).
		Scan(&topProducts)

	return c.JSON(fiber.Map{
		"umkm_id":       umkmID,
		"avg_rating":    4.8, // stub
		"review_count":  127, // stub
		"total_revenue": totalRevenue,
		"total_units":   totalUnits,
		"top_products":  topProducts,
		"keyword_safety": fiber.Map{
			"badge": "Aman",
			"top_positive": []fiber.Map{
				{"keyword": "enak", "count": 15},
			},
			"top_negative": []fiber.Map{},
		},
	})
}

func GetListingMetrics(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "Listing metrics - under development"})
}

