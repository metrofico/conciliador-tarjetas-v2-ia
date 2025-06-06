package repository

import (
	"api-starter-jobs/internal/config"
	"go.mongodb.org/mongo-driver/mongo"
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
	DatafastCollection := client.Database(cfg.Mongo.Database).Collection("fetch-deunapichincha")
	return &MongoDataRepository{Client: client, DatafastCollection: DatafastCollection}
}
