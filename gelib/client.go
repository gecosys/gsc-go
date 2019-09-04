package gelib

import (
	"errors"
	fmt "fmt"
	"plugin"
	"sync"
)

var pathLib = "."
var onceClient sync.Once
var client GEHClient

// GEHClient is client which communicates with GSCHub
type GEHClient interface {
	OpenConn(aliasName string) error
	Listen() chan []byte
	SendMessage(receiver string, data []byte) error
	RenameConnection(aliasName string) error

	GetID() string
	GetVersion() string
	GetAliasName() string
}

// GetClient returns object GEHClient (singleton)
func GetClient() (GEHClient, error) {
	if client != nil {
		return client, nil
	}

	var err error
	onceClient.Do(func() {
		if client != nil {
			return
		}

		var plug *plugin.Plugin
		plug, err = plugin.Open(fmt.Sprintf("%s/gsc-core.so", pathLib))
		if err != nil {
			return
		}

		var symClient plugin.Symbol
		symClient, err = plug.Lookup("GEHClient")
		if err != nil {
			return
		}

		var ok bool
		client, ok = symClient.(GEHClient)
		if !ok {
			err = errors.New("Unexpected type")
			return
		}
	})
	return client, err
}
