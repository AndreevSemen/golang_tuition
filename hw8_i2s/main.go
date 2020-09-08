package main

import (
	"fmt"
	"reflect"
)

type Hello struct {
	Hello string
	Bye   string
}

func main() {
	var data = map[string]interface{}{
		"Hello": "greetings",
		"Bye"  : "goodnight",
	}

	var hello = Hello{
		Hello: "hello",
		Bye:   "goodbye",
	}

	rt := reflect.TypeOf(hello)
	rv := reflect.ValueOf(&hello)
	if rv.Type().Kind() == reflect.Ptr {
		rv = rv.Elem()
	}


	for i := 0; i < rt.NumField(); i++ {
		if rt.Field(i).Name == "Hello" {
			rv.Field(i).SetString("Aloha")
		}
	}

	fmt.Printf("%#v\n", hello)

	if err := i2s(data, &hello); err != nil {
		fmt.Printf("error: %s\n", err)
	} else {
		fmt.Printf("result: %#v\n", hello)
	}
}
