package client

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gecosys/gsc-go/config"
	pb "github.com/gecosys/gsc-go/message"
	security "github.com/gecosys/gsc-go/security"
	"github.com/gecosys/gsc-go/socket"

	"github.com/golang/protobuf/proto"
)

// Version is version of hub
const Version = "2.2.0"

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
	Listen() (chan *GEHMessage, chan error)
	SendMessage(receiver string, data []byte, isEncrypted bool) error
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
		instance = new(client)
		instance.isOpen = false
	})
	return instance
}

type client struct {
	isOpen              bool
	config              *config.Config
	clientInfo          *pb.Client
	clientTicket        *pb.ClientTicket
	socket              socket.GEHSocket
	isDisconnected      int32
	waitForReconnecting sync.WaitGroup
}

// OpenConn opens connection to GSCHub
func (c *client) OpenConn(aliasName string) error {
	if c.isOpen {
		return nil
	}
	c.isOpen = true

	conf, err := config.GetConfig()
	if err != nil {
		c.isOpen = false
		return err
	}
	c.config = conf

	c.clientInfo = &pb.Client{
		ID:        conf.ID,
		Token:     conf.Token,
		AliasName: aliasName,
	}

	err = c.connect()
	if err != nil {
		c.isOpen = false
		return err
	}
	go c.loopAction()
	return nil
}

func (c *client) GetID() string {
	return c.clientTicket.ConnID
}

func (c *client) GetVersion() string {
	return Version
}

func (c *client) GetAliasName() string {
	return c.clientInfo.AliasName
}

func calcHMAC(key string, data []byte) []byte {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return h.Sum(nil)
}

