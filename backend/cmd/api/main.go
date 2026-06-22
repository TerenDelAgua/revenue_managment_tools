package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	internalapi "github.com/terendelagua/teren-hotels-backend/internal/api"
	"github.com/terendelagua/teren-hotels-backend/internal/repository"
	"github.com/terendelagua/teren-hotels-backend/internal/service"
	"github.com/terendelagua/teren-hotels-backend/pkg/config"
	"github.com/terendelagua/teren-hotels-backend/pkg/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	propertyRepo := repository.NewPropertyRepository(db.Pool)
	floorRepo := repository.NewFloorRepository(db.Pool)
	roomRepo := repository.NewRoomRepository(db.Pool)
	roomBlockRepo := repository.NewRoomBlockRepository(db.Pool)
	bookingRepo := repository.NewBookingRepository(db.Pool)
	guestRepo := repository.NewGuestRepository(db.Pool)
	invoiceRepo := repository.NewInvoiceRepository(db.Pool)

	propertyHandler := internalapi.NewPropertyHandler(propertyRepo)
	floorHandler := internalapi.NewFloorHandler(floorRepo)
	roomHandler := internalapi.NewRoomHandler(roomRepo)

	inventoryService := service.NewInventoryService(db.Pool, roomRepo, roomBlockRepo)
	inventoryHandler := internalapi.NewInventoryHandler(inventoryService)
	roomBlockHandler := internalapi.NewRoomBlockHandler(inventoryService)

	// InvoiceService wired without a PDFGenerator (B5 will inject it).
	// Until then, the service falls back to "pdf_url NULL, regenerable later"
	// per spec §8.1.
	invoiceService := service.NewInvoiceService(db.Pool, invoiceRepo, bookingRepo, nil)
	bookingService := service.NewBookingService(db.Pool, bookingRepo, guestRepo, inventoryService, invoiceService)
	bookingHandler := internalapi.NewBookingHandler(bookingService)
	invoiceHandler := internalapi.NewInvoiceHandler(invoiceService)

	guestService := service.NewGuestService(guestRepo)
	guestHandler := internalapi.NewGuestHandler(guestService)

	reportRepo := repository.NewReportRepository(db.Pool)
	reportService := service.NewReportService(reportRepo)
	reportHandler := internalapi.NewReportHandler(reportService)
	healthHandler := internalapi.NewHealthHandler(db.Pool)

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	allowedOrigins := []string{
		"http://localhost:5173",
		"http://localhost:3000",
	}

	if envOrigins := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS")); envOrigins != "" {
		for _, origin := range strings.Split(envOrigins, ",") {
			origin = strings.TrimSpace(origin)
			origin = strings.Trim(origin, `"'`)
			if origin != "" {
				allowedOrigins = append(allowedOrigins, origin)
				log.Printf("[CORS] Added origin: %s", origin)
			}
		}
	}
	log.Printf("[CORS] Final allowed origins: %v", allowedOrigins)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"},
		AllowedHeaders: []string{
			"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Session-Id", "X-Requested-With",
			"X-Property-ID", "Accept-Encoding", "User-Agent", "Cache-Control", "Pragma", "Origin",
		},
		ExposedHeaders: []string{
			"Link", "Content-Length", "X-Request-Id", "X-Session-Id",
			"Accept-Encoding",
			"User-Agent",
			"Cache-Control",
			"Pragma", "Origin",
		},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("TEREN Hotels Revenue Management API"))
	})

	r.Get("/health", healthHandler.Check)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/properties", func(r chi.Router) {
			r.Get("/", propertyHandler.List)
			r.Post("/", propertyHandler.Create)
			r.Get("/{id}", propertyHandler.GetByID)
		})

		r.Route("/properties/{propertyID}/floors", func(r chi.Router) {
			r.Get("/", floorHandler.ListByProperty)
		})

		r.Route("/properties/{propertyID}/room-types", func(r chi.Router) {
			r.Get("/", roomHandler.ListRoomTypes)
		})

		r.Route("/floors", func(r chi.Router) {
			r.Post("/", floorHandler.Create)
			r.Get("/{id}", floorHandler.GetByID)
		})

		r.Route("/floors/{floorID}/rooms", func(r chi.Router) {
			r.Get("/", roomHandler.ListByFloor)
		})

		r.Route("/rooms", func(r chi.Router) {
			r.Post("/", roomHandler.Create)
			r.Get("/{id}", roomHandler.GetByID)
			r.Patch("/{id}", roomHandler.Update)
			r.Delete("/{id}", roomHandler.Delete)
			r.Put("/{id}/position", roomHandler.UpdatePosition)
			// Housekeeping state transitions (Spec FMB-001 follow-up: cleaning)
			r.Post("/{id}/cleaning", inventoryHandler.SetCleaning)
			r.Delete("/{id}/cleaning", inventoryHandler.ClearCleaning)
		})

		r.Get("/map", inventoryHandler.GetMap)

		r.Route("/room-blocks", func(r chi.Router) {
			r.Post("/", roomBlockHandler.Create)
			r.Delete("/{id}", roomBlockHandler.Delete)
		})

		r.Route("/bookings", func(r chi.Router) {
			r.Post("/", bookingHandler.Create)
			r.Get("/", bookingHandler.List)
			r.Get("/{id}", bookingHandler.GetByID)
			r.Patch("/{id}", bookingHandler.Update)
			r.Get("/pending", bookingHandler.GetPending)
			r.Patch("/{id}/assign", bookingHandler.Assign)
			r.Post("/{id}/checkin", bookingHandler.CheckIn)
			r.Post("/{id}/checkout", bookingHandler.CheckOut)
			r.Post("/{id}/cancel", bookingHandler.Cancel)
			// Invoicing module (spec §4) — invoice under booking.
			r.Get("/{id}/invoice", invoiceHandler.GetByBookingID)
		})

		// Invoicing module (spec §4)
		r.Route("/invoices", func(r chi.Router) {
			r.Get("/", invoiceHandler.List)
			r.Get("/daily-summary", invoiceHandler.DailySummary)
			r.Get("/tax-report", invoiceHandler.MonthlyTaxReport)
			r.Get("/by-booking/{bookingId}", invoiceHandler.GetByBookingID)
			r.Get("/{id}", invoiceHandler.GetByID)
			r.Patch("/{id}/notes", invoiceHandler.UpdateNotes)
			r.Post("/{id}/void", invoiceHandler.Void)
			r.Post("/{id}/payments", invoiceHandler.RegisterPayment)
			r.Post("/{id}/regenerate-pdf", invoiceHandler.RegeneratePDF)
		})

		r.Route("/guests", func(r chi.Router) {
			r.Post("/", guestHandler.Create)
			r.Get("/", guestHandler.List)
			r.Get("/{id}", guestHandler.GetByID)
			r.Patch("/{id}", guestHandler.Update)
		})

		r.Route("/reports", func(r chi.Router) {
			r.Get("/metrics", reportHandler.GetMetrics)
			r.Get("/daily", reportHandler.GetDailyBreakdown)
		})
	})

	log.Printf("Server starting on :%s...", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, r))
}
