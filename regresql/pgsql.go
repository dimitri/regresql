package regresql

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type ResultSet struct {
	Cols []string
	Rows [][]interface{}
}

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

func QueryDB(db *sql.DB, query string, args ...interface{}) (*ResultSet, error) {
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

	res := make([][]interface{}, 0)

	for rows.Next() {
		container := make([]interface{}, len(cols))
		dest := make([]interface{}, len(cols))
		for i, _ := range container {
			dest[i] = &container[i]
		}
		rows.Scan(dest...)
		r := make([]interface{}, len(cols))
		for i, _ := range cols {
			val := dest[i].(*interface{})
			r[i] = *val
		}

		res = append(res, r)
	}
	return &ResultSet{cols, res}, nil
}

// ResultSet pretty printer, ala psql (much simplified)
func (r *ResultSet) Println() {
	cn := len(r.Cols)

	// compute max length of values for each col, including column
	// name (used as an header)
	maxl := make([]int, cn)
	for i, colname := range r.Cols {
		maxl[i] = len(colname)
	}
	for _, row := range r.Rows {
		for i, value := range row {
			l := len(valueToString(value))
			if l > maxl[i] {
				maxl[i] = l
			}
		}
	}
	fmts := make([]string, cn)
	for i, l := range maxl {
		fmts[i] = fmt.Sprintf("%%-%ds", l)
	}

	for i, colname := range r.Cols {
		justify := strings.Repeat(" ", (maxl[i]-len(colname))/2)
		centered := justify + colname
		fmt.Printf(fmts[i], centered)
		if i+1 < cn {
			fmt.Printf(" | ")
		}
	}
	fmt.Println()

	for i, l := range maxl {
		fmt.Printf("%s", strings.Repeat("-", l))
		if i+1 < cn {
			fmt.Printf("-+-")
		}
	}
	fmt.Println()

	for _, row := range r.Rows {
		for i, value := range row {
			s := valueToString(value)
			if i+1 < cn {
				fmt.Printf(fmts[i], s)
				fmt.Printf(" | ")
			} else {
				fmt.Printf(s)
			}
		}
		fmt.Println()
	}
}

func valueToString(value interface{}) string {
	var s string
	switch value.(type) {
	case int: s = fmt.Sprintf("%d", value)
	case float32: s = fmt.Sprintf("%g", value)
	case float64: s = fmt.Sprintf("%g", value)
	case time.Time: s = fmt.Sprintf("%s", value)
	case []byte: s = fmt.Sprintf("%s", value)
	default: s = fmt.Sprintf("%v", value)
	}
	return s
}

func (r *ResultSet) Store(fname string) error {
	b := new(bytes.Buffer)
	enc := gob.NewEncoder(b)
	err := enc.Encode(r)
	if err != nil {
		return err
	}

	fh, eopen := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0666)
	defer fh.Close()
	if eopen != nil {
		return eopen
	}
	n, e := fh.Write(b.Bytes())
	if e != nil {
		return e
	}
	fmt.Fprintf(os.Stderr, "%d bytes successfully written to file\n", n)
	return nil
}

func Load(fname string) (*ResultSet, error) {
	fh, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	r := new(ResultSet)
	dec := gob.NewDecoder(fh)
	err = dec.Decode(&r)
	if err != nil {
		return nil, err
	}
	return r, nil
}
