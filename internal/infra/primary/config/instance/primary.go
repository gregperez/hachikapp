package instance

import (
	"jwt/base/internal/infra/primary/config/database"
	"jwt/base/internal/infra/primary/config/prop"
	"jwt/base/internal/initComponent"
)

/*func (s *Subscriber) Start() {
	os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8957")

	ctx := context.Background()

	// Create a client for the Pub/Sub service
	client, err := pubsub.NewClient(ctx, s.prop.Project)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	sub := client.Subscription(s.prop.Name)

	// Pull messages from the subscription

	go func() {
		sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			fmt.Printf("Got message: %q\n", string(msg.Data))
			msg.Ack()
		})
	}()
}
*/

func GetDatabasePersistencePgxInstance() initComponent.IPrimaryProcess {
	return database.NewConnectionPostgresPgx(prop.GetDatabaseProperty())
}

func GetDatabasePersistenceMongoInstance() initComponent.IPrimaryProcess {
	return database.NewConnectionMongo(prop.GetDatabaseMongoProperty())
}
