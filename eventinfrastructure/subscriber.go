package eventinfrastructure

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/xuther/go-message-router/common"
	"github.com/xuther/go-message-router/subscriber"
)

type Subscriber struct {
	subscriber          subscriber.Subscriber
	newSubscriptionChan chan SubscriptionRequest
	MessageChan         chan common.Message
}

type SubscriptionRequest struct {
	address string
	filters []string
}

type ConnectionRequest struct {
	PublisherAddr      string `json:"publisher-address"`   // subscribe to my publisher at this address
	SubscriberEndpoint string `json:"subscriber-endpoint"` // hit this endpoint with your publisher address, and I will subscribe to you
	// omitempty
}

/* sooo....
After doing that, if there is a Subscriber Endpoint attached, the handler should respond by sending their publishers address to that endpoint.
*/

func NewSubscriber(requests ...SubscriptionRequest) *Subscriber {
	var s Subscriber
	var err error

	s.subscriber, err = subscriber.NewSubscriber(20)
	if err != nil {
		log.Fatalf("[error] Failed to create subscriber. error: %s", err.Error())
	}

	s.newSubscriptionChan = make(chan SubscriptionRequest, 5)
	go s.addSubscriptions()

	// subscribe to each of the requested addresses
	for _, sr := range requests {
		s.newSubscriptionChan <- sr
	}

	// read messages
	s.MessageChan = make(chan common.Message, 20)
	go s.read()

	return &s
}

func (s *Subscriber) HandleConnectionRequest(cr ConnectionRequest, filters []string, publisherAddr string) error {
	var sr SubscriptionRequest
	if len(cr.PublisherAddr) > 0 && len(filters) > 0 {
		sr.address = cr.PublisherAddr
		sr.filters = filters
		s.newSubscriptionChan <- sr
	} else {
		log.Printf("Request is missing an address to subscribe to and/or filters")
	}

	// respond to Subscriber Endpoint
	if len(cr.SubscriberEndpoint) > 0 && len(publisherAddr) > 0 {
		var response ConnectionRequest
		response.PublisherAddr = publisherAddr

		body, err := json.Marshal(response)
		if err != nil {
			return err
		}

		res, err := http.Post(cr.SubscriberEndpoint, "application/json", bytes.NewBuffer(body))
		if err != nil {
			return err
		}
		// while loop? continue posting until you get a success? maybe only if the server isn't up yet?
		if res.StatusCode != 200 {
			log.Printf("[error] response from %s: %v", cr.SubscriberEndpoint, res)
		}
	}
	return nil
}

func (s *Subscriber) addSubscriptions() {
	for {
		select {
		case request, ok := <-s.newSubscriptionChan:
			if !ok {
				log.Printf("[error] New subscription channel closed")
			}
			// handle request for new subscription
			log.Printf("[subscriber] Starting subscription to %s", request.address)
			err := s.subscriber.Subscribe(request.address, request.filters)
			for err != nil {
				log.Printf("[error] failed to subscribe. Error %s", err)
				log.Printf("trying again in 5 seconds...")
				time.Sleep(5 * time.Second)

				err = s.subscriber.Subscribe(request.address, request.filters)
			}
		}
	}
}

func (s *Subscriber) read() {
	for {
		message := s.subscriber.Read()
		log.Printf("[subscriber] Recieved message: %s", message)
		s.MessageChan <- message
	}
}
