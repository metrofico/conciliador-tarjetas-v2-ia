package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
	"report-system/utils"
)

type Config struct {
	HttpServer string
	Mongo      MongoConfig
	Nats       NatsConfig
	TimeZone   string
}

type MongoConfig struct {
	URI      string
	Database string
}
type NatsConfig struct {
	URI string
}
type SqlServerSir struct {
	JDBC string
}

func LoadConfig() Config {
	defer func() {
		utils.Info.Println("✅ Configuración cargada correctamente")
	}()
	utils.Info.Println("📌 Cargando configuración...")
	// Cargar el archivo .env si existe
	if err := godotenv.Load(); err != nil {
		log.Println("No se pudo cargar el archivo .env, usando variables de entorno del sistema")
	}
	return Config{
		HttpServer: getEnv("HTTP_SERVER", ":8080"),
		Mongo: MongoConfig{
			URI:      getEnv("MONGO_URI", "no_configurado"),
			Database: getEnv("MONGO_DATABASE", "no_configurado"),
		},
		Nats: NatsConfig{
			URI: getEnv("NATS_URI", "no_configurado"),
		},
		TimeZone: getEnv("TIMEZONE", "no_configurado"),
	}
}

// getEnv obtiene una variable de entorno o usa un valor por defecto.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
