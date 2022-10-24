package storage

type Interface interface {
	CreateUser(user UserTable) (userID int, err error)
	GetUserIDByCredentials(login, password string) (userID int, err error)
	CreateOrder(order OrderTable) (err error)
	UpdateOrderByNumber(number string, order OrderTable) (err error)
	GetOrderByNumber(number string) (order OrderTable, err error)
	GetProcessingOrderList() (orders []OrderTable, err error)
	GetOrdersByUserID(userID int) (orders []OrderTable, err error)
	GetUserBalance(userID int) (userBalance UserBalance, err error)
	CreateUserWithdraw(userID int, orderNumber string, sum float64) (err error)
	GetWithdrawListByUserID(userID int) (withdrawals []WithdrawalTable, err error)
}
