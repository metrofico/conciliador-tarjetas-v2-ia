package main

import (
	"invoker-services/internal/config"
	"invoker-services/internal/server"
	"invoker-services/utils"
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
	utils.Info.Println("✅ Invoker Servicio inicializado")
}
