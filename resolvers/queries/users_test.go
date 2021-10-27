package queries_test

import (
	"testing"

	"github.com/rislah/fakes/internal/local"
	"github.com/rislah/fakes/internal/tests"
)

func TestLocalUsers(t *testing.T) {
	tests.TestUsers(t, local.MakeUserDB, local.MakeRoleDB)
}
