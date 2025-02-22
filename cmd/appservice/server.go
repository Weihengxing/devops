package main

import (
	"flag"
	"github.com/yametech/devops/pkg/api"
	"github.com/yametech/devops/pkg/api/action/appservice"
	"github.com/yametech/devops/pkg/service"
	"github.com/yametech/devops/pkg/store/mongo"
)

var storageUri string

func main() {

	flag.StringVar(&storageUri, "storage_uri", "mongodb://10.200.10.46:27017/devops", "127.0.0.1:3306")
	flag.Parse()

	store, err, errC := mongo.NewMongo(storageUri)
	if err != nil {
		panic(err)
	}

	baseService := service.NewBaseService(store)
	server := api.NewServer(baseService)

	appservice.NewAppServiceServer("appservice", server)

	go func() {
		if err := server.Run(":8080"); err != nil {
			errC <- err
		}
	}()

	if e := <-errC; e != nil {
		panic(e)
	}

}