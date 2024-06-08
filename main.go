package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

var user, password, host, database string
var port int

func init() {
	const (
		defaultUser     = "root"
		defaultPassword = ""
		defaultHost     = "localhost"
		defaultDB       = ""
		defaultPort     = 3306
	)
	flag.StringVar(&user, "user", defaultUser, "database administrator account for sign-in")
	flag.StringVar(&password, "password", defaultPassword, "password for the administrator account")
	flag.StringVar(&host, "host", defaultHost, "hostname for server on which database is hosted")
	flag.StringVar(&database, "database", defaultDB, "the database to access")
	flag.IntVar(&port, "port", defaultPort, "the port the database server is on")

}

func dbURI() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", user, password, host, port, database)
}

func dbTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}

	tables := make([]string, 0)

	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tables, nil

}

func allTableRows(db *sql.DB, table string) ([]byte, error) {
	query := fmt.Sprintf("SELECT * FROM %s.%s", database, table)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	colLen := len(cols)
	// Retrieve a list of name->value maps like in JSON
	namedColumns := make([]map[string]interface{}, 0)
	// Pass interfaces to sql.Row.Scan
	colVals := make([]interface{}, colLen)

	for rows.Next() {
		colAssoc := make(map[string]interface{}, len(cols))
		for i := range colVals {
			colVals[i] = new(interface{})
		}
		if err := rows.Scan(colVals...); err != nil {
			return nil, err
		}
		for i, col := range cols {
			grabbedValue := *colVals[i].(*interface{})
			if fmt.Sprintf("%T", grabbedValue) == "[]uint8" {
				colAssoc[col] = fmt.Sprintf("%s", grabbedValue)
			} else {
				colAssoc[col] = grabbedValue
			}
		}
		namedColumns = append(namedColumns, colAssoc)
	}

	json, err := json.Marshal(namedColumns)
	if err != nil {
		return nil, err
	}

	return json, nil
}

func main() {
	flag.Parse()
	uri := dbURI()
	db, err := sql.Open("mysql", uri)
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	tables, err := dbTables(db)
	if err != nil {
		panic(err.Error())
	}

	for _, t := range tables {
		fmt.Println(t)
	}

	jsonResults, err := allTableRows(db, "ProgrammingLanguages")
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(jsonResults))

	jsonResults, err = allTableRows(db, "Notes")
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(jsonResults))
}
