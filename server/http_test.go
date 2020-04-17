package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/mihaitodor/ferrum/config"
	"github.com/mihaitodor/ferrum/db"
	. "github.com/smartystreets/goconvey/convey"
)

type mockDBConn struct {
	failPing bool
}

func (c *mockDBConn) Ping() error {
	if c.failPing {
		return errors.New("some error")
	}
	return nil
}
func (*mockDBConn) Close() error { return nil }
func (*mockDBConn) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, nil
}
func (*mockDBConn) PrepareContext(context.Context, string) (*sql.Stmt, error) { return nil, nil }
func (*mockDBConn) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return &sql.Rows{}, nil
}
func (*mockDBConn) QueryRowContext(context.Context, string, ...interface{}) *sql.Row { return nil }

type mockQueries struct {
	Patients []db.Patient
}

func (q *mockQueries) AddPatient(_ context.Context, patient db.AddPatientParams) (db.Patient, error) {
	return db.Patient{
		FirstName: patient.FirstName,
		LastName:  patient.LastName,
		Address:   patient.Address,
		Phone:     patient.Phone,
		Email:     patient.Email,
	}, nil
}
func (q *mockQueries) GetPatient(_ context.Context, id int32) (db.Patient, error) {
	if len(q.Patients) > 0 && q.Patients[0].ID == id {
		return q.Patients[0], nil
	}
	return db.Patient{}, nil
}
func (q *mockQueries) GetPatients(context.Context) ([]db.Patient, error) { return q.Patients, nil }

func Test_HTTPHandlers(t *testing.T) {
	Convey("HTTP handlers test", t, func() {
		c := config.Config{
			HTTPMaxPOSTSize:   102400,
			HTTPJWTVClaimName: "test",
			HTTPJWTSigningKey: "deadbeef",
			HTTPJWTExpiration: 1 * time.Second,
		}

		dbConn := &mockDBConn{}
		queries := &mockQueries{}

		jwt.TimeFunc = func() time.Time {
			return time.Date(
				2020, 4, 17, 0, 0, 0, 0, time.UTC)
		}
		s := Server{
			config:        c,
			databaseConn:  dbConn,
			database:      queries,
			currentTimeFn: jwt.TimeFunc,
		}

		router := s.getHTTPRouter()
		w := httptest.NewRecorder()

		dummyJWTToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhZG1pbiI6dHJ1ZSwiZXhwIjoxNTg3MDgxNjAxLCJuYW1lIjoidGVzdCJ9.2mg7OyC4s_OrZk1KNcSZFDyBdlTNRFLpPdFEzv_XwkI"

		Convey("generateToken should return a valid token", func() {
			s.generateToken(w, nil)

			resp := w.Result()
			So(resp.StatusCode, ShouldEqual, http.StatusOK)

			body, err := ioutil.ReadAll(resp.Body)
			So(err, ShouldBeNil)
			So(string(body), ShouldContainSubstring, dummyJWTToken)
		})

		Convey("healthHandler should", func() {

			Convey("return OK if database pings succeed", func() {
				s.healthHandler(w, nil)

				resp := w.Result()
				So(resp.StatusCode, ShouldEqual, http.StatusOK)

				body, err := ioutil.ReadAll(resp.Body)
				So(err, ShouldBeNil)
				So(string(body), ShouldContainSubstring, "OK")
			})

			Convey("return error if database pings fail", func() {
				dbConn.failPing = true
				s.healthHandler(w, nil)

				resp := w.Result()
				So(resp.StatusCode, ShouldEqual, http.StatusInternalServerError)

				body, err := ioutil.ReadAll(resp.Body)
				So(err, ShouldBeNil)
				So(string(body), ShouldContainSubstring, "Error")
			})
		})

		Convey("patientsHandler should", func() {
			Convey("return OK for", func() {
				Convey("GET requests", func() {
					queries.Patients = []db.Patient{{ID: 123}}

					req := httptest.NewRequest(http.MethodGet, "http://example.com/api/v1/patients", nil)
					s.patientsHandler(w, req)

					resp := w.Result()
					So(resp.StatusCode, ShouldEqual, http.StatusOK)
					body, err := ioutil.ReadAll(resp.Body)
					So(err, ShouldBeNil)
					So(string(body), ShouldContainSubstring, "123")
				})

				Convey("POST requests", func() {
					patient := db.Patient{FirstName: "Bilbo", LastName: "Baggins"}

					postData, err := json.Marshal(patient)
					So(err, ShouldBeNil)

					req := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/patients", bytes.NewReader(postData))
					s.patientsHandler(w, req)

					resp := w.Result()
					So(resp.StatusCode, ShouldEqual, http.StatusCreated)
					body, err := ioutil.ReadAll(resp.Body)
					So(err, ShouldBeNil)
					So(string(body), ShouldContainSubstring, "Baggins")
				})
			})
		})

		Convey("patientHandler should", func() {
			Convey("return OK for", func() {
				Convey("GET requests", func() {
					patientID := int32(123)
					queries.Patients = []db.Patient{{ID: patientID}}

					req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("http://example.com/api/v1/patients/%d", patientID), nil)
					req.Header.Set("Authorization", "Bearer "+dummyJWTToken)
					router.ServeHTTP(w, req)

					resp := w.Result()
					So(resp.StatusCode, ShouldEqual, http.StatusOK)
					body, err := ioutil.ReadAll(resp.Body)
					So(err, ShouldBeNil)
					So(string(body), ShouldContainSubstring, "123")
				})
			})

			Convey("return error", func() {
				Convey("for unauthenticated GET requests", func() {
					patientID := int32(123)
					queries.Patients = []db.Patient{{ID: patientID}}

					req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("http://example.com/api/v1/patients/%d", patientID), nil)
					router.ServeHTTP(w, req)

					resp := w.Result()
					So(resp.StatusCode, ShouldEqual, http.StatusUnauthorized)
				})

				Convey("when the requested patient doesn't exist", func() {
					patientID := int32(123)
					req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("http://example.com/api/v1/patients/%d", patientID), nil)
					router.ServeHTTP(w, req)

					resp := w.Result()
					So(resp.StatusCode, ShouldEqual, http.StatusUnauthorized)
				})
			})
		})
	})
}
