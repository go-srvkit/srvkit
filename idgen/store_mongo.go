// Copyright © 2022 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package idgen

import (
	"context"
	"fmt"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

const CollectionName = "uuid"

// Counter 计数器
type Counter struct {
	Label string `bson:"label"`   // 识别符
	Value int64  `bson:"counter"` // 计数器
	Step  int32  `bson:"step" `   // 步长
}

// MongoDBCounter 使用MongoDB的自增计数器
type MongoDBCounter struct {
	guard sync.Mutex //
	uri   string     // 连接uri
	db    string     // DB名称
	label string     // 识别符
	step  int32
	cli   *mongo.Client //
}

func NewMongoDBCounter(uri, db, label string, step int32) CounterStore {
	return &MongoDBCounter{
		uri:   uri,
		db:    db,
		label: label,
		step:  step,
	}
}

func (s *MongoDBCounter) Init(ctx context.Context) error {
	if err := s.createConn(ctx); err != nil {
		return err
	}
	return s.initIndex(ctx)
}

func (s *MongoDBCounter) initIndex(ctx context.Context) error {
	var collection = s.cli.Database(s.db).Collection(CollectionName,
		options.Collection().SetWriteConcern(writeconcern.Majority()))
	index := mongo.IndexModel{
		Keys:    bson.D{{Key: "label", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	if _, err := collection.Indexes().CreateOne(ctx, index); err != nil {
		return err
	}
	return nil
}

func (s *MongoDBCounter) createConn(ctx context.Context) error {
	clientOpts := options.Client().ApplyURI(s.uri)
	client, err := mongo.Connect(clientOpts)
	if err != nil {
		return err
	}
	if err = client.Ping(ctx, nil); err != nil {
		return err
	}
	s.cli = client
	return nil
}

func (s *MongoDBCounter) Close() error {
	s.guard.Lock()
	s.cli.Disconnect(context.Background())
	s.cli = nil
	s.guard.Unlock()
	return nil
}

func (s *MongoDBCounter) Incr(ctx context.Context) (int64, error) {
	s.guard.Lock()
	defer s.guard.Unlock()

	if s.cli == nil {
		return 0, ErrStoreClosed
	}
	var ctr = &Counter{
		Label: s.label,
		Step:  s.step,
	}
	// 把counter自增再读取最新的counter
	if err := s.IncrementAndLoad(ctx, 1, ctr); err != nil {
		return 0, err
	}
	return ctr.Value, nil
}

// IncrementAndLoad 把counter自增再读取最新的counter
// https://docs.mongodb.com/manual/core/write-operations-atomicity/
func (s *MongoDBCounter) IncrementAndLoad(ctx context.Context, n int, ctr *Counter) (ret error) {
	defer func() {
		if val := recover(); val != nil {
			ret = fmt.Errorf("%v", val)
		}
	}()
	var filter = bson.M{"label": s.label}
	var update = bson.M{
		"$setOnInsert": bson.M{
			"label": ctr.Label,
		},
		"$inc": bson.M{"counter": n},
	}

	// https://www.mongodb.com/docs/manual/reference/method/db.collection.findOneAndUpdate/
	// 依赖副本集一致性级别，使用writeconcern=majority避免主从切换导致的ID回退
	var collection = s.cli.Database(s.db).Collection(CollectionName,
		options.Collection().SetWriteConcern(writeconcern.Majority()))
	var result = collection.FindOneAndUpdate(ctx, filter, update,
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After))
	if err := result.Err(); err != nil {
		return err
	}
	if err := result.Decode(ctr); err != nil {
		return err
	}
	return nil
}