func (c *client) Listen() (chan *GEHMessage, chan error) {
	var chanError = make(chan error)
	var chanMessage = make(chan *GEHMessage)
	go func(chanMessage chan *GEHMessage, chanError chan error) {
		var chanClientMessage = c.socket.ListenMessage()
		for {
			msg, ok := <-chanClientMessage
			if !ok {
				c.waitForReconnecting.Add(1)
				atomic.StoreInt32(&c.isDisconnected, 1)
				c.waitForReconnecting.Wait()
				chanClientMessage = c.socket.ListenMessage()
				continue
			}

			if c.validateMessage(msg.HMAC, msg.Data) == false {
				chanError <- errors.New("Invalid message")
				continue
			}

			chanMessage <- &GEHMessage{
				Sender:    msg.Sender,
				Data:      msg.Data,
				Timestamp: msg.Timestamp,
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
		if atomic.LoadInt32(&c.isDisconnected) == 1 {
			if c.connect() != nil {
				timer.Reset(timeDuration)
				continue
			}
			if c.ping() == nil {
				atomic.StoreInt32(&c.isDisconnected, 0)
				c.waitForReconnecting.Done()
			}
		} else {
			c.ping()
		}
		timer.Reset(timeDuration)
	}
}

func (c *client) connect() error {
	var (
		err    error
		iv     []byte
		data   []byte
		ticket *pb.Ticket
	)

	// Setup public key + shared key
	err = c.setupSecurity(c.config.Host)
	if err != nil {
		return err
	}

	// Register connection
	ticket, err = c.register(c.config.Host)
	if err != nil {
		return err
	}

	// Build activation message
	data, err = proto.Marshal(ticket.ClientTicket)
	if err != nil {
		return err
	}
	iv, data, err = security.Encrypt(data)

	id, err := security.EncryptRSA([]byte(ticket.ClientTicket.ConnID))
	if err != nil {
		return err
	}

	data, err = proto.Marshal(&pb.CipherTicket{
		ID: id,
		Cipher: &pb.Cipher{
			IV:   iv,
			Data: data,
		},
	})
	if err != nil {
		return err
	}

	// Create and activate socket
	socket, err := socket.NewSocketClient(ticket.Address)
	if err != nil {
		return err
	}
	err = socket.SendMessage(data)
	if err != nil {
		return err
	}

	if c.socket != nil {
		c.socket.Close()
	}
	c.socket = socket
	c.socket.SetSecretKey(ticket.SecretKey)
	c.clientTicket = ticket.ClientTicket

	return nil
}

func (c *client) RenameConnection(aliasName string) error {
	var (
		err  error
		data []byte
	)

	data, err = proto.Marshal(&pb.Client{
		ID:        c.clientTicket.ConnID,
		Token:     c.clientTicket.Token,
		AliasName: aliasName,
	})
	if err != nil {
		return err
	}

	err = c.sendMessage(pb.Letter_Rename, "", data, true)
	if err != nil {
		return err
	}
	c.clientInfo.AliasName = aliasName
	return nil
}

func (c *client) SendMessage(receiver string, data []byte, isEncrypted bool) error {
	return c.sendMessage(pb.Letter_Single, receiver, data, isEncrypted)
}

func (c *client) sendMessage(letterType pb.Letter_Type, receiver string, data []byte, isEncrypted bool) error {
	buffer, err := c.buildMessage(&pb.Letter{
		Type:     letterType,
		Receiver: receiver,
		Data:     data,
	}, isEncrypted)
	if err != nil {
		return err
	}
	return c.socket.SendMessage(buffer)
}

func (c *client) setupSecurity(address string) error {
	httpRes, err := http.Get(fmt.Sprintf("%s/public-key", c.config.Host))
	if err != nil {
		return err
	}

	defer httpRes.Body.Close()

	data, err := ioutil.ReadAll(httpRes.Body)
	if err != nil {
		return err
	}
	var res socket.GEResponse
	err = json.Unmarshal(data, &res)
	if err != nil {
		return err
	}

	if res.ReturnCode != 1 {
		return errors.New(res.Data)
	}

	buffer, err := base64.StdEncoding.DecodeString(res.Data)
	if err != nil {
		return err
	}

	return security.Setup(buffer)
}

func (c *client) register(address string) (*pb.Ticket, error) {
	var (
		err     error
		iv      []byte
		data    []byte
		body    []byte
		httpReq *http.Request
		httpRes *http.Response
		res     socket.GEResponse
	)

	// Build body
	data, err = proto.Marshal(c.clientInfo)
	if err != nil {
		return nil, err
	}
	iv, data, err = security.Encrypt(data)
	if err != nil {
		return nil, err
	}

	key, err := security.GetSharedKey()
	if err != nil {
		return nil, err
	}

	sharedKey := pb.SharedKey{
		Key: key,
		Cipher: &pb.Cipher{
			IV:   iv,
			Data: data,
		},
	}
	body, err = proto.Marshal(&sharedKey)

	// Build request
	httpReq, err = http.NewRequest(
		"POST",
		fmt.Sprintf("%s/conn/register", address),
		bytes.NewBuffer([]byte(base64.StdEncoding.EncodeToString(body))),
	)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Version", Version)
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	httpRes, err = client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpRes.Body.Close()

	// Parse response
	data, err = ioutil.ReadAll(httpRes.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	if res.ReturnCode != 1 {
		return nil, errors.New(res.Data)
	}

	buffer, err := base64.StdEncoding.DecodeString(res.Data)
	if err != nil {
		return nil, err
	}

	var cipher pb.Cipher
	err = proto.Unmarshal(buffer, &cipher)
	if err != nil {
		return nil, err
	}

	data, err = security.Decrypt(cipher.IV, cipher.Data)
	if err != nil {
		return nil, err
	}

	ticket := new(pb.Ticket)
	err = proto.Unmarshal(data, ticket)
	return ticket, nil
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
	return c.sendMessage(pb.Letter_Ping, "", []byte("Ping"), true)
}

func (c *client) buildMessage(letter *pb.Letter, isEncrypted bool) ([]byte, error) {
	buffer, err := proto.Marshal(letter)
	if err != nil {
		return []byte{}, err
	}

	iv := []byte{}
	if isEncrypted {
		iv, buffer, err = security.Encrypt(buffer)
		if err != nil {
			return []byte{}, err
		}
	}

	return proto.Marshal(&pb.Cipher{
		IV:   iv,
		Data: buffer,
	})
}
