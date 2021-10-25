package loaders

import (
	"context"
	"fmt"
	"net/http"

	"github.com/graph-gophers/dataloader"
	"github.com/jmoiron/sqlx"
	app "github.com/rislah/fakes/internal"
)

type contextKey string

const DataloadersKey contextKey = "Dataloaders"

type LoaderDetails struct {
	options     []dataloader.Option
	batchLoadFn dataloader.BatchFunc
}

type Dataloaders struct {
	loaderDetails map[contextKey]LoaderDetails
}

func New(db *sqlx.DB, userDB app.UserDB, userBackend app.UserBackend) Dataloaders {
	return Dataloaders{
		loaderDetails: map[contextKey]LoaderDetails{
			usersByIDs:    newUsersByIDsLoader(db),
			roleByUserID:  newRoleByUserID(userBackend),
			usersByRoleID: newUsersByRoleIDLoader(userDB),
			rolesByNames:  newRolesByNamesLoader(db),
			rolesByIDs:    newRolesByIDsLoader(db),
		},
	}
}

type loadersMap map[contextKey]dataloader.Interface

func (d Dataloaders) Attach(ctx context.Context) context.Context {
	loadersMap := loadersMap{}
	for key, loaderDetails := range d.loaderDetails {
		loadersMap[key] = dataloader.NewBatchedLoader(loaderDetails.batchLoadFn, loaderDetails.options...)
	}

	ctx = context.WithValue(ctx, DataloadersKey, loadersMap)

	return ctx
}

func (d Dataloaders) AttachMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = d.Attach(ctx)
		h.ServeHTTP(rw, r.WithContext(ctx))
	})
}

func extractLoader(ctx context.Context, key contextKey) (dataloader.Interface, error) {
	loaderMap, ok := ctx.Value(DataloadersKey).(loadersMap)
	if !ok {
		return nil, fmt.Errorf("unknown key: %v", key)
	}

	loader, ok := loaderMap[key]
	if !ok {
		return nil, fmt.Errorf("loader not found in key: %v", key)
	}

	return loader, nil
}