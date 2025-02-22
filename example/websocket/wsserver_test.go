package main

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/Allenxuxu/gev/log"

	"github.com/Allenxuxu/gev"
	"github.com/Allenxuxu/gev/connection"
	"github.com/Allenxuxu/gev/plugins/websocket/ws"
	"github.com/Allenxuxu/gev/plugins/websocket/ws/util"
	"github.com/Allenxuxu/toolkit/sync"
	"github.com/Allenxuxu/toolkit/sync/atomic"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/websocket"
)

type wsExample struct {
	ClientNum atomic.Int64
}

func (s *wsExample) OnConnect(c *connection.Connection) {
	s.ClientNum.Add(1)
	//log.Println(" OnConnect ： ", c.PeerAddr())
}
func (s *wsExample) OnMessage(c *connection.Connection, data []byte) (messageType ws.MessageType, out []byte) {
	messageType = ws.MessageText

	switch rand.Int() % 3 {
	case 0:
		out = data
	case 1:
		msg, err := util.PackData(ws.MessageText, data)
		if err != nil {
			panic(err)
		}
		if err := c.Send(msg); err != nil {
			msg, err := util.PackCloseData(err.Error())
			if err != nil {
				panic(err)
			}
			if e := c.Send(msg); e != nil {
				panic(e)
			}
		}
	case 2:
		msg, err := util.PackCloseData("close")
		if err != nil {
			panic(err)
		}
		if e := c.Send(msg); e != nil {
			panic(e)
		}
	}
	return
}

func (s *wsExample) OnClose(c *connection.Connection) {
	s.ClientNum.Add(-1)
	//log.Println("OnClose")
}

func TestWebSocketServer_Start(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	handler := new(wsExample)

	s, err := NewWebSocketServer(handler, &ws.Upgrader{},
		gev.Address(":1834"),
		gev.NumLoops(8))
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(time.Second)
		sw := sync.WaitGroupWrapper{}
		for i := 0; i < 100; i++ {
			sw.AddAndRun(func() {
				startWebSocketClient(s.Options().Address)
			})
		}

		sw.Wait()
		s.Stop()
	}()

	s.Start()
}

func startWebSocketClient(addr string) {
	rand.Seed(time.Now().UnixNano())
	addr = "ws://localhost" + addr
	c, err := websocket.Dial(addr, "", addr)
	if err != nil {
		panic(err)
	}
	defer c.Close()
	duration := time.Duration((rand.Float64()*2+1)*float64(time.Second)) / 8
	start := time.Now()
	for time.Since(start) < duration {
		sz := rand.Int()%(1024*3) + 1
		data := make([]byte, sz)
		if _, err := rand.Read(data); err != nil {
			panic(err)
		}
		if n, err := c.Write(data); err != nil || n != len(data) {
			panic(err)
		}

		data2 := make([]byte, len(data))
		if n, err := io.ReadFull(c, data2); err != nil || n != len(data) {
			if err != io.EOF {
				panic(err)
			} else {
				return
			}
		}
		if !bytes.Equal(data, data2) {
			panic("mismatch")
		}
	}
}

func TestWebSocketServer_CloseConnection(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	handler := new(wsExample)

	s, err := NewWebSocketServer(handler, &ws.Upgrader{},
		gev.Address(":2021"),
		gev.NumLoops(8))
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(time.Second)

		var (
			err     error
			n       = 100
			toClose = 50
			conn    = make([]*websocket.Conn, n)
			addr    = "ws://localhost" + s.Options().Address
		)

		log.SetLevel(log.LevelDebug)
		for i := 0; i < n; i++ {
			conn[i], err = websocket.Dial(addr, "", addr)
			if err != nil {
				panic(fmt.Errorf("%d %s", i, err.Error()))
			}

		}
		assert.Equal(t, n, int(handler.ClientNum.Get()))

		for i := 0; i < toClose; i++ {
			if err := conn[i].Close(); err != nil {
				panic(err)
			}
		}
		time.Sleep(time.Second)
		assert.Equal(t, n-toClose, int(handler.ClientNum.Get()))

		s.Stop()
	}()

	s.Start()
}
