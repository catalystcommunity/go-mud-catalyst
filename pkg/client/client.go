package client

import (
    "net"
)

type Client struct {
    host string
    port string
    conn net.Conn
}

func NewClient(host, port string) Client {
    c := Client{
        host: host,
        port: port,
    }
    if host == "" {
        c.host = "localhost"
    }
    if port == "" {
        c.port = "7777"
    }
    return c
}

func (c *Client) Run() error {
    defer func () {
        if c.conn != nil {
        c.conn.Close()
        }
    }()
    var err error
    c.conn, err = net.Dial("tcp", c.host + ":" + c.port)
    if err != nil {
        return err
    }
    return nil
}
