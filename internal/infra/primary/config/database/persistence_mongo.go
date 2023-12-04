package database

import (
	"context"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"jwt/base/internal/infra/primary/config/prop"
	"sync"
)

var (
	databaseMongoSingleton *MongoPersistence
	databaseMongoOnce      sync.Once
)

type IMongo interface {
	Save(collection string, args interface{}) (interface{}, error)
}

type MongoPersistence struct {
	pool *mongo.Database
	ctx  context.Context
}

func NewConnectionMongo(prop *prop.DatabaseMongoProperty) *MongoPersistence {
	databaseMongoOnce.Do(func() {
		ctx := context.Background()
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(prop.Url))
		if err != nil {
			panic(err)
		}
		databaseMongoSingleton = &MongoPersistence{client.Database("test"), ctx}
	})
	return databaseMongoSingleton
}

func GetDatabaseMongo() *MongoPersistence {
	return databaseMongoSingleton
}

func (db *MongoPersistence) Start() {
	//TODO implement me
}

func (db *MongoPersistence) Save(collection string, args interface{}) (interface{}, error) {
	data, err := db.pool.Collection(collection).InsertOne(db.ctx, args)
	log.Infof("data: %v", data)
	if err != nil {
		log.Errorf("error in insert, error: %s", err.Error())
		return nil, err
	}
	return data, nil
}
