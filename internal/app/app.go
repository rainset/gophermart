package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/rainset/gophermart/internal/storage"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

type App struct {
	Config Config
	Router *gin.Engine
	s      storage.Interface
}

func New(storage storage.Interface, c Config) *App {
	return &App{
		s:      storage,
		Config: c,
	}
}

func (a *App) GetOrderStatusRequest(orderNumber string) (err error) {
	requestURL := fmt.Sprintf("%s/api/orders/%s", a.Config.AccrualSystemAddress, orderNumber)

	resp, err := http.Get(requestURL)
	if err != nil {
		time.Sleep(10 * time.Second)
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		responseBody := struct {
			Order   string  `json:"order"`
			Status  string  `json:"status"`
			Accrual float64 `json:"accrual"`
		}{}

		err = json.Unmarshal(body, &responseBody)
		if err != nil {
			return err
		}
		order := storage.OrderTable{
			Status:  responseBody.Status,
			Accrual: responseBody.Accrual,
		}

		errDB := a.s.UpdateOrderByNumber(orderNumber, order)
		if errDB != nil {
			return errDB
		}

	case http.StatusNoContent:
		return errors.New("StatusNoContent")
	case http.StatusTooManyRequests:
		sleepSeconds, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
		if sleepSeconds > 0 {
			time.Sleep(time.Duration(sleepSeconds) * time.Second)
		} else {
			time.Sleep(10 * time.Second)
		}
		return errors.New("TooManyRequests")
	default:
		time.Sleep(5 * time.Second)
		return errors.New("StatusCodeNotFound")
	}

	return nil
}

func (a *App) UpdateOrderStatusServer() {
	orders, err := a.s.GetProcessingOrderList()
	if err != nil {
		a.UpdateOrderStatusServer()
		return
	}

	for _, v := range orders {
		err = a.GetOrderStatusRequest(v.Number)
		if err != nil {
			log.Println(err)
		}
	}

	time.Sleep(10 * time.Second)
	a.UpdateOrderStatusServer()
}
