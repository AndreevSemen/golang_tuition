package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные
type DBExplorer struct {
	DB     *sql.DB
	Tables map[string]DBTable
}

type DBTable struct {
	Primary string
	Fields  map[string]DBField
}

type DBField struct {
	PrimaryKey bool
	Nullable   bool
	Kind       reflect.Kind
}

func (e *DBExplorer) NewHandlerFuncTablesList() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(
				`{"error": "unknown table"}`,
			))
			return
		}

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(
				`"error":"bad method: expected GET method"`,
			))
			return
		}

		rows, err := e.DB.Query("SHOW TABLES;")
		if err != nil {
			log.Printf("[GET %s] SHOW TABLES error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var tables []string
		for rows.Next() {
			var table string
			err = rows.Scan(&table)
			if err != nil {
				log.Printf("[GET %s] scan table error: %s", r.URL.Path, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			tables = append(tables, table)
		}

		j, err := json.Marshal(map[string]interface{}{
			"response": map[string][]string{
				"tables": tables,
			},
		})
		if err != nil {
			log.Printf("[GET %s] json marshaling error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(j)
	}
}

func (e *DBExplorer) NewHandlerFuncByTable(table string, tableInfo DBTable) func(w http.ResponseWriter, r *http.Request) {
	page := e.NewHandlerFuncTablePage(table, tableInfo)

	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			page(w, r)
		default:
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(
				`{"error":"bad method"}`,
			))
		}
	}
}

func (e *DBExplorer) NewHandlerFuncByIndex(table string, tableInfo DBTable) func(w http.ResponseWriter, r *http.Request) {
	get  := e.NewHandlerFuncItemGet   (table, tableInfo)
	put  := e.NewHandlerFuncItemPut   (table, tableInfo)
	post := e.NewHandlerFuncItemPost  (table, tableInfo)
	del  := e.NewHandlerFuncItemDelete(table, tableInfo)
	
	return func(w http.ResponseWriter, r *http.Request) {
		url := strings.Split(r.URL.Path, "/")

		switch len(url) {
		case 3:
			if url[2] == "" {
				switch r.Method {
				case http.MethodPut:
					put(w, r)
					return
				}
			}
			id, err := strconv.Atoi(url[2])
			if err != nil || id < 0 {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(
					`{"error":"in url pattern '/` + table + `/{id} 'id' must be positive int"}`,
				))
				return
			}

			ctx := context.WithValue(
				r.Context(),
				tableInfo.Primary,
				id,
			)

			switch r.Method {
			case http.MethodGet:
				get(w, r.WithContext(ctx))
			case http.MethodPost:
				post(w, r.WithContext(ctx))
			case http.MethodDelete:
				del(w, r.WithContext(ctx))
			default:
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(
					`{"error":"bad method"}`,
				))
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(
				`{"error":"too long url path"}`,
			))
		}
	}
}

