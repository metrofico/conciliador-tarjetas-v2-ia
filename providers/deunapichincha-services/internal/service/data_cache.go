package service

import (
	db "deunapichincha-services/internal/app/databases"
	"deunapichincha-services/internal/config"
	"deunapichincha-services/utils"
)

type Restaurante struct {
	CodTienda string
	SwitchT   string
}

type GrupoTarjeta struct {
	IdGrupoTarjeta string
	CodCadena      int
	Minimo         int
	Maximo         int
}

type DataCacheRestaurant struct {
	RestaurantCache map[string]Restaurante
	Cfg             config.Config
}

func NewDataCacheRestaurant(cfg config.Config) *DataCacheRestaurant {
	return &DataCacheRestaurant{
		RestaurantCache: make(map[string]Restaurante),
		Cfg:             cfg,
	}
}

func (cache *DataCacheRestaurant) LoadRestaurant() {

	// Crear una nueva conexión a la base de datos
	sql := db.SQLServerConnection{}
	conn, err := sql.NewSQLServerConnection(cache.Cfg.SqlServerSir.JDBC)
	if err != nil {
		utils.Error.Panic("[cache-restaurant-datafast] error conectando a la base de datos: ", err)
		return
	}
	defer conn.Close()
	// Ejecutar la consulta y obtener múltiples registros como un slice de mapas
	query := `SELECT SwitchT, Cod_Tienda FROM Restaurante where SwitchT is not null and TRIM(SwitchT) <> ''`
	rows, err := conn.Query(query)
	if err != nil {
		utils.Error.Panic("[cache-restaurant-datafast] error ejecutando la consulta: ", err)
		return
	}
	for rows.Next() {
		var (
			switchT   string
			codTienda string
		)
		err := rows.Scan(&switchT, &codTienda)
		if err != nil {
			utils.Error.Panic("[cache-restaurant-datafast] error al retornar los datos (Scan)", err)
			return
		}
		cache.RestaurantCache[codTienda] = Restaurante{
			CodTienda: codTienda,
			SwitchT:   switchT,
		}
	}
	rows.Close()

}
func (cache *DataCacheRestaurant) getMerchantId(storeName string) string {
	for _, restaurante := range cache.RestaurantCache {
		if restaurante.CodTienda == storeName {
			return restaurante.SwitchT
		}
	}
	return ""
}
