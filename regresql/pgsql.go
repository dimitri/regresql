package regresql

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/lib/pq"
)

func TestConnectionString(pguri string) error {
	fmt.Printf("Trying to connect to %s\n", pguri)
	db, err := sql.Open("postgres", pguri)

	if err != nil {
		return err
	}

	defer db.Close()

	fmt.Println("Connected")

	return nil
}

func QueryDB(db *sql.DB, query string, args ...interface{}) ([]map[string]interface{}, error) {
	if db == nil {
		return nil, errors.New("db is nil")
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	res := make([]map[string]interface{}, 0)

	for rows.Next() {
		container := make([]interface{}, len(cols))
		dest := make([]interface{}, len(cols))
		for i, _ := range container {
			dest[i] = &container[i]
		}
		rows.Scan(dest...)
		r := make(map[string]interface{})
		for i, colname := range cols {
			val := dest[i].(*interface{})
			r[colname] = *val
		}
		res = append(res, r)
	}
	return res, nil
}
