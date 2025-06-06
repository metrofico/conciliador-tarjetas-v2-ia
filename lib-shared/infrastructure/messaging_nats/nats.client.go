package messaging_nats

import (
	"context"
	"errors"
	"fmt"
	"lib-shared/utils"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type NatsStarter struct {
	ManagerDataNats *NatsModuleManager
	EventSender     SenderEventPort
	EventListener   ListenerEventPort
}

type OptsNats struct {
	NameStream string
	Subjects   []string
	MaxAge     time.Duration
}

func NewStartNats(urlNats string, data *OptsNats) *NatsStarter {
	ctx := context.Background()
	for {
		nc, err := nats.Connect(
			urlNats,
			nats.DontRandomize(),
			nats.ReconnectWait(time.Second*3),
			nats.MaxReconnects(-1),
			nats.MaxPingsOutstanding(5),
			nats.PingInterval(10*time.Second),
		)
		if err != nil {
			utils.Error.Println("error al conectar a nats", err)
			time.Sleep(1 * time.Second)
			continue
		}
		js, err := jetstream.New(nc)
		if err != nil {
			utils.Error.Println("error al inicializar jetstream en nats", err)
			time.Sleep(1 * time.Second)
			continue
		}
		utils.Info.Println("connected to JetStream " + urlNats)
		var stream jetstream.Stream
		if data != nil {
			utils.Info.Println("created or updated Stream to JetStream " + urlNats)
			stream, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
				Name:         data.NameStream,
				Subjects:     data.Subjects,
				MaxConsumers: -1,
				Retention:    jetstream.WorkQueuePolicy,
				MaxAge:       data.MaxAge, // 15 días
			})
			if err != nil {
				utils.Error.Println("error al crear el STREAM "+data.NameStream+" en nats", err)
				time.Sleep(1 * time.Second)
				continue
			}
		}

		DataNats := NewManager(nc, js, stream, ctx)
		Sender := NewSenderEventAdapter(DataNats)
		Listener := NewListenerEventAdapter(DataNats)
		return &NatsStarter{
			ManagerDataNats: DataNats,
			EventSender:     Sender,
			EventListener:   Listener,
		}
	}
}
func (st NatsStarter) GetMainStream() jetstream.Stream {
	return st.ManagerDataNats.GetMainStream()
}
func (st NatsStarter) CreateIfNotExistBucket(bucketName string) jetstream.KeyValue {
	js := st.ManagerDataNats.jetStream
	bucket, err := js.KeyValue(context.Background(), bucketName)
	if errors.Is(err, jetstream.ErrBucketNotFound) {
		bucket, err = js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{
			Bucket:      bucketName,
			Description: "",
			TTL:         2 * time.Hour,
			Storage:     jetstream.FileStorage,
			Compression: true,
		})
		if err != nil {
			utils.Error.Println(fmt.Sprintf("error al crear el bucket %s", bucketName), err)
			return nil
		}
		utils.Info.Println(fmt.Sprintf("bucket creado en nats con nombre %s", bucketName))
		return bucket
	}
	if err != nil {
		utils.Error.Println(fmt.Sprintf("error al crear el bucket %s: ", bucketName), err)
		return nil
	}
	utils.Info.Println(fmt.Sprintf("[retomando-bucket] el bucket ya existe devolviendo el Bucket %s", bucketName))
	return bucket
}
func (st NatsStarter) AcquiredLock(bucketName, key string) (bool, error) {
	bucket := st.CreateIfNotExistBucket(bucketName)
	entry, err := bucket.Create(context.Background(), key, []byte(time.Now().String()))
	if errors.Is(err, jetstream.ErrKeyExists) {
		return false, nil // Lock already taken
	}
	if err != nil {
		return false, fmt.Errorf("error trying to acquire lock for %s: %v", key, err)
	}
	utils.Info.Printf("Lock acquired for %s: %v\n", key, entry)
	return true, nil
}
func (st NatsStarter) AcquiredUnlock(bucketName, key string) error {
	bucket := st.CreateIfNotExistBucket(bucketName)
	err := bucket.Delete(context.Background(), key)
	if err != nil {
		return fmt.Errorf("error releasing lock for %s: %v", key, err)
	}
	return nil
}
func (st NatsStarter) GetValueBucket(bucketName, key string) (string, error) {
	bucket := st.CreateIfNotExistBucket(bucketName)
	kve, err := bucket.Get(context.Background(), key)
	if err != nil {
		return "0", fmt.Errorf("error releasing lock for %s: %v", key, err)
	}

	return string(kve.Value()), nil
}
func (st NatsStarter) GetStream(streamName string) jetstream.Stream {
	js := st.ManagerDataNats.jetStream
	stream, err := js.Stream(context.Background(), streamName)
	if err != nil {
		return nil
	}
	return stream
}
func (st NatsStarter) Close() {
	client := st.ManagerDataNats.GetClient()
	if client != nil {
		client.Close()
	}
}
