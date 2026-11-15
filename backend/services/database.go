package services

import (
	"fmt"
	"log"
	"os"

	"github.com/saveplate/backend/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB() error {
	var db *gorm.DB
	var err error

	dbUrl := os.Getenv("DATABASE_URL")
	host := os.Getenv("DB_HOST")

	if dbUrl != "" {
		db, err = gorm.Open(postgres.Open(dbUrl), &gorm.Config{})
	} else if host != "" {
		port := os.Getenv("DB_PORT")
		if port == "" {
			port = "5432"
		}
		user := os.Getenv("DB_USER")
		password := os.Getenv("DB_PASSWORD")
		dbname := os.Getenv("DB_NAME")
		sslmode := os.Getenv("DB_SSLMODE")
		if sslmode == "" {
			if host == "localhost" || host == "127.0.0.1" {
				sslmode = "disable"
			} else {
				sslmode = "require"
			}
		}
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Jakarta", host, port, user, password, dbname, sslmode)
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	} else {
		return fmt.Errorf("database configuration missing: please create backend/.env with DATABASE_URL or DB_HOST")
	}

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	DB = db
	log.Println("✅ Database connection established")

	// Auto-migrate semua model (Inti MVP + Perluasan)
	if os.Getenv("AUTO_MIGRATE") == "true" {
		err = DB.AutoMigrate(
			// Core auth & profiles
			&models.User{},
			&models.CustomerProfile{},
			&models.UMKMProfile{},
			&models.MitraDonasiProfile{},

			// Core products & orders
			&models.Product{},
			&models.Order{},
			&models.Payment{},
			&models.PaymentLog{},

			// Reviews & keywords
			&models.Review{},
			&models.ReviewKeyword{},
			&models.KeywordScore{},

			// Platform revenue
			&models.PlatformRevenue{},

			// Perluasan: ads, help, waste, notifications
			&models.Advertisement{},
			&models.WasteLog{},
			&models.HelpTicket{},
			&models.Notification{},

			// Mitra Donasi: donation offers & history (REVISI #40)
			&models.DonationOffer{},
			&models.DonationHistory{},
		)

		if err != nil {
			log.Fatalf("Failed to auto-migrate models: %v", err)
			return err
		}

		log.Println("✅ Database migrations completed")
	} else {
		log.Println("⏩ Skipping Database migrations (AUTO_MIGRATE != true)")
	}

	return nil
}

func GetDB() *gorm.DB {
	return DB
}
