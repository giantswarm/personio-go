package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	util "github.com/giantswarm/personio-go"
)

// lastToken is the last token HandlePersonioMock() successfully authenticated
type PersonioMock struct {
	lastToken string
}

// authenticate Authenticates a request (valid access tokens: "ghi" and "jkl") and simulates token rotation
func (p *PersonioMock) authenticate(w http.ResponseWriter, req *http.Request) bool {
	// "authenticate"
	token := strings.Replace(req.Header.Get("authorization"), "Bearer ", "", 1)
	if (token != "ghi" && token != "jkl") || token == p.lastToken {
		w.WriteHeader(401)
		return false
	}

	// token rotation
	if token == "ghi" {
		w.Header().Add("authorization", "Bearer jkl")
	} else {
		w.Header().Add("authorization", "Bearer ghi")
	}

	p.lastToken = token
	return true
}

// PersonioMockHandler is a simple handler that emulates parts of the Personio API with anonymous fake data for testing
func (p *PersonioMock) PersonioMockHandler(w http.ResponseWriter, req *http.Request) {

	method := req.Method
	path := req.URL.Path
	if method == http.MethodPost && (path == "/auth" || path == "/auth/") {

		err := req.ParseForm()
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		} else if req.FormValue("client_id") == "abc" && req.FormValue("client_secret") == "def" {
			_, _ = io.WriteString(w, "{\"success\": true, \"data\": { \"token\": \"ghi\" } }")
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	} else if method == http.MethodGet && (path == "/company/time-offs" || path == "/company/time-offs/") {

		if !p.authenticate(w, req) {
			return
		}

		timeOffsData, err := os.ReadFile(filepath.Join("testdata", "time-offs-body.json"))
		if err != nil {
			fmt.Printf("Failed to read time-offs test data file: %s\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		type timeOffsResultBody struct {
			Success bool `json:"success"`
			Error   struct {
				Code    int    `json:"code,omitempty"`
				Message string `json:"message,omitempty"`
			} `json:"error,omitempty"`
			Data []timeOffContainer `json:"data"`
		}

		var result timeOffsResultBody
		err = json.Unmarshal(timeOffsData, &result)
		if err != nil {
			fmt.Printf("Failed to unmarshall time-offs test data file: %s\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		query := req.URL.Query()
		limitArg := query.Get("limit")
		limit, limitErr := strconv.Atoi(limitArg)
		offsetArg := query.Get("offset")
		offset, offsetErr := strconv.Atoi(offsetArg)
		startArg := query.Get("start_date")
		endArg := query.Get("end_date")
		var start time.Time
		var end time.Time
		var errStart error
		var errEnd error
		if startArg != "" {
			start, errStart = time.Parse(queryDateFormat, startArg)
		} else {
			start = time.Time{}
		}

		if endArg != "" {
			end, errEnd = time.Parse(queryDateFormat, endArg)
		} else {
			end = util.PersonioDateMax
		}

		if limitArg == "" {
			limit = pagingMaxLimit
		}

		if errStart != nil || errEnd != nil || end.Before(start) ||
			(limitArg != "" && (limitErr != nil || limit > pagingMaxLimit || limit < 1)) ||
			(offsetArg != "" && (offsetErr != nil || offset < 0)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// remove entries outside range
		filteredTimeOffsResult := timeOffsResultBody{Success: result.Success, Error: result.Error, Data: make([]timeOffContainer, 0)}
		count := 0
		for i := range result.Data {
			offStart := result.Data[i].Attributes.StartDate
			offEnd := result.Data[i].Attributes.EndDate
			// "end" empty and time-off ends after "start"
			// OR overlapping start/end and time-off ranges
			// (empty start and time-off before end is handled implicitly by start being zero == epoch)
			if util.GetTimeIntersection(offStart, offEnd, start, end) >= 0 {
				if count >= offset {
					filteredTimeOffsResult.Data = append(filteredTimeOffsResult.Data, result.Data[i])
				}
				count++
				if count >= offset+limit {
					break
				}
			}
		}

		timeOffResponseBody, err := json.Marshal(filteredTimeOffsResult)
		if err != nil {
			fmt.Printf("Failed to marshall filtered time-offs test data: %s\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_, _ = w.Write(timeOffResponseBody)
	} else if method == http.MethodGet && strings.HasPrefix(path, "/company/employees") {

		if !p.authenticate(w, req) {
			return
		}

		if path == "/company/employees" || path == "/company/employees/" {
			employeesResponseBody, err := os.ReadFile(filepath.Join("testdata", "employees.json"))
			if err != nil {
				fmt.Printf("Failed to read employees test data file: %s\n", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			_, _ = w.Write(employeesResponseBody)
		} else {
			pathSegments := strings.FieldsFunc(path, func(char rune) bool { return char == '/' })
			if len(pathSegments) > 3 {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			id, err := strconv.ParseInt(pathSegments[len(pathSegments)-1], 10, 64)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			employeeResponseBody, err := os.ReadFile(filepath.Join("testdata", fmt.Sprintf("employee-%d.json", id)))
			if err != nil {
				if os.IsNotExist(err) {
					w.WriteHeader(http.StatusNotFound)
				} else {
					fmt.Printf("Failed to read employee %d test data file: %s\n", id, err)
					w.WriteHeader(http.StatusInternalServerError)
				}
				return
			}

			_, _ = w.Write(employeeResponseBody)
		}

	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// testServer is a mocked test server for Personio client testing
// implements io.Closer
type testServer struct {
	mock   PersonioMock
	port   int
	closer io.Closer
}

// Close closes this testServer instance
func (t *testServer) Close() error {
	if t.closer == nil {
		return nil
	}

	return t.closer.Close()
}

// newTestServer creates a new, running test server instance or returns an error
func newTestServer() (testServer, error) {

	mock := PersonioMock{""}

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return testServer{}, err
	}

	go func() {
		srv := &http.Server{
			Handler:           http.HandlerFunc(mock.PersonioMockHandler),
			ReadHeaderTimeout: time.Duration(30) * time.Second,
		}
		_ = srv.Serve(listener)
	}()

	port := listener.Addr().(*net.TCPAddr).Port

	return testServer{mock: PersonioMock{""}, port: port, closer: listener}, nil
}

// makeTime Forces parsing a timestamp in ISO8601 RFC3339 format and returns Time{} on any error
func makeTime(ts string) time.Time {
	t, _ := time.Parse(time.RFC3339, ts)
	return t
}

// makeDuration Forces parsing a duration and returns Duration{} on any error
func makeDuration(duration string) time.Duration {
	d, _ := time.ParseDuration(duration)
	return d
}

type authTestCase struct {
	creds      Credentials
	wantToken  string
	wantStatus int
}

func TestClient_Authenticate(t *testing.T) {
	authTestCases := []authTestCase{
		{creds: Credentials{ClientId: "abc", ClientSecret: "def"}, wantToken: "ghi", wantStatus: 0},
		{creds: Credentials{ClientId: "abc", ClientSecret: "crap"}, wantToken: "", wantStatus: http.StatusUnauthorized},
	}

	server, err := newTestServer()
	if err != nil {
		t.Errorf("Failed to setup mock Personio server: failed to listen: %s", err)
		return
	}

	defer func() {
		_ = server.Close()
	}()

	personioCredentials := Credentials{ClientId: "abc", ClientSecret: "def"}
	personio, err := NewClient(context.TODO(), fmt.Sprintf("http://localhost:%d", server.port), personioCredentials)
	if err != nil {
		t.Errorf("Failed to create Personio API v1 client: %s", err)
		return
	}

	for testNumber, testCase := range authTestCases {

		token, err := personio.Authenticate(testCase.creds.ClientId, testCase.creds.ClientSecret)

		if testCase.wantStatus != 0 {
			if err == nil {
				t.Errorf("[%d] Expected error code %d but none returned", testNumber, testCase.wantStatus)
			} else {
				switch e := err.(type) {
				case Error:
					if e.Status() != testCase.wantStatus {
						t.Errorf("[%d] Expected error code %d but got %d: %s", testNumber, testCase.wantStatus, e.Status(), e)
					}
					err = nil // handled
				}
			}
		}
		if err != nil {
			t.Errorf("[%d] Failed to authenticate: %s", testNumber, err)
			continue
		}

		if testCase.wantToken != token {
			t.Errorf("[%d] Expected access token to be \"%s\", got \"%s\"", testNumber, testCase.wantToken, token)
		}
	}
}

type employeeTestCase struct {
	id             *int64
	wantId         int64
	wantHttpStatus int
	wantFixSalary  float64
	wantEmail      string
	wantHireDate   time.Time
}

func TestClient_GetEmployee(t *testing.T) {
	var employeeElGonzo int64 = 6205887
	var employeeMegaHui int64 = 7161253
	var employeeNada int64 = 0xdeadbeef
	employeeCases := []employeeTestCase{
		{id: &employeeElGonzo, wantId: employeeElGonzo, wantFixSalary: 7042.42, wantEmail: "gonzo@giantswarm.io", wantHireDate: makeTime("2022-01-12T00:00:00+01:00"), wantHttpStatus: 0},
		{id: &employeeMegaHui, wantId: employeeMegaHui, wantFixSalary: 5120.50, wantEmail: "mega@giantswarm.io", wantHireDate: makeTime("2022-05-05T00:00:00+02:00"), wantHttpStatus: 0},
		{id: &employeeNada, wantHttpStatus: http.StatusNotFound},
	}

	server, err := newTestServer()
	if err != nil {
		t.Errorf("Failed to setup mock Personio server: failed to listen: %s", err)
		return
	}

	defer func() {
		_ = server.Close()
	}()

	personioCredentials := Credentials{ClientId: "abc", ClientSecret: "def"}
	personio, err := NewClient(context.TODO(), fmt.Sprintf("http://localhost:%d", server.port), personioCredentials)
	if err != nil {
		t.Errorf("Failed to create Personio API v1 client: %s", err)
		return
	}

	for testNumber, testCase := range employeeCases {

		employee, err := personio.GetEmployee(*testCase.id)

		if testCase.wantHttpStatus != 0 {
			if err == nil {
				t.Errorf("[%d] Expected error code %d but none returned", testNumber, testCase.wantHttpStatus)
			} else {
				switch e := err.(type) {
				case Error:
					if e.Status() != testCase.wantHttpStatus {
						t.Errorf("[%d] Expected error code %d but got %d: %s", testNumber, testCase.wantHttpStatus, e.Status(), e)
					}
					err = nil // handled
				}
			}
		}
		if err != nil {
			t.Errorf("[%d] Failed to query employee with ID %d: %s", testNumber, *testCase.id, err)
			continue
		}

		if testCase.wantId != 0 {
			if employee == nil {
				t.Errorf("[%d] Expected employee with ID %d, but employee is nil", testNumber, testCase.wantId)
				continue
			}

			employeeId := employee.GetIntAttribute("id")
			if employeeId == nil || *employeeId != testCase.wantId {
				t.Errorf("[%d] Employee with ID %d not found in employees", testNumber, testCase.wantId)
				continue
			}

			fixSalary := employee.GetFloatAttribute("fix_salary")
			if fixSalary == nil || *fixSalary != testCase.wantFixSalary {
				t.Errorf("[%d] Expected fix_salary to be %f for employee with ID %d, got %v", testNumber, testCase.wantFixSalary, testCase.wantId, fixSalary)
				continue
			}

			email := employee.GetStringAttribute("email")
			if email == nil || *email != testCase.wantEmail {
				t.Errorf("[%d] Expected email to be %s for employee with ID %d, got %v", testNumber, testCase.wantEmail, testCase.wantId, email)
				continue
			}

			hireDate := employee.GetTimeAttribute("hire_date")
			if hireDate == nil || !hireDate.Equal(testCase.wantHireDate) {
				t.Errorf("[%d] Expected hire_date to be %s for employee with ID %d, got %v", testNumber, testCase.wantHireDate, testCase.wantId, hireDate)
				continue
			}
		}
	}
}

type employeesTestCase struct {
	wantIds []int64
}

func TestClient_GetEmployees(t *testing.T) {

	var employeeElGonzo int64 = 6205887
	var employeeMegaHui int64 = 7161253
	employeeCases := []employeesTestCase{
		{wantIds: []int64{employeeElGonzo, employeeMegaHui}},
	}

	server, err := newTestServer()
	if err != nil {
		t.Errorf("Failed to setup mock Personio server: failed to listen: %s", err)
		return
	}

	defer func() {
		_ = server.Close()
	}()

	personioCredentials := Credentials{ClientId: "abc", ClientSecret: "def"}
	personio, err := NewClient(context.TODO(), fmt.Sprintf("http://localhost:%d", server.port), personioCredentials)
	if err != nil {
		t.Errorf("Failed to create Personio API v1 client: %s", err)
		return
	}

	for testNumber, testCase := range employeeCases {

		employees, err := personio.GetEmployees()
		if err != nil {
			t.Errorf("[%d] Failed to query all employees: %s", testNumber, err)
			continue
		}

		if len(testCase.wantIds) != len(employees) {
			t.Errorf("[%d] Expected %d employees, got %d", testNumber, len(testCase.wantIds), len(employees))
			continue
		}

		for _, id := range testCase.wantIds {
			found := false
			for i := range employees {
				employeeId := employees[i].GetIntAttribute("id")
				if employeeId != nil && *employeeId == id {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("[%d] Employee with ID %d not found in result", testNumber, id)
				break
			}
		}
	}
}

type timeOffTestCase struct {
	start   *time.Time
	end     *time.Time
	wantIds []int64
}

func TestClient_GetTimeOffs(t *testing.T) {

	tsTooEarly := makeTime("1971-01-01T00:00:00Z")
	tsEarly := makeTime("2022-09-06T05:00:00Z")
	tsMiddle := makeTime("2022-09-08T06:00:00Z")
	tsMiddlePlus6h := tsMiddle.Add(makeDuration("6h"))
	tsLate := makeTime("2022-09-10T05:00:00Z")
	tsTooLate := makeTime("2022-09-16T03:00:00Z")
	timeOffCases := []timeOffTestCase{
		{start: nil, end: nil, wantIds: []int64{125814620, 125682392}},
		{start: nil, end: &tsTooEarly, wantIds: []int64{}},
		{start: nil, end: &tsEarly, wantIds: []int64{125814620}},
		{start: &tsLate, end: &util.PersonioDateMax, wantIds: []int64{125682392}},
		{start: &tsMiddle, end: &tsMiddlePlus6h, wantIds: []int64{125682392, 125814620}},
		{start: &tsTooLate, end: nil, wantIds: []int64{}},
	}

	server, err := newTestServer()
	if err != nil {
		t.Errorf("Failed to setup mock Personio server: failed to listen: %s", err)
		return
	}

	defer func() {
		_ = server.Close()
	}()

	personioCredentials := Credentials{ClientId: "abc", ClientSecret: "def"}
	personio, err := NewClient(context.TODO(), fmt.Sprintf("http://localhost:%d", server.port), personioCredentials)
	if err != nil {
		t.Errorf("Failed to create Personio API v1 client: %s", err)
		return
	}

	for testNumber, testCase := range timeOffCases {
		timeOffs, err := personio.GetTimeOffs(testCase.start, testCase.end, 0, 1)
		if err != nil {
			t.Errorf("[%d] Failed to query time-offs: %s", testNumber, err)
			continue
		}
		timeOffs2, err := personio.GetTimeOffs(testCase.start, testCase.end, 1, 1)
		if err != nil {
			t.Errorf("[%d] Failed to query time-offs: %s", testNumber, err)
			continue
		}

		totalLength := len(timeOffs) + len(timeOffs2)
		if len(testCase.wantIds) != totalLength {
			t.Errorf("[%d] Expected %d time-offs, got %d", testNumber, len(testCase.wantIds), totalLength)
			continue
		}

		for _, id := range testCase.wantIds {
			found := false
			for i := range timeOffs {
				if timeOffs[i].Id == id {
					found = true
					break
				}
			}
			for i := range timeOffs2 {
				if timeOffs2[i].Id == id {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("[%d] Time-off with ID %d not found in time-offs", testNumber, id)
				break
			}
		}
	}
}
