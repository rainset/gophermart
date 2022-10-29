package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	"github.com/rainset/gophermart/internal/config"
	"github.com/rainset/gophermart/internal/storage"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Session struct {
	UserID int
}

type App struct {
	Config *config.Config
	Router *gin.Engine
	s      storage.Interface
}

func New(storage storage.Interface, c *config.Config) *App {
	return &App{
		s:      storage,
		Config: c,
	}
}

func (a *App) UpdateOrderFromAccrualSystem(orderNumber string) (err error) {
	requestURL := fmt.Sprintf("%s/api/orders/%s", a.Config.AccrualSystemAddress, orderNumber)

	responseBody := struct {
		Order   string  `json:"order"`
		Status  string  `json:"status"`
		Accrual float64 `json:"accrual"`
	}{}

	client := resty.New()
	resp, err := client.R().SetResult(&responseBody).Get(requestURL)
	if err != nil {
		log.Println("UpdateOrderFromAccrualSystem:", err)
		return err
	}

	switch resp.StatusCode() {
	case http.StatusOK:

		err = json.Unmarshal(resp.Body(), &responseBody)
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
		sleepSeconds, _ := strconv.Atoi(resp.Header().Get("Retry-After"))
		if sleepSeconds > 0 {
			time.Sleep(time.Duration(sleepSeconds) * time.Second)
		} else {
			time.Sleep(10 * time.Second)
		}
		return errors.New("TooManyRequests")
	default:
		return errors.New("StatusCodeNotFound")
	}

	return nil
}

func (a *App) UpdateOrderStatusServer() (err error) {

	log.Println("UpdateOrderStatusServer...")
	orders, err := a.s.GetProcessingOrderList()
	if err != nil {
		log.Println("GetProcessingOrderList:", err)
		return err
	}

	var wg sync.WaitGroup
	for _, v := range orders {
		wg.Add(1)
		go func(v storage.OrderTable) {
			defer wg.Done()
			err = a.UpdateOrderFromAccrualSystem(v.Number)
			if err != nil {
				log.Println(err)
			}
		}(v)
	}
	wg.Wait()
	time.Sleep(2 * time.Second)
	err = a.UpdateOrderStatusServer()
	if err != nil {
		log.Println("UpdateOrderStatusServer:", err)
		return err
	}
	return err
}
