package main

import (
	"encoding/gob"
	"flag"
	"github.com/joho/godotenv"
	"github.com/rainset/gophermart/internal/app"
	"github.com/rainset/gophermart/internal/config"
	"github.com/rainset/gophermart/internal/storage"
	"log"
	"os"
)

var (
	serverAddress        *string
	databaseDsn          *string
	accrualSystemAddress *string
)

func init() {

	// регистрация структуры для сессии
	gob.Register(app.Session{})

	// loads values from .env into the system
	if err := godotenv.Load(); err != nil {
		log.Fatalln("No .env file found")
	}

	serverAddress = flag.String("a", os.Getenv("RUN_ADDRESS"), "адрес и порт запуска сервиса, string host, ex:[localhost:8080]")
	databaseDsn = flag.String("d", os.Getenv("DATABASE_URI"), "адрес подключения к базе данных, string db connection, ex:[postgres://root:12345@localhost:5432/gophermart]")
	accrualSystemAddress = flag.String("r", os.Getenv("ACCRUAL_SYSTEM_ADDRESS"), "адрес системы расчёта начислений, string db connection, ex:[localhost:8081]")
}

func main() {
	var err error
	flag.Parse()

	conf := config.New()
	conf.ServerAddress = *serverAddress
	conf.DatabaseDsn = *databaseDsn
	conf.AccrualSystemAddress = *accrualSystemAddress

	var s storage.Interface = storage.New(*databaseDsn)
	a := app.New(s, conf)

	go func() {
		err := a.UpdateOrderStatusServer()
		panic(err)
	}()

	r := a.NewRouter()
	err = r.Run(conf.ServerAddress)
	if err != nil {
		panic(err)
	}

}
