package regresql

import (
	"database/sql"
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
