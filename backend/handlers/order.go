package handlers

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/saveplate/backend/models"
	"github.com/saveplate/backend/services"
)

// canAccessOrder memeriksa hak akses pengguna atas suatu order.
// allowCustomer=true untuk read detail; allowCustomer=false untuk aksi UMKM/Admin.
func canAccessOrder(claims *JWTClaims, order models.Order, allowCustomer bool) bool {
	if claims == nil {
		return false
	}
	switch strings.ToUpper(claims.Role) {
	case models.RoleAdmin:
		return true
	case models.RoleUmkm:
		return order.Product.UmkmID == claims.UserID
	case models.RoleCustomer:
		return allowCustomer && order.CustomerID == claims.UserID
	default:
		return false
	}
}

// maskOrderForRole mengosongkan pickup_code untuk role UMKM agar kode hanya diketahui pelanggan.
func maskOrderForRole(role string, order *models.Order) {
	if order == nil {
		return
	}
	if strings.ToUpper(role) == models.RoleUmkm {
		order.PickupCode = nil
	}
}

// validatePickupEligibility memeriksa seluruh syarat serah-terima sebelum menandai COMPLETED.
func validatePickupEligibility(order models.Order, inputCode string, now time.Time) error {
	if order.PaymentStatus != models.PaymentPaid {
		return errors.New("Pembayaran pesanan belum lunas")
	}
	if order.Status != models.OrderReadyForPickup {
		return errors.New("Order belum siap diambil atau sudah diproses")
	}
	if order.PickupDeadline != nil && now.After(*order.PickupDeadline) {
		return errors.New("Batas waktu pengambilan sudah lewat")
	}
	code := strings.ToUpper(strings.TrimSpace(inputCode))
	if code == "" || order.PickupCode == nil || strings.ToUpper(strings.TrimSpace(*order.PickupCode)) != code {
		return errors.New("Pickup code tidak valid")
	}
	return nil
}

type OrderHandler struct {
	xenditService *services.XenditService
}

func NewOrderHandler(xendit *services.XenditService) *OrderHandler {
	return &OrderHandler{
		xenditService: xendit,
	}
}

// CreateOrder - POST /orders (Customer checkout)
func (h *OrderHandler) CreateOrder(c *fiber.Ctx) error {
	db := services.GetDB()

	// Extract customerID dari JWT middleware
	userLocal := c.Locals("user")
	if userLocal == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "Unauthorized: token tidak valid",
		})
	}

	claims := userLocal.(*JWTClaims)
	customerID := claims.UserID

	var req services.CreateOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body: " + err.Error(),
		})
	}

	// Validasi manual (Fiber tidak support tag binding: seperti Gin)
	if req.ProductID == 0 {
		return c.Status(422).JSON(fiber.Map{
			"error": "Product ID wajib diisi",
		})
	}
	if req.Quantity < 1 {
		return c.Status(422).JSON(fiber.Map{
			"error": "Jumlah harus minimal 1",
		})
	}
	if req.BillingName == "" {
		return c.Status(422).JSON(fiber.Map{
			"error": "Nama pemesan wajib diisi",
		})
	}
	if req.BillingPhone == "" {
		return c.Status(422).JSON(fiber.Map{
			"error": "Nomor telepon wajib diisi",
		})
	}

	response, err := services.CreateOrder(db, customerID, req, h.xenditService)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(201).JSON(response)
}

// GetOrders - GET /orders (list order by role)
func (h *OrderHandler) GetOrders(c *fiber.Ctx) error {
	db := services.GetDB()

	// Extract user dari JWT (role-based filtering)
	userLocal := c.Locals("user")
	if userLocal == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "Unauthorized: token tidak valid",
		})
	}

	claims := userLocal.(*JWTClaims)
	userID := claims.UserID
	role := claims.Role

	var orders []models.Order
	query := db.Preload("Product").Preload("Customer").Order("created_at desc")

	// Role-based filtering (case-insensitive)
	roleUpper := strings.ToUpper(role)
	switch roleUpper {
	case "CUSTOMER":
		// Customer hanya lihat order miliknya sendiri
		query = query.Where("customer_id = ?", userID)
	case "ADMIN":
		// Admin lihat semua order (tidak ada filter)
	case "UMKM":
		query = query.Joins("JOIN products ON products.id = orders.product_id").Where("products.umkm_id = ?", userID)
	default:
		// Role lain tidak punya akses ke orders
		return c.Status(403).JSON(fiber.Map{
			"error": "Forbidden: role tidak memiliki akses ke orders",
		})
	}

	if err := query.Find(&orders).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if roleUpper == models.RoleUmkm {
		for i := range orders {
			maskOrderForRole(roleUpper, &orders[i])
		}
	}

	return c.JSON(orders)
}

