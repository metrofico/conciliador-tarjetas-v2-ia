package repository

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"kioscos-services/internal/config"
	lib_mapper "lib-shared/mapper"
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

func (receiver *MongoDataRepository) FindPaymentsHash(paymentsIds []string) ([]*MerchantPaymentHash, error) {
	filter := bson.D{{"uniqueId", bson.D{{"$in", paymentsIds}}}}
	projection := bson.D{
		{"uniqueId", 1}, // Incluir el campo 'uniqueId'
		{"hash", 1},     // Incluir el campo 'hash'
	}
	cursor, err := receiver.DatafastCollection.Find(context.Background(), filter, options.Find().SetProjection(projection))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
	var paymentsHash []*MerchantPaymentHash
	for cursor.Next(context.Background()) {
		var currentPaymentHash *MerchantPaymentHash
		if err := cursor.Decode(&currentPaymentHash); err != nil {
			return nil, err
		}
		paymentsHash = append(paymentsHash, currentPaymentHash)
	}
	return paymentsHash, nil
}
