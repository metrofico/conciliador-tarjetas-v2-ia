package service

import (
	db "kioscos-services/internal/app/databases"
	"kioscos-services/internal/config"
	"kioscos-services/utils"
)

type Restaurante struct {
	IdLocal string
	SwitchT string
}

type GrupoTarjeta struct {
	IdGrupoTarjeta string
	CodCadena      int
	Minimo         int
	Maximo         int
}

type DataCacheRestaurant struct {
	RestaurantCache   map[string]Restaurante
	GrupoTarjetaCache map[string]GrupoTarjeta
	Cfg               config.Config
}

func NewDataCacheRestaurant(cfg config.Config) *DataCacheRestaurant {
	return &DataCacheRestaurant{
		RestaurantCache:   make(map[string]Restaurante),
		GrupoTarjetaCache: make(map[string]GrupoTarjeta),
		Cfg:               cfg,
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
	query := `SELECT SwitchT, Cod_Restaurante FROM Restaurante where SwitchT is not null and TRIM(SwitchT) <> ''`
	rows, err := conn.Query(query)
	if err != nil {
		utils.Error.Panic("[cache-restaurant-datafast] error ejecutando la consulta: ", err)
		return
	}
	for rows.Next() {
		var (
			switchT string
			idLocal string
		)
		err := rows.Scan(&switchT, &idLocal)
		if err != nil {
			utils.Error.Panic("[cache-restaurant-datafast] error al retornar los datos (Scan)", err)
			return
		}
		cache.RestaurantCache[idLocal] = Restaurante{
			IdLocal: idLocal,
			SwitchT: switchT,
		}
	}
	rows.Close()

}
func (cache *DataCacheRestaurant) getMerchantId(idLocal string) string {
	for _, restaurante := range cache.RestaurantCache {
		if restaurante.IdLocal == idLocal {
			return restaurante.SwitchT
		}
	}
	return ""
}
