package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/joho/godotenv"
	"github.com/saveplate/backend/database"
	"github.com/saveplate/backend/handlers"
	"github.com/saveplate/backend/middleware"
	"github.com/saveplate/backend/models"
	"github.com/saveplate/backend/routes"
	"github.com/saveplate/backend/services"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ .env file not found, using environment variables")
	}

	// Init database
	if err := services.InitDB(); err != nil {
		log.Fatalf("❌ Failed to initialize database: %v", err)
	}

	// Share connection: auth/admin handlers use database.DB, avoid duplicate connection
	// AutoMigrate dikelola terpusat di services.InitDB() (hanya jalan jika AUTO_MIGRATE=true)
	database.DB = services.GetDB()

	// Start cron jobs (auto-expire products)
	services.StartCronJobs()
	log.Println("✅ Cron jobs started")

	// Init Xendit service
	xenditService := services.NewXenditService()
	log.Println("✅ Xendit service initialized")

	// Start scheduler (fallback payment expiry + no-show)
	go func() {
		scheduler := services.NewSchedulerService(services.GetDB())
		scheduler.RunScheduler()
	}()
	log.Println("✅ Scheduler started")

	// Setup Fiber app
	app := fiber.New()

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, PATCH, DELETE",
	}))

	// Setup routes (inline routes for order/payment/review/help-ticket - milik anggota lain)
	setupRoutes(app, xenditService)

	// Setup centralized routes (auth/admin - dari routes/routes.go)
	routes.SetupRoutes(app)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("🚀 Server running on port %s", port)
	log.Fatal(app.Listen(":" + port))
}

func setupRoutes(app *fiber.App, xenditService *services.XenditService) {
	// Grup /api untuk semua route backend (konvensi final tim)
	api := app.Group("/api")

	// Product routes (read publik; mutasi wajib JWT + cek kepemilikan di handler)
	api.Get("/products/umkm/:umkm_id", handlers.GetProductsByUMKM)
	api.Get("/products/marketplace", handlers.GetActiveMarketplaceProducts)
	api.Post("/products", middleware.AuthMiddleware, handlers.CreateProduct)
	api.Put("/products/:id", middleware.AuthMiddleware, handlers.UpdateProduct)
	api.Delete("/products/:id", middleware.AuthMiddleware, handlers.DeleteProduct)

	// Order routes (seluruh operasi order wajib JWT + cek kepemilikan di handler)
	orderHandler := handlers.NewOrderHandler(xenditService)
	api.Post("/orders", middleware.AuthMiddleware, orderHandler.CreateOrder)
	api.Get("/orders", middleware.AuthMiddleware, orderHandler.GetOrders)
	api.Get("/orders/:id", middleware.AuthMiddleware, orderHandler.GetOrderDetail)
	api.Patch("/orders/:id/status", middleware.AuthMiddleware, orderHandler.UpdateOrderStatus)
	api.Post("/orders/:id/validate-pickup", middleware.AuthMiddleware, orderHandler.ValidatePickupCode)

	// Verified-gram redistribution report (Algoritma 1: Wd, Wc, OBR, EC, audit per order)
	api.Get("/redistribution/report", middleware.AuthMiddleware, handlers.GetRedistributionReport)

	// Payment routes (webhook Xendit sekarang di /api/payments/xendit-webhook)
	paymentHandler := handlers.NewPaymentHandler(xenditService)
	api.Post("/payments/xendit-webhook", paymentHandler.XenditWebhook)

	// Review routes (CreateReview wajib JWT; baca publik)
	reviewHandler := handlers.NewReviewHandler()
	api.Post("/reviews", middleware.AuthMiddleware, reviewHandler.CreateReview)
	api.Get("/reviews/keywords/:umkm_id", reviewHandler.GetKeywordSafety)
	api.Get("/reviews/umkm/:umkm_id", reviewHandler.GetReviewsByUmkm)
	api.Get("/reviews/product/:product_id", reviewHandler.GetReviewsByProduct)

	// Help Ticket & Payment Log routes
	helpHandler := handlers.NewHelpTicketHandler()
	api.Post("/help-tickets", middleware.AuthMiddleware, helpHandler.CreateHelpTicket)
	api.Get("/help-tickets", middleware.AuthMiddleware, middleware.RequireRole(models.RoleAdmin), helpHandler.GetHelpTickets)
	api.Get("/payments/:payment_id/logs", middleware.AuthMiddleware, middleware.RequireRole(models.RoleAdmin), helpHandler.GetPaymentLogs)
	api.Patch("/help-tickets/:id/status", middleware.AuthMiddleware, middleware.RequireRole(models.RoleAdmin), helpHandler.UpdateTicketStatus)

	// // Ad routes (Tugas 1)
	// TODO(iklan-soon): dinonaktifkan sementara mengikuti build tag di handlers/ads.go
	// api.Get("/ads/packages", handlers.GetAdPackages)
	// api.Post("/ads", handlers.CreateAd)
	// api.Get("/ads/umkm/:umkm_id", handlers.GetAdsByUMKM)
	// api.Put("/ads/:id/status", handlers.UpdateAdStatus)
	// api.Get("/ads/active", handlers.GetActiveAds)

	// Waste Log routes (Tugas 2)
	api.Get("/waste-logs/umkm/:umkm_id", handlers.GetWasteLogsByUMKM)
	api.Post("/waste-logs", middleware.AuthMiddleware, middleware.RequireRole(models.RoleUmkm), handlers.CreateWasteLog)

	// Analytics routes (Tugas 4)
	api.Get("/analytics/insight/:umkm_id", handlers.GetUmkmInsight)
}
