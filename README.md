Library is used for Golang programing language to connect to Goldeneye Hubs System.

## Install
1. Download `gsc-services.json` from here (comming soon).

2. Copy `gsc-services.json` and [`gsc-core.so`](https://raw.githubusercontent.com/gecosys/gsc-go/master/gsc-core.so) to the same folder with executable file (binary file).

3. Install library: ```go get https://github.com/gecosys/gsc-go```

4. Install dependency: ```go get https://github.com/golang/protobuf```

## Example
```golang
package main

import (
	"fmt"
	"time"

	"github.com/gecosys/gsc-go/gelib"
	"github.com/golang/protobuf/proto"
)

func main() {
	// Other connections will send messages to yours by this aliasName
	const aliasName = {Your aliasname}

	client, err := gelib.GetClient()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Open connection to Goldeneye Hubs system with the aliasName
	err = client.OpenConn(aliasName)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Send message every 3 seconds
	go func(client gelib.GEHClient) {
		for range time.Tick(3000 * time.Millisecond) {
			err := client.SendMessage(client.GetAliasName(), []byte("Goldeneye Technologies"))
			if err != nil {
				fmt.Println(err)
			}
		}
	}(client)

	// Listen comming messages
	channel := client.Listen()
	for data := range channel {
		var msg gelib.Data
		err := proto.Unmarshal(data, &msg)
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Printf("Sender: %s\n", msg.Sender)
		fmt.Printf("Data: %s\n", string(msg.Data))
		fmt.Printf("Timestamp: %d\n\n", msg.Timestamp)
	}
}
```
