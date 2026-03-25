package db

import _ "github.com/sijms/go-ora/v2"

func oracleDriverName() (string, error) {
	return "oracle", nil
}
