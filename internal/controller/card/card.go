/*
Copyright 2020 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package card

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/bmutziu/provider-cards/apis/card/v1alpha1"
	apisv1alpha1 "github.com/bmutziu/provider-cards/apis/v1alpha1"
)

const (
	errNotCard      = "managed resource is not a Card custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"

	errNewClient = "cannot create new Service"
)

// A NoOpService does nothing.
type NoOpService struct{}

type cardCredentials struct {
	Seed int64 `json:"seed"`
}

type deck struct {
	Cards       []v1alpha1.Card
	Credentials []byte
}

var (
	newNoOpService = func(_ []byte) (interface{}, error) { return &NoOpService{}, nil }
	decks          = make(map[string]deck)
)

// In a non-demo provider, we would be querying an API that held our state.
// For this demonstration, we are holding the deck in memory. Therefore,
// creation and state management must take place within our Observe method.
func deckClient(name string, creds []byte) error {
	suits := [4]string{"♠", "♥", "♦", "♣"}
	ranks := [13]string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}

	d := deck{}

	if _, ok := decks[name]; ok {
		d = decks[name]
	}

	// If the deck has cards, we're done
	if len(d.Cards) != 0 {
		return nil
	}

	// If there are no cards, instantiate a new deck using the Credentials Seed
	cc := cardCredentials{}
	err := json.Unmarshal(creds, &cc)
	if err != nil {
		return err
	}

	cards := []v1alpha1.Card{}
	for _, suit := range suits {
		for _, rank := range ranks {
			newCard := v1alpha1.Card{
				Status: v1alpha1.CardStatus{
					AtProvider: v1alpha1.CardObservation{
						Suit: suit,
						Rank: rank,
						Face: suit + rank,
					},
				},
			}
			cards = append(cards, newCard)
		}
	}

	rand.Seed(cc.Seed)
	for i := range cards {
		j := rand.Intn(i + 1) //nolint:golint,gosec
		cards[i], cards[j] = cards[j], cards[i]
	}

	d.Cards = cards
	decks[name] = d

	return nil
}

// cardClient verifies the card is valid
func cardClient(face string, deckName string) error {
	// Because we store the deck in-memory, we can lose the state on a container
	// restart. Because of this, we double-check that this card is _not_ in the
	// deck. If found, we remove it.
	deckCards := decks[deckName].Cards
	newCards := []v1alpha1.Card{}

	if len(deckCards) == 0 {
		return errors.New("no cards found")
	}

	for _, card := range deckCards {
		if card.Status.AtProvider.Face != face {
			newCards = append(newCards, card)
		}
	}

	d := deck{
		Cards: newCards,
	}

	decks[deckName] = d

	return nil
}

// Setup adds a controller that reconciles Card managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter) error {
	name := managed.ControllerName(v1alpha1.CardGroupKind)

	o := controller.Options{
		RateLimiter: ratelimiter.NewDefaultManagedRateLimiter(rl),
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.CardGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: newNoOpService,
		}),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o).
		For(&v1alpha1.Card{}).
		Complete(r)
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(creds []byte) (interface{}, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Card)
	if !ok {
		return nil, errors.New(errNotCard)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	cd := pc.Spec.Credentials
	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	svc, err := c.newServiceFn(data)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	err = deckClient(pc.Name, data)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: svc}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	service interface{}
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Card)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotCard)
	}

	// If there is no Face defined, we have observed a card that needs to be
	// created (dealt).
	if cr.Status.AtProvider.Face == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	// If we have a Face defined, we want to confirm it is a valid card
	deckName := cr.GetProviderConfigReference().Name
	err := cardClient(cr.Status.AtProvider.Face, deckName)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	cr.SetConditions(xpv1.Available())
	return managed.ExternalObservation{
		// Return false when the external resource does not exist. This lets
		// the managed resource reconciler know that it needs to call Create to
		// (re)create the resource, or that it has successfully been deleted.
		ResourceExists: true,

		// Return false when the external resource exists, but it not up to date
		// with the desired managed resource state. This lets the managed
		// resource reconciler know that it needs to call Update.
		ResourceUpToDate: true,

		// Return any details that may be required to connect to the external
		// resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Card)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotCard)
	}

	fmt.Printf("Dealing Card: %+v\n", cr)
	deckName := cr.GetProviderConfigReference().Name
	thisDeck := decks[deckName]

	// Remove the top card from the deck and truncate deck.
	cr.Status.AtProvider = thisDeck.Cards[0].Status.AtProvider // Copy first card.
	thisDeck.Cards = thisDeck.Cards[1:]                        // Truncate deck.
	decks[deckName] = thisDeck                                 // Put deck back.

	fmt.Printf("Dealt card from: %s\n", deckName)
	fmt.Printf("Deck now has %d cards\n", len(decks[deckName].Cards))

	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{
			"Face": []byte(cr.Status.AtProvider.Face),
		},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Card)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotCard)
	}

	fmt.Printf("Updating: %+v", cr)

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Card)
	if !ok {
		return errors.New(errNotCard)
	}

	fmt.Printf("Deleting: %+v", cr)

	disCard := v1alpha1.Card{
		Status: v1alpha1.CardStatus{
			AtProvider: cr.Status.AtProvider,
		},
	}

	cr.Status.AtProvider = v1alpha1.CardObservation{}

	deckName := cr.Spec.ProviderConfigReference.Name
	disCards := append(decks[deckName].Cards, disCard)
	disDeck := deck{Cards: disCards}

	decks[deckName] = disDeck

	return nil
}
