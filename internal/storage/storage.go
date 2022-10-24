package storage

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v4"
	"log"
	"os"
	"time"
)

type Database struct {
	pgx *pgx.Conn
	ctx context.Context
}

type UserTable struct {
	ID        int
	Login     string
	Password  string
	Balance   float64
	Withdrawn float64
}

type UserBalance struct {
	Balance   float64
	Withdrawn float64
}

const (
	OrderStatusNew       string = "NEW"       // — заказ загружен в систему, но не попал в обработку;
	OrderStatusInvalid          = "INVALID"   //INVALID — система расчёта вознаграждений отказала в расчёте;
	OrderStatusProcessed        = "PROCESSED" //PROCESSED — данные по заказу проверены и информация о расчёте успешно получена.
)

type OrderTable struct {
	ID         int
	UserID     int
	Number     string
	Status     string
	Accrual    float64
	UploadedAt time.Time
}

type WithdrawalTable struct {
	ID          int
	UserID      int
	OrderNumber string
	Sum         float64
	ProcessedAt time.Time
}

func New(dataSourceName string) *Database {
	ctx := context.Background()
	db, err := pgx.Connect(ctx, dataSourceName)
	if err != nil {
		panic(err)
	}

	if err == nil {
		log.Print("DB: connection initialized...")
		err = CreateTables(ctx, db)
		if err != nil {
			log.Println(err)
		}
	}
	return &Database{
		pgx: db,
		ctx: ctx,
	}
}

func CreateTables(ctx context.Context, db *pgx.Conn) error {
	c, ioErr := os.ReadFile("migrations/tables.sql")
	if ioErr != nil {
		log.Println("OS: read file tables: ", ioErr)
		return ioErr
	}
	q := string(c)
	_, err := db.Exec(ctx, q)
	if err != nil {
		log.Println("DB: create tables error: ", err)
		return err
	}
	log.Print("DB: tables created")
	return nil
}

func GetMD5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

func (d *Database) CreateUser(user UserTable) (userID int, err error) {
	var hash = GetMD5Hash(user.Password)
	sql := "INSERT INTO users (login,password) VALUES ($1, $2) RETURNING id"
	err = d.pgx.QueryRow(d.ctx, sql, user.Login, hash).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return userID, ErrorUserAlreadyExists
			}
		}
	}
	return userID, err
}

func (d *Database) GetUserIDByCredentials(login, password string) (userID int, err error) {
	var hash = GetMD5Hash(password)
	sql := "SELECT id FROM users WHERE login = $1 AND password = $2"
	err = d.pgx.QueryRow(d.ctx, sql, login, hash).Scan(&userID)
	if err != nil {
		return userID, ErrorUserCredentials
	}
	return userID, err
}

func (d *Database) CreateOrder(order OrderTable) (err error) {
	var ID int
	sql := "INSERT INTO orders (user_id,number,status,uploaded_at) VALUES ($1, $2, $3, $4) ON CONFLICT (number) DO NOTHING RETURNING id"
	err = d.pgx.QueryRow(d.ctx, sql, order.UserID, order.Number, order.Status, time.Now().UTC()).Scan(&ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrorOrderAlreadyExists
		}
	}
	return err
}

func (d *Database) UpdateOrderByNumber(number string, order OrderTable) (err error) {

	tx, err := d.pgx.Begin(d.ctx)
	defer func() {
		if err == nil {
			_ = tx.Commit(d.ctx)
		} else {
			_ = tx.Rollback(d.ctx)
		}
	}()

	var balance float64
	var orderID int
	var userID int

	if order.Accrual > 0 {

		sql := "SELECT u.id, u.balance, o.id FROM users u JOIN orders o ON u.id = o.user_id WHERE o.number = $1 LIMIT 1"
		err = tx.QueryRow(d.ctx, sql, number).Scan(&userID, &balance, &orderID)
		if err != nil {
			return err
		}

		resultBalance := balance + order.Accrual
		sql = "UPDATE users SET balance=$1 WHERE id=$2"
		_, err = tx.Exec(d.ctx, sql, resultBalance, userID)
		if err != nil {
			return err
		}

		sql = "UPDATE orders SET status=$1,accrual=$2,uploaded_at=$3 WHERE number=$4"
		_, err = tx.Exec(d.ctx, sql, order.Status, order.Accrual, time.Now().UTC(), number)
		if err != nil {
			return err
		}

	} else {
		sql := "UPDATE orders SET status=$1,uploaded_at=$3 WHERE number=$4"
		_, err = tx.Exec(d.ctx, sql, order.Status, time.Now().UTC(), number)
		if err != nil {
			return err
		}
	}

	return err
}

