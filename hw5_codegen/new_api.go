package main

import "fmt"
import "strings"
import "encoding/json"
import "errors"
import "net/http"
import "net/url"
import "strconv"

// SKIP: *ast.ImportSpec is not ast.TypeSpec

// SKIP: *ast.ImportSpec is not ast.TypeSpec

// SKIP: *ast.ImportSpec is not ast.TypeSpec

// SKIP: *ast.ImportSpec is not ast.TypeSpec

// SKIP: parsing validation error occurred: invalid type: supports string or int type
// SKIP: parsing method error occurred: method Error has no docs
// SKIP: *ast.ValueSpec is not ast.TypeSpec

// SKIP: *ast.ValueSpec is not ast.TypeSpec

// SKIP: *ast.ValueSpec is not ast.TypeSpec

// SKIP: parsing validation error occurred: invalid type: supports string or int type
// SKIP: parsing method error occurred: function NewMyApi is not a method
// SKIP: parsing validation error occurred: invalid type: supports string or int type
// SKIP: parsing validation error occurred: invalid type: supports string or int type
// SKIP: parsing method error occurred: function NewOtherApi is not a method
// SKIP: parsing validation error occurred: invalid type: supports string or int type
// HTTP обертка для структуры *MyApi
func (owner *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/profile":
		owner.WrappedProfile(w, r)
	case "/user/create":
		owner.WrappedCreate(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(
			"{\"error\":\"unknown method\"}",
		))
	}
}
func (owner *MyApi) WrappedProfile(w http.ResponseWriter, r *http.Request) { 

	var in ProfileParams
	var err error
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		in, err = ValidateProfileParams(r.Form)
	} else {
		in, err = ValidateProfileParams(r.URL.Query())
	}

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(
			"{\"error\":\""+err.Error()+"\"}",
		))
		return
	}

	if resp, err := owner.Profile(r.Context(), in); err != nil {
		if err, ok := err.(ApiError); ok {
			w.WriteHeader(err.HTTPStatus)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write([]byte(
			"{\"error\":\""+err.Error()+"\"}",
		))
	} else {
		w.WriteHeader(http.StatusOK)
		
		data, err := json.Marshal(map[string]interface{}{
			"error":    "",
			"response": resp,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}
}
func (owner *MyApi) WrappedCreate(w http.ResponseWriter, r *http.Request) { 
	if r.Header.Get("X-Auth") != "100500" {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(
			"{\"error\": \"unauthorized\"}",
		))
		return
	}
	
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(
			"{\"error\":\"bad method\"}",
		))
		return
	}
	

	var in CreateParams
	var err error
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		in, err = ValidateCreateParams(r.Form)
	} else {
		in, err = ValidateCreateParams(r.URL.Query())
	}

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(
			"{\"error\":\""+err.Error()+"\"}",
		))
		return
	}

	if resp, err := owner.Create(r.Context(), in); err != nil {
		if err, ok := err.(ApiError); ok {
			w.WriteHeader(err.HTTPStatus)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write([]byte(
			"{\"error\":\""+err.Error()+"\"}",
		))
	} else {
		w.WriteHeader(http.StatusOK)
		
		data, err := json.Marshal(map[string]interface{}{
			"error":    "",
			"response": resp,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}
}
// HTTP обертка для структуры *OtherApi
func (owner *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/create":
		owner.WrappedCreate(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(
			"{\"error\":\"unknown method\"}",
		))
	}
}
func (owner *OtherApi) WrappedCreate(w http.ResponseWriter, r *http.Request) { 
	if r.Header.Get("X-Auth") != "100500" {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(
			"{\"error\": \"unauthorized\"}",
		))
		return
	}
	
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(
			"{\"error\":\"bad method\"}",
		))
		return
	}
	

	var in OtherCreateParams
	var err error
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		in, err = ValidateOtherCreateParams(r.Form)
	} else {
		in, err = ValidateOtherCreateParams(r.URL.Query())
	}

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(
			"{\"error\":\""+err.Error()+"\"}",
		))
		return
	}

	if resp, err := owner.Create(r.Context(), in); err != nil {
		if err, ok := err.(ApiError); ok {
			w.WriteHeader(err.HTTPStatus)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write([]byte(
			"{\"error\":\""+err.Error()+"\"}",
		))
	} else {
		w.WriteHeader(http.StatusOK)
		
		data, err := json.Marshal(map[string]interface{}{
			"error":    "",
			"response": resp,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(data)
	}
}
func ValidateProfileParams(query url.Values) (ProfileParams, error) {
	var res ProfileParams

	// Fields declaration
	login := query.Get("login")
	

	// Process Required
	if login == "" {
		return res, errors.New("login must me not empty")
	}

	// Process Enums
	
	// Process Max
	// Process Min

	// Release results
	res.Login = login


	return res, nil
}
func ValidateCreateParams(query url.Values) (CreateParams, error) {
	var res CreateParams

	// Fields declaration
	login := query.Get("login")
	
	full_name := query.Get("full_name")
	
	status := query.Get("status")
	
	age, err := strconv.Atoi(query.Get("age"))
	if err != nil {
		return res, errors.New("age must be int")
	}

	// Process Required
	if login == "" {
		return res, errors.New("login must me not empty")
	}


	if status == "" {
			status = "user"
	}


	// Process Enums
	
	
	statusenum := []string{
		"user", "moderator", "admin", 
	}
	switch status {
	case "user":
	case "moderator":
	case "admin":
	default:
		return res, fmt.Errorf("status must be one of ["+strings.Join(statusenum, ", ")+"]")
	}
	
	
	// Process Max
	if age > 128 {
		return res, fmt.Errorf("age must be <= %d", 128)
	}

	// Process Min
	if len(login) < 10 {
		return res, fmt.Errorf("login len must be >= %d", 10)
	}



	if age < 0 {
		return res, fmt.Errorf("age must be >= %d", 0)
	}

	// Release results
	res.Login = login

	res.Name = full_name

	res.Status = status

	res.Age = age


	return res, nil
}
func ValidateOtherApi(query url.Values) (OtherApi, error) {
	var res OtherApi

	// Fields declaration

	// Process Required
	// Process Enums
	// Process Max
	// Process Min
	// Release results

	return res, nil
}
func ValidateOtherCreateParams(query url.Values) (OtherCreateParams, error) {
	var res OtherCreateParams

	// Fields declaration
	username := query.Get("username")
	
	account_name := query.Get("account_name")
	
	class := query.Get("class")
	
	level, err := strconv.Atoi(query.Get("level"))
	if err != nil {
		return res, errors.New("level must be int")
	}

	// Process Required
	if username == "" {
		return res, errors.New("username must me not empty")
	}


	if class == "" {
			class = "warrior"
	}


	// Process Enums
	
	
	classenum := []string{
		"warrior", "sorcerer", "rouge", 
	}
	switch class {
	case "warrior":
	case "sorcerer":
	case "rouge":
	default:
		return res, fmt.Errorf("class must be one of ["+strings.Join(classenum, ", ")+"]")
	}
	
	
	// Process Max
	if level > 50 {
		return res, fmt.Errorf("level must be <= %d", 50)
	}

	// Process Min
	if len(username) < 3 {
		return res, fmt.Errorf("username len must be >= %d", 3)
	}



	if level < 1 {
		return res, fmt.Errorf("level must be >= %d", 1)
	}

	// Release results
	res.Username = username

	res.Name = account_name

	res.Class = class

	res.Level = level


	return res, nil
}
