package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"
)

type XMLUser struct {
	Id        int    `xml:"id"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Age       int    `xml:"age"`
	About     string `xml:"about"`
	Gender    string `xml:"gender"`
}

func (xu XMLUser) ToUser() User {
	return User{
		Id:     xu.Id,
		Name:   xu.FirstName + xu.LastName,
		Age:    xu.Age,
		About:  xu.About,
		Gender: xu.Gender,
	}
}

func ProcessQuery(q url.Values) (*SearchRequest, error) {
	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil {
		return nil, err
	}
	if limit < 0 {
		return nil, fmt.Errorf("ErrorBadLimit")
	}

	offset, err := strconv.Atoi(q.Get("offset"))
	if err != nil {
		return nil, err
	}
	if offset < 0 {
		return nil, fmt.Errorf("ErrorBadOffset")
	}
	
	query := q.Get("query")
	switch query {
	case "Name":
	case "About":
	case "":
		query = "Name"
	default:
		return nil, fmt.Errorf("ErrorBadQuery")
	}

	orderBy, err := strconv.Atoi(q.Get("order_by"))
	if err != nil {
		return nil, err
	}
	switch orderBy {
	case OrderByAsc:
		fallthrough
	case OrderByAsIs:
		orderBy = orderAsc
	case OrderByDesc:
		orderBy = orderDesc
	default:
		return nil, fmt.Errorf("ErrorBadOrderBy")
	}
	
	orderField := q.Get("order_field")
	switch orderField {
	case "Id":
	case "Age":
	case "Name":
	case "":
		orderField = "Name"
	default:
		return nil, fmt.Errorf("ErrorBadOrderField")
	}

	req := &SearchRequest{
		Limit:      limit,
		Offset:     offset,
		Query:      query,
		OrderField: orderField,
		OrderBy:    orderBy,
	}

	return req, nil
}


func SearchServer(w http.ResponseWriter, r *http.Request) {
	if token := r.Header.Get("AccessToken"); token != "Token_OK" {
		if token == "InternalError" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if token == "BadErrorJSON" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("{\"Id\":20"))
			return
		}

		if token == "BadBodyJSON" {
			w.Write([]byte("broken"))
			goto Label
		}

		if token == "TimeOut" {
			time.Sleep(1*time.Second)
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	Label:
		func () {}()

	params, err := ProcessQuery(r.URL.Query())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		payLoad, err := json.Marshal(SearchErrorResponse{err.Error()})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(payLoad)
		return
	}

	file, err := os.Open("dataset.xml")
	if err != nil {
		panic(err)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}
	
	var users []User
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil && err != io.EOF {
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if err == io.EOF {
			break
		}
		if token == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		switch token := token.(type) {
		case xml.StartElement:
			if token.Name.Local == "row" {
				user := XMLUser{}
				if err := decoder.DecodeElement(&user, &token); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				users = append(users, user.ToUser())
			}
		case xml.EndElement:
			if token.Name.Local == "root" {
				break
			}
		}
	}

	var intOp func(lhs, rhs int) bool
	var stringOp func(lhs, rhs string) bool
	switch params.OrderBy {
	case orderAsc:
		intOp = func(lhs, rhs int) bool {
			return lhs < rhs
		}
		stringOp = func(lhs, rhs string) bool {
			return lhs < rhs
		}
	case orderDesc:
		intOp = func(lhs, rhs int) bool {
			return lhs > rhs
		}
		stringOp = func(lhs, rhs string) bool {
			return lhs > rhs
		}
	}

	switch params.OrderField {
	case "Id":
		sort.Slice(users, func(i, j int) bool {
			return intOp(users[i].Id, users[j].Id)
		})
	case "Age":
		sort.Slice(users, func(i, j int) bool {
			return intOp(users[i].Age, users[j].Age)
		})
	case "Name":
		sort.Slice(users, func(i, j int) bool {
			return stringOp(users[i].Name, users[j].Name)
		})
	}

	var lBound, rBound int
	if params.Offset > len(users) - 1 {
		lBound = len(users) - 1
	} else {
		lBound = params.Offset
	}
	if params.Offset + params.Limit > len(users) {
		rBound = len(users)
	} else {
		rBound = params.Offset + params.Limit
	}
	payLoad, err := json.Marshal(users[lBound:rBound])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(payLoad)
}

func TestOk(t *testing.T) {
	// expected ok
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))

	client := SearchClient{
		AccessToken: "Token_OK",
		URL:         testServer.URL,
	}
	req := SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      "Name",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	resp, err := client.FindUsers(req)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if len(resp.Users) != 2 {
		t.Errorf("invalid response length: \nexpected: %d\nactual: %d", 2, len(resp.Users))
	}

	if !resp.NextPage {
		t.Error("expected next page")
	}
}

func TestSearchClient_TimedOut(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))

	client := SearchClient{
		AccessToken: "TimeOut",
		URL:         testServer.URL,
	}
	req := SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      "Name",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	resp, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("expected error")
	}

	if resp != nil {
		t.Error("expected nil")
	}
}

func TestSearchRequestParams_NegativeLimit(t *testing.T) {
	//expected fail
	client := SearchClient{}
	req := SearchRequest{
		Limit:      -1,
	}

	resp, err := client.FindUsers(req)

	if err == nil {
		t.Errorf("\necpected error")
	} else if err.Error() != "limit must be > 0" {
		t.Errorf("\nexpected: %s\nactual: %s)", "limit must be > 0", err)
	}
	if resp != nil {
		t.Errorf("response should be nil")
	}
}

func TestSearchRequestParams_HighLimit(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))

	client := SearchClient{
		AccessToken: "Token_OK",
		URL:         testServer.URL,
	}
	req := SearchRequest{
		Limit:      35,
		Offset:     10,
		Query:      "Name",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	resp, err := client.FindUsers(req)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if len(resp.Users) != 25 {
		t.Errorf("invalid response length: \nexpected: %d\nactual: %d", 25, len(resp.Users))
	}

	if resp.NextPage {
		t.Error("unexpected next page")
	}
}

func TestSearchRequestParams_NegativeOffset(t *testing.T) {
	//expected fail
	client := SearchClient{}
	req := SearchRequest{
		Limit:      20,
		Offset:     -1,
	}

	resp, err := client.FindUsers(req)

	if err == nil {
		t.Errorf("ecpected error")
	} else if err.Error() != "offset must be > 0" {
		t.Errorf("\nexpected: %s\nactual: %s)", "offset must be > 0", err)
	}
	if resp != nil {
		t.Errorf("response should be nil")
	}
}

func TestStatuses_Unauthorized(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))

	client := SearchClient{
		AccessToken: "Token_Fail",
		URL:         testServer.URL,
	}
	req := SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      "Name",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	resp, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("expected error")
	} else if err.Error() != "Bad AccessToken" {
		t.Errorf("expected another error expected: %s\nactual: %s", fmt.Errorf("Bad AccessToken"), err)
	}

	if resp != nil {
		t.Error("expected nil")
	}
}

func TestStatuses_InternalError(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))

	client := SearchClient{
		AccessToken: "InternalError",
		URL:         testServer.URL,
	}
	req := SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      "Name",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	resp, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("expected error")
	} else if err.Error() != "SearchServer fatal error" {
		t.Errorf("expected another error expected: %s\nactual: %s", fmt.Errorf("SearchServer fatal error"), err)
	}

	if resp != nil {
		t.Error("expected nil")
	}
}

func TestStatuses_BadRequest_BadQuery(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))

	client := SearchClient{
		AccessToken: "Token_OK",
		URL:         testServer.URL,
	}
	req := SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      "Last Name",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	resp, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("expected error")
	}

	if resp != nil {
		t.Error("expected nil")
	}

	req.Query = "Name"
	req.OrderField = "Gender"
	resp, err = client.FindUsers(req)
	if err == nil {
		t.Errorf("expected error")
	}

	if resp != nil {
		t.Error("expected nil")
	}
}

func TestBadURL_InvalidURL(t *testing.T) {
	// expected ok
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))

	client := SearchClient{
		AccessToken: "Token_OK",
		URL:         testServer.URL[:10],
	}
	req := SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      "Name",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	resp, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("expected error: %s", err)
	}

	if resp != nil {
		t.Error("expected nil")
	}
}

func TestStatuses_BadRequest_BadErrorJSON(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))

	client := SearchClient{
		AccessToken: "BadErrorJSON",
		URL:         testServer.URL,
	}
	req := SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      "Name",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	resp, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("expected erro")
	}

	if resp != nil {
		t.Error("expected nil")
	}
}

func TestStatuses_BadRequest_BadBodyJSON(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(SearchServer))

	client := SearchClient{
		AccessToken: "BadBodyJSON",
		URL:         testServer.URL,
	}
	req := SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      "Name",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	resp, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("expected error")
	}

	if resp != nil {
		t.Error("expected nil")
	}
}

func TestBadRequest(t *testing.T) {
	client := SearchClient{
		AccessToken: "BadBodyJSON",
		URL:         "",
	}
	req := SearchRequest{
		Limit:      2,
		Offset:     0,
		Query:      "Name",
		OrderField: "Id",
		OrderBy:    OrderByAsc,
	}

	resp, err := client.FindUsers(req)
	if err == nil {
		t.Errorf("expected error")
	}

	if resp != nil {
		t.Error("expected nil")
	}
}
