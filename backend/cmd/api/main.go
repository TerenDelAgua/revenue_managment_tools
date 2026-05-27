package main

import (
	"log"
	"net/http"

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

	propertyHandler := internalapi.NewPropertyHandler(propertyRepo)
	floorHandler := internalapi.NewFloorHandler(floorRepo)
	roomHandler := internalapi.NewRoomHandler(roomRepo)

	inventoryService := service.NewInventoryService(db.Pool, roomRepo, roomBlockRepo)
	inventoryHandler := internalapi.NewInventoryHandler(inventoryService)
	roomBlockHandler := internalapi.NewRoomBlockHandler(inventoryService)

	bookingService := service.NewBookingService(db.Pool, bookingRepo, inventoryService)
	bookingHandler := internalapi.NewBookingHandler(bookingService)

	reportRepo := repository.NewReportRepository(db.Pool)
	reportService := service.NewReportService(reportRepo)
	reportHandler := internalapi.NewReportHandler(reportService)

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Property-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("TEREN Hotels Revenue Management API"))
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/properties", func(r chi.Router) {
			r.Get("/", propertyHandler.List)
			r.Post("/", propertyHandler.Create)
			r.Get("/{id}", propertyHandler.GetByID)
		})

		r.Route("/properties/{propertyID}/floors", func(r chi.Router) {
			r.Get("/", floorHandler.ListByProperty)
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
			r.Put("/{id}/position", roomHandler.UpdatePosition)
		})

		r.Get("/map", inventoryHandler.GetMap)

		r.Route("/room-blocks", func(r chi.Router) {
			r.Post("/", roomBlockHandler.Create)
			r.Delete("/{id}", roomBlockHandler.Delete)
		})

		r.Route("/bookings", func(r chi.Router) {
			r.Post("/", bookingHandler.Create)
			r.Get("/pending", bookingHandler.GetPending)
			r.Patch("/{id}", bookingHandler.Assign)
			r.Post("/{id}/checkin", bookingHandler.CheckIn)
			r.Post("/{id}/checkout", bookingHandler.CheckOut)
		})

		r.Route("/reports", func(r chi.Router) {
			r.Get("/metrics", reportHandler.GetMetrics)
			r.Get("/daily", reportHandler.GetDailyBreakdown)
		})
	})

	log.Printf("Server starting on :%s...", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, r))
}
