package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/saveplate/backend/handlers"
	"github.com/saveplate/backend/middleware"
	"github.com/saveplate/backend/models"
)

func SetupRoutes(app *fiber.App) {
	api := app.Group("/api")

	// Auth Routes (public) - Added by Alia (auth module)
	api.Post("/auth/register", handlers.RegisterHandler)
	api.Post("/auth/login", handlers.LoginHandler)

	// Mitra Donasi Register (public)
	api.Post("/mitra-donasi/register", handlers.RegisterMitraDonasiHandler)

	// Profile Routes (protected) - Added by Alia (admin module, BUG 2 fix)
	api.Get("/me", middleware.AuthMiddleware, handlers.GetProfileHandler)
	api.Patch("/me", middleware.AuthMiddleware, handlers.UpdateProfileHandler)

	// Product & Waste Log routes didaftarkan di main.go (hindari duplikasi registrasi)

	// Orders
	// TODO(pemilik modul): handler tidak ditemukan — main tidak compile sejak e4cd920, wiring perlu diperbaiki pemilik modul
	// api.Get("/orders/umkm/:umkm_id", handlers.GetOrdersByUMKM)
	// api.Put("/orders/:id/status", handlers.UpdateOrderStatus)

	// Notifications (protected)
	notifications := api.Group("/notifications", middleware.AuthMiddleware)
	notifications.Get("/user/:user_id", handlers.GetNotificationsByUser)
	notifications.Get("/unread/:user_id", handlers.GetUnreadCount)
	notifications.Put("/:id/read", handlers.MarkAsRead)
	notifications.Put("/read-all/:user_id", handlers.MarkAllAsRead)

	// Analytics
	// TODO(pemilik modul): handler tidak ditemukan — main tidak compile sejak e4cd920, wiring perlu diperbaiki pemilik modul
	// api.Get("/analytics/dashboard/:umkm_id", handlers.GetAnalyticsDashboard)
	// api.Get("/analytics/sales/:umkm_id", handlers.GetAnalyticsSales)
	// api.Get("/analytics/products/:umkm_id", handlers.GetProductAnalytics)
	api.Get("/analytics/trend/:umkm_id", handlers.GetSalesTrend)
	// TODO(pemilik modul): handler tidak ditemukan — main tidak compile sejak e4cd920, wiring perlu diperbaiki pemilik modul
	// api.Get("/analytics/top-products/:umkm_id", handlers.GetTopProducts)
	// api.Get("/analytics/insight/:umkm_id", handlers.GetUmkmInsight)
	// api.Get("/analytics/listing-metrics/:umkm_id", handlers.GetListingMetrics)

	// Ads (Iklan UMKM) - Replaced with stub by Alia (ads parking task)
	api.Get("/ads/packages", handlers.AdComingSoonStub)
	api.Post("/ads", handlers.AdComingSoonStub)
	api.Get("/ads/umkm/:umkm_id", handlers.AdComingSoonStub)
	api.Put("/ads/:id/status", handlers.AdComingSoonStub)
	api.Get("/ads/active", handlers.AdComingSoonStub)

	// Waste Logs didaftarkan di main.go (hindari duplikasi registrasi)

	// Reviews & Keywords
	// TODO(pemilik modul): handler tidak ditemukan — main tidak compile sejak e4cd920, wiring perlu diperbaiki pemilik modul
	// api.Post("/reviews", handlers.CreateReview)
	// api.Get("/reviews/umkm/:umkm_id", handlers.GetReviewsByUMKM)
	// api.Get("/keywords/badge/:umkm_id", handlers.GetKeywordSafetyBadge)

	// Dynamic Discount
	api.Post("/discount/calculate", handlers.CalculateDynamicDiscount)

	// Upload Image (protected)
	api.Post("/upload/image", middleware.AuthMiddleware, handlers.UploadImage)

	// Mitra Donasi Routes (protected - wajib login + role MITRA_DONASI)
	mitraDonasi := api.Group("/mitra-donasi", middleware.AuthMiddleware, middleware.RequireRole(models.RoleMitraDonasi))
	mitraDonasi.Get("/dashboard", handlers.GetMitraDashboardStatsHandler)
	mitraDonasi.Get("/penawaran", handlers.GetMitraDonationOffersHandler)
	mitraDonasi.Post("/penawaran/:id/accept", handlers.AcceptDonationOfferHandler)
	mitraDonasi.Post("/penawaran/:id/reject", handlers.RejectDonationOfferHandler)
	mitraDonasi.Get("/riwayat", handlers.GetMitraDonationHistoryHandler)

	// Mitra Pengolah Routes (protected - wajib login) - Added by Alia (mitra pengolah module)
	api.Post("/mitra-pengolah/apply", middleware.AuthMiddleware, handlers.ApplyMitraPengolahHandler)

	// UMKM Registration Routes (protected - wajib login)
	api.Post("/umkm/register", middleware.AuthMiddleware, handlers.ApplyUMKMHandler)

	// Admin Routes (protected with JWT + RBAC) - Added by Alia (admin module)
	admin := api.Group("/admin", middleware.AuthMiddleware, middleware.RequireRole(models.RoleAdmin))
	admin.Get("/reports/summary", handlers.GetAdminSummaryHandler)
	admin.Get("/umkm", handlers.GetUMKMListHandler)
	admin.Patch("/umkm/:id/verification", handlers.VerifyUMKMHandler)
	admin.Get("/users", handlers.GetUsersHandler)
	admin.Patch("/users/:id/warning", handlers.ModerateUserHandler)
	admin.Patch("/users/:id/suspend", handlers.ModerateUserHandler)
	admin.Get("/revenue", handlers.GetRevenueHandler)
	admin.Get("/revenue/export", handlers.ExportRevenueHandler)
	admin.Get("/advertisements", handlers.GetAdsHandler)
	admin.Patch("/advertisements/:id/status", handlers.ApproveRejectAdHandler)
	admin.Get("/products", handlers.GetProductsHandler)
	admin.Patch("/products/:id/status", handlers.ModerateProductHandler)
	admin.Get("/mitra-donasi", handlers.GetMitraDonasiListHandler)
	admin.Patch("/mitra-donasi/:id/verify", handlers.VerifyMitraDonasiHandler)
	admin.Get("/mitra-pengolah", handlers.GetMitraPengolahListHandler)
	admin.Get("/mitra-pengolah/:id", handlers.GetMitraPengolahDetailHandler)
	admin.Patch("/mitra-pengolah/:id/verify", handlers.VerifyMitraPengolahHandler)
	admin.Get("/help-tickets", handlers.GetTicketsHandler)
	admin.Patch("/help-tickets/:id/status", handlers.UpdateTicketStatusHandler)
}
