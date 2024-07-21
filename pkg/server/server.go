package server

import (
	"bufio"
	"log"
	"net"

	"github.com/catalystcommunity/muddycore/pkg/ids"
)

// Represents a client connection, not a client like entity
type ConnClient struct {
	Id   string
	Name string
	Conn net.Conn
}

// Represents a Server Connection, of which we could have several
// with different ports or encodings, like TCP vs websocket server
type ConnServer struct {
	port    string
	host    string
	clients map[string]ConnClient
}

func NewServer(host, port string) *ConnServer {
	s := ConnServer{host: host, port: port}
	if port == "" {
		s.port = "7777"
	}
	return &s
}

// Attempt to start the server, and if we can't, exit because what else is there?
func (s *ConnServer) StartServer() {
	listen, err := net.Listen("tcp", s.host+":"+s.port)
	if err != nil {
		log.Fatal("Listening has failed: ", err)
	}

	// We can return immediately and do other stuff if we want.
	go func() {
		for {
			conn, err := listen.Accept()

			if err != nil {
				log.Println("Failed to accept new connection: ", err)
			}
			id := ids.RandStringRunes(12)
			newClient := ConnClient{Id: id, Name: "", Conn: conn}
			log.Println("Connection accepted for client: " + newClient.Id)
			s.clients[id] = newClient
			go joinConnection(newClient)
		}
	}()
}

// If we need to cleanup client connections we do it here.
func (s *ConnServer) removeClient(id string) {
	if client, ok := s.clients[id]; ok {
		// We don't care if it fails to close, the point is we tried
		_ = client.Conn.Close()
		delete(s.clients, id)
	}
}
