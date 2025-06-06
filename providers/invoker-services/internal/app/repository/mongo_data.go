package repository

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"invoker-services/internal/config"
)

type MerchantPaymentHash struct {
	UniqueId string `bson:"uniqueId"`
	Hash     string `bson:"hash"`
}

type MongoDataRepository struct {
	Client             *mongo.Client
	DatafastCollection *mongo.Collection
}

func NewMongoDataRepository(client *mongo.Client, cfg config.Config) *MongoDataRepository {
	DatafastCollection := client.Database(cfg.Mongo.Database).Collection("invoker-services")
	return &MongoDataRepository{Client: client, DatafastCollection: DatafastCollection}
}

func (receiver MongoDataRepository) Close() {
	receiver.Client.Disconnect(context.Background())
}
