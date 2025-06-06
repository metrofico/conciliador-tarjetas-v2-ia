package main

import (
	"deunapichincha-services/internal/config"
	"deunapichincha-services/internal/server"
	"deunapichincha-services/utils"
)

/*
* DATAFAST SERVICE PROVIDER
*
* Este microservicio está diseñado para ejecutarse como un CronJob dentro de un clúster de Kubernetes.
* El servicio de DATAFAST se encarga de generar y proporcionar el reporte de las transacciones
* 24 horas después de haber efectuado el corte del lote.
 */
func main() {
	cfg := config.LoadConfig()
	server.NewContainer(cfg)
	utils.Info.Println("✅ Servicio inicializado")
}