// GetOrderDetail - GET /orders/:id
func (h *OrderHandler) GetOrderDetail(c *fiber.Ctx) error {
	userLocal := c.Locals("user")
	if userLocal == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "Unauthorized: token tidak valid",
		})
	}
	claims := userLocal.(*JWTClaims)

	db := services.GetDB()
	id := c.Params("id")

	var order models.Order
	if err := db.Preload("Product").Preload("Customer").Preload("Payment").First(&order, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Order tidak ditemukan",
		})
	}

	if !canAccessOrder(claims, order, true) {
		return c.Status(403).JSON(fiber.Map{
			"error": "Forbidden: Anda tidak memiliki akses ke pesanan ini",
		})
	}

	maskOrderForRole(claims.Role, &order)

	// DTO response untuk tidak bocorkan data sensitif customer
	type OrderDetailResponse struct {
		ID                       uint       `json:"id"`
		ProductID                uint       `json:"product_id"`
		CustomerID               uint       `json:"customer_id"`
		Quantity                 int        `json:"quantity"`
		WeightPerPortionSnapshot float64    `json:"weight_per_portion_snapshot"`
		Subtotal                 float64    `json:"subtotal"`
		ServiceFee               float64    `json:"service_fee"`
		TotalPrice               float64    `json:"total_price"`
		PaymentMethod            string     `json:"payment_method"`
		PaymentStatus            string     `json:"payment_status"`
		PickupCode               *string    `json:"pickup_code,omitempty"`
		ReservedUntil            time.Time  `json:"reserved_until"`
		PickupDeadline           *time.Time `json:"pickup_deadline,omitempty"`
		Status                   string     `json:"status"`
		CancelReason             string     `json:"cancel_reason,omitempty"`
		CreatedAt                time.Time  `json:"created_at"`
		PaidAt                   *time.Time `json:"paid_at,omitempty"`
		CompletedAt              *time.Time `json:"completed_at,omitempty"`

		// Relations (selective fields only)
		Product      models.Product  `json:"product,omitempty"`
		CustomerName string          `json:"customer_name"`
		Payment      *models.Payment `json:"payment,omitempty"`
	}

	response := OrderDetailResponse{
		ID:                       order.ID,
		ProductID:                order.ProductID,
		CustomerID:               order.CustomerID,
		Quantity:                 order.Quantity,
		WeightPerPortionSnapshot: order.WeightPerPortionSnapshot,
		Subtotal:                 order.Subtotal,
		ServiceFee:               order.ServiceFee,
		TotalPrice:               order.TotalPrice,
		PaymentMethod:            order.PaymentMethod,
		PaymentStatus:            order.PaymentStatus,
		PickupCode:               order.PickupCode,
		ReservedUntil:            order.ReservedUntil,
		PickupDeadline:           order.PickupDeadline,
		Status:                   order.Status,
		CancelReason:             order.CancelReason,
		CreatedAt:                order.CreatedAt,
		PaidAt:                   order.PaidAt,
		CompletedAt:              order.CompletedAt,
		Product:                  order.Product,
		CustomerName:             order.Customer.Name,
		Payment:                  order.Payment,
	}

	return c.JSON(response)
}

