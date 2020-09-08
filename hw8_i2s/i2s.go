package main

import (
	"fmt"
	"reflect"
)

func i2s(data interface{}, out interface{}) error {
	rt := reflect.TypeOf(out)
	rv := reflect.ValueOf(out)
	if rt.Kind() != reflect.Ptr {
		return fmt.Errorf("out argument must be pointer")
	} else {
		rt = rt.Elem()
		rv = rv.Elem()
	}

	return i2s_impl(data, rt, rv)
}

func i2s_impl(data interface{}, rt reflect.Type, rv reflect.Value) error {
	switch data := data.(type) {
	case map[string]interface{}:
		return m2s(data, rt, rv)
	case []interface{}:
		return s2s(data, rt, rv)
	case string:
		if rv.Kind() != reflect.String {
			return fmt.Errorf("out is not a string")
		}
		rv.SetString(data)
		return nil
	case int:
		if rv.Kind() != reflect.Int {
			return fmt.Errorf("out is not an int")
		}
		rv.SetInt(int64(data))
		return nil
	case float64:
		if rv.Kind() != reflect.Int {
			return fmt.Errorf("out is not an int")
		}
		rv.SetInt(int64(data))
		return nil
	case bool:
		if rv.Kind() != reflect.Bool {
			return fmt.Errorf("out is not an bool")
		}
		rv.SetBool(data)
		return nil
	default:
		return fmt.Errorf("unknown data type: %T", data)
	}
}

func m2s(data map[string]interface{}, rt reflect.Type, rv reflect.Value) error {
	if rt.Kind() != reflect.Struct {
		fmt.Printf("%#v\n", data)
		fmt.Printf("%#v\n", rv)
		fmt.Printf("%#v\n", rt.Kind().String())
		return fmt.Errorf("out argument is not struct")
	}

	if rt.NumField() != len(data) {
		fmt.Printf("data not needed length (%d != %d)", rt.NumField(), len(data))
	}

	for i := 0; i < rt.NumField(); i++ {
		name := rt.Field(i).Name
		v, ok := data[name]
		if !ok {
			return fmt.Errorf("no such field in data: %s", name)
		}
		if err := i2s_impl(v, rt.Field(i).Type, rv.Field(i)); err != nil {
			return err
		}
	}

	return nil
}

func s2s(data []interface{}, rt reflect.Type, rv reflect.Value) error {
	if rt.Kind() != reflect.Slice {
		return fmt.Errorf("out argument is not slice" )
	}

	for i := range data {
		var node = reflect.New(reflect.TypeOf(rv.Interface()).Elem())
		if node.Kind() == reflect.Ptr {
			node = node.Elem()
		}

		err := i2s_impl(data[i], node.Type(), node)
		if err != nil {
			return fmt.Errorf("parsing slice error: %s", err)
		}

		rv.Set(reflect.Append(rv, reflect.ValueOf(node.Interface())))
	}

	return nil
}
