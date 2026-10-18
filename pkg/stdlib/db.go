// Package stdlib provides production-grade database connectivity for SQLite and PostgreSQL.
package stdlib

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alca-lang/alca/pkg/interpreter"
)

// ActiveDBRegistry tracks active database pools for graceful cleanup.
type ActiveDBRegistry struct {
	mu    sync.RWMutex
	pools []*sql.DB
}

var dbRegistry = &ActiveDBRegistry{
	pools: make([]*sql.DB, 0),
}

// CloseAllDatabases cleanly terminates all open connection pools.
func CloseAllDatabases() {
	dbRegistry.mu.Lock()
	defer dbRegistry.mu.Unlock()

	for _, pool := range dbRegistry.pools {
		if pool != nil {
			_ = pool.Close()
		}
	}
	dbRegistry.pools = nil
}

func registerDBPool(db *sql.DB) {
	dbRegistry.mu.Lock()
	defer dbRegistry.mu.Unlock()
	dbRegistry.pools = append(dbRegistry.pools, db)
}

func unregisterDBPool(db *sql.DB) {
	dbRegistry.mu.Lock()
	defer dbRegistry.mu.Unlock()
	for idx, pool := range dbRegistry.pools {
		if pool == db {
			dbRegistry.pools = append(dbRegistry.pools[:idx], dbRegistry.pools[idx+1:]...)
			break
		}
	}
}

// RegisterDBBuiltins registers std.db, std.db.postgres, and sqlite builtins.
func RegisterDBBuiltins(env *interpreter.Environment) {
	dbModule := interpreter.NewMap()

	// db.open(driver, dsn)
	dbModule.Set("open", &interpreter.BuiltinFunction{
		Name: "db.open",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 2 {
				return interpreter.NewError(400, "db.open(driver, dsn) requires driver and connection string")
			}
			driver := strings.ToLower(interpreter.CoerceToString(args[0]))
			dsn := interpreter.CoerceToString(args[1])

			switch driver {
			case "sqlite", "sqlite3":
				driver = "sqlite"
			case "postgres", "postgresql", "pg":
				driver = "postgres"
			}

			db, err := sql.Open(driver, dsn)
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("database open error: %s", err.Error()))
			}

			// Default enterprise connection pool settings
			db.SetMaxOpenConns(25)
			db.SetMaxIdleConns(5)
			db.SetConnMaxLifetime(5 * time.Minute)
			db.SetConnMaxIdleTime(2 * time.Minute)

			registerDBPool(db)
			return wrapDBConnection(db, driver)
		},
	})

	// db.sqlite(path)
	dbModule.Set("sqlite", &interpreter.BuiltinFunction{
		Name: "db.sqlite",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			path := ":memory:"
			if len(args) > 0 {
				path = interpreter.CoerceToString(args[0])
			}
			db, err := sql.Open("sqlite", path)
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("sqlite open error: %s", err.Error()))
			}
			db.SetMaxOpenConns(1) // Best practice for SQLite to avoid lock contention
			registerDBPool(db)
			return wrapDBConnection(db, "sqlite")
		},
	})

	// db.postgres(dsn)
	dbModule.Set("postgres", &interpreter.BuiltinFunction{
		Name: "db.postgres",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.NewError(400, "db.postgres(dsn) requires connection string")
			}
			dsn := interpreter.CoerceToString(args[0])
			db, err := sql.Open("postgres", dsn)
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("postgres open error: %s", err.Error()))
			}
			db.SetMaxOpenConns(50)
			db.SetMaxIdleConns(10)
			db.SetConnMaxLifetime(10 * time.Minute)
			registerDBPool(db)
			return wrapDBConnection(db, "postgres")
		},
	})

	env.Set("db", dbModule)
	env.Set("std.db", dbModule)
}