// UpdateOrderStatus - PATCH /orders/:id/status (UMKM/Admin)
func (h *OrderHandler) UpdateOrderStatus(c *fiber.Ctx) error {
	userLocal := c.Locals("user")
	if userLocal == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "Unauthorized: token tidak valid",
		})
	}
	claims := userLocal.(*JWTClaims)

	type Request struct {
		Status string `json:"status"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Tutup bypass: COMPLETED wajib lewat verifikasi kode pickup (ValidatePickupCode)
	if req.Status == models.OrderCompleted {
		return c.Status(400).JSON(fiber.Map{
			"error": "Gunakan verifikasi kode pickup untuk menyelesaikan pesanan",
		})
	}

	db := services.GetDB()
	id := c.Params("id")

	var order models.Order
	if err := db.Preload("Product").First(&order, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Order tidak ditemukan",
		})
	}

	if !canAccessOrder(claims, order, false) {
		return c.Status(403).JSON(fiber.Map{
			"error": "Forbidden: Anda tidak berhak mengubah status pesanan ini",
		})
	}

	// Validasi transisi khusus: Paid -> Ready for Pickup (REVISI #8)
	if req.Status == models.OrderReadyForPickup {
		if order.Status != models.OrderPaid {
			return c.Status(400).JSON(fiber.Map{
				"error": "Order harus berstatus Paid untuk diubah menjadi Ready for Pickup",
			})
		}

		res := db.Model(&models.Order{}).
			Where("id = ? AND status = ?", order.ID, models.OrderPaid).
			Updates(map[string]interface{}{
				"status": models.OrderReadyForPickup,
			})

		if res.Error != nil {
			return c.Status(500).JSON(fiber.Map{
				"error": res.Error.Error(),
			})
		}

		if res.RowsAffected == 0 {
			return c.Status(400).JSON(fiber.Map{
				"error": "Order sudah diproses atau status berubah",
			})
		}

		// Refresh order untuk response
		db.Preload("Product").First(&order, id)
		maskOrderForRole(claims.Role, &order)

		return c.JSON(fiber.Map{
			"message": "Order siap diambil",
			"order":   order,
		})
	}

	// Transisi lain (mis. No Show)
	if err := services.TransitionOrderStatus(db, order.ID, req.Status); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Status order berhasil diupdate",
	})
}

// ValidatePickupCode - POST /orders/:id/validate-pickup (UMKM/Admin)
func (h *OrderHandler) ValidatePickupCode(c *fiber.Ctx) error {
	userLocal := c.Locals("user")
	if userLocal == nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "Unauthorized: token tidak valid",
		})
	}
	claims := userLocal.(*JWTClaims)

	db := services.GetDB()
	id := c.Params("id")

	type Request struct {
		PickupCode string `json:"pickup_code"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	var order models.Order
	if err := db.Preload("Product").First(&order, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "Order tidak ditemukan",
		})
	}

	if !canAccessOrder(claims, order, false) {
		return c.Status(403).JSON(fiber.Map{
			"error": "Forbidden: Anda tidak berhak memverifikasi pickup pesanan ini",
		})
	}

	now := time.Now()
	if err := validatePickupEligibility(order, req.PickupCode, now); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Update atomik ke Completed (idempoten: panggilan kedua gagal karena status bukan READY_FOR_PICKUP)
	res := db.Model(&models.Order{}).
		Where("id = ? AND status = ? AND payment_status = ?", order.ID, models.OrderReadyForPickup, models.PaymentPaid).
		Updates(map[string]interface{}{
			"status":       models.OrderCompleted,
			"completed_at": now,
		})

	if res.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": res.Error.Error(),
		})
	}

	if res.RowsAffected == 0 {
		return c.Status(400).JSON(fiber.Map{
			"error": "Order sudah diproses atau status berubah",
		})
	}

	// Refresh order untuk response
	db.Preload("Product").First(&order, id)
	maskOrderForRole(claims.Role, &order)

	verifiedWeightGrams := 0.0
	if order.WeightPerPortionSnapshot > 0 && order.Quantity > 0 {
		verifiedWeightGrams = order.WeightPerPortionSnapshot * float64(order.Quantity)
	}

	return c.JSON(fiber.Map{
		"message":               "Pickup berhasil, order selesai",
		"order":                 order,
		"verified_weight_grams": verifiedWeightGrams,
	})
}

// Standalone function stubs for routes.go compatibility
func GetOrdersByUMKM(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": "Not implemented"})
}

func UpdateOrderStatus(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{"error": "Not implemented - use OrderHandler.UpdateOrderStatus"})
}
