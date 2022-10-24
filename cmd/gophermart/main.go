package main

import (
	"flag"
	"github.com/rainset/gophermart/internal/app"
	"github.com/rainset/gophermart/internal/storage"
	"os"
)

var (
	serverAddress        *string
	databaseDsn          *string
	accrualSystemAddress *string
)

func init() {
	serverAddress = flag.String("a", os.Getenv("RUN_ADDRESS"), "адрес и порт запуска сервиса, string host, ex:[localhost:8080]")
	databaseDsn = flag.String("d", os.Getenv("DATABASE_URI"), "адрес подключения к базе данных, string db connection, ex:[postgres://root:12345@localhost:5432/gophermart]")
	accrualSystemAddress = flag.String("r", os.Getenv("ACCRUAL_SYSTEM_ADDRESS"), "адрес системы расчёта начислений, string db connection, ex:[localhost:8081]")
}

func main() {
	flag.Parse()

	conf := app.Config{
		DatabaseDsn:          *databaseDsn,
		ServerAddress:        *serverAddress,
		AccrualSystemAddress: *accrualSystemAddress,
		SecretKey:            "49a8aca82c132d8d1f430e32be1e6ff3",
		SessionMaxAge:        3600,
		SessionName:          "userID",
	}

	var s storage.Interface = storage.New(*databaseDsn)

	a := app.New(s, conf)

	go a.UpdateOrderStatusServer()

	r := a.NewRouter()
	err := r.Run(conf.ServerAddress)
	if err != nil {
		panic(err)
	}

}
