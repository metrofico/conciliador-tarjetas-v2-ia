package repository

import (
	"context"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"lib-shared/reports_models"
	"report-system/internal/config"
)

type MerchantPaymentHash struct {
	UniqueId string `bson:"uniqueId"`
	Hash     string `bson:"hash"`
}

type MongoDataRepository struct {
	Client                *mongo.Client
	ReportCollection      *mongo.Collection
	DataReportsCollection *mongo.Collection
}

func NewMongoDataRepository(client *mongo.Client, cfg config.Config) *MongoDataRepository {
	ReportCollection := client.Database(cfg.Mongo.Database).Collection("reports")
	DataReportsCollection := client.Database(cfg.Mongo.Database).Collection("data-reports")
	return &MongoDataRepository{Client: client, ReportCollection: ReportCollection, DataReportsCollection: DataReportsCollection}
}
func (receiver *MongoDataRepository) CreateReport(report reports_models.ReportConciliator) error {
	_, err := receiver.ReportCollection.InsertOne(context.Background(), report)
	if err != nil {
		return err
	}
	return nil
}
func (receiver *MongoDataRepository) AddToReport(report reports_models.ReportData) error {
	_, err := receiver.DataReportsCollection.InsertOne(context.Background(), report)
	if err != nil {
		return err
	}
	return nil
}
func (receiver *MongoDataRepository) StartedReport(report reports_models.StartedReport) error {
	_, err := receiver.ReportCollection.UpdateOne(context.Background(), bson.M{"conciliatorId": report.ConciliatorId},
		bson.M{"$set": bson.M{
			"startedAt": report.StartedTime,
		}})
	if err != nil {
		return err
	}
	return nil
}
func (receiver *MongoDataRepository) CompletedReport(report reports_models.CompletedReport) error {
	_, err := receiver.ReportCollection.UpdateOne(context.Background(), bson.M{"conciliatorId": report.ConciliatorId},
		bson.M{"$set": bson.M{
			"completedAt": report.CompletedAt,
			"elapsedTime": report.ElapsedTime,
			"entries":     report.Entries,
		}})
	if err != nil {
		return err
	}
	return nil
}

func (receiver *MongoDataRepository) FindById(id string) (*reports_models.ReportConciliator, error) {
	var report *reports_models.ReportConciliator
	err := receiver.ReportCollection.FindOne(context.Background(), bson.M{"conciliatorId": id}).Decode(&report)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("ha ocurrido un error al retornar el Reporte %v", err)
	}

	return report, nil
}
func (receiver *MongoDataRepository) FindDataById(id string) ([]reports_models.ReportDataBson, error) {
	var reports []reports_models.ReportDataBson

	// Crear un filtro para buscar documentos con el conciliatorId proporcionado
	filter := bson.M{"conciliatorId": id}
	findOptions := options.Find().SetSort(bson.M{"createdAt": 1})
	// Ejecutar la consulta Find con el filtro
	cursor, err := receiver.DataReportsCollection.Find(context.Background(), filter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("ha ocurrido un error al buscar los reportes: %v", err)
	}
	defer cursor.Close(context.Background())

	// Iterar sobre el cursor y decodificar cada documento en la slice reports
	for cursor.Next(context.Background()) {
		var report reports_models.ReportDataBson
		if err := cursor.Decode(&report); err != nil {
			return nil, fmt.Errorf("error al decodificar el reporte: %v", err)
		}
		reports = append(reports, report)
	}

	// Verificar si hubo errores durante la iteración
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("error durante la iteración de los reportes: %v", err)
	}

	return reports, nil
}
