package config

import (
	"log"
	"os"

	"wakirim/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDatabase() error {
	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")
	sslmode := os.Getenv("DB_SSLMODE")

	if dbHost == "" || dbUser == "" || dbName == "" {
		log.Println("[Database] DB config not fully set, skipping database initialization")
		return nil
	}

	dsn := "host=" + dbHost + " user=" + dbUser + " password=" + dbPassword + " dbname=" + dbName + " port=" + dbPort + " sslmode=" + sslmode

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return err
	}

	DB = db
	log.Println("[Database] Database connected successfully")

	if err := RunMigration(); err != nil {
		return err
	}

	return nil
}

func RunMigration() error {
	err := DB.AutoMigrate(
		&model.Akun{},
		&model.Payment{},
		&model.Paket{},
		&model.Pengaduan{},
		&model.DataContact{},
	)
	if err != nil {
		return err
	}

	log.Println("[Migration] AutoMigrate completed successfully")

	// Seed default pakets
	if err := SeedPakets(); err != nil {
		log.Printf("[Migration] Warning: failed to seed pakets: %v", err)
	}

	return nil
}

func SeedPakets() error {
	pakets := []model.Paket{
		{ID: "starter", Nama: "Starter", Harga: 59000, DurasiBulan: 1, Aktif: true},
		{ID: "growth", Nama: "Growth", Harga: 149000, DurasiBulan: 3, Aktif: true},
		{ID: "best-value", Nama: "Best Value", Harga: 295000, DurasiBulan: 12, Aktif: true},
	}

	for _, p := range pakets {
		var existing model.Paket
		result := DB.First(&existing, "id = ?", p.ID)
		if result.Error == gorm.ErrRecordNotFound {
			if err := DB.Create(&p).Error; err != nil {
				return err
			}
			log.Printf("[Seed] Created paket: %s", p.Nama)
		}
	}

	log.Println("[Seed] Paket seeding completed")
	return nil
}

func CloseDatabase() {
	sqlDB, err := DB.DB()
	if err != nil {
		return
	}
	sqlDB.Close()
	log.Println("[Database] Database connection closed")
}
