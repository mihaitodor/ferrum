package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/mihaitodor/ferrum/db"
	log "github.com/sirupsen/logrus"
)

type healthPayload struct {
	Version   string `json:"version,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
	Message   string `json:"message,omitempty"`
}

type tokenPayload struct {
	Token string `json:"token,omitempty"`
}

func (s Server) getHTTPRouter() *mux.Router {
	router := mux.NewRouter()
	router.Use(commonMiddleware)

	router.HandleFunc("/health", s.healthHandler).Methods(http.MethodGet)
	router.HandleFunc("/generate-token", s.generateToken).Methods("GET")

	authMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return []byte(s.config.HTTPJWTSigningKey), nil
		},
		SigningMethod: jwt.SigningMethodHS256,
	})
	apiRouter := router.PathPrefix("/api/v1").Subrouter()
	apiRouter.HandleFunc("/patients", corsHandler(jwtHandlerWithNext(authMiddleware, s.patientsHandler))).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)
	apiRouter.HandleFunc("/patients/{id}", jwtHandlerWithNext(authMiddleware, s.patientHandler)).Methods(http.MethodGet)

	return router
}

// SetupHTTPHandlers sets up the server HTTP handlers
func (s Server) SetupHTTPHandlers() {
	http.Handle("/", s.getHTTPRouter())
}

func commonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func corsHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")

		// For OPTIONS requests, we just forward the Access-Control-Request-Headers
		// as Access-Control-Allow-Headers in the reply and return
		if r.Method == http.MethodOptions {
			if headers, ok := r.Header["Access-Control-Request-Headers"]; ok {
				for _, header := range headers {
					w.Header().Add("Access-Control-Allow-Headers", header)
				}
			}

			return
		}

		next(w, r)
	}
}

func jwtHandlerWithNext(authMiddleware *jwtmiddleware.JWTMiddleware, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authMiddleware.HandlerWithNext(w, r, next)
	}
}

func (s Server) generateToken(w http.ResponseWriter, _ *http.Request) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"admin": true,
		"name":  s.config.HTTPJWTVClaimName,
		"exp":   s.currentTimeFn().Add(s.config.HTTPJWTExpiration).Unix(),
	})

	signedToken, err := token.SignedString([]byte(s.config.HTTPJWTSigningKey))
	if err != nil {
		log.Warnf("Failed to sign the JWT token: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	jsonData, err := json.Marshal(tokenPayload{Token: signedToken})
	if err != nil {
		log.Warnf("Failed to serialise JWT token payload to JSON: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, string(jsonData))
}

func (s Server) healthHandler(w http.ResponseWriter, _ *http.Request) {
	payload := healthPayload{
		Version:   s.config.Version,
		BuildDate: s.config.BuildDate,
	}

	if err := s.databaseConn.Ping(); err != nil {
		payload.Message = "Error"

		jsonData, err := json.Marshal(payload)
		if err != nil {
			log.Warnf("Failed to serialise health payload to JSON: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, string(jsonData))
		return
	}

	payload.Message = "OK"
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Warnf("Failed to serialise health payload to JSON: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, string(jsonData))
}

func (s Server) patientsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, done := context.WithTimeout(r.Context(), s.config.HTTPRequestTimeout)
	defer done()

	if r.Method == http.MethodPost {
		if r.ContentLength > s.config.HTTPMaxPOSTSize {
			log.Debugf("Request entity too large: %d bytes", r.ContentLength)
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		}

		decoder := json.NewDecoder(r.Body)

		var patient db.AddPatientParams
		if err := decoder.Decode(&patient); err != nil {
			log.Warnf("Failed to decode new patient data: %v", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		patientRecord, err := s.database.AddPatient(ctx, patient)
		if err != nil {
			log.Warnf("Failed to insert patient data into database: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// TODO: Write a custom marshaller for sql.NullTime
		jsonData, err := json.Marshal(patientRecord)
		if err != nil {
			log.Warnf("Failed to serialise patient data to JSON: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// TODO: Try to determine the scheme from the Origin header, since r.URL
		// does not attempt to populate it
		w.Header().Set("Location", fmt.Sprintf("http://%s%s/%s", r.Host, r.URL.Path, strconv.Itoa(int(patientRecord.ID))))
		w.WriteHeader(http.StatusCreated)

		fmt.Fprint(w, string(jsonData))
	} else if r.Method == http.MethodGet {
		patients, err := s.database.GetPatients(ctx)
		if err != nil {
			log.Warnf("Failed to retrieve patients from the database: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if patients == nil {
			patients = []db.Patient{}
		}

		jsonData, err := json.Marshal(patients)
		if err != nil {
			log.Warnf("Failed to serialise patients to JSON: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		fmt.Fprint(w, string(jsonData))
	} else {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func (s Server) patientHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idString, ok := vars["id"]
	if !ok {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	}

	id, err := strconv.ParseInt(idString, 10, 32)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	}

	ctx, done := context.WithTimeout(r.Context(), s.config.HTTPRequestTimeout)
	defer done()

	patient, err := s.database.GetPatient(ctx, int32(id))
	if err != nil {
		log.Warnf("Failed to retrieve patient %d data from the database: %v", id, err)
		if err == sql.ErrNoRows {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		} else {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		return
	}

	jsonData, err := json.Marshal(patient)
	if err != nil {
		log.Warnf("Failed to serialise patient data to JSON: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, string(jsonData))
}
