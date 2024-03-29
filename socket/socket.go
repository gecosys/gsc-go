package socket

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"

	pb "github.com/gecosys/gsc-go/message"
	security "github.com/gecosys/gsc-go/security"

	"github.com/golang/protobuf/proto"
)

// GEHSocket is socket connects to GSCHub
type GEHSocket interface {
	Close()
	GetSecretKey() string
	SetSecretKey(key string)
	SendMessage(data []byte) error
	ListenMessage() chan *pb.Reply
}

// NewSocketClient creates socket connecting to GSCHub
func NewSocketClient(address string) (GEHSocket, error) {
	conn, err := net.Dial("tcp", address)
	client := &socket{
		conn:            conn,
		chanNextMessage: make(chan *pb.Reply),
	}
	return client, err
}

type socket struct {
	conn            net.Conn
	secretKey       string
	chanNextMessage chan *pb.Reply
}

func (s *socket) Close() {
	s.conn.Close()
}

func (s *socket) GetSecretKey() string {
	return s.secretKey
}

func (s *socket) SetSecretKey(key string) {
	s.secretKey = key
}

func (s *socket) SendMessage(data []byte) error {
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(data)))
	buffer := bytes.NewBuffer(header)
	_, err := buffer.Write(data)
	if err != nil {
		return err
	}

	w := bufio.NewWriter(s.conn)
	_, err = w.Write(buffer.Bytes())
	if err != nil {
		return err
	}

	return w.Flush()
}

func (s *socket) ListenMessage() chan *pb.Reply {
	go func() {
		var (
			err      error
			nBytes   int
			bodySize uint32
			data     []byte
			message  *pb.Reply
			header   = make([]byte, 4)
			reader   = bufio.NewReader(s.conn)
		)

	LOOP:
		for {
			if bodySize == 0 { // Get size of body
				nBytes, err = io.ReadAtLeast(reader, header, 4)
				if nBytes != 4 || err != nil { // io.EOF || other errors
					s.Close()
					break LOOP
				}
				bodySize = binary.LittleEndian.Uint32(header)
			}

			data, err = s.getBody(reader, bodySize)
			if err != nil { // io.EOF || other errors
				s.Close()
				break LOOP
			}
			bodySize = 0

			message, err = s.parseMessage(data)
			if err == nil {
				s.chanNextMessage <- message
			}
		}
		close(s.chanNextMessage)
	}()
	return s.chanNextMessage
}

func (s *socket) getBody(reader *bufio.Reader, size uint32) ([]byte, error) {
	var (
		err  error
		body = make([]byte, size)
	)
	_, err = io.ReadAtLeast(reader, body, int(size))
	return body, err
}

func (s *socket) parseMessage(data []byte) (*pb.Reply, error) {
	var (
		err    error
		cipher pb.Cipher
	)

	err = proto.Unmarshal(data, &cipher)
	if err != nil {
		return nil, err
	}

	data = cipher.Data
	if len(cipher.IV) > 0 {
		data, err = security.Decrypt(cipher.IV, data)
		if err != nil {
			return nil, err
		}
	}

	message := new(pb.Reply)
	err = proto.Unmarshal(data, message)
	return message, err
}