func (d *Database) GetOrderByNumber(number string) (order OrderTable, err error) {
	sql := "SELECT id,user_id,number,status,accrual,uploaded_at FROM orders WHERE number = $1"
	err = d.pgx.QueryRow(d.ctx, sql, number).Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &order.Accrual, &order.UploadedAt)
	return order, err
}

func (d *Database) GetOrdersByUserID(userID int) (orders []OrderTable, err error) {
	sql := "SELECT id,user_id,number,status,accrual,uploaded_at FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC"
	rows, err := d.pgx.Query(d.ctx, sql, userID)
	if err != nil {
		return orders, err
	}
	defer rows.Close()

	for rows.Next() {
		var order OrderTable
		err = rows.Scan(&order.ID, &order.UserID, &order.Number, &order.Status, &order.Accrual, &order.UploadedAt)
		if err != nil {
			return orders, err
		}
		orders = append(orders, order)
	}
	if err = rows.Err(); err != nil {
		return orders, err
	}

	return orders, err
}

func (d *Database) GetProcessingOrderList() (orders []OrderTable, err error) {
	sql := "SELECT number,status FROM orders WHERE status NOT IN ($1,$2)"
	rows, err := d.pgx.Query(d.ctx, sql, OrderStatusProcessed, OrderStatusInvalid)
	if err != nil {
		return orders, err
	}
	defer rows.Close()

	for rows.Next() {
		var order OrderTable
		err = rows.Scan(&order.Number, &order.Status)
		if err != nil {
			return orders, err
		}
		orders = append(orders, order)
	}
	if err = rows.Err(); err != nil {
		return orders, err
	}

	return orders, err
}

func (d *Database) GetUserBalance(userID int) (userBalance UserBalance, err error) {
	sql := "SELECT balance,withdrawn FROM users WHERE id = $1 LIMIT 1"
	err = d.pgx.QueryRow(d.ctx, sql, userID).Scan(&userBalance.Balance, &userBalance.Withdrawn)
	return userBalance, err
}

func (d *Database) CreateUserWithdraw(userID int, orderNumber string, sum float64) (err error) {

	tx, err := d.pgx.Begin(d.ctx)
	defer func() {
		if err == nil {
			_ = tx.Commit(d.ctx)
		} else {
			_ = tx.Rollback(d.ctx)
		}
	}()

	var balance float64
	var withdrawn float64

	s1 := "SELECT balance,withdrawn FROM users WHERE id=$1 LIMIT 1"
	err = tx.QueryRow(d.ctx, s1, userID).Scan(&balance, &withdrawn)
	if err != nil {
		return err
	}

	resultSum := balance - sum
	resultWithdrawn := withdrawn + sum
	if resultSum <= 0 {
		return ErrorUserBalanceWithdraw
	}

	s2 := "UPDATE users SET balance=$1,withdrawn=$2 WHERE id=$3"
	_, err = tx.Exec(d.ctx, s2, resultSum, resultWithdrawn, userID)
	if err != nil {
		return err
	}

	s3 := "INSERT INTO withdrawals (user_id,order_number,sum,processed_at) VALUES ($1, $2, $3, $4)"
	_, err = tx.Exec(d.ctx, s3, userID, orderNumber, sum, time.Now().UTC())
	if err != nil {
		return err
	}

	return err
}

func (d *Database) GetWithdrawListByUserID(userID int) (withdrawals []WithdrawalTable, err error) {
	sql := "SELECT id,user_id,order_number,sum, processed_at FROM withdrawals WHERE user_id = $1 ORDER BY processed_at DESC"
	rows, err := d.pgx.Query(d.ctx, sql, userID)
	if err != nil {
		return withdrawals, err
	}
	defer rows.Close()

	for rows.Next() {
		var withdrawn WithdrawalTable
		err := rows.Scan(&withdrawn.ID, &withdrawn.UserID, &withdrawn.OrderNumber, &withdrawn.Sum, &withdrawn.ProcessedAt)
		if err != nil {
			return withdrawals, err
		}
		withdrawals = append(withdrawals, withdrawn)
	}
	if err := rows.Err(); err != nil {
		return withdrawals, err
	}
	return withdrawals, err
}
