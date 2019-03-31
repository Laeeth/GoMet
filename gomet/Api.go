package gomet

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strconv"
)


type Api struct {
	server *Server
}

func NewApi(server *Server) *Api {

	return &Api{
		server: server,
	}
}

func (s *Api) Start() {
	router := mux.NewRouter()

	router.HandleFunc("/sessions", s.GetSessions).Methods("GET")
	router.HandleFunc("/sessions/{Id}", s.GetSession).Methods("GET")
	router.HandleFunc("/sessions/{Id}", s.CloseSession).Methods("DELETE")
	router.HandleFunc("/sessions/{Id}/{Command}", s.GetSessionCommand).Methods("GET")

	log.Fatal(http.ListenAndServe(s.server.config.Api.Addr, router))
}

func (s *Api) GetSessions(w http.ResponseWriter, r *http.Request) {
	sessions := make([]Session, 0)
	for _, session := range s.server.sessions {
		sessions = append(sessions, *session)
	}
	json.NewEncoder(w).Encode(sessions)
}

func (s *Api) GetSession(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	id, err := strconv.Atoi(params["Id"])
	if err != nil {

	}

	session, err := s.server.GetSession(id)
	json.NewEncoder(w).Encode(session)
}

func (s *Api) GetSessionCommand(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	id, err := strconv.Atoi(params["Id"])
	if err != nil {

	}

	session, err := s.server.GetSession(id)

	command := params["Command"]
	session.RunCommand(&Execute{
		writer: w,
		command: s.server.osCommands[session.Os][command],
	})
}

func (s *Api) CloseSession(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	id, err := strconv.Atoi(params["Id"])
	if err != nil {

	}

	s.server.CloseSession(id)
}