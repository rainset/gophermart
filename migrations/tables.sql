
CREATE TABLE IF NOT EXISTS users (
                                     id serial PRIMARY KEY,
                                     login    text NOT NULL UNIQUE,
                                     password text NOT NULL,
                                     balance double precision DEFAULT 0,
                                     withdrawn double precision DEFAULT 0
);

CREATE TABLE IF NOT EXISTS orders (
                                     id serial PRIMARY KEY,
                                     user_id    integer NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                     number text NOT NULL UNIQUE,
                                     status text NOT NULL,
                                     accrual double precision DEFAULT 0,
                                     uploaded_at timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS withdrawals (
                                      id serial PRIMARY KEY,
                                      user_id    integer NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                                      order_number text NOT NULL,
                                      sum double precision NOT NULL,
                                      processed_at timestamptz NOT NULL
);
