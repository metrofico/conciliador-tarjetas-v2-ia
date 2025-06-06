package db

import (
	"context"
	"database/sql"
	"datafast-services/utils"
	"fmt"
	"github.com/microsoft/go-mssqldb/azuread"
)

// Estructura que representa una conexión a la base de datos
type SQLServerConnection struct {
	db         *sql.DB
	ConnString *string
	conection  *sql.Conn
	tx         *sql.Tx
}

// Función para establecer la conexión
func NewSQLServerConnection(connString string) (*SQLServerConnection, error) {

	db, err := sql.Open(azuread.DriverName, connString)
	if err != nil {
		utils.Info.Println("Error al conectar a la base de datos:", err)
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, err
	}
	return &SQLServerConnection{db: db}, nil
}
func (conn *SQLServerConnection) CreateBegin() error {
	tx, err := conn.db.Begin()
	if err != nil {
		return err
	}
	conn.SetTx(tx)
	return err
}
func (conn *SQLServerConnection) SetTx(tx *sql.Tx) {
	conn.tx = tx
}

func (conn *SQLServerConnection) Commit() error {
	if conn.tx != nil {
		defer conn.SetTx(nil)
		err := conn.tx.Commit()
		if err != nil {
			return err
		}
	}
	return nil
}

func (conn *SQLServerConnection) Rollback() error {
	if conn.tx != nil {
		defer conn.SetTx(nil)
		err := conn.tx.Rollback()
		if err != nil {
			fmt.Print("Register Log: ", err.Error())
		}
	}
	return nil
}

func (conn *SQLServerConnection) Query(query string, args ...sql.NamedArg) (*sql.Rows, error) {
	newArgs := []any{}
	for _, arg := range args {
		newArgs = append(newArgs, arg)
	}
	if conn.tx != nil {
		rows, err := conn.tx.Query(query, newArgs...)
		if err != nil {
			fmt.Println(query, args)
			fmt.Println(err.Error())
			return nil, err
		}
		return rows, nil
	}
	rows, err := conn.db.Query(query, newArgs...)
	if err != nil {
		fmt.Println(query, args)
		fmt.Println(err.Error())
		return nil, err
	}
	return rows, nil
}

func (conn *SQLServerConnection) IQueryRow(query string, params []any) (*sql.Row, error) {
	ctx := context.Background()
	var err error
	if conn.tx != nil {
		result := conn.tx.QueryRow(query, params...)
		return result, nil
	}
	if conn.db == nil {
		return nil, err
	}
	err = conn.db.PingContext(ctx)
	if err != nil {
		return nil, err
	}
	result := conn.db.QueryRow(query, params...)
	return result, nil
}
func (conn *SQLServerConnection) IQuery(query string, params []any) (*sql.Rows, error) {
	ctx := context.Background()
	var err error

	if conn.tx != nil {
		result, err := conn.tx.Query(query, params...)
		if err != nil {
			return nil, err
		}
		return result, nil
	}
	if conn.db == nil {
		return nil, err
	}
	err = conn.db.PingContext(ctx)
	if err != nil {
		return nil, err
	}
	result, err := conn.db.Query(query, params...)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (conn *SQLServerConnection) QueryRow(query string, args ...sql.NamedArg) *sql.Row {
	var newArgs []any
	for _, arg := range args {
		newArgs = append(newArgs, arg)
	}
	if conn.tx != nil {
		row := conn.tx.QueryRow(query, newArgs...)
		return row
	}
	row := conn.db.QueryRow(query, newArgs...)
	return row
}

func (conn *SQLServerConnection) Exec(query string, args ...sql.NamedArg) (sql.Result, error) {
	var newArgs []any
	for _, arg := range args {
		newArgs = append(newArgs, arg)
	}
	if conn.tx != nil {
		result, err := conn.tx.Exec(query, newArgs...)
		if err != nil {
			fmt.Println(query, args)
			fmt.Println(err.Error())
			return nil, err
		}
		return result, nil
	}
	result, err := conn.db.Exec(query, newArgs...)
	if err != nil {
		fmt.Println(query, args)
		fmt.Println(err.Error())
		return nil, err
	}
	return result, nil
}

func (conn *SQLServerConnection) Prepare(query string) (*sql.Stmt, error) {
	stmt, err := conn.db.Prepare(query)
	if err != nil {
		fmt.Println(query)
		fmt.Println(err.Error())
		return nil, err
	}
	return stmt, nil
}

func (conn *SQLServerConnection) PrepareTx(query string, args ...sql.NamedArg) (*sql.Stmt, error) {

	stmt, err := conn.tx.Prepare(query)
	if err != nil {
		fmt.Println(query)
		fmt.Println(err.Error())
		return nil, err
	}
	return stmt, nil
}
func (conn *SQLServerConnection) Close() {
	err := conn.db.Close()
	if err != nil {
		return
	}
}
