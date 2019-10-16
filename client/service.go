package client

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	config "gelib/config"
	pb "gelib/protos"
	socket "gelib/socket"

	"github.com/golang/protobuf/proto"
)

// Version is version of hub
const Version = "2.0"

var once sync.Once
var instance *client

// GEHMessage is message received from Goldeneye Hubs System
type GEHMessage struct {
	Sender    string
	Data      []byte
	Timestamp int32
}

// GEHClient is client which communicates with Goldeneye Hubs System
type GEHClient interface {
	OpenConn(aliasName string) error
	Listen() (chan GEHMessage, chan error)
	SendMessage(receiver string, data []byte) error
	RenameConnection(aliasName string) error

	GetID() string
	GetVersion() string
	GetAliasName() string
}

// GetClient returns shared instance of GEHClient
func GetClient() GEHClient {
	if instance != nil {
		return instance
	}
	once.Do(func() {
		if instance != nil {
			return
		}
		instance = new(client)
	})
	return instance
}

type client struct {
	config              *config.Config
	conn                socket.GEConnection
	clientInfo          socket.GEClientInfo
	socket              socket.GEHSocket
	safeConnection      sync.Mutex
	isDisconnected      int32
	waitForReconnecting sync.WaitGroup
}

// OpenConn opens connection to GSCHub
func (c *client) OpenConn(aliasName string) error {
	conf, err := config.GetConfig()
	if err != nil {
		return err
	}
	c.config = conf

	c.conn = socket.GEConnection{
		ID:        conf.ID,
		Token:     conf.Token,
		AliasName: aliasName,
		Ver:       Version,
	}

	err = c.connect()
	if err != nil {
		return err
	}
	go c.loopAction()
	return nil
}

func (c *client) GetID() string {
	return c.clientInfo.ID
}

func (c *client) GetVersion() string {
	return c.conn.Ver
}

func (c *client) GetAliasName() string {
	return c.conn.AliasName
}

func calcHMAC(key string, data []byte) []byte {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return h.Sum(nil)
}

func (c *client) Listen() (chan GEHMessage, chan error) {
	var chanError = make(chan error)
	var chanMessage = make(chan GEHMessage)
	go func(chanMessage chan GEHMessage, chanError chan error) {
		var chanClientMessage = c.socket.ListenMessage()
		for {
			msg, ok := <-chanClientMessage
			if !ok {
				atomic.StoreInt32(&c.isDisconnected, 1)
				c.waitForReconnecting.Add(1)
				c.waitForReconnecting.Wait()
				chanClientMessage = c.socket.ListenMessage()
				continue
			}

			if c.validateMessage(msg.HMAC, msg.Data) == false {
				chanError <- errors.New("Invalid message")
				continue
			}

			data, err := proto.Marshal(&pb.Data{
				Sender:    msg.Sender,
				Data:      msg.Data,
				Timestamp: msg.Timestamp,
			})
			if err != nil {
				chanError <- err
				continue
			}

			var recvMsg pb.Data
			err = proto.Unmarshal(data, &recvMsg)
			if err != nil {
				chanError <- err
				continue
			}
			chanMessage <- GEHMessage{
				Sender:    recvMsg.Sender,
				Data:      recvMsg.Data,
				Timestamp: recvMsg.Timestamp,
			}
		}
	}(chanMessage, chanError)
	return chanMessage, chanError
}

func (c *client) loopAction() {
	var (
		timer        *time.Timer
		timeDuration = 1 * time.Second
	)
	timer = time.NewTimer(timeDuration)

	for range timer.C {
		if atomic.LoadInt32(&c.isDisconnected) == 1 || c.ping() != nil {
			if c.connect() != nil {
				timer.Reset(timeDuration)
				continue
			}
			atomic.StoreInt32(&c.isDisconnected, 0)
			c.waitForReconnecting.Done()
		}
		timer.Reset(timeDuration)
	}
}

func (c *client) connect() error {
	var (
		err  error
		res  socket.GEHostHub
		data []byte
	)

	res, err = c.register(c.config.Host)
	if err != nil {
		return err
	}

	socket, err := socket.NewSocketClient(res.Host)
	if err != nil {
		return err
	}

	data, err = proto.Marshal(&pb.Ticket{
		ConnID: res.ID,
		Token:  res.Token,
	})
	if err != nil {
		return err
	}

	err = socket.SendMessage(data)
	if err != nil {
		return err
	}

	c.safeConnection.Lock()
	if c.socket != nil {
		c.socket.Close()
	}
	c.socket = socket
	c.clientInfo.ID = res.ID
	c.clientInfo.Token = res.Token
	c.safeConnection.Unlock()

	return nil
}

func (c *client) RenameConnection(aliasName string) error {
	var (
		err     error
		data    []byte
		httpRes *http.Response
		body    = new(bytes.Buffer)
		res     socket.GERenameResposne
	)

	json.NewEncoder(body).Encode(socket.GEConnection{
		ID:        c.clientInfo.ID,
		Token:     c.clientInfo.Token,
		AliasName: aliasName,
		Ver:       c.conn.Ver,
	})

	httpRes, err = http.Post(
		fmt.Sprintf("%s/v1/conn/rename", c.config.Host),
		"application/json",
		body,
	)
	if err != nil {
		return err
	}

	defer httpRes.Body.Close()
	data, err = ioutil.ReadAll(httpRes.Body)
	if err != nil {
		return err
	}

	json.NewDecoder(bytes.NewBuffer(data)).Decode(&res)
	if res.ReturnCode < 1 {
		return errors.New(res.Data.(string))
	}
	c.conn.AliasName = aliasName
	return nil
}

func (c *client) SendMessage(receiver string, data []byte) error {
	buffer, err := proto.Marshal(&pb.Letter{
		Type:     pb.Letter_Single,
		Receiver: receiver,
		Data:     data,
	})
	if err != nil {
		return err
	}
	return c.socket.SendMessage(buffer)
}

func (c *client) register(address string) (socket.GEHostHub, error) {
	var (
		err     error
		data    []byte
		httpRes *http.Response
		body    = new(bytes.Buffer)
		res     socket.GERegisterResponse
	)
	json.NewEncoder(body).Encode(c.conn)

	httpRes, err = http.Post(
		fmt.Sprintf("%s/v1/conn/register", address),
		"application/json",
		body,
	)
	if err != nil {
		return res.Data, err
	}

	defer httpRes.Body.Close()
	data, err = ioutil.ReadAll(httpRes.Body)
	if err != nil {
		return res.Data, err
	}

	json.NewDecoder(bytes.NewBuffer(data)).Decode(&res)
	return res.Data, nil
}

func (c *client) validateMessage(hmac, data []byte) bool {
	realHMAC := calcHMAC(
		c.socket.GetSecretKey(),
		data,
	)

	if len(realHMAC) != len(hmac) {
		return false
	}

	for idx, digit := range hmac {
		if digit != realHMAC[idx] {
			return false
		}
	}
	return true
}

func (c *client) ping() error {
	buffer, err := proto.Marshal(&pb.Letter{
		Type: pb.Letter_Ping,
		Data: []byte("Ping"),
	})
	if err != nil {
		return err
	}
	return c.socket.SendMessage(buffer)
}
