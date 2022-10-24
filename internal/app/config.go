package app

type Config struct {
	ServerAddress        string
	DatabaseDsn          string
	AccrualSystemAddress string
	SecretKey            string
	SessionName          string
	SessionMaxAge        int
}