func (e *DBExplorer) NewHandlerFuncTablePage(table string, tableInfo DBTable) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
		if err != nil || limit < 0 {
			limit = 5
		}

		offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
		if err != nil || offset < 0 {
			offset = 0
		}

		rows, err := e.DB.Query("SELECT * FROM `"+table+"` LIMIT ?, ?;", offset, limit)
		if err != nil {
			log.Printf("[GET %s] SELECT error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		records, err := ToJSONInterface(rows)
		if err != nil {
			log.Printf("[GET %s] scaning rows error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		j, err := json.Marshal(map[string]interface{}{
			"response": map[string]interface{}{
				"records": records,
			},
		})
		if err != nil {
			log.Printf("[GET %s] json marshaling error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(j)
	}
}

func (e *DBExplorer) NewHandlerFuncItemGet(table string, tableInfo DBTable) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := r.Context().Value(tableInfo.Primary).(int)
		if !ok {
			log.Printf("GET %s] context value %#v is not int", r.URL.Path, id)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		rows, err := e.DB.Query("SELECT * FROM "+table+" WHERE "+tableInfo.Primary+"=?", id)
		if err != nil {
			log.Printf("[GET %s] SELECT error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var record map[string]interface{}
		if rows.Next() {
			columns, _ := rows.Columns()
			var item = make([]interface{}, len(columns))
			for i := range columns {
				item[i] = &item[i]
			}

			err := rows.Scan(item...)
			if err != nil {
				log.Printf("[GET %s] scan error: %s", r.URL.Path, err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			record = make(map[string]interface{}, )
			for i, col := range columns {
				val := item[i]

				b, ok := val.([]byte)
				var v interface{}
				if (ok) {
					v = string(b)
				} else {
					v = val
				}

				record[col] = v
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(
				`{"error": "record not found"}`,
			))
			return
		}

		j, err := json.Marshal(map[string]interface{}{
			"response": map[string]interface{}{
				"record": record,
			},
		})
		if err != nil {
			log.Printf("[GET %s] json marshaling error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write(j)
	}
}

func (e *DBExplorer) NewHandlerFuncItemPut(table string, tableInfo DBTable) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var data map[string]interface{}
		dec := json.NewDecoder(r.Body)
		err := dec.Decode(&data)
		if err != nil {
			log.Printf("[PUT %s] bad body: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(
				`"error":"bad body"`,
			))
			return
		}

		var fieldTup  []string
		var holderTup []string
		var values    []interface{}

		for k, field := range e.Tables[table].Fields {
			v, ok := data[k]
			if ok {
				if field.PrimaryKey {
					continue
				}

				if field.PrimaryKey ||
					(v == nil && !field.Nullable) ||
					(v != nil && field.Kind != reflect.TypeOf(v).Kind()) {
					log.Printf("nullable")
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte(
						`{"error": "field ` + k + ` have invalid type"}`,
					))
					return
				}
			} else if !field.Nullable {
				switch field.Kind {
				case reflect.Int:
					v = 0
				case reflect.Float64:
					v = 0
				case reflect.String:
					v = ""
				default:
					log.Printf("field %s has unsupported type `%s`", k, field.Kind)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			fieldTup = append(fieldTup, k)
			holderTup = append(holderTup, "?")
			values = append(values, v)
		}

		resp, err := e.DB.Exec("INSERT INTO `"+table+
		                       "` ("+strings.Join(fieldTup, ",")+
		                       ") VALUES("+strings.Join(holderTup, ",")+")",
		                       values...
		)
		if err != nil {
			log.Printf("[PUT %s] INSERT error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		inserted, err := resp.LastInsertId()
		if err != nil {
			log.Printf("[PUT %s] last insert id error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		j, err := json.Marshal(map[string]interface{}{
			"response": map[string]interface{}{
				tableInfo.Primary: inserted,
			},
		})
		if err != nil {
			log.Printf("[PUT %s] json marshaling error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(j)
	}
}

func (e *DBExplorer) NewHandlerFuncItemPost(table string, tableInfo DBTable) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := r.Context().Value(tableInfo.Primary).(int)
		if !ok {
			log.Printf("[POST %s] context value %#v is not int", r.URL.Path, id)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var data map[string]interface{}
		dec := json.NewDecoder(r.Body)
		err := dec.Decode(&data)
		if err != nil {
			log.Printf("[POST %s] bad body: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(
				`"error":"bad body"`,
			))
			return
		}

		var fieldTup  []string
		var holderTup []string
		var values    []interface{}

		for k, field := range e.Tables[table].Fields {
			v, ok := data[k]
			if !ok {
				continue
			}

			if field.PrimaryKey ||
				(v == nil && !field.Nullable) ||
				(v != nil && field.Kind != reflect.TypeOf(v).Kind()) {
				log.Printf("nullable")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(
					`{"error": "field ` + k + ` have invalid type"}`,
				))
				return
			}

			fieldTup  = append(fieldTup, k)
			holderTup = append(holderTup, "?")
			values    = append(values, v)
		}

		sqlFields := strings.Join(fieldTup, "=?, ") + "=?"
		resp, err := e.DB.Exec("UPDATE `"+table+
		                       "` SET "+sqlFields+
		                       " WHERE "+tableInfo.Primary+"=?;", append(values, id)...,
		)
		if err != nil {
			log.Printf("[POST %s] POST error: %s", r.URL.Path, err)
			log.Printf(sqlFields)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		affected, err := resp.RowsAffected()
		if err != nil {
			log.Printf("[POST %s] rows affected error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		j, err := json.Marshal(map[string]interface{}{
			"response": map[string]interface{}{
				"updated": affected,
			},
		})
		if err != nil {
			log.Printf("[POST %s] json marshaling error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(j)
	}
}

func (e *DBExplorer) NewHandlerFuncItemDelete(table string, tableInfo DBTable) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := r.Context().Value(tableInfo.Primary).(int)
		if !ok {
			log.Printf("[DELETE %s] context value %#v is not int", r.URL.Path, id)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp, err := e.DB.Exec("DELETE FROM `"+table+"` WHERE "+tableInfo.Primary+"=?;", id)
		if err != nil {
			log.Printf("[DELETE %s] DELETE error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		affected, err := resp.RowsAffected()
		if err != nil {
			log.Printf("[DELETE %s] rows affected error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		j, err := json.Marshal(map[string]interface{}{
			"response": map[string]interface{}{
				"deleted": affected,
			},
		})
		if err != nil {
			log.Printf("[DELETE %s] json marshaling error: %s", r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(j)
	}
}

func ToJSONInterface(rows *sql.Rows) (interface{}, error) {
	var items []interface{}
	columns, _ := rows.Columns()
	for rows.Next() {
		var item = make([]interface{}, len(columns))
		for i := range columns {
			item[i] = &item[i]
		}

		err := rows.Scan(item...)
		if err != nil {
			return nil, err
		}

		var jsonItem = make(map[string]interface{})
		for i, col := range columns {
			val := item[i]

			b, ok := val.([]byte)
			var v interface{}
			if (ok) {
				v = string(b)
			} else {
				v = val
			}

			jsonItem[col] = v
		}
		items = append(items, jsonItem)
	}

	return items, nil
}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	// Сканем какие есть таблицы
	// Делаем соответствующие роутинги
	// Вызываем соответсвтующие функции для обработки роутов
	rows, err := db.Query("SHOW TABLES;")
	if err != nil {
		return nil, fmt.Errorf("showing tables from DB error: %s", err)
	}

	var tables []string
	for rows.Next() {
		var table string
		err := rows.Scan(&table)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("scaning table from rows error: %s", err)
		}
		tables = append(tables, table)
	}
	rows.Close()

	var explorer = DBExplorer{
		DB:     db,
		Tables: make(map[string]DBTable),
	}
	for _, table := range tables {
		var dbTable = DBTable{
			Fields: make(map[string]DBField),
		}

		rows, err = db.Query("SHOW FULL COLUMNS FROM `"+table+"`;")
		if err != nil {
			return nil, fmt.Errorf("showing columns from %s table error: %s", table, err)
		}

		j, _ := ToJSONInterface(rows)
		arr := j.([]interface{})
		rows.Close()

		for i := range arr {
			var field DBField

			m := arr[i].(map[string]interface{})

			fieldName, ok := m["Field"].(string)
			if !ok {
				log.Printf("value of %#v is not string", m["Field"])
				continue
			}

			switch m["Type"].(string) {
			case "int(11)":
				fallthrough
			case "int":
				field.Kind = reflect.Int
			case "float64":
				field.Kind = reflect.Float64
			case "varchar(255)":
				fallthrough
			case "text":
				field.Kind = reflect.String
			default:
				return nil, fmt.Errorf("type unsupported: %s", m["Type"].(string))
			}

			switch m["Null"].(string) {
			case "YES":
				field.Nullable = true
			case "NO":
				field.Nullable = false
			default:
				return nil, fmt.Errorf("field proprty `Null` unknown: %s", m["Null"].(string))
			}

			if m["Extra"] == "auto_increment" {
				dbTable.Primary  = fieldName
				field.PrimaryKey = true
			}

			dbTable.Fields[fieldName] = field
		}

		explorer.Tables[table] = dbTable
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", explorer.NewHandlerFuncTablesList())
	for table, info := range explorer.Tables {
		log.Printf(fmt.Sprintf("/%s" , table))
		log.Printf(fmt.Sprintf("/%s/", table))
		mux.HandleFunc(fmt.Sprintf("/%s" , table), explorer.NewHandlerFuncByTable(table, info))
		mux.HandleFunc(fmt.Sprintf("/%s/", table), explorer.NewHandlerFuncByIndex(table, info))
	}

	return mux, nil
}
