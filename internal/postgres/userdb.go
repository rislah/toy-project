package postgres

import (
	"context"
	"database/sql"

	"github.com/cep21/circuit/v3"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	app "github.com/rislah/fakes/internal"
	"github.com/rislah/fakes/internal/errors"
)

type postgresUserDB struct {
	pg      *sqlx.DB
	circuit *circuit.Circuit
}

var _ app.UserDB = &postgresUserDB{}

func NewUserDB(pg *sqlx.DB, cc *circuit.Circuit) (*postgresUserDB, error) {
	pgUserDB := &postgresUserDB{pg: pg, circuit: cc}
	return pgUserDB, nil
}

func (p *postgresUserDB) CreateUser(ctx context.Context, user app.User) error {
	err := p.circuit.Run(ctx, func(c context.Context) error {
		tx, err := p.pg.BeginTx(ctx, &sql.TxOptions{})
		if err != nil {
			return err
		}

		res := tx.QueryRowContext(ctx, "insert into users (username, password_hash) VALUES ($1, $2) RETURNING user_id", user.Username, user.Password)
		if err != nil {
			return err
		}

		var userID string
		err = res.Scan(&userID)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, "insert into user_role (user_id) VALUES ($1)", userID)
		if err != nil {
			return err
		}

		err = tx.Commit()
		if err != nil {
			return err
		}

		return nil
	})
	return errors.New(err)
}

func (p *postgresUserDB) GetUsers(ctx context.Context) ([]app.User, error) {
	var users []app.User

	err := p.circuit.Run(ctx, func(c context.Context) error {
		err := p.pg.SelectContext(ctx, &users, `
				select u.user_id, u.username, u.password_hash, r.name as role
				from users u 
				inner join user_role ur on u.user_id = ur.user_id
				inner join role r on ur.role_id = r.id`)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, errors.New(err)
	}

	return users, nil
}

func (p *postgresUserDB) GetUserByUsername(ctx context.Context, username string) (app.User, error) {
	var user app.User
	err := p.circuit.Run(ctx, func(c context.Context) error {
		err := p.pg.GetContext(ctx, &user, `
			SELECT u.user_id, u.username, u.password_hash, r.name as role
			FROM users u
			INNER JOIN user_role ur ON u.user_id = ur.user_id
			INNER JOIN role r ON ur.role_id = r.id
			WHERE username = $1
		`, username)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}

		return nil
	})

	if err != nil {
		return app.User{}, errors.New(err)
	}

	return user, nil
}
