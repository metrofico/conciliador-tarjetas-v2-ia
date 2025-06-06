package service

import (
	db "sir-writer/internal/app/databases"
	"sir-writer/internal/config"
	"sir-writer/utils"
	"strconv"
)

type Restaurante struct {
	CodTienda  string
	SwitchT    string
	MID        string
	TipoSwitch string
	Origen     string
	Sistema    string
	CodCadena  string
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

func (cache *DataCacheRestaurant) LoadRestaurantAndGrupo() {

	// Crear una nueva conexión a la base de datos
	conn, err := db.NewSQLServerConnection(cache.Cfg.SqlServerSir.JDBC)
	if err != nil {
		utils.Error.Panic("[cache-restaurant-datafast] error conectando a la base de datos: ", err)
		return
	}
	defer conn.Close()
	// Ejecutar la consulta y obtener múltiples registros como un slice de mapas
	query := `SELECT 
				r.Cod_Tienda, 
				r.SwitchT, 
				mids.MID, 
				ts.Id_tipo_switch AS tipo_switch, 
				ot.id_origen AS origen, 
				'DATBALANCE' AS sistema, 
				r.Cod_Cadena
			FROM dbo.MIDs_Restaurante AS mids WITH(NOLOCK)
			INNER JOIN dbo.Restaurante AS r WITH(NOLOCK) 
				ON r.Cod_Restaurante = mids.Cod_Restaurante 
			INNER JOIN dbo.Red_Pagos AS medio WITH(NOLOCK) 
				ON medio.Cod_Red_Pagos = mids.Cod_Red_Pagos 
			CROSS JOIN (SELECT Id_tipo_switch FROM ST_Tipo_Switch WITH(NOLOCK) WHERE Descripcion = 'Alignet/Externo') ts
			CROSS JOIN (SELECT id_origen FROM ST_Origen_Transaccion WITH(NOLOCK) WHERE Descripcion = 'DataFast') ot
			WHERE medio.Descripcion = 'DATAFAST' 
			AND mids.Estado = 1;`
	rows, err := conn.Query(query)
	if err != nil {
		utils.Error.Panic("[cache-restaurant-datafast] error ejecutando la consulta: ", err)
		return
	}
	for rows.Next() {
		var (
			codTienda  string
			switchT    string
			mid        string
			tipoSwitch string
			origen     string
			sistema    string
			codCadena  string
		)
		err := rows.Scan(&codTienda, &switchT, &mid, &tipoSwitch, &origen, &sistema, &codCadena)
		if err != nil {
			utils.Error.Panic("[cache-restaurant-datafast] error al retornar los datos (Scan)", err)
			return
		}
		cache.RestaurantCache[codTienda] = Restaurante{
			CodTienda:  codTienda,
			SwitchT:    switchT,
			MID:        mid,
			TipoSwitch: tipoSwitch,
			Origen:     origen,
			Sistema:    sistema,
			CodCadena:  codCadena,
		}
	}
	rows.Close()
	// Cargar Grupo Tarjetas

	query = `SELECT 
				id_grupo_tarjeta, 
				FPN.Cod_Cadena, 
				FPN.Minimo, 
				FPN.Maximo 
			FROM ST_Grupo_Tarjeta WITH(NOLOCK) 
			INNER JOIN (
				SELECT DISTINCT 
					RTRIM(LTRIM(fp.Nombre)) AS Nombre, 
					fpb.Cod_Cadena, 
					fpb.Minimo, 
					fpb.Maximo 
				FROM FormadePago_Bines AS fpb WITH(NOLOCK) 
				INNER JOIN FormasPago AS fp WITH(NOLOCK) 
					ON fp.Cod_FormaPago = fpb.Cod_FormaPago
			) AS FPN 
			ON LTRIM(RTRIM(Descripcion)) LIKE '%' + 
				(CASE 
					WHEN Nombre = 'ALIA' THEN 'COUTA FACIL' 
					ELSE Nombre 
				END) + '%';`

	rows, err = conn.Query(query)
	if err != nil {
		utils.Error.Panic("[cache-grupostarjetas-datafast] error ejecutando la consulta: ", err)
		return
	}
	for rows.Next() {
		var (
			idGrupoTarjeta string
			codCadena      int
			minimo         int
			maximo         int
		)
		err := rows.Scan(&idGrupoTarjeta, &codCadena, &minimo, &maximo)
		if err != nil {
			utils.Error.Panic("[cache-restaurant-datafast] error al retornar los datos (Scan)", err)
			return
		}
		cache.GrupoTarjetaCache[idGrupoTarjeta] = GrupoTarjeta{
			IdGrupoTarjeta: idGrupoTarjeta,
			CodCadena:      codCadena,
			Minimo:         minimo,
			Maximo:         maximo,
		}
	}
	rows.Close()
}
func (cache *DataCacheRestaurant) getMerchantId(mid string) string {
	for _, restaurante := range cache.RestaurantCache {
		if restaurante.MID == mid {
			return restaurante.SwitchT
		}
	}
	return ""
}
func (cache *DataCacheRestaurant) getTipoSwitch(mid string) int {
	for _, restaurante := range cache.RestaurantCache {
		if restaurante.MID == mid {
			result, err := strconv.Atoi(restaurante.TipoSwitch)
			if err != nil {
				return -1
			}
			return result
		}
	}
	return -1
}
func (cache *DataCacheRestaurant) getOrigen(mid string) int {
	for _, restaurante := range cache.RestaurantCache {
		if restaurante.MID == mid {
			result, err := strconv.Atoi(restaurante.Origen)
			if err != nil {
				return -1
			}
			return result
		}
	}
	return -1
}

func (cache *DataCacheRestaurant) getSistema(mid string) string {
	for _, restaurante := range cache.RestaurantCache {
		if restaurante.MID == mid {
			return restaurante.Sistema
		}
	}
	return ""
}
func (cache *DataCacheRestaurant) getCodCadena(mid string) int {
	for _, restaurante := range cache.RestaurantCache {
		if restaurante.MID == mid {
			codCadena, err := strconv.Atoi(restaurante.CodCadena)
			if err != nil {
				utils.Error.Println("[getCodCadenaFromCache] Error al convertir CodCadena:", err)
				return 0
			}
			return codCadena
		}
	}
	return 0
}
func (cache *DataCacheRestaurant) getIdGrupoTarjeta(CodCadena int, Bin string) string {
	// Convertir Bin a entero
	binInt, err := strconv.Atoi(Bin)
	if err != nil {
		utils.Error.Println("[getIdGrupoTarjeta] Error al convertir Bin a entero:", err)
		return ""
	}

	for _, grupoTarjeta := range cache.GrupoTarjetaCache {
		// Comprobamos si CodCadena coincide
		if grupoTarjeta.CodCadena == CodCadena {
			// Evaluamos si el Bin está dentro del rango definido por Minimo y Maximo
			if binInt >= grupoTarjeta.Minimo && binInt <= grupoTarjeta.Maximo {
				return grupoTarjeta.IdGrupoTarjeta
			}
		}
	}
	return ""
}
