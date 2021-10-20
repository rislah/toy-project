package app_test

import (
	"context"
	"testing"

	app "github.com/rislah/fakes/internal"
	"github.com/rislah/fakes/internal/jwt"
	"github.com/rislah/fakes/internal/local"
	"github.com/stretchr/testify/assert"
)

func TestUserImpl_CreateUser(t *testing.T) {
	tests := []struct {
		name string
		user app.User
		test func(ctx context.Context, t *testing.T, user app.User, userBackend app.UserBackend)
	}{
		{
			name: "should create user",
			user: app.User{
				Username: "user",
				Password: "pw",
			},
			test: func(ctx context.Context, t *testing.T, user app.User, userBackend app.UserBackend) {
				err := userBackend.CreateUser(ctx, user)
				assert.NoError(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userDB, teardown, err := local.MakeUserDB()
			defer teardown()
			assert.NoError(t, err)

			jwtWrapper := jwt.NewHS256Wrapper("wrap")
			svc := app.NewUserBackend(userDB, jwtWrapper)
			test.test(context.Background(), t, test.user, svc)
		})
	}
}