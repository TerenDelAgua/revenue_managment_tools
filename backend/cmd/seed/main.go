package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// Cargar archivo .env local si existe
	_ = godotenv.Overload()

	// 1. Definición de Flags
	envFlag := flag.String("env", "development", "Entorno de ejecución (development, staging, production)")
	cleanFlag := flag.Bool("clean", false, "Elimina datos previos en tablas críticas antes de sembrar (¡Peligro en producción!)")
	forceFlag := flag.Bool("force", false, "Fuerza la ejecución en producción")
	fileFlag := flag.String("file", "", "Ejecuta un archivo de seed específico (ej: 004_seed_bookings.sql). Por defecto ejecuta todos.")
	seedsDir := flag.String("dir", "seeds", "Directorio donde se encuentran los archivos SQL de seed")
	flag.Parse()

	// 2. Conexión a Base de Datos (con fallback al valor por defecto local)
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://teren:teren123@localhost:5432/teren_hotels?sslmode=disable"
	}

	// 3. Validación de Seguridad para Producción
	isProduction := *envFlag == "production" || strings.Contains(strings.ToLower(dbURL), "railway.app") || strings.Contains(strings.ToLower(dbURL), "amazonaws")
	if isProduction {
		fmt.Println("⚠️  ADVERTENCIA: Se ha detectado un entorno de PRODUCCIÓN / NUBE.")
		if !*forceFlag {
			log.Fatal("❌ ERROR DE SEGURIDAD: Para ejecutar semillas en producción debes usar el flag '-force'. Abortando.")
		}
		if *cleanFlag {
			log.Fatal("❌ ERROR DE SEGURIDAD: El flag '-clean' está estrictamente prohibido en entornos productivos. Abortando.")
		}
		fmt.Println("ℹ️  Continuando bajo la responsabilidad del usuario (-force activo)...")
	}

	log.Printf("🔌 Conectando a la base de datos [%s]...", *envFlag)
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("❌ Error al abrir la conexión: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("❌ Error de ping a la base de datos: %v", err)
	}
	log.Println("✅ Conexión establecida con éxito.")

	// 4. Operación opcional de limpieza (Clean)
	if *cleanFlag {
		log.Println("🧹 Limpiando tablas de datos de prueba...")
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			log.Fatalf("❌ Fallo al iniciar transacción de limpieza: %v", err)
		}
		
		// Orden de eliminación respetando claves foráneas
		queries := []string{
			"TRUNCATE TABLE bookings CASCADE;",
			"TRUNCATE TABLE guests CASCADE;",
			"TRUNCATE TABLE users CASCADE;",
			"TRUNCATE TABLE rooms CASCADE;",
			"TRUNCATE TABLE room_types CASCADE;",
			"TRUNCATE TABLE floors CASCADE;",
			"TRUNCATE TABLE properties CASCADE;",
		}
		for _, q := range queries {
			if _, err := tx.ExecContext(ctx, q); err != nil {
				tx.Rollback()
				log.Fatalf("❌ Fallo al limpiar tabla con query '%s': %v", q, err)
			}
		}
		if err := tx.Commit(); err != nil {
			log.Fatalf("❌ Fallo al confirmar limpieza: %v", err)
		}
		log.Println("✨ Limpieza completada con éxito.")
	}

	// 5. Lectura y Ordenamiento de archivos de Seeds desde el filesystem
	entries, err := os.ReadDir(*seedsDir)
	if err != nil {
		log.Fatalf("❌ Error al leer directorio de seeds '%s': %v. Asegúrate de ejecutar el comando desde la raíz del backend.", *seedsDir, err)
	}

	var filesToRun []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		if *fileFlag != "" && entry.Name() != *fileFlag {
			continue
		}
		filesToRun = append(filesToRun, entry.Name())
	}

	if len(filesToRun) == 0 {
		log.Fatalf("❌ No se encontraron archivos SQL para ejecutar (filtro file='%s' en directorio '%s')", *fileFlag, *seedsDir)
	}

	sort.Strings(filesToRun)

	// 6. Ejecución Secuencial dentro de Transacciones
	log.Printf("🚀 Iniciando la siembra de %d archivo(s) SQL...", len(filesToRun))
	for _, filename := range filesToRun {
		filePath := filepath.Join(*seedsDir, filename)
		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("❌ Error al leer el archivo de seed %s: %v", filePath, err)
		}

		log.Printf("⏳ Ejecutando %s...", filename)
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			log.Fatalf("❌ Fallo al iniciar transacción para %s: %v", filename, err)
		}

		_, err = tx.ExecContext(ctx, string(content))
		if err != nil {
			tx.Rollback()
			log.Fatalf("❌ ERROR en %s: %v. Transacción revertida.", filename, err)
		}

		if err := tx.Commit(); err != nil {
			log.Fatalf("❌ Fallo al confirmar transacción para %s: %v", filename, err)
		}
		log.Printf("✓ %s aplicado correctamente.", filename)
	}

	log.Println("🎉 Proceso de Seeding finalizado con éxito de forma segura.")
}

