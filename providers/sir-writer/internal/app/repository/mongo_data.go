package repository

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	lib_mapper "lib-shared/mapper"
	"sir-writer/internal/config"
)

type MongoDataRepository struct {
	Client             *mongo.Client
	DatafastCollection *mongo.Collection
}

func NewMongoDataRepository(client *mongo.Client, cfg config.Config) *MongoDataRepository {
	DatafastCollection := client.Database(cfg.Mongo.Database).Collection("fetch-kioscos")
	return &MongoDataRepository{Client: client, DatafastCollection: DatafastCollection}
}
func (receiver *MongoDataRepository) SaveBulkModel(payments []lib_mapper.Payment) error {
	models := make([]mongo.WriteModel, 0)
	for _, payment := range payments {
		models = append(models, mongo.
			NewUpdateOneModel().
			SetFilter(bson.D{{"uniqueId", payment.UniqueId}}).
			SetUpdate(bson.M{"$set": payment}).SetUpsert(true))
	}
	opts := options.BulkWrite().SetOrdered(true)
	// Runs a bulk write operation for the specified write operations
	_, err := receiver.DatafastCollection.BulkWrite(context.Background(), models, opts)
	if err != nil {
		return err
	}
	return nil
}
