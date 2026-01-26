package main

import (
	"fmt"
	"reflect"

	"github.com/multiversx/mx-sdk-go/blockchain"
)

func main() {
	p, _ := blockchain.NewProxy(blockchain.ArgsProxy{
		ProxyURL: "https://devnet-api.multiversx.com",
	})
	t := reflect.TypeOf(p)
	fmt.Printf("Methods for %v:\n", t)
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		fmt.Printf("- %s %v\n", m.Name, m.Type)
	}
}
