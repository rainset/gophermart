package app

import (
	"errors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/rainset/gophermart/internal/storage"
	"log"
	"net/http"
	"strconv"
	"time"
)

func (a *App) NewRouter() *gin.Engine {
	r := gin.Default()
	r.Use(gzip.Gzip(gzip.DefaultCompression))

	store := cookie.NewStore([]byte(a.Config.SecretKey))
	store.Options(sessions.Options{MaxAge: a.Config.SessionMaxAge})
	r.Use(sessions.Sessions(a.Config.SessionName, store))

	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError})
		c.AbortWithStatus(http.StatusInternalServerError)
	}))

	r.POST("/api/user/register", a.UserRegisterHandler)
	r.POST("/api/user/login", a.UserLoginHandler)

	r.POST("/api/user/orders", a.AuthMiddleware, a.CreateUserOrderHandler)
	r.GET("/api/user/orders", a.AuthMiddleware, a.GetUserOrdersHandler)

	r.GET("/api/user/balance", a.AuthMiddleware, a.GetUserBalanceHandler)
	r.POST("/api/user/balance/withdraw", a.AuthMiddleware, a.CreateUserWithdrawHandler)
	r.GET("/api/user/withdrawals", a.AuthMiddleware, a.GetUserWithdrawalsHandler)

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError})
	})
	return r
}

func (a *App) AuthMiddleware(c *gin.Context) {
	session := sessions.Default(c)
	sessionHash := session.Get(a.Config.SessionName)
	if sessionHash == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized})
	}
	c.Next()
}

func (a *App) UserRegisterHandler(c *gin.Context) {

	clientData := struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}{}

	err := c.BindJSON(&clientData)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "err": err})
		return
	}

	preparedUser := storage.UserTable{
		Login:    clientData.Login,
		Password: clientData.Password,
	}
	userID, err := a.s.CreateUser(preparedUser)
	if err != nil {
		if errors.Is(err, storage.ErrorUserAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"code": http.StatusConflict})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest})
		return
	}

	if userID > 0 {
		session := sessions.Default(c)
		session.Set(a.Config.SessionName, userID)
		_ = session.Save()
	}

	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK})

}

func (a *App) UserLoginHandler(c *gin.Context) {

	clientData := struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}{}

	err := c.BindJSON(&clientData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest})
		return
	}

	userID, err := a.s.GetUserIDByCredentials(clientData.Login, clientData.Password)
	if err != nil {
		if errors.Is(err, storage.ErrorUserCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest})
		return
	}

	if userID > 0 {
		session := sessions.Default(c)
		session.Set(a.Config.SessionName, userID)
		_ = session.Save()
	}

	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK})
}

func (a *App) CreateUserOrderHandler(c *gin.Context) {

	session := sessions.Default(c)
	sessionUserID := session.Get(a.Config.SessionName).(int)

	var requestOrderNumber int
	err := c.BindJSON(&requestOrderNumber)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1})
		return
	}

	requestOrderNumberStr := strconv.Itoa(requestOrderNumber)
	isValidOrderNumber := ValidateLuhn(requestOrderNumber)
	if !isValidOrderNumber {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": http.StatusUnprocessableEntity})
		return
	}

	orderData := storage.OrderTable{
		UserID: sessionUserID,
		Number: requestOrderNumberStr,
		Status: storage.OrderStatusNew,
	}

	err = a.s.CreateOrder(orderData)

	if err != nil {
		if errors.Is(err, storage.ErrorOrderAlreadyExists) {
			order, _ := a.s.GetOrderByNumber(requestOrderNumberStr)
			if sessionUserID == order.UserID { // заказ есть у текущего пользователя
				c.JSON(http.StatusOK, gin.H{"code": http.StatusOK})
				c.Abort()
				return
			} else { // заказ есть у другого пользователя
				c.JSON(http.StatusConflict, gin.H{"code": http.StatusConflict})
				c.Abort()
				return
			}
		}
	}

	c.JSON(http.StatusAccepted, gin.H{"code": http.StatusAccepted})
}

func (a *App) GetUserOrdersHandler(c *gin.Context) {
	session := sessions.Default(c)
	sessionUserID := session.Get(a.Config.SessionName).(int)

	type ResponseOrderData struct {
		Number     string  `json:"number"`
		Status     string  `json:"status"`
		Accrual    float64 `json:"accrual,omitempty"`
		UploadedAt string  `json:"uploaded_at"`
	}

	orders, err := a.s.GetOrdersByUserID(sessionUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError})
		return
	}
	var result []ResponseOrderData
	for _, v := range orders {
		preparedOrder := ResponseOrderData{
			Number:     v.Number,
			Status:     v.Status,
			Accrual:    v.Accrual,
			UploadedAt: v.UploadedAt.Format(time.RFC3339),
		}
		result = append(result, preparedOrder)
	}
	if len(result) > 0 {
		c.JSON(http.StatusOK, result)
		return
	} else {
		c.JSON(http.StatusNoContent, "")
		return
	}
}

func (a *App) GetUserBalanceHandler(c *gin.Context) {
	session := sessions.Default(c)
	sessionUserID := session.Get(a.Config.SessionName).(int)
	userBalance, err := a.s.GetUserBalance(sessionUserID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError})
		return
	}

	c.JSON(http.StatusOK, gin.H{"current": userBalance.Balance, "withdrawn": userBalance.Withdrawn})

}

func (a *App) CreateUserWithdrawHandler(c *gin.Context) {

	session := sessions.Default(c)
	sessionUserID := session.Get(a.Config.SessionName).(int)

	clientData := struct {
		OrderNumber string  `json:"order"`
		Sum         float64 `json:"sum"`
	}{}

	err := c.BindJSON(&clientData)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest})
		return
	}

	orderNumberInt, _ := strconv.Atoi(clientData.OrderNumber)
	isValidOrderNumber := ValidateLuhn(orderNumberInt)
	if !isValidOrderNumber {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest})
		return
	}

	err = a.s.CreateUserWithdraw(sessionUserID, clientData.OrderNumber, clientData.Sum)

	if err != nil {
		if errors.Is(err, storage.ErrorOrderNotFound) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"code": http.StatusUnprocessableEntity})
			return
		}
		if errors.Is(err, storage.ErrorUserBalanceWithdraw) {
			c.JSON(http.StatusPaymentRequired, gin.H{"code": http.StatusPaymentRequired})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": http.StatusInternalServerError})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK})

}

func (a *App) GetUserWithdrawalsHandler(c *gin.Context) {

	session := sessions.Default(c)
	sessionUserID := session.Get(a.Config.SessionName).(int)

	type ResponseWithrawn struct {
		OrderNumber string  `json:"order"`
		Sum         float64 `json:"sum"`
		ProcessedAt string  `json:"processed_at"`
	}

	withdrawals, _ := a.s.GetWithdrawListByUserID(sessionUserID)

	var result []ResponseWithrawn
	for _, v := range withdrawals {
		preparedOrder := ResponseWithrawn{
			OrderNumber: v.OrderNumber,
			Sum:         v.Sum,
			ProcessedAt: v.ProcessedAt.Format(time.RFC3339),
		}
		result = append(result, preparedOrder)
	}
	if len(result) > 0 {
		c.JSON(http.StatusOK, result)
		return
	} else {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}
}
