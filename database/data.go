package database

import (
	"context"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"subscriptionbot/apperrors"
	"subscriptionbot/config"
	"time"
)

type SubscriptionRepository interface {
	Disconnect(ctx context.Context) error
	InsertOne(ctx context.Context, subscription *Subscription) (primitive.ObjectID, error)
	UpsertOne(ctx context.Context, subscription *Subscription) (*mongo.UpdateResult, error)
	FindSubscriptionByChatID(ctx context.Context, chatID int64) (*Subscription, error)
	Find(ctx context.Context, filter bson.D) ([]*Subscription, error)
	UpdateByID(ctx context.Context, id primitive.ObjectID, upd *Subscription) (int, error)
	DeleteOne(ctx context.Context, id primitive.ObjectID) error
}

type SubscriptionStorage struct {
	collection *mongo.Collection
	client     *mongo.Client
}

type Subscription struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	ChatId     int64              `bson:"chat_id"`
	Lat        float64            `bson:"lat, required"`
	Lon        float64            `bson:"lon, required"`
	UpdateTime primitive.DateTime `bson:"update_time"`
	Update     *tgbotapi.Update
}

func NewSubscriptionStorage(cfg *config.Config) SubscriptionRepository {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.DbTimeout)*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.DbHost))
	if err != nil {
		log.Fatal(err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal(err)
	}

	coll := client.Database(cfg.Db).Collection(cfg.Collection)

	return &SubscriptionStorage{
		collection: coll,
		client:     client,
	}
}

func (db *SubscriptionStorage) Disconnect(ctx context.Context) error {
	return db.client.Disconnect(ctx)
}

func (db *SubscriptionStorage) InsertOne(ctx context.Context, subscription *Subscription) (primitive.ObjectID, error) {
	result, err := db.collection.InsertOne(ctx, subscription)
	if err != nil {
		return primitive.NilObjectID, apperrors.MongoDBDataNotFoundErr.AppendMessage(err)
	}

	id, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, apperrors.MongoDBDataNotFoundErr.AppendMessage(err)
	}

	return id, nil
}

func (db *SubscriptionStorage) UpsertOne(ctx context.Context, subscription *Subscription) (*mongo.UpdateResult, error) {
	filter := bson.D{{"chat_id", subscription.ChatId}}
	update := bson.D{{"$set", bson.D{{"lat", subscription.Lat}, {"lon", subscription.Lon},
		{"update_time", subscription.UpdateTime}}}}
	opts := options.Update().SetUpsert(true)
	result, err := db.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return nil, apperrors.MongoDBUpdateErr.AppendMessage(err)
	}

	return result, nil
}

func (db *SubscriptionStorage) FindSubscriptionByChatID(ctx context.Context, chatID int64) (*Subscription, error) {
	var res Subscription
	filter := bson.D{{"chat_id", chatID}}

	err := db.collection.FindOne(ctx, filter).Decode(&res)
	if err == mongo.ErrNoDocuments {
		return nil, apperrors.MongoDBFindOneErr.AppendMessage(err)
	} else if err != nil {
		return nil, apperrors.MongoDBDataNotFoundErr.AppendMessage(err)
	}
	return &res, nil
}

func (db *SubscriptionStorage) Find(ctx context.Context, filter bson.D) ([]*Subscription, error) {
	var res []*Subscription

	cursor, err := db.collection.Find(ctx, filter)
	if err != nil {
		return nil, apperrors.MongoDBFindErr.AppendMessage(err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	err = cursor.All(ctx, &res)
	if err != nil {
		log.Error(err)
		return nil, apperrors.MongoDBCursorErr.AppendMessage(err)
	}

	if err := cursor.Err(); err != nil {
		return nil, apperrors.MongoDBCursorErr.AppendMessage(err)
	}

	if len(res) <= 0 {
		return nil, apperrors.MongoDBDataNotFoundErr.AppendMessage(err)
	}

	return res, nil
}

func (db *SubscriptionStorage) UpdateByID(ctx context.Context, id primitive.ObjectID, upd *Subscription) (int, error) {
	updBson := bson.D{{"lat", upd.Lat}, {"lon", upd.Lon}}
	updRes, err := db.collection.UpdateByID(ctx, id, bson.D{{"$set", updBson}})
	if updRes.MatchedCount == 0 {
		return 0, apperrors.MongoDBUpdateErr.AppendMessage(err)
	}
	if err != nil {
		return 0, apperrors.MongoDBDataNotFoundErr.AppendMessage(err)
	}
	return int(updRes.ModifiedCount), nil
}

func (db *SubscriptionStorage) DeleteOne(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.D{{"_id", id}}
	del, err := db.collection.DeleteOne(ctx, filter)
	if err != nil {
		return apperrors.MongoDBDeleteErr.AppendMessage(err)
	}
	if del.DeletedCount == 0 {
		return apperrors.MongoDBDataNotFoundErr.AppendMessage(err)
	}
	return nil
}