func wrapDBConnection(db *sql.DB, driver string) *interpreter.MapValue {
	connObj := interpreter.NewMap()
	connObj.Set("driver", &interpreter.StringValue{Val: driver})

	// conn.ping() -> bool
	connObj.Set("ping", &interpreter.BuiltinFunction{
		Name: "ping",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			err := db.Ping()
			return &interpreter.BoolValue{Val: err == nil}
		},
	})

	// conn.exec(query, args...) -> {rows_affected: int, last_insert_id: int}
	connObj.Set("exec", &interpreter.BuiltinFunction{
		Name: "exec",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.NewError(400, "exec requires SQL query string")
			}
			query := interpreter.CoerceToString(args[0])
			sqlArgs := convertAlcaArgs(args[1:])

			res, err := db.Exec(query, sqlArgs...)
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("exec error: %s", err.Error()))
			}

			resultMap := interpreter.NewMap()
			if ra, err := res.RowsAffected(); err == nil {
				resultMap.Set("rows_affected", &interpreter.IntegerValue{Val: ra})
			} else {
				resultMap.Set("rows_affected", &interpreter.IntegerValue{Val: 0})
			}

			if lid, err := res.LastInsertId(); err == nil {
				resultMap.Set("last_insert_id", &interpreter.IntegerValue{Val: lid})
			} else {
				resultMap.Set("last_insert_id", interpreter.NULL)
			}

			return resultMap
		},
	})

	// conn.query(query, args...) -> [ {col1: val1, ...}, ... ]
	connObj.Set("query", &interpreter.BuiltinFunction{
		Name: "query",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.NewError(400, "query requires SQL query string")
			}
			query := interpreter.CoerceToString(args[0])
			sqlArgs := convertAlcaArgs(args[1:])

			rows, err := db.Query(query, sqlArgs...)
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("query error: %s", err.Error()))
			}
			defer rows.Close()

			return scanRows(rows)
		},
	})

	// conn.query_row(query, args...) -> {col1: val1, ...} or null
	connObj.Set("query_row", &interpreter.BuiltinFunction{
		Name: "query_row",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.NewError(400, "query_row requires SQL query string")
			}
			query := interpreter.CoerceToString(args[0])
			sqlArgs := convertAlcaArgs(args[1:])

			rows, err := db.Query(query, sqlArgs...)
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("query error: %s", err.Error()))
			}
			defer rows.Close()

			listVal := scanRows(rows)
			if l, ok := listVal.(*interpreter.ListValue); ok && len(l.Elements) > 0 {
				return l.Elements[0]
			}
			return interpreter.NULL
		},
	})

	// conn.set_max_open_conns(n)
	connObj.Set("set_max_open_conns", &interpreter.BuiltinFunction{
		Name: "set_max_open_conns",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) > 0 {
				if iv, ok := args[0].(*interpreter.IntegerValue); ok {
					db.SetMaxOpenConns(int(iv.Val))
				}
			}
			return interpreter.NULL
		},
	})

	// conn.set_max_idle_conns(n)
	connObj.Set("set_max_idle_conns", &interpreter.BuiltinFunction{
		Name: "set_max_idle_conns",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) > 0 {
				if iv, ok := args[0].(*interpreter.IntegerValue); ok {
					db.SetMaxIdleConns(int(iv.Val))
				}
			}
			return interpreter.NULL
		},
	})

	// conn.set_conn_max_lifetime(seconds)
	connObj.Set("set_conn_max_lifetime", &interpreter.BuiltinFunction{
		Name: "set_conn_max_lifetime",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) > 0 {
				if iv, ok := args[0].(*interpreter.IntegerValue); ok {
					db.SetConnMaxLifetime(time.Duration(iv.Val) * time.Second)
				}
			}
			return interpreter.NULL
		},
	})

	// conn.begin() -> Transaction
	connObj.Set("begin", &interpreter.BuiltinFunction{
		Name: "begin",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			tx, err := db.Begin()
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("transaction begin error: %s", err.Error()))
			}
			return wrapTransaction(tx)
		},
	})

	// conn.transaction(func(tx) { ... })
	connObj.Set("transaction", &interpreter.BuiltinFunction{
		Name: "transaction",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.NewError(400, "transaction requires callback function")
			}
			tx, err := db.Begin()
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("transaction begin error: %s", err.Error()))
			}
			txObj := wrapTransaction(tx)

			interp := interpreter.New()
			res, err := interp.InvokeFunction(args[0], []interpreter.Value{txObj})
			if err != nil || (res != nil && res.Type() == interpreter.ERROR_VAL) {
				_ = tx.Rollback()
				if err != nil {
					return interpreter.NewError(500, fmt.Sprintf("transaction rolled back: %s", err.Error()))
				}
				return res
			}

			if err := tx.Commit(); err != nil {
				return interpreter.NewError(500, fmt.Sprintf("transaction commit error: %s", err.Error()))
			}
			return res
		},
	})

	// conn.close()
	connObj.Set("close", &interpreter.BuiltinFunction{
		Name: "close",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			unregisterDBPool(db)
			err := db.Close()
			if err != nil {
				return interpreter.NewError(500, err.Error())
			}
			return &interpreter.BoolValue{Val: true}
		},
	})

	return connObj
}

func wrapTransaction(tx *sql.Tx) *interpreter.MapValue {
	txObj := interpreter.NewMap()

	txObj.Set("exec", &interpreter.BuiltinFunction{
		Name: "exec",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.NewError(400, "tx.exec requires SQL query")
			}
			query := interpreter.CoerceToString(args[0])
			sqlArgs := convertAlcaArgs(args[1:])

			res, err := tx.Exec(query, sqlArgs...)
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("tx.exec error: %s", err.Error()))
			}

			resultMap := interpreter.NewMap()
			if ra, err := res.RowsAffected(); err == nil {
				resultMap.Set("rows_affected", &interpreter.IntegerValue{Val: ra})
			}
			if lid, err := res.LastInsertId(); err == nil {
				resultMap.Set("last_insert_id", &interpreter.IntegerValue{Val: lid})
			}
			return resultMap
		},
	})

	txObj.Set("query", &interpreter.BuiltinFunction{
		Name: "query",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			if len(args) < 1 {
				return interpreter.NewError(400, "tx.query requires SQL query")
			}
			query := interpreter.CoerceToString(args[0])
			sqlArgs := convertAlcaArgs(args[1:])

			rows, err := tx.Query(query, sqlArgs...)
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("tx.query error: %s", err.Error()))
			}
			defer rows.Close()

			return scanRows(rows)
		},
	})

	txObj.Set("commit", &interpreter.BuiltinFunction{
		Name: "commit",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			err := tx.Commit()
			if err != nil {
				return interpreter.NewError(500, fmt.Sprintf("tx.commit error: %s", err.Error()))
			}
			return &interpreter.BoolValue{Val: true}
		},
	})

	txObj.Set("rollback", &interpreter.BuiltinFunction{
		Name: "rollback",
		Fn: func(args ...interpreter.Value) interpreter.Value {
			err := tx.Rollback()
			if err != nil && err != sql.ErrTxDone {
				return interpreter.NewError(500, fmt.Sprintf("tx.rollback error: %s", err.Error()))
			}
			return &interpreter.BoolValue{Val: true}
		},
	})

	return txObj
}

func convertAlcaArgs(args []interpreter.Value) []interface{} {
	out := make([]interface{}, len(args))
	for idx, a := range args {
		switch v := a.(type) {
		case *interpreter.IntegerValue:
			out[idx] = v.Val
		case *interpreter.FloatValue:
			out[idx] = v.Val
		case *interpreter.StringValue:
			out[idx] = v.Val
		case *interpreter.BoolValue:
			out[idx] = v.Val
		case *interpreter.BlobValue:
			out[idx] = v.Data
		case *interpreter.NullValue:
			out[idx] = nil
		default:
			out[idx] = interpreter.CoerceToString(v)
		}
	}
	return out
}

func scanRows(rows *sql.Rows) interpreter.Value {
	cols, err := rows.Columns()
	if err != nil {
		return interpreter.NewError(500, err.Error())
	}

	results := interpreter.NewList()

	for rows.Next() {
		rawVals := make([]interface{}, len(cols))
		scanArgs := make([]interface{}, len(cols))
		for i := range rawVals {
			scanArgs[i] = &rawVals[i]
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return interpreter.NewError(500, fmt.Sprintf("row scan error: %s", err.Error()))
		}

		rowMap := interpreter.NewMap()
		for i, col := range cols {
			val := rawVals[i]
			rowMap.Set(col, goValueToAlcaValue(val))
		}
		results.Elements = append(results.Elements, rowMap)
	}

	return results
}

func goValueToAlcaValue(val interface{}) interpreter.Value {
	if val == nil {
		return interpreter.NULL
	}

	switch v := val.(type) {
	case int64:
		return &interpreter.IntegerValue{Val: v}
	case int:
		return &interpreter.IntegerValue{Val: int64(v)}
	case int32:
		return &interpreter.IntegerValue{Val: int64(v)}
	case float64:
		return &interpreter.FloatValue{Val: v}
	case float32:
		return &interpreter.FloatValue{Val: float64(v)}
	case bool:
		return &interpreter.BoolValue{Val: v}
	case string:
		return &interpreter.StringValue{Val: v}
	case []byte:
		// Convert text byte slices to string, otherwise blob
		return &interpreter.StringValue{Val: string(v)}
	case time.Time:
		return &interpreter.StringValue{Val: v.Format(time.RFC3339)}
	default:
		return &interpreter.StringValue{Val: fmt.Sprintf("%v", v)}
	}
}
